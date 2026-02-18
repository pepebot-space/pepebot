package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pepebot-space/pepebot/pkg/agent"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// GatewayServer is the HTTP API server for OpenAI-compatible endpoints
type GatewayServer struct {
	config       *config.Config
	agentManager *agent.AgentManager
	httpServer   *http.Server
}

// NewGatewayServer creates a new gateway HTTP server
func NewGatewayServer(cfg *config.Config, agentManager *agent.AgentManager) *GatewayServer {
	return &GatewayServer{
		config:       cfg,
		agentManager: agentManager,
	}
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
