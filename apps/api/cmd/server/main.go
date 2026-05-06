package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"crypto/sha256"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/example/realtime-prototype/api/internal/registry"
)

const cerebrasDefaultEndpoint = "https://api.cerebras.ai/v1/chat/completions"
const cerebrasDefaultModel = "llama3.1-8b"
const inspirationImageDefaultOpenAIEndpoint = "https://api.openai.com/v1/images/generations"
const inspirationImageDefaultGoogleEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/imagen-4.0-generate-001:predict"
const inspirationImageDefaultOpenAIModel = "gpt-image-1"
const inspirationImageDefaultGoogleModel = "imagen-4.0-generate-001"
const inspirationVisionDefaultOpenAIEndpoint = "https://api.openai.com/v1/responses"
const inspirationVisionDefaultGoogleEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"
const inspirationVisionDefaultOpenAIModel = "gpt-4.1-mini"
const inspirationVisionDefaultGoogleModel = "gemini-2.5-flash"
const generationSystemPromptTemplatePath = "cmd/server/generation-system-prompt.txt"
const generationSystemPromptTemplateEnv = "GENERATION_SYSTEM_PROMPT_PATH"
const uxEvaluatorSystemPromptTemplatePath = "cmd/server/ux-evaluator.txt"
const uxEvaluatorSystemPromptTemplateEnv = "UX_EVALUATOR_SYSTEM_PROMPT_PATH"
const inspirationConversionPromptTemplatePath = "cmd/server/inspiration-conversion-prompt.txt"
const inspirationConversionPromptTemplateEnv = "INSPIRATION_CONVERSION_PROMPT_PATH"
const generationRepairEnabledEnv = "GENERATION_REPAIR_ENABLED"
const inspirationImageProviderEnv = "INSPIRATION_IMAGE_PROVIDER"
const inspirationVisionProviderEnv = "INSPIRATION_VISION_PROVIDER"
const inspirationOpenAIImageAPIURL = "INSPIRATION_OPENAI_IMAGE_API_URL"
const inspirationOpenAIImageModelEnv = "INSPIRATION_OPENAI_IMAGE_MODEL"
const inspirationOpenAIImageSizeEnv = "INSPIRATION_OPENAI_IMAGE_SIZE"
const inspirationOpenAIImageQualityEnv = "INSPIRATION_OPENAI_IMAGE_QUALITY"
const inspirationOpenAIImageStyleEnv = "INSPIRATION_OPENAI_IMAGE_STYLE"
const inspirationOpenAIImageTimeoutEnv = "INSPIRATION_OPENAI_IMAGE_TIMEOUT_MS"
const inspirationOpenAIImageNEnv = "INSPIRATION_OPENAI_IMAGE_N"
const inspirationOpenAIImageAPIKeyEnv = "INSPIRATION_OPENAI_IMAGE_API_KEY"
const inspirationOpenAIVisionAPIURL = "INSPIRATION_OPENAI_VISION_API_URL"
const inspirationOpenAIVisionModelEnv = "INSPIRATION_OPENAI_VISION_MODEL"
const inspirationOpenAIVisionTimeoutEnv = "INSPIRATION_OPENAI_VISION_TIMEOUT_MS"
const inspirationOpenAIVisionAPIKeyEnv = "INSPIRATION_OPENAI_VISION_API_KEY"
const inspirationGoogleImageAPIURL = "INSPIRATION_GOOGLE_IMAGE_API_URL"
const inspirationGoogleImageModelEnv = "INSPIRATION_GOOGLE_IMAGE_MODEL"
const inspirationGoogleImageSizeEnv = "INSPIRATION_GOOGLE_IMAGE_SIZE"
const inspirationGoogleImageAspectRatioEnv = "INSPIRATION_GOOGLE_IMAGE_ASPECT_RATIO"
const inspirationGoogleImageTimeoutEnv = "INSPIRATION_GOOGLE_IMAGE_TIMEOUT_MS"
const inspirationGoogleImageNEnv = "INSPIRATION_GOOGLE_IMAGE_N"
const inspirationGoogleImageAPIKeyEnv = "INSPIRATION_GOOGLE_IMAGE_API_KEY"
const inspirationGoogleVisionAPIURL = "INSPIRATION_GOOGLE_VISION_API_URL"
const inspirationGoogleVisionModelEnv = "INSPIRATION_GOOGLE_VISION_MODEL"
const inspirationGoogleVisionTimeoutEnv = "INSPIRATION_GOOGLE_VISION_TIMEOUT_MS"
const inspirationGoogleVisionAPIKeyEnv = "INSPIRATION_GOOGLE_VISION_API_KEY"
const inspirationImageCacheEnabledEnv = "INSPIRATION_IMAGE_CACHE_ENABLED"
const inspirationImageCacheDirEnv = "INSPIRATION_IMAGE_CACHE_DIR"
const inspirationImageCacheDefaultDir = "cache/inspiration/images"
const inspirationImageProviderGoogle = "google"
const inspirationImageProviderOpenAI = "openai"

func main() {
	loadEnvFile(".env")

	storagePath := strings.TrimSpace(os.Getenv("COMPONENT_REGISTRY_PATH"))
	if storagePath == "" {
		storagePath = "data/component-registry.json"
	}

	svc := registry.NewRegistryService(storagePath)
	sessionDBPath := strings.TrimSpace(os.Getenv("SCREEN_SESSION_DB_PATH"))
	if sessionDBPath == "" {
		sessionDBPath = defaultSessionDatabasePath
	}
	sessionStore, err := newSessionProjectStore(sessionDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := sessionStore.Close(); closeErr != nil {
			log.Printf("failed to close session store: %v", closeErr)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/component-registry", withCORS(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			enabledOnly := r.URL.Query().Get("enabled") == "true"
			payload := svc.List(enabledOnly)
			writeJSON(w, http.StatusOK, payload)
		case http.MethodPost:
			var payload registry.ComponentRegistrationPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}

			if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Module) == "" || strings.TrimSpace(payload.Tag) == "" || strings.TrimSpace(payload.Pack) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, module, tag and pack are required"})
				return
			}

			created := svc.Register(payload)
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/session", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snapshot, err := sessionStore.getSnapshot(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	}))

	mux.HandleFunc("/api/session/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		subPath := strings.TrimPrefix(r.URL.Path, "/api/session/")

		project, err := sessionStore.getDefaultProject(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		switch {
		case subPath == "theme":
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var payload struct {
				Theme string `json:"theme"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}
			if err := sessionStore.setTheme(r.Context(), project.ID, payload.Theme); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			updated, err := sessionStore.getDefaultProject(r.Context())
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"projectId": updated.ID, "theme": updated.Theme})
			return

		case subPath == "screens":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}
			screen, err := sessionStore.createScreen(r.Context(), project.ID, payload.Name)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, screen)
			return
		}

		if strings.HasPrefix(subPath, "screens/") {
			screenPath := strings.TrimPrefix(subPath, "screens/")
			parts := strings.Split(screenPath, "/")
			screenID := strings.TrimSpace(parts[0])
			if screenID == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if len(parts) == 2 && parts[1] == "activate" {
				if r.Method != http.MethodPatch {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if err := sessionStore.activateScreen(r.Context(), project.ID, screenID); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						writeJSON(w, http.StatusNotFound, map[string]string{"error": "screen not found"})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusNoContent, map[string]string{"status": "ok"})
				return
			}

			if r.Method == http.MethodDelete && len(parts) == 1 {
				if err := sessionStore.deleteScreen(r.Context(), project.ID, screenID); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						writeJSON(w, http.StatusNotFound, map[string]string{"error": "screen not found"})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodPost {
				var payload saveScreenStateRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
					return
				}
				state, err := sessionStore.saveState(r.Context(), project.ID, screenID, payload)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						writeJSON(w, http.StatusNotFound, map[string]string{"error": "screen not found"})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusCreated, state)
				return
			}

			if len(parts) == 3 && parts[1] == "state" && parts[2] == "latest" && r.Method == http.MethodGet {
				state, err := sessionStore.getLatestState(r.Context(), screenID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeJSON(w, http.StatusNotFound, map[string]string{"error": "no state found"})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, state)
				return
			}

			if len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodGet {
				limit := 20
				if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
					value, parseErr := strconv.Atoi(rawLimit)
					if parseErr != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
						return
					}
					limit = value
				}

				items, err := sessionStore.listScreenStates(r.Context(), project.ID, screenID, limit)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						writeJSON(w, http.StatusNotFound, map[string]string{"error": "screen not found"})
						return
					}
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	mux.HandleFunc("/api/component-registry/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/component-registry/")
		if name == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if strings.HasSuffix(name, "/enabled") && r.Method == http.MethodPatch {
			name = strings.TrimSuffix(name, "/enabled")
			name = strings.TrimSuffix(name, "/")
			var body struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}
			component, err := svc.SetEnabled(name, body.Enabled)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "component not found"})
				return
			}
			writeJSON(w, http.StatusOK, component)
			return
		}

		if r.Method == http.MethodPut {
			var payload registry.ComponentRegistrationPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}

			if strings.TrimSpace(payload.Name) == "" {
				payload.Name = strings.TrimSpace(name)
			}

			if strings.TrimSpace(payload.Module) == "" || strings.TrimSpace(payload.Tag) == "" || strings.TrimSpace(payload.Pack) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, module, tag and pack are required"})
				return
			}

			component, err := svc.Update(name, payload)
			if err != nil {
				if errors.Is(err, registry.ErrComponentNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "component not found"})
					return
				}
				if errors.Is(err, registry.ErrDuplicateComponentName) {
					writeJSON(w, http.StatusConflict, map[string]string{"error": "component with this name already exists"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			writeJSON(w, http.StatusOK, component)
			return
		}

		if r.Method == http.MethodDelete {
			if err := svc.Delete(name); err != nil {
				if errors.Is(err, registry.ErrComponentNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "component not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete component"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		component, ok := svc.Get(name)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "component not found"})
			return
		}

		writeJSON(w, http.StatusOK, component)
	}))

	mux.HandleFunc("/api/generation", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var input generationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}
		output, err := callCerebrasGeneration(r.Context(), input)
		if err != nil {
			if errors.Is(err, errMissingCerebrasKey) {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, output)
	}))

	mux.HandleFunc("/api/data-generation", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var input dataGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}
		output, err := callCerebrasDataGeneration(r.Context(), input)
		if err != nil {
			if errors.Is(err, errMissingCerebrasKey) {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, output)
	}))

	mux.HandleFunc("/api/pug-generation", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var input pugGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}
		output, err := callCerebrasPugGeneration(r.Context(), input)
		if err != nil {
			if errors.Is(err, errMissingCerebrasKey) {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, output)
	}))

	inspirationHandler := withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var input inspirationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}

		output, err := callImageInspiration(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, output)
	})
	mux.HandleFunc("/api/inspiration", inspirationHandler)
	mux.HandleFunc("/api/inspiracion", inspirationHandler)

	mux.HandleFunc("/api/ux-evaluator", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var input uxEvaluatorRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}

		output, err := callCerebrasUXEvaluator(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writePlainText(w, http.StatusOK, output)
	}))

	addr := strings.TrimSpace(os.Getenv("PORT"))
	if addr == "" {
		addr = ":3000"
	}
	if !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("component registry API listening on %s", addr)
	log.Printf("generation service listening on %s", addr)
	log.Printf("inspiration service listening on %s", addr)
	if err := http.ListenAndServe(addr, muxWithCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

func writePlainText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(text)); err != nil {
		log.Printf("failed to write text response: %v", err)
	}
}

func applyCORSHeaders(w http.ResponseWriter) {
	allowedOrigin := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN"))
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		applyCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handler(w, r)
	}
}

func muxWithCORS(base *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyCORSHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		base.ServeHTTP(w, r)
	})
}

type generationContext struct {
	Locale        string   `json:"locale"`
	Theme         string   `json:"theme"`
	EnabledPacks  []string `json:"enabledPacks"`
	TargetDensity string   `json:"targetDensity"`
}

type generationRequest struct {
	Prompt   string                `json:"prompt"`
	Context  *generationContext    `json:"context"`
	Messages []cerebrasChatMessage `json:"messages"`
}

type dataGenerationRequest struct {
	Prompt    string                `json:"prompt"`
	Context   *generationContext    `json:"context"`
	CurrentPug string               `json:"currentPug"`
	CurrentData interface{}         `json:"currentData"`
	Messages  []cerebrasChatMessage `json:"messages"`
}

type pugGenerationRequest struct {
	Prompt      string                `json:"prompt"`
	Context     *generationContext    `json:"context"`
	CurrentPug  string                `json:"currentPug"`
	CurrentCss  string                `json:"currentCss"`
	CurrentData interface{}           `json:"currentData"`
	Messages    []cerebrasChatMessage `json:"messages"`
}

type inspirationRequest struct {
	Prompt          string                `json:"prompt"`
	Context         *generationContext    `json:"context"`
	ImagePrompt     string                `json:"imagePrompt"`
	ImageModel      string                `json:"imageModel"`
	ImageSize       string                `json:"imageSize"`
	ImageQuality    string                `json:"imageQuality"`
	ImageStyle      string                `json:"imageStyle"`
	VisionModel     string                `json:"visionModel"`
	Messages        []cerebrasChatMessage `json:"messages"`
	ConversionNotes string                `json:"conversionNotes"`
}

type generationResponse struct {
	Pug  string      `json:"pug"`
	Css  string      `json:"css"`
	Data interface{} `json:"data"`
	// Messages represent the conversation after this turn, excluding system messages.
	Messages []cerebrasChatMessage `json:"messages,omitempty"`
}

type dataGenerationResponse struct {
	Data     interface{}           `json:"data"`
	Messages []cerebrasChatMessage  `json:"messages,omitempty"`
}

type pugGenerationResponse struct {
	Pug      string                `json:"pug"`
	Messages []cerebrasChatMessage  `json:"messages,omitempty"`
}

type uxEvaluatorRequest struct {
	Pug  string      `json:"pug"`
	Css  string      `json:"css"`
	Data interface{} `json:"data"`
}

type cerebrasChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cerebrasChatPayload struct {
	Model          string                  `json:"model"`
	Messages       []cerebrasChatMessage   `json:"messages"`
	ResponseFormat *cerebrasResponseFormat `json:"response_format,omitempty"`
}

type cerebrasResponseFormat struct {
	Type string `json:"type"`
}

type cerebrasChatResponse struct {
	Usage struct {
		PromptTokens       int `json:"prompt_tokens"`
		CompletionTokens   int `json:"completion_tokens"`
		TotalTokens        int `json:"total_tokens"`
		PromptTokensDetail struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage,omitempty"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

var errMissingCerebrasKey = errors.New("missing CEREBRAS_API_KEY env variable")

func callCerebrasGeneration(ctx context.Context, input generationRequest) (generationResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	if apiKey == "" {
		return generationResponse{}, errMissingCerebrasKey
	}

	endpoint := strings.TrimSpace(os.Getenv("CEREBRAS_API_URL"))
	if endpoint == "" {
		endpoint = cerebrasDefaultEndpoint
	}
	model := strings.TrimSpace(os.Getenv("CEREBRAS_MODEL"))
	if model == "" {
		model = cerebrasDefaultModel
	}

	timeoutMs := parseIntFromEnv("CEREBRAS_TIMEOUT_MS", 20000)
	clientTimeout := time.Duration(timeoutMs) * time.Millisecond
	messages := buildCerebrasRequestMessages(input)

	maxAttempts := 1
	repairEnabled := parseBoolFromEnv(generationRepairEnabledEnv, false)
	if repairEnabled {
		maxAttempts = 2
	}

	var output generationResponse
	var outputContent string
	var jsonCandidate string
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		jsonCandidate = ""
		content, err := requestCerebrasContent(
			ctx,
			endpoint,
			model,
			apiKey,
			clientTimeout,
			messages,
			&cerebrasResponseFormat{
				Type: "json_object",
			},
		)
		if err != nil {
			return generationResponse{}, err
		}
		outputContent = content

		jsonCandidate, err = extractJSONFromText(outputContent)
		if err != nil {
			lastErr = fmt.Errorf("model output is not JSON: %w", err)
		} else {
			jsonCandidate = normalizeGeneratedJSONCandidate(jsonCandidate)
			lastErr = parseGenerationJSONCandidate(jsonCandidate, &output)
		}

		if lastErr == nil {
			output = sanitizeGenerationResponse(output)
			if looksLikeHTML(output.Pug) {
				converted, err := convertHTMLToPug(output.Pug)
				if err != nil {
					log.Printf("warning: failed to parse model HTML output in pug field: %v", err)
				} else if strings.TrimSpace(converted) != "" {
					log.Printf("warning: model returned HTML in pug field; converted to pseudo-Pug")
					output.Pug = strings.TrimSpace(converted)
				}
			}
			if err := validateGenerationResponse(&output); err != nil {
				lastErr = fmt.Errorf("invalid generated response: %w", err)
			} else {
				output.Messages = appendConversationMessagesForResponse(messages, outputContent)
				return output, nil
			}
		}

		if !repairEnabled || attempt+1 >= maxAttempts || !isRecoverableGeneratedJSONError(lastErr) {
			return generationResponse{}, fmt.Errorf("%w\n%s", lastErr, jsonCandidate)
		}

		messages = append(messages, cerebrasChatMessage{
			Role:    "assistant",
			Content: outputContent,
		}, cerebrasChatMessage{
			Role:    "user",
			Content: buildGenerationRepairPrompt(outputContent, jsonCandidate, lastErr),
		})
	}

	return generationResponse{}, fmt.Errorf("%w\n%s", lastErr, outputContent)
}

func callCerebrasDataGeneration(ctx context.Context, input dataGenerationRequest) (dataGenerationResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	if apiKey == "" {
		return dataGenerationResponse{}, errMissingCerebrasKey
	}

	endpoint := strings.TrimSpace(os.Getenv("CEREBRAS_API_URL"))
	if endpoint == "" {
		endpoint = cerebrasDefaultEndpoint
	}
	model := strings.TrimSpace(os.Getenv("CEREBRAS_MODEL"))
	if model == "" {
		model = cerebrasDefaultModel
	}

	timeoutMs := parseIntFromEnv("CEREBRAS_TIMEOUT_MS", 20000)
	clientTimeout := time.Duration(timeoutMs) * time.Millisecond
	messages := buildCerebrasDataGenerationMessages(input)

	maxAttempts := 1
	repairEnabled := parseBoolFromEnv(generationRepairEnabledEnv, false)
	if repairEnabled {
		maxAttempts = 2
	}

	var output dataGenerationResponse
	var outputContent string
	var jsonCandidate string
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		jsonCandidate = ""
		content, err := requestCerebrasContent(
			ctx,
			endpoint,
			model,
			apiKey,
			clientTimeout,
			messages,
			&cerebrasResponseFormat{
				Type: "json_object",
			},
		)
		if err != nil {
			return dataGenerationResponse{}, err
		}
		outputContent = content

		jsonCandidate, err = extractJSONFromText(outputContent)
		if err != nil {
			lastErr = fmt.Errorf("model output is not JSON: %w", err)
		} else {
			jsonCandidate = normalizeGeneratedJSONCandidate(jsonCandidate)
			lastErr = parseDataGenerationJSONCandidate(jsonCandidate, &output)
		}

		if lastErr == nil {
			output.Messages = appendConversationMessagesForResponse(messages, outputContent)
			output = sanitizeDataGenerationResponse(output)
			return output, nil
		}

		if !repairEnabled || attempt+1 >= maxAttempts || !isRecoverableGeneratedJSONError(lastErr) {
			return dataGenerationResponse{}, fmt.Errorf("%w\n%s", lastErr, jsonCandidate)
		}

		messages = append(messages, cerebrasChatMessage{
			Role:    "assistant",
			Content: outputContent,
		}, cerebrasChatMessage{
			Role:    "user",
			Content: buildDataGenerationRepairPrompt(outputContent, jsonCandidate, lastErr),
		})
	}

	return dataGenerationResponse{}, fmt.Errorf("%w\n%s", lastErr, outputContent)
}

func callCerebrasPugGeneration(ctx context.Context, input pugGenerationRequest) (pugGenerationResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	if apiKey == "" {
		return pugGenerationResponse{}, errMissingCerebrasKey
	}

	endpoint := strings.TrimSpace(os.Getenv("CEREBRAS_API_URL"))
	if endpoint == "" {
		endpoint = cerebrasDefaultEndpoint
	}
	model := strings.TrimSpace(os.Getenv("CEREBRAS_MODEL"))
	if model == "" {
		model = cerebrasDefaultModel
	}

	timeoutMs := parseIntFromEnv("CEREBRAS_TIMEOUT_MS", 20000)
	clientTimeout := time.Duration(timeoutMs) * time.Millisecond
	messages := buildCerebrasPugGenerationMessages(input)

	maxAttempts := 1
	repairEnabled := parseBoolFromEnv(generationRepairEnabledEnv, false)
	if repairEnabled {
		maxAttempts = 2
	}

	var output pugGenerationResponse
	var outputContent string
	var jsonCandidate string
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		jsonCandidate = ""
		content, err := requestCerebrasContent(
			ctx,
			endpoint,
			model,
			apiKey,
			clientTimeout,
			messages,
			&cerebrasResponseFormat{
				Type: "json_object",
			},
		)
		if err != nil {
			return pugGenerationResponse{}, err
		}
		outputContent = content

		jsonCandidate, err = extractJSONFromText(outputContent)
		if err != nil {
			lastErr = fmt.Errorf("model output is not JSON: %w", err)
		} else {
			jsonCandidate = normalizeGeneratedJSONCandidate(jsonCandidate)
			lastErr = parsePugGenerationJSONCandidate(jsonCandidate, &output)
		}

		if lastErr == nil {
			output.Messages = appendConversationMessagesForResponse(messages, outputContent)
			output = sanitizePugGenerationResponse(output)
			return output, nil
		}

		if !repairEnabled || attempt+1 >= maxAttempts || !isRecoverableGeneratedJSONError(lastErr) {
			return pugGenerationResponse{}, fmt.Errorf("%w\n%s", lastErr, jsonCandidate)
		}

		messages = append(messages, cerebrasChatMessage{
			Role:    "assistant",
			Content: outputContent,
		}, cerebrasChatMessage{
			Role:    "user",
			Content: buildPugGenerationRepairPrompt(outputContent, jsonCandidate, lastErr),
		})
	}

	return pugGenerationResponse{}, fmt.Errorf("%w\n%s", lastErr, outputContent)
}

func callImageInspiration(ctx context.Context, input inspirationRequest) (generationResponse, error) {
	imagePrompt := strings.TrimSpace(input.ImagePrompt)
	if imagePrompt == "" {
		imagePrompt = strings.TrimSpace(input.Prompt)
	}
	imagePrompt = strings.TrimSpace(imagePrompt)
	if imagePrompt == "" {
		return generationResponse{}, errors.New("prompt is required")
	}

	imageProvider := normalizeInspirationProvider(
		strings.TrimSpace(os.Getenv(inspirationImageProviderEnv)),
		"",
		strings.TrimSpace(input.ImageModel),
	)

	imageTimeoutMs := parseIntFromEnv(
		inspirationImageProviderEnvVar(imageProvider, "TIMEOUT_MS"),
		30000,
	)
	imageTimeout := time.Duration(imageTimeoutMs) * time.Millisecond
	imageEndpoint := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "API_URL")))
	if imageEndpoint == "" {
		imageEndpoint = inspirationImageDefaultEndpoint(imageProvider)
	}

	imageModel := strings.TrimSpace(input.ImageModel)
	if imageModel == "" {
		imageModel = strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "MODEL")))
	}
	if imageModel == "" {
		imageModel = inspirationImageDefaultModel(imageProvider)
	}

	imageProvider = normalizeInspirationProvider(
		strings.TrimSpace(os.Getenv(inspirationImageProviderEnv)),
		imageEndpoint,
		imageModel,
	)

	imageSize := strings.TrimSpace(input.ImageSize)
	if imageSize == "" {
		imageSize = strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "ASPECT_RATIO")))
	}
	if imageSize == "" {
		imageSize = strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "SIZE")))
	}
	if imageSize == "" {
		imageSize = "1024x1024"
	}

	imageQuality := strings.TrimSpace(input.ImageQuality)
	if imageQuality == "" {
		imageQuality = strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "QUALITY")))
	}
	if imageQuality == "" {
		imageQuality = ""
	}
	imageStyle := strings.TrimSpace(input.ImageStyle)
	if imageStyle == "" {
		imageStyle = strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "STYLE")))
	}
	if imageStyle == "" {
		imageStyle = ""
	}

	imageCount := parseIntFromEnv(
		inspirationImageProviderEnvVar(imageProvider, "N"),
		1,
	)
	if imageCount < 1 {
		imageCount = 1
	}
	if imageCount > 4 {
		imageCount = 4
	}

	imageCacheEnabled := parseBoolFromEnv(inspirationImageCacheEnabledEnv, true)
	imageCacheDir := strings.TrimSpace(os.Getenv(inspirationImageCacheDirEnv))
	if imageCacheDir == "" {
		imageCacheDir = inspirationImageCacheDefaultDir
	}
	imageCacheKey := inspirationImageCacheKey(
		imageEndpoint,
		imageModel,
		imageProvider,
		imagePrompt,
		imageSize,
		imageQuality,
		imageStyle,
		fmt.Sprint(imageCount),
	)
	imageCachePath := inspirationImageCachePath(imageCacheDir, imageCacheKey)

	imageKey := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(imageProvider, "API_KEY")))
	if imageKey == "" {
		imageKey = strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	}
	if imageKey == "" {
		return generationResponse{}, fmt.Errorf("missing image API key for %s provider (set %s)", imageProvider, inspirationImageProviderEnvVar(imageProvider, "API_KEY"))
	}

	var imageBase64 string
	var err error
	if imageCacheEnabled {
		cachedImageBase64, cacheErr := loadImageInspirationCache(imageCachePath)
		if cacheErr != nil {
			log.Printf("inspiration image cache read error for %s: %v", imageCachePath, cacheErr)
		} else if cachedImageBase64 != "" {
			imageBase64 = cachedImageBase64
		}
	}

	if imageBase64 == "" {
		imageBase64, err = callImageGeneration(
			ctx,
			imageEndpoint,
			imageModel,
			imageProvider,
			imageKey,
			imageTimeout,
			imagePrompt,
			imageSize,
			imageQuality,
			imageStyle,
			imageCount,
		)
		if err != nil {
			return generationResponse{}, err
		}
		if imageCacheEnabled {
			if cacheErr := saveImageInspirationCache(imageCachePath, imageBase64); cacheErr != nil {
				log.Printf("inspiration image cache write error for %s: %v", imageCachePath, cacheErr)
			}
		}
	} else {
		log.Printf("inspiration image cache hit: %s", imageCachePath)
	}

	visionProvider := normalizeInspirationProvider(
		strings.TrimSpace(os.Getenv(inspirationVisionProviderEnv)),
		"",
		strings.TrimSpace(input.VisionModel),
	)

	visionTimeoutMs := parseIntFromEnv(
		inspirationVisionProviderEnvVar(visionProvider, "TIMEOUT_MS"),
		30000,
	)
	visionTimeout := time.Duration(visionTimeoutMs) * time.Millisecond
	visionEndpoint := strings.TrimSpace(os.Getenv(inspirationVisionProviderEnvVar(visionProvider, "API_URL")))
	if visionEndpoint == "" {
		visionEndpoint = inspirationVisionDefaultEndpoint(visionProvider)
	}

	visionModel := strings.TrimSpace(input.VisionModel)
	if visionModel == "" {
		visionModel = strings.TrimSpace(os.Getenv(inspirationVisionProviderEnvVar(visionProvider, "MODEL")))
	}
	if visionModel == "" {
		visionModel = inspirationVisionDefaultModel(visionProvider)
	}

	visionProvider = normalizeInspirationProvider(
		strings.TrimSpace(os.Getenv(inspirationVisionProviderEnv)),
		visionEndpoint,
		visionModel,
	)

	visionKey := strings.TrimSpace(os.Getenv(inspirationVisionProviderEnvVar(visionProvider, "API_KEY")))
	if visionKey == "" {
		visionKey = strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	}
	if visionKey == "" {
		return generationResponse{}, fmt.Errorf("missing vision API key for %s provider (set %s)", visionProvider, inspirationVisionProviderEnvVar(visionProvider, "API_KEY"))
	}

	conversionPrompt := strings.TrimSpace(input.ConversionNotes)
	if conversionPrompt == "" {
		conversionPrompt = buildInspirationConversionPrompt(input.Context)
	}

	visionContent, err := callVisionToCode(
		ctx,
		visionEndpoint,
		visionModel,
		visionProvider,
		visionKey,
		visionTimeout,
		conversionPrompt,
		imageBase64,
		imagePrompt,
	)
	if err != nil {
		return generationResponse{}, err
	}

	var output generationResponse
	jsonCandidate, err := extractJSONFromText(visionContent)
	if err != nil {
		return generationResponse{}, fmt.Errorf("vision model output is not JSON: %w", err)
	}
	decodedCandidate := normalizeGeneratedJSONCandidate(jsonCandidate)
	if err := parseGenerationJSONCandidate(decodedCandidate, &output); err != nil {
		return generationResponse{}, err
	}

	output = sanitizeGenerationResponse(output)
	if err := validateGenerationResponse(&output); err != nil {
		return generationResponse{}, fmt.Errorf("invalid generated output: %w", err)
	}
	output.Messages = appendConversationMessagesForResponse(
		buildCerebrasRequestMessages(generationRequest{
			Prompt:   strings.TrimSpace(input.Prompt),
			Context:  input.Context,
			Messages: input.Messages,
		}),
		visionContent,
	)
	return output, nil
}

type imageGenerationPayload struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size"`
	N              int    `json:"n"`
	Quality        string `json:"quality,omitempty"`
	Style          string `json:"style,omitempty"`
}

type googleImageGenerationPayload struct {
	Instances  []googleImageInstance `json:"instances"`
	Parameters *googleImageParameters `json:"parameters,omitempty"`
}

type googleImageInstance struct {
	Prompt string `json:"prompt"`
}

type googleImageParameters struct {
	SampleCount int    `json:"sampleCount,omitempty"`
	Seed        int    `json:"seed,omitempty"`
	AspectRatio string `json:"aspectRatio,omitempty"`
}

type imageGenerationResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
}

type googleImageGenerationResponse struct {
	Predictions []struct {
		B64JSON string `json:"b64_json"`
		Base64  string `json:"base64"`
		Bytes   string `json:"bytesBase64Encoded"`
		URL     string `json:"url"`
		URI     string `json:"uri"`
		GcsURI  string `json:"gcsUri"`
	} `json:"predictions"`
	Candidates []struct {
		B64JSON string `json:"b64_json"`
		Base64  string `json:"base64"`
		Bytes   string `json:"bytesBase64Encoded"`
		URL     string `json:"url"`
		URI     string `json:"uri"`
		GcsURI  string `json:"gcsUri"`
	} `json:"candidates"`
}

func callImageGeneration(
	ctx context.Context,
	endpoint string,
	model string,
	provider string,
	apiKey string,
	timeout time.Duration,
	prompt string,
	size string,
	quality string,
	style string,
	n int,
) (string, error) {
	switch provider {
	case inspirationImageProviderGoogle:
		return callImageGenerationGoogle(ctx, endpoint, model, apiKey, timeout, prompt, size, n)
	default:
		return callImageGenerationOpenAI(ctx, endpoint, model, apiKey, timeout, prompt, size, quality, style, n)
	}
}

func callImageGenerationOpenAI(
	ctx context.Context,
	endpoint string,
	model string,
	apiKey string,
	timeout time.Duration,
	prompt string,
	size string,
	quality string,
	style string,
	n int,
) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	normalizedQuality := normalizeImageQuality(model, quality)
	normalizedStyle := normalizeImageStyle(model, style)

	payload := imageGenerationPayload{
		Model:          model,
		Prompt:         prompt,
		Size:           size,
		N:              n,
		Quality:        normalizedQuality,
		Style:          normalizedStyle,
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to build image request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	applyProviderAuth(httpRequest, inspirationImageProviderOpenAI, apiKey)

	httpClient := &http.Client{Timeout: timeout}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("image generation request timeout")
		}
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(body))
		if errorMessage == "" {
			errorMessage = http.StatusText(response.StatusCode)
		}
		return "", fmt.Errorf("image API returned %d: %s", response.StatusCode, errorMessage)
	}

	var parsed imageGenerationResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("invalid image response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return "", fmt.Errorf("image API returned empty data set")
	}
	if parsed.Data[0].B64JSON != "" {
		return parsed.Data[0].B64JSON, nil
	}

	if parsed.Data[0].URL != "" {
		return fetchImageAsBase64(ctx, parsed.Data[0].URL, timeout)
	}

	return "", fmt.Errorf("image API did not return base64 or url content")
}

func callImageGenerationGoogle(
	ctx context.Context,
	endpoint string,
	model string,
	apiKey string,
	timeout time.Duration,
	prompt string,
	size string,
	n int,
) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	requestEndpoint, err := appendGoogleAPIKey(endpoint, apiKey)
	if err != nil {
		return "", err
	}

	aspectRatio := mapImageSizeToAspectRatio(size)
	requestBody, err := json.Marshal(googleImageGenerationPayload{
		Instances: []googleImageInstance{
			{
				Prompt: prompt,
			},
		},
		Parameters: &googleImageParameters{
			SampleCount: n,
			AspectRatio: aspectRatio,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to build google image request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, requestEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	applyProviderAuth(httpRequest, inspirationImageProviderGoogle, apiKey)

	httpClient := &http.Client{Timeout: timeout}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("google image generation request timeout")
		}
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(body))
		if errorMessage == "" {
			errorMessage = http.StatusText(response.StatusCode)
		}
		if response.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf(
				"google image API returned 404: %s. Verify model and method for model in endpoint. Current config usually expects imagen-4.0-generate-001 with .../v1beta/models/{model}:predict",
				errorMessage,
			)
		}
		return "", fmt.Errorf("google image API returned %d: %s", response.StatusCode, errorMessage)
	}

	var parsed googleImageGenerationResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("invalid google image response: %w", err)
	}

	var base64Image string
	for _, prediction := range parsed.Predictions {
		base64Image = pickFirstImageContent(prediction.B64JSON, prediction.Base64, prediction.Bytes, prediction.URL, prediction.URI)
		if base64Image != "" {
			break
		}
	}
	if base64Image != "" {
		return base64Image, nil
	}

	for _, candidate := range parsed.Candidates {
		base64Image = pickFirstImageContent(candidate.B64JSON, candidate.Base64, candidate.Bytes, candidate.URL, candidate.URI)
		if base64Image != "" {
			break
		}
	}
	if base64Image != "" {
		return base64Image, nil
	}

	// Fallback for providers returning URL-style payload with predictions.
	for _, prediction := range parsed.Predictions {
		if strings.TrimSpace(prediction.URL) != "" {
			return fetchImageAsBase64(ctx, prediction.URL, timeout)
		}
		if strings.TrimSpace(prediction.URI) != "" {
			return fetchImageAsBase64(ctx, prediction.URI, timeout)
		}
		if strings.TrimSpace(prediction.GcsURI) != "" {
			return fetchImageAsBase64(ctx, prediction.GcsURI, timeout)
		}
	}
	for _, candidate := range parsed.Candidates {
		if strings.TrimSpace(candidate.URL) != "" {
			return fetchImageAsBase64(ctx, candidate.URL, timeout)
		}
		if strings.TrimSpace(candidate.URI) != "" {
			return fetchImageAsBase64(ctx, candidate.URI, timeout)
		}
		if strings.TrimSpace(candidate.GcsURI) != "" {
			return fetchImageAsBase64(ctx, candidate.GcsURI, timeout)
		}
	}

	return "", fmt.Errorf("google image API did not return image content")
}

func pickFirstImageContent(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func inspirationImageCacheKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func inspirationImageCachePath(cacheDir string, key string) string {
	return filepath.Join(strings.TrimSpace(cacheDir), key+".png")
}

func loadImageInspirationCache(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			txtPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".txt"
			textData, txtErr := os.ReadFile(txtPath)
			if txtErr == nil {
				return strings.TrimSpace(string(textData)), nil
			}
			if os.IsNotExist(txtErr) {
				return "", nil
			}
			return "", txtErr
		}
		return "", err
	}

	if strings.HasPrefix(string(data), "data:") {
		if commaIndex := strings.Index(string(data), ","); commaIndex >= 0 && commaIndex < len(data) {
			return strings.TrimSpace(string(data[commaIndex+1:])), nil
		}
		return strings.TrimSpace(string(data)), nil
	}

	if isBase64ImageData(string(data)) {
		return strings.TrimSpace(string(data)), nil
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func saveImageInspirationCache(path string, imageBase64 string) error {
	cacheDir := filepath.Dir(path)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	imageData := strings.TrimSpace(imageBase64)
	rawData, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, rawData, 0o644)
}

func isBase64ImageData(value string) bool {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return false
	}
	if strings.ContainsRune(clean, '\n') || strings.ContainsRune(clean, '\r') {
		return false
	}
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") || strings.HasPrefix(clean, "data:") {
		return false
	}
	for _, char := range clean {
		if (char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '+' ||
			char == '/' ||
			char == '=' {
			continue
		}
		return false
	}
	return true
}

func normalizeImageQuality(model string, quality string) string {
	modelName := strings.ToLower(strings.TrimSpace(model))
	requested := strings.ToLower(strings.TrimSpace(quality))
	if requested == "" {
		return ""
	}

	if strings.Contains(modelName, "gpt-image-1") {
		switch requested {
		case "low", "medium", "high", "auto":
			return requested
		case "standard", "hd":
			return "medium"
		default:
			return ""
		}
	}

	return requested
}

func normalizeImageStyle(model string, style string) string {
	modelName := strings.ToLower(strings.TrimSpace(model))
	requested := strings.TrimSpace(style)
	if requested == "" {
		return ""
	}

	if strings.Contains(modelName, "gpt-image-1") {
		return ""
	}

	return requested
}

func inspirationImageProviderEnvVar(provider string, suffix string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	switch providerName {
	case inspirationImageProviderOpenAI:
		return "INSPIRATION_OPENAI_IMAGE_" + strings.TrimSpace(suffix)
	case inspirationImageProviderGoogle:
		return "INSPIRATION_GOOGLE_IMAGE_" + strings.TrimSpace(suffix)
	default:
		return "INSPIRATION_OPENAI_IMAGE_" + strings.TrimSpace(suffix)
	}
}

func inspirationVisionProviderEnvVar(provider string, suffix string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	switch providerName {
	case inspirationImageProviderOpenAI:
		return "INSPIRATION_OPENAI_VISION_" + strings.TrimSpace(suffix)
	case inspirationImageProviderGoogle:
		return "INSPIRATION_GOOGLE_VISION_" + strings.TrimSpace(suffix)
	default:
		return "INSPIRATION_OPENAI_VISION_" + strings.TrimSpace(suffix)
	}
}

func inspirationImageDefaultEndpoint(provider string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == inspirationImageProviderGoogle {
		return inspirationImageDefaultGoogleEndpoint
	}
	return inspirationImageDefaultOpenAIEndpoint
}

func inspirationImageDefaultModel(provider string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == inspirationImageProviderGoogle {
		return inspirationImageDefaultGoogleModel
	}
	return inspirationImageDefaultOpenAIModel
}

func inspirationVisionDefaultEndpoint(provider string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == inspirationImageProviderGoogle {
		return inspirationVisionDefaultGoogleEndpoint
	}
	return inspirationVisionDefaultOpenAIEndpoint
}

func inspirationVisionDefaultModel(provider string) string {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == inspirationImageProviderGoogle {
		return inspirationVisionDefaultGoogleModel
	}
	return inspirationVisionDefaultOpenAIModel
}

func normalizeInspirationProvider(rawProvider string, endpoint string, model string) string {
	provider := strings.ToLower(strings.TrimSpace(rawProvider))
	if provider == inspirationImageProviderGoogle || provider == inspirationImageProviderOpenAI {
		return provider
	}

	lowerEndpoint := strings.ToLower(strings.TrimSpace(endpoint))
	if strings.Contains(lowerEndpoint, "googleapis") || strings.Contains(lowerEndpoint, "generativelanguage") {
		return inspirationImageProviderGoogle
	}

	lowerModel := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(lowerModel, "gemini") || strings.Contains(lowerModel, "imagen") {
		return inspirationImageProviderGoogle
	}

	return inspirationImageProviderOpenAI
}

func mapImageSizeToAspectRatio(size string) string {
	switch strings.TrimSpace(size) {
	case "1024x1792", "768x1024", "1024x1536", "9:16":
		return "9:16"
	case "1792x1024", "1536x1024", "1365x768", "16:10", "4:3", "16:9":
		return "16:9"
	case "1024x1024", "1080x1080", "1:1":
		return "1:1"
	case "1365x1024", "1024x1365":
		return "4:3"
	default:
		return "1:1"
	}
}

func appendGoogleAPIKey(endpoint string, apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return endpoint, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	if query.Get("key") == "" {
		query.Set("key", apiKey)
		parsed.RawQuery = query.Encode()
	}

	return parsed.String(), nil
}

func applyProviderAuth(request *http.Request, provider string, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	switch provider {
	case inspirationImageProviderGoogle:
		request.Header.Set("x-goog-api-key", apiKey)
	case inspirationImageProviderOpenAI:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	default:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func fetchImageAsBase64(ctx context.Context, imageURL string, timeout time.Duration) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{Timeout: timeout}
	response, err := httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("download image timeout")
		}
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("failed to download image from provider: %s", http.StatusText(response.StatusCode))
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

type visionInput struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type visionPrompt struct {
	Role    string       `json:"role"`
	Content []visionInput `json:"content"`
}

type responsesRequest struct {
	Model  string        `json:"model"`
	Input  []visionPrompt `json:"input"`
}

type geminiPart struct {
	Text string          `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiContent struct {
	Role  string      `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiVisionRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiCandidate struct {
	Content struct {
		Parts []geminiPart `json:"parts"`
	} `json:"content"`
}

type geminiVisionResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type visionResponse struct {
	OutputText string                  `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callVisionToCode(
	ctx context.Context,
	endpoint string,
	model string,
	provider string,
	apiKey string,
	timeout time.Duration,
	conversionPrompt string,
	imageBase64 string,
	imagePrompt string,
) (string, error) {
	switch provider {
	case inspirationImageProviderGoogle:
		return callVisionToCodeGoogle(ctx, endpoint, model, apiKey, timeout, conversionPrompt, imageBase64, imagePrompt)
	default:
		return callVisionToCodeOpenAI(ctx, endpoint, model, apiKey, timeout, conversionPrompt, imageBase64, imagePrompt)
	}
}

func callVisionToCodeOpenAI(
	ctx context.Context,
	endpoint string,
	model string,
	apiKey string,
	timeout time.Duration,
	conversionPrompt string,
	imageBase64 string,
	imagePrompt string,
) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	input := []visionPrompt{
		{
			Role: "user",
			Content: []visionInput{
				{
					Type: "input_text",
					Text: buildVisionPromptForImage(conversionPrompt, imagePrompt),
				},
				{
					Type:    "input_image",
					ImageURL: "data:image/png;base64," + imageBase64,
				},
			},
		},
	}

	requestBody, err := json.Marshal(responsesRequest{
		Model: model,
		Input: input,
	})
	if err != nil {
		return "", fmt.Errorf("failed to build vision request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	applyProviderAuth(httpRequest, inspirationImageProviderOpenAI, apiKey)

	httpClient := &http.Client{Timeout: timeout}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("vision conversion request timeout")
		}
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(body))
		if errorMessage == "" {
			errorMessage = http.StatusText(response.StatusCode)
		}
		return "", fmt.Errorf("vision API returned %d: %s", response.StatusCode, errorMessage)
	}

	var parsed visionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("invalid vision response: %w", err)
	}

	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText), nil
	}

	for _, output := range parsed.Output {
		for _, content := range output.Content {
			if strings.TrimSpace(content.Text) != "" {
				return strings.TrimSpace(content.Text), nil
			}
		}
	}

	if len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Content) != "" {
		return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("vision API response has no textual output")
}

func callVisionToCodeGoogle(
	ctx context.Context,
	endpoint string,
	model string,
	apiKey string,
	timeout time.Duration,
	conversionPrompt string,
	imageBase64 string,
	imagePrompt string,
) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	requestEndpoint, err := appendGoogleAPIKey(endpoint, apiKey)
	if err != nil {
		return "", err
	}

	requestBody, err := json.Marshal(geminiVisionRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: buildVisionPromptForImage(conversionPrompt, imagePrompt)},
					{
						InlineData: &geminiInlineData{
							MimeType: "image/png",
							Data:     imageBase64,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to build google vision request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, requestEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	applyProviderAuth(httpRequest, inspirationImageProviderGoogle, apiKey)

	httpClient := &http.Client{Timeout: timeout}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("google vision conversion request timeout")
		}
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(body))
		if errorMessage == "" {
			errorMessage = http.StatusText(response.StatusCode)
		}
		return "", fmt.Errorf("google vision API returned %d: %s", response.StatusCode, errorMessage)
	}

	var parsed geminiVisionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("invalid google vision response: %w", err)
	}

	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return strings.TrimSpace(part.Text), nil
			}
		}
	}

	return "", fmt.Errorf("google vision response has no textual output")
}

func buildVisionPromptForImage(basePrompt string, imagePrompt string) string {
	if basePrompt == "" {
		basePrompt = "No extra instructions."
	}
	if imagePrompt != "" {
		return basePrompt + "\n\nImage prompt used during generation:\n" + imagePrompt
	}
	return basePrompt
}

func buildInspirationConversionPrompt(context *generationContext) string {
	template := loadPromptTemplateFromEnv(
		inspirationConversionPromptTemplatePath,
		inspirationConversionPromptTemplateEnv,
		defaultInspirationConversionPromptTemplate(),
	)

	contextLines := make([]string, 0, 6)
	if context != nil {
		if strings.TrimSpace(context.Locale) != "" {
			contextLines = append(contextLines, "Locale: "+strings.TrimSpace(context.Locale)+".")
		}
		if strings.TrimSpace(context.Theme) != "" {
			contextLines = append(contextLines, "Theme: "+strings.TrimSpace(context.Theme)+".")
		}
		if strings.TrimSpace(context.TargetDensity) != "" {
			contextLines = append(contextLines, "Target density: "+strings.TrimSpace(context.TargetDensity)+".")
		}
		if len(context.EnabledPacks) > 0 {
			contextLines = append(contextLines, "Enabled packs: "+strings.Join(context.EnabledPacks, ", ")+".")
		}
	}
	contextText := strings.TrimSpace(strings.Join(contextLines, "\n"))
	if contextText == "" {
		contextText = "No additional context constraints."
	}

	template = strings.ReplaceAll(template, "{{context}}", contextText)
	return template
}

func defaultInspirationConversionPromptTemplate() string {
	return `You are a Vue screen generator. Return ONLY a valid JSON object with EXACT keys: pug, css, data.
No markdown, no code fences, no explanations, no extra keys.
The output must match this contract.
Contract example:
{
  "pug": "div.screen",
  "css": ".screen { width: 100%; }",
  "data": { "title": "Example", "subtitle": "Example" }
}
Output constraints:
- "pug": bootstrap-vue-first, no html wrappers like <html>, <head>, <body>, no <img> tags.
- "css": plain CSS only, no preprocessors.
- "data": object used by the generated template with realistic defaults.
Prefer semantic class names and avoid inventing unknown components.
If needed, use Vue bindings (v-model, :prop, @event) and reference fields from data.
Convert the provided image into a valid screen implementation that matches the visual design.

Context:
{{context}}`
}

func callCerebrasUXEvaluator(ctx context.Context, input uxEvaluatorRequest) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("CEREBRAS_API_KEY"))
	if apiKey == "" {
		return "", errMissingCerebrasKey
	}

	endpoint := strings.TrimSpace(os.Getenv("CEREBRAS_API_URL"))
	if endpoint == "" {
		endpoint = cerebrasDefaultEndpoint
	}
	model := strings.TrimSpace(os.Getenv("CEREBRAS_MODEL"))
	if model == "" {
		model = cerebrasDefaultModel
	}

	if input.Data == nil {
		input.Data = map[string]interface{}{}
	}

	timeoutMs := parseIntFromEnv("CEREBRAS_TIMEOUT_MS", 20000)
	clientTimeout := time.Duration(timeoutMs) * time.Millisecond
	messages, err := buildUxEvaluatorRequestMessages(input)
	if err != nil {
		return "", err
	}

	outputContent, err := requestCerebrasContent(
		ctx,
		endpoint,
		model,
		apiKey,
		clientTimeout,
		messages,
		nil,
	)
	if err != nil {
		return "", err
	}

	return sanitizeUxEvaluatorText(outputContent), nil
}

func buildCerebrasRequestMessages(input generationRequest) []cerebrasChatMessage {
	messages := make([]cerebrasChatMessage, 0, len(input.Messages)+2)
	systemMessage := buildGenerationSystemPrompt(input.Prompt, input.Context)
	if strings.TrimSpace(systemMessage) != "" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "system",
			Content: systemMessage,
		})
	}

	for _, message := range input.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, cerebrasChatMessage{
			Role:    role,
			Content: content,
		})
	}

	userPrompt := strings.TrimSpace(input.Prompt)
	if userPrompt == "" {
		return messages
	}

	if len(messages) == 0 || messages[len(messages)-1].Role != "user" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "user",
			Content: userPrompt,
		})
	}

	return messages
}

func buildCerebrasDataGenerationMessages(input dataGenerationRequest) []cerebrasChatMessage {
	messages := make([]cerebrasChatMessage, 0, len(input.Messages)+2)
	systemMessage := buildDataGenerationSystemPrompt(input)
	if strings.TrimSpace(systemMessage) != "" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "system",
			Content: systemMessage,
		})
	}

	for _, message := range input.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue;
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, cerebrasChatMessage{
			Role:    role,
			Content: content,
		})
	}

	userPrompt := strings.TrimSpace(input.Prompt)
	if userPrompt == "" {
		return messages
	}

	if len(messages) == 0 || messages[len(messages)-1].Role != "user" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "user",
			Content: userPrompt,
		})
	}

	return messages
}

func buildCerebrasPugGenerationMessages(input pugGenerationRequest) []cerebrasChatMessage {
	messages := make([]cerebrasChatMessage, 0, len(input.Messages)+2)
	systemMessage := buildPugGenerationSystemPrompt(input)
	if strings.TrimSpace(systemMessage) != "" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "system",
			Content: systemMessage,
		})
	}

	for _, message := range input.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, cerebrasChatMessage{
			Role:    role,
			Content: content,
		})
	}

	userPrompt := strings.TrimSpace(input.Prompt)
	if userPrompt == "" {
		return messages
	}

	if len(messages) == 0 || messages[len(messages)-1].Role != "user" {
		messages = append(messages, cerebrasChatMessage{
			Role:    "user",
			Content: userPrompt,
		})
	}

	return messages
}

func appendConversationMessagesForResponse(messages []cerebrasChatMessage, assistantRawContent string) []cerebrasChatMessage {
	history := make([]cerebrasChatMessage, 0, len(messages)+1)
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "system" {
			continue
		}
		if role != "user" && role != "assistant" {
			continue
		}

		history = append(history, cerebrasChatMessage{
			Role:    role,
			Content: strings.TrimSpace(message.Content),
		})
	}

	responseContent := strings.TrimSpace(assistantRawContent)
	if responseContent == "" {
		responseContent = "{}"
	}
	history = append(history, cerebrasChatMessage{
		Role:    "assistant",
		Content: responseContent,
	})
	return history
}

func buildDataGenerationSystemPrompt(input dataGenerationRequest) string {
	currentData := map[string]interface{}{}
	if input.CurrentData != nil {
		switch value := input.CurrentData.(type) {
		case map[string]interface{}:
			currentData = value
		default:
			rawData, _ := json.Marshal(input.CurrentData)
			if len(rawData) > 0 && len(rawData) < 1<<20 {
				var fallback map[string]interface{}
				if err := json.Unmarshal(rawData, &fallback); err == nil {
					currentData = fallback
				}
			}
		}
	}
	currentDataJSON, _ := json.Marshal(currentData)

	template := loadDataGenerationSystemPromptTemplate()
	parts := []string{template}

	if input.Context != nil {
		if input.Context.Locale != "" {
			parts = append(parts, "Locale: "+input.Context.Locale+".")
		}
		if input.Context.Theme != "" {
			parts = append(parts, "Theme: "+input.Context.Theme+".")
		}
		if input.Context.TargetDensity != "" {
			parts = append(parts, "Target density: "+input.Context.TargetDensity+".")
		}
		if len(input.Context.EnabledPacks) > 0 {
			parts = append(parts, "Enabled packs: "+strings.Join(input.Context.EnabledPacks, ", ")+".")
		}
	}

	currentPug := strings.TrimSpace(input.CurrentPug)
	if currentPug != "" {
		parts = append(parts, "Current screen pug:")
		parts = append(parts, currentPug)
	}
	parts = append(parts, "Current data object:")
	parts = append(parts, strings.TrimSpace(string(currentDataJSON)))
	parts = append(parts, "User request: "+strings.TrimSpace(input.Prompt))

	return strings.Join(parts, " ")
}

func buildPugGenerationSystemPrompt(input pugGenerationRequest) string {
	currentData := map[string]interface{}{}
	if input.CurrentData != nil {
		switch value := input.CurrentData.(type) {
		case map[string]interface{}:
			currentData = value
		default:
			rawData, _ := json.Marshal(input.CurrentData)
			if len(rawData) > 0 && len(rawData) < 1<<20 {
				var fallback map[string]interface{}
				if err := json.Unmarshal(rawData, &fallback); err == nil {
					currentData = fallback
				}
			}
		}
	}
	currentDataJSON, _ := json.Marshal(currentData)

	template := loadPugGenerationSystemPromptTemplate()
	parts := []string{template}

	if input.Context != nil {
		if input.Context.Locale != "" {
			parts = append(parts, "Locale: "+input.Context.Locale+".")
		}
		if input.Context.Theme != "" {
			parts = append(parts, "Theme: "+input.Context.Theme+".")
		}
		if input.Context.TargetDensity != "" {
			parts = append(parts, "Target density: "+input.Context.TargetDensity+".")
		}
		if len(input.Context.EnabledPacks) > 0 {
			parts = append(parts, "Enabled packs: "+strings.Join(input.Context.EnabledPacks, ", ")+".")
		}
	}

	currentPug := strings.TrimSpace(input.CurrentPug)
	if currentPug != "" {
		parts = append(parts, "Current screen pug:")
		parts = append(parts, currentPug)
	}
	if css := strings.TrimSpace(input.CurrentCss); css != "" {
		parts = append(parts, "Current screen css:")
		parts = append(parts, css)
	}
	parts = append(parts, "Current data object:")
	parts = append(parts, strings.TrimSpace(string(currentDataJSON)))
	parts = append(parts, "User request: "+strings.TrimSpace(input.Prompt))

	return strings.Join(parts, " ")
}

func normalizeGeneratedPug(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	templatePattern := regexp.MustCompile(`(?is)<template[^>]*>([\s\S]*?)</template>`)
	matches := templatePattern.FindStringSubmatch(text)
	if len(matches) == 2 {
		text = matches[1]
	}

	text = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?is)<style[\s\S]*?</style>`).ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}

func buildGenerationSystemPrompt(userPrompt string, ctx *generationContext) string {
	template := loadGenerationSystemPromptTemplate()
	parts := []string{template}

	if ctx != nil {
		if ctx.Locale != "" {
			parts = append(parts, "Locale: "+ctx.Locale+".")
		}
		if ctx.Theme != "" {
			parts = append(parts, "Theme: "+ctx.Theme+".")
		}
		if ctx.TargetDensity != "" {
			parts = append(parts, "Target density: "+ctx.TargetDensity+".")
		}
		if len(ctx.EnabledPacks) > 0 {
			parts = append(parts, "Enabled packs: "+strings.Join(ctx.EnabledPacks, ", "))
		}
	}

	parts = append(parts, "User request: "+strings.TrimSpace(userPrompt))

	return strings.Join(parts, " ")
}

func defaultUxEvaluatorPromptTemplate() string {
	return `You are an expert UX Evaluator and a simulated user with strong attention to usability.
Return only plain text, no JSON, no markdown.
Return one observation per line.
Use this line format:
	[High|Medium|Low] issue - recommendation
If no findings are identified, return exactly:
No issues identified.
The Pug provided uses BootstrapVue components (for example: b-form, b-form-group, b-form-input,
b-form-select, b-button, b-form-invalid-feedback), and these tags should be interpreted as their
corresponding interactive UI elements.
---
Evaluate the following screen code
template pug:
{{{pug}}}
css style:
{{{css}}}
data example:
{{{data}}}`
}

func sanitizeUxEvaluatorText(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		if idx := strings.Index(cleaned, "\n"); idx >= 0 {
			cleaned = cleaned[idx+1:]
		}
		cleaned = strings.TrimSpace(cleaned)
	}
	cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "```"))
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

func splitPromptFile(raw string) (string, string, error) {
	parts := strings.SplitN(raw, "---", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid prompt file: divider not found")
	}

	systemPrompt := strings.TrimSpace(parts[0])
	templatePrompt := strings.TrimSpace(parts[1])
	if systemPrompt == "" || templatePrompt == "" {
		return "", "", fmt.Errorf("invalid prompt file: missing sections")
	}
	return systemPrompt, templatePrompt, nil
}

func loadPromptTemplateFromEnv(templatePath string, envVar string, fallback string) string {
	resolvedPath := strings.TrimSpace(os.Getenv(envVar))
	searchPaths := []string{}
	if resolvedPath != "" {
		searchPaths = append(searchPaths, resolvedPath)
	}

	searchPaths = append(searchPaths,
		templatePath,
		filepath.Join("apps", "api", "cmd", "server", templatePath),
	)
	if cwd, err := os.Getwd(); err == nil {
		searchPaths = append(searchPaths,
			filepath.Join(cwd, templatePath),
			filepath.Join(cwd, "apps", "api", "cmd", "server", templatePath),
		)
	}
	if executablePath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executablePath)
		searchPaths = append(searchPaths,
			filepath.Join(execDir, templatePath),
			filepath.Join(execDir, "apps", "api", "cmd", "server", templatePath),
			filepath.Join(execDir, "cmd", "server", templatePath),
		)
	}

	checked := map[string]struct{}{}
	for _, path := range searchPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, exists := checked[path]; exists {
			continue
		}
		checked[path] = struct{}{}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		prompt := strings.TrimSpace(string(content))
		if prompt == "" {
			continue
		}
		return prompt
	}

	if fallback != "" {
		return strings.TrimSpace(fallback)
	}
	log.Printf("warning: unable to load prompt template from configured/path search; using fallback")
	return ""
}

func loadUxEvaluatorSystemPromptTemplate() string {
	const defaultPrompt = `You are an expert UX Evaluator.
Return only plain text and no JSON.
Each line must follow: [High|Medium|Low] issue - recommendation.
If no findings, return: No issues identified.
Treat the provided Pug as BootstrapVue template syntax. Tags such as b-form, b-form-group,
b-form-input, b-button, and b-form-invalid-feedback represent real interactive UI controls
and form feedback rendered by BootstrapVue.
Do not use markdown, code blocks, or extra prose.`
	fullPrompt := loadPromptTemplateFromEnv(
		uxEvaluatorSystemPromptTemplatePath,
		uxEvaluatorSystemPromptTemplateEnv,
		defaultPrompt,
	)
	systemPrompt, _, err := splitPromptFile(fullPrompt)
	if err == nil {
		return systemPrompt
	}
	return defaultPrompt
}

func buildUxEvaluatorRequestMessages(input uxEvaluatorRequest) ([]cerebrasChatMessage, error) {
	systemPrompt := strings.TrimSpace(loadUxEvaluatorSystemPromptTemplate())
	if systemPrompt == "" {
		return nil, fmt.Errorf("ux evaluator system prompt is empty")
	}

	uxTemplate := loadPromptTemplateFromEnv(
		uxEvaluatorSystemPromptTemplatePath,
		uxEvaluatorSystemPromptTemplateEnv,
		defaultUxEvaluatorPromptTemplate(),
	)
	_, templatePrompt, err := splitPromptFile(uxTemplate)
	if err != nil {
		return nil, err
	}
	templatePrompt = strings.TrimSpace(templatePrompt)
	if templatePrompt == "" {
		return nil, fmt.Errorf("ux evaluator user template is empty")
	}

	dataJSON, err := json.Marshal(input.Data)
	if err != nil {
		dataJSON = []byte("{}")
	}

	renderedPrompt := strings.ReplaceAll(templatePrompt, "{{{pug}}}", input.Pug)
	renderedPrompt = strings.ReplaceAll(renderedPrompt, "{{{css}}}", input.Css)
	renderedPrompt = strings.ReplaceAll(renderedPrompt, "{{{data}}}", string(dataJSON))
	if strings.TrimSpace(renderedPrompt) == "" {
		return nil, fmt.Errorf("ux evaluator user template is empty")
	}
	return []cerebrasChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: renderedPrompt,
		},
	}, nil
}

func loadGenerationSystemPromptTemplate() string {
	resolvedPath := strings.TrimSpace(os.Getenv(generationSystemPromptTemplateEnv))
	searchPaths := []string{}
	if resolvedPath != "" {
		searchPaths = append(searchPaths, resolvedPath)
	}
	searchPaths = append(searchPaths,
		generationSystemPromptTemplatePath,
		filepath.Join("apps", "api", "cmd", "server", "generation-system-prompt.txt"),
	)
	if cwd, err := os.Getwd(); err == nil {
		searchPaths = append(searchPaths,
			filepath.Join(cwd, generationSystemPromptTemplatePath),
			filepath.Join(cwd, "apps", "api", "cmd", "server", "generation-system-prompt.txt"),
		)
	}
	if executablePath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executablePath)
		searchPaths = append(searchPaths,
			filepath.Join(execDir, generationSystemPromptTemplatePath),
			filepath.Join(execDir, "apps", "api", "cmd", "server", "generation-system-prompt.txt"),
			filepath.Join(execDir, "cmd", "server", "generation-system-prompt.txt"),
		)
	}

	checked := map[string]struct{}{}
	for _, path := range searchPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, exists := checked[path]; exists {
			continue
		}
		checked[path] = struct{}{}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		prompt := strings.TrimSpace(string(content))
		if prompt == "" {
			continue
		}
		return prompt
	}

	log.Printf("warning: unable to load generation system prompt from configured/path search; using embedded fallback")
	return defaultGenerationSystemPromptTemplate()
}

func defaultGenerationSystemPromptTemplate() string {
	return `You are a Vue screen generator that outputs strict JSON only.
Return exactly one JSON object, no markdown, no fences, no code blocks, no commentary.
The JSON must contain exactly these keys: "pug", "css", "data".
All keys and string values must be valid JSON and quoted with double quotes.
Do not add comments, trailing commas, single-quoted strings, unquoted keys, backticks, markdown, or extra text.
If css contains quoted property names, do not include those quotes (for example avoid "max-width":, use max-width:).
Double quotes used inside pug or css strings must be escaped as \\\" in the JSON output.
pug must be plain Pug markup and must never contain HTML tags like <div> or </div>.
Reject the temptation to use HTML; always use Pug indentation syntax.
Do not emit any keys other than pug, css, and data.
pug example:
div.login-screen
  b-form(@submit.prevent="onSubmit")
    b-form-input(v-model="form.username" placeholder="Username")
    b-button(type="submit") Login
css must be valid plain CSS (no nested syntax, no preprocessors, no trailing commas).
data can be an object with any example values used by the screen.`
}

func defaultDataGenerationSystemPromptTemplate() string {
	return `You are a JSON data generation agent for an existing Vue screen implementation.
Return exactly one JSON object, no markdown, no fences, and no commentary.
The response must contain only one key: "data".
The "data" value must be an object used by the screen template.
Do not include pug or css keys.
If data contains strings requiring quotes in JSON, escape correctly with backslashes.
Only output valid JSON.
Use the current screen context to keep the data compatible with existing bindings.
`
}

func defaultPugGenerationSystemPromptTemplate() string {
	return `You are a Vue screen markup assistant for editing the existing template.
Return exactly one JSON object, no markdown, no fences, and no commentary.
The response must contain only one key: "pug".
Do not include css or data keys.
Only output valid JSON.
The "pug" value must be a complete Pug template compatible with Vue and the current bindings.
Preserve existing component usage and data bindings when feasible, and only apply edits requested by the user.
If unsure, apply minimal and safe edits.`
}

func loadDataGenerationSystemPromptTemplate() string {
	return strings.TrimSpace(defaultDataGenerationSystemPromptTemplate())
}

func loadPugGenerationSystemPromptTemplate() string {
	return strings.TrimSpace(defaultPugGenerationSystemPromptTemplate())
}

func extractJSONFromText(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("empty model text")
	}

	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			end := -1
			for i := len(lines) - 1; i > 0; i-- {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					end = i
					break
				}
			}
			if end > 0 {
				text = strings.TrimSpace(strings.Join(lines[1:end], "\n"))
			}
		}
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return "", fmt.Errorf("no JSON object found")
	}

	return strings.TrimSpace(text[start : end+1]), nil
}

func decodeLooseJSONOutput(raw string, output *generationResponse) error {
	raw = sanitizeJSONCandidate(raw)
	var loose map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &loose); err != nil {
		return err
	}

	pug := firstAvailableString(loose, []string{"pug", "Pug", "PUG"})
	css := firstAvailableString(loose, []string{"css", "Css", "CSS"})
	data := loose["data"]
	if data == nil {
		data = loose["Data"]
	}

	output.Pug = pug
	output.Css = css
	output.Data = data
	if output.Data == nil {
		output.Data = map[string]interface{}{}
	}

	*output = sanitizeGenerationResponse(*output)

	if err := validateGenerationResponse(output); err != nil {
		return err
	}
	return nil
}

func sanitizeGenerationResponse(output generationResponse) generationResponse {
	output.Pug = strings.TrimSpace(output.Pug)
	output.Css = strings.TrimSpace(output.Css)
	output.Pug = normalizeGeneratedPug(output.Pug)
	if output.Data == nil {
		output.Data = map[string]interface{}{}
	}
	return output
}

func sanitizeDataGenerationResponse(output dataGenerationResponse) dataGenerationResponse {
	if output.Data == nil {
		output.Data = map[string]interface{}{}
	}
	return output
}

func sanitizePugGenerationResponse(output pugGenerationResponse) pugGenerationResponse {
	output.Pug = strings.TrimSpace(output.Pug)
	output.Pug = normalizeGeneratedPug(output.Pug)
	if output.Pug == "" {
		output.Pug = "div"
	}
	return output
}

func looksLikeHTML(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, ">")
}

func convertHTMLToPug(raw string) (string, error) {
	htmlTokens := regexp.MustCompile(`(?s)<[^>]*>|[^<]+`).FindAllString(raw, -1)
	if len(htmlTokens) == 0 {
		return "", errors.New("empty html payload")
	}

	var output strings.Builder
	depth := 0

	for _, token := range htmlTokens {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}

		if isCommentOrClosingDoctype(trimmed) {
			continue
		}

		if strings.HasPrefix(trimmed, "<") {
			tagName, attrs, selfClosing, err := parseHTMLTag(trimmed)
			if err != nil {
				return "", err
			}
			if tagName == "" {
				continue
			}

			if strings.HasPrefix(trimmed, "</") {
				if depth > 0 {
					depth--
				}
				continue
			}

			linePrefix := strings.Repeat("  ", depth)
			output.WriteString(linePrefix)
			output.WriteString(tagName)
			if len(attrs) > 0 {
				output.WriteString("(")
				output.WriteString(strings.Join(attrs, " "))
				output.WriteString(")")
			}
			output.WriteRune('\n')

			if !selfClosing {
				depth++
			}
			continue
		}

		if depth > 0 && strings.TrimSpace(trimmed) != "" {
			output.WriteString(strings.Repeat("  ", depth))
			output.WriteString("| ")
			output.WriteString(trimmed)
			output.WriteRune('\n')
		}
	}

	converted := strings.TrimSpace(output.String())
	if converted == "" {
		return "", errors.New("unable to convert html to pug")
	}
	return converted, nil
}

func isCommentOrClosingDoctype(token string) bool {
	lower := strings.ToLower(strings.TrimSpace(token))
	return strings.HasPrefix(lower, "<!--") ||
		strings.HasPrefix(lower, "<!doctype")
}

func parseHTMLTag(token string) (string, []string, bool, error) {
	trimmed := strings.TrimSpace(token)
	if strings.HasPrefix(trimmed, "</") {
		return "", nil, false, nil
	}

	isSelfClosing := strings.HasSuffix(trimmed, "/>")
	content := strings.TrimSpace(trimmed)
	content = strings.TrimPrefix(content, "<")
	content = strings.TrimSuffix(content, ">")
	if isSelfClosing {
		content = strings.TrimSuffix(content, "/")
		content = strings.TrimSpace(content)
	}

	reader := []rune(content)
	i := 0
	for i < len(reader) && unicode.IsSpace(reader[i]) {
		i++
	}
	start := i
	for i < len(reader) && !unicode.IsSpace(reader[i]) {
		i++
	}
	if start == i {
		return "", nil, isSelfClosing, nil
	}

	tagName := strings.ToLower(string(reader[start:i]))
	if i < len(reader) && strings.HasSuffix(strings.ToLower(tagName), "/") {
		tagName = strings.TrimSuffix(strings.ToLower(tagName), "/")
	}

	attributesText := strings.TrimSpace(content[i:])
	attrs, err := parseHTMLAttributes(attributesText)
	if err != nil {
		return "", nil, isSelfClosing, err
	}

	if isVoidHTMLTag(tagName) {
		isSelfClosing = true
	}

	return tagName, attrs, isSelfClosing, nil
}

func parseHTMLAttributes(raw string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, nil
	}

	attrs := make([]string, 0)
	for len(text) > 0 {
		text = strings.TrimSpace(text)
		if text == "" {
			break
		}

		keyStart := 0
		keyEnd := keyStart
		for keyEnd < len(text) && isHTMLAttrChar(rune(text[keyEnd])) {
			keyEnd++
		}
		if keyEnd == keyStart {
			break
		}

		key := text[keyStart:keyEnd]
		text = strings.TrimSpace(text[keyEnd:])
		if text == "" {
			attrs = append(attrs, formatAttribute(key, "true"))
			break
		}

		if !strings.HasPrefix(text, "=") {
			attrs = append(attrs, formatAttribute(key, "true"))
			continue
		}
		text = strings.TrimPrefix(text, "=")
		text = strings.TrimSpace(text)

		if text == "" {
			attrs = append(attrs, formatAttribute(key, "true"))
			break
		}

		quote := rune(0)
		value := ""
		if text[0] == '"' || text[0] == '\'' {
			quote = rune(text[0])
			text = text[1:]
			endIdx := strings.IndexRune(text, quote)
			if endIdx < 0 {
				return nil, fmt.Errorf("unterminated quoted attribute value")
			}
			value = text[:endIdx]
			text = text[endIdx+1:]
		} else {
			endIdx := 0
			for endIdx < len(text) && !unicode.IsSpace(rune(text[endIdx])) {
				endIdx++
			}
			value = text[:endIdx]
			text = text[endIdx:]
		}
		attrs = append(attrs, formatAttribute(key, value))
	}

	return attrs, nil
}

func isHTMLAttrChar(char rune) bool {
	return char > 32 && char != '=' && char != '>' && char != '/' && char != '<'
}

func isVoidHTMLTag(tagName string) bool {
	switch tagName {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}

func formatAttribute(key, value string) string {
	safeValue := strings.ReplaceAll(strings.ReplaceAll(value, "'", "\\'"), "\\", "\\\\")
	return fmt.Sprintf("%s='%s'", key, safeValue)
}

func validateGenerationResponse(output *generationResponse) error {
	if strings.TrimSpace(output.Pug) == "" && strings.TrimSpace(output.Css) == "" && isEmptyGenerationData(output.Data) {
		return fmt.Errorf("empty generation output")
	}
	return nil
}

func isEmptyGenerationData(value interface{}) bool {
	if value == nil {
		return true
	}
	if nested, ok := value.(map[string]interface{}); ok {
		return len(nested) == 0
	}
	return false
}

func firstAvailableString(values map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			if text, casted := raw.(string); casted {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func parseGenerationJSONOutput(raw string, output *generationResponse) error {
	return parseGenerationJSONCandidate(raw, output)
}

func parseDataGenerationJSONCandidate(raw string, output *dataGenerationResponse) error {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return err
	}

	data, ok := parsed["data"]
	if !ok {
		return fmt.Errorf("missing \"data\"")
	}
	if data == nil {
		data = map[string]interface{}{}
	}

	if _, ok := data.(map[string]interface{}); !ok {
		return errors.New(`"data" must be an object`)
	}

	*output = dataGenerationResponse{
		Data: data,
	}
	return nil
}

func parsePugGenerationJSONCandidate(raw string, output *pugGenerationResponse) error {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return err
	}

	pugValue, ok := parsed["pug"]
	if !ok {
		return fmt.Errorf("missing \"pug\"")
	}
	pug, casted := pugValue.(string)
	if !casted {
		return fmt.Errorf(`"pug" must be a string`)
	}

	pug = strings.TrimSpace(pug)
	*output = pugGenerationResponse{
		Pug: pug,
	}
	return nil
}

func normalizeGeneratedJSONCandidate(raw string) string {
	text := sanitizeJSONCandidate(raw)
	return strings.TrimSpace(text)
}

func repairEscapedFieldDelimiters(raw string) string {
	fieldKeys := []string{"pug", "css", "data", "Pug", "Css", "Data"}
	fieldPattern := strings.Join(fieldKeys, "|")

	fieldKeyPattern := regexp.MustCompile(`([,{]\s*)\\*"((?:` + fieldPattern + `))"\s*:`)
	raw = fieldKeyPattern.ReplaceAllString(raw, `$1"$2":`)

	fieldOpenPattern := regexp.MustCompile(`("(?:(?:` + fieldPattern + `)"\s*:\s*)\\+"`)
	raw = fieldOpenPattern.ReplaceAllString(raw, `$1"`)

	fieldClosePattern := regexp.MustCompile(`\\+"\s*(,\s*"(?:` + fieldPattern + `)"\s*:)`)
	raw = fieldClosePattern.ReplaceAllString(raw, `",$1`)

	fieldCloseFinalPattern := regexp.MustCompile(`\\+"\s*}`)
	raw = fieldCloseFinalPattern.ReplaceAllString(raw, `"}`)

	return raw
}

func isRecoverableGeneratedJSONError(err error) bool {
	if err == nil {
		return false
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid character") ||
		strings.Contains(message, "invalid generated JSON format") ||
		strings.Contains(message, "model output is not JSON")
}

func buildGenerationRepairPrompt(rawOutput string, normalizedCandidate string, parseErr error) string {
	errText := "invalid JSON output"
	if parseErr != nil {
		errText = strings.TrimSpace(parseErr.Error())
	}

	return fmt.Sprintf(`The previous assistant output was not valid JSON for the generation contract.
Return only one valid JSON object with keys "pug", "css", and "data".
Rules:
- No markdown, no fences, no comments, no extra text.
- Keep exactly the keys "pug", "css", and "data" only.
- If CSS or Pug text requires double quotes, escape them as JSON string quotes.
- In CSS, avoid quoted property names such as "max-width"; use max-width instead.
Error: %s
Raw output to repair:
%s
Normalized candidate used for parsing:
%s`, errText, rawOutput, normalizedCandidate)
}

func buildDataGenerationRepairPrompt(rawOutput string, normalizedCandidate string, parseErr error) string {
	errText := "invalid JSON output"
	if parseErr != nil {
		errText = strings.TrimSpace(parseErr.Error())
	}

	return fmt.Sprintf(`The previous assistant output was not valid JSON for the data-generation contract.
Return only one valid JSON object with the key "data".
Rules:
- No markdown, no fences, no comments, no extra text.
- Keep exactly the key "data" only.
- The "data" value must be a JSON object.
Error: %s
Raw output to repair:
%s
Normalized candidate used for parsing:
%s`, errText, rawOutput, normalizedCandidate)
}

func buildPugGenerationRepairPrompt(rawOutput string, normalizedCandidate string, parseErr error) string {
	errText := "invalid JSON output"
	if parseErr != nil {
		errText = strings.TrimSpace(parseErr.Error())
	}

	return fmt.Sprintf(`The previous assistant output was not valid JSON for the pug-generation contract.
Return only one valid JSON object with the key "pug".
Rules:
- No markdown, no fences, no comments, no extra text.
- Keep exactly the key "pug" only.
- The "pug" value must be a string with a full, valid template.
Error: %s
Raw output to repair:
%s
Normalized candidate used for parsing:
%s`, errText, rawOutput, normalizedCandidate)
}

func requestCerebrasContent(
	ctx context.Context,
	endpoint, model, apiKey string,
	timeout time.Duration,
	messages []cerebrasChatMessage,
	responseFormat *cerebrasResponseFormat,
) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBody := cerebrasChatPayload{
		Model:    model,
		Messages: messages,
	}
	if responseFormat != nil {
		reqBody.ResponseFormat = responseFormat
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %w", err)
	}
	requestBytes, err := json.Marshal(messages)
	if err == nil {
		fmt.Println(string(requestBytes))
	} else {
		fmt.Printf("failed to marshal llm request: %v", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("cerebras request timeout")
		}
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(responseBody))
		if errorMessage == "" {
			errorMessage = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("cerebras API returned %d: %s", resp.StatusCode, errorMessage)
	}

	var parsed cerebrasChatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", fmt.Errorf("invalid cerebras response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty response from cerebras")
	}

	if parsed.Usage.TotalTokens > 0 || parsed.Usage.PromptTokensDetail.CachedTokens > 0 {
		log.Printf(
			"cerebras usage: total_tokens=%d cached_tokens=%d prompt_tokens=%d completion_tokens=%d",
			parsed.Usage.TotalTokens,
			parsed.Usage.PromptTokensDetail.CachedTokens,
			parsed.Usage.PromptTokens,
			parsed.Usage.CompletionTokens,
		)
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("empty content from cerebras")
	}
	fmt.Println(content)
	return content, nil
}

func repairUnescapedJSONStringQuotes(raw string) string {
	var output strings.Builder
	inString := false
	escaped := false
	stringStart := -1

	for i := 0; i < len(raw); i++ {
		character := raw[i]

		if !inString {
			output.WriteByte(character)
			if character == '"' {
				inString = true
				stringStart = i
			}
			continue
		}

		if escaped {
			output.WriteByte(character)
			escaped = false
			continue
		}

		if character == '\\' {
			output.WriteByte(character)
			escaped = true
			continue
		}

		if character == '"' {
			if isJSONStringTerminator(raw, i+1) {
				if isQuotedCSSPropertyName(raw, stringStart, i) {
					output.WriteString(`\\\"`)
					continue
				}
				inString = false
				output.WriteByte(character)
			} else {
				output.WriteString(`\\\"`)
			}
			continue
		}

		output.WriteByte(character)
	}

	return output.String()
}

func repairEscapedKeyLikeQuotes(raw string) string {
	return regexp.MustCompile(`\\\"([A-Za-z_][A-Za-z0-9_-]*)"\s*:`).ReplaceAllString(raw, `"$1":`)
}

func isQuotedCSSPropertyName(raw string, stringStart int, quoteIndex int) bool {
	if stringStart < 0 || quoteIndex <= stringStart || quoteIndex >= len(raw) {
		return false
	}

	stringContent := raw[stringStart+1 : quoteIndex]
	stringContent = strings.TrimRight(stringContent, " \t\r\n")
	if stringContent == "" {
		return false
	}

	tokenEnd := len(stringContent)
	tokenStart := tokenEnd
	for tokenStart > 0 && isCSSPropertyChar(rune(stringContent[tokenStart-1])) {
		tokenStart--
	}
	token := stringContent[tokenStart:tokenEnd]
	if token == "" || !isCSSPropertyToken(token) {
		return false
	}

	prefix := strings.TrimRight(stringContent[:tokenStart], " \t\r\n")
	if prefix == "" {
		return true
	}
	last := rune(prefix[len(prefix)-1])
	if last == '{' || last == ';' || last == '(' {
		return true
	}

	return false
}

func isCSSPropertyToken(text string) bool {
	for i, char := range text {
		if i == 0 {
			if char != '-' && !isASCIILetter(char) {
				return false
			}
			continue
		}
		if !isCSSPropertyChar(char) {
			return false
		}
	}
	return text != ""
}

func isCSSPropertyChar(char rune) bool {
	return isASCIILetter(char) || (char >= '0' && char <= '9') || char == '-' || char == '_'
}

func isASCIILetter(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func isJSONStringTerminator(raw string, start int) bool {
	for start < len(raw) && unicode.IsSpace(rune(raw[start])) {
		start++
	}
	if start >= len(raw) {
		return true
	}

	switch raw[start] {
	case ':', ',', '}', ']':
		return true
	}
	return false
}

func sanitizeJSONCandidate(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return text
	}

	text = regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?s)/\*[\s\S]*?\*/`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\bundefined\b`).ReplaceAllString(text, "null")
	text = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`([,{]\s*)([A-Za-z_][A-Za-z0-9_-]*)\s*:`).ReplaceAllString(text, `$1"$2":`)

	return strings.TrimSpace(text)
}

func parseIntFromEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseBoolFromEnv(key string, fallback bool) bool {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(rawValue)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadEnvFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		idx := strings.Index(trimmed, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:idx])
		value := strings.TrimSpace(trimmed[idx+1:])
		if key == "" {
			continue
		}

		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = strings.Trim(value, "\"")
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
			value = strings.Trim(value, "'")
		}

		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, value)
		}
	}
}
