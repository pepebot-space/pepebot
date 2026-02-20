package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/providers"
)

// OpenAI-compatible request/response types

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ChatContentBlock represents an OpenAI-compatible content block (text, image_url, file)
type ChatContentBlock struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ChatImageURL `json:"image_url,omitempty"`
	File     *ChatFileData `json:"file,omitempty"`
}

type ChatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type ChatFileData struct {
	FileData string `json:"file_data,omitempty"`
	URL      string `json:"url,omitempty"`
}

// parseMessageContent extracts text content and media URLs from a ChatMessage.
// Content can be a plain string or an array of content blocks (OpenAI multimodal format).
func parseMessageContent(msg ChatMessage) (text string, media []string) {
	switch v := msg.Content.(type) {
	case string:
		return v, nil
	case []interface{}:
		var texts []string
		for _, item := range v {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				if t, ok := block["text"].(string); ok {
					texts = append(texts, t)
				}
			case "image_url":
				if imgObj, ok := block["image_url"].(map[string]interface{}); ok {
					if url, ok := imgObj["url"].(string); ok {
						media = append(media, url)
					}
				}
			case "file":
				if fileObj, ok := block["file"].(map[string]interface{}); ok {
					if fd, ok := fileObj["file_data"].(string); ok {
						media = append(media, fd)
					} else if u, ok := fileObj["url"].(string); ok {
						media = append(media, u)
					}
				}
			}
		}
		return strings.Join(texts, "\n"), media
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// getContentString returns the string representation of ChatMessage content for responses
func getContentString(content interface{}) string {
	if s, ok := content.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", content)
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *UsageResponse         `json:"usage,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	FinishReason string       `json:"finish_reason"`
}

type UsageResponse struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunkResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []StreamChunkChoice `json:"choices"`
}

type StreamChunkChoice struct {
	Index        int              `json:"index"`
	Delta        StreamChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason,omitempty"`
}

type StreamChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type ModelListResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type SessionListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	Key          string `json:"key"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
	MessageCount int    `json:"message_count"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// handleHealth returns a simple health check response
func (gs *GatewayServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleChatCompletions handles the OpenAI-compatible chat completions endpoint
func (gs *GatewayServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages array is required and must not be empty", "invalid_request_error")
		return
	}

	// Extract agent name from header, default to "default"
	agentName := r.Header.Get("X-Agent")
	if agentName == "" {
		agentName = "default"
	}

	// Extract session key from header, default to "web:<agent>"
	sessionKey := r.Header.Get("X-Session-Key")
	if sessionKey == "" {
		sessionKey = "web:" + agentName
	}

	// Get the last user message as the content to process
	lastMessage := req.Messages[len(req.Messages)-1]
	if lastMessage.Role != "user" {
		writeError(w, http.StatusBadRequest, "last message must be from user", "invalid_request_error")
		return
	}

	// Parse content: supports plain string or multimodal content blocks
	textContent, media := parseMessageContent(lastMessage)

	logger.DebugCF("gateway", "Chat completion request", map[string]interface{}{
		"agent":       agentName,
		"session_key": sessionKey,
		"stream":      req.Stream,
		"model":       req.Model,
		"has_media":   len(media) > 0,
	})

	completionID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	if req.Stream {
		gs.handleStreamingResponse(w, r, textContent, media, sessionKey, agentName, req.Model, completionID)
	} else {
		gs.handleNonStreamingResponse(w, r, textContent, media, sessionKey, agentName, req.Model, completionID)
	}
}

// handleNonStreamingResponse handles non-streaming chat completions
func (gs *GatewayServer) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, content string, media []string, sessionKey, agentName, model, completionID string) {
	ctx := r.Context()

	response, err := gs.agentManager.ProcessDirect(ctx, content, media, sessionKey, agentName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "processing error: "+err.Error(), "server_error")
		return
	}

	resp := ChatCompletionResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: &ChatMessage{
					Role:    "assistant",
					Content: response,
				},
				FinishReason: "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleStreamingResponse handles SSE streaming chat completions
func (gs *GatewayServer) handleStreamingResponse(w http.ResponseWriter, r *http.Request, content string, media []string, sessionKey, agentName, model, completionID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported", "server_error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Send initial role chunk
	initialChunk := StreamChunkResponse{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []StreamChunkChoice{
			{
				Index: 0,
				Delta: StreamChunkDelta{
					Role: "assistant",
				},
			},
		},
	}
	writeSSEChunk(w, initialChunk)
	flusher.Flush()

	ctx := r.Context()

	err := gs.agentManager.ProcessDirectStream(ctx, content, media, sessionKey, agentName, func(chunk providers.StreamChunk) {
		if chunk.Done {
			// Send finish chunk
			stopReason := "stop"
			finishChunk := StreamChunkResponse{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   model,
				Choices: []StreamChunkChoice{
					{
						Index:        0,
						Delta:        StreamChunkDelta{},
						FinishReason: &stopReason,
					},
				},
			}
			writeSSEChunk(w, finishChunk)
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		if chunk.Content != "" {
			contentChunk := StreamChunkResponse{
				ID:      completionID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   model,
				Choices: []StreamChunkChoice{
					{
						Index: 0,
						Delta: StreamChunkDelta{
							Content: chunk.Content,
						},
					},
				},
			}
			writeSSEChunk(w, contentChunk)
			flusher.Flush()
		}
	})

	if err != nil {
		logger.ErrorCF("gateway", "Stream processing error", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// handleListModels returns available models
func (gs *GatewayServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	agents := gs.agentManager.ListAgents()
	models := make([]ModelObject, 0)

	// Collect unique models from agents
	seen := make(map[string]bool)
	for _, agentDef := range agents {
		if !agentDef.Enabled || seen[agentDef.Model] {
			continue
		}
		seen[agentDef.Model] = true
		models = append(models, ModelObject{
			ID:      agentDef.Model,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "pepebot",
		})
	}

	// Add default model from config if not already present
	defaultModel := gs.config.Agents.Defaults.Model
	if !seen[defaultModel] {
		models = append(models, ModelObject{
			ID:      defaultModel,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "pepebot",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModelListResponse{
		Object: "list",
		Data:   models,
	})
}

// handleListSessions returns active web sessions
func (gs *GatewayServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	sessions := gs.agentManager.GetSessions()
	if sessions == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SessionListResponse{Sessions: []SessionInfo{}})
		return
	}

	allSessions := sessions.ListSessions("")
	sessionInfos := make([]SessionInfo, 0, len(allSessions))

	for _, s := range allSessions {
		sessionInfos = append(sessionInfos, SessionInfo{
			Key:          s.Key,
			Created:      s.Created.Format(time.RFC3339),
			Updated:      s.Updated.Format(time.RFC3339),
			MessageCount: len(s.Messages),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SessionListResponse{Sessions: sessionInfos})
}

// handleSessionRoutes dispatches session sub-routes
func (gs *GatewayServer) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse: /v1/sessions/{key}/new, /v1/sessions/{key}/stop, /v1/sessions/{key}
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if path == "" {
		gs.handleListSessions(w, r)
		return
	}

	// Check for sub-actions
	if strings.HasSuffix(path, "/new") {
		sessionKey := strings.TrimSuffix(path, "/new")
		gs.handleSessionNew(w, r, sessionKey)
		return
	}

	if strings.HasSuffix(path, "/stop") {
		sessionKey := strings.TrimSuffix(path, "/stop")
		gs.handleSessionStop(w, r, sessionKey)
		return
	}

	// Direct session key - GET to get history, DELETE to delete
	sessionKey := path
	if r.Method == http.MethodGet {
		gs.handleGetSession(w, r, sessionKey)
		return
	}
	if r.Method == http.MethodDelete {
		gs.handleDeleteSession(w, r, sessionKey)
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
}

// handleGetSession returns the full session history
func (gs *GatewayServer) handleGetSession(w http.ResponseWriter, r *http.Request, sessionKey string) {
	sessions := gs.agentManager.GetSessions()
	if sessions == nil {
		writeError(w, http.StatusNotFound, "session not found", "invalid_request_error")
		return
	}

	session := sessions.GetSession(sessionKey)
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found: "+sessionKey, "invalid_request_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// handleSessionNew clears and creates a new session
func (gs *GatewayServer) handleSessionNew(w http.ResponseWriter, r *http.Request, sessionKey string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	// Extract agent from session key (web:agentName)
	agentName := "default"
	if strings.HasPrefix(sessionKey, "web:") {
		agentName = strings.TrimPrefix(sessionKey, "web:")
	}

	gs.agentManager.ClearSession(sessionKey, agentName)

	logger.InfoCF("gateway", "Session cleared", map[string]interface{}{
		"session_key": sessionKey,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"session_key": sessionKey,
		"message":     "Session cleared. Starting fresh conversation.",
	})
}

// handleSessionStop stops in-flight processing for a session
func (gs *GatewayServer) handleSessionStop(w http.ResponseWriter, r *http.Request, sessionKey string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	result := gs.agentManager.StopSession(sessionKey)

	logger.InfoCF("gateway", "Session stop requested", map[string]interface{}{
		"session_key": sessionKey,
		"result":      result,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": result,
	})
}

// handleDeleteSession deletes a specific session
func (gs *GatewayServer) handleDeleteSession(w http.ResponseWriter, r *http.Request, sessionKey string) {
	agentName := "default"
	if strings.HasPrefix(sessionKey, "web:") {
		agentName = strings.TrimPrefix(sessionKey, "web:")
	}

	gs.agentManager.ClearSession(sessionKey, agentName)

	logger.InfoCF("gateway", "Session deleted", map[string]interface{}{
		"session_key": sessionKey,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"session_key": sessionKey,
		"message":     "Session deleted.",
	})
}

// handleListAgents returns raw registry.json content
func (gs *GatewayServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	registryPath := filepath.Join(gs.config.WorkspacePath(), "agents", "registry.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		// Fall back to listing from agent manager
		agents := gs.agentManager.ListAgents()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleListSkills lists all available skills
func (gs *GatewayServer) handleListSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	workspace := gs.config.WorkspacePath()
	type skillInfo struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		Description string `json:"description"`
		Available   bool   `json:"available"`
		Missing     string `json:"missing,omitempty"`
	}

	var skills []skillInfo

	// Read workspace skills
	skillsDirs := []struct {
		path   string
		source string
	}{
		{filepath.Join(workspace, "skills"), "workspace"},
	}

	// Also check builtin skills path
	builtinPath := filepath.Join(filepath.Dir(workspace), "skills-builtin")
	if _, err := os.Stat(builtinPath); err == nil {
		skillsDirs = append(skillsDirs, struct {
			path   string
			source string
		}{builtinPath, "builtin"})
	}

	for _, sd := range skillsDirs {
		dirs, err := os.ReadDir(sd.path)
		if err != nil {
			continue
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			skillFile := filepath.Join(sd.path, dir.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}

			info := skillInfo{
				Name:      dir.Name(),
				Source:    sd.source,
				Available: true,
			}

			// Try to read frontmatter for description
			content, err := os.ReadFile(skillFile)
			if err == nil {
				// Simple frontmatter extraction
				text := string(content)
				if strings.HasPrefix(text, "---\n") {
					if end := strings.Index(text[4:], "\n---"); end != -1 {
						frontmatter := text[4 : 4+end]
						// Parse JSON frontmatter
						var meta struct {
							Name        string `json:"name"`
							Description string `json:"description"`
						}
						if json.Unmarshal([]byte(frontmatter), &meta) == nil {
							info.Description = meta.Description
						}
					}
				}
			}

			skills = append(skills, info)
		}
	}

	if skills == nil {
		skills = []skillInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
	})
}

// findSkillPath resolves a skill name to its directory path
func (gs *GatewayServer) findSkillPath(name string) string {
	workspace := gs.config.WorkspacePath()
	// Check workspace skills first
	p := filepath.Join(workspace, "skills", name)
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}
	// Check builtin skills
	p = filepath.Join(filepath.Dir(workspace), "skills-builtin", name)
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}
	return ""
}

// handleSkillRoutes handles /v1/skills/{name} and /v1/skills/{name}/{path...}
func (gs *GatewayServer) handleSkillRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	// Parse: /v1/skills/{name} or /v1/skills/{name}/{path...}
	rest := strings.TrimPrefix(r.URL.Path, "/v1/skills/")
	if rest == "" {
		writeError(w, http.StatusBadRequest, "skill name required", "invalid_request_error")
		return
	}

	// Split into name and optional file path
	parts := strings.SplitN(rest, "/", 2)
	skillName := parts[0]
	filePath := ""
	if len(parts) > 1 {
		filePath = parts[1]
	}

	skillDir := gs.findSkillPath(skillName)
	if skillDir == "" {
		writeError(w, http.StatusNotFound, "skill not found: "+skillName, "not_found")
		return
	}

	if r.Method == http.MethodPost {
		if filePath == "" {
			writeError(w, http.StatusBadRequest, "file path required for POST", "invalid_request_error")
			return
		}
		gs.handleSkillFileSave(w, r, skillDir, filePath)
		return
	}

	if filePath == "" {
		gs.handleSkillFileList(w, skillDir, skillName)
	} else {
		gs.handleSkillFileContent(w, skillDir, filePath)
	}
}

// handleSkillFileList returns a recursive file tree for a skill
func (gs *GatewayServer) handleSkillFileList(w http.ResponseWriter, skillDir, skillName string) {
	type fileEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}

	var files []fileEntry

	filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Get relative path
		rel, _ := filepath.Rel(skillDir, path)
		if rel == "." {
			return nil
		}
		// Skip hidden files/dirs
		if strings.HasPrefix(filepath.Base(rel), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		files = append(files, fileEntry{
			Name:  filepath.Base(rel),
			Path:  rel,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})
		return nil
	})

	if files == nil {
		files = []fileEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skill": skillName,
		"files": files,
	})
}

// handleSkillFileContent returns the raw content of a file within a skill
func (gs *GatewayServer) handleSkillFileContent(w http.ResponseWriter, skillDir, filePath string) {
	// Sanitize path to prevent directory traversal
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		writeError(w, http.StatusBadRequest, "invalid path", "invalid_request_error")
		return
	}

	fullPath := filepath.Join(skillDir, cleanPath)

	// Ensure the resolved path is still within the skill directory
	absSkillDir, _ := filepath.Abs(skillDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFullPath, absSkillDir) {
		writeError(w, http.StatusForbidden, "access denied", "permission_error")
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found: "+filePath, "not_found")
		return
	}

	// Detect content type
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "text/plain; charset=utf-8"
	switch ext {
	case ".json":
		contentType = "application/json"
	case ".md":
		contentType = "text/markdown; charset=utf-8"
	case ".py":
		contentType = "text/x-python; charset=utf-8"
	case ".js":
		contentType = "text/javascript; charset=utf-8"
	case ".sh", ".bash":
		contentType = "text/x-shellscript; charset=utf-8"
	case ".yaml", ".yml":
		contentType = "text/yaml; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

// handleSkillFileSave writes content to a file within a skill
func (gs *GatewayServer) handleSkillFileSave(w http.ResponseWriter, r *http.Request, skillDir, filePath string) {
	// Sanitize path to prevent directory traversal
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		writeError(w, http.StatusBadRequest, "invalid path", "invalid_request_error")
		return
	}

	fullPath := filepath.Join(skillDir, cleanPath)

	// Ensure the resolved path is still within the skill directory
	absSkillDir, _ := filepath.Abs(skillDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFullPath, absSkillDir) {
		writeError(w, http.StatusForbidden, "access denied", "permission_error")
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body", "invalid_request_error")
		return
	}
	defer r.Body.Close()

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory: "+err.Error(), "server_error")
		return
	}

	if err := os.WriteFile(fullPath, body, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file: "+err.Error(), "server_error")
		return
	}

	logger.InfoC("gateway", fmt.Sprintf("Skill file updated via dashboard: %s/%s", filepath.Base(skillDir), filePath))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "File saved",
		"path":    filePath,
	})
}

// handleListWorkflows lists all available workflows
func (gs *GatewayServer) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	workspace := gs.config.WorkspacePath()
	workflowsDir := filepath.Join(workspace, "workflows")

	type workflowInfo struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		StepCount   int               `json:"step_count"`
		Variables   map[string]string `json:"variables,omitempty"`
	}

	var workflows []workflowInfo

	dirs, err := os.ReadDir(workflowsDir)
	if err == nil {
		for _, file := range dirs {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(workflowsDir, file.Name()))
			if err != nil {
				continue
			}

			var wf struct {
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Variables   map[string]string `json:"variables,omitempty"`
				Steps       []interface{}     `json:"steps"`
			}
			if json.Unmarshal(data, &wf) != nil {
				continue
			}

			name := strings.TrimSuffix(file.Name(), ".json")
			if wf.Name != "" {
				name = wf.Name
			}

			workflows = append(workflows, workflowInfo{
				Name:        name,
				Description: wf.Description,
				StepCount:   len(wf.Steps),
				Variables:   wf.Variables,
			})
		}
	}

	if workflows == nil {
		workflows = []workflowInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": workflows,
	})
}

// handleGetWorkflow returns a full workflow definition
func (gs *GatewayServer) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/v1/workflows/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "workflow name required", "invalid_request_error")
		return
	}

	workspace := gs.config.WorkspacePath()
	filePath := filepath.Join(workspace, "workflows", name+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "workflow not found", "not_found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// configPath returns the path to config.json
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pepebot", "config.json")
}

// handleConfig handles GET (read) and PUT (write) for config.json
func (gs *GatewayServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gs.handleGetConfig(w, r)
	case http.MethodPut:
		gs.handlePutConfig(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
	}
}

// handleGetConfig reads config.json and returns it (with masked API keys)
func (gs *GatewayServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		writeError(w, http.StatusNotFound, "config not found", "not_found")
		return
	}

	// Parse to map so we can mask API keys
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse config", "server_error")
		return
	}

	// Mask API keys for security
	maskAPIKeys(cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

// handlePutConfig saves the provided JSON to config.json
func (gs *GatewayServer) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error(), "invalid_request_error")
		return
	}

	// Read current config to preserve masked fields
	currentData, err := os.ReadFile(configPath())
	if err == nil {
		var currentConfig map[string]interface{}
		if json.Unmarshal(currentData, &currentConfig) == nil {
			// Restore masked API keys from current config
			restoreMaskedKeys(newConfig, currentConfig)
		}
	}

	// Write with pretty formatting
	data, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config", "server_error")
		return
	}

	if err := os.WriteFile(configPath(), data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error(), "server_error")
		return
	}

	logger.InfoC("gateway", "Config updated via dashboard")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Configuration saved. Restart gateway to apply changes.",
	})
}

// maskAPIKeys masks sensitive fields in config for display
func maskAPIKeys(obj map[string]interface{}) {
	for key, val := range obj {
		switch v := val.(type) {
		case map[string]interface{}:
			maskAPIKeys(v)
		case string:
			if (strings.Contains(key, "api_key") || strings.Contains(key, "token") || strings.Contains(key, "secret")) && v != "" {
				if len(v) > 8 {
					obj[key] = v[:4] + "****" + v[len(v)-4:]
				} else {
					obj[key] = "****"
				}
			}
		}
	}
}

// restoreMaskedKeys replaces masked values (containing ****) with originals from current config
func restoreMaskedKeys(newCfg, currentCfg map[string]interface{}) {
	for key, newVal := range newCfg {
		currentVal, exists := currentCfg[key]
		if !exists {
			continue
		}

		switch nv := newVal.(type) {
		case map[string]interface{}:
			if cv, ok := currentVal.(map[string]interface{}); ok {
				restoreMaskedKeys(nv, cv)
			}
		case string:
			if strings.Contains(nv, "****") {
				if cv, ok := currentVal.(string); ok {
					newCfg[key] = cv
				}
			}
		}
	}
}

// writeError writes a JSON error response
func writeError(w http.ResponseWriter, statusCode int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errType,
			Code:    http.StatusText(statusCode),
		},
	})
}

// writeSSEChunk writes a single SSE data line
func writeSSEChunk(w http.ResponseWriter, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
}
