package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/realtime-prototype/api/internal/db/sessionmigrations"
	_ "modernc.org/sqlite"
)

const (
	defaultSessionDatabasePath = "data/session-store.sqlite"
	defaultProjectID          = "project-default"
	defaultTheme              = "bootstrap"
)

var (
	errProjectNotFound      = errors.New("project not found")
	errProjectNameRequired  = errors.New("project name is required")
	errProjectDeleteDefault = errors.New("default project cannot be deleted")
	errProjectDeleteLast    = errors.New("cannot delete the last project")
)

type sessionChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sessionPayload struct {
	SourcePug string              `json:"sourcePug"`
	CSS       string              `json:"css"`
	Data      json.RawMessage     `json:"data"`
	Messages  []cerebrasChatMessage `json:"messages"`
	Metadata  json.RawMessage     `json:"metadata"`
}

type flowTaskPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type flowDiagramTask struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	ScreenID string          `json:"screenId"`
	IsPopup  bool            `json:"isPopupTask"`
	Position flowTaskPosition `json:"position"`
}

type flowDiagramConnection struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	SourceHandle    *string `json:"sourceHandle"`
	TargetHandle    *string `json:"targetHandle"`
	IsSubmitPrimary bool    `json:"isSubmitPrimary"`
}

type taskFlowDiagram struct {
	Tasks []flowDiagramTask       `json:"tasks"`
	Edges []flowDiagramConnection `json:"edges"`
}

type flowDiagramRecord struct {
	ProjectID string         `json:"projectId"`
	Diagram  taskFlowDiagram `json:"diagram"`
	UpdatedAt string        `json:"updatedAt"`
}

type saveScreenStateRequest struct {
	Conversation    []sessionChatMessage `json:"conversation"`
	Recommendations []string             `json:"recommendations"`
	Payload         sessionPayload       `json:"screenPayload"`
}

type projectRecord struct {
	ID           string
	Name         string
	Theme        string
	ActiveScreen string
	CreatedAt    string
	UpdatedAt    string
}

type projectSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Theme           string `json:"theme"`
	ActiveScreenID  string `json:"activeScreenId"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type screenRecord struct {
	ID        string
	ProjectID string
	Name      string
	Position  int
	UpdatedAt string
	IsActive  bool
}

type screenStateRecord struct {
	ID              int64
	ScreenID        string
	Revision        int
	ScreenPayload   string
	Conversation    string
	Recommendations string
	CreatedAt       string
}

type screenSessionState struct {
	ID              int64              `json:"id"`
	Revision        int                `json:"revision"`
	Payload         sessionPayload     `json:"screenPayload"`
	Conversation    []sessionChatMessage `json:"conversation"`
	Recommendations []string           `json:"recommendations"`
	CreatedAt       string             `json:"createdAt"`
}

type sessionScreenSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	UpdatedAt  string `json:"updatedAt"`
	IsActive   bool   `json:"isActive"`
	LastRevision int  `json:"lastRevision"`
}

type screenStateSummary struct {
	ID        int64  `json:"id"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"createdAt"`
}

type screenStateListResponse struct {
	Items []screenStateSummary `json:"items"`
}

type sessionSnapshot struct {
	ProjectID      string             `json:"projectId"`
	ProjectName    string             `json:"projectName"`
	Theme          string             `json:"theme"`
	ActiveScreenID string             `json:"activeScreenId"`
	Screens        []sessionScreenSummary `json:"screens"`
	ActiveState    *screenSessionState `json:"activeState"`
}

type sessionProjectStore struct {
	db *sql.DB
}

func newSessionProjectStore(path string) (*sessionProjectStore, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultSessionDatabasePath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &sessionProjectStore{db: db}
	if err := store.bootstrap(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.normalizeScreenNames(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *sessionProjectStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *sessionProjectStore) bootstrap(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return err
	}

	if err := sessionmigrations.RunMigrations(ctx, s.db); err != nil {
		return err
	}
	return nil
}

func (s *sessionProjectStore) normalizeScreenNames(ctx context.Context) error {
	const projectQuery = `SELECT id FROM projects;`
	projects, err := s.db.QueryContext(ctx, projectQuery)
	if err != nil {
		return err
	}
	defer projects.Close()

	for projects.Next() {
		var projectID string
		if err := projects.Scan(&projectID); err != nil {
			return err
		}

		if err := s.normalizeProjectScreenNames(ctx, projectID); err != nil {
			return err
		}
	}
	return projects.Err()
}

func (s *sessionProjectStore) normalizeProjectScreenNames(ctx context.Context, projectID string) error {
	const screensQuery = `
		SELECT id, name
		FROM screens
		WHERE project_id = ? AND is_deleted = 0
		ORDER BY created_at ASC, id ASC;
	`
	rows, err := s.db.QueryContext(ctx, screensQuery, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()

	usedNames := make(map[string]int, 16)
	for rows.Next() {
		var screenID string
		var screenName string
		if err := rows.Scan(&screenID, &screenName); err != nil {
			return err
		}

		seen := usedNames[screenName]
		if seen == 0 {
			usedNames[screenName] = 1
			continue
		}

		nextName := fmt.Sprintf("%s (%d)", screenName, seen)
		usedNames[screenName] = seen + 1
		usedNames[nextName] = 1

		if _, err := s.db.ExecContext(ctx, `UPDATE screens SET name = ? WHERE id = ?;`, nextName, screenID); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (s *sessionProjectStore) getDefaultProject(ctx context.Context) (projectRecord, error) {
	const query = `
		SELECT id, name, theme, COALESCE(active_screen_id, ''), created_at, updated_at
		FROM projects
		WHERE id = ?;
	`
	var project projectRecord
	row := s.db.QueryRowContext(ctx, query, defaultProjectID)
	if err := row.Scan(&project.ID, &project.Name, &project.Theme, &project.ActiveScreen, &project.CreatedAt, &project.UpdatedAt); err != nil {
		return project, err
	}
	return project, nil
}

func (s *sessionProjectStore) getProject(ctx context.Context, projectID string) (projectRecord, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return s.getDefaultProject(ctx)
	}

	const query = `
		SELECT id, name, theme, COALESCE(active_screen_id, ''), created_at, updated_at
		FROM projects
		WHERE id = ?;
	`
	var project projectRecord
	row := s.db.QueryRowContext(ctx, query, projectID)
	if err := row.Scan(&project.ID, &project.Name, &project.Theme, &project.ActiveScreen, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return project, errProjectNotFound
		}
		return project, err
	}
	return project, nil
}

func (s *sessionProjectStore) listProjects(ctx context.Context) ([]projectSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, theme, COALESCE(active_screen_id, ''), created_at, updated_at
		 FROM projects
		 ORDER BY updated_at DESC, created_at DESC;`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]projectSummary, 0)
	for rows.Next() {
		var project projectSummary
		if err := rows.Scan(&project.ID, &project.Name, &project.Theme, &project.ActiveScreenID, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *sessionProjectStore) createProject(ctx context.Context, name string) (projectSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Nuevo proyecto"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	projectID := fmt.Sprintf("project-%s", newSessionID())
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO projects (id, name, theme, active_screen_id, created_at, updated_at)
		 VALUES (?, ?, ?, NULL, ?, ?);`,
		projectID,
		name,
		defaultTheme,
		now,
		now,
	)
	if err != nil {
		return projectSummary{}, err
	}

	return projectSummary{
		ID:     projectID,
		Name:   name,
		Theme:  defaultTheme,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *sessionProjectStore) renameProject(ctx context.Context, projectID, name string) error {
	projectID = strings.TrimSpace(projectID)
	name = strings.TrimSpace(name)
	if projectID == "" {
		return errProjectNotFound
	}
	if name == "" {
		return errProjectNameRequired
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE projects SET name = ?, updated_at = ? WHERE id = ?;`,
		name,
		now,
		projectID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errProjectNotFound
	}
	return nil
}

func (s *sessionProjectStore) deleteProject(ctx context.Context, projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return errProjectNotFound
	}
	if projectID == defaultProjectID {
		return errProjectDeleteDefault
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM projects;`).Scan(&total); err != nil {
		return err
	}
	if total <= 1 {
		return errProjectDeleteLast
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?;`, projectID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errProjectNotFound
	}
	return nil
}

func (s *sessionProjectStore) listScreens(ctx context.Context, projectID string) ([]sessionScreenSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, position, updated_at, is_active
		FROM screens
		WHERE project_id = ? AND is_deleted = 0
		ORDER BY is_active DESC, position ASC, updated_at DESC;`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	screens := []sessionScreenSummary{}
	for rows.Next() {
		var row screenRecord
		var isActive int
		if err := rows.Scan(&row.ID, &row.Name, &row.Position, &row.UpdatedAt, &isActive); err != nil {
			return nil, err
		}
		screen := sessionScreenSummary{
			ID:        row.ID,
			Name:      row.Name,
			Position:  row.Position,
			UpdatedAt: row.UpdatedAt,
			IsActive:  isActive == 1,
		}
		lastRevision, _ := s.getLatestRevision(ctx, row.ID)
		screen.LastRevision = lastRevision
		screens = append(screens, screen)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return screens, nil
}

func (s *sessionProjectStore) getLatestRevision(ctx context.Context, screenID string) (int, error) {
	var revision int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM screen_states WHERE screen_id = ?;`,
		screenID,
	).Scan(&revision)
	if err != nil {
		return 0, err
	}
	return revision, nil
}

func (s *sessionProjectStore) createScreen(ctx context.Context, projectID, name string) (sessionScreenSummary, error) {
	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM screens WHERE project_id = ? AND is_deleted = 0;`, projectID).Scan(&existing); err != nil {
		return sessionScreenSummary{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Pantalla %d", existing+1)
	} else {
		const countByNameQuery = `SELECT COUNT(1) FROM screens WHERE project_id = ? AND is_deleted = 0 AND name = ?;`
		currentName := name
		for i := 1; ; i++ {
			var conflicts int
			if err := s.db.QueryRowContext(ctx, countByNameQuery, projectID, currentName).Scan(&conflicts); err != nil {
				return sessionScreenSummary{}, err
			}
			if conflicts == 0 {
				break
			}
			currentName = fmt.Sprintf("%s (%d)", name, i)
		}
		name = currentName
	}

	now := time.Now().UTC().Format(time.RFC3339)
	screenID := newSessionID()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sessionScreenSummary{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(
		ctx,
		`UPDATE screens SET is_active = 0 WHERE project_id = ?;`,
		projectID,
	)
	if err != nil {
		return sessionScreenSummary{}, err
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO screens (id, project_id, name, position, created_at, updated_at, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, 1);`,
		screenID,
		projectID,
		name,
		existing+1,
		now,
		now,
	)
	if err != nil {
		return sessionScreenSummary{}, err
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
		screenID,
		now,
		now,
		projectID,
	)
	if err != nil {
		return sessionScreenSummary{}, err
	}
	if err = tx.Commit(); err != nil {
		return sessionScreenSummary{}, err
	}

	return sessionScreenSummary{
		ID:       screenID,
		Name:     name,
		Position: existing + 1,
		IsActive: true,
		UpdatedAt: now,
	}, nil
}

func (s *sessionProjectStore) activateScreen(ctx context.Context, projectID, screenID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE screens SET is_active = CASE WHEN id = ? THEN 1 ELSE 0 END
		 WHERE project_id = ? AND is_deleted = 0;`,
		screenID,
		projectID,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return os.ErrNotExist
	}
	_, err = s.db.ExecContext(
		ctx,
		`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
		screenID,
		now,
		now,
		projectID,
	)
	return err
}

func (s *sessionProjectStore) deleteScreen(ctx context.Context, projectID, screenID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var belongs int
	if err = tx.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
		screenID,
		projectID,
	).Scan(&belongs); err != nil {
		return err
	}
	if belongs == 0 {
		return os.ErrNotExist
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE screens
		 SET is_deleted = 1, is_active = 0
		 WHERE id = ? AND project_id = ?;`,
		screenID,
		projectID,
	); err != nil {
		return err
	}

	var replacementScreenID sql.NullString
	if err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM screens
		 WHERE project_id = ? AND is_deleted = 0
		 ORDER BY is_active DESC, updated_at DESC, position ASC
		 LIMIT 1;`,
		projectID,
	).Scan(&replacementScreenID); err != nil && err != sql.ErrNoRows {
		return err
	}

	if replacementScreenID.Valid {
		nextActiveScreen := replacementScreenID.String
		_, err = tx.ExecContext(
			ctx,
			`UPDATE screens SET is_active = CASE WHEN id = ? THEN 1 ELSE 0 END
			 WHERE project_id = ? AND is_deleted = 0;`,
			nextActiveScreen,
			projectID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			nextActiveScreen,
			now,
			now,
			projectID,
		)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE projects SET active_screen_id = NULL, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
		now,
		now,
		projectID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sessionProjectStore) listScreenStates(ctx context.Context, projectID, screenID string, limit int) ([]screenStateSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var belongs int
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
		screenID,
		projectID,
	).Scan(&belongs); err != nil {
		return nil, err
	}
	if belongs == 0 {
		return nil, os.ErrNotExist
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, revision, created_at
		 FROM screen_states
		 WHERE screen_id = ?
		 ORDER BY revision DESC
		 LIMIT ?;`,
		screenID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stateRows := make([]screenStateSummary, 0)
	for rows.Next() {
		var state screenStateSummary
		if err := rows.Scan(&state.ID, &state.Revision, &state.CreatedAt); err != nil {
			return nil, err
		}
		stateRows = append(stateRows, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stateRows, nil
}

func (s *sessionProjectStore) setTheme(ctx context.Context, projectID, theme string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	theme = strings.TrimSpace(theme)
	if theme == "" {
		theme = defaultTheme
	}
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET theme = ?, updated_at = ? WHERE id = ?;`, theme, now, projectID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return os.ErrNotExist
	}
	return nil
}

func (s *sessionProjectStore) saveState(ctx context.Context, projectID, screenID string, payload saveScreenStateRequest) (*screenSessionState, error) {
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		return nil, fmt.Errorf("invalid screen payload: %w", err)
	}
	conversationBytes, err := json.Marshal(payload.Conversation)
	if err != nil {
		return nil, fmt.Errorf("invalid conversation: %w", err)
	}
	recommendationsBytes, err := json.Marshal(payload.Recommendations)
	if err != nil {
		return nil, fmt.Errorf("invalid recommendations: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var belongs int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`, screenID, projectID).Scan(&belongs); err != nil {
		return nil, err
	}
	if belongs == 0 {
		return nil, os.ErrNotExist
	}

	var revision int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision), 0) + 1 FROM screen_states WHERE screen_id = ?;`, screenID).Scan(&revision); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO screen_states (screen_id, revision, screen_payload_json, conversation_json, recommendations_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?);`,
		screenID, revision, string(payloadBytes), string(conversationBytes), string(recommendationsBytes), now,
	)
	if err != nil {
		return nil, err
	}
	lastInsertID, _ := result.LastInsertId()

	_, err = tx.ExecContext(
		ctx,
		`UPDATE screens SET updated_at = ?, is_active = 1 WHERE id = ?;`,
		now,
		screenID,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
		screenID,
		now,
		now,
		projectID,
	)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &screenSessionState{
		ID:             lastInsertID,
		Revision:       revision,
		Payload:        payload.Payload,
		Conversation:   payload.Conversation,
		Recommendations: payload.Recommendations,
		CreatedAt:      now,
	}, nil
}

func (s *sessionProjectStore) getLatestState(ctx context.Context, screenID string) (*screenSessionState, error) {
	var row screenStateRecord
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, revision, screen_payload_json, conversation_json, recommendations_json, created_at
		 FROM screen_states
		 WHERE screen_id = ?
		 ORDER BY revision DESC LIMIT 1;`,
		screenID,
	).Scan(&row.ID, &row.Revision, &row.ScreenPayload, &row.Conversation, &row.Recommendations, &row.CreatedAt)
	if err != nil {
		return nil, err
	}

	payload, conversation, recommendations := sessionPayload{}, []sessionChatMessage{}, []string{}
	if row.ScreenPayload != "" {
		_ = json.Unmarshal([]byte(row.ScreenPayload), &payload)
	}
	if row.Conversation != "" {
		_ = json.Unmarshal([]byte(row.Conversation), &conversation)
	}
	if row.Recommendations != "" {
		_ = json.Unmarshal([]byte(row.Recommendations), &recommendations)
	}
	return &screenSessionState{
		ID:              row.ID,
		Revision:        row.Revision,
		Payload:         payload,
		Conversation:    conversation,
		Recommendations: recommendations,
		CreatedAt:       row.CreatedAt,
	}, nil
}

func (s *sessionProjectStore) getSnapshot(ctx context.Context, projectID string) (sessionSnapshot, error) {
	project, err := s.getProject(ctx, projectID)
	if err != nil {
		return sessionSnapshot{}, err
	}

	screens, err := s.listScreens(ctx, project.ID)
	if err != nil {
		return sessionSnapshot{}, err
	}
	activeScreenID := strings.TrimSpace(project.ActiveScreen)
	if activeScreenID == "" {
		if len(screens) > 0 {
			activeScreenID = screens[0].ID
			if err := s.activateScreen(ctx, project.ID, activeScreenID); err != nil {
				return sessionSnapshot{}, err
			}
		}
	}
	snapshot := sessionSnapshot{
		ProjectID:      project.ID,
		ProjectName:    project.Name,
		Theme:          project.Theme,
		ActiveScreenID: activeScreenID,
		Screens:        screens,
		ActiveState:    nil,
	}
	if activeScreenID != "" {
		state, stateErr := s.getLatestState(ctx, activeScreenID)
		if stateErr == nil {
			snapshot.ActiveState = state
		} else if !errors.Is(stateErr, sql.ErrNoRows) {
			return sessionSnapshot{}, stateErr
		}
	}

	return snapshot, nil
}

func (s *sessionProjectStore) saveFlowDiagram(ctx context.Context, projectID string, diagram taskFlowDiagram) (flowDiagramRecord, error) {
	diagramPayload, err := json.Marshal(diagram)
	if err != nil {
		return flowDiagramRecord{}, fmt.Errorf("invalid flow diagram payload: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO flow_diagrams (project_id, diagram_payload_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET
		  diagram_payload_json = excluded.diagram_payload_json,
		  updated_at = excluded.updated_at;`,
		projectID,
		string(diagramPayload),
		now,
		now,
	); err != nil {
		return flowDiagramRecord{}, err
	}

	return flowDiagramRecord{
		ProjectID: projectID,
		Diagram:   diagram,
		UpdatedAt: now,
	}, nil
}

func (s *sessionProjectStore) loadFlowDiagram(ctx context.Context, projectID string) (flowDiagramRecord, bool, error) {
	const query = `SELECT diagram_payload_json, updated_at FROM flow_diagrams WHERE project_id = ?;`

	var payload string
	var updatedAt string
	if err := s.db.QueryRowContext(ctx, query, projectID).Scan(&payload, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return flowDiagramRecord{
				ProjectID: projectID,
				Diagram: taskFlowDiagram{
					Tasks: []flowDiagramTask{},
					Edges: []flowDiagramConnection{},
				},
				UpdatedAt: "",
			}, false, nil
		}
		return flowDiagramRecord{}, false, err
	}

	diagram := taskFlowDiagram{
		Tasks: []flowDiagramTask{},
		Edges: []flowDiagramConnection{},
	}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &diagram); err != nil {
			return flowDiagramRecord{}, true, err
		}
	}

	if diagram.Tasks == nil {
		diagram.Tasks = []flowDiagramTask{}
	}
	if diagram.Edges == nil {
		diagram.Edges = []flowDiagramConnection{}
	}

	return flowDiagramRecord{
		ProjectID: projectID,
		Diagram:   diagram,
		UpdatedAt: updatedAt,
	}, true, nil
}

func newSessionID() string {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		now := time.Now().UTC().UnixNano()
		return fmt.Sprintf("screen-%d", now)
	}
	return hex.EncodeToString(bytes)
}
