package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSaveStateRejectsStaleBaseRevision(t *testing.T) {
	ctx := context.Background()
	store, err := newSessionProjectStore(filepath.Join(t.TempDir(), "session.sqlite"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	project, err := store.getDefaultProject(ctx)
	if err != nil {
		t.Fatalf("failed to load default project: %v", err)
	}
	screen, err := store.createScreen(ctx, project.ID, "Collaboration test")
	if err != nil {
		t.Fatalf("failed to create screen: %v", err)
	}

	baseRevision := 0
	_, err = store.saveState(ctx, project.ID, screen.ID, saveScreenStateRequest{
		Payload: sessionPayload{
			SourcePug: "div First",
			CSS:       ".first { color: red; }",
			Data:      []byte(`{"value":1}`),
		},
		BaseRevision: &baseRevision,
	})
	if err != nil {
		t.Fatalf("expected initial save to succeed: %v", err)
	}

	_, err = store.saveState(ctx, project.ID, screen.ID, saveScreenStateRequest{
		Payload: sessionPayload{
			SourcePug: "div Stale",
			CSS:       ".stale { color: blue; }",
			Data:      []byte(`{"value":2}`),
		},
		BaseRevision: &baseRevision,
	})
	if !errors.Is(err, errScreenRevisionConflict) {
		t.Fatalf("expected stale save conflict, got %v", err)
	}
}
