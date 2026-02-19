//go:build !mips && !mipsle && !mips64 && !mips64le
// +build !mips,!mipsle,!mips64,!mips64le

package channels

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

type WhatsAppChannel struct {
	*BaseChannel
	client    *whatsmeow.Client
	config    config.WhatsAppConfig
	container *sqlstore.Container
	mu        sync.Mutex
}

func NewWhatsAppChannel(cfg config.WhatsAppConfig, messageBus *bus.MessageBus) (*WhatsAppChannel, error) {
	base := NewBaseChannel("whatsapp", cfg, messageBus, cfg.AllowFrom)

	dbPath := expandDBPath(cfg.DBPath)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbLog := waLog.Noop
	container, err := sqlstore.New(context.Background(), "sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath), dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlstore: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device store: %w", err)
	}

	clientLog := waLog.Noop
	client := whatsmeow.NewClient(deviceStore, clientLog)

	ch := &WhatsAppChannel{
		BaseChannel: base,
		client:      client,
		config:      cfg,
		container:   container,
	}

	client.AddEventHandler(ch.handleEvent)

	return ch, nil
}

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	logger.InfoC("whatsapp", "Starting WhatsApp channel...")

	if c.client.Store.ID == nil {
		qrChan, err := c.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}

		if err := c.client.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		go c.handleQRChannel(qrChan)
	} else {
		if err := c.client.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		logger.InfoC("whatsapp", "WhatsApp connected (session restored)")
	}

	c.setRunning(true)
	return nil
}

func (c *WhatsAppChannel) handleQRChannel(qrChan <-chan whatsmeow.QRChannelItem) {
	for evt := range qrChan {
		if evt.Event == "code" {
			logger.InfoC("whatsapp", "Scan this QR code to login:")
			fmt.Println("\n  WhatsApp QR Code - Scan to login")
			fmt.Println()
			qr, err := qrcode.New(evt.Code, qrcode.Medium)
			if err == nil {
				fmt.Println(qr.ToSmallString(false))
			} else {
				fmt.Printf("  QR data: %s\n", evt.Code)
			}
			fmt.Println("  Open WhatsApp > Linked Devices > Link a Device")
			fmt.Println()
		} else {
			logger.InfoCF("whatsapp", "QR channel event", map[string]interface{}{
				"event": evt.Event,
			})
			if evt.Event == "success" {
				logger.InfoC("whatsapp", "WhatsApp login successful!")
			} else if evt.Event == "timeout" {
				logger.WarnC("whatsapp", "WhatsApp QR code timeout, restart gateway to try again")
			}
		}
	}
}

func (c *WhatsAppChannel) Stop(ctx context.Context) error {
	logger.InfoC("whatsapp", "Stopping WhatsApp channel...")
	c.client.Disconnect()
	c.setRunning(false)
	return nil
}

func (c *WhatsAppChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.client.IsConnected() {
		return fmt.Errorf("whatsapp client not connected")
	}

	jid, err := types.ParseJID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("failed to parse JID %q: %w", msg.ChatID, err)
	}

	// If there are media attachments, send with media
	if len(msg.Media) > 0 {
		return c.sendWithMedia(ctx, jid, msg.Content, msg.Media)
	}

	// Send text-only message
	_, err = c.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(msg.Content),
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// sendWithMedia sends a message with media attachments (images, documents, audio, video)
func (c *WhatsAppChannel) sendWithMedia(ctx context.Context, jid types.JID, caption string, mediaURLs []string) error {
	for _, mediaURL := range mediaURLs {
		// Read file content
		var fileData []byte
		var fileName string
		var err error

		if strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
			// Download from URL
			logger.WarnCF("whatsapp", "HTTP URL media not yet supported for sending", map[string]interface{}{
				"url": mediaURL,
			})
			continue
		} else {
			// Read local file
			fileData, err = os.ReadFile(mediaURL)
			if err != nil {
				logger.ErrorCF("whatsapp", "Failed to read media file", map[string]interface{}{
					"path":  mediaURL,
					"error": err.Error(),
				})
				continue
			}
			fileName = filepath.Base(mediaURL)
		}

		// Detect MIME type and upload type from extension
		ext := strings.ToLower(filepath.Ext(mediaURL))
		var mimeType string
		var uploadType whatsmeow.MediaType

		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			uploadType = whatsmeow.MediaImage
		case ".mp4", ".avi", ".mov", ".mkv", ".webm":
			uploadType = whatsmeow.MediaVideo
		case ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".opus":
			uploadType = whatsmeow.MediaAudio
		default:
			uploadType = whatsmeow.MediaDocument
		}

		// Upload file to WhatsApp with correct media type
		uploaded, err := c.client.Upload(ctx, fileData, uploadType)
		if err != nil {
			logger.ErrorCF("whatsapp", "Failed to upload media", map[string]interface{}{
				"file":  fileName,
				"error": err.Error(),
			})
			continue
		}

		// Create message based on file type
		var waMsg *waE2E.Message

		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			mimeType = "image/jpeg"
			if ext == ".png" {
				mimeType = "image/png"
			} else if ext == ".gif" {
				mimeType = "image/gif"
			} else if ext == ".webp" {
				mimeType = "image/webp"
			}
			waMsg = &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
					Caption:       proto.String(caption),
				},
			}
		case ".mp4", ".avi", ".mov", ".mkv", ".webm":
			mimeType = "video/mp4"
			waMsg = &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
					Caption:       proto.String(caption),
				},
			}
		case ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".opus":
			mimeType = "audio/mpeg"
			if ext == ".ogg" || ext == ".opus" {
				mimeType = "audio/ogg; codecs=opus"
			}
			waMsg = &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				},
			}
		default:
			// Send as document for all other file types
			mimeType = "application/octet-stream"
			if ext == ".pdf" {
				mimeType = "application/pdf"
			}
			waMsg = &waE2E.Message{
				DocumentMessage: &waE2E.DocumentMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
					FileName:      proto.String(fileName),
					Caption:       proto.String(caption),
				},
			}
		}

		// Send the message
		_, err = c.client.SendMessage(ctx, jid, waMsg)
		if err != nil {
			logger.ErrorCF("whatsapp", "Failed to send media message", map[string]interface{}{
				"file":  fileName,
				"error": err.Error(),
			})
			return fmt.Errorf("failed to send media %s: %w", fileName, err)
		}

		logger.InfoCF("whatsapp", "Media sent successfully", map[string]interface{}{
			"file": fileName,
			"type": mimeType,
		})
	}

	return nil
}

func (c *WhatsAppChannel) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleIncomingMessage(v)
	case *events.Connected:
		logger.InfoC("whatsapp", "WhatsApp connected")
	case *events.Disconnected:
		logger.WarnC("whatsapp", "WhatsApp disconnected")
	case *events.LoggedOut:
		logger.WarnC("whatsapp", "WhatsApp logged out, delete db and restart to re-login")
	}
}

func (c *WhatsAppChannel) handleIncomingMessage(evt *events.Message) {
	if evt.Info.IsFromMe {
		return
	}

	senderID := evt.Info.Sender.String()
	chatID := evt.Info.Chat.String()

	content := extractTextContent(evt.Message)
	mediaPaths := []string{}

	// Handle image messages
	if imgMsg := evt.Message.GetImageMessage(); imgMsg != nil {
		logger.DebugCF("whatsapp", "Downloading image", map[string]interface{}{
			"sender": senderID,
		})

		mediaPath := c.downloadWhatsAppMedia(evt)
		if mediaPath != "" {
			mediaPaths = append(mediaPaths, mediaPath)
			if imgMsg.GetCaption() != "" {
				content = imgMsg.GetCaption()
			}
			if content == "" {
				content = "[image received]"
			}
		}
	}

	// Handle video messages
	if vidMsg := evt.Message.GetVideoMessage(); vidMsg != nil {
		logger.DebugCF("whatsapp", "Downloading video", map[string]interface{}{
			"sender": senderID,
		})

		mediaPath := c.downloadWhatsAppMedia(evt)
		if mediaPath != "" {
			mediaPaths = append(mediaPaths, mediaPath)
			if vidMsg.GetCaption() != "" {
				content = vidMsg.GetCaption()
			}
			if content == "" {
				content = "[video received]"
			}
		}
	}

	// Handle audio messages
	if audMsg := evt.Message.GetAudioMessage(); audMsg != nil {
		logger.DebugCF("whatsapp", "Downloading audio", map[string]interface{}{
			"sender": senderID,
		})

		mediaPath := c.downloadWhatsAppMedia(evt)
		if mediaPath != "" {
			mediaPaths = append(mediaPaths, mediaPath)
			if content == "" {
				content = "[audio received]"
			}
		}
	}

	// Handle document messages
	if docMsg := evt.Message.GetDocumentMessage(); docMsg != nil {
		logger.DebugCF("whatsapp", "Downloading document", map[string]interface{}{
			"sender":   senderID,
			"filename": docMsg.GetFileName(),
		})

		mediaPath := c.downloadWhatsAppMedia(evt)
		if mediaPath != "" {
			mediaPaths = append(mediaPaths, mediaPath)
			if docMsg.GetCaption() != "" {
				content = docMsg.GetCaption()
			}
			if content == "" {
				content = fmt.Sprintf("[document received: %s]", docMsg.GetFileName())
			}
		}
	}

	// If no content and no media, ignore message
	if content == "" && len(mediaPaths) == 0 {
		return
	}

	metadata := map[string]string{
		"message_id": string(evt.Info.ID),
		"push_name":  evt.Info.PushName,
	}

	logger.DebugCF("whatsapp", "Message received", map[string]interface{}{
		"sender":      senderID,
		"chat":        chatID,
		"text":        truncateString(content, 50),
		"has_media":   len(mediaPaths) > 0,
		"media_count": len(mediaPaths),
	})

	c.HandleMessage(senderID, chatID, content, mediaPaths, metadata)
}

// downloadWhatsAppMedia downloads media from WhatsApp message
func (c *WhatsAppChannel) downloadWhatsAppMedia(evt *events.Message) string {
	// Create temp directory for WhatsApp media
	tempDir := "/tmp/pepebot_whatsapp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		logger.ErrorCF("whatsapp", "Failed to create temp dir", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	// Determine filename and get downloadable message
	var filename string
	var ext string
	var downloadable whatsmeow.DownloadableMessage

	if imgMsg := evt.Message.GetImageMessage(); imgMsg != nil {
		downloadable = imgMsg
		ext = ".jpg"
		if imgMsg.GetMimetype() == "image/png" {
			ext = ".png"
		} else if imgMsg.GetMimetype() == "image/gif" {
			ext = ".gif"
		} else if imgMsg.GetMimetype() == "image/webp" {
			ext = ".webp"
		}
		filename = fmt.Sprintf("image_%s%s", evt.Info.ID, ext)
	} else if vidMsg := evt.Message.GetVideoMessage(); vidMsg != nil {
		downloadable = vidMsg
		ext = ".mp4"
		filename = fmt.Sprintf("video_%s%s", evt.Info.ID, ext)
	} else if audMsg := evt.Message.GetAudioMessage(); audMsg != nil {
		downloadable = audMsg
		ext = ".ogg"
		if strings.Contains(audMsg.GetMimetype(), "mpeg") {
			ext = ".mp3"
		}
		filename = fmt.Sprintf("audio_%s%s", evt.Info.ID, ext)
	} else if docMsg := evt.Message.GetDocumentMessage(); docMsg != nil {
		downloadable = docMsg
		docFilename := docMsg.GetFileName()
		if docFilename != "" {
			filename = docFilename
		} else {
			ext = ".pdf"
			filename = fmt.Sprintf("document_%s%s", evt.Info.ID, ext)
		}
	} else {
		logger.ErrorC("whatsapp", "Unknown media type")
		return ""
	}

	// Download the media with context
	ctx := context.Background()
	data, err := c.client.Download(ctx, downloadable)
	if err != nil {
		logger.ErrorCF("whatsapp", "Failed to download media", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	// Save to file
	filePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logger.ErrorCF("whatsapp", "Failed to save media file", map[string]interface{}{
			"error": err.Error(),
			"path":  filePath,
		})
		return ""
	}

	logger.InfoCF("whatsapp", "Media downloaded successfully", map[string]interface{}{
		"filename": filename,
		"size":     len(data),
		"path":     filePath,
	})

	return filePath
}

func extractTextContent(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	if conv := msg.GetConversation(); conv != "" {
		return conv
	}

	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if text := ext.GetText(); text != "" {
			return text
		}
	}

	return ""
}

func expandDBPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
