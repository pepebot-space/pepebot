// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"testing"

	"github.com/pepebot-space/pepebot/pkg/config"
)

// Tool definitions are prefilled again on every turn, and on a voice session
// that is the difference between 591 ms and 2131 ms to first token (measured
// against jalak/qwen3.8-27b over a 0.3 ms LAN hop). A deployment has to be able
// to turn them off without every client knowing to ask.
func TestResolveEnableTools(t *testing.T) {
	no, yes := false, true
	withCfg := func(v *bool) *config.Config {
		c := &config.Config{}
		c.Live.EnableTools = v
		return c
	}

	cases := []struct {
		name   string
		cfg    *config.Config
		setup  *SetupConfig
		expect bool
	}{
		{"nothing set keeps tools", withCfg(nil), &SetupConfig{}, true},
		{"no config at all keeps tools", nil, &SetupConfig{}, true},
		{"config turns them off", withCfg(&no), &SetupConfig{}, false},
		{"client asks for them despite config", withCfg(&no), &SetupConfig{EnableTools: &yes}, true},
		{"client declines despite config", withCfg(&yes), &SetupConfig{EnableTools: &no}, false},
		{"no setup message at all", withCfg(&no), nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveEnableTools(tc.cfg, tc.setup); got != tc.expect {
				t.Errorf("resolveEnableTools = %v, want %v", got, tc.expect)
			}
		})
	}
}
