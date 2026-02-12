package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ManageAgentTool allows the bot to manage agents via tool calls.
// It reads/writes registry.json directly to avoid circular dependency with the agent package.
type ManageAgentTool struct {
	workspace    string
	registryPath string
}

type agentRegistry struct {
	Version string                        `json:"version"`
	Agents  map[string]*agentDefinition   `json:"agents"`
}

type agentDefinition struct {
	Enabled     bool    `json:"enabled"`
	Model       string  `json:"model"`
	Provider    string  `json:"provider,omitempty"`
	Description string  `json:"description,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	PromptFile  string  `json:"prompt_file,omitempty"`
}

func NewManageAgentTool(workspace string) *ManageAgentTool {
	return &ManageAgentTool{
		workspace:    workspace,
		registryPath: filepath.Join(workspace, "agents", "registry.json"),
	}
}

func (t *ManageAgentTool) Name() string {
	return "manage_agent"
}

func (t *ManageAgentTool) Description() string {
	return "Manage bot agents: register new agents, list agents, enable/disable agents, and create bootstrap files for agent personalization."
}

func (t *ManageAgentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"register", "list", "enable", "disable", "create_bootstrap"},
				"description": "Action to perform: register (new agent), list (all agents), enable/disable (toggle agent), create_bootstrap (create template files in agent dir)",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Agent name (required for register, enable, disable, create_bootstrap)",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (required for register, e.g. 'maia/gemini-3-pro-preview')",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Agent description (optional, for register)",
			},
			"temperature": map[string]interface{}{
				"type":        "number",
				"description": "Temperature setting 0.0-1.0 (optional, for register)",
			},
			"max_tokens": map[string]interface{}{
				"type":        "integer",
				"description": "Max tokens for responses (optional, for register)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ManageAgentTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action must be a string")
	}

	switch action {
	case "register":
		return t.registerAgent(args)
	case "list":
		return t.listAgents()
	case "enable":
		return t.toggleAgent(args, true)
	case "disable":
		return t.toggleAgent(args, false)
	case "create_bootstrap":
		return t.createBootstrap(args)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ManageAgentTool) loadRegistry() (*agentRegistry, error) {
	reg := &agentRegistry{
		Version: "1.0",
		Agents:  make(map[string]*agentDefinition),
	}

	data, err := os.ReadFile(t.registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}

	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	return reg, nil
}

func (t *ManageAgentTool) saveRegistry(reg *agentRegistry) error {
	dir := filepath.Dir(t.registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	return os.WriteFile(t.registryPath, data, 0644)
}

func (t *ManageAgentTool) registerAgent(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for register action")
	}

	model, ok := args["model"].(string)
	if !ok || model == "" {
		return "", fmt.Errorf("model is required for register action")
	}

	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	def := &agentDefinition{
		Enabled: true,
		Model:   model,
	}

	if desc, ok := args["description"].(string); ok {
		def.Description = desc
	}
	if temp, ok := args["temperature"].(float64); ok {
		def.Temperature = temp
	}
	if mt, ok := args["max_tokens"].(float64); ok {
		def.MaxTokens = int(mt)
	}

	// Auto-set PromptFile to agent directory
	agentDir := filepath.Join(filepath.Dir(t.registryPath), name)
	def.PromptFile = agentDir

	reg.Agents[name] = def

	if err := t.saveRegistry(reg); err != nil {
		return "", fmt.Errorf("failed to save registry: %w", err)
	}

	// Create agent directory
	os.MkdirAll(agentDir, 0755)

	result := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Agent '%s' registered with model '%s'", name, model),
		"agent_dir": agentDir,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func (t *ManageAgentTool) listAgents() (string, error) {
	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	if len(reg.Agents) == 0 {
		result := map[string]interface{}{
			"agents":  []interface{}{},
			"message": "No agents registered",
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	agents := make([]map[string]interface{}, 0, len(reg.Agents))
	for name, def := range reg.Agents {
		agent := map[string]interface{}{
			"name":    name,
			"enabled": def.Enabled,
			"model":   def.Model,
		}
		if def.Description != "" {
			agent["description"] = def.Description
		}
		if def.Temperature > 0 {
			agent["temperature"] = def.Temperature
		}
		if def.MaxTokens > 0 {
			agent["max_tokens"] = def.MaxTokens
		}
		if def.PromptFile != "" {
			agent["prompt_dir"] = def.PromptFile
		}
		agents = append(agents, agent)
	}

	result := map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func (t *ManageAgentTool) toggleAgent(args map[string]interface{}, enable bool) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for enable/disable action")
	}

	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	def, exists := reg.Agents[name]
	if !exists {
		return "", fmt.Errorf("agent '%s' not found", name)
	}

	def.Enabled = enable

	if err := t.saveRegistry(reg); err != nil {
		return "", fmt.Errorf("failed to save registry: %w", err)
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Agent '%s' %s", name, action),
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func (t *ManageAgentTool) createBootstrap(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for create_bootstrap action")
	}

	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	def, exists := reg.Agents[name]
	if !exists {
		return "", fmt.Errorf("agent '%s' not found", name)
	}

	agentDir := def.PromptFile
	if agentDir == "" {
		agentDir = filepath.Join(filepath.Dir(t.registryPath), name)
	}

	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create agent directory: %w", err)
	}

	templates := map[string]string{
		"SOUL.md": fmt.Sprintf(`# Soul - %s

## Personality

- Helpful and friendly
- Concise and to the point

## Values

- Accuracy over speed
- User privacy and safety
`, name),
		"USER.md": `# User

Information about user goes here.

## Preferences

- Communication style: (casual/formal)
- Language: (preferred language)
`,
		"IDENTITY.md": fmt.Sprintf(`# Identity

## Name
%s

## Description
Custom agent for pepebot.
`, name),
	}

	created := []string{}
	skipped := []string{}
	for filename, content := range templates {
		filePath := filepath.Join(agentDir, filename)
		if _, err := os.Stat(filePath); err == nil {
			skipped = append(skipped, filename)
			continue
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write %s: %w", filename, err)
		}
		created = append(created, filename)
	}

	result := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Bootstrap files created for agent '%s'", name),
		"agent_dir": agentDir,
		"created":   created,
		"skipped":   skipped,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}
