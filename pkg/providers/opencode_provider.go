// Pepebot -Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

type OpenCodeProvider struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
}

func NewOpenCodeProvider(apiKey, apiBase string) *OpenCodeProvider {
	if apiBase == "" {
		apiBase = "https://opencode.ai/zen/go"
	}
	return &OpenCodeProvider{
		apiKey:  apiKey,
		apiBase: apiBase,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

func (p *OpenCodeProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		toolNames = append(toolNames, t.Function.Name)
	}

	logger.DebugCF("provider", "OpenCode Go chat request", map[string]interface{}{
		"model":      model,
		"api_base":   p.apiBase,
		"messages":   len(messages),
		"tools":      len(tools),
		"tool_names": toolNames,
	})

	requestBody := p.buildAnthropicRequest(messages, tools, model, options)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("opencode: failed to marshal request: %w", err)
	}

	endpoint := p.apiBase + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("opencode: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("opencode: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode API error (status %d): %s", resp.StatusCode, string(body))
	}

	parsed, err := p.parseAnthropicResponse(body)
	if err != nil {
		return nil, err
	}

	respToolNames := make([]string, 0, len(parsed.ToolCalls))
	for _, tc := range parsed.ToolCalls {
		respToolNames = append(respToolNames, tc.Name)
	}

	logger.DebugCF("provider", "OpenCode Go chat response", map[string]interface{}{
		"finish_reason":   parsed.FinishReason,
		"content_len":     len(parsed.Content),
		"content_preview": truncateString(parsed.Content, 120),
		"tool_calls":      len(parsed.ToolCalls),
		"tool_names":      respToolNames,
	})

	return parsed, nil
}

func (p *OpenCodeProvider) ChatStream(ctx context.Context, messages []Message, model string, options map[string]interface{}, callback StreamCallback) error {
	requestBody := p.buildAnthropicRequest(messages, nil, model, options)
	requestBody["stream"] = true

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("opencode: failed to marshal request: %w", err)
	}

	endpoint := p.apiBase + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("opencode: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opencode API error (status %d): %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index,omitempty"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"delta,omitempty"`
			Message *struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
				} `json:"content"`
			} `json:"message,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				callback(StreamChunk{Content: event.Delta.Text})
			}
		case "message_stop":
			callback(StreamChunk{Done: true})
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("opencode: error reading stream: %w", err)
	}

	callback(StreamChunk{Done: true})
	return nil
}

func (p *OpenCodeProvider) GetDefaultModel() string {
	return "minimax-m2.5"
}

func (p *OpenCodeProvider) buildAnthropicRequest(messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) map[string]interface{} {
	request := map[string]interface{}{
		"model": model,
	}

	var systemPrompt string
	var anthropicMessages []map[string]interface{}

	for _, msg := range messages {
		if msg.Role == "system" {
			contentStr := getMessageContentString(msg.Content)
			if systemPrompt != "" {
				systemPrompt += "\n\n" + contentStr
			} else {
				systemPrompt = contentStr
			}
			continue
		}

		role := msg.Role
		if role == "tool" {
			role = "user"
		}

		content := p.buildContent(msg)

		if msg.Role == "tool" && msg.ToolCallID != "" {
			content = []map[string]interface{}{
				{
					"type":        "tool_result",
					"tool_use_id": msg.ToolCallID,
					"content":     getMessageContentString(msg.Content),
				},
			}
		}

		anthropicMsg := map[string]interface{}{
			"role":    role,
			"content": content,
		}

		if len(msg.ToolCalls) > 0 {
			contentArray := []map[string]interface{}{}
			contentStr := getMessageContentString(msg.Content)
			if contentStr != "" {
				contentArray = append(contentArray, map[string]interface{}{
					"type": "text",
					"text": contentStr,
				})
			}
			for _, tc := range msg.ToolCalls {
				name := tc.Name
				if name == "" && tc.Function != nil {
					name = tc.Function.Name
				}
				contentArray = append(contentArray, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  name,
					"input": tc.Arguments,
				})
			}
			anthropicMsg["content"] = contentArray
		}

		anthropicMessages = append(anthropicMessages, anthropicMsg)
	}

	if systemPrompt != "" {
		request["system"] = systemPrompt
	}

	request["messages"] = anthropicMessages

	if len(tools) > 0 {
		anthropicTools := []map[string]interface{}{}
		for _, tool := range tools {
			anthropicTools = append(anthropicTools, map[string]interface{}{
				"name":         tool.Function.Name,
				"description":  tool.Function.Description,
				"input_schema": tool.Function.Parameters,
			})
		}
		request["tools"] = anthropicTools
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		request["max_tokens"] = maxTokens
	} else {
		request["max_tokens"] = 4096
	}

	if temperature, ok := options["temperature"].(float64); ok {
		request["temperature"] = temperature
	}

	return request
}

func (p *OpenCodeProvider) buildContent(msg Message) interface{} {
	switch content := msg.Content.(type) {
	case string:
		if content == "" {
			return []map[string]interface{}{}
		}
		return []map[string]interface{}{
			{"type": "text", "text": content},
		}
	case []interface{}:
		var result []map[string]interface{}
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				switch blockMap["type"] {
				case "text":
					if text, ok := blockMap["text"].(string); ok && text != "" {
						result = append(result, map[string]interface{}{
							"type": "text",
							"text": text,
						})
					}
				case "image_url":
					if imgURL, ok := blockMap["image_url"].(map[string]interface{}); ok {
						if url, ok := imgURL["url"].(string); ok {
							if strings.HasPrefix(url, "data:") {
								mimeType, data := parseDataURL(url)
								result = append(result, map[string]interface{}{
									"type": "image",
									"source": map[string]interface{}{
										"type":       "base64",
										"media_type": mimeType,
										"data":       data,
									},
								})
							}
						}
					}
				}
			}
		}
		return result
	default:
		if msg.ToolCallID != "" {
			return []map[string]interface{}{}
		}
		return []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("%v", content)},
		}
	}
}

func (p *OpenCodeProvider) parseAnthropicResponse(body []byte) (*LLMResponse, error) {
	var resp struct {
		Content []struct {
			Type  string                 `json:"type"`
			Text  string                 `json:"text,omitempty"`
			ID    string                 `json:"id,omitempty"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("opencode: failed to parse response: %w", err)
	}

	var contentParts []string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				contentParts = append(contentParts, block.Text)
			}
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	finishReason := "stop"
	switch resp.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	case "stop_sequence":
		finishReason = "stop"
	}

	var usage *UsageInfo
	if resp.Usage != nil {
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return &LLMResponse{
		Content:      strings.Join(contentParts, ""),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}
