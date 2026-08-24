// Pepebot - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import (
	"testing"
)

func TestOpenCodeProvider_GetDefaultModel(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")
	expected := "minimax-m3"
	if provider.GetDefaultModel() != expected {
		t.Errorf("Expected default model %s, got %s", expected, provider.GetDefaultModel())
	}
}

func TestOpenCodeProvider_DefaultAPIBase(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")
	expected := "https://opencode.ai/zen/go"
	if provider.apiBase != expected {
		t.Errorf("Expected apiBase %s, got %s", expected, provider.apiBase)
	}
}

func TestOpenCodeProvider_CustomAPIBase(t *testing.T) {
	customBase := "https://custom.opencode.ai/api"
	provider := NewOpenCodeProvider("test-key", customBase)
	if provider.apiBase != customBase {
		t.Errorf("Expected apiBase %s, got %s", customBase, provider.apiBase)
	}
}

func TestOpenCodeProvider_BuildAnthropicRequest(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")

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

func TestOpenCodeProvider_BuildAnthropicRequestWithSystem(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")

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

func TestOpenCodeProvider_BuildAnthropicRequestWithTools(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")

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
func TestOpenCodeProvider_ToolUseInputIsAlwaysObject(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")

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
func TestOpenCodeProvider_MultimodalContent(t *testing.T) {
	provider := NewOpenCodeProvider("test-key", "")

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
