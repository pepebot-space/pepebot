// Pepebot - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/mcp"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/session"
	"github.com/pepebot-space/pepebot/pkg/tools"
	"github.com/pepebot-space/pepebot/pkg/workflow"
)

type AgentLoop struct {
	bus            *bus.MessageBus
	provider       providers.LLMProvider
	workspace      string
	model          string
	temperature    float64
	contextWindow  int
	maxIterations  int
	sessions       *session.SessionManager
	contextBuilder *ContextBuilder
	tools          *tools.ToolRegistry
	workflowHelper *workflow.WorkflowHelper
	mcpRuntime     *mcp.Runtime
	running        bool
	summarizing    sync.Map
	agentName      string
}

// WorkflowHelper returns the workflow helper for external wiring (e.g. agent processor injection)
func (al *AgentLoop) WorkflowHelper() *workflow.WorkflowHelper {
	return al.workflowHelper
}

func NewAgentLoop(cfg *config.Config, bus *bus.MessageBus, provider providers.LLMProvider) *AgentLoop {
	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)

	toolsRegistry := tools.NewToolRegistry()
	toolsRegistry.Register(tools.NewReadFileTool(workspace))
	toolsRegistry.Register(tools.NewWriteFileTool(workspace))
	toolsRegistry.Register(tools.NewListDirTool(workspace))
	toolsRegistry.Register(tools.NewExecTool(workspace))

	// Register workflow tools (always available, no dependencies)
	workflowHelper := workflow.NewWorkflowHelper(workspace, toolsRegistry)
	toolsRegistry.Register(tools.NewWorkflowExecuteTool(workflowHelper))
	toolsRegistry.Register(tools.NewWorkflowSaveTool(workflowHelper))
	toolsRegistry.Register(tools.NewWorkflowListTool(workflowHelper))

	// Register ADB tools (conditional on ADB binary availability)
	if adbHelper, err := tools.NewAdbHelper(workspace); err == nil {
		toolsRegistry.Register(tools.NewAdbDevicesTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbShellTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbTapTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbInputTextTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbScreenshotTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbUIDumpTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbSwipeTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbOpenAppTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbKeyEventTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbRecordWorkflowTool(adbHelper, workflowHelper))
	}

	braveAPIKey := cfg.Tools.Web.Search.APIKey
	toolsRegistry.Register(tools.NewWebSearchTool(braveAPIKey, cfg.Tools.Web.Search.MaxResults))
	toolsRegistry.Register(tools.NewWebFetchTool(50000))
	toolsRegistry.Register(tools.NewSendImageTool(bus, workspace))
	toolsRegistry.Register(tools.NewSendFileTool(bus, workspace))
	toolsRegistry.Register(tools.NewManageAgentTool(workspace))
	toolsRegistry.Register(tools.NewManageMCPTool(workspace))

	var mcpRuntime *mcp.Runtime
	if rt, count, err := tools.RegisterMCPTools(workspace, toolsRegistry); err != nil {
		logger.WarnCF("mcp", "Failed to register MCP tools", map[string]interface{}{"error": err.Error()})
	} else {
		mcpRuntime = rt
		if count > 0 {
			logger.InfoCF("mcp", "MCP tools ready", map[string]interface{}{"count": count})
		}
	}

	// Platform messaging tools (direct API — no gateway required)
	if cfg.Channels.Telegram.Token != "" {
		toolsRegistry.Register(tools.NewTelegramSendTool(cfg.Channels.Telegram.Token, workspace))
	}
	if cfg.Channels.Discord.Token != "" {
		toolsRegistry.Register(tools.NewDiscordSendTool(cfg.Channels.Discord.Token, workspace))
	}
	toolsRegistry.Register(tools.NewWhatsAppSendTool(bus, workspace))

	sessionsManager := session.NewSessionManager(filepath.Join(filepath.Dir(cfg.WorkspacePath()), "sessions"))

	contextBuilder := NewContextBuilder(workspace)
	workflowHelper.SetSkillProvider(contextBuilder.SkillsLoader())

	return &AgentLoop{
		bus:            bus,
		provider:       provider,
		workspace:      workspace,
		model:          cfg.Agents.Defaults.Model,
		temperature:    cfg.Agents.Defaults.Temperature,
		contextWindow:  cfg.Agents.Defaults.MaxTokens,
		maxIterations:  cfg.Agents.Defaults.MaxToolIterations,
		sessions:       sessionsManager,
		contextBuilder: contextBuilder,
		tools:          toolsRegistry,
		workflowHelper: workflowHelper,
		mcpRuntime:     mcpRuntime,
		running:        false,
		summarizing:    sync.Map{},
		agentName:      "default",
	}
}

// NewAgentLoopWithDefinition creates a new agent loop with specific agent definition
func NewAgentLoopWithDefinition(cfg *config.Config, bus *bus.MessageBus, provider providers.LLMProvider, agentName string, agentDef *AgentDefinition) *AgentLoop {
	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)

	toolsRegistry := tools.NewToolRegistry()
	toolsRegistry.Register(tools.NewReadFileTool(workspace))
	toolsRegistry.Register(tools.NewWriteFileTool(workspace))
	toolsRegistry.Register(tools.NewListDirTool(workspace))
	toolsRegistry.Register(tools.NewExecTool(workspace))

	// Register workflow tools (always available, no dependencies)
	workflowHelper := workflow.NewWorkflowHelper(workspace, toolsRegistry)
	toolsRegistry.Register(tools.NewWorkflowExecuteTool(workflowHelper))
	toolsRegistry.Register(tools.NewWorkflowSaveTool(workflowHelper))
	toolsRegistry.Register(tools.NewWorkflowListTool(workflowHelper))

	// Register ADB tools (conditional on ADB binary availability)
	if adbHelper, err := tools.NewAdbHelper(workspace); err == nil {
		toolsRegistry.Register(tools.NewAdbDevicesTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbShellTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbTapTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbInputTextTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbScreenshotTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbUIDumpTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbSwipeTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbOpenAppTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbKeyEventTool(adbHelper))
		toolsRegistry.Register(tools.NewAdbRecordWorkflowTool(adbHelper, workflowHelper))
	}

	braveAPIKey := cfg.Tools.Web.Search.APIKey
	toolsRegistry.Register(tools.NewWebSearchTool(braveAPIKey, cfg.Tools.Web.Search.MaxResults))
	toolsRegistry.Register(tools.NewWebFetchTool(50000))
	toolsRegistry.Register(tools.NewSendImageTool(bus, workspace))
	toolsRegistry.Register(tools.NewSendFileTool(bus, workspace))
	toolsRegistry.Register(tools.NewManageAgentTool(workspace))
	toolsRegistry.Register(tools.NewManageMCPTool(workspace))

	var mcpRuntime *mcp.Runtime
	if rt, count, err := tools.RegisterMCPTools(workspace, toolsRegistry); err != nil {
		logger.WarnCF("mcp", "Failed to register MCP tools", map[string]interface{}{"error": err.Error()})
	} else {
		mcpRuntime = rt
		if count > 0 {
			logger.InfoCF("mcp", "MCP tools ready", map[string]interface{}{"count": count})
		}
	}

	// Platform messaging tools (direct API — no gateway required)
	if cfg.Channels.Telegram.Token != "" {
		toolsRegistry.Register(tools.NewTelegramSendTool(cfg.Channels.Telegram.Token, workspace))
	}
	if cfg.Channels.Discord.Token != "" {
		toolsRegistry.Register(tools.NewDiscordSendTool(cfg.Channels.Discord.Token, workspace))
	}
	toolsRegistry.Register(tools.NewWhatsAppSendTool(bus, workspace))

	sessionsManager := session.NewSessionManager(filepath.Join(filepath.Dir(cfg.WorkspacePath()), "sessions"))

	// Use agent definition values, fallback to config defaults
	model := agentDef.Model
	temperature := agentDef.Temperature
	if temperature == 0 {
		temperature = cfg.Agents.Defaults.Temperature
	}
	maxTokens := agentDef.MaxTokens
	if maxTokens == 0 {
		maxTokens = cfg.Agents.Defaults.MaxTokens
	}

	// Use agent-specific prompt dir if PromptFile is set
	var contextBuilder *ContextBuilder
	if agentDef.PromptFile != "" {
		contextBuilder = NewContextBuilderWithAgentDir(workspace, agentDef.PromptFile)
	} else {
		contextBuilder = NewContextBuilder(workspace)
	}

	workflowHelper.SetSkillProvider(contextBuilder.SkillsLoader())

	return &AgentLoop{
		bus:            bus,
		provider:       provider,
		workspace:      workspace,
		model:          model,
		temperature:    temperature,
		contextWindow:  maxTokens,
		maxIterations:  cfg.Agents.Defaults.MaxToolIterations,
		sessions:       sessionsManager,
		contextBuilder: contextBuilder,
		tools:          toolsRegistry,
		workflowHelper: workflowHelper,
		mcpRuntime:     mcpRuntime,
		running:        false,
		summarizing:    sync.Map{},
		agentName:      agentName,
	}
}

func (al *AgentLoop) Run(ctx context.Context) error {
	al.running = true

	for al.running {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := al.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			response, err := al.processMessage(ctx, msg)
			if err != nil {
				response = fmt.Sprintf("Error processing message: %v", err)
			}

			if response != "" {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: response,
				})
			}
		}
	}

	return nil
}

func (al *AgentLoop) Stop() {
	al.running = false
	if al.mcpRuntime != nil {
		al.mcpRuntime.Close()
	}
}

func (al *AgentLoop) ClearSession(sessionKey string) {
	al.sessions.ClearSession(sessionKey)
}

func (al *AgentLoop) Model() string {
	return al.model
}

func (al *AgentLoop) AgentName() string {
	return al.agentName
}

func (al *AgentLoop) Sessions() *session.SessionManager {
	return al.sessions
}

func (al *AgentLoop) ProcessDirect(ctx context.Context, content string, media []string, sessionKey string) (string, error) {
	msg := bus.InboundMessage{
		Channel:    "cli",
		SenderID:   "user",
		ChatID:     "direct",
		Content:    content,
		Media:      media,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

// ProcessDirectStream processes a message with streaming for the final response.
// Tool iterations use non-streaming Chat(); only the final LLM call streams.
func (al *AgentLoop) ProcessDirectStream(ctx context.Context, content string, media []string, sessionKey string, callback providers.StreamCallback) error {
	msg := bus.InboundMessage{
		Channel:    "web",
		SenderID:   "user",
		ChatID:     "web",
		Content:    content,
		Media:      media,
		SessionKey: sessionKey,
	}

	logger.DebugCF("agent", "Processing stream message", map[string]interface{}{
		"session_key": msg.SessionKey,
	})

	history := al.sessions.GetHistory(msg.SessionKey)
	summary := al.sessions.GetSummary(msg.SessionKey)

	metadata := map[string]string{
		"channel":    msg.Channel,
		"channel_id": msg.ChatID,
	}

	messages := al.contextBuilder.BuildMessages(
		history,
		summary,
		msg.Content,
		msg.Media,
		metadata,
	)

	iteration := 0

	for iteration < al.maxIterations {
		iteration++

		toolDefs := al.tools.GetDefinitions()
		providerToolDefs := make([]providers.ToolDefinition, 0, len(toolDefs))
		for _, td := range toolDefs {
			providerToolDefs = append(providerToolDefs, providers.ToolDefinition{
				Type: td["type"].(string),
				Function: providers.ToolFunctionDefinition{
					Name:        td["function"].(map[string]interface{})["name"].(string),
					Description: td["function"].(map[string]interface{})["description"].(string),
					Parameters:  td["function"].(map[string]interface{})["parameters"].(map[string]interface{}),
				},
			})
		}

		// Non-streaming call for tool iterations
		response, err := al.provider.Chat(ctx, messages, providerToolDefs, al.model, map[string]interface{}{
			"max_tokens":  al.contextWindow,
			"temperature": al.temperature,
		})

		if err != nil {
			return fmt.Errorf("LLM call failed: %w", err)
		}

		if len(response.ToolCalls) == 0 {
			// No tool calls - this is the final response.
			// If we already got content from Chat(), stream it character-by-character
			// (provider already returned full content, so we just emit it)
			if response.Content != "" {
				// Use streaming for the final call instead
				// Re-do the last call with streaming
				err := al.provider.ChatStream(ctx, messages, al.model, map[string]interface{}{
					"max_tokens":  al.contextWindow,
					"temperature": al.temperature,
				}, callback)
				if err != nil {
					// Fallback: emit the non-streamed content
					callback(providers.StreamChunk{Content: response.Content})
					callback(providers.StreamChunk{Done: true})
				}
			} else {
				callback(providers.StreamChunk{Content: "I've completed processing but have no response to give."})
				callback(providers.StreamChunk{Done: true})
			}

			// Save to session
			finalContent := response.Content
			if finalContent == "" {
				finalContent = "I've completed processing but have no response to give."
			}
			al.sessions.AddMessage(msg.SessionKey, "user", msg.Content)
			al.sessions.AddMessage(msg.SessionKey, "assistant", finalContent)

			newHistory := al.sessions.GetHistory(msg.SessionKey)
			tokenEstimate := al.estimateTokens(newHistory)
			threshold := al.contextWindow * 75 / 100

			if len(newHistory) > 20 || tokenEstimate > threshold {
				if _, loading := al.summarizing.LoadOrStore(msg.SessionKey, true); !loading {
					go func() {
						defer al.summarizing.Delete(msg.SessionKey)
						al.summarizeSession(msg.SessionKey)
					}()
				}
			}

			al.sessions.Save(al.sessions.GetOrCreate(msg.SessionKey))
			return nil
		}

		// Handle tool calls (non-streaming)
		assistantMsg := providers.Message{
			Role:    "assistant",
			Content: response.Content,
		}

		for _, tc := range response.ToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &providers.FunctionCall{
					Name:      tc.Name,
					Arguments: string(argumentsJSON),
				},
			})
		}
		messages = append(messages, assistantMsg)

		for _, tc := range response.ToolCalls {
			logger.DebugCF("agent", "Executing tool (stream mode)", map[string]interface{}{
				"tool_name": tc.Name,
				"tool_id":   truncateString(tc.ID, 80),
				"arguments": truncateString(mustJSON(tc.Arguments), 300),
			})

			result, err := al.tools.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)
		}
	}

	// Max iterations reached - stream the final content we have
	callback(providers.StreamChunk{Content: "I've completed processing but have no response to give."})
	callback(providers.StreamChunk{Done: true})

	al.sessions.AddMessage(msg.SessionKey, "user", msg.Content)
	al.sessions.AddMessage(msg.SessionKey, "assistant", "I've completed processing but have no response to give.")
	al.sessions.Save(al.sessions.GetOrCreate(msg.SessionKey))

	return nil
}

func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	logger.DebugCF("agent", "Processing message", map[string]interface{}{
		"channel":     msg.Channel,
		"sender_id":   msg.SenderID,
		"chat_id":     msg.ChatID,
		"session_key": msg.SessionKey,
		"has_media":   len(msg.Media) > 0,
	})

	history := al.sessions.GetHistory(msg.SessionKey)
	summary := al.sessions.GetSummary(msg.SessionKey)

	// Ensure metadata has channel information
	metadata := msg.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	if metadata["channel"] == "" {
		metadata["channel"] = msg.Channel
	}
	if metadata["channel_id"] == "" {
		metadata["channel_id"] = msg.ChatID
	}

	messages := al.contextBuilder.BuildMessages(
		history,
		summary,
		msg.Content,
		msg.Media, // Pass media for multimodal support (images, documents, audio, video)
		metadata,  // Pass conversation context for send tools
	)

	iteration := 0
	var finalContent string

	for iteration < al.maxIterations {
		iteration++

		toolDefs := al.tools.GetDefinitions()
		providerToolDefs := make([]providers.ToolDefinition, 0, len(toolDefs))
		for _, td := range toolDefs {
			providerToolDefs = append(providerToolDefs, providers.ToolDefinition{
				Type: td["type"].(string),
				Function: providers.ToolFunctionDefinition{
					Name:        td["function"].(map[string]interface{})["name"].(string),
					Description: td["function"].(map[string]interface{})["description"].(string),
					Parameters:  td["function"].(map[string]interface{})["parameters"].(map[string]interface{}),
				},
			})
		}

		logger.DebugCF("agent", "Calling LLM", map[string]interface{}{
			"iteration": iteration,
			"model":     al.model,
			"tools":     len(providerToolDefs),
		})

		response, err := al.provider.Chat(ctx, messages, providerToolDefs, al.model, map[string]interface{}{
			"max_tokens":  al.contextWindow,
			"temperature": al.temperature,
		})

		if err != nil {
			logger.ErrorCF("agent", "LLM call failed", map[string]interface{}{
				"error": err.Error(),
			})
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		logger.DebugCF("agent", "LLM response received", map[string]interface{}{
			"has_content":     response.Content != "",
			"tool_calls":      len(response.ToolCalls),
			"tool_names":      toolCallNames(response.ToolCalls),
			"content_preview": truncateString(response.Content, 100),
		})

		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			break
		}

		assistantMsg := providers.Message{
			Role:    "assistant",
			Content: response.Content,
		}

		for _, tc := range response.ToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &providers.FunctionCall{
					Name:      tc.Name,
					Arguments: string(argumentsJSON),
				},
			})
		}
		messages = append(messages, assistantMsg)

		for _, tc := range response.ToolCalls {
			logger.DebugCF("agent", "Executing tool", map[string]interface{}{
				"tool_name": tc.Name,
				"tool_id":   truncateString(tc.ID, 80),
				"arguments": truncateString(mustJSON(tc.Arguments), 300),
			})

			result, err := al.tools.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				logger.ErrorCF("agent", "Tool execution failed", map[string]interface{}{
					"tool_name": tc.Name,
					"error":     err.Error(),
				})
				result = fmt.Sprintf("Error: %v", err)
			} else {
				logger.DebugCF("agent", "Tool execution completed", map[string]interface{}{
					"tool_name":      tc.Name,
					"result_preview": truncateString(result, 300),
				})
			}

			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)
		}
	}

	if finalContent == "" {
		finalContent = "I've completed processing but have no response to give."
	}

	al.sessions.AddMessage(msg.SessionKey, "user", msg.Content)
	al.sessions.AddMessage(msg.SessionKey, "assistant", finalContent)

	// Context compression logic
	newHistory := al.sessions.GetHistory(msg.SessionKey)

	// Token Awareness (Dynamic)
	// Trigger if history > 20 messages OR estimated tokens > 75% of context window
	tokenEstimate := al.estimateTokens(newHistory)
	threshold := al.contextWindow * 75 / 100

	if len(newHistory) > 20 || tokenEstimate > threshold {
		if _, loading := al.summarizing.LoadOrStore(msg.SessionKey, true); !loading {
			go func() {
				defer al.summarizing.Delete(msg.SessionKey)
				al.summarizeSession(msg.SessionKey)
			}()
		}
	}

	al.sessions.Save(al.sessions.GetOrCreate(msg.SessionKey))

	return finalContent, nil
}

func (al *AgentLoop) summarizeSession(sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	history := al.sessions.GetHistory(sessionKey)
	summary := al.sessions.GetSummary(sessionKey)

	// Keep last 4 messages for continuity
	if len(history) <= 4 {
		return
	}

	toSummarize := history[:len(history)-4]

	// Oversized Message Guard (Dynamic)
	// Skip messages larger than 50% of context window to prevent summarizer overflow.
	maxMessageTokens := al.contextWindow / 2
	validMessages := make([]providers.Message, 0)
	omitted := false

	for _, m := range toSummarize {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// Estimate tokens for this message
		msgTokens := al.getContentLength(m.Content) / 4
		if msgTokens > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, m)
	}

	if len(validMessages) == 0 {
		return
	}

	// Multi-Part Summarization
	// Split into two parts if history is significant
	var finalSummary string
	if len(validMessages) > 10 {
		mid := len(validMessages) / 2
		part1 := validMessages[:mid]
		part2 := validMessages[mid:]

		s1, _ := al.summarizeBatch(ctx, part1, "")
		s2, _ := al.summarizeBatch(ctx, part2, "")

		// Merge them
		mergePrompt := fmt.Sprintf("Merge these two conversation summaries into one cohesive summary:\n\n1: %s\n\n2: %s", s1, s2)
		resp, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, al.model, map[string]interface{}{
			"max_tokens":  1024,
			"temperature": 0.3,
		})
		if err == nil {
			finalSummary = resp.Content
		} else {
			finalSummary = s1 + " " + s2
		}
	} else {
		finalSummary, _ = al.summarizeBatch(ctx, validMessages, summary)
	}

	if omitted && finalSummary != "" {
		finalSummary += "\n[Note: Some oversized messages were omitted from this summary for efficiency.]"
	}

	if finalSummary != "" {
		al.sessions.SetSummary(sessionKey, finalSummary)
		al.sessions.TruncateHistory(sessionKey, 4)
		al.sessions.Save(al.sessions.GetOrCreate(sessionKey))
	}
}

func (al *AgentLoop) summarizeBatch(ctx context.Context, batch []providers.Message, existingSummary string) (string, error) {
	prompt := "Provide a concise summary of this conversation segment, preserving core context and key points.\n"
	if existingSummary != "" {
		prompt += "Existing context: " + existingSummary + "\n"
	}
	prompt += "\nCONVERSATION:\n"
	for _, m := range batch {
		prompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	response, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, al.model, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

func (al *AgentLoop) estimateTokens(messages []providers.Message) int {
	total := 0
	for _, m := range messages {
		total += al.getContentLength(m.Content) / 4 // Simple heuristic: 4 chars per token
	}
	return total
}

// getContentLength returns the character length of message content
// Handles both string and multimodal content blocks
func (al *AgentLoop) getContentLength(content interface{}) int {
	switch v := content.(type) {
	case string:
		return len(v)
	case []providers.ContentBlock:
		total := 0
		for _, block := range v {
			if block.Type == "text" {
				total += len(block.Text)
			} else if block.Type == "image_url" {
				// Estimate tokens for images (varies by model, rough estimate)
				total += 1000 // Rough estimate for image tokens
			}
		}
		return total
	default:
		return 0
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func toolCallNames(toolCalls []providers.ToolCall) []string {
	names := make([]string, 0, len(toolCalls))
	for _, tc := range toolCalls {
		names = append(names, tc.Name)
	}
	return names
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
