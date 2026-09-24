package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/providers"
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

// ModelPrompt is asked which of two disagreeing models should win. It returns
// true to overwrite the registry with the config value. A nil prompt — no
// terminal to ask at, as under systemd — means keep the registry.
type ModelPrompt func(registryModel, configModel string) (bool, error)

// ReconcileModel settles the one duplicated setting in pepebot's config.
//
// The model lived in two places: agents.defaults.model in config.json and the
// "default" entry in the agent registry. The registry silently won, so editing
// config.json looked like it did nothing — and when the registry held a stale
// value, the request went out with it. That is how a model id with the provider
// name glued to its front survived being "fixed" in config.json twice (v0.5.20).
//
// The registry is the source of truth. config.json seeds a fresh install, and
// when the two disagree the user is asked rather than one silently winning.
func (ar *AgentRegistry) ReconcileModel(cfg *config.Config, ask ModelPrompt) error {
	ar.mu.Lock()
	def, exists := ar.Agents["default"]
	ar.mu.Unlock()

	configModel := providers.ParseModelRef(cfg.Agents.Defaults.Provider, cfg.Agents.Defaults.Model).String()

	// Fresh install: seed the registry and there is nothing to reconcile.
	if !exists || def.Model == "" {
		if configModel == "" {
			return nil
		}
		if err := ar.InitializeFromConfig(cfg); err != nil {
			return err
		}
		ar.mu.Lock()
		ar.Agents["default"].Model = configModel
		ar.Agents["default"].Provider = ""
		ar.mu.Unlock()
		return ar.Save()
	}

	registryModel := providers.ParseModelRef(def.Provider, def.Model).String()
	if registryModel == configModel || configModel == "" {
		// Still rewrite a legacy spelling into the canonical one, so what the
		// user reads back is what they could type.
		if def.Model != registryModel || def.Provider != "" {
			ar.mu.Lock()
			def.Model, def.Provider = registryModel, ""
			ar.mu.Unlock()
			return ar.Save()
		}
		return nil
	}

	// Logged before anything is asked, so the disagreement is on the record even
	// where there is no terminal — a gateway that silently prefers the registry
	// is exactly how editing config.json came to look like it did nothing.
	logger.WarnCF("agent", "config.json and the agent registry name different models", map[string]interface{}{
		"registry": registryModel,
		"config":   configModel,
		"using":    registryModel,
		"hint":     "run `pepebot agent` in a terminal to choose, or edit workspace/agents/registry.json",
	})

	if ask == nil {
		return nil
	}

	useConfig, err := ask(registryModel, configModel)
	if err != nil {
		return err
	}
	if !useConfig {
		return nil
	}

	ar.mu.Lock()
	def.Model, def.Provider = configModel, ""
	ar.mu.Unlock()
	logger.InfoCF("agent", "Registry model overwritten from config.json", map[string]interface{}{"model": configModel})
	return ar.Save()
}
