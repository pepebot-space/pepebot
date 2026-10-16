// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/config"
)

func loopWithBus(notify string) (*AgentLoop, *bus.MessageBus) {
	b := bus.NewMessageBus()
	return &AgentLoop{bus: b, memoryCfg: config.MemoryConfig{Notify: notify}}, b
}

func nextOutbound(t *testing.T, b *bus.MessageBus) (bus.OutboundMessage, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	return b.SubscribeOutbound(ctx)
}

// Learning used to be visible only as a log line on a server. A bot that
// quietly edits what it believes about you should say so in the conversation
// it learned from.
func TestNotifyLearnedTellsTheChannel(t *testing.T) {
	al, b := loopWithBus("on")
	al.notifyLearned("discord:1501096055536750744", []string{"ingatan: suka zsh", "skill baru: pepebot-release"})

	msg, ok := nextOutbound(t, b)
	if !ok {
		t.Fatal("nothing was sent to the channel")
	}
	if msg.Channel != "discord" || msg.ChatID != "1501096055536750744" {
		t.Errorf("routed to %s/%s", msg.Channel, msg.ChatID)
	}
	if !strings.Contains(msg.Content, "2") {
		t.Errorf("content = %q, want it to say how much was kept", msg.Content)
	}
	// "on" is a one-liner; the detail belongs to verbose.
	if strings.Contains(msg.Content, "zsh") {
		t.Errorf("content = %q, want the short form", msg.Content)
	}
}

func TestNotifyLearnedVerboseSaysWhat(t *testing.T) {
	al, b := loopWithBus("verbose")
	al.notifyLearned("telegram:42", []string{"skill baru: pepebot-release"})

	msg, ok := nextOutbound(t, b)
	if !ok {
		t.Fatal("nothing was sent")
	}
	if !strings.Contains(msg.Content, "pepebot-release") {
		t.Errorf("verbose content = %q, want it to name what was kept", msg.Content)
	}
}

func TestNotifyLearnedStaysQuietWhenItShould(t *testing.T) {
	cases := []struct {
		name       string
		notify     string
		sessionKey string
		learned    []string
	}{
		{"off", "off", "discord:1", []string{"ingatan: x"}},
		{"nothing learned", "on", "discord:1", nil},
		{"cli has no channel to answer on", "on", "cli:default", []string{"ingatan: x"}},
		{"session key with no channel", "on", "aneh", []string{"ingatan: x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			al, b := loopWithBus(tc.notify)
			al.notifyLearned(tc.sessionKey, tc.learned)
			if msg, ok := nextOutbound(t, b); ok {
				t.Errorf("sent %q when it should have stayed quiet", msg.Content)
			}
		})
	}
}

// An unset value must behave like "on": a deployment that never heard of this
// setting still gets told.
func TestNotifyLearnedDefaultsToOn(t *testing.T) {
	al, b := loopWithBus("")
	al.notifyLearned("discord:7", []string{"ingatan: x"})
	if _, ok := nextOutbound(t, b); !ok {
		t.Error("an unset notify setting stayed silent; it should default to on")
	}
}
