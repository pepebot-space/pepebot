// Pepebot - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import (
	"testing"
)

func TestAnthropicProvider_GetDefaultModel(t *testing.T) {
	if got := NewAnthropicProvider("test-key", "").GetDefaultModel(); got != "claude-sonnet-5-5" {
		t.Errorf("Anthropic default model = %s", got)
	}
	if got := NewOpenCodeProvider("test-key", "").GetDefaultModel(); got != "minimax-m3" {
		t.Errorf("OpenCode default model = %s", got)
	}
}

func TestAnthropicProvider_DefaultAPIBase(t *testing.T) {
	if got := NewAnthropicProvider("test-key", "").apiBase; got != "https://api.anthropic.com" {
		t.Errorf("Anthropic default apiBase = %s", got)
	}
	// The same provider pointed at opencode's gateway keeps its own default.
	if got := NewOpenCodeProvider("test-key", "").apiBase; got != "https://opencode.ai/zen/go" {
		t.Errorf("OpenCode default apiBase = %s", got)
	}
	// A base written with /v1 already on it still resolves: this provider
	// appends /v1/messages itself.
	if got := NewAnthropicProvider("k", "https://api.anthropic.com/v1").apiBase; got != "https://api.anthropic.com" {
		t.Errorf("apiBase with /v1 = %s, want it trimmed", got)
	}
}

func TestAnthropicProvider_CustomAPIBase(t *testing.T) {
	customBase := "https://custom.opencode.ai/api"
	provider := NewAnthropicProvider("test-key", customBase)
	if provider.apiBase != customBase {
		t.Errorf("Expected apiBase %s, got %s", customBase, provider.apiBase)
	}
}

func TestAnthropicProvider_BuildAnthropicRequest(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "")

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	request := provider.buildAnthropicRequest(messages, nil, "minimax-m3", nil)

	model, ok := request["model"].(string)
	if !ok || model != "minimax-m3" {
		t.Errorf("Expected model 'minimax-m3', got %v", request["model"])
	}

	maxTokens, ok := request["max_tokens"].(int)
	if !ok || maxTokens != 4096 {
		t.Errorf("Expected max_tokens 4096, got %v", request["max_tokens"])
	}
}

func TestAnthropicProvider_BuildAnthropicRequestWithSystem(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "")

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
	}

	request := provider.buildAnthropicRequest(messages, nil, "minimax-m3", nil)

	system, ok := request["system"].(string)
	if !ok || system != "You are a helpful assistant." {
		t.Errorf("Expected system prompt, got %v", request["system"])
	}
}

func TestAnthropicProvider_BuildAnthropicRequestWithTools(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "")

	messages := []Message{
		{Role: "user", Content: "What is the weather?"},
	}

	tools := []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunctionDefinition{
				Name:        "get_weather",
				Description: "Get the current weather",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	request := provider.buildAnthropicRequest(messages, tools, "minimax-m3", nil)

	toolsArray, ok := request["tools"].([]map[string]interface{})
	if !ok || len(toolsArray) != 1 {
		t.Errorf("Expected 1 tool, got %v", request["tools"])
	}

	if toolsArray[0]["name"] != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %v", toolsArray[0]["name"])
	}
}

// Regression: the agent loop stores tool calls OpenAI-style (Function.Arguments
// as a JSON string, Arguments nil). Sending that through as `input` yields null
// and the API rejects the whole request with a 400.
func TestAnthropicProvider_ToolUseInputIsAlwaysObject(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "")

	cases := map[string]ToolCall{
		"openai shape": {ID: "t1", Type: "function", Function: &FunctionCall{Name: "exec", Arguments: `{"command":"echo hi"}`}},
		"no arguments": {ID: "t2", Name: "list_dir"},
		"bad json":     {ID: "t3", Name: "exec", Function: &FunctionCall{Name: "exec", Arguments: "not json"}},
	}

	for name, tc := range cases {
		request := provider.buildAnthropicRequest([]Message{
			{Role: "user", Content: "go"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{tc}},
		}, nil, "minimax-m3", nil)

		block := toolUseBlock(t, request)

		if block["name"] == "" {
			t.Errorf("%s: tool_use name is empty", name)
		}
		if _, ok := block["input"].(map[string]interface{}); !ok {
			t.Errorf("%s: input must be an object, got %#v", name, block["input"])
		}
	}

	// The decoded arguments must survive the round trip.
	request := provider.buildAnthropicRequest([]Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", ToolCalls: []ToolCall{cases["openai shape"]}},
	}, nil, "minimax-m3", nil)
	input := toolUseBlock(t, request)["input"].(map[string]interface{})
	if input["command"] != "echo hi" {
		t.Errorf("expected command 'echo hi', got %v", input["command"])
	}
}

func toolUseBlock(t *testing.T, request map[string]interface{}) map[string]interface{} {
	t.Helper()
	msgs := request["messages"].([]map[string]interface{})
	for _, block := range msgs[1]["content"].([]map[string]interface{}) {
		if block["type"] == "tool_use" {
			return block
		}
	}
	t.Fatal("no tool_use block in assistant message")
	return nil
}

// Regression: the agent builds multimodal content as a typed []ContentBlock, but
// providers used to only recognize the JSON-decoded []interface{} shape, so images
// fell through to fmt.Sprintf and reached the model as a Go struct dump.
func TestAnthropicProvider_MultimodalContent(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "")

	typed := []ContentBlock{
		{Type: "text", Text: "what color?"},
		{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,QUJD", Detail: "auto"}},
	}
	generic := []interface{}{
		map[string]interface{}{"type": "text", "text": "what color?"},
		map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,QUJD"}},
	}

	for name, content := range map[string]interface{}{"typed blocks": typed, "decoded blocks": generic} {
		blocks, ok := provider.buildContent(Message{Role: "user", Content: content}).([]map[string]interface{})
		if !ok || len(blocks) != 2 {
			t.Fatalf("%s: expected 2 content blocks, got %#v", name, blocks)
		}
		if blocks[0]["type"] != "text" || blocks[0]["text"] != "what color?" {
			t.Errorf("%s: bad text block: %#v", name, blocks[0])
		}
		if blocks[1]["type"] != "image" {
			t.Fatalf("%s: expected an image block, got %#v", name, blocks[1])
		}
		source := blocks[1]["source"].(map[string]interface{})
		if source["media_type"] != "image/png" || source["data"] != "QUJD" {
			t.Errorf("%s: bad image source: %#v", name, source)
		}
	}
}

// Regression: media reaches the agent as a data URL, which has no file extension —
// extension-based detection classified images as generic files and the providers
// dropped them.
func TestDetectFileType_DataURL(t *testing.T) {
	cases := map[string]FileType{
		"data:image/png;base64,QUJD":       FileTypeImage,
		"data:image/jpeg;base64,QUJD":      FileTypeImage,
		"data:application/pdf;base64,QUJD": FileTypeDocument,
		"data:audio/mpeg;base64,QUJD":      FileTypeAudio,
		"/tmp/photo.png":                   FileTypeImage,
	}
	for url, want := range cases {
		if got, _ := DetectFileType(url); got != want {
			t.Errorf("DetectFileType(%q) = %q, want %q", truncateString(url, 40), got, want)
		}
	}
}

// A PDF arrives as an OpenAI-shaped file block and has to leave as an Anthropic
// document block. Before this the case was simply missing: the block was
// dropped and the model answered as if nothing had been attached. Measured
// against claude-sonnet-5-5, a document block reads the file correctly while
// the same PDF on Anthropic's OpenAI-compatible endpoint returns a 400.
func TestAnthropicProviderTranslatesFileToDocument(t *testing.T) {
	p := NewAnthropicProvider("k", "")
	msg := Message{Role: "user", Content: []ContentBlock{
		{Type: "text", Text: "apa isinya"},
		{Type: "file", File: &FileData{FileData: "data:application/pdf;base64,JVBERi0xLjQ="}},
	}}

	blocks, ok := p.buildContent(msg).([]map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content shape: %#v", p.buildContent(msg))
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want text + document: %#v", len(blocks), blocks)
	}

	doc := blocks[1]
	if doc["type"] != "document" {
		t.Fatalf("second block = %v, want a document block", doc["type"])
	}
	src, _ := doc["source"].(map[string]interface{})
	if src["type"] != "base64" || src["media_type"] != "application/pdf" || src["data"] != "JVBERi0xLjQ=" {
		t.Errorf("document source = %#v", src)
	}
}

// Images keep working through the same path.
func TestAnthropicProviderTranslatesImage(t *testing.T) {
	p := NewAnthropicProvider("k", "")
	msg := Message{Role: "user", Content: []ContentBlock{
		{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,QUJD"}},
	}}

	blocks := p.buildContent(msg).([]map[string]interface{})
	if len(blocks) != 1 || blocks[0]["type"] != "image" {
		t.Fatalf("blocks = %#v, want one image block", blocks)
	}
	src := blocks[0]["source"].(map[string]interface{})
	if src["media_type"] != "image/png" || src["data"] != "QUJD" {
		t.Errorf("image source = %#v", src)
	}
}

// Sonnet 5.5 answers a request carrying temperature with
// "`temperature` is deprecated for this model" and a 400 — sampling parameters
// are gone from the current Claude models. The gateway flavour still sends them.
func TestAnthropicProviderOmitsSamplingForClaude(t *testing.T) {
	opts := map[string]interface{}{"max_tokens": 1024, "temperature": 0.7}

	direct := NewAnthropicProvider("k", "").buildAnthropicRequest(
		[]Message{{Role: "user", Content: "hai"}}, nil, "claude-sonnet-5-5", opts)
	if _, present := direct["temperature"]; present {
		t.Error("temperature was sent to the Anthropic API; the model rejects it")
	}

	gateway := NewOpenCodeProvider("k", "").buildAnthropicRequest(
		[]Message{{Role: "user", Content: "hai"}}, nil, "minimax-m3", opts)
	if gateway["temperature"] != 0.7 {
		t.Errorf("gateway temperature = %v, want it preserved", gateway["temperature"])
	}
}
