package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/anak10thn/pepebot/pkg/bus"
	"github.com/anak10thn/pepebot/pkg/config"
	"github.com/anak10thn/pepebot/pkg/logger"
	"github.com/anak10thn/pepebot/pkg/providers"
)

// AgentManager manages multiple agent instances
type AgentManager struct {
	config    *config.Config
	bus       *bus.MessageBus
	provider  providers.LLMProvider
	registry  *AgentRegistry
	agents    map[string]*AgentLoop
	mu        sync.RWMutex
	defaultAgent string
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

			// Extract agent name from metadata if present
			agentName := ""
			if msg.Metadata != nil {
				agentName = msg.Metadata["agent"]
			}

			response, err := am.ProcessMessage(ctx, msg, agentName)
			if err != nil {
				response = fmt.Sprintf("Error processing message: %v", err)
			}

			if response != "" {
				am.bus.PublishOutbound(bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: response,
				})
			}
		}
	}
}
