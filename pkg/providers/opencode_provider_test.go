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
	expected := "minimax-m2.5"
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

	request := provider.buildAnthropicRequest(messages, nil, "minimax-m2.5", nil)

	model, ok := request["model"].(string)
	if !ok || model != "minimax-m2.5" {
		t.Errorf("Expected model 'minimax-m2.5', got %v", request["model"])
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

	request := provider.buildAnthropicRequest(messages, nil, "minimax-m2.5", nil)

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

	request := provider.buildAnthropicRequest(messages, tools, "minimax-m2.5", nil)

	toolsArray, ok := request["tools"].([]map[string]interface{})
	if !ok || len(toolsArray) != 1 {
		t.Errorf("Expected 1 tool, got %v", request["tools"])
	}

	if toolsArray[0]["name"] != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %v", toolsArray[0]["name"])
	}
}
