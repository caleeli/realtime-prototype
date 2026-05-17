package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const projectImagesDirEnv = "PROJECT_IMAGES_DIR"
const projectImagesDefaultDir = "data/project-images"

type projectImageVersion struct {
	ID               string `json:"id"`
	Prompt           string `json:"prompt"`
	CreatedAt        string `json:"createdAt"`
	SourceType       string `json:"sourceType"`
	FileName         string `json:"fileName"`
	OriginalFileName string `json:"originalFileName,omitempty"`
	SizeBytes        int64  `json:"sizeBytes"`
	RequestedSize    string `json:"requestedSize,omitempty"`
	GeneratedSize    string `json:"generatedSize,omitempty"`
}

type projectImageRecord struct {
	ID               string                `json:"id"`
	ProjectID        string                `json:"projectId"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	CreatedAt        string                `json:"createdAt"`
	UpdatedAt        string                `json:"updatedAt"`
	CurrentVersionID string                `json:"currentVersionId"`
	RedoVersionIDs   []string              `json:"redoVersionIds"`
	Versions         []projectImageVersion `json:"versions"`
}

type projectImageManifest struct {
	ProjectID string               `json:"projectId"`
	Images    []projectImageRecord `json:"images"`
}

type projectImageResponse struct {
	ID                string                `json:"id"`
	ProjectID         string                `json:"projectId"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	CreatedAt         string                `json:"createdAt"`
	UpdatedAt         string                `json:"updatedAt"`
	CurrentVersionID  string                `json:"currentVersionId"`
	CurrentImageURL   string                `json:"currentImageUrl"`
	Versions          []projectImageVersion `json:"versions"`
	RedoAvailable     bool                  `json:"redoAvailable"`
	RollbackAvailable bool                  `json:"rollbackAvailable"`
}

func registerProjectImageRoutes(mux *http.ServeMux, sessionStore *sessionProjectStore) {
	mux.HandleFunc("/api/project-images", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		project, ok := resolveProjectForImageRoutes(w, r, sessionStore)
		if !ok {
			return
		}
		manifest, err := loadProjectImageManifest(project.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, toProjectImageListResponse(manifest))
	}))

	mux.HandleFunc("/api/project-images/generate", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		project, ok := resolveProjectForImageRoutes(w, r, sessionStore)
		if !ok {
			return
		}
		var payload struct {
			Prompt       string `json:"prompt"`
			Name         string `json:"name"`
			Description  string `json:"description"`
			ImageModel   string `json:"imageModel"`
			ImageSize    string `json:"imageSize"`
			ImageQuality string `json:"imageQuality"`
			ImageStyle   string `json:"imageStyle"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}
		prompt := strings.TrimSpace(payload.Prompt)
		if prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
			return
		}

		model := strings.TrimSpace(payload.ImageModel)
		if model == "" {
			model = inspirationImageDefaultOpenAIModel
		}
		size := strings.TrimSpace(payload.ImageSize)
		if size == "" {
			size = "1024x1024"
		}
		requestedW, requestedH, parseErr := parseRequestedSize(size)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": parseErr.Error()})
			return
		}
		generatedW, generatedH := pickNearestSupportedSize(requestedW, requestedH)
		generatedSize := fmt.Sprintf("%dx%d", generatedW, generatedH)
		requestedSize := fmt.Sprintf("%dx%d", requestedW, requestedH)
		quality := strings.TrimSpace(payload.ImageQuality)
		if quality == "" {
			quality = "standard"
		}
		style := strings.TrimSpace(payload.ImageStyle)
		if style == "" {
			style = "vivid"
		}
		provider := normalizeInspirationProvider(strings.TrimSpace(os.Getenv(inspirationImageProviderEnv)), "", model)
		endpoint := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(provider, "API_URL")))
		if endpoint == "" {
			endpoint = inspirationImageDefaultEndpoint(provider)
		}
		apiKey := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(provider, "API_KEY")))
		if apiKey == "" {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("missing image API key for %s provider", provider)})
			return
		}
		timeoutMs := parseIntFromEnv(inspirationImageProviderEnvVar(provider, "TIMEOUT_MS"), 90000)
		projectSettings, _ := sessionStore.getProjectSettings(r.Context(), project.ID)
		projectImageNotes := buildProjectImageGenerationNotes(project, projectSettings)
		b64, err := callImageGeneration(
			r.Context(),
			endpoint,
			model,
			provider,
			apiKey,
			time.Duration(timeoutMs)*time.Millisecond,
			prompt,
			generatedSize,
			quality,
			style,
			1,
			projectImageNotes,
		)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		imageBytes, err := decodeBase64ImageString(b64)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid image payload from provider"})
			return
		}
		resizedBytes, resizeErr := resizeImageBytes(imageBytes, requestedW, requestedH)
		if resizeErr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to resize generated image"})
			return
		}

		record, err := createProjectImageFromBytes(
			project.ID,
			strings.TrimSpace(payload.Name),
			strings.TrimSpace(payload.Description),
			prompt,
			"generated",
			resizedBytes,
			imageBytes,
			requestedSize,
			generatedSize,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, toProjectImageResponse(record))
	}))

	mux.HandleFunc("/api/project-images/upload", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		project, ok := resolveProjectForImageRoutes(w, r, sessionStore)
		if !ok {
			return
		}
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart payload"})
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
			return
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, 12<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read uploaded file"})
			return
		}
		if len(body) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file cannot be empty"})
			return
		}

		record, err := createProjectImageFromBytes(
			project.ID,
			strings.TrimSpace(r.FormValue("name")),
			strings.TrimSpace(r.FormValue("description")),
			"upload",
			"uploaded",
			body,
			nil,
			"",
			"",
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, toProjectImageResponse(record))
	}))

	mux.HandleFunc("/api/project-images/assets/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		relPath := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/project-images/assets/"))
		if relPath == "" || strings.Contains(relPath, "..") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fullPath := filepath.Join(projectImagesRootDir(), filepath.Clean(relPath))
		if _, err := os.Stat(fullPath); err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, fullPath)
	}))

	mux.HandleFunc("/api/project-images/", withCORS(func(w http.ResponseWriter, r *http.Request) {
		project, ok := resolveProjectForImageRoutes(w, r, sessionStore)
		if !ok {
			return
		}
		subPath := strings.TrimPrefix(r.URL.Path, "/api/project-images/")
		parts := strings.Split(strings.TrimSpace(subPath), "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		imageID := strings.TrimSpace(parts[0])

		switch {
		case len(parts) == 1 && r.Method == http.MethodPatch:
			var payload struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}
			record, err := updateProjectImageMetadata(project.ID, imageID, payload.Name, payload.Description)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not found"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, toProjectImageResponse(record))
			return
		case len(parts) == 2 && parts[1] == "edit" && r.Method == http.MethodPost:
			var payload struct {
				Prompt string `json:"prompt"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
				return
			}
			prompt := strings.TrimSpace(payload.Prompt)
			if prompt == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
				return
			}
			projectSettings, _ := sessionStore.getProjectSettings(r.Context(), project.ID)
			projectImageNotes := buildProjectImageGenerationNotes(project, projectSettings)
			record, err := editProjectImageWithPrompt(r.Context(), project.ID, imageID, prompt, projectImageNotes)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not found"})
					return
				}
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, toProjectImageResponse(record))
			return

		case len(parts) == 2 && parts[1] == "rollback" && r.Method == http.MethodPost:
			record, err := rollbackProjectImage(project.ID, imageID)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not found"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, toProjectImageResponse(record))
			return

		case len(parts) == 2 && parts[1] == "redo" && r.Method == http.MethodPost:
			record, err := redoProjectImage(project.ID, imageID)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "image not found"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, toProjectImageResponse(record))
			return

		case len(parts) == 2 && parts[1] == "download" && r.Method == http.MethodGet:
			record, version, filePath, err := resolveProjectImageFile(project.ID, imageID, strings.TrimSpace(r.URL.Query().Get("versionId")))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			if r.URL.Query().Get("original") == "true" && strings.TrimSpace(version.OriginalFileName) != "" {
				filePath = filepath.Join(projectImagesRootDir(), sanitizeFileName(project.ID), sanitizeFileName(imageID), version.OriginalFileName)
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s.png\"", sanitizeFileName(record.Name), version.ID))
			http.ServeFile(w, r, filePath)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
}

func resolveProjectForImageRoutes(w http.ResponseWriter, r *http.Request, sessionStore *sessionProjectStore) (projectRecord, bool) {
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	project, err := sessionStore.getProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, errProjectNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return projectRecord{}, false
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return projectRecord{}, false
	}
	return project, true
}

func projectImagesRootDir() string {
	value := strings.TrimSpace(os.Getenv(projectImagesDirEnv))
	if value == "" {
		return projectImagesDefaultDir
	}
	return value
}

func projectImageManifestPath(projectID string) string {
	return filepath.Join(projectImagesRootDir(), sanitizeFileName(projectID), "manifest.json")
}

func ensureProjectImageDir(projectID string) (string, error) {
	dir := filepath.Join(projectImagesRootDir(), sanitizeFileName(projectID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func loadProjectImageManifest(projectID string) (projectImageManifest, error) {
	if _, err := ensureProjectImageDir(projectID); err != nil {
		return projectImageManifest{}, err
	}
	path := projectImageManifestPath(projectID)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return projectImageManifest{ProjectID: projectID, Images: []projectImageRecord{}}, nil
		}
		return projectImageManifest{}, err
	}
	var manifest projectImageManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return projectImageManifest{}, err
	}
	if manifest.ProjectID == "" {
		manifest.ProjectID = projectID
	}
	if manifest.Images == nil {
		manifest.Images = []projectImageRecord{}
	}
	return manifest, nil
}

func saveProjectImageManifest(manifest projectImageManifest) error {
	if _, err := ensureProjectImageDir(manifest.ProjectID); err != nil {
		return err
	}
	path := projectImageManifestPath(manifest.ProjectID)
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func createProjectImageFromBytes(
	projectID, name, description, prompt, sourceType string,
	content []byte,
	originalContent []byte,
	requestedSize string,
	generatedSize string,
) (projectImageRecord, error) {
	manifest, err := loadProjectImageManifest(projectID)
	if err != nil {
		return projectImageRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	imageID := "img-" + newSessionID()
	versionID := "v-" + newSessionID()
	if strings.TrimSpace(name) == "" {
		name = "Imagen " + time.Now().Format("2006-01-02 15:04")
	}
	fileName := versionID + ".png"
	originalFileName := ""
	if len(originalContent) > 0 {
		originalFileName = versionID + "-original.png"
	}
	relPath := filepath.Join(sanitizeFileName(projectID), sanitizeFileName(imageID), fileName)
	absPath := filepath.Join(projectImagesRootDir(), relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return projectImageRecord{}, err
	}
	if err := os.WriteFile(absPath, content, 0o644); err != nil {
		return projectImageRecord{}, err
	}
	if len(originalContent) > 0 && originalFileName != "" {
		origPath := filepath.Join(projectImagesRootDir(), sanitizeFileName(projectID), sanitizeFileName(imageID), originalFileName)
		if err := os.WriteFile(origPath, originalContent, 0o644); err != nil {
			return projectImageRecord{}, err
		}
	}
	record := projectImageRecord{
		ID:               imageID,
		ProjectID:        projectID,
		Name:             name,
		Description:      strings.TrimSpace(description),
		CreatedAt:        now,
		UpdatedAt:        now,
		CurrentVersionID: versionID,
		RedoVersionIDs:   []string{},
		Versions: []projectImageVersion{{
			ID:               versionID,
			Prompt:           prompt,
			CreatedAt:        now,
			SourceType:       sourceType,
			FileName:         fileName,
			SizeBytes:        int64(len(content)),
			OriginalFileName: originalFileName,
			RequestedSize:    requestedSize,
			GeneratedSize:    generatedSize,
		}},
	}
	manifest.Images = append(manifest.Images, record)
	if err := saveProjectImageManifest(manifest); err != nil {
		return projectImageRecord{}, err
	}
	return record, nil
}

func updateProjectImageMetadata(projectID, imageID, name, description string) (projectImageRecord, error) {
	manifest, index, err := loadProjectImageByID(projectID, imageID)
	if err != nil {
		return projectImageRecord{}, err
	}
	record := manifest.Images[index]
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		record.Name = trimmed
	}
	record.Description = strings.TrimSpace(description)
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Images[index] = record
	if err := saveProjectImageManifest(manifest); err != nil {
		return projectImageRecord{}, err
	}
	return record, nil
}

func editProjectImageWithPrompt(ctx context.Context, projectID, imageID, prompt string, projectImageNotes string) (projectImageRecord, error) {
	manifest, index, err := loadProjectImageByID(projectID, imageID)
	if err != nil {
		return projectImageRecord{}, err
	}
	record := manifest.Images[index]

	provider := normalizeInspirationProvider(strings.TrimSpace(os.Getenv(inspirationImageProviderEnv)), "", inspirationImageDefaultOpenAIModel)
	endpoint := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(provider, "API_URL")))
	if endpoint == "" {
		endpoint = inspirationImageDefaultEndpoint(provider)
	}
	apiKey := strings.TrimSpace(os.Getenv(inspirationImageProviderEnvVar(provider, "API_KEY")))
	if apiKey == "" {
		return projectImageRecord{}, fmt.Errorf("missing image API key for %s provider", provider)
	}
	timeoutMs := parseIntFromEnv(inspirationImageProviderEnvVar(provider, "TIMEOUT_MS"), 90000)
	enhancedPrompt := "Edita o mejora una imagen existente según esta instrucción: " + prompt
	b64, err := callImageGeneration(
		ctx,
		endpoint,
		inspirationImageDefaultModel(provider),
		provider,
		apiKey,
		time.Duration(timeoutMs)*time.Millisecond,
		enhancedPrompt,
		"1024x1024",
		"standard",
		"vivid",
		1,
		projectImageNotes,
	)
	if err != nil {
		return projectImageRecord{}, err
	}
	content, err := decodeBase64ImageString(b64)
	if err != nil {
		return projectImageRecord{}, fmt.Errorf("invalid image payload from provider")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	versionID := "v-" + newSessionID()
	fileName := versionID + ".png"
	relPath := filepath.Join(sanitizeFileName(projectID), sanitizeFileName(record.ID), fileName)
	absPath := filepath.Join(projectImagesRootDir(), relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return projectImageRecord{}, err
	}
	if err := os.WriteFile(absPath, content, 0o644); err != nil {
		return projectImageRecord{}, err
	}
	record.UpdatedAt = now
	record.CurrentVersionID = versionID
	record.RedoVersionIDs = []string{}
	record.Versions = append(record.Versions, projectImageVersion{
		ID:         versionID,
		Prompt:     prompt,
		CreatedAt:  now,
		SourceType: "edited",
		FileName:   fileName,
		SizeBytes:  int64(len(content)),
	})
	manifest.Images[index] = record
	if err := saveProjectImageManifest(manifest); err != nil {
		return projectImageRecord{}, err
	}
	return record, nil
}

func rollbackProjectImage(projectID, imageID string) (projectImageRecord, error) {
	manifest, index, err := loadProjectImageByID(projectID, imageID)
	if err != nil {
		return projectImageRecord{}, err
	}
	record := manifest.Images[index]
	currentIndex := findVersionIndex(record.Versions, record.CurrentVersionID)
	if currentIndex <= 0 {
		return projectImageRecord{}, fmt.Errorf("no previous version available")
	}
	record.RedoVersionIDs = append(record.RedoVersionIDs, record.CurrentVersionID)
	record.CurrentVersionID = record.Versions[currentIndex-1].ID
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Images[index] = record
	if err := saveProjectImageManifest(manifest); err != nil {
		return projectImageRecord{}, err
	}
	return record, nil
}

func redoProjectImage(projectID, imageID string) (projectImageRecord, error) {
	manifest, index, err := loadProjectImageByID(projectID, imageID)
	if err != nil {
		return projectImageRecord{}, err
	}
	record := manifest.Images[index]
	if len(record.RedoVersionIDs) == 0 {
		return projectImageRecord{}, fmt.Errorf("no redo version available")
	}
	nextID := record.RedoVersionIDs[len(record.RedoVersionIDs)-1]
	record.RedoVersionIDs = record.RedoVersionIDs[:len(record.RedoVersionIDs)-1]
	record.CurrentVersionID = nextID
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.Images[index] = record
	if err := saveProjectImageManifest(manifest); err != nil {
		return projectImageRecord{}, err
	}
	return record, nil
}

func resolveProjectImageFile(projectID, imageID, versionID string) (projectImageRecord, projectImageVersion, string, error) {
	manifest, index, err := loadProjectImageByID(projectID, imageID)
	if err != nil {
		return projectImageRecord{}, projectImageVersion{}, "", err
	}
	record := manifest.Images[index]
	selectedID := strings.TrimSpace(versionID)
	if selectedID == "" {
		selectedID = record.CurrentVersionID
	}
	for _, version := range record.Versions {
		if version.ID != selectedID {
			continue
		}
		filePath := filepath.Join(projectImagesRootDir(), sanitizeFileName(projectID), sanitizeFileName(imageID), version.FileName)
		if _, statErr := os.Stat(filePath); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return projectImageRecord{}, projectImageVersion{}, "", os.ErrNotExist
			}
			return projectImageRecord{}, projectImageVersion{}, "", statErr
		}
		return record, version, filePath, nil
	}
	return projectImageRecord{}, projectImageVersion{}, "", os.ErrNotExist
}

func loadProjectImageByID(projectID, imageID string) (projectImageManifest, int, error) {
	manifest, err := loadProjectImageManifest(projectID)
	if err != nil {
		return projectImageManifest{}, -1, err
	}
	for i := range manifest.Images {
		if manifest.Images[i].ID == imageID {
			return manifest, i, nil
		}
	}
	return projectImageManifest{}, -1, os.ErrNotExist
}

func findVersionIndex(versions []projectImageVersion, versionID string) int {
	for i := range versions {
		if versions[i].ID == versionID {
			return i
		}
	}
	return -1
}

func toProjectImageListResponse(manifest projectImageManifest) []projectImageResponse {
	items := make([]projectImageResponse, 0, len(manifest.Images))
	for _, image := range manifest.Images {
		items = append(items, toProjectImageResponse(image))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return items
}

func toProjectImageResponse(record projectImageRecord) projectImageResponse {
	currentURL := ""
	if record.CurrentVersionID != "" {
		currentURL = fmt.Sprintf("/api/project-images/assets/%s/%s/%s.png", sanitizeFileName(record.ProjectID), sanitizeFileName(record.ID), sanitizeFileName(record.CurrentVersionID))
	}
	rollbackAvailable := false
	if idx := findVersionIndex(record.Versions, record.CurrentVersionID); idx > 0 {
		rollbackAvailable = true
	}
	return projectImageResponse{
		ID:                record.ID,
		ProjectID:         record.ProjectID,
		Name:              record.Name,
		Description:       record.Description,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
		CurrentVersionID:  record.CurrentVersionID,
		CurrentImageURL:   currentURL,
		Versions:          record.Versions,
		RedoAvailable:     len(record.RedoVersionIDs) > 0,
		RollbackAvailable: rollbackAvailable,
	}
}

func sanitizeFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "item"
	}
	builder := strings.Builder{}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			builder.WriteRune(ch)
			continue
		}
		builder.WriteRune('-')
	}
	clean := strings.Trim(builder.String(), "-")
	if clean == "" {
		return "item"
	}
	return clean
}

func decodeBase64ImageString(raw string) ([]byte, error) {
	data := strings.TrimSpace(raw)
	if data == "" {
		return nil, fmt.Errorf("empty image data")
	}
	if comma := strings.Index(data, ","); comma >= 0 && strings.Contains(data[:comma], "base64") {
		data = strings.TrimSpace(data[comma+1:])
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func buildProjectImageGenerationNotes(project projectRecord, settings projectSettingsRecord) string {
	lines := []string{
		"Project context for image generation:",
		fmt.Sprintf("- Project name: %s", strings.TrimSpace(project.Name)),
		fmt.Sprintf("- Active theme: %s", strings.TrimSpace(project.Theme)),
	}
	if value := strings.TrimSpace(settings.DesignStyle); value != "" {
		lines = append(lines, "- Design style: "+value)
	}
	if value := strings.TrimSpace(settings.ColorPalette); value != "" {
		lines = append(lines, "- Color palette: "+value)
	}
	if value := strings.TrimSpace(settings.BrandGuidelines); value != "" {
		lines = append(lines, "- Brand guidelines: "+value)
	}
	if value := strings.TrimSpace(settings.ComponentExamples); value != "" {
		lines = append(lines, "- Component examples: "+value)
	}
	if value := strings.TrimSpace(settings.TechnicalConstraints); value != "" {
		lines = append(lines, "- Technical constraints: "+value)
	}
	if value := strings.TrimSpace(settings.LayoutPreferences); value != "" {
		lines = append(lines, "- Layout preferences: "+value)
	}
	if value := strings.TrimSpace(settings.GenerationContext); value != "" {
		lines = append(lines, "- Additional generation context: "+value)
	}
	if value := strings.TrimSpace(settings.ImageGenerationNotes); value != "" {
		lines = append(lines, "- Image-specific notes: "+value)
	}
	return strings.Join(lines, "\n")
}

type imageSizeOption struct {
	Width  int
	Height int
}

var supportedGenerationSizes = []imageSizeOption{
	{Width: 1024, Height: 1024},
	{Width: 1024, Height: 1536},
	{Width: 1536, Height: 1024},
}

func parseRequestedSize(size string) (int, int, error) {
	trimmed := strings.TrimSpace(size)
	parts := strings.Split(trimmed, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid imageSize format, expected WxH")
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid imageSize values, width and height must be positive integers")
	}
	if w > 4096 || h > 4096 {
		return 0, 0, fmt.Errorf("imageSize too large, max supported requested dimension is 4096")
	}
	return w, h, nil
}

func pickNearestSupportedSize(requestedW, requestedH int) (int, int) {
	targetRatio := float64(requestedW) / float64(requestedH)
	best := supportedGenerationSizes[0]
	bestDistance := math.Abs((float64(best.Width)/float64(best.Height) - targetRatio))
	for _, option := range supportedGenerationSizes[1:] {
		distance := math.Abs((float64(option.Width)/float64(option.Height) - targetRatio))
		if distance < bestDistance {
			bestDistance = distance
			best = option
		}
	}
	return best.Width, best.Height
}

func resizeImageBytes(raw []byte, targetW, targetH int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		decodedJpeg, jpegErr := jpeg.Decode(bytes.NewReader(raw))
		if jpegErr != nil {
			return nil, err
		}
		img = decodedJpeg
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid source image dimensions")
	}
	for y := 0; y < targetH; y++ {
		srcY := srcBounds.Min.Y + (y*srcH)/targetH
		for x := 0; x < targetW; x++ {
			srcX := srcBounds.Min.X + (x*srcW)/targetW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	buffer := bytes.Buffer{}
	if encodeErr := png.Encode(&buffer, dst); encodeErr != nil {
		return nil, encodeErr
	}
	return buffer.Bytes(), nil
}
