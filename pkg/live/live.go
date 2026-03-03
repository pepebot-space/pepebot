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

// SetupMessage is the first message sent by the client to configure the session
type SetupMessage struct {
	Setup *SetupConfig `json:"setup,omitempty"`
}

// SetupConfig contains model and provider selection for a live session
type SetupConfig struct {
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
}

// LiveServer manages WebSocket live sessions
type LiveServer struct {
	providers map[string]LiveProvider
	config    *config.Config
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
	createdAt    time.Time
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

	logger.InfoCF("live", "Session setup", map[string]interface{}{
		"provider": providerName,
		"model":    model,
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
	if setupData := provider.SetupMessage(model); setupData != nil {
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
		ls.proxyMessages(ctx, clientConn, upstreamConn, "client→upstream")
		cancel() // If client closes, cancel the context
	}()

	// Upstream → Client
	go func() {
		defer wg.Done()
		ls.proxyMessages(ctx, upstreamConn, clientConn, "upstream→client")
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
func (ls *LiveServer) proxyMessages(ctx context.Context, src, dst *websocket.Conn, direction string) {
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

		if err := dst.WriteMessage(msgType, data); err != nil {
			logger.DebugCF("live", "Write error", map[string]interface{}{
				"direction": direction,
				"error":     err.Error(),
			})
			return
		}
	}
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
