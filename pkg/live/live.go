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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/tools"
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

// SystemPromptSource optionally supplies an agent's persona prompt for use as the
// SkillsSource supplies the agent's skills block for a Live session. Separate from
// SystemPromptSource so skills are included no matter which prompt source wins.
type SkillsSource interface {
	LiveSkillsPrompt(agentName string) string
}

// SessionHistory supplies what was said earlier under a session key, so a live session
// can pick a conversation back up instead of starting blank every time. Turns are plain
// {"role","content"} maps, oldest first — the same shape ToolExecutor uses for tool
// definitions, so pkg/agent does not have to import this package. A ToolExecutor may
// implement it; when it does not, sessions simply start fresh.
type SessionHistory interface {
	LiveHistory(sessionKey string, limit int) []map[string]string
}

// SessionRecorder persists live conversation turns into the agent's session history,
// so a voice conversation and a text one share the same memory. A ToolExecutor may
// implement it; when it does not, live sessions simply are not recorded.
type SessionRecorder interface {
	RecordLiveTurn(sessionKey, role, content string)
}

// Live systemInstruction. A ToolExecutor may implement it; resolution is opt-in
// via live.use_agent_prompt and only used when no explicit prompt is set.
type SystemPromptSource interface {
	LiveSystemPrompt(agentName string) string
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
	// SessionKey controls persistent context keys for downstream tool calls.
	SessionKey string `json:"session_key,omitempty"`
	// EnableTools controls whether gateway-side tool execution is enabled for the session.
	// Defaults to true when omitted.
	EnableTools *bool `json:"enable_tools,omitempty"`
	// SystemPrompt sets the session systemInstruction (highest precedence override).
	SystemPrompt string `json:"system_prompt,omitempty"`
	// Language is a BCP-47 code (e.g. "id-ID") the model should reply in. Overrides
	// live.language for this session. Applied on the OpenAI Realtime protocol, where
	// the only lever is the instructions text.
	Language string `json:"language,omitempty"`
	// App namespaces the tools this client declares. Required when Tools is set, so a
	// client can never shadow one of the gateway's own tools.
	App string `json:"app,omitempty"`
	// Tools are declared by the client and executed on the client. Same flat shape the
	// Realtime API uses: {name, description, parameters}. The model sees them as
	// "<app>-<name>"; the client is asked for them by the bare name it declared.
	Tools []map[string]interface{} `json:"tools,omitempty"`
	// ClientToolTimeoutMs bounds the wait for a client tool result (default 30s,
	// capped at 90s to match gateway tool execution).
	ClientToolTimeoutMs int `json:"client_tool_timeout_ms,omitempty"`
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
	sessionKey   string
	enableTools  bool
	createdAt    time.Time
	upstreamMu   sync.Mutex

	// Everything a reconnect needs to rebuild the upstream leg identically.
	liveProvider LiveProvider
	setupFrames  [][]byte

	// Tools the client declared and executes itself: upstream name -> the bare name
	// the client knows it by. Read-only after setup.
	clientTools       map[string]string
	clientToolTimeout time.Duration

	// Calls waiting on a client result, keyed by the upstream call id.
	pendingMu sync.Mutex
	pending   map[string]chan string
}

// clientTool reports whether a tool call belongs to the client, and under which name
// the client declared it.
func (s *LiveSession) clientTool(name string) (string, bool) {
	bare, ok := s.clientTools[name]
	return bare, ok
}

// upstream returns the current upstream connection, which changes on a reconnect.
func (s *LiveSession) upstream() *websocket.Conn {
	s.upstreamMu.Lock()
	defer s.upstreamMu.Unlock()
	return s.upstreamConn
}

// writeUpstream serializes writes against tool results and reconnects.
func (s *LiveSession) writeUpstream(msgType int, data []byte) error {
	s.upstreamMu.Lock()
	defer s.upstreamMu.Unlock()
	return s.upstreamConn.WriteMessage(msgType, data)
}

func (s *LiveSession) replaceUpstream(conn *websocket.Conn) *websocket.Conn {
	s.upstreamMu.Lock()
	defer s.upstreamMu.Unlock()
	old := s.upstreamConn
	s.upstreamConn = conn
	return old
}

func supportsLiveVideo(provider string) bool {
	switch provider {
	case "vertex", "gemini":
		return true
	default:
		return false
	}
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

	sessionKey := strings.TrimSpace(setupMsg.Setup.SessionKey)
	if sessionKey == "" {
		sessionKey = fmt.Sprintf("live:%s:%s:%d", providerName, agentName, time.Now().UnixNano())
	}

	logger.InfoCF("live", "Session setup", map[string]interface{}{
		"provider": providerName,
		"model":    model,
		"agent":    agentName,
		"session":  sessionKey,
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

	videoRequested := ls.config.Live.Video
	videoSupported := supportsLiveVideo(providerName)
	videoEnabled := videoRequested && videoSupported
	if videoRequested && !videoSupported {
		logger.WarnCF("live", "Video requested but provider has no explicit video support", map[string]interface{}{
			"provider": providerName,
			"model":    model,
		})
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

	upstreamConn, resp, err := dialUpstream(upstreamURL, headers)
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
	logger.InfoCF("live", "Upstream connected", map[string]interface{}{
		"provider": providerName,
		"model":    model,
	})

	// Step 5: Send provider-specific setup message to upstream (e.g. BidiGenerateContentSetup for Vertex).
	// Whatever we send here is kept so a reconnect can replay it.
	var setupFrames [][]byte
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

	// Resolve the session systemInstruction with precedence:
	//   client setup.system_prompt > live.system_prompt(/_file) > agent persona.
	// SetupMessage already injects the config prompt; re-injecting is idempotent.
	// When nothing resolves, setupData is left untouched (byte-identical to before).
	sysPrompt := strings.TrimSpace(setupMsg.Setup.SystemPrompt)
	promptSource := "client"
	if sysPrompt == "" {
		sysPrompt = strings.TrimSpace(ls.config.Live.SystemPrompt)
		promptSource = "config"
	}
	if sysPrompt == "" && ls.config.Live.UseAgentPrompt && ls.tools != nil {
		if src, ok := ls.tools.(SystemPromptSource); ok {
			sysPrompt = strings.TrimSpace(src.LiveSystemPrompt(agentName))
			promptSource = "agent"
		}
	}
	if sysPrompt != "" && setupData != nil {
		setupData = injectGeminiSystemInstruction(setupData, sysPrompt)
		logger.InfoCF("live", "Applied system instruction", map[string]interface{}{
			"agent":  agentName,
			"source": promptSource,
			"chars":  len(sysPrompt),
		})
	}

	// Client tool registration happens inside the Realtime branch below; the session
	// needs it afterwards.
	var clientToolNames map[string]string
	clientToolTimeout := 30 * time.Second
	if ms := setupMsg.Setup.ClientToolTimeoutMs; ms > 0 {
		clientToolTimeout = time.Duration(ms) * time.Millisecond
		if clientToolTimeout > 90*time.Second {
			clientToolTimeout = 90 * time.Second
		}
	}

	// Providers on the OpenAI Realtime protocol have no setup frame — model selection
	// rides in the URL — so the persona and tools go up as a session.update instead.
	// Without this, both are silently dropped for those providers.
	if _, isOpenAI := provider.(*OpenAILiveProvider); isOpenAI {
		var toolDefs []map[string]interface{}
		if enableTools && ls.tools != nil {
			defs, err := ls.tools.GetToolDefinitions(agentName)
			if err != nil {
				logger.WarnCF("live", "Failed to load tool definitions for live session", map[string]interface{}{
					"agent": agentName,
					"error": err.Error(),
				})
			} else {
				toolDefs = defs
			}
		}
		lang := strings.TrimSpace(setupMsg.Setup.Language)
		if lang == "" {
			lang = strings.TrimSpace(ls.config.Live.Language)
		}
		// Client-declared tools ride alongside the gateway's, namespaced so they can
		// never shadow one. A bad declaration fails the session now rather than
		// leaving the model calling something that never answers.
		gatewayNames := make(map[string]bool, len(toolDefs))
		for _, def := range toolDefs {
			if fn, ok := def["function"].(map[string]interface{}); ok {
				if name, _ := fn["name"].(string); name != "" {
					gatewayNames[name] = true
				}
			} else if name, _ := def["name"].(string); name != "" {
				gatewayNames[name] = true
			}
		}

		clientDefs, names, err := namespaceClientTools(setupMsg.Setup.App, setupMsg.Setup.Tools, gatewayNames)
		if err != nil {
			logger.WarnCF("live", "Rejected client tool declaration", map[string]interface{}{
				"error": err.Error(),
			})
			ls.sendError(clientConn, err.Error())
			return
		}
		clientToolNames = names
		if len(clientDefs) > 0 {
			toolDefs = append(toolDefs, clientDefs...)
			logger.InfoCF("live", "Registered client tools", map[string]interface{}{
				"app":   setupMsg.Setup.App,
				"tools": len(clientDefs),
			})
		}

		skillsPrompt := ""
		if src, ok := ls.tools.(SkillsSource); ok {
			skillsPrompt = src.LiveSkillsPrompt(agentName)
		}
		prompt := liveInstructions(sysPrompt, skillsPrompt, lang)
		update := buildRealtimeSessionUpdate(prompt, toolDefs, ls.config.Live.RealtimeSession)
		if update != nil {
			setupFrames = append(setupFrames, update)
			if err := upstreamConn.WriteMessage(websocket.TextMessage, update); err != nil {
				logger.ErrorCF("live", "Failed to send upstream session.update", map[string]interface{}{
					"error": err.Error(),
				})
				ls.sendError(clientConn, "Upstream setup failed: "+err.Error())
				return
			}
			ls.replayHistory(upstreamConn, sessionKey)
			logger.InfoCF("live", "Sent upstream session.update", map[string]interface{}{
				"provider":      providerName,
				"tools":         len(toolDefs),
				"prompt_chars":  len(prompt),
				"prompt_source": promptSource,
				"skills_chars":  len(skillsPrompt),
				"language":      lang,
			})
		}
	}

	if setupData != nil {
		setupFrames = append(setupFrames, setupData)
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
		sessionKey:   sessionKey,
		enableTools:  enableTools,
		createdAt:    time.Now(),
		liveProvider: provider,
		setupFrames:  setupFrames,

		clientTools:       clientToolNames,
		clientToolTimeout: clientToolTimeout,
		pending:           map[string]chan string{},
	}

	ls.addSession(session)
	defer ls.removeSession(session)
	// Close whichever upstream connection is current — a reconnect swaps it.
	defer func() {
		if conn := session.upstream(); conn != nil {
			conn.Close()
		}
	}()

	// Send confirmation to client
	confirmMsg, _ := json.Marshal(map[string]interface{}{
		"status":   "connected",
		"provider": providerName,
		"model":    model,
		"session":  sessionKey,
		"video": map[string]interface{}{
			"requested": videoRequested,
			"supported": videoSupported,
			"enabled":   videoEnabled,
		},
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

func dialUpstream(url string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	return dialer.Dial(url, headers)
}

// reconnectUpstream rebuilds the upstream leg after it drops, without disturbing the
// client connection. The upstream conversation state is gone either way, so the setup
// frames (persona, tools, session config) are replayed and the client is told to
// treat the session as fresh.
func (ls *LiveServer) reconnectUpstream(ctx context.Context, session *LiveSession) bool {
	if session.liveProvider == nil {
		return false
	}

	const attempts = 3
	backoff := time.Second

	for attempt := 1; attempt <= attempts; attempt++ {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff *= 2

		headers, err := session.liveProvider.AuthHeaders()
		if err != nil {
			logger.WarnCF("live", "Reconnect auth failed", map[string]interface{}{"error": err.Error()})
			continue
		}

		conn, _, err := dialUpstream(session.liveProvider.BuildUpstreamURL(session.model), headers)
		if err != nil {
			logger.WarnCF("live", "Upstream reconnect failed", map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
			continue
		}

		failed := false
		for _, frame := range session.setupFrames {
			if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
				logger.WarnCF("live", "Reconnect setup replay failed", map[string]interface{}{"error": err.Error()})
				conn.Close()
				failed = true
				break
			}
		}
		if failed {
			continue
		}

		if old := session.replaceUpstream(conn); old != nil {
			old.Close()
		}

		logger.InfoCF("live", "Upstream reconnected", map[string]interface{}{
			"provider": session.provider,
			"model":    session.model,
			"attempt":  attempt,
			"frames":   len(session.setupFrames),
		})

		notice, _ := json.Marshal(map[string]interface{}{
			"status":   "reconnected",
			"provider": session.provider,
			"model":    session.model,
			"session":  session.sessionKey,
			"note":     "upstream conversation state was reset",
		})
		session.clientConn.WriteMessage(websocket.TextMessage, notice)
		return true
	}

	return false
}

// proxyMessages forwards messages from src to dst until context is cancelled or connection closes
func (ls *LiveServer) proxyMessages(ctx context.Context, session *LiveSession, src, dst *websocket.Conn, direction string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if direction == "upstream→client" {
			// A reconnect swaps the connection under us, so re-read it each pass.
			src = session.upstream()
		}

		msgType, data, err := src.ReadMessage()
		if err != nil {
			normalClose := websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
			if normalClose {
				logger.DebugCF("live", "Connection closed normally", map[string]interface{}{
					"direction": direction,
				})
			} else {
				logger.DebugCF("live", "Read error", map[string]interface{}{
					"direction": direction,
					"error":     err.Error(),
				})
			}

			// An upstream that drops mid-session is worth recovering from; the client
			// is still here. A client that hangs up ends the session, as before.
			if direction == "upstream→client" && !normalClose && ctx.Err() == nil {
				if ls.reconnectUpstream(ctx, session) {
					continue
				}
			}
			return
		}

		// A tool_result answers a client tool; it is consumed here and never forwarded,
		// because upstream only ever sees the function_call_output built from it.
		if direction == "client→upstream" && ls.handleClientToolResult(session, msgType, data) {
			continue
		}

		ls.recordTurn(session, direction, msgType, data)

		if direction == "upstream→client" {
			// Run tool calls off the proxy loop so a slow tool (up to 90s) never
			// stalls forwarding of audio/video frames. The toolResponse write is
			// serialized via session.upstreamMu, so concurrent writes stay safe.
			go ls.handleUpstreamToolCalls(ctx, session, msgType, data)
		}

		var writeErr error
		if direction == "client→upstream" {
			writeErr = session.writeUpstream(msgType, data)
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

// recordTurn writes finished conversation turns into the agent's session history.
// Only the OpenAI Realtime protocol is covered: it transcribes both directions by
// default, while Gemini live sessions do not request transcription at all, so there
// is nothing there to record.
func (ls *LiveServer) recordTurn(session *LiveSession, direction string, msgType int, data []byte) {
	recorder, ok := ls.tools.(SessionRecorder)
	if !ok || session.sessionKey == "" {
		return
	}
	if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	role, text := "", ""
	switch payload["type"] {
	case "conversation.item.input_audio_transcription.completed":
		// What the user said, as heard by the server.
		role, text = "user", stringField(payload, "transcript")
	case "response.audio_transcript.done":
		// What the assistant said. Text-only replies also carry a transcript here.
		role, text = "assistant", stringField(payload, "transcript")
	case "conversation.item.create":
		// A typed turn never passes through transcription, so catch it on the way out.
		if direction != "client→upstream" {
			return
		}
		item, _ := payload["item"].(map[string]interface{})
		if item == nil || item["type"] == "function_call_output" {
			return
		}
		role, text = "user", inputTextOf(item)
	default:
		return
	}

	if text = strings.TrimSpace(text); text == "" {
		return
	}

	recorder.RecordLiveTurn(session.sessionKey, role, text)
	logger.DebugCF("live", "Recorded live turn", map[string]interface{}{
		"session": session.sessionKey,
		"role":    role,
		"chars":   len(text),
	})
}

func stringField(payload map[string]interface{}, key string) string {
	v, _ := payload[key].(string)
	return v
}

// inputTextOf pulls the text out of a conversation item's content blocks.
func inputTextOf(item map[string]interface{}) string {
	blocks, _ := item["content"].([]interface{})
	var parts []string
	for _, raw := range blocks {
		block, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if t := block["type"]; t != "input_text" && t != "text" {
			continue
		}
		if text, _ := block["text"].(string); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
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

	// OpenAI Realtime reports a call as one response.output_item.done carrying the
	// name, call_id and complete arguments; Gemini uses a toolCall envelope.
	if payload["type"] == "response.output_item.done" {
		ls.handleRealtimeToolCall(ctx, session, payload)
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
		toolCtx = tools.WithSessionKey(toolCtx, session.sessionKey)
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

// handleRealtimeToolCall executes one OpenAI Realtime function call and feeds the
// result back as a function_call_output item, then asks for the follow-up response.
func (ls *LiveServer) handleRealtimeToolCall(ctx context.Context, session *LiveSession, payload map[string]interface{}) {
	item, ok := payload["item"].(map[string]interface{})
	if !ok || item["type"] != "function_call" {
		return
	}

	name, _ := item["name"].(string)
	callID, _ := item["call_id"].(string)
	if name == "" || callID == "" {
		return
	}

	// arguments arrive as a JSON string, and an argument-less call may send "" or {}.
	args := map[string]interface{}{}
	if raw, _ := item["arguments"].(string); strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			logger.WarnCF("live", "Bad tool call arguments", map[string]interface{}{
				"tool":  name,
				"error": err.Error(),
			})
			args = map[string]interface{}{}
		}
	}

	var output string
	var err error

	if bare, isClient := session.clientTool(name); isClient {
		// The client owns this one: ask it, and pass whatever it says straight through.
		output = ls.callClientTool(ctx, session, callID, bare, args)
	} else {
		toolCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		toolCtx = tools.WithSessionKey(toolCtx, session.sessionKey)
		var result string
		result, err = ls.tools.ExecuteTool(toolCtx, session.agent, name, args)
		cancel()

		output = result
		if err != nil {
			output = "Error: " + err.Error()
		}
	}

	logger.InfoCF("live", "Executed live tool call", map[string]interface{}{
		"agent":   session.agent,
		"tool":    name,
		"call_id": callID,
		"failed":  err != nil,
		"chars":   len(output),
	})

	frames := make([][]byte, 0, 2)
	resultFrame, marshalErr := json.Marshal(map[string]interface{}{
		"type": "conversation.item.create",
		"item": map[string]interface{}{
			"type":    "function_call_output",
			"call_id": callID,
			"output":  output,
		},
	})
	if marshalErr != nil {
		return
	}
	frames = append(frames, resultFrame, []byte(`{"type":"response.create"}`))

	session.upstreamMu.Lock()
	defer session.upstreamMu.Unlock()
	for _, frame := range frames {
		if err := session.upstreamConn.WriteMessage(websocket.TextMessage, frame); err != nil {
			logger.WarnCF("live", "Failed to write tool result upstream", map[string]interface{}{
				"agent": session.agent,
				"tool":  name,
				"error": err.Error(),
			})
			return
		}
	}
}

// speechDirective keeps replies speakable. Realtime servers usually carry guidance
// like this in their own default instructions — and Pepebot's session.update replaces
// those wholesale, so without re-stating it the model happily emits markdown tables
// and emoji that a TTS voice then reads out character by character.
const speechDirective = "Your reply will be converted to speech and read aloud, never displayed. " +
	"Write it as plain spoken sentences: no markdown, no headings, no bullet or numbered lists, " +
	"no tables, no code blocks, no emoji, no asterisks or underscores for emphasis. " +
	"Say symbols and abbreviations as words. When you list several things, say them in one sentence " +
	"separated by commas. Keep it to a sentence or two unless more detail is asked for."

// liveInstructions assembles the session instructions: the agent persona, its skills,
// the rules that make the output speakable, then the requested reply language.
func liveInstructions(prompt, skills, languageCode string) string {
	parts := make([]string, 0, 4)
	if p := strings.TrimSpace(prompt); p != "" {
		parts = append(parts, p)
	}
	if s := strings.TrimSpace(skills); s != "" {
		parts = append(parts, s)
	}
	parts = append(parts, speechDirective)
	if directive := replyLanguageDirective(languageCode); directive != "" {
		parts = append(parts, directive)
	}
	return strings.Join(parts, "\n\n")
}

// replyLanguages maps the BCP-47 codes worth naming explicitly to a language name a
// model reliably understands. Anything else falls through as the code itself.
var replyLanguages = map[string]string{
	"id": "Indonesian", "en": "English", "jv": "Javanese", "su": "Sundanese",
	"ms": "Malay", "ja": "Japanese", "ko": "Korean", "zh": "Chinese",
	"ar": "Arabic", "es": "Spanish", "fr": "French", "de": "German",
}

// replyLanguageDirective renders the reply-language rule. The Realtime protocol has no
// language field — transcription and output language are whatever the server was built
// with — so instructions are the only lever.
func replyLanguageDirective(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	name, ok := replyLanguages[strings.ToLower(strings.SplitN(code, "-", 2)[0])]
	if !ok {
		name = code
	}
	return fmt.Sprintf("Always reply in %s.", name)
}

// Gateway tools are all snake_case, so joining an app name and a tool name with a
// hyphen keeps client tools structurally distinct and splittable back apart.
const clientToolSeparator = "-"

// Both halves are restricted to what cannot contain the separator, so "<app>-<tool>"
// always splits cleanly, and to what the Realtime tool-name grammar accepts.
var toolNameHalf = regexp.MustCompile(`^[A-Za-z0-9_]{1,48}$`)

// namespaceClientTools validates the tools a client declared and rewrites their names
// as "<app>-<tool>". Returns the upstream definitions and a map from the upstream name
// back to the bare name the client knows, or an error describing what to fix.
func namespaceClientTools(app string, decls []map[string]interface{}, gatewayNames map[string]bool) ([]map[string]interface{}, map[string]string, error) {
	if len(decls) == 0 {
		return nil, nil, nil
	}
	if !toolNameHalf.MatchString(app) {
		return nil, nil, fmt.Errorf("setup.app must be set to a short name matching %s when declaring tools", toolNameHalf)
	}

	defs := make([]map[string]interface{}, 0, len(decls))
	names := make(map[string]string, len(decls))

	for i, decl := range decls {
		bare, _ := decl["name"].(string)
		if !toolNameHalf.MatchString(bare) {
			return nil, nil, fmt.Errorf("setup.tools[%d].name %q must match %s", i, bare, toolNameHalf)
		}

		full := app + clientToolSeparator + bare
		if gatewayNames[full] {
			// Namespacing makes this near-impossible, but a silent shadow would leave
			// the model calling a tool that never answers.
			return nil, nil, fmt.Errorf("client tool %q collides with a gateway tool", full)
		}
		if _, dup := names[full]; dup {
			return nil, nil, fmt.Errorf("client tool %q declared twice", bare)
		}

		def := map[string]interface{}{"type": "function", "name": full}
		if desc, ok := decl["description"].(string); ok && desc != "" {
			def["description"] = desc
		}
		if params, ok := decl["parameters"]; ok && params != nil {
			def["parameters"] = params
		}
		defs = append(defs, def)
		names[full] = bare
	}

	return defs, names, nil
}

// callClientTool asks the client to run one of its own tools and waits for the answer.
// A device can be slow, backgrounded or gone, so the wait is bounded: on expiry the
// model is told the tool timed out rather than the turn hanging forever.
func (ls *LiveServer) callClientTool(ctx context.Context, session *LiveSession, callID, bare string, args map[string]interface{}) string {
	result := make(chan string, 1)

	session.pendingMu.Lock()
	if session.pending == nil {
		session.pending = map[string]chan string{}
	}
	session.pending[callID] = result
	session.pendingMu.Unlock()

	defer func() {
		session.pendingMu.Lock()
		delete(session.pending, callID)
		session.pendingMu.Unlock()
	}()

	frame, err := json.Marshal(map[string]interface{}{
		"type":      "tool_call",
		"call_id":   callID,
		"name":      bare,
		"arguments": args,
	})
	if err != nil {
		return "Error: could not encode the tool call"
	}
	if err := session.clientConn.WriteMessage(websocket.TextMessage, frame); err != nil {
		return "Error: could not reach the client to run this tool"
	}

	timeout := session.clientToolTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case output := <-result:
		return output
	case <-time.After(timeout):
		logger.WarnCF("live", "Client tool timed out", map[string]interface{}{
			"tool":    bare,
			"call_id": callID,
			"timeout": timeout.String(),
		})
		return "Error: client tool timed out"
	case <-ctx.Done():
		return "Error: session ended before the tool answered"
	}
}

// handleClientToolResult consumes a tool_result frame from the client, handing it to
// whichever call is waiting. Reports whether the frame was consumed: these never go
// upstream — the upstream only sees the function_call_output built from them.
func (ls *LiveServer) handleClientToolResult(session *LiveSession, msgType int, data []byte) bool {
	if msgType != websocket.TextMessage || len(session.clientTools) == 0 {
		return false
	}

	var frame struct {
		Type   string `json:"type"`
		CallID string `json:"call_id"`
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(data, &frame); err != nil || frame.Type != "tool_result" {
		return false
	}

	output := frame.Output
	if frame.Error != "" {
		output = "Error: " + frame.Error
	}

	session.pendingMu.Lock()
	waiter := session.pending[frame.CallID]
	session.pendingMu.Unlock()

	if waiter == nil {
		// Late or unknown result — the call already timed out, or was never made.
		logger.DebugCF("live", "Dropped an unmatched tool_result", map[string]interface{}{
			"call_id": frame.CallID,
		})
		return true
	}

	select {
	case waiter <- output:
	default:
	}
	return true
}

// maxReplayTurns bounds how much of a conversation is handed back to the model. Enough
// to carry context across a reconnect or a new client, without re-sending an entire
// history — and the upstream server charges for every token of it.
const maxReplayTurns = 20

// replayHistory seeds the upstream conversation with what was said earlier under this
// session key. Without it the recorded history is write-only: turns are stored and
// never read back, so every live session starts amnesiac even when its key is reused.
//
// The items are added without a response.create, so nothing is generated — they simply
// become the context the next real turn sees.
func (ls *LiveServer) replayHistory(upstream *websocket.Conn, sessionKey string) {
	source, ok := ls.tools.(SessionHistory)
	if !ok || sessionKey == "" {
		return
	}

	turns := source.LiveHistory(sessionKey, maxReplayTurns)
	sent := 0
	for _, turn := range turns {
		content := strings.TrimSpace(turn["content"])
		if content == "" {
			continue
		}
		// The Realtime API takes input_text for the user side and text for the
		// assistant side; sending the wrong one has the item rejected.
		blockType := "input_text"
		role := turn["role"]
		switch role {
		case "assistant":
			blockType = "text"
		case "user":
		default:
			// system or tool turns are not part of the spoken conversation
			continue
		}

		frame, err := json.Marshal(map[string]interface{}{
			"type": "conversation.item.create",
			"item": map[string]interface{}{
				"type":    "message",
				"role":    role,
				"content": []map[string]interface{}{{"type": blockType, "text": content}},
			},
		})
		if err != nil {
			continue
		}
		if err := upstream.WriteMessage(websocket.TextMessage, frame); err != nil {
			logger.WarnCF("live", "Failed to replay a history turn", map[string]interface{}{
				"session": sessionKey,
				"error":   err.Error(),
			})
			return
		}
		sent++
	}

	if sent > 0 {
		logger.InfoCF("live", "Replayed session history", map[string]interface{}{
			"session": sessionKey,
			"turns":   sent,
		})
	}
}

// buildRealtimeSessionUpdate renders the persona, tool definitions and any
// live.realtime_session passthrough as an OpenAI Realtime session.update. Tool schemas are converted from the chat-completions
// shape Pepebot uses internally ({type, function:{...}}) to the flat Realtime shape.
// Returns nil when there is nothing to send.
func buildRealtimeSessionUpdate(systemPrompt string, toolDefs []map[string]interface{}, extra map[string]interface{}) []byte {
	session := map[string]interface{}{}

	// Config passthrough first, so agent-owned instructions and tools below win on
	// a key collision.
	for k, v := range extra {
		session[k] = v
	}

	if prompt := strings.TrimSpace(systemPrompt); prompt != "" {
		session["instructions"] = prompt
	}

	realtimeTools := make([]map[string]interface{}, 0, len(toolDefs))
	for _, def := range toolDefs {
		fn, ok := def["function"].(map[string]interface{})
		if !ok {
			// Already flat (name at the top level) — pass it through.
			if _, hasName := def["name"]; hasName {
				realtimeTools = append(realtimeTools, def)
			}
			continue
		}
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		tool := map[string]interface{}{"type": "function", "name": name}
		if desc, ok := fn["description"].(string); ok && desc != "" {
			tool["description"] = desc
		}
		if params, ok := fn["parameters"]; ok && params != nil {
			tool["parameters"] = params
		}
		realtimeTools = append(realtimeTools, tool)
	}
	if len(realtimeTools) > 0 {
		session["tools"] = realtimeTools
		session["tool_choice"] = "auto"
	}

	if len(session) == 0 {
		return nil
	}

	update, err := json.Marshal(map[string]interface{}{
		"type":    "session.update",
		"session": session,
	})
	if err != nil {
		return nil
	}
	return update
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

// injectGeminiSystemInstruction sets setup.systemInstruction.parts[0].text on a
// Gemini/Vertex BidiGenerateContentSetup payload, replacing any existing value.
func injectGeminiSystemInstruction(setupData []byte, prompt string) []byte {
	if len(setupData) == 0 || prompt == "" {
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

	setupInner["systemInstruction"] = map[string]interface{}{
		"parts": []map[string]interface{}{{"text": prompt}},
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
