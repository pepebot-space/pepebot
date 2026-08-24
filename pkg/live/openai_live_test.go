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
	if err := json.Unmarshal(buildRealtimeSessionUpdate("be brief", defs, nil), &update); err != nil {
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
	if got := buildRealtimeSessionUpdate("", nil, nil); got != nil {
		t.Errorf("expected nil when there is nothing to send, got %s", got)
	}
	// Tools alone, or a prompt alone, are both worth sending.
	if got := buildRealtimeSessionUpdate("  persona  ", nil, nil); got == nil {
		t.Error("expected an update for a prompt with no tools")
	} else if !strings.Contains(string(got), `"instructions":"persona"`) {
		t.Errorf("prompt not trimmed/included: %s", got)
	}
}

// live.realtime_session carries whatever the upstream server allows a client to set,
// but must not be able to override the agent's own instructions or tools.
func TestBuildRealtimeSessionUpdate_Passthrough(t *testing.T) {
	extra := map[string]interface{}{
		"voice":                      "JV-00027",
		"max_response_output_tokens": 256,
		"instructions":               "should lose to the agent persona",
	}
	defs := []map[string]interface{}{
		{"type": "function", "function": map[string]interface{}{"name": "exec"}},
	}

	var update map[string]interface{}
	if err := json.Unmarshal(buildRealtimeSessionUpdate("agent persona", defs, extra), &update); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	session := update["session"].(map[string]interface{})

	if session["voice"] != "JV-00027" {
		t.Errorf("voice = %v, want JV-00027", session["voice"])
	}
	if session["max_response_output_tokens"] != float64(256) {
		t.Errorf("max_response_output_tokens = %v", session["max_response_output_tokens"])
	}
	if session["instructions"] != "agent persona" {
		t.Errorf("passthrough overrode the agent persona: %v", session["instructions"])
	}
	if len(session["tools"].([]interface{})) != 1 {
		t.Errorf("tools = %v", session["tools"])
	}

	// Passthrough alone is worth sending even with no persona and no tools.
	if got := buildRealtimeSessionUpdate("", nil, map[string]interface{}{"voice": "JV-00027"}); got == nil {
		t.Error("expected an update for passthrough-only config")
	}
}

// Live output is spoken, so the instructions must always carry the speech rules, and
// the language rule must be additive to the persona rather than replacing it.
func TestLiveInstructions(t *testing.T) {
	// The speech directive is unconditional — a bare session still gets it.
	bare := liveInstructions("", "", "")
	if !strings.Contains(bare, "converted to speech") {
		t.Errorf("speech directive missing from a bare session: %q", bare)
	}
	for _, banned := range []string{"markdown", "tables", "emoji"} {
		if !strings.Contains(bare, banned) {
			t.Errorf("speech directive does not mention %q", banned)
		}
	}

	full := liveInstructions("  You are a butler.  ", "## Available Skills\n\n<skills/>", "id-ID")
	if !strings.HasPrefix(full, "You are a butler.") {
		t.Errorf("persona should come first and be trimmed: %q", full)
	}
	if !strings.HasSuffix(full, "Always reply in Indonesian.") {
		t.Errorf("language directive should come last: %q", full)
	}
	if !strings.Contains(full, "converted to speech") {
		t.Error("speech directive dropped when a persona is set")
	}
	if !strings.Contains(full, "Available Skills") {
		t.Error("skills dropped from the instructions")
	}
	if strings.Index(full, "Available Skills") < strings.Index(full, "You are a butler.") {
		t.Error("skills should follow the persona, not precede it")
	}
}

func TestReplyLanguageDirective(t *testing.T) {
	cases := map[string]string{
		"":      "",
		"id-ID": "Always reply in Indonesian.",
		"id":    "Always reply in Indonesian.",
		"en-US": "Always reply in English.",
		"jv-ID": "Always reply in Javanese.",
		"xx-YY": "Always reply in xx-YY.", // unknown code passes through
	}
	for code, want := range cases {
		if got := replyLanguageDirective(code); got != want {
			t.Errorf("replyLanguageDirective(%q) = %q, want %q", code, got, want)
		}
	}
}
