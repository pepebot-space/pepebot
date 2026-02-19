package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/providers"
)

type SendFileTool struct {
	bus       *bus.MessageBus
	workspace string
}

func NewSendFileTool(bus *bus.MessageBus, workspace string) *SendFileTool {
	return &SendFileTool{
		bus:       bus,
		workspace: workspace,
	}
}

func (t *SendFileTool) Name() string {
	return "send_file"
}

func (t *SendFileTool) Description() string {
	return "Send a file to a chat channel. Supports images, PDFs, documents, audio, video, and other file types. IMPORTANT: Always use the full absolute path for local files (e.g., /Users/.../.pepebot/workspace/file.pdf)."
}

func (t *SendFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "The channel to send to (e.g., 'discord', 'telegram', 'whatsapp')",
			},
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "The chat/channel ID to send the file to",
			},
			"file_url": map[string]interface{}{
				"type":        "string",
				"description": "Full absolute file path or URL of the file to send. Must be a complete path like /Users/.../.pepebot/workspace/file.pdf",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Optional caption/message to send with the file",
			},
		},
		"required": []string{"channel", "chat_id", "file_url"},
	}
}

func (t *SendFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract parameters
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel must be a string")
	}

	chatID, ok := args["chat_id"].(string)
	if !ok {
		return "", fmt.Errorf("chat_id must be a string")
	}

	fileURL, ok := args["file_url"].(string)
	if !ok {
		return "", fmt.Errorf("file_url must be a string")
	}

	caption := ""
	if c, ok := args["caption"].(string); ok {
		caption = c
	}

	// Resolve and validate path for local files
	fileURL = t.resolveFilePath(fileURL)

	// Detect file type
	fileType, mimeType := providers.DetectFileType(fileURL)
	fileName := providers.GetFileName(fileURL)

	// Publish outbound message with media
	t.bus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: caption,
		Media:   []string{fileURL},
	})

	result := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("File sent to %s channel %s", channel, chatID),
		"file_type": string(fileType),
		"mime_type": mimeType,
		"file_name": fileName,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// resolveFilePath resolves a file path to an absolute path.
// If it's a URL, return as-is. If relative, resolve against workspace and common directories.
func (t *SendFileTool) resolveFilePath(path string) string {
	// URLs pass through unchanged
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "data:") {
		return path
	}

	// Already absolute and exists — use as-is
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find the file in common locations
	basename := filepath.Base(path)
	candidates := []string{
		path,                                   // as given
		filepath.Join(t.workspace, path),       // relative to workspace
		filepath.Join(t.workspace, basename),   // just filename in workspace
		filepath.Join("/tmp", basename),         // /tmp
		filepath.Join("/tmp/pepebot_whatsapp", basename), // whatsapp downloads
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}

	// If nothing found but path is relative, at least make it absolute via workspace
	if !filepath.IsAbs(path) {
		return filepath.Join(t.workspace, path)
	}

	return path
}
