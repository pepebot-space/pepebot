//go:build mips || mipsle || mips64 || mips64le
// +build mips mipsle mips64 mips64le

package channels

import (
	"context"
	"fmt"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
)

// WhatsAppChannel stub for MIPS architectures (SQLite not supported)
type WhatsAppChannel struct {
	*BaseChannel
}

func NewWhatsAppChannel(cfg config.WhatsAppConfig, messageBus *bus.MessageBus) (*WhatsAppChannel, error) {
	return nil, fmt.Errorf("WhatsApp channel is not supported on MIPS architecture (SQLite dependency unavailable)")
}

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	return fmt.Errorf("WhatsApp channel is not supported on MIPS architecture")
}

func (c *WhatsAppChannel) Stop(ctx context.Context) error {
	return nil
}

func (c *WhatsAppChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	return fmt.Errorf("WhatsApp channel is not supported on MIPS architecture")
}
