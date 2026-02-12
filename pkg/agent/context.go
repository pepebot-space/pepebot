package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anak10thn/pepebot/pkg/providers"
	"github.com/anak10thn/pepebot/pkg/skills"
)

type ContextBuilder struct {
	workspace      string
	agentPromptDir string
	skillsLoader   *skills.SkillsLoader
}

func NewContextBuilder(workspace string) *ContextBuilder {
	builtinSkillsDir := filepath.Join(filepath.Dir(workspace), "pepebot", "skills")
	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, builtinSkillsDir),
	}
}

// NewContextBuilderWithAgentDir creates a ContextBuilder that checks agent-specific dir first
func NewContextBuilderWithAgentDir(workspace, agentPromptDir string) *ContextBuilder {
	builtinSkillsDir := filepath.Join(filepath.Dir(workspace), "pepebot", "skills")
	return &ContextBuilder{
		workspace:      workspace,
		agentPromptDir: agentPromptDir,
		skillsLoader:   skills.NewSkillsLoader(workspace, builtinSkillsDir),
	}
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
- Send images to chat channels (Discord, Telegram, etc.)
- View and analyze images sent by users
- Spawn subagents for complex background tasks

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
When remembering something, write to %s/memory/MEMORY.md`,
		now, workspacePath, workspacePath, workspacePath, workspacePath, workspacePath)
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"TOOLS.md",
		"IDENTITY.md",
		"MEMORY.md",
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

	skillsSummary := cb.skillsLoader.BuildSkillsSummary()
	if skillsSummary != "" {
		systemPrompt += "\n\n## Available Skills\n\n" + skillsSummary
	}

	skillsContent := cb.loadSkills()
	if skillsContent != "" {
		systemPrompt += "\n\n" + skillsContent
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

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills(true)
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// buildUserMessage creates a user message with optional media attachments for vision support
func (cb *ContextBuilder) buildUserMessage(text string, media []string) providers.Message {
	// If no media, return simple text message
	if len(media) == 0 {
		return providers.Message{
			Role:    "user",
			Content: text,
		}
	}

	// Build multimodal content with text and images
	content := []providers.ContentBlock{}

	// Add text if present
	if text != "" {
		content = append(content, providers.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	// Add images
	for _, mediaURL := range media {
		content = append(content, providers.ContentBlock{
			Type: "image_url",
			ImageURL: &providers.ImageURL{
				URL:    mediaURL,
				Detail: "auto", // Let the model decide the detail level
			},
		})
	}

	return providers.Message{
		Role:    "user",
		Content: content,
	}
}
