// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package config

import (
	"strings"
	"testing"
)

func validConfig() *Config {
	c := DefaultConfig()
	c.Agents.Defaults.Model = "anthropic:claude-sonnet-5-5"
	c.Agents.Defaults.Provider = ""
	return c
}

func problemsFor(t *testing.T, mutate func(*Config)) []string {
	t.Helper()
	c := validConfig()
	mutate(c)
	return c.Validate()
}

func mustMention(t *testing.T, problems []string, needle string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(p, needle) {
			return
		}
	}
	t.Errorf("no problem mentioned %q; got %v", needle, problems)
}

func TestValidConfigHasNoProblems(t *testing.T) {
	if got := validConfig().Validate(); len(got) != 0 {
		t.Errorf("a good config reported problems: %v", got)
	}
}

// The v0.5.20 bug: "maiarouter/zai/glm-5.3" went out whole and the endpoint
// answered "no healthy deployments for this model", naming the model and not
// the config line that produced it.
func TestProviderNameRepeatedInModel(t *testing.T) {
	problems := problemsFor(t, func(c *Config) {
		c.Agents.Defaults.Provider = "maiarouter"
		c.Agents.Defaults.Model = "maiarouter/zai/glm-5.3"
	})
	mustMention(t, problems, "repeats the provider name")
	mustMention(t, problems, "maiarouter:zai/glm-5.3")
}

func TestCustomProviderMustExist(t *testing.T) {
	mustMention(t, problemsFor(t, func(c *Config) {
		c.Agents.Defaults.Model = "custom:r9/myr"
	}), "not defined under providers.custom")

	mustMention(t, problemsFor(t, func(c *Config) {
		c.Agents.Defaults.Model = "custom:r9/myr"
		c.Providers.Custom = map[string]CustomProviderConfig{"r9": {APIKey: "k"}}
	}), "has no api_base")

	if got := problemsFor(t, func(c *Config) {
		c.Agents.Defaults.Model = "custom:r9/myr"
		c.Providers.Custom = map[string]CustomProviderConfig{"r9": {APIKey: "k", APIBase: "http://localhost:20128/v1"}}
	}); len(got) != 0 {
		t.Errorf("a defined custom provider reported problems: %v", got)
	}
}

// One number served as both the reply budget and the input window, so sessions
// on a 128k model were compacted at 75% of 8k — silently, every time.
func TestContextWindowSmallerThanMaxTokens(t *testing.T) {
	mustMention(t, problemsFor(t, func(c *Config) {
		c.Agents.Defaults.MaxTokens = 8192
		c.Agents.Defaults.ContextWindow = 4096
	}), "smaller than max_tokens")
}

func TestMemoryAndLiveChecks(t *testing.T) {
	mustMention(t, problemsFor(t, func(c *Config) {
		c.Memory.Review = true
		c.Memory.ReviewInterval = 0
	}), "review never runs")

	mustMention(t, problemsFor(t, func(c *Config) {
		c.Live.Enabled = true
		c.Live.Provider = ""
	}), "live.provider is empty")

	mustMention(t, problemsFor(t, func(c *Config) {
		c.Agents.Defaults.Model = ""
	}), "nothing will answer")
}

func TestCustomTarget(t *testing.T) {
	cases := []struct {
		in, endpoint, rest string
		ok                 bool
	}{
		{"custom:ollama/qwen3", "ollama", "qwen3", true},
		{"custom:ollama", "ollama", "", true},
		{"anthropic:claude-sonnet-5-5", "", "", false},
		{"zai/glm-4.5v", "", "", false},
	}
	for _, tc := range cases {
		e, r, ok := customTarget(tc.in)
		if e != tc.endpoint || r != tc.rest || ok != tc.ok {
			t.Errorf("customTarget(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.in, e, r, ok, tc.endpoint, tc.rest, tc.ok)
		}
	}
}
