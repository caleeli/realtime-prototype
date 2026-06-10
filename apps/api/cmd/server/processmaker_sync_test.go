package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncProjectExportToProcessMakerSendsPrototypePayload(t *testing.T) {
	prototype := map[string]any{
		"projectExport": map[string]any{
			"project": map[string]any{
				"id":   "project-1",
				"name": "Demo",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected json content type, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := payload["prototype"].(map[string]any); !ok {
			t.Fatalf("expected prototype object in payload, got %#v", payload["prototype"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	t.Setenv(processMakerAPISyncURLEnv, server.URL)
	t.Setenv(processMakerAPITokenEnv, "test-token")

	status, err := syncProjectExportToProcessMaker(context.Background(), prototype)
	if err != nil {
		t.Fatalf("syncProjectExportToProcessMaker returned error: %v", err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected upstream status %d, got %d", http.StatusCreated, status)
	}
}
