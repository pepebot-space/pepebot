package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentCaller delegates a message to a named agent.
// Implemented by agent.AgentManager to avoid circular dependency.
type AgentCaller interface {
	ProcessDirect(ctx context.Context, content string, media []string, sessionKey, agentName string) (string, error)
}

// ManageAgentTool allows the bot to manage agents via tool calls.
// It reads/writes registry.json directly to avoid circular dependency with the agent package.
type ManageAgentTool struct {
	workspace    string
	registryPath string
	agentCaller  AgentCaller
}

type agentRegistry struct {
	Version string                      `json:"version"`
	Agents  map[string]*agentDefinition `json:"agents"`
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

// SetAgentCaller injects a runtime agent caller (typically AgentManager).
func (t *ManageAgentTool) SetAgentCaller(caller AgentCaller) {
	t.agentCaller = caller
}

func (t *ManageAgentTool) Name() string {
	return "manage_agent"
}

func (t *ManageAgentTool) Description() string {
	return "Manage bot agents: register/list/enable/disable/remove(delete) agents, create bootstrap files, assign skills into agent memory, and call a specific agent directly."
}

func (t *ManageAgentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"register", "list", "enable", "disable", "remove", "delete", "create_bootstrap", "assign_skill", "call"},
				"description": "Action to perform: register, list, enable, disable, remove/delete, create_bootstrap, assign_skill, call",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Agent name (required for register, enable, disable, remove, create_bootstrap, assign_skill, call)",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (required for register, e.g. 'maia/gemini-3-pro-preview')",
			},
			"provider": map[string]interface{}{
				"type":        "string",
				"description": "Provider override for register (optional, e.g. 'vertex', 'openrouter', 'anthropic', 'opencodego')",
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
			"remove_files": map[string]interface{}{
				"type":        "boolean",
				"description": "Also delete agent prompt directory on remove (optional, default false)",
			},
			"skill": map[string]interface{}{
				"type":        "string",
				"description": "Skill name to assign to an agent memory (required for assign_skill)",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message to send to target agent (required for call)",
			},
			"session_key": map[string]interface{}{
				"type":        "string",
				"description": "Optional session key for call action",
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
	case "remove", "delete":
		return t.removeAgent(args)
	case "create_bootstrap":
		return t.createBootstrap(args)
	case "assign_skill":
		return t.assignSkill(args)
	case "call":
		return t.callAgent(ctx, args)
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
	if provider, ok := args["provider"].(string); ok {
		def.Provider = strings.TrimSpace(provider)
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
		if def.Provider != "" {
			agent["provider"] = def.Provider
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

func (t *ManageAgentTool) removeAgent(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for remove action")
	}
	if name == "default" {
		return "", fmt.Errorf("cannot remove default agent")
	}

	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	def, exists := reg.Agents[name]
	if !exists {
		return "", fmt.Errorf("agent '%s' not found", name)
	}

	delete(reg.Agents, name)
	if err := t.saveRegistry(reg); err != nil {
		return "", fmt.Errorf("failed to save registry: %w", err)
	}

	removedDir := ""
	if removeFiles, _ := args["remove_files"].(bool); removeFiles {
		agentDir := def.PromptFile
		if agentDir == "" {
			agentDir = filepath.Join(filepath.Dir(t.registryPath), name)
		}
		if err := os.RemoveAll(agentDir); err != nil {
			return "", fmt.Errorf("agent removed from registry but failed to remove files at %s: %w", agentDir, err)
		}
		removedDir = agentDir
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Agent '%s' removed", name),
	}
	if removedDir != "" {
		result["removed_dir"] = removedDir
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

func (t *ManageAgentTool) assignSkill(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for assign_skill action")
	}
	skill, ok := args["skill"].(string)
	if !ok || strings.TrimSpace(skill) == "" {
		return "", fmt.Errorf("skill is required for assign_skill action")
	}
	skill = strings.TrimSpace(skill)

	reg, err := t.loadRegistry()
	if err != nil {
		return "", err
	}

	def, exists := reg.Agents[name]
	if !exists {
		return "", fmt.Errorf("agent '%s' not found", name)
	}

	if !t.skillExists(skill) {
		return "", fmt.Errorf("skill '%s' not found in workspace/builtin skills", skill)
	}

	agentDir := def.PromptFile
	if agentDir == "" {
		agentDir = filepath.Join(filepath.Dir(t.registryPath), name)
	}
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create memory directory: %w", err)
	}

	memoryPath := filepath.Join(memoryDir, "MEMORY.md")
	existing := ""
	if data, err := os.ReadFile(memoryPath); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read agent memory: %w", err)
	}

	marker := "## Assigned Skills\n"
	entry := fmt.Sprintf("- %s\n", skill)
	updated := existing

	if strings.Contains(existing, entry) {
		result := map[string]interface{}{
			"success":     true,
			"message":     fmt.Sprintf("Skill '%s' already assigned to agent '%s'", skill, name),
			"memory_path": memoryPath,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	if strings.Contains(existing, marker) {
		updated = strings.Replace(existing, marker, marker+entry, 1)
	} else {
		if strings.TrimSpace(existing) != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		updated = existing + "\n" + marker + entry
	}

	if err := os.WriteFile(memoryPath, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("failed to write agent memory: %w", err)
	}

	result := map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("Assigned skill '%s' to agent '%s'", skill, name),
		"memory_path": memoryPath,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func (t *ManageAgentTool) callAgent(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.agentCaller == nil {
		return "", fmt.Errorf("agent caller is not available in this runtime")
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for call action")
	}
	message, ok := args["message"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("message is required for call action")
	}

	sessionKey, _ := args["session_key"].(string)
	if strings.TrimSpace(sessionKey) == "" {
		parentSessionKey := SessionKeyFromContext(ctx)
		if parentSessionKey == "" {
			parentSessionKey = "default"
		}
		sessionKey = fmt.Sprintf("%s:tool:manage_agent:call:%s", parentSessionKey, name)
	}

	response, err := t.agentCaller.ProcessDirect(ctx, message, nil, sessionKey, name)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"success":     true,
		"agent":       name,
		"session_key": sessionKey,
		"response":    response,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

func (t *ManageAgentTool) skillExists(skill string) bool {
	if strings.TrimSpace(skill) == "" {
		return false
	}
	workspaceSkill := filepath.Join(t.workspace, "skills", skill, "SKILL.md")
	if _, err := os.Stat(workspaceSkill); err == nil {
		return true
	}
	builtinSkill := filepath.Join(filepath.Dir(t.workspace), "pepebot", "skills", skill, "SKILL.md")
	if _, err := os.Stat(builtinSkill); err == nil {
		return true
	}
	return false
}
