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

	_, err = c.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(msg.Content),
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
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
	if content == "" {
		return
	}

	metadata := map[string]string{
		"message_id": string(evt.Info.ID),
		"push_name":  evt.Info.PushName,
	}

	logger.DebugCF("whatsapp", "Message received", map[string]interface{}{
		"sender": senderID,
		"chat":   chatID,
		"text":   truncateString(content, 50),
	})

	c.HandleMessage(senderID, chatID, content, nil, metadata)
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
