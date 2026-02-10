package channels

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/anak10thn/pepebot/pkg/bus"
	"github.com/anak10thn/pepebot/pkg/config"
	"github.com/anak10thn/pepebot/pkg/logger"
)

type DiscordChannel struct {
	*BaseChannel
	session *discordgo.Session
	config  config.DiscordConfig
}

func NewDiscordChannel(cfg config.DiscordConfig, bus *bus.MessageBus) (*DiscordChannel, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	base := NewBaseChannel("discord", cfg, bus, cfg.AllowFrom)

	return &DiscordChannel{
		BaseChannel: base,
		session:     session,
		config:      cfg,
	}, nil
}

func (c *DiscordChannel) Start(ctx context.Context) error {
	logger.InfoC("discord", "Starting Discord bot")

	c.session.AddHandler(c.handleMessage)

	if err := c.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord session: %w", err)
	}

	c.setRunning(true)

	botUser, err := c.session.User("@me")
	if err != nil {
		return fmt.Errorf("failed to get bot user: %w", err)
	}
	logger.InfoCF("discord", "Discord bot connected", map[string]interface{}{
		"username": botUser.Username,
		"user_id":  botUser.ID,
	})

	return nil
}

func (c *DiscordChannel) Stop(ctx context.Context) error {
	logger.InfoC("discord", "Stopping Discord bot")
	c.setRunning(false)

	if err := c.session.Close(); err != nil {
		return fmt.Errorf("failed to close discord session: %w", err)
	}

	return nil
}

func (c *DiscordChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("discord bot not running")
	}

	channelID := msg.ChatID
	if channelID == "" {
		return fmt.Errorf("channel ID is empty")
	}

	message := msg.Content

	// Discord has a 2000 character limit per message
	const maxLength = 2000

	// If message is short enough, send it directly
	if len(message) <= maxLength {
		if _, err := c.session.ChannelMessageSend(channelID, message); err != nil {
			return fmt.Errorf("failed to send discord message: %w", err)
		}
		return nil
	}

	// Split long message into multiple parts
	parts := splitMessage(message, maxLength)

	logger.DebugCF("discord", "Splitting long message", map[string]interface{}{
		"original_length": len(message),
		"parts":           len(parts),
	})

	// Send each part
	for i, part := range parts {
		// Add part indicator if multiple parts
		if len(parts) > 1 {
			partHeader := fmt.Sprintf("*[Part %d/%d]*\n", i+1, len(parts))
			part = partHeader + part
		}

		if _, err := c.session.ChannelMessageSend(channelID, part); err != nil {
			return fmt.Errorf("failed to send discord message part %d: %w", i+1, err)
		}

		// Small delay between messages to avoid rate limiting
		if i < len(parts)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

func (c *DiscordChannel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Author == nil {
		return
	}

	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if this is a DM (direct message)
	isDM := m.GuildID == ""

	// In groups, only respond if:
	// 1. Bot is mentioned
	// 2. Message is a reply to bot's message
	if !isDM {
		isMentioned := false
		isReplyToBot := false

		// Check if bot is mentioned
		for _, mention := range m.Mentions {
			if mention.ID == s.State.User.ID {
				isMentioned = true
				break
			}
		}

		// Check if message is a reply to bot
		if m.ReferencedMessage != nil && m.ReferencedMessage.Author != nil {
			if m.ReferencedMessage.Author.ID == s.State.User.ID {
				isReplyToBot = true
			}
		}

		// Ignore message if bot is not mentioned or replied to
		if !isMentioned && !isReplyToBot {
			logger.DebugCF("discord", "Ignoring message (not mentioned or replied)", map[string]interface{}{
				"sender":     m.Author.Username,
				"channel_id": m.ChannelID,
				"guild_id":   m.GuildID,
			})
			return
		}
	}

	// Add frog reaction to indicate message has been seen
	if err := s.MessageReactionAdd(m.ChannelID, m.ID, "🐸"); err != nil {
		logger.DebugCF("discord", "Failed to add reaction", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail the whole message processing just because reaction failed
	}

	senderID := m.Author.ID
	senderName := m.Author.Username
	if m.Author.Discriminator != "" && m.Author.Discriminator != "0" {
		senderName += "#" + m.Author.Discriminator
	}

	content := m.Content
	mediaPaths := []string{}

	// Remove bot mention from content if present
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			content = removeMention(content, mention.ID)
		}
	}

	for _, attachment := range m.Attachments {
		mediaPaths = append(mediaPaths, attachment.URL)
		if content != "" {
			content += "\n"
		}
		content += fmt.Sprintf("[attachment: %s]", attachment.URL)
	}

	if content == "" && len(mediaPaths) == 0 {
		return
	}

	if content == "" {
		content = "[media only]"
	}

	logger.DebugCF("discord", "Processing message", map[string]interface{}{
		"sender_name": senderName,
		"sender_id":   senderID,
		"is_dm":       isDM,
		"preview":     truncateString(content, 50),
	})

	metadata := map[string]string{
		"message_id":   m.ID,
		"user_id":      senderID,
		"username":     m.Author.Username,
		"display_name": senderName,
		"guild_id":     m.GuildID,
		"channel_id":   m.ChannelID,
		"is_dm":        fmt.Sprintf("%t", isDM),
	}

	c.HandleMessage(senderID, m.ChannelID, content, mediaPaths, metadata)
}

// removeMention removes bot mention tags from message content
func removeMention(content string, botID string) string {
	// Remove <@botID> and <@!botID> patterns
	content = strings.ReplaceAll(content, "<@"+botID+">", "")
	content = strings.ReplaceAll(content, "<@!"+botID+">", "")

	// Trim extra whitespace
	content = strings.TrimSpace(content)

	return content
}

// splitMessage splits a long message into chunks that fit Discord's character limit
func splitMessage(message string, maxLength int) []string {
	// Reserve space for part indicator like "[Part 1/3]\n"
	effectiveMaxLength := maxLength - 20

	if len(message) <= effectiveMaxLength {
		return []string{message}
	}

	var parts []string
	remaining := message

	for len(remaining) > 0 {
		if len(remaining) <= effectiveMaxLength {
			parts = append(parts, remaining)
			break
		}

		// Find a good split point
		splitPoint := effectiveMaxLength
		chunk := remaining[:splitPoint]

		// Try to split at a newline
		if lastNewline := strings.LastIndex(chunk, "\n"); lastNewline > effectiveMaxLength/2 {
			splitPoint = lastNewline + 1
		} else if lastSpace := strings.LastIndex(chunk, " "); lastSpace > effectiveMaxLength/2 {
			// Try to split at a space
			splitPoint = lastSpace + 1
		} else if lastPeriod := strings.LastIndex(chunk, ". "); lastPeriod > effectiveMaxLength/2 {
			// Try to split at a sentence
			splitPoint = lastPeriod + 2
		}

		parts = append(parts, strings.TrimSpace(remaining[:splitPoint]))
		remaining = strings.TrimSpace(remaining[splitPoint:])
	}

	return parts
}
