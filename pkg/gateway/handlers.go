package gateway

import (
	"encoding/json"
	"fmt"
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
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Stream      bool              `json:"stream"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []ChatCompletionChoice   `json:"choices"`
	Usage   *UsageResponse           `json:"usage,omitempty"`
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
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []StreamChunkChoice  `json:"choices"`
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

	logger.DebugCF("gateway", "Chat completion request", map[string]interface{}{
		"agent":       agentName,
		"session_key": sessionKey,
		"stream":      req.Stream,
		"model":       req.Model,
	})

	completionID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	if req.Stream {
		gs.handleStreamingResponse(w, r, lastMessage.Content, sessionKey, agentName, req.Model, completionID)
	} else {
		gs.handleNonStreamingResponse(w, r, lastMessage.Content, sessionKey, agentName, req.Model, completionID)
	}
}

// handleNonStreamingResponse handles non-streaming chat completions
func (gs *GatewayServer) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, content, sessionKey, agentName, model, completionID string) {
	ctx := r.Context()

	response, err := gs.agentManager.ProcessDirect(ctx, content, sessionKey, agentName)
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
func (gs *GatewayServer) handleStreamingResponse(w http.ResponseWriter, r *http.Request, content, sessionKey, agentName, model, completionID string) {
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

	err := gs.agentManager.ProcessDirectStream(ctx, content, sessionKey, agentName, func(chunk providers.StreamChunk) {
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

	// Direct session key - DELETE to delete
	sessionKey := path
	if r.Method == http.MethodDelete {
		gs.handleDeleteSession(w, r, sessionKey)
		return
	}

	writeError(w, http.StatusNotFound, "not found", "invalid_request_error")
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
