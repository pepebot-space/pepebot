package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pepebot-space/pepebot/pkg/agent"
	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/live"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// GatewayServer is the HTTP API server for OpenAI-compatible endpoints
type GatewayServer struct {
	config       *config.Config
	agentManager *agent.AgentManager
	bus          *bus.MessageBus
	httpServer   *http.Server
	liveServer   *live.LiveServer
	restartFunc  func() // called to trigger graceful restart
}

// SetRestartFunc sets the function called when a restart is requested via API or chat command
func (gs *GatewayServer) SetRestartFunc(fn func()) {
	gs.restartFunc = fn
}

// NewGatewayServer creates a new gateway HTTP server
func NewGatewayServer(cfg *config.Config, agentManager *agent.AgentManager, msgBus *bus.MessageBus) *GatewayServer {
	gs := &GatewayServer{
		config:       cfg,
		agentManager: agentManager,
		bus:          msgBus,
	}

	// Initialize Live API server if enabled
	if cfg.Live.Enabled {
		gs.liveServer = live.NewLiveServer(cfg)

		// Register Vertex AI Live provider if configured
		if cfg.Providers.Vertex.CredentialsFile != "" && cfg.Providers.Vertex.ProjectID != "" {
			vertexLive, err := live.NewVertexLiveProvider(
				cfg.Providers.Vertex.CredentialsFile,
				cfg.Providers.Vertex.ProjectID,
				cfg.Providers.Vertex.Region,
				cfg.Live,
			)
			if err != nil {
				logger.WarnCF("gateway", "Failed to init Vertex Live provider", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				gs.liveServer.RegisterProvider("vertex", vertexLive)
			}
		}

		// Register OpenAI Live provider if configured
		if cfg.Providers.OpenAI.APIKey != "" {
			openaiLive, err := live.NewOpenAILiveProvider("openai", cfg.Providers.OpenAI.APIBase, cfg.Providers.OpenAI.APIKey)
			if err == nil {
				gs.liveServer.RegisterProvider("openai", openaiLive)
			}
		}

		// Register MAIA Router Live provider if configured
		if cfg.Providers.MAIARouter.APIKey != "" {
			maiaLive, err := live.NewOpenAILiveProvider("maiarouter", cfg.Providers.MAIARouter.APIBase, cfg.Providers.MAIARouter.APIKey)
			if err == nil {
				gs.liveServer.RegisterProvider("maiarouter", maiaLive)
			}
		}

		// Register Gemini (Google AI Studio) Live provider if configured
		if cfg.Providers.Gemini.APIKey != "" {
			geminiLive, err := live.NewGeminiLiveProvider(cfg.Providers.Gemini.APIKey, cfg.Live)
			if err == nil {
				gs.liveServer.RegisterProvider("gemini", geminiLive)
			}
		}

		logger.InfoC("gateway", "Live API enabled on /v1/live")
	}

	return gs
}

// Start starts the HTTP server
func (gs *GatewayServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", gs.corsMiddleware(gs.handleHealth))
	mux.HandleFunc("/v1/chat/completions", gs.corsMiddleware(gs.handleChatCompletions))
	mux.HandleFunc("/v1/models", gs.corsMiddleware(gs.handleListModels))
	mux.HandleFunc("/v1/sessions", gs.corsMiddleware(gs.handleListSessions))
	mux.HandleFunc("/v1/sessions/", gs.corsMiddleware(gs.handleSessionRoutes))
	mux.HandleFunc("/v1/agents", gs.corsMiddleware(gs.handleListAgents))
	mux.HandleFunc("/v1/skills", gs.corsMiddleware(gs.handleListSkills))
	mux.HandleFunc("/v1/skills/", gs.corsMiddleware(gs.handleSkillRoutes))
	mux.HandleFunc("/v1/workflows", gs.corsMiddleware(gs.handleListWorkflows))
	mux.HandleFunc("/v1/workflows/", gs.corsMiddleware(gs.handleGetWorkflow))
	mux.HandleFunc("/v1/config", gs.corsMiddleware(gs.handleConfig))
	mux.HandleFunc("/v1/restart", gs.corsMiddleware(gs.handleRestart))
	mux.HandleFunc("/v1/send", gs.corsMiddleware(gs.handleSend))

	// Live API WebSocket endpoint
	if gs.liveServer != nil {
		mux.HandleFunc("/v1/live", gs.liveServer.HandleWebSocket)
	}

	addr := fmt.Sprintf("%s:%d", gs.config.Gateway.Host, gs.config.Gateway.Port)
	gs.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.InfoCF("gateway", "HTTP API server starting", map[string]interface{}{
		"addr": addr,
	})

	go func() {
		if err := gs.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("gateway", "HTTP server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (gs *GatewayServer) Stop(ctx context.Context) error {
	if gs.httpServer == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger.InfoC("gateway", "HTTP API server shutting down")
	return gs.httpServer.Shutdown(shutdownCtx)
}

// corsMiddleware adds CORS headers for dashboard access
func (gs *GatewayServer) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Agent, X-Session-Key")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
