package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/anak10thn/pepebot/pkg/config"
	"github.com/anak10thn/pepebot/pkg/logger"
)

// AgentDefinition defines a registered agent configuration
type AgentDefinition struct {
	Enabled     bool    `json:"enabled"`
	Model       string  `json:"model"`
	Provider    string  `json:"provider"`
	Description string  `json:"description"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	PromptFile  string  `json:"prompt_file,omitempty"`
}

// AgentRegistry manages multiple agent configurations
type AgentRegistry struct {
	Version string                      `json:"version"`
	Agents  map[string]*AgentDefinition `json:"agents"`
	mu      sync.RWMutex
	path    string
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry(workspacePath string) *AgentRegistry {
	registryPath := filepath.Join(workspacePath, "agents", "registry.json")
	return &AgentRegistry{
		Version: "1.0",
		Agents:  make(map[string]*AgentDefinition),
		path:    registryPath,
	}
}

// Load loads the agent registry from disk
func (ar *AgentRegistry) Load() error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	// Check if registry file exists
	if _, err := os.Stat(ar.path); os.IsNotExist(err) {
		logger.DebugC("agent", "Registry file not found, will create on first save")
		return nil
	}

	data, err := os.ReadFile(ar.path)
	if err != nil {
		return fmt.Errorf("failed to read registry: %w", err)
	}

	if err := json.Unmarshal(data, ar); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	logger.InfoCF("agent", "Loaded agent registry", map[string]interface{}{
		"agents": len(ar.Agents),
		"path":   ar.path,
	})

	return nil
}

// Save saves the agent registry to disk
func (ar *AgentRegistry) Save() error {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(ar.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	data, err := json.MarshalIndent(ar, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(ar.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry: %w", err)
	}

	logger.InfoCF("agent", "Saved agent registry", map[string]interface{}{
		"agents": len(ar.Agents),
		"path":   ar.path,
	})

	return nil
}

// AgentPromptDir returns the directory path for agent-specific bootstrap files
func (ar *AgentRegistry) AgentPromptDir(agentName string) string {
	return filepath.Join(filepath.Dir(ar.path), agentName)
}

// EnsureAgentDir creates the agent-specific directory if it doesn't exist
func (ar *AgentRegistry) EnsureAgentDir(agentName string) (string, error) {
	dir := ar.AgentPromptDir(agentName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create agent directory: %w", err)
	}
	return dir, nil
}

// Register adds or updates an agent in the registry
func (ar *AgentRegistry) Register(name string, def *AgentDefinition) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	if def.Model == "" {
		return fmt.Errorf("agent model cannot be empty")
	}

	// Auto-set PromptFile to agent directory if not specified
	if def.PromptFile == "" {
		def.PromptFile = filepath.Join(filepath.Dir(ar.path), name)
	}

	ar.Agents[name] = def

	logger.InfoCF("agent", "Registered agent", map[string]interface{}{
		"name":  name,
		"model": def.Model,
	})

	return nil
}

// Unregister removes an agent from the registry
func (ar *AgentRegistry) Unregister(name string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if _, exists := ar.Agents[name]; !exists {
		return fmt.Errorf("agent '%s' not found", name)
	}

	delete(ar.Agents, name)

	logger.InfoCF("agent", "Unregistered agent", map[string]interface{}{
		"name": name,
	})

	return nil
}

// Get retrieves an agent definition by name
func (ar *AgentRegistry) Get(name string) (*AgentDefinition, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	agent, exists := ar.Agents[name]
	if !exists {
		return nil, fmt.Errorf("agent '%s' not found", name)
	}

	return agent, nil
}

// List returns all registered agents
func (ar *AgentRegistry) List() map[string]*AgentDefinition {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*AgentDefinition)
	for name, agent := range ar.Agents {
		result[name] = agent
	}

	return result
}

// ListEnabled returns only enabled agents
func (ar *AgentRegistry) ListEnabled() map[string]*AgentDefinition {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	result := make(map[string]*AgentDefinition)
	for name, agent := range ar.Agents {
		if agent.Enabled {
			result[name] = agent
		}
	}

	return result
}

// Enable enables an agent
func (ar *AgentRegistry) Enable(name string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	agent, exists := ar.Agents[name]
	if !exists {
		return fmt.Errorf("agent '%s' not found", name)
	}

	agent.Enabled = true

	logger.InfoCF("agent", "Enabled agent", map[string]interface{}{
		"name": name,
	})

	return nil
}

// Disable disables an agent
func (ar *AgentRegistry) Disable(name string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	agent, exists := ar.Agents[name]
	if !exists {
		return fmt.Errorf("agent '%s' not found", name)
	}

	agent.Enabled = false

	logger.InfoCF("agent", "Disabled agent", map[string]interface{}{
		"name": name,
	})

	return nil
}

// InitializeFromConfig ensures a default agent exists from config
func (ar *AgentRegistry) InitializeFromConfig(cfg *config.Config) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	// Always ensure "default" agent exists
	if _, exists := ar.Agents["default"]; !exists {
		ar.Agents["default"] = &AgentDefinition{
			Enabled:     true,
			Model:       cfg.Agents.Defaults.Model,
			Provider:    "",
			Description: "Default general-purpose agent",
			Temperature: cfg.Agents.Defaults.Temperature,
			MaxTokens:   cfg.Agents.Defaults.MaxTokens,
		}
		logger.InfoC("agent", "Initialized default agent from config")
	}

	return nil
}

// GetOrDefault gets an agent by name, or returns default agent
func (ar *AgentRegistry) GetOrDefault(name string) (*AgentDefinition, string, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// If name is specified, try to get it
	if name != "" {
		agent, exists := ar.Agents[name]
		if !exists {
			return nil, "", fmt.Errorf("agent '%s' not found", name)
		}
		return agent, name, nil
	}

	// Try to get "default" agent
	if agent, exists := ar.Agents["default"]; exists {
		return agent, "default", nil
	}

	// If no default, return first enabled agent
	for name, agent := range ar.Agents {
		if agent.Enabled {
			return agent, name, nil
		}
	}

	return nil, "", fmt.Errorf("no agents available")
}
