package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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
const generationSystemPromptTemplatePath = "cmd/server/generation-system-prompt.txt"
const generationSystemPromptTemplateEnv = "GENERATION_SYSTEM_PROMPT_PATH"

func main() {
	loadEnvFile(".env")

	storagePath := strings.TrimSpace(os.Getenv("COMPONENT_REGISTRY_PATH"))
	if storagePath == "" {
		storagePath = "data/component-registry.json"
	}

	svc := registry.NewRegistryService(storagePath)

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

	addr := strings.TrimSpace(os.Getenv("PORT"))
	if addr == "" {
		addr = ":3000"
	}
	if !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("component registry API listening on %s", addr)
	log.Printf("generation service listening on %s", addr)
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

type generationResponse struct {
	Pug  string      `json:"pug"`
	Css  string      `json:"css"`
	Data interface{} `json:"data"`
	// Messages represent the conversation after this turn, excluding system messages.
	Messages []cerebrasChatMessage `json:"messages,omitempty"`
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
	callCtx, cancel := context.WithTimeout(ctx, clientTimeout)
	defer cancel()

	messages := buildCerebrasRequestMessages(input)
	requestBytes, err := json.Marshal(messages)
	if err == nil {
		fmt.Println(string(requestBytes))
	} else {
		fmt.Printf("failed to marshal llm request: %v", err)
	}

	reqBody := cerebrasChatPayload{
		Model:    model,
		Messages: messages,
		ResponseFormat: &cerebrasResponseFormat{
			Type: "json_object",
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return generationResponse{}, fmt.Errorf("failed to encode request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return generationResponse{}, err
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	httpClient := &http.Client{Timeout: clientTimeout}
	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return generationResponse{}, fmt.Errorf("cerebras request timeout")
		}
		return generationResponse{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return generationResponse{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorMessage := strings.TrimSpace(string(responseBody))
		if errorMessage == "" {
			errorMessage = http.StatusText(resp.StatusCode)
		}
		return generationResponse{}, fmt.Errorf("cerebras API returned %d: %s", resp.StatusCode, errorMessage)
	}

	var parsed cerebrasChatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return generationResponse{}, fmt.Errorf("invalid cerebras response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return generationResponse{}, fmt.Errorf("empty response from cerebras")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return generationResponse{}, fmt.Errorf("empty content from cerebras")
	}
	fmt.Println(content)

	jsonCandidate, err := extractJSONFromText(content)
	if err != nil {
		return generationResponse{}, fmt.Errorf("model output is not JSON: %w", err)
	}
	fmt.Println(jsonCandidate)

	var output generationResponse
	jsonCandidate = sanitizeJSONCandidate(jsonCandidate)
	if err := json.Unmarshal([]byte(jsonCandidate), &output); err != nil {
		if err := decodeLooseJSONOutput(jsonCandidate, &output); err != nil {
			return generationResponse{}, fmt.Errorf("invalid generated JSON format: %w\n%s", err, jsonCandidate)
		}
	}

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
		return generationResponse{}, fmt.Errorf("invalid generated response: %w", err)
	}
	output.Messages = appendConversationMessagesForResponse(messages, content)

	return output, nil
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
