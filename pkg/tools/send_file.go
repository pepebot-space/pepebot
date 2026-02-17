package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/providers"
)

type SendFileTool struct {
	bus *bus.MessageBus
}

func NewSendFileTool(bus *bus.MessageBus) *SendFileTool {
	return &SendFileTool{
		bus: bus,
	}
}

func (t *SendFileTool) Name() string {
	return "send_file"
}

func (t *SendFileTool) Description() string {
	return "Send a file to a chat channel. Supports images, PDFs, documents, audio, video, and other file types. Use this when you need to share any file with the user."
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
				"description": "URL or local file path of the file to send (supports images, PDFs, documents, audio, video, etc.)",
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
