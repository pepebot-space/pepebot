// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A thinking model writes reasoning_content first and only then the answer. When
// the output budget runs out mid-thought, content comes back empty with
// finish_reason "length" — measured against a live GLM endpoint, 775 characters
// of reasoning and zero of answer. The agent loop could only report that as
// "I've completed processing but have no response to give."
func TestReasoningContentUsedWhenAnswerIsEmpty(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"content":"","reasoning_content":"Pengguna menanyakan ibu kota Perancis. Itu Paris."},"finish_reason":"length"}]}`)

	resp, err := (&HTTPProvider{}).parseResponse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(resp.Content, "Paris") {
		t.Errorf("reasoning was not used as the reply, got %q", resp.Content)
	}
}

func TestAnswerWinsOverReasoning(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"content":"Paris.","reasoning_content":"panjang sekali pertimbangannya"},"finish_reason":"stop"}]}`)

	resp, err := (&HTTPProvider{}).parseResponse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Content != "Paris." {
		t.Errorf("content = %q, want the answer, not the reasoning", resp.Content)
	}
}

// Of the four ways to ask GLM not to think, only extra_body survives litellm.
func TestExtraBodyReachesTheRequest(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &got)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	p := NewHTTPProviderFor("maiarouter", "k", srv.URL)
	_, err := p.Chat(context.Background(), []Message{{Role: "user", Content: "hai"}}, nil, "zai/glm-4.5v",
		map[string]interface{}{
			"max_tokens": 8192,
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "disabled"}},
		})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	extra, ok := got["extra_body"].(map[string]interface{})
	if !ok {
		t.Fatalf("extra_body missing from request body: %v", got)
	}
	thinking, ok := extra["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "disabled" {
		t.Errorf("extra_body did not arrive intact: %v", extra)
	}
}

func TestNoExtraBodyKeyWhenUnset(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &got)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	p := NewHTTPProviderFor("maiarouter", "k", srv.URL)
	if _, err := p.Chat(context.Background(), []Message{{Role: "user", Content: "hai"}}, nil, "m", nil); err != nil {
		t.Fatalf("chat: %v", err)
	}
	if _, present := got["extra_body"]; present {
		t.Error("an empty extra_body was sent; providers should not see the key at all")
	}
}

// Streaming has the same failure and needs the same answer — but reasoning is
// buffered, not streamed: GLM emits several times more of it than answer, and
// nobody asked to watch the model deliberate.
func TestStreamFallsBackToReasoningOnlyWhenNoAnswerArrives(t *testing.T) {
	sse := func(chunks ...string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, c := range chunks {
				io.WriteString(w, "data: "+c+"\n\n")
			}
			io.WriteString(w, "data: [DONE]\n\n")
		}
	}

	t.Run("reasoning only", func(t *testing.T) {
		srv := httptest.NewServer(sse(
			`{"choices":[{"delta":{"reasoning_content":"Pengguna bertanya... "}}]}`,
			`{"choices":[{"delta":{"reasoning_content":"jawabannya Paris."}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"length"}]}`,
		))
		defer srv.Close()

		var sb strings.Builder
		err := NewHTTPProviderFor("maiarouter", "k", srv.URL).ChatStream(
			context.Background(), []Message{{Role: "user", Content: "hai"}}, "m", nil,
			func(c StreamChunk) { sb.WriteString(c.Content) })
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		if !strings.Contains(sb.String(), "Paris") {
			t.Errorf("nothing reached the user, got %q", sb.String())
		}
	})

	t.Run("answer present", func(t *testing.T) {
		srv := httptest.NewServer(sse(
			`{"choices":[{"delta":{"reasoning_content":"pertimbangan panjang"}}]}`,
			`{"choices":[{"delta":{"content":"Paris."}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		))
		defer srv.Close()

		var sb strings.Builder
		err := NewHTTPProviderFor("maiarouter", "k", srv.URL).ChatStream(
			context.Background(), []Message{{Role: "user", Content: "hai"}}, "m", nil,
			func(c StreamChunk) { sb.WriteString(c.Content) })
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		if got := sb.String(); got != "Paris." {
			t.Errorf("stream = %q, want only the answer — reasoning must stay buffered", got)
		}
	})
}
