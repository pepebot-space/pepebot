package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/bus"
)

type SendImageTool struct {
	bus       *bus.MessageBus
	workspace string
}

func NewSendImageTool(bus *bus.MessageBus, workspace string) *SendImageTool {
	return &SendImageTool{
		bus:       bus,
		workspace: workspace,
	}
}

func (t *SendImageTool) Name() string {
	return "send_image"
}

func (t *SendImageTool) Description() string {
	return "Send an image to a chat channel. Use this when you need to show visual content to the user. IMPORTANT: Always use the full absolute path for local files (e.g., /Users/.../.pepebot/workspace/screenshot.png)."
}

func (t *SendImageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "The channel to send to (e.g., 'discord', 'telegram')",
			},
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "The chat/channel ID to send the image to",
			},
			"image_url": map[string]interface{}{
				"type":        "string",
				"description": "Full absolute file path or URL of the image to send. Must be a complete path like /Users/.../.pepebot/workspace/screenshot.png",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Optional caption/message to send with the image",
			},
		},
		"required": []string{"channel", "chat_id", "image_url"},
	}
}

func (t *SendImageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Extract parameters
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel must be a string")
	}

	chatID, ok := args["chat_id"].(string)
	if !ok {
		return "", fmt.Errorf("chat_id must be a string")
	}

	imageURL, ok := args["image_url"].(string)
	if !ok {
		return "", fmt.Errorf("image_url must be a string")
	}

	caption := ""
	if c, ok := args["caption"].(string); ok {
		caption = c
	}

	// Resolve and validate path for local files
	imageURL = t.resolveFilePath(imageURL)

	// Publish outbound message with media
	t.bus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: caption,
		Media:   []string{imageURL},
	})

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Image sent to %s channel %s", channel, chatID),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// resolveFilePath resolves a file path to an absolute path.
// If it's a URL, return as-is. If relative, resolve against workspace and common directories.
func (t *SendImageTool) resolveFilePath(path string) string {
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
