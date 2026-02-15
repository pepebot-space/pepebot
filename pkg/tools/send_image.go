package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pepebot-space/pepebot/pkg/bus"
)

type SendImageTool struct {
	bus *bus.MessageBus
}

func NewSendImageTool(bus *bus.MessageBus) *SendImageTool {
	return &SendImageTool{
		bus: bus,
	}
}

func (t *SendImageTool) Name() string {
	return "send_image"
}

func (t *SendImageTool) Description() string {
	return "Send an image to a chat channel. Use this when you need to show visual content to the user."
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
				"description": "URL of the image to send, or local file path",
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
