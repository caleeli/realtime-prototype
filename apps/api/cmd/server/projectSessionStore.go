package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/realtime-prototype/api/internal/db/sessionmigrations"
	_ "modernc.org/sqlite"
)

type queryRowContextProvider interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const (
	defaultSessionDatabasePath = "data/session-store.sqlite"
	defaultProjectID           = "project-default"
	defaultTheme               = "bootstrap"
	sqliteBusyTimeout          = 5000
	sqliteWriteRetryAttempts   = 4
)

var (
	errProjectNotFound      = errors.New("project not found")
	errProjectNameRequired  = errors.New("project name is required")
	errProjectDeleteDefault = errors.New("default project cannot be deleted")
	errProjectDeleteLast    = errors.New("cannot delete the last project")
	errScreenNameRequired   = errors.New("screen name is required")
)

type sessionChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sessionPayload struct {
	SourcePug string                `json:"sourcePug"`
	CSS       string                `json:"css"`
	Data      json.RawMessage       `json:"data"`
	Messages  []cerebrasChatMessage `json:"messages"`
	Metadata  json.RawMessage       `json:"metadata"`
}

type flowTaskPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type flowDiagramTask struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	ScreenID string           `json:"screenId"`
	IsPopup  bool             `json:"isPopupTask"`
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
	ProjectID string          `json:"projectId"`
	Diagram   taskFlowDiagram `json:"diagram"`
	UpdatedAt string          `json:"updatedAt"`
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
	ID             string `json:"id"`
	Name           string `json:"name"`
	Theme          string `json:"theme"`
	ActiveScreenID string `json:"activeScreenId"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
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
	ID              int64                `json:"id"`
	Revision        int                  `json:"revision"`
	Payload         sessionPayload       `json:"screenPayload"`
	Conversation    []sessionChatMessage `json:"conversation"`
	Recommendations []string             `json:"recommendations"`
	CreatedAt       string               `json:"createdAt"`
}

type sessionScreenSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Position     int    `json:"position"`
	UpdatedAt    string `json:"updatedAt"`
	IsActive     bool   `json:"isActive"`
	LastRevision int    `json:"lastRevision"`
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
	ProjectID      string                 `json:"projectId"`
	ProjectName    string                 `json:"projectName"`
	Theme          string                 `json:"theme"`
	ActiveScreenID string                 `json:"activeScreenId"`
	Screens        []sessionScreenSummary `json:"screens"`
	ActiveState    *screenSessionState    `json:"activeState"`
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
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

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
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA busy_timeout = %d;`, sqliteBusyTimeout)); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA synchronous = NORMAL;`); err != nil {
		return err
	}

	if err := sessionmigrations.RunMigrations(ctx, s.db); err != nil {
		return err
	}
	return nil
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database is busy")
}

func (s *sessionProjectStore) withWriteRetry(ctx context.Context, fn func() error) error {
	backoff := 40 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < sqliteWriteRetryAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isSQLiteBusy(err) || attempt == sqliteWriteRetryAttempts-1 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return lastErr
}

func (s *sessionProjectStore) normalizeScreenNames(ctx context.Context) error {
	const projectQuery = `SELECT id FROM projects;`
	projects, err := s.db.QueryContext(ctx, projectQuery)
	if err != nil {
		return err
	}

	projectIDs := make([]string, 0, 8)

	for projects.Next() {
		var projectID string
		if err := projects.Scan(&projectID); err != nil {
			_ = projects.Close()
			return err
		}
		projectIDs = append(projectIDs, projectID)
	}
	if err := projects.Err(); err != nil {
		_ = projects.Close()
		return err
	}
	if err := projects.Close(); err != nil {
		return err
	}

	for _, projectID := range projectIDs {
		if err := s.normalizeProjectScreenNames(ctx, projectID); err != nil {
			return err
		}
	}
	return nil
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

	type screenNameRow struct {
		id   string
		name string
	}
	screenRows := make([]screenNameRow, 0, 16)

	for rows.Next() {
		var screenID string
		var screenName string
		if err := rows.Scan(&screenID, &screenName); err != nil {
			_ = rows.Close()
			return err
		}
		screenRows = append(screenRows, screenNameRow{id: screenID, name: screenName})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	usedNames := make(map[string]int, 16)
	for _, row := range screenRows {
		screenID := row.id
		screenName := row.name
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

	return nil
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
	err := s.withWriteRetry(ctx, func() error {
		_, execErr := s.db.ExecContext(
			ctx,
			`INSERT INTO projects (id, name, theme, active_screen_id, created_at, updated_at)
			 VALUES (?, ?, ?, NULL, ?, ?);`,
			projectID,
			name,
			defaultTheme,
			now,
			now,
		)
		return execErr
	})
	if err != nil {
		return projectSummary{}, err
	}

	return projectSummary{
		ID:        projectID,
		Name:      name,
		Theme:     defaultTheme,
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
	var result sql.Result
	err := s.withWriteRetry(ctx, func() error {
		var execErr error
		result, execErr = s.db.ExecContext(
			ctx,
			`UPDATE projects SET name = ?, updated_at = ? WHERE id = ?;`,
			name,
			now,
			projectID,
		)
		return execErr
	})
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

	var result sql.Result
	err := s.withWriteRetry(ctx, func() error {
		var execErr error
		result, execErr = s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?;`, projectID)
		return execErr
	})
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
		`SELECT
			s.id,
			s.name,
			s.position,
			s.updated_at,
			s.is_active,
			COALESCE(MAX(ss.revision), 0) AS last_revision
		FROM screens s
		LEFT JOIN screen_states ss ON ss.screen_id = s.id
		WHERE s.project_id = ? AND s.is_deleted = 0
		GROUP BY s.id, s.name, s.position, s.updated_at, s.is_active
		ORDER BY s.is_active DESC, s.position ASC, s.updated_at DESC;`,
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
		var lastRevision int
		if err := rows.Scan(&row.ID, &row.Name, &row.Position, &row.UpdatedAt, &isActive, &lastRevision); err != nil {
			return nil, err
		}
		screen := sessionScreenSummary{
			ID:           row.ID,
			Name:         row.Name,
			Position:     row.Position,
			UpdatedAt:    row.UpdatedAt,
			IsActive:     isActive == 1,
			LastRevision: lastRevision,
		}
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
	name = s.nextAvailableScreenName(ctx, projectID, name, existing+1)
	if name == "" {
		return sessionScreenSummary{}, fmt.Errorf("could not resolve screen name")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	screenID := newSessionID()

	err := s.withWriteRetry(ctx, func() error {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE screens SET is_active = 0 WHERE project_id = ?;`,
			projectID,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`INSERT INTO screens (id, project_id, name, position, created_at, updated_at, is_active)
			 VALUES (?, ?, ?, ?, ?, ?, 1);`,
			screenID,
			projectID,
			name,
			existing+1,
			now,
			now,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			screenID,
			now,
			now,
			projectID,
		); txErr != nil {
			return txErr
		}
		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		committed = true
		return nil
	})
	if err != nil {
		return sessionScreenSummary{}, err
	}

	return sessionScreenSummary{
		ID:        screenID,
		Name:      name,
		Position:  existing + 1,
		IsActive:  true,
		UpdatedAt: now,
	}, nil
}

func (s *sessionProjectStore) renameScreen(ctx context.Context, projectID, screenID, name string) error {
	projectID = strings.TrimSpace(projectID)
	screenID = strings.TrimSpace(screenID)
	name = strings.TrimSpace(name)
	if projectID == "" || screenID == "" {
		return os.ErrNotExist
	}
	if name == "" {
		return errScreenNameRequired
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err := s.withWriteRetry(ctx, func() error {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var belongs int
		if txErr = tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
			screenID,
			projectID,
		).Scan(&belongs); txErr != nil {
			return txErr
		}
		if belongs == 0 {
			return os.ErrNotExist
		}

		resolvedName := nextAvailableScreenNameFromQuery(ctx, tx, projectID, name, 1)
		if resolvedName == "" {
			return fmt.Errorf("could not resolve screen name")
		}

		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE screens SET name = ?, updated_at = ? WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
			resolvedName,
			now,
			screenID,
			projectID,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE projects SET updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			now,
			now,
			projectID,
		); txErr != nil {
			return txErr
		}

		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		committed = true
		return nil
	})
	return err
}

func (s *sessionProjectStore) nextAvailableScreenName(ctx context.Context, projectID, requestedName string, fallbackIndex int) string {
	return nextAvailableScreenNameFromQuery(ctx, s.db, projectID, requestedName, fallbackIndex)
}

func nextAvailableScreenNameFromQuery(ctx context.Context, queryer queryRowContextProvider, projectID, requestedName string, fallbackIndex int) string {
	baseName := strings.TrimSpace(requestedName)
	if baseName == "" {
		baseName = fmt.Sprintf("Pantalla %d", fallbackIndex)
	}
	const countByNameQuery = `SELECT COUNT(1) FROM screens WHERE project_id = ? AND is_deleted = 0 AND name = ?;`
	currentName := baseName
	for i := 1; ; i++ {
		var conflicts int
		if err := queryer.QueryRowContext(ctx, countByNameQuery, projectID, currentName).Scan(&conflicts); err != nil {
			return ""
		}
		if conflicts == 0 {
			return currentName
		}
		currentName = fmt.Sprintf("%s (%d)", baseName, i)
	}
}

func (s *sessionProjectStore) duplicateScreen(ctx context.Context, projectID, sourceScreenID string) (sessionScreenSummary, error) {
	var created sessionScreenSummary
	err := s.withWriteRetry(ctx, func() error {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var sourceScreenName string
		var sourceBelongs int
		if txErr = tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1), COALESCE(MAX(name), '') FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
			sourceScreenID,
			projectID,
		).Scan(&sourceBelongs, &sourceScreenName); txErr != nil {
			return txErr
		}
		if sourceBelongs == 0 {
			return os.ErrNotExist
		}

		var sourceState screenStateRecord
		txErr = tx.QueryRowContext(
			ctx,
			`SELECT id, screen_id, revision, screen_payload_json, conversation_json, recommendations_json, created_at
			 FROM screen_states
			 WHERE screen_id = ?
			 ORDER BY revision DESC LIMIT 1;`,
			sourceScreenID,
		).Scan(
			&sourceState.ID,
			&sourceState.ScreenID,
			&sourceState.Revision,
			&sourceState.ScreenPayload,
			&sourceState.Conversation,
			&sourceState.Recommendations,
			&sourceState.CreatedAt,
		)
		if txErr != nil && !errors.Is(txErr, sql.ErrNoRows) {
			return txErr
		}

		payload := sessionPayload{
			SourcePug: "",
			CSS:       "",
			Data:      json.RawMessage("{}"),
			Messages:  []cerebrasChatMessage{},
			Metadata:  nil,
		}
		if txErr == nil && strings.TrimSpace(sourceState.ScreenPayload) != "" {
			_ = json.Unmarshal([]byte(sourceState.ScreenPayload), &payload)
		}
		if len(payload.Data) == 0 {
			payload.Data = json.RawMessage("{}")
		}

		assistantResponsePayload := struct {
			Pug  string          `json:"pug"`
			CSS  string          `json:"css"`
			Data json.RawMessage `json:"data"`
		}{
			Pug:  payload.SourcePug,
			CSS:  payload.CSS,
			Data: payload.Data,
		}
		assistantResponseBytes, marshalErr := json.Marshal(assistantResponsePayload)
		if marshalErr != nil {
			return marshalErr
		}
		assistantContent := string(assistantResponseBytes)

		condensedMessages := []cerebrasChatMessage{
			{Role: "user", Content: "screen"},
			{Role: "assistant", Content: assistantContent},
		}
		condensedConversation := []sessionChatMessage{
			{Role: "user", Content: "screen"},
			{Role: "assistant", Content: assistantContent},
		}
		payload.Messages = condensedMessages

		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		conversationBytes, marshalErr := json.Marshal(condensedConversation)
		if marshalErr != nil {
			return marshalErr
		}
		recommendationsBytes, marshalErr := json.Marshal([]string{})
		if marshalErr != nil {
			return marshalErr
		}

		var existing int
		if txErr = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM screens WHERE project_id = ? AND is_deleted = 0;`, projectID).Scan(&existing); txErr != nil {
			return txErr
		}

		duplicateName := strings.TrimSpace(sourceScreenName)
		if duplicateName == "" {
			duplicateName = "Pantalla"
		}
		duplicateName = fmt.Sprintf("%s (copia)", duplicateName)
		duplicateName = nextAvailableScreenNameFromQuery(ctx, tx, projectID, duplicateName, existing+1)
		if duplicateName == "" {
			return fmt.Errorf("could not resolve duplicate screen name")
		}

		now := time.Now().UTC().Format(time.RFC3339)
		duplicatedScreenID := newSessionID()
		if _, txErr = tx.ExecContext(ctx, `UPDATE screens SET is_active = 0 WHERE project_id = ?;`, projectID); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`INSERT INTO screens (id, project_id, name, position, created_at, updated_at, is_active)
			 VALUES (?, ?, ?, ?, ?, ?, 1);`,
			duplicatedScreenID,
			projectID,
			duplicateName,
			existing+1,
			now,
			now,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`INSERT INTO screen_states (screen_id, revision, screen_payload_json, conversation_json, recommendations_json, created_at)
			 VALUES (?, 1, ?, ?, ?, ?);`,
			duplicatedScreenID,
			string(payloadBytes),
			string(conversationBytes),
			string(recommendationsBytes),
			now,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			duplicatedScreenID,
			now,
			now,
			projectID,
		); txErr != nil {
			return txErr
		}

		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		committed = true
		created = sessionScreenSummary{
			ID:           duplicatedScreenID,
			Name:         duplicateName,
			Position:     existing + 1,
			UpdatedAt:    now,
			IsActive:     true,
			LastRevision: 1,
		}
		return nil
	})
	if err != nil {
		return sessionScreenSummary{}, err
	}
	return created, nil
}

func (s *sessionProjectStore) activateScreen(ctx context.Context, projectID, screenID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var result sql.Result
	err := s.withWriteRetry(ctx, func() error {
		var execErr error
		result, execErr = s.db.ExecContext(
			ctx,
			`UPDATE screens SET is_active = CASE WHEN id = ? THEN 1 ELSE 0 END
			 WHERE project_id = ? AND is_deleted = 0;`,
			screenID,
			projectID,
		)
		return execErr
	})
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return os.ErrNotExist
	}
	err = s.withWriteRetry(ctx, func() error {
		_, execErr := s.db.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			screenID,
			now,
			now,
			projectID,
		)
		return execErr
	})
	return err
}

func (s *sessionProjectStore) deleteScreen(ctx context.Context, projectID, screenID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.withWriteRetry(ctx, func() error {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var belongs int
		if txErr = tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`,
			screenID,
			projectID,
		).Scan(&belongs); txErr != nil {
			return txErr
		}
		if belongs == 0 {
			return os.ErrNotExist
		}

		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE screens
			 SET is_deleted = 1, is_active = 0
			 WHERE id = ? AND project_id = ?;`,
			screenID,
			projectID,
		); txErr != nil {
			return txErr
		}

		var replacementScreenID sql.NullString
		if txErr = tx.QueryRowContext(
			ctx,
			`SELECT id FROM screens
			 WHERE project_id = ? AND is_deleted = 0
			 ORDER BY is_active DESC, updated_at DESC, position ASC
			 LIMIT 1;`,
			projectID,
		).Scan(&replacementScreenID); txErr != nil && txErr != sql.ErrNoRows {
			return txErr
		}

		if replacementScreenID.Valid {
			nextActiveScreen := replacementScreenID.String
			if _, txErr = tx.ExecContext(
				ctx,
				`UPDATE screens SET is_active = CASE WHEN id = ? THEN 1 ELSE 0 END
				 WHERE project_id = ? AND is_deleted = 0;`,
				nextActiveScreen,
				projectID,
			); txErr != nil {
				return txErr
			}
			if _, txErr = tx.ExecContext(
				ctx,
				`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
				nextActiveScreen,
				now,
				now,
				projectID,
			); txErr != nil {
				return txErr
			}
			if txErr = tx.Commit(); txErr != nil {
				return txErr
			}
			committed = true
			return nil
		}

		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = NULL, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			now,
			now,
			projectID,
		); txErr != nil {
			return txErr
		}

		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		committed = true
		return nil
	})
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
	var result sql.Result
	err := s.withWriteRetry(ctx, func() error {
		var execErr error
		result, execErr = s.db.ExecContext(ctx, `UPDATE projects SET theme = ?, updated_at = ? WHERE id = ?;`, theme, now, projectID)
		return execErr
	})
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

	var (
		lastInsertID int64
		revision     int
		now          string
	)
	err = s.withWriteRetry(ctx, func() error {
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var belongs int
		if txErr = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM screens WHERE id = ? AND project_id = ? AND is_deleted = 0;`, screenID, projectID).Scan(&belongs); txErr != nil {
			return txErr
		}
		if belongs == 0 {
			return os.ErrNotExist
		}

		if txErr = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision), 0) + 1 FROM screen_states WHERE screen_id = ?;`, screenID).Scan(&revision); txErr != nil {
			return txErr
		}

		now = time.Now().UTC().Format(time.RFC3339)
		var result sql.Result
		result, txErr = tx.ExecContext(
			ctx,
			`INSERT INTO screen_states (screen_id, revision, screen_payload_json, conversation_json, recommendations_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?);`,
			screenID, revision, string(payloadBytes), string(conversationBytes), string(recommendationsBytes), now,
		)
		if txErr != nil {
			return txErr
		}
		lastInsertID, _ = result.LastInsertId()

		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE screens SET updated_at = ?, is_active = 1 WHERE id = ?;`,
			now,
			screenID,
		); txErr != nil {
			return txErr
		}
		if _, txErr = tx.ExecContext(
			ctx,
			`UPDATE projects SET active_screen_id = ?, updated_at = ?, last_opened_at = ? WHERE id = ?;`,
			screenID,
			now,
			now,
			projectID,
		); txErr != nil {
			return txErr
		}

		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		committed = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &screenSessionState{
		ID:              lastInsertID,
		Revision:        revision,
		Payload:         payload.Payload,
		Conversation:    payload.Conversation,
		Recommendations: payload.Recommendations,
		CreatedAt:       now,
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
	if err := s.withWriteRetry(ctx, func() error {
		_, execErr := s.db.ExecContext(
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
		)
		return execErr
	}); err != nil {
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

type projectSettingsRecord struct {
	ProjectID            string `json:"projectId"`
	DesignStyle          string `json:"designStyle"`
	ColorPalette         string `json:"colorPalette"`
	BrandGuidelines      string `json:"brandGuidelines"`
	ComponentExamples    string `json:"componentExamples"`
	TechnicalConstraints string `json:"technicalConstraints"`
	LayoutPreferences    string `json:"layoutPreferences"`
	ImageGenerationNotes string `json:"imageGenerationNotes"`
	GenerationContext    string `json:"generationContext"`
	UpdatedAt            string `json:"updatedAt"`
}

func (s *sessionProjectStore) getProjectSettings(ctx context.Context, projectID string) (projectSettingsRecord, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return projectSettingsRecord{}, errProjectNotFound
	}

	const query = `
		SELECT project_id, design_style, color_palette, brand_guidelines,
		       component_examples, technical_constraints, layout_preferences,
		       image_generation_notes, generation_context, updated_at
		FROM project_settings
		WHERE project_id = ?;
	`
	var settings projectSettingsRecord
	row := s.db.QueryRowContext(ctx, query, projectID)
	err := row.Scan(
		&settings.ProjectID,
		&settings.DesignStyle,
		&settings.ColorPalette,
		&settings.BrandGuidelines,
		&settings.ComponentExamples,
		&settings.TechnicalConstraints,
		&settings.LayoutPreferences,
		&settings.ImageGenerationNotes,
		&settings.GenerationContext,
		&settings.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return projectSettingsRecord{
				ProjectID: projectID,
			}, nil
		}
		return projectSettingsRecord{}, err
	}
	return settings, nil
}

func (s *sessionProjectStore) saveProjectSettings(ctx context.Context, projectID string, settings projectSettingsRecord) (projectSettingsRecord, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return projectSettingsRecord{}, errProjectNotFound
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err := s.withWriteRetry(ctx, func() error {
		_, execErr := s.db.ExecContext(
			ctx,
			`INSERT INTO project_settings (
				project_id, design_style, color_palette, brand_guidelines,
				component_examples, technical_constraints, layout_preferences,
				image_generation_notes, generation_context, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(project_id) DO UPDATE SET
				design_style = excluded.design_style,
				color_palette = excluded.color_palette,
				brand_guidelines = excluded.brand_guidelines,
				component_examples = excluded.component_examples,
				technical_constraints = excluded.technical_constraints,
				layout_preferences = excluded.layout_preferences,
				image_generation_notes = excluded.image_generation_notes,
				generation_context = excluded.generation_context,
				updated_at = excluded.updated_at;`,
			projectID,
			strings.TrimSpace(settings.DesignStyle),
			strings.TrimSpace(settings.ColorPalette),
			strings.TrimSpace(settings.BrandGuidelines),
			strings.TrimSpace(settings.ComponentExamples),
			strings.TrimSpace(settings.TechnicalConstraints),
			strings.TrimSpace(settings.LayoutPreferences),
			strings.TrimSpace(settings.ImageGenerationNotes),
			strings.TrimSpace(settings.GenerationContext),
			now,
		)
		return execErr
	})
	if err != nil {
		return projectSettingsRecord{}, err
	}

	return s.getProjectSettings(ctx, projectID)
}
