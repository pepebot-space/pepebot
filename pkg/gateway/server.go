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
	"github.com/pepebot-space/pepebot/pkg/task"
)

// GatewayServer is the HTTP API server for OpenAI-compatible endpoints
type GatewayServer struct {
	config         *config.Config
	agentManager   *agent.AgentManager
	bus            *bus.MessageBus
	httpServer     *http.Server
	liveServer     *live.LiveServer
	taskStore      task.TaskStore
	taskDispatcher *task.Dispatcher
	taskStreamHub  *TaskStreamHub
	taskStopChan   chan struct{}
	restartFunc    func() // called to trigger graceful restart
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

	// Initialize task store if orchestration is enabled
	if cfg.Orchestration.Enabled {
		store, err := task.NewTaskStore(&cfg.Orchestration)
		if err != nil {
			logger.WarnCF("gateway", "Failed to initialize task store", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			gs.taskStore = store
			gs.taskDispatcher = task.NewDispatcher(store, agentManager, agentManager)
			gs.taskStreamHub = NewTaskStreamHub()
			gs.taskStopChan = make(chan struct{})

			// Wire notifications via message bus
			var notifyChannels []task.NotifyChannel
			if cfg.Channels.Telegram.Enabled && cfg.Channels.Telegram.Token != "" {
				for _, chatID := range cfg.Channels.Telegram.AllowFrom {
					notifyChannels = append(notifyChannels, task.NotifyChannel{Channel: "telegram", ChatID: chatID})
				}
			}
			if cfg.Channels.Discord.Enabled && cfg.Channels.Discord.Token != "" {
				for _, chatID := range cfg.Channels.Discord.AllowFrom {
					notifyChannels = append(notifyChannels, task.NotifyChannel{Channel: "discord", ChatID: chatID})
				}
			}
			if len(notifyChannels) > 0 {
				notifier := task.NewNotifier(func(channel, chatID, message string) {
					msgBus.PublishOutbound(bus.OutboundMessage{
						Channel: channel,
						ChatID:  chatID,
						Content: message,
					})
				}, notifyChannels)
				gs.taskDispatcher.SetNotifier(notifier)
				logger.InfoCF("gateway", "Task notifications enabled", map[string]interface{}{
					"channels": len(notifyChannels),
				})
			}

			logger.InfoC("gateway", "Task orchestration enabled")
		}
	}

	// Initialize Live API server if enabled
	if cfg.Live.Enabled {
		gs.liveServer = live.NewLiveServer(cfg)
		if agentManager != nil {
			gs.liveServer.SetToolExecutor(agentManager)
		}

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
	mux.HandleFunc("/v1/features", gs.corsMiddleware(gs.handleFeatures))
	mux.HandleFunc("/v1/tasks", gs.corsMiddleware(gs.handleTaskRoutes))
	mux.HandleFunc("/v1/tasks/", gs.corsMiddleware(gs.handleTaskRoutes))
	mux.HandleFunc("/v1/task-templates", gs.corsMiddleware(gs.handleTemplateRoutes))
	mux.HandleFunc("/v1/task-templates/", gs.corsMiddleware(gs.handleTemplateRoutes))
	if gs.taskStreamHub != nil {
		mux.HandleFunc("/v1/tasks/stream", gs.taskStreamHub.HandleWebSocket)
	}

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

	// Start task dispatch + cleanup ticker (every 30 seconds)
	if gs.taskDispatcher != nil {
		go gs.runTaskTicker(ctx)
	}

	return nil
}

// emitTaskEvent broadcasts a task event to WebSocket clients.
func (gs *GatewayServer) emitTaskEvent(eventType string, t *task.Task) {
	if gs.taskStreamHub == nil || t == nil {
		return
	}
	gs.taskStreamHub.Broadcast(TaskEvent{Type: eventType, Task: t})
}

// runTaskTicker periodically dispatches tasks and runs cleanup.
func (gs *GatewayServer) runTaskTicker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-gs.taskStopChan:
			return
		case <-ticker.C:
			gs.taskDispatcher.Dispatch(ctx)
			task.RunCleanup(gs.taskStore, gs.config.Orchestration.TTL.DoneDays, gs.config.Orchestration.TTL.FailedDays)
		}
	}
}

// Stop gracefully shuts down the HTTP server
func (gs *GatewayServer) Stop(ctx context.Context) error {
	if gs.taskStopChan != nil {
		close(gs.taskStopChan)
	}
	if gs.taskStore != nil {
		gs.taskStore.Close()
	}

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
