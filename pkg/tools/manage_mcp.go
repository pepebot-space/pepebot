package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pepebot-space/pepebot/pkg/mcp"
)

type ManageMCPTool struct {
	store *mcp.RegistryStore
}

func NewManageMCPTool(workspace string) *ManageMCPTool {
	return &ManageMCPTool{store: mcp.NewRegistryStore(workspace)}
}

func (t *ManageMCPTool) Name() string {
	return "manage_mcp"
}

func (t *ManageMCPTool) Description() string {
	return "Manage MCP servers: add/update, remove, or list server definitions. Supports stdio, remote SSE, and remote HTTP transports."
}

func (t *ManageMCPTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"add", "remove", "list"},
				"description": "MCP action: add (create/update), remove (delete by name), list (show all)",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "MCP server name (required for add/remove)",
			},
			"transport": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"stdio", "sse", "http"},
				"description": "Transport type (required for add)",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Remote MCP URL (required for sse/http)",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Executable command (required for stdio)",
			},
			"args": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Command arguments for stdio servers",
			},
			"env": map[string]interface{}{
				"type":        "object",
				"description": "Environment variables map for stdio servers",
			},
			"headers": map[string]interface{}{
				"type":        "object",
				"description": "HTTP headers map for remote sse/http servers",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Optional description for this MCP server",
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether this MCP server is enabled (default true)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ManageMCPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return "", fmt.Errorf("action must be a string")
	}

	switch action {
	case "add":
		return t.addOrUpdate(args)
	case "remove":
		return t.remove(args)
	case "list":
		return t.list()
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ManageMCPTool) addOrUpdate(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for add action")
	}

	transport, ok := args["transport"].(string)
	if !ok || transport == "" {
		return "", fmt.Errorf("transport is required for add action")
	}

	def := &mcp.ServerDefinition{Enabled: true, Transport: transport}
	if v, ok := args["url"].(string); ok {
		def.URL = v
	}
	if v, ok := args["command"].(string); ok {
		def.Command = v
	}
	if v, ok := args["args"].([]interface{}); ok {
		def.Args = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				def.Args = append(def.Args, s)
			}
		}
	}
	if v, ok := args["env"].(map[string]interface{}); ok {
		def.Env = make(map[string]string, len(v))
		for k, raw := range v {
			def.Env[k] = fmt.Sprintf("%v", raw)
		}
	}
	if v, ok := args["headers"].(map[string]interface{}); ok {
		def.Headers = make(map[string]string, len(v))
		for k, raw := range v {
			def.Headers[k] = fmt.Sprintf("%v", raw)
		}
	}
	if v, ok := args["description"].(string); ok {
		def.Description = v
	}
	if v, ok := args["enabled"].(bool); ok {
		def.Enabled = v
	}

	if err := t.store.AddOrUpdate(name, def); err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"success":   true,
		"action":    "add",
		"name":      name,
		"transport": def.Transport,
		"message":   fmt.Sprintf("MCP server '%s' saved", name),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

func (t *ManageMCPTool) remove(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required for remove action")
	}

	if err := t.store.Remove(name); err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"success": true,
		"action":  "remove",
		"name":    name,
		"message": fmt.Sprintf("MCP server '%s' removed", name),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

func (t *ManageMCPTool) list() (string, error) {
	servers, err := t.store.List()
	if err != nil {
		return "", err
	}

	items := make([]map[string]interface{}, 0, len(servers))
	for _, name := range mcp.SortedServerNames(servers) {
		def := servers[name]
		item := map[string]interface{}{
			"name":      name,
			"enabled":   def.Enabled,
			"transport": def.Transport,
		}
		if def.Description != "" {
			item["description"] = def.Description
		}
		if def.URL != "" {
			item["url"] = def.URL
		}
		if def.Command != "" {
			item["command"] = def.Command
		}
		if len(def.Args) > 0 {
			item["args"] = def.Args
		}
		if len(def.Env) > 0 {
			item["env"] = def.Env
		}
		if len(def.Headers) > 0 {
			item["headers"] = def.Headers
		}
		if def.Source != "" {
			item["source"] = def.Source
		}
		if def.Skill != "" {
			item["skill"] = def.Skill
		}
		items = append(items, item)
	}

	result := map[string]interface{}{
		"servers": items,
		"total":   len(items),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}
