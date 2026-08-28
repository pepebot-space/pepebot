// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import "testing"

// A model id that repeats the provider name gets rejected upstream — MAIA Router
// answers "no healthy deployments for model=maiarouter/zai/glm-5.3-flash". Choosing the
// endpoint is the config's job, so the name is dropped from the model before sending.
func TestModelForProvider(t *testing.T) {
	cases := []struct{ provider, model, want string }{
		// the reported bug
		{"maiarouter", "maiarouter/zai/glm-5.3-flash", "zai/glm-5.3-flash"},
		{"maiarouter", "MaiaRouter/zai/glm-5.3", "zai/glm-5.3"}, // case-insensitive match

		// same shape on the other providers
		{"openrouter", "openrouter/anthropic/claude-3.5-sonnet", "anthropic/claude-3.5-sonnet"},
		{"openai", "openai/gpt-4o", "gpt-4o"},
		{"anthropic", "anthropic/claude-3-5-sonnet", "claude-3-5-sonnet"},
		{"groq", "groq/llama-3.3-70b", "llama-3.3-70b"},

		// vendor namespaces belong to the model and must survive
		{"maiarouter", "zai/glm-5.3-flash", "zai/glm-5.3-flash"},
		{"openrouter", "anthropic/claude-3.5-sonnet", "anthropic/claude-3.5-sonnet"},
		{"openrouter", "meta-llama/llama-3-70b", "meta-llama/llama-3-70b"},
		{"openai", "gpt-4o", "gpt-4o"},

		// "maia/" is MAIA's own model namespace, not the provider key: never strip it,
		// even when "maia" is the configured alias for the provider
		{"maiarouter", "maia/gemini-2.5-flash", "maia/gemini-2.5-flash"},
		{"maia", "maia/gemini-2.5-flash", "maia/gemini-2.5-flash"},

		// nothing configured, nothing to strip
		{"", "maiarouter/zai/glm-5.3", "maiarouter/zai/glm-5.3"},

		// only a whole leading segment counts
		{"openai", "openai-compatible/gpt-4o", "openai-compatible/gpt-4o"},
	}

	for _, c := range cases {
		if got := modelForProvider(c.provider, c.model); got != c.want {
			t.Errorf("modelForProvider(%q, %q) = %q, want %q", c.provider, c.model, got, c.want)
		}
	}
}
