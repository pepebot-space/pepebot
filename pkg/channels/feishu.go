package channels

import (
	"context"
	"fmt"
	"log"

	"github.com/anak10thn/pepebot/pkg/bus"
	"github.com/anak10thn/pepebot/pkg/config"
)

type FeishuChannel struct {
	*BaseChannel
	config config.FeishuConfig
}

func NewFeishuChannel(cfg config.FeishuConfig, bus *bus.MessageBus) (*FeishuChannel, error) {
	base := NewBaseChannel("feishu", cfg, bus, cfg.AllowFrom)

	return &FeishuChannel{
		BaseChannel: base,
		config:      cfg,
	}, nil
}

func (c *FeishuChannel) Start(ctx context.Context) error {
	log.Println("Feishu channel started")
	c.setRunning(true)
	return nil
}

func (c *FeishuChannel) Stop(ctx context.Context) error {
	log.Println("Feishu channel stopped")
	c.setRunning(false)
	return nil
}

func (c *FeishuChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("feishu channel not running")
	}

	htmlContent := markdownToFeishuCard(msg.Content)

	log.Printf("Feishu send to %s: %s", msg.ChatID, truncateString(htmlContent, 100))

	return nil
}

func (c *FeishuChannel) handleIncomingMessage(data map[string]interface{}) {
	senderID, _ := data["sender_id"].(string)
	chatID, _ := data["chat_id"].(string)
	content, _ := data["content"].(string)

	log.Printf("Feishu message from %s: %s...", senderID, truncateString(content, 50))

	metadata := make(map[string]string)
	if messageID, ok := data["message_id"].(string); ok {
		metadata["message_id"] = messageID
	}
	if userName, ok := data["sender_name"].(string); ok {
		metadata["sender_name"] = userName
	}

	c.HandleMessage(senderID, chatID, content, nil, metadata)
}

func markdownToFeishuCard(markdown string) string {
	return markdown
}
