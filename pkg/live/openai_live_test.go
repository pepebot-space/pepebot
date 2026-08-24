// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"encoding/json"
	"strings"
	"testing"
)

// Self-hosted OpenAI-compatible realtime servers usually run without auth, so an
// empty api_key must only be rejected for api.openai.com itself.
func TestOpenAILiveProvider_KeylessSelfHosted(t *testing.T) {
	if _, err := NewOpenAILiveProvider("openai", "", ""); err == nil {
		t.Error("expected an error for api.openai.com without an api_key")
	}

	p, err := NewOpenAILiveProvider("vllm", "http://100.104.36.93:8000/v1", "")
	if err != nil {
		t.Fatalf("keyless self-hosted endpoint rejected: %v", err)
	}

	headers, err := p.AuthHeaders()
	if err != nil {
		t.Fatalf("AuthHeaders: %v", err)
	}
	if got := headers.Get("Authorization"); got != "" {
		t.Errorf("expected no Authorization header without a key, got %q", got)
	}

	want := "ws://100.104.36.93:8000/v1/realtime?model=google/gemma-4-31B-it"
	if got := p.BuildUpstreamURL("google/gemma-4-31B-it"); got != want {
		t.Errorf("BuildUpstreamURL = %q, want %q", got, want)
	}
}

func TestOpenAILiveProvider_URLScheme(t *testing.T) {
	cases := map[string]string{
		"https://api.openai.com/v1": "wss://api.openai.com/v1/realtime?model=m",
		"http://localhost:8000/v1":  "ws://localhost:8000/v1/realtime?model=m",
		"http://localhost:8000/v1/": "ws://localhost:8000/v1/realtime?model=m",
	}
	for base, want := range cases {
		p, err := NewOpenAILiveProvider("test", base, "k")
		if err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		if got := p.BuildUpstreamURL("m"); got != want {
			t.Errorf("%s -> %q, want %q", base, got, want)
		}
	}
}

// Pepebot stores tool schemas in the chat-completions shape ({type, function:{...}}),
// but the Realtime protocol wants them flat, with the name at the top level.
func TestBuildRealtimeSessionUpdate(t *testing.T) {
	defs := []map[string]interface{}{
		{"type": "function", "function": map[string]interface{}{
			"name":        "exec",
			"description": "Execute a shell command",
			"parameters":  map[string]interface{}{"type": "object"},
		}},
		{"type": "function", "name": "already_flat"},               // passed through
		{"type": "function", "function": map[string]interface{}{}}, // no name: dropped
	}

	var update map[string]interface{}
	if err := json.Unmarshal(buildRealtimeSessionUpdate("be brief", defs), &update); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if update["type"] != "session.update" {
		t.Errorf("type = %v, want session.update", update["type"])
	}

	session := update["session"].(map[string]interface{})
	if session["instructions"] != "be brief" {
		t.Errorf("instructions = %v", session["instructions"])
	}
	if session["tool_choice"] != "auto" {
		t.Errorf("tool_choice = %v, want auto", session["tool_choice"])
	}

	toolList := session["tools"].([]interface{})
	if len(toolList) != 2 {
		t.Fatalf("expected 2 tools (the nameless one dropped), got %d", len(toolList))
	}
	first := toolList[0].(map[string]interface{})
	if _, wrapped := first["function"]; wrapped {
		t.Error("tool still wrapped in a chat-completions \"function\" object")
	}
	if first["name"] != "exec" || first["description"] != "Execute a shell command" {
		t.Errorf("flattened tool = %#v", first)
	}
	if first["parameters"] == nil {
		t.Error("parameters dropped")
	}
}

func TestBuildRealtimeSessionUpdate_Empty(t *testing.T) {
	if got := buildRealtimeSessionUpdate("", nil); got != nil {
		t.Errorf("expected nil when there is nothing to send, got %s", got)
	}
	// Tools alone, or a prompt alone, are both worth sending.
	if got := buildRealtimeSessionUpdate("  persona  ", nil); got == nil {
		t.Error("expected an update for a prompt with no tools")
	} else if !strings.Contains(string(got), `"instructions":"persona"`) {
		t.Errorf("prompt not trimmed/included: %s", got)
	}
}
