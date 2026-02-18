package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/session"
)

// AgentManager manages multiple agent instances
type AgentManager struct {
	config       *config.Config
	bus          *bus.MessageBus
	provider     providers.LLMProvider
	registry     *AgentRegistry
	agents       map[string]*AgentLoop
	mu           sync.RWMutex
	defaultAgent string
	inFlight     sync.Map // map[sessionKey]context.CancelFunc
}

// NewAgentManager creates a new agent manager
func NewAgentManager(cfg *config.Config, bus *bus.MessageBus, provider providers.LLMProvider) (*AgentManager, error) {
	registry := NewAgentRegistry(cfg.WorkspacePath())

	// Load registry
	if err := registry.Load(); err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	// Initialize from config if empty
	if err := registry.InitializeFromConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize registry: %w", err)
	}

	// Save registry if it was just initialized
	if err := registry.Save(); err != nil {
		logger.WarnCF("agent", "Failed to save registry", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return &AgentManager{
		config:       cfg,
		bus:          bus,
		provider:     provider,
		registry:     registry,
		agents:       make(map[string]*AgentLoop),
		defaultAgent: "default",
	}, nil
}

// GetOrCreateAgent gets an existing agent or creates a new one
func (am *AgentManager) GetOrCreateAgent(agentName string) (*AgentLoop, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if agent already exists
	if agent, exists := am.agents[agentName]; exists {
		return agent, nil
	}

	// Get agent definition from registry
	agentDef, err := am.registry.Get(agentName)
	if err != nil {
		return nil, fmt.Errorf("agent '%s' not found in registry: %w", agentName, err)
	}

	// Check if agent is enabled
	if !agentDef.Enabled {
		return nil, fmt.Errorf("agent '%s' is disabled", agentName)
	}

	// Create new agent loop
	agentLoop := NewAgentLoopWithDefinition(am.config, am.bus, am.provider, agentName, agentDef)
	am.agents[agentName] = agentLoop

	logger.InfoCF("agent", "Created agent instance", map[string]interface{}{
		"name":  agentName,
		"model": agentDef.Model,
	})

	return agentLoop, nil
}

// GetDefaultAgent gets the default agent
func (am *AgentManager) GetDefaultAgent() (*AgentLoop, error) {
	return am.GetOrCreateAgent(am.defaultAgent)
}

// ProcessMessage processes a message with the specified agent (or default)
func (am *AgentManager) ProcessMessage(ctx context.Context, msg bus.InboundMessage, agentName string) (string, error) {
	// If no agent specified, use default
	if agentName == "" {
		agentName = am.defaultAgent
	}

	// Get or create agent
	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return "", err
	}

	logger.DebugCF("agent", "Processing with agent", map[string]interface{}{
		"agent_name": agentName,
		"model":      agentLoop.model,
	})

	// Process message
	return agentLoop.processMessage(ctx, msg)
}

// SetDefaultAgent sets the default agent
func (am *AgentManager) SetDefaultAgent(agentName string) error {
	// Verify agent exists and is enabled
	agentDef, err := am.registry.Get(agentName)
	if err != nil {
		return fmt.Errorf("agent '%s' not found: %w", agentName, err)
	}

	if !agentDef.Enabled {
		return fmt.Errorf("agent '%s' is disabled", agentName)
	}

	am.mu.Lock()
	am.defaultAgent = agentName
	am.mu.Unlock()

	logger.InfoCF("agent", "Set default agent", map[string]interface{}{
		"agent": agentName,
	})

	return nil
}

// ListAgents returns all registered agents
func (am *AgentManager) ListAgents() map[string]*AgentDefinition {
	return am.registry.List()
}

// ListEnabledAgents returns only enabled agents
func (am *AgentManager) ListEnabledAgents() map[string]*AgentDefinition {
	return am.registry.ListEnabled()
}

// GetRegistry returns the agent registry
func (am *AgentManager) GetRegistry() *AgentRegistry {
	return am.registry
}

// GetConfig returns the agent manager's config
func (am *AgentManager) GetConfig() *config.Config {
	return am.config
}

// ProcessDirectStream processes a message with streaming using the specified agent
func (am *AgentManager) ProcessDirectStream(ctx context.Context, content, sessionKey, agentName string, callback providers.StreamCallback) error {
	if agentName == "" {
		agentName = am.defaultAgent
	}

	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return err
	}

	return agentLoop.ProcessDirectStream(ctx, content, sessionKey, callback)
}

// ProcessDirect processes a message without streaming using the specified agent
func (am *AgentManager) ProcessDirect(ctx context.Context, content, sessionKey, agentName string) (string, error) {
	if agentName == "" {
		agentName = am.defaultAgent
	}

	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return "", err
	}

	return agentLoop.ProcessDirect(ctx, content, sessionKey)
}

// ClearSession clears a session on the specified agent
func (am *AgentManager) ClearSession(sessionKey, agentName string) {
	if agentName == "" {
		agentName = am.defaultAgent
	}

	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return
	}

	agentLoop.ClearSession(sessionKey)
}

// GetSessions returns the session manager from the default agent
func (am *AgentManager) GetSessions() *session.SessionManager {
	agentLoop, err := am.GetDefaultAgent()
	if err != nil {
		return nil
	}
	return agentLoop.Sessions()
}

// StopSession stops in-flight processing for a session key (reuses cmdStop logic)
func (am *AgentManager) StopSession(sessionKey string) string {
	cancelVal, ok := am.inFlight.Load(sessionKey)
	if !ok {
		return "No active processing to stop."
	}

	if cancel, ok := cancelVal.(context.CancelFunc); ok {
		cancel()
		return "Stopping current processing..."
	}

	return "No active processing to stop."
}

// Run starts processing messages from the bus
func (am *AgentManager) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := am.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			// Check if message is a command
			if strings.HasPrefix(msg.Content, "/") {
				am.handleCommand(ctx, msg)
				continue
			}

			// Process normal messages in a goroutine for concurrency
			go am.processAndRespond(ctx, msg)
		}
	}
}

// processAndRespond processes a message with cancellation support and publishes the response
func (am *AgentManager) processAndRespond(ctx context.Context, msg bus.InboundMessage) {
	chatCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Store cancel func so /stop can abort this
	am.inFlight.Store(msg.SessionKey, cancel)
	defer am.inFlight.Delete(msg.SessionKey)

	// Extract agent name from metadata if present
	agentName := ""
	if msg.Metadata != nil {
		agentName = msg.Metadata["agent"]
	}

	response, err := am.ProcessMessage(chatCtx, msg, agentName)
	if err != nil {
		if chatCtx.Err() != nil {
			// Context was cancelled (by /stop)
			response = "Processing stopped."
		} else {
			response = fmt.Sprintf("Error processing message: %v", err)
		}
	}

	if response != "" {
		am.bus.PublishOutbound(bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: response,
		})
	}
}

// handleCommand dispatches slash commands
func (am *AgentManager) handleCommand(ctx context.Context, msg bus.InboundMessage) {
	parts := strings.Fields(msg.Content)
	command := strings.ToLower(parts[0])

	var response string

	switch command {
	case "/new":
		response = am.cmdNew(msg)
	case "/stop":
		response = am.cmdStop(msg)
	case "/help":
		response = am.cmdHelp()
	case "/status":
		response = am.cmdStatus(msg)
	default:
		// Not a known command, process as normal message
		go am.processAndRespond(ctx, msg)
		return
	}

	if response != "" {
		am.bus.PublishOutbound(bus.OutboundMessage{
			Channel: msg.Channel,
			ChatID:  msg.ChatID,
			Content: response,
		})
	}
}

// cmdNew clears the session for the current chat
func (am *AgentManager) cmdNew(msg bus.InboundMessage) string {
	agentName := am.defaultAgent
	if msg.Metadata != nil && msg.Metadata["agent"] != "" {
		agentName = msg.Metadata["agent"]
	}

	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	agentLoop.ClearSession(msg.SessionKey)
	return "Session cleared. Starting fresh conversation."
}

// cmdStop cancels any in-flight LLM call for this session
func (am *AgentManager) cmdStop(msg bus.InboundMessage) string {
	cancelVal, ok := am.inFlight.Load(msg.SessionKey)
	if !ok {
		return "No active processing to stop."
	}

	if cancel, ok := cancelVal.(context.CancelFunc); ok {
		cancel()
		return "Stopping current processing..."
	}

	return "No active processing to stop."
}

// cmdHelp returns a list of available commands
func (am *AgentManager) cmdHelp() string {
	return "Available commands:\n" +
		"/new    - Clear session, start fresh conversation\n" +
		"/stop   - Cancel current LLM processing\n" +
		"/help   - Show this help message\n" +
		"/status - Show agent & session info"
}

// cmdStatus returns info about the current agent and session
func (am *AgentManager) cmdStatus(msg bus.InboundMessage) string {
	agentName := am.defaultAgent
	if msg.Metadata != nil && msg.Metadata["agent"] != "" {
		agentName = msg.Metadata["agent"]
	}

	agentLoop, err := am.GetOrCreateAgent(agentName)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	_, processing := am.inFlight.Load(msg.SessionKey)
	processingStatus := "idle"
	if processing {
		processingStatus = "processing"
	}

	return fmt.Sprintf("Agent: %s\nModel: %s\nSession: %s\nStatus: %s",
		agentLoop.AgentName(), agentLoop.Model(), msg.SessionKey, processingStatus)
}
