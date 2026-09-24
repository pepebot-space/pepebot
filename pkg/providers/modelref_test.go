// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import "testing"

func TestParseModelRef(t *testing.T) {
	cases := []struct {
		name         string
		provider     string
		model        string
		wantProvider string
		wantEndpoint string
		wantModel    string
	}{
		{"canonical form", "", "zai:glm-4.5v", "zai", "", "glm-4.5v"},
		{"vendor namespace survives the colon form", "", "maiarouter:zai/glm-4.5v", "maiarouter", "", "zai/glm-4.5v"},
		{"custom endpoint and model", "", "custom:ollama/qwen3", "custom", "ollama", "qwen3"},
		{"custom endpoint, its own default model", "", "custom:ollama", "custom", "ollama", ""},
		{"provider set, model id alone", "zai", "glm-4.5v", "zai", "", "glm-4.5v"},
		{"provider set, vendor namespace kept", "maiarouter", "zai/glm-4.5v", "maiarouter", "", "zai/glm-4.5v"},
		{"colon wins over the provider field", "openai", "zai:glm-4.6", "zai", "", "glm-4.6"},
		{"case is normalised", "", "ZAI:glm-4.5v", "zai", "", "glm-4.5v"},
		{"no provider anywhere", "", "gpt-4.1", "", "", "gpt-4.1"},

		// The v0.5.20 bug, now unrepresentable in the canonical form and still
		// repaired in the legacy one.
		{"legacy duplicated provider is stripped", "maiarouter", "maiarouter/zai/glm-5.3", "maiarouter", "", "zai/glm-5.3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseModelRef(tc.provider, tc.model)
			if got.Provider != tc.wantProvider || got.Endpoint != tc.wantEndpoint || got.Model != tc.wantModel {
				t.Errorf("ParseModelRef(%q, %q) = {%q %q %q}, want {%q %q %q}",
					tc.provider, tc.model,
					got.Provider, got.Endpoint, got.Model,
					tc.wantProvider, tc.wantEndpoint, tc.wantModel)
			}
		})
	}
}

// What pepebot prints back has to be something a user can paste into a config.
func TestModelRefRoundTrips(t *testing.T) {
	for _, spelling := range []string{"zai:glm-4.5v", "maiarouter:zai/glm-4.5v", "custom:ollama/qwen3", "custom:ollama"} {
		if got := ParseModelRef("", spelling).String(); got != spelling {
			t.Errorf("round trip of %q produced %q", spelling, got)
		}
	}
	if got := ParseModelRef("zai", "glm-4.5v").String(); got != "zai:glm-4.5v" {
		t.Errorf("legacy pair rendered as %q, want the canonical spelling", got)
	}
}
