package channels

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/anak10thn/pepebot/pkg/bus"
	"github.com/anak10thn/pepebot/pkg/config"
	"github.com/anak10thn/pepebot/pkg/logger"
)

type DiscordChannel struct {
	*BaseChannel
	session        *discordgo.Session
	config         config.DiscordConfig
	typingChannels map[string]chan bool
	typingMutex    sync.RWMutex
}

func NewDiscordChannel(cfg config.DiscordConfig, bus *bus.MessageBus) (*DiscordChannel, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	base := NewBaseChannel("discord", cfg, bus, cfg.AllowFrom)

	return &DiscordChannel{
		BaseChannel:    base,
		session:        session,
		config:         cfg,
		typingChannels: make(map[string]chan bool),
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

	// Stop typing indicator since we're about to send the response
	c.stopTyping(channelID)

	message := msg.Content

	// If there are media attachments, send with files
	if len(msg.Media) > 0 {
		return c.sendWithMedia(channelID, message, msg.Media)
	}

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

	// Start typing indicator to show bot is processing
	// Discord typing indicator lasts ~10 seconds, so we need to keep refreshing it
	stopTyping := make(chan bool, 1)
	go c.keepTyping(s, m.ChannelID, stopTyping)

	// Store the stop channel so we can stop typing when response is sent
	c.storeTypingChannel(m.ChannelID, stopTyping)

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

	// Check for attachments in the referenced message (reply)
	if m.ReferencedMessage != nil && len(m.ReferencedMessage.Attachments) > 0 {
		for _, attachment := range m.ReferencedMessage.Attachments {
			mediaPaths = append(mediaPaths, attachment.URL)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[referenced attachment: %s]", attachment.URL)
		}
	}

	// Check for attachments in the current message
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

// keepTyping continuously sends typing indicator to Discord channel
// Discord typing indicator lasts ~10 seconds, so we refresh every 8 seconds
func (c *DiscordChannel) keepTyping(s *discordgo.Session, channelID string, stop chan bool) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	// Send initial typing indicator
	if err := s.ChannelTyping(channelID); err != nil {
		logger.DebugCF("discord", "Failed to send typing indicator", map[string]interface{}{
			"error":      err.Error(),
			"channel_id": channelID,
		})
		return
	}

	// Keep refreshing typing indicator until stop signal or timeout (2 minutes max)
	timeout := time.After(2 * time.Minute)

	for {
		select {
		case <-stop:
			// Stop signal received, exit goroutine
			return
		case <-timeout:
			// Timeout reached, stop typing
			logger.DebugCF("discord", "Typing indicator timeout", map[string]interface{}{
				"channel_id": channelID,
			})
			return
		case <-ticker.C:
			// Refresh typing indicator
			if err := s.ChannelTyping(channelID); err != nil {
				logger.DebugCF("discord", "Failed to refresh typing indicator", map[string]interface{}{
					"error":      err.Error(),
					"channel_id": channelID,
				})
				return
			}
		}
	}
}

// storeTypingChannel stores the stop channel for a specific Discord channel
func (c *DiscordChannel) storeTypingChannel(channelID string, stop chan bool) {
	c.typingMutex.Lock()
	defer c.typingMutex.Unlock()
	c.typingChannels[channelID] = stop
}

// stopTyping stops the typing indicator for a specific Discord channel
func (c *DiscordChannel) stopTyping(channelID string) {
	c.typingMutex.Lock()
	defer c.typingMutex.Unlock()

	if stop, exists := c.typingChannels[channelID]; exists {
		// Send stop signal (non-blocking)
		select {
		case stop <- true:
		default:
		}
		// Remove from map
		delete(c.typingChannels, channelID)
	}
}

// sendWithMedia sends a message with media attachments (images/files)
func (c *DiscordChannel) sendWithMedia(channelID, content string, mediaURLs []string) error {
	// Download and prepare files
	files := make([]*discordgo.File, 0, len(mediaURLs))
	tempFiles := make([]string, 0, len(mediaURLs))
	defer func() {
		// Clean up temporary files
		for _, tf := range tempFiles {
			os.Remove(tf)
		}
	}()

	for i, mediaURL := range mediaURLs {
		logger.DebugCF("discord", "Preparing media attachment", map[string]interface{}{
			"url":   mediaURL,
			"index": i + 1,
			"total": len(mediaURLs),
		})

		// Download file
		tempFile, filename, err := c.downloadMedia(mediaURL)
		if err != nil {
			logger.ErrorCF("discord", "Failed to download media", map[string]interface{}{
				"url":   mediaURL,
				"error": err.Error(),
			})
			continue // Skip this file but continue with others
		}

		tempFiles = append(tempFiles, tempFile)

		// Open file for reading
		file, err := os.Open(tempFile)
		if err != nil {
			logger.ErrorCF("discord", "Failed to open temp file", map[string]interface{}{
				"file":  tempFile,
				"error": err.Error(),
			})
			continue
		}
		defer file.Close()

		files = append(files, &discordgo.File{
			Name:   filename,
			Reader: file,
		})
	}

	if len(files) == 0 {
		// No files to send, fallback to text-only
		logger.WarnC("discord", "No media files could be prepared, sending text only")
		if content != "" {
			_, err := c.session.ChannelMessageSend(channelID, content)
			return err
		}
		return fmt.Errorf("no content or media to send")
	}

	// Send message with files
	_, err := c.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Files:   files,
	})

	if err != nil {
		return fmt.Errorf("failed to send message with media: %w", err)
	}

	logger.InfoCF("discord", "Sent message with media", map[string]interface{}{
		"channel_id":  channelID,
		"media_count": len(files),
	})

	return nil
}

// downloadMedia downloads a file from URL or copies from local path
// Returns: (tempFilePath, filename, error)
func (c *DiscordChannel) downloadMedia(urlOrPath string) (string, string, error) {
	// Check if it's a URL or local file path
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		return c.downloadFromURL(urlOrPath)
	}
	return c.copyLocalFile(urlOrPath)
}

// downloadFromURL downloads a file from a URL to a temporary file
func (c *DiscordChannel) downloadFromURL(url string) (string, string, error) {
	// Create HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// Extract filename from URL or Content-Disposition
	filename := filepath.Base(url)
	if filename == "." || filename == "/" {
		filename = "image.png"
	}

	// Create temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("pepebot_discord_%d_%s", time.Now().Unix(), filename))

	out, err := os.Create(tempFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Copy content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tempFile)
		return "", "", fmt.Errorf("failed to save file: %w", err)
	}

	return tempFile, filename, nil
}

// copyLocalFile copies a local file to a temporary location
func (c *DiscordChannel) copyLocalFile(filePath string) (string, string, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("file not found: %s", filePath)
	}

	// Open source file
	src, err := os.Open(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Extract filename
	filename := filepath.Base(filePath)

	// Create temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("pepebot_discord_%d_%s", time.Now().Unix(), filename))

	dst, err := os.Create(tempFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer dst.Close()

	// Copy content
	_, err = io.Copy(dst, src)
	if err != nil {
		os.Remove(tempFile)
		return "", "", fmt.Errorf("failed to copy file: %w", err)
	}

	return tempFile, filename, nil
}
