package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/mcp"
)

type MCPProxyTool struct {
	runtime      *mcp.Runtime
	serverName   string
	toolName     string
	registeredAs string
	description  string
	parameters   map[string]interface{}
}

func (t *MCPProxyTool) Name() string {
	return t.registeredAs
}

func (t *MCPProxyTool) Description() string {
	return t.description
}

func (t *MCPProxyTool) Parameters() map[string]interface{} {
	if t.parameters != nil {
		return t.parameters
	}
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *MCPProxyTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	logger.DebugCF("mcp", "Calling MCP proxied tool", map[string]interface{}{
		"registered_name": t.registeredAs,
		"server":          t.serverName,
		"tool":            t.toolName,
	})
	return t.runtime.CallTool(ctx, t.serverName, t.toolName, args)
}

func RegisterMCPTools(workspace string, registry *ToolRegistry) (*mcp.Runtime, int, error) {
	runtime := mcp.NewRuntime(workspace)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	if err := runtime.Load(ctx); err != nil {
		runtime.Close()
		return nil, 0, err
	}

	loaded := 0
	for _, rt := range runtime.Tools() {
		name := rt.Name
		if _, exists := registry.Get(name); exists {
			name = sanitizeMCPToolName(rt.ServerName + "_" + rt.Name)
		}

		desc := strings.TrimSpace(rt.Description)
		if desc == "" {
			desc = fmt.Sprintf("MCP tool '%s' from server '%s'", rt.Name, rt.ServerName)
		}
		desc = fmt.Sprintf("[MCP:%s/%s] %s", rt.ServerName, rt.Transport, desc)

		tool := &MCPProxyTool{
			runtime:      runtime,
			serverName:   rt.ServerName,
			toolName:     rt.Name,
			registeredAs: name,
			description:  desc,
			parameters:   normalizeMCPParameters(rt.InputSchema),
		}
		registry.Register(tool)
		loaded++
	}

	if loaded == 0 {
		runtime.Close()
		return nil, 0, nil
	}

	logger.InfoCF("mcp", "Registered MCP tools in registry", map[string]interface{}{"count": loaded})
	return runtime, loaded, nil
}

func normalizeMCPParameters(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	if _, ok := schema["type"].(string); !ok {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]interface{}{}
	}
	return schema
}

func sanitizeMCPToolName(name string) string {
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "_", "/", "_", "-", "_", ".", "_", ":", "_")
	name = replacer.Replace(name)
	return "mcp_" + name
}
