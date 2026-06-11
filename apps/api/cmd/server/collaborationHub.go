package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type collaborationHub struct {
	mu    sync.Mutex
	rooms map[string]*collaborationRoom
	store *sessionProjectStore
}

type collaborationRoom struct {
	key       string
	projectID string
	screenID  string
	draft     collaborationDraft
	clients   map[string]*collaborationClient
}

type collaborationDraft struct {
	SourcePug    string          `json:"sourcePug"`
	CSS          string          `json:"css"`
	Data         json.RawMessage `json:"data"`
	BaseRevision int             `json:"baseRevision"`
	DocVersion   int64           `json:"docVersion"`
	UpdatedAt    string          `json:"updatedAt"`
}

type collaborationClient struct {
	id   string
	name string
	conn *websocket.Conn
	send chan collaborationServerMessage
	room *collaborationRoom
	hub  *collaborationHub
}

type collaborationPresence struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type collaborationClientMessage struct {
	Type  string          `json:"type"`
	Field string          `json:"field,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

type collaborationServerMessage struct {
	Type       string                  `json:"type"`
	Field      string                  `json:"field,omitempty"`
	Value      json.RawMessage         `json:"value,omitempty"`
	Draft      *collaborationDraft     `json:"draft,omitempty"`
	ClientID   string                  `json:"clientId,omitempty"`
	ClientName string                  `json:"clientName,omitempty"`
	DocVersion int64                   `json:"docVersion,omitempty"`
	Presence   []collaborationPresence `json:"presence,omitempty"`
	Message    string                  `json:"message,omitempty"`
}

func newCollaborationHub(store *sessionProjectStore) *collaborationHub {
	return &collaborationHub{
		rooms: make(map[string]*collaborationRoom),
		store: store,
	}
}

func (h *collaborationHub) handleScreen(w http.ResponseWriter, r *http.Request, projectID, screenID string) {
	projectID = strings.TrimSpace(projectID)
	screenID = strings.TrimSpace(screenID)
	if projectID == "" || screenID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId and screenId are required"})
		return
	}

	if ok, err := h.store.screenBelongsToProject(r.Context(), projectID, screenID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	} else if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "screen not found"})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
	if clientID == "" {
		clientID = newSessionID()
	}
	clientName := strings.TrimSpace(r.URL.Query().Get("name"))
	if clientName == "" {
		clientName = "Collaborator"
	}

	room, err := h.joinRoom(r.Context(), projectID, screenID, &collaborationClient{
		id:   clientID,
		name: clientName,
		conn: conn,
		send: make(chan collaborationServerMessage, 32),
		hub:  h,
	})
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return
	}

	client := room.clients[clientID]
	defer client.close()

	go client.writeLoop()
	draft := room.draft
	draft.Data = append(json.RawMessage(nil), room.draft.Data...)
	client.send <- collaborationServerMessage{
		Type:  "snapshot",
		Draft: &draft,
	}
	h.broadcastPresence(room)
	client.readLoop(r.Context())
}

func (h *collaborationHub) joinRoom(ctx context.Context, projectID, screenID string, client *collaborationClient) (*collaborationRoom, error) {
	key := collaborationRoomKey(projectID, screenID)

	h.mu.Lock()
	if room := h.rooms[key]; room != nil {
		client.room = room
		room.clients[client.id] = client
		h.mu.Unlock()
		return room, nil
	}
	h.mu.Unlock()

	draft, err := h.loadInitialDraft(ctx, screenID)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[key]
	if room == nil {
		room = &collaborationRoom{
			key:       key,
			projectID: projectID,
			screenID:  screenID,
			draft:     draft,
			clients:   make(map[string]*collaborationClient),
		}
		h.rooms[key] = room
	}
	client.room = room
	room.clients[client.id] = client
	return room, nil
}

func (h *collaborationHub) loadInitialDraft(ctx context.Context, screenID string) (collaborationDraft, error) {
	state, err := h.store.getLatestState(ctx, screenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return collaborationDraft{
				Data:      json.RawMessage(`{}`),
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return collaborationDraft{}, err
	}
	data := state.Payload.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	return collaborationDraft{
		SourcePug:    state.Payload.SourcePug,
		CSS:          state.Payload.CSS,
		Data:         data,
		BaseRevision: state.Revision,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (h *collaborationHub) leave(client *collaborationClient) {
	h.mu.Lock()
	room := client.room
	if room == nil {
		h.mu.Unlock()
		return
	}
	delete(room.clients, client.id)
	if len(room.clients) == 0 {
		delete(h.rooms, room.key)
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.broadcastPresence(room)
}

func (h *collaborationHub) applyFieldUpdate(client *collaborationClient, field string, value json.RawMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	room := client.room
	if room == nil {
		return fmt.Errorf("client is not attached to a collaboration room")
	}
	switch field {
	case "sourcePug":
		var next string
		if err := json.Unmarshal(value, &next); err != nil {
			return fmt.Errorf("sourcePug must be a string")
		}
		room.draft.SourcePug = next
	case "css":
		var next string
		if err := json.Unmarshal(value, &next); err != nil {
			return fmt.Errorf("css must be a string")
		}
		room.draft.CSS = next
	case "data":
		if !json.Valid(value) {
			return fmt.Errorf("data must be valid json")
		}
		room.draft.Data = append(json.RawMessage(nil), value...)
	default:
		return fmt.Errorf("unsupported collaboration field %q", field)
	}

	room.draft.DocVersion += 1
	room.draft.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	message := collaborationServerMessage{
		Type:       "field_updated",
		Field:      field,
		Value:      append(json.RawMessage(nil), value...),
		ClientID:   client.id,
		ClientName: client.name,
		DocVersion: room.draft.DocVersion,
	}
	for id, peer := range room.clients {
		if id == client.id {
			continue
		}
		select {
		case peer.send <- message:
		default:
		}
	}
	return nil
}

func (h *collaborationHub) broadcastPresence(room *collaborationRoom) {
	h.mu.Lock()
	presence := make([]collaborationPresence, 0, len(room.clients))
	for _, client := range room.clients {
		presence = append(presence, collaborationPresence{ID: client.id, Name: client.name})
	}
	message := collaborationServerMessage{
		Type:     "presence",
		Presence: presence,
	}
	clients := make([]*collaborationClient, 0, len(room.clients))
	for _, client := range room.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	for _, client := range clients {
		select {
		case client.send <- message:
		default:
		}
	}
}

func (c *collaborationClient) readLoop(ctx context.Context) {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		var message collaborationClientMessage
		if err := json.Unmarshal(data, &message); err != nil {
			c.sendError("invalid collaboration message")
			continue
		}
		switch message.Type {
		case "field_update":
			if err := c.hub.applyFieldUpdate(c, strings.TrimSpace(message.Field), message.Value); err != nil {
				c.sendError(err.Error())
			}
		}
	}
}

func (c *collaborationClient) writeLoop() {
	for message := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := wsjsonWrite(ctx, c.conn, message)
		cancel()
		if err != nil {
			return
		}
	}
}

func (c *collaborationClient) sendError(message string) {
	select {
	case c.send <- collaborationServerMessage{Type: "error", Message: message}:
	default:
	}
}

func (c *collaborationClient) close() {
	c.hub.leave(c)
	close(c.send)
	_ = c.conn.Close(websocket.StatusNormalClosure, "")
}

func collaborationRoomKey(projectID, screenID string) string {
	return strings.TrimSpace(projectID) + "/" + strings.TrimSpace(screenID)
}

func wsjsonWrite(ctx context.Context, conn *websocket.Conn, payload collaborationServerMessage) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
