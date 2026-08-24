// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import "testing"

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
