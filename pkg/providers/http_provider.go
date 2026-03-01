// Pepebot - Ultra-lightweight personal AI agent
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

	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

type HTTPProvider struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
}

func NewHTTPProvider(apiKey, apiBase string) *HTTPProvider {
	return &HTTPProvider{
		apiKey:  apiKey,
		apiBase: apiBase,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

func (p *HTTPProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	if p.apiBase == "" {
		return nil, fmt.Errorf("API base not configured")
	}

	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		toolNames = append(toolNames, t.Function.Name)
	}

	logger.DebugCF("provider", "HTTP chat request", map[string]interface{}{
		"model":          model,
		"api_base":       p.apiBase,
		"messages":       len(messages),
		"tools":          len(tools),
		"tool_names":     toolNames,
		"has_max_tokens": options["max_tokens"] != nil,
		"temperature":    options["temperature"],
	})

	requestBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	if len(tools) > 0 {
		requestBody["tools"] = tools
		requestBody["tool_choice"] = "auto"
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		requestBody["max_tokens"] = maxTokens
	}

	if temperature, ok := options["temperature"].(float64); ok {
		requestBody["temperature"] = temperature
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiBase+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		authHeader := "Bearer " + p.apiKey
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	parsed, err := p.parseResponse(body)
	if err != nil {
		return nil, err
	}

	respToolNames := make([]string, 0, len(parsed.ToolCalls))
	for _, tc := range parsed.ToolCalls {
		respToolNames = append(respToolNames, tc.Name)
	}

	logger.DebugCF("provider", "HTTP chat response", map[string]interface{}{
		"finish_reason":   parsed.FinishReason,
		"content_len":     len(parsed.Content),
		"content_preview": truncateString(parsed.Content, 120),
		"tool_calls":      len(parsed.ToolCalls),
		"tool_names":      respToolNames,
	})

	return parsed, nil
}

func (p *HTTPProvider) parseResponse(body []byte) (*LLMResponse, error) {
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function *struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *UsageInfo `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return &LLMResponse{
			Content:      "",
			FinishReason: "stop",
		}, nil
	}

	choice := apiResponse.Choices[0]

	toolCalls := make([]ToolCall, 0, len(choice.Message.ToolCalls))
	for _, tc := range choice.Message.ToolCalls {
		arguments := make(map[string]interface{})
		name := ""

		// Handle OpenAI format with nested function object
		if tc.Type == "function" && tc.Function != nil {
			name = tc.Function.Name
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &arguments); err != nil {
					arguments["raw"] = tc.Function.Arguments
				}
			}
		} else if tc.Function != nil {
			// Legacy format without type field
			name = tc.Function.Name
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &arguments); err != nil {
					arguments["raw"] = tc.Function.Arguments
				}
			}
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      name,
			Arguments: arguments,
		})
	}

	return &LLMResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: choice.FinishReason,
		Usage:        apiResponse.Usage,
	}, nil
}

func (p *HTTPProvider) ChatStream(ctx context.Context, messages []Message, model string, options map[string]interface{}, callback StreamCallback) error {
	if p.apiBase == "" {
		return fmt.Errorf("API base not configured")
	}

	requestBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		requestBody["max_tokens"] = maxTokens
	}

	if temperature, ok := options["temperature"].(float64); ok {
		requestBody["temperature"] = temperature
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiBase+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
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

		if data == "[DONE]" {
			callback(StreamChunk{Done: true})
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				callback(StreamChunk{Content: delta.Content})
			}
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
				callback(StreamChunk{Done: true})
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	callback(StreamChunk{Done: true})
	return nil
}

func (p *HTTPProvider) GetDefaultModel() string {
	return ""
}

func CreateProvider(cfg *config.Config) (LLMProvider, error) {
	model := cfg.Agents.Defaults.Model

	var apiKey, apiBase string

	lowerModel := strings.ToLower(model)

	switch {
	case strings.HasPrefix(model, "maia/"):
		apiKey = cfg.Providers.MAIARouter.APIKey
		if cfg.Providers.MAIARouter.APIBase != "" {
			apiBase = cfg.Providers.MAIARouter.APIBase
		} else {
			apiBase = "https://api.maiarouter.ai/v1"
		}

	case strings.HasPrefix(model, "openrouter/") || strings.HasPrefix(model, "anthropic/") || strings.HasPrefix(model, "openai/") || strings.HasPrefix(model, "meta-llama/") || strings.HasPrefix(model, "deepseek/") || strings.HasPrefix(model, "google/"):
		apiKey = cfg.Providers.OpenRouter.APIKey
		if cfg.Providers.OpenRouter.APIBase != "" {
			apiBase = cfg.Providers.OpenRouter.APIBase
		} else {
			apiBase = "https://openrouter.ai/api/v1"
		}

	case strings.Contains(lowerModel, "claude") || strings.HasPrefix(model, "anthropic/"):
		apiKey = cfg.Providers.Anthropic.APIKey
		apiBase = cfg.Providers.Anthropic.APIBase
		if apiBase == "" {
			apiBase = "https://api.anthropic.com/v1"
		}

	case strings.Contains(lowerModel, "gpt") || strings.HasPrefix(model, "openai/"):
		apiKey = cfg.Providers.OpenAI.APIKey
		apiBase = cfg.Providers.OpenAI.APIBase
		if apiBase == "" {
			apiBase = "https://api.openai.com/v1"
		}

	case strings.Contains(lowerModel, "gemini") || strings.HasPrefix(model, "google/"):
		apiKey = cfg.Providers.Gemini.APIKey
		apiBase = cfg.Providers.Gemini.APIBase
		if apiBase == "" {
			apiBase = "https://generativelanguage.googleapis.com/v1beta"
		}

	case strings.Contains(lowerModel, "glm") || strings.Contains(lowerModel, "zhipu") || strings.Contains(lowerModel, "zai"):
		apiKey = cfg.Providers.Zhipu.APIKey
		apiBase = cfg.Providers.Zhipu.APIBase
		if apiBase == "" {
			apiBase = "https://open.bigmodel.cn/api/paas/v4"
		}

	case strings.Contains(lowerModel, "groq") || strings.HasPrefix(model, "groq/"):
		apiKey = cfg.Providers.Groq.APIKey
		apiBase = cfg.Providers.Groq.APIBase
		if apiBase == "" {
			apiBase = "https://api.groq.com/openai/v1"
		}

	case cfg.Providers.VLLM.APIBase != "":
		apiKey = cfg.Providers.VLLM.APIKey
		apiBase = cfg.Providers.VLLM.APIBase

	default:
		if cfg.Providers.MAIARouter.APIKey != "" {
			apiKey = cfg.Providers.MAIARouter.APIKey
			if cfg.Providers.MAIARouter.APIBase != "" {
				apiBase = cfg.Providers.MAIARouter.APIBase
			} else {
				apiBase = "https://api.maiarouter.ai/v1"
			}
		} else if cfg.Providers.OpenRouter.APIKey != "" {
			apiKey = cfg.Providers.OpenRouter.APIKey
			if cfg.Providers.OpenRouter.APIBase != "" {
				apiBase = cfg.Providers.OpenRouter.APIBase
			} else {
				apiBase = "https://openrouter.ai/api/v1"
			}
		} else {
			return nil, fmt.Errorf("no API key configured for model: %s", model)
		}
	}

	if apiKey == "" && !strings.HasPrefix(model, "bedrock/") {
		return nil, fmt.Errorf("no API key configured for provider (model: %s)", model)
	}

	if apiBase == "" {
		return nil, fmt.Errorf("no API base configured for provider (model: %s)", model)
	}

	return NewHTTPProvider(apiKey, apiBase), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
