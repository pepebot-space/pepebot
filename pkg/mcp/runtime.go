package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

type RuntimeTool struct {
	ServerName   string
	Name         string
	Description  string
	InputSchema  map[string]interface{}
	Transport    string
	OriginalName string
}

type Runtime struct {
	store   *RegistryStore
	mu      sync.RWMutex
	clients map[string]Client
	tools   []RuntimeTool
}

func NewRuntime(workspace string) *Runtime {
	return &Runtime{
		store:   NewRegistryStore(workspace),
		clients: make(map[string]Client),
		tools:   []RuntimeTool{},
	}
}

func (r *Runtime) Load(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = []RuntimeTool{}

	servers, err := r.store.List()
	if err != nil {
		return err
	}

	for _, serverName := range SortedServerNames(servers) {
		def := servers[serverName]
		if def == nil || !def.Enabled {
			continue
		}

		client, err := createClient(def)
		if err != nil {
			logger.WarnCF("mcp", "Skipping MCP server (invalid config)", map[string]interface{}{
				"server": serverName,
				"error":  err.Error(),
			})
			continue
		}

		if err := client.Initialize(ctx); err != nil {
			logger.WarnCF("mcp", "Failed to initialize MCP server", map[string]interface{}{
				"server":    serverName,
				"transport": def.Transport,
				"error":     err.Error(),
			})
			_ = client.Close()
			continue
		}

		remoteTools, err := client.ListTools(ctx)
		if err != nil {
			logger.WarnCF("mcp", "Failed to list MCP tools", map[string]interface{}{
				"server":    serverName,
				"transport": def.Transport,
				"error":     err.Error(),
			})
			_ = client.Close()
			continue
		}

		r.clients[serverName] = client
		for _, rt := range remoteTools {
			r.tools = append(r.tools, RuntimeTool{
				ServerName:   serverName,
				Name:         rt.Name,
				OriginalName: rt.Name,
				Description:  strings.TrimSpace(rt.Description),
				InputSchema:  rt.InputSchema,
				Transport:    def.Transport,
			})
		}

		logger.InfoCF("mcp", "Loaded MCP tools", map[string]interface{}{
			"server": serverName,
			"tools":  len(remoteTools),
		})
	}

	return nil
}

func (r *Runtime) Tools() []RuntimeTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RuntimeTool, len(r.tools))
	copy(out, r.tools)
	return out
}

func (r *Runtime) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	client, ok := r.clients[serverName]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("mcp server '%s' is not loaded", serverName)
	}

	return client.CallTool(ctx, toolName, args)
}

func (r *Runtime) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, c := range r.clients {
		if err := c.Close(); err != nil {
			logger.DebugCF("mcp", "Failed to close MCP client", map[string]interface{}{
				"server": name,
				"error":  err.Error(),
			})
		}
	}
	r.clients = make(map[string]Client)
	r.tools = []RuntimeTool{}
}

func createClient(def *ServerDefinition) (Client, error) {
	if err := ValidateServerDefinition("runtime", def); err != nil {
		return nil, err
	}

	switch strings.ToLower(def.Transport) {
	case "stdio":
		return NewStdioClient(def.Command, def.Args, def.Env), nil
	case "http":
		return NewHTTPClient(def.URL, def.Headers), nil
	case "sse":
		// For compatibility, reuse HTTP JSON-RPC transport.
		// Many modern MCP deployments expose a POST JSON-RPC endpoint behind the same URL.
		return NewHTTPClient(def.URL, def.Headers), nil
	default:
		return nil, fmt.Errorf("unsupported transport: %s", def.Transport)
	}
}
