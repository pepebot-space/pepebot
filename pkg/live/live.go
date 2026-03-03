// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// LiveProvider abstracts a real-time streaming AI backend (e.g. Vertex AI Live API)
type LiveProvider interface {
	// BuildUpstreamURL returns the WSS endpoint for the given model
	BuildUpstreamURL(model string) string
	// AuthHeaders returns headers needed to authenticate with upstream
	AuthHeaders() (http.Header, error)
	// SetupMessage returns the initial setup message to send to upstream after connecting.
	// Return nil if no setup message is needed (e.g. OpenAI handles setup via URL params).
	SetupMessage(model string) []byte
	// Name returns the provider name (e.g. "vertex")
	Name() string
}

// ToolExecutor executes local tools for live tool-calling sessions.
type ToolExecutor interface {
	GetToolDefinitions(agentName string) ([]map[string]interface{}, error)
	ExecuteTool(ctx context.Context, agentName, toolName string, args map[string]interface{}) (string, error)
}

// SetupMessage is the first message sent by the client to configure the session
type SetupMessage struct {
	Setup *SetupConfig `json:"setup,omitempty"`
}

// SetupConfig contains model and provider selection for a live session
type SetupConfig struct {
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
	Agent    string `json:"agent,omitempty"`
	// EnableTools controls whether gateway-side tool execution is enabled for the session.
	// Defaults to true when omitted.
	EnableTools *bool `json:"enable_tools,omitempty"`
}

// LiveServer manages WebSocket live sessions
type LiveServer struct {
	providers map[string]LiveProvider
	config    *config.Config
	tools     ToolExecutor
	upgrader  websocket.Upgrader
	mu        sync.RWMutex
	sessions  []*LiveSession
}

// LiveSession represents an active bidirectional proxy session
type LiveSession struct {
	clientConn   *websocket.Conn
	upstreamConn *websocket.Conn
	cancel       context.CancelFunc
	provider     string
	model        string
	agent        string
	enableTools  bool
	createdAt    time.Time
	upstreamMu   sync.Mutex
}

// NewLiveServer creates a new live API server
func NewLiveServer(cfg *config.Config) *LiveServer {
	return &LiveServer{
		providers: make(map[string]LiveProvider),
		config:    cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins (same as gateway CORS policy)
			},
			ReadBufferSize:  16 * 1024,
			WriteBufferSize: 16 * 1024,
		},
	}
}

// RegisterProvider registers a LiveProvider by name
func (ls *LiveServer) RegisterProvider(name string, provider LiveProvider) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.providers[name] = provider
	logger.InfoCF("live", "Registered live provider", map[string]interface{}{
		"provider": name,
	})
}

// GetProvider returns a registered provider by name
func (ls *LiveServer) GetProvider(name string) (LiveProvider, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	p, ok := ls.providers[name]
	return p, ok
}

// SetToolExecutor registers an executor for live tool calls.
func (ls *LiveServer) SetToolExecutor(executor ToolExecutor) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.tools = executor
}

// HandleWebSocket is the HTTP handler for /v1/live WebSocket upgrades
func (ls *LiveServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	clientConn, err := ls.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.ErrorCF("live", "WebSocket upgrade failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	logger.InfoC("live", "New WebSocket connection")

	go ls.handleConnection(clientConn)
}

// handleConnection manages a single client WebSocket connection
func (ls *LiveServer) handleConnection(clientConn *websocket.Conn) {
	defer clientConn.Close()

	// Step 1: Read setup message (with timeout)
	clientConn.SetReadDeadline(time.Now().Add(10 * time.Second))

	_, rawMsg, err := clientConn.ReadMessage()
	if err != nil {
		logger.ErrorCF("live", "Failed to read setup message", map[string]interface{}{
			"error": err.Error(),
		})
		ls.sendError(clientConn, "Failed to read setup message: "+err.Error())
		return
	}

	// Reset read deadline
	clientConn.SetReadDeadline(time.Time{})

	var setupMsg SetupMessage
	if err := json.Unmarshal(rawMsg, &setupMsg); err != nil || setupMsg.Setup == nil {
		// Not a setup message — use config defaults
		setupMsg.Setup = &SetupConfig{}
	}

	// Determine provider and model from setup or config defaults
	providerName := setupMsg.Setup.Provider
	if providerName == "" {
		providerName = ls.config.Live.Provider
	}
	if providerName == "" {
		providerName = "vertex"
	}

	model := setupMsg.Setup.Model
	if model == "" {
		model = ls.config.Live.Model
	}
	if model == "" {
		model = "gemini-live-2.5-flash-native-audio"
	}

	agentName := setupMsg.Setup.Agent
	if agentName == "" {
		agentName = "default"
	}

	enableTools := true
	if setupMsg.Setup.EnableTools != nil {
		enableTools = *setupMsg.Setup.EnableTools
	}

	logger.InfoCF("live", "Session setup", map[string]interface{}{
		"provider": providerName,
		"model":    model,
		"agent":    agentName,
		"tools":    enableTools,
	})

	// Step 2: Resolve provider
	provider, ok := ls.GetProvider(providerName)
	if !ok {
		errMsg := fmt.Sprintf("Live provider '%s' not available", providerName)
		logger.ErrorC("live", errMsg)
		ls.sendError(clientConn, errMsg)
		return
	}

	// Step 3: Get auth headers
	headers, err := provider.AuthHeaders()
	if err != nil {
		logger.ErrorCF("live", "Failed to get auth headers", map[string]interface{}{
			"error": err.Error(),
		})
		ls.sendError(clientConn, "Authentication failed: "+err.Error())
		return
	}

	// Step 4: Connect to upstream
	upstreamURL := provider.BuildUpstreamURL(model)
	logger.InfoCF("live", "Connecting to upstream", map[string]interface{}{
		"url": upstreamURL,
	})

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	upstreamConn, resp, err := dialer.Dial(upstreamURL, headers)
	if err != nil {
		errDetail := err.Error()
		if resp != nil {
			errDetail = fmt.Sprintf("%s (HTTP %d)", errDetail, resp.StatusCode)
		}
		logger.ErrorCF("live", "Failed to connect to upstream", map[string]interface{}{
			"error": errDetail,
			"url":   upstreamURL,
		})
		ls.sendError(clientConn, "Upstream connection failed: "+errDetail)
		return
	}
	defer upstreamConn.Close()

	logger.InfoCF("live", "Upstream connected", map[string]interface{}{
		"provider": providerName,
		"model":    model,
	})

	// Step 5: Send provider-specific setup message to upstream (e.g. BidiGenerateContentSetup for Vertex)
	setupData := provider.SetupMessage(model)
	if enableTools && ls.tools != nil && setupData != nil {
		if toolDefs, err := ls.tools.GetToolDefinitions(agentName); err != nil {
			logger.WarnCF("live", "Failed to load tool definitions for live session", map[string]interface{}{
				"agent": agentName,
				"error": err.Error(),
			})
		} else {
			setupData = injectGeminiToolsIntoSetup(setupData, toolDefs)
		}
	}

	if setupData != nil {
		if err := upstreamConn.WriteMessage(websocket.TextMessage, setupData); err != nil {
			logger.ErrorCF("live", "Failed to send upstream setup message", map[string]interface{}{
				"error": err.Error(),
			})
			ls.sendError(clientConn, "Upstream setup failed: "+err.Error())
			return
		}
		logger.InfoCF("live", "Sent upstream setup message", map[string]interface{}{
			"provider": providerName,
			"model":    model,
		})
	}

	// Step 6: Create session and start bidirectional proxy
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := &LiveSession{
		clientConn:   clientConn,
		upstreamConn: upstreamConn,
		cancel:       cancel,
		provider:     providerName,
		model:        model,
		agent:        agentName,
		enableTools:  enableTools,
		createdAt:    time.Now(),
	}

	ls.addSession(session)
	defer ls.removeSession(session)

	// Send confirmation to client
	confirmMsg, _ := json.Marshal(map[string]interface{}{
		"status":   "connected",
		"provider": providerName,
		"model":    model,
	})
	clientConn.WriteMessage(websocket.TextMessage, confirmMsg)

	// Run bidirectional proxy
	var wg sync.WaitGroup
	wg.Add(2)

	// Client → Upstream
	go func() {
		defer wg.Done()
		ls.proxyMessages(ctx, session, clientConn, upstreamConn, "client→upstream")
		cancel() // If client closes, cancel the context
	}()

	// Upstream → Client
	go func() {
		defer wg.Done()
		ls.proxyMessages(ctx, session, upstreamConn, clientConn, "upstream→client")
		cancel() // If upstream closes, cancel the context
	}()

	wg.Wait()

	logger.InfoCF("live", "Session ended", map[string]interface{}{
		"provider": providerName,
		"model":    model,
		"duration": time.Since(session.createdAt).String(),
	})
}

// proxyMessages forwards messages from src to dst until context is cancelled or connection closes
func (ls *LiveServer) proxyMessages(ctx context.Context, session *LiveSession, src, dst *websocket.Conn, direction string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := src.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.DebugCF("live", "Connection closed normally", map[string]interface{}{
					"direction": direction,
				})
			} else {
				logger.DebugCF("live", "Read error", map[string]interface{}{
					"direction": direction,
					"error":     err.Error(),
				})
			}
			return
		}

		if direction == "upstream→client" {
			ls.handleUpstreamToolCalls(ctx, session, msgType, data)
		}

		var writeErr error
		if direction == "client→upstream" {
			session.upstreamMu.Lock()
			writeErr = dst.WriteMessage(msgType, data)
			session.upstreamMu.Unlock()
		} else {
			writeErr = dst.WriteMessage(msgType, data)
		}

		if writeErr != nil {
			logger.DebugCF("live", "Write error", map[string]interface{}{
				"direction": direction,
				"error":     writeErr.Error(),
			})
			return
		}
	}
}

func (ls *LiveServer) handleUpstreamToolCalls(ctx context.Context, session *LiveSession, msgType int, data []byte) {
	if !session.enableTools || ls.tools == nil {
		return
	}
	if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	toolCallRaw, ok := payload["toolCall"]
	if !ok {
		return
	}

	toolCall, ok := toolCallRaw.(map[string]interface{})
	if !ok {
		return
	}

	functionCalls, ok := toolCall["functionCalls"].([]interface{})
	if !ok || len(functionCalls) == 0 {
		return
	}

	responses := make([]map[string]interface{}, 0, len(functionCalls))
	for _, fcRaw := range functionCalls {
		fc, ok := fcRaw.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := fc["id"].(string)
		name, _ := fc["name"].(string)
		if name == "" {
			continue
		}

		args := map[string]interface{}{}
		if v, ok := fc["args"].(map[string]interface{}); ok {
			args = v
		}

		toolCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		result, err := ls.tools.ExecuteTool(toolCtx, session.agent, name, args)
		cancel()

		resultPayload := map[string]interface{}{}
		if err != nil {
			resultPayload["error"] = err.Error()
		} else if strings.TrimSpace(result) == "" {
			resultPayload["result"] = ""
		} else {
			var anyJSON interface{}
			if json.Unmarshal([]byte(result), &anyJSON) == nil {
				resultPayload["result"] = anyJSON
			} else {
				resultPayload["result"] = result
			}
		}

		responses = append(responses, map[string]interface{}{
			"id":       id,
			"name":     name,
			"response": resultPayload,
		})
	}

	if len(responses) == 0 {
		return
	}

	msg, err := json.Marshal(map[string]interface{}{
		"toolResponse": map[string]interface{}{
			"functionResponses": responses,
		},
	})
	if err != nil {
		return
	}

	session.upstreamMu.Lock()
	defer session.upstreamMu.Unlock()
	if err := session.upstreamConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		logger.WarnCF("live", "Failed to write toolResponse upstream", map[string]interface{}{
			"agent": session.agent,
			"error": err.Error(),
		})
	}
}

func injectGeminiToolsIntoSetup(setupData []byte, toolDefs []map[string]interface{}) []byte {
	if len(setupData) == 0 || len(toolDefs) == 0 {
		return setupData
	}

	var setup map[string]interface{}
	if err := json.Unmarshal(setupData, &setup); err != nil {
		return setupData
	}

	setupInner, ok := setup["setup"].(map[string]interface{})
	if !ok {
		return setupData
	}

	functionDecls := make([]map[string]interface{}, 0, len(toolDefs))
	for _, td := range toolDefs {
		fn, ok := td["function"].(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params, _ := fn["parameters"].(map[string]interface{})
		if name == "" {
			continue
		}

		decl := map[string]interface{}{
			"name":        name,
			"description": desc,
		}
		if params != nil {
			decl["parameters"] = params
		}
		functionDecls = append(functionDecls, decl)
	}

	if len(functionDecls) == 0 {
		return setupData
	}

	setupInner["tools"] = []map[string]interface{}{
		{"functionDeclarations": functionDecls},
	}

	b, err := json.Marshal(setup)
	if err != nil {
		return setupData
	}
	return b
}

// sendError sends an error message to the client WebSocket and closes the connection
func (ls *LiveServer) sendError(conn *websocket.Conn, errMsg string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"error": errMsg,
	})
	conn.WriteMessage(websocket.TextMessage, msg)
	conn.Close()
}

// addSession tracks an active session
func (ls *LiveServer) addSession(session *LiveSession) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.sessions = append(ls.sessions, session)
}

// removeSession removes a session from tracking
func (ls *LiveServer) removeSession(session *LiveSession) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	for i, s := range ls.sessions {
		if s == session {
			ls.sessions = append(ls.sessions[:i], ls.sessions[i+1:]...)
			break
		}
	}
}

// ActiveSessions returns the number of active live sessions
func (ls *LiveServer) ActiveSessions() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return len(ls.sessions)
}
