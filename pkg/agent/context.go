package agent

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/memory"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/skills"
)

type ContextBuilder struct {
	notes          *memory.Store
	profile        *memory.Store
	workspace      string
	agentPromptDir string
	skillsLoader   *skills.SkillsLoader
}

func NewContextBuilder(workspace string) *ContextBuilder {
	builtinSkillsDir := filepath.Join(filepath.Dir(workspace), "pepebot", "skills")
	loader := skills.NewSkillsLoader(workspace, builtinSkillsDir)
	if err := loader.SyncMCPRegistry(); err != nil {
		logger.WarnCF("agent", "Failed to sync MCP servers from skills", map[string]interface{}{"error": err.Error()})
	}
	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: loader,
	}
}

// NewContextBuilderWithAgentDir creates a ContextBuilder that checks agent-specific dir first
func NewContextBuilderWithAgentDir(workspace, agentPromptDir string) *ContextBuilder {
	builtinSkillsDir := filepath.Join(filepath.Dir(workspace), "pepebot", "skills")
	loader := skills.NewSkillsLoader(workspace, builtinSkillsDir)
	if err := loader.SyncMCPRegistry(); err != nil {
		logger.WarnCF("agent", "Failed to sync MCP servers from skills", map[string]interface{}{"error": err.Error()})
	}
	return &ContextBuilder{
		workspace:      workspace,
		agentPromptDir: agentPromptDir,
		skillsLoader:   loader,
	}
}

// SkillsLoader returns the underlying skills loader for external use (e.g. workflow skill steps)
// SetMemory attaches the bounded memory stores. Without them the builder falls
// back to loading MEMORY.md and USER.md as plain bootstrap files, which is what
// callers that have no memory configured (subagents, Live sessions) still do.
func (cb *ContextBuilder) SetMemory(notes, profile *memory.Store) {
	cb.notes, cb.profile = notes, profile
}

// MemoryPrompt renders the memory block for the system prompt: the entries plus
// how full each store is. The capacity line is for the agent, not decoration —
// it is what lets it consolidate before a write fails.
func (cb *ContextBuilder) MemoryPrompt() string {
	if cb.notes == nil {
		return ""
	}
	blocks := []string{}
	for _, s := range []*memory.Store{cb.notes, cb.profile} {
		if r := s.Render(); r != "" {
			blocks = append(blocks, r)
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	return "## Memory\n\n" + strings.Join(blocks, "\n\n") + "\n"
}

func (cb *ContextBuilder) SkillsLoader() *skills.SkillsLoader {
	return cb.skillsLoader
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))

	return fmt.Sprintf(`# pepebot 🐸

You are pepebot, a helpful AI assistant. You have access to tools that allow you to:
- Read, write, and edit files
- Execute shell commands
- Search the web and fetch web pages
- Send messages to users on chat channels
- Send files to chat channels (images, PDFs, documents, audio, video) - use send_file or send_image tools
- View and analyze files sent by users (images, documents, PDFs, audio, video)
- Spawn subagents for complex background tasks
- Manage agent registry via manage_agent (register/list/enable/disable/remove/create_bootstrap/assign_skill/call)
- Manage MCP server registry (stdio, remote SSE, remote HTTP) via the manage_mcp tool

## Current Time
%s

## Workspace
Your workspace is at: %s
- Memory files: %s/memory/MEMORY.md
- Daily notes: %s/memory/2006-01-02.md
- Custom skills: %s/skills/{skill-name}/SKILL.md

## Weather Information
When users ask about weather, use the web_fetch tool with wttr.in URLs:
- Current weather: https://wttr.in/{city}?format=j1
- Jakarta: https://wttr.in/Jakarta?format=j1
- Beijing: https://wttr.in/Beijing?format=j1
- Shanghai: https://wttr.in/Shanghai?format=j1
- New York: https://wttr.in/New_York?format=j1
- London: https://wttr.in/London?format=j1
- Tokyo: https://wttr.in/Tokyo?format=j1

IMPORTANT: When responding to direct questions or conversations, reply directly with your text response.
Only use the 'message' tool when you need to send a message to a specific chat channel (like WhatsApp).
For normal conversation, just respond with text - do not call the message tool.

Always be helpful, accurate, and concise. When using tools, explain what you're doing.

## Workflow Tools Policy
IMPORTANT: Only use workflow tools (workflow_save, workflow_execute, workflow_list, adb_record_workflow) when the user EXPLICITLY asks you to create, save, record, list, or run a workflow.
Do NOT proactively create or suggest creating workflows. Do NOT save multi-step operations as workflows unless the user directly requests it.
If the user asks to "call", "use", "switch", or "delegate to" another agent but does NOT mention workflow, do not create a workflow. Handle the request directly when possible, or explain briefly that agent delegation is only available through workflow steps.

When (and only when) creating workflows, use the correct step type:
- When the user says "use skill X" or "with skill X": use a SKILL step ({"skill":"X", "goal":"..."}) — do NOT manually replicate the skill's commands via tool/exec steps.
- When the user says "use agent X" or "delegate to agent X": use an AGENT step ({"agent":"X", "goal":"..."}).
- For direct tool calls: use TOOL step ({"tool":"...", "args":{...}}).
- For LLM decisions: use GOAL step ({"goal":"..."}).

## Memory Instructions
When the user asks you to remember, save, or note something, call the memory tool
- memory(action="add", target="memory", content="...") for facts about projects, environment or conventions
- target="user" for the user's own preferences and communication style
- Correct a wrong entry with action="replace" and fix old_text; drop one with action="remove"
- NEVER just say "I'll remember that" without calling the tool — the information WILL BE LOST
- Both stores are size-limited. If a write fails because a store is full, consolidate two
  related entries into one with replace, or remove something stale, then retry`,
		now, workspacePath, workspacePath, workspacePath, workspacePath)
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"TOOLS.md",
		"IDENTITY.md",
		"memory/MEMORY.md",
	}

	// When memory is bounded, these two are rendered by MemoryPrompt with their
	// capacity header instead of being pasted in raw and unlimited.
	if cb.notes != nil {
		bootstrapFiles = []string{"AGENTS.md", "SOUL.md", "TOOLS.md", "IDENTITY.md"}
	}

	var result string
	for _, filename := range bootstrapFiles {
		// Per-file fallback: check agent dir first, then workspace root
		var data []byte
		var err error

		if cb.agentPromptDir != "" {
			agentPath := filepath.Join(cb.agentPromptDir, filename)
			data, err = os.ReadFile(agentPath)
		}

		if data == nil || err != nil {
			filePath := filepath.Join(cb.workspace, filename)
			data, err = os.ReadFile(filePath)
		}

		if err == nil {
			result += fmt.Sprintf("## %s\n\n%s\n\n", filename, string(data))
		}
	}

	return result
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, metadata map[string]string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPrompt()
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		systemPrompt += "\n\n" + bootstrapContent
	}

	if memoryPrompt := cb.MemoryPrompt(); memoryPrompt != "" {
		systemPrompt += "\n\n" + memoryPrompt
	}

	if skillsPrompt := cb.SkillsPrompt(); skillsPrompt != "" {
		systemPrompt += "\n\n" + skillsPrompt
	}

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	// Add current conversation context
	if metadata != nil && metadata["channel_id"] != "" {
		channel := metadata["channel"]
		if channel == "" {
			channel = "unknown"
		}
		chatID := metadata["channel_id"]

		systemPrompt += fmt.Sprintf("\n\n## Current Conversation Context\n\n")
		systemPrompt += fmt.Sprintf("- Channel: %s\n", channel)
		systemPrompt += fmt.Sprintf("- Chat ID: %s\n", chatID)
		systemPrompt += fmt.Sprintf("\nIMPORTANT: When using the send_image tool, use these values:\n")
		systemPrompt += fmt.Sprintf("- channel: \"%s\"\n", channel)
		systemPrompt += fmt.Sprintf("- chat_id: \"%s\"\n", chatID)
	}

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	// Build user message with optional media (vision support)
	userMessage := cb.buildUserMessage(currentMessage, media)
	messages = append(messages, userMessage)

	return messages
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]interface{}) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		messages = append(messages, msg)
	}
	return messages
}

// SkillsPrompt renders the skills block that goes into a system prompt: every skill's
// name, description and location, and nothing more. Shared with Live sessions so a
// voice conversation knows about the same skills a text one does.
//
// The full SKILL.md bodies deliberately stay out. Injecting all of them does not
// scale — on one deployment 89 skills came to roughly 335k tokens of prompt before
// the user had typed anything, which no context window survives, and every request
// paid for all of them to use at most one. The summary carries each skill's
// <location>, so the agent opens the one it needs with read_file instead.
func (cb *ContextBuilder) SkillsPrompt() string {
	summary := cb.skillsLoader.BuildSkillsSummary()
	if summary == "" {
		return ""
	}

	return "## Available Skills\n\n" + summary + "\n\n" +
		"Each skill's full instructions are in the SKILL.md at its <location>. " +
		"When a task calls for a skill, read that file first with read_file and " +
		"follow it — do not work from the description alone."
}

// convertFileToDataURL converts a local file path to a base64 data URL
// Returns the original URL if it's already an HTTP/HTTPS URL
func convertFileToDataURL(filePath string) string {
	// If it's already a URL or data URL, return as-is
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") || strings.HasPrefix(filePath, "data:") {
		return filePath
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.ErrorCF("agent", "Failed to read media file for base64 encoding", map[string]interface{}{
			"path":  filePath,
			"error": err.Error(),
		})
		return filePath // Return original path as fallback
	}

	// Detect MIME type
	_, mimeType := providers.DetectFileType(filePath)

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	// Create data URL
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	logger.DebugCF("agent", "Converted local file to base64 data URL", map[string]interface{}{
		"path":      filePath,
		"mime_type": mimeType,
		"size":      len(data),
	})

	return dataURL
}

// maxInlineFileBytes caps how much of a remote attachment we base64 into a
// request body. ponytail: hard cap, switch to a provider file-upload API if
// bigger documents ever matter.
const maxInlineFileBytes = 20 << 20

// fetchRemoteAsDataURL downloads an http(s) attachment and returns it as a
// base64 data URL. Returns "" when it cannot be inlined, so the caller can drop
// the block instead of sending one the provider will reject.
func fetchRemoteAsDataURL(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		logger.ErrorCF("agent", "Failed to fetch remote attachment", map[string]interface{}{"url": url, "error": err.Error()})
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.ErrorCF("agent", "Remote attachment fetch returned non-200", map[string]interface{}{"url": url, "status": resp.StatusCode})
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxInlineFileBytes+1))
	if err != nil || len(data) == 0 {
		logger.ErrorCF("agent", "Failed to read remote attachment", map[string]interface{}{"url": url, "error": fmt.Sprintf("%v", err)})
		return ""
	}
	if len(data) > maxInlineFileBytes {
		logger.WarnCF("agent", "Remote attachment too large to inline", map[string]interface{}{"url": url, "limit": maxInlineFileBytes})
		return ""
	}

	_, mimeType := providers.DetectFileType(url)
	if mimeType == "" {
		mimeType = resp.Header.Get("Content-Type")
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// buildUserMessage creates a user message with optional media attachments for multimodal support
func (cb *ContextBuilder) buildUserMessage(text string, media []string) providers.Message {
	// If no media, return simple text message
	if len(media) == 0 {
		return providers.Message{
			Role:    "user",
			Content: text,
		}
	}

	// Build multimodal content with text and files (images, documents, audio, video)
	content := []providers.ContentBlock{}

	// Add text if present
	if text != "" {
		content = append(content, providers.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	// Add media files with automatic type detection
	for _, mediaURL := range media {
		// Convert local file paths to base64 data URLs for LLM providers
		processedURL := convertFileToDataURL(mediaURL)

		fileType, _ := providers.DetectFileType(mediaURL)

		// Everything remote is inlined, images included. Passing the URL through
		// only works if the provider can fetch it, and channel attachments are
		// signed, expiring CDN links (Discord's ?ex=&hm=) that it cannot: z.ai
		// answers every one of them with "图片输入格式/解析错误". Inlining costs
		// request size; not inlining costs the whole request.
		if !strings.HasPrefix(processedURL, "data:") {
			processedURL = fetchRemoteAsDataURL(processedURL)
		}
		if processedURL == "" {
			content = append(content, providers.ContentBlock{
				Type: "text",
				Text: fmt.Sprintf("[attachment could not be read: %s]", mediaURL),
			})
			continue
		}

		switch fileType {
		case providers.FileTypeImage:
			// Images use image_url format
			content = append(content, providers.ContentBlock{
				Type: "image_url",
				ImageURL: &providers.ImageURL{
					URL:    processedURL,
					Detail: "auto", // Let the model decide the detail level
				},
			})
		default:
			// All other file types (documents, audio, video) use file format
			// Format: { "type": "file", "file": { "file_data": "data:mime/type;base64,..." } }
			// Reference: https://developers.openai.com/api/docs/guides/pdf-files
			content = append(content, providers.ContentBlock{
				Type: "file",
				File: &providers.FileData{
					FileData: processedURL,
				},
			})
		}
	}

	return providers.Message{
		Role:    "user",
		Content: content,
	}
}
