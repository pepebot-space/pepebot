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
	provider   string
	httpClient *http.Client
}

func NewHTTPProvider(apiKey, apiBase string) *HTTPProvider {
	return NewHTTPProviderFor("", apiKey, apiBase)
}

// NewHTTPProviderFor records which provider key selected this endpoint, so the model id
// can have that name stripped back off if it was repeated there. See modelForProvider.
func NewHTTPProviderFor(provider, apiKey, apiBase string) *HTTPProvider {
	return &HTTPProvider{
		apiKey:   apiKey,
		apiBase:  apiBase,
		provider: strings.ToLower(provider),
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

// modelForProvider drops a provider name repeated at the front of a model id. Choosing
// the endpoint is the config's job; carrying the provider name into the model as well
// only gets the request rejected — MAIA Router answers "no healthy deployments for
// model=maiarouter/zai/glm-5.3-flash" for exactly this.
//
// Only the configured provider key is stripped, never a vendor namespace: OpenRouter
// genuinely wants "anthropic/claude-3.5-sonnet", and MAIA genuinely wants
// "maia/gemini-2.5-flash", so "maia" is left alone even when it is the configured alias.
func modelForProvider(provider, model string) string {
	if provider == "" || provider == "maia" {
		return model
	}
	prefix := provider + "/"
	if strings.HasPrefix(strings.ToLower(model), prefix) {
		return model[len(prefix):]
	}
	return model
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
		"model":          modelForProvider(p.provider, model),
		"api_base":       p.apiBase,
		"messages":       len(messages),
		"tools":          len(tools),
		"tool_names":     toolNames,
		"has_max_tokens": options["max_tokens"] != nil,
		"temperature":    options["temperature"],
	})

	requestBody := map[string]interface{}{
		"model":    modelForProvider(p.provider, model),
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
	applyExtraBody(requestBody, options)

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

// applyExtraBody copies caller-supplied request fields into the body verbatim.
//
// This is how a thinking model is told not to think. GLM spends its output budget
// on reasoning_content before it writes a single character of the answer — on a
// tight budget the reasoning consumes all of it and `content` comes back empty,
// which the agent loop can only report as "no response to give". Of the four ways
// to ask for that upstream, only extra_body survives the litellm hop:
//
//	"extra_body": {"thinking": {"type": "disabled"}}
//
// Kept as a passthrough rather than a named flag so any other provider-specific
// parameter can be set the same way without another release.
func applyExtraBody(requestBody map[string]interface{}, options map[string]interface{}) {
	extra, ok := options["extra_body"].(map[string]interface{})
	if !ok || len(extra) == 0 {
		return
	}
	requestBody["extra_body"] = extra
}

func (p *HTTPProvider) parseResponse(body []byte) (*LLMResponse, error) {
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				// Thinking models put their prose here and can leave Content
				// empty when the output budget runs out mid-thought. Better a
				// verbose answer than "no response to give".
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
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

	content := choice.Message.Content
	if content == "" && choice.Message.ReasoningContent != "" {
		content = choice.Message.ReasoningContent
		logger.WarnCF("provider", "Model returned only reasoning_content; using it as the reply", map[string]interface{}{
			"finish_reason": choice.FinishReason,
			"chars":         len(content),
		})
	}

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
		Content:      content,
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
		"model":    modelForProvider(p.provider, model),
		"messages": messages,
		"stream":   true,
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		requestBody["max_tokens"] = maxTokens
	}

	if temperature, ok := options["temperature"].(float64); ok {
		requestBody["temperature"] = temperature
	}
	applyExtraBody(requestBody, options)

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

	var reasoning strings.Builder
	sentContent := false

	// flushReasoning emits the buffered thinking only when the answer never
	// came — a budget exhausted mid-thought used to reach the user as
	// "I've completed processing but have no response to give."
	flushReasoning := func() {
		if sentContent || reasoning.Len() == 0 {
			return
		}
		logger.WarnCF("provider", "Stream ended with reasoning but no answer; sending the reasoning", map[string]interface{}{
			"chars": reasoning.Len(),
		})
		callback(StreamChunk{Content: reasoning.String()})
		sentContent = true
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
			flushReasoning()
			callback(StreamChunk{Done: true})
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
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
				sentContent = true
				callback(StreamChunk{Content: delta.Content})
			}
			// Reasoning is buffered, never streamed: a thinking model emits
			// several times more of it than answer, and the user asked a
			// question, not for the deliberation. It is only used if the answer
			// never arrives — see flushReasoning.
			if delta.ReasoningContent != "" {
				reasoning.WriteString(delta.ReasoningContent)
			}
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
				flushReasoning()
				callback(StreamChunk{Done: true})
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	flushReasoning()
	callback(StreamChunk{Done: true})
	return nil
}

func (p *HTTPProvider) GetDefaultModel() string {
	return ""
}

func CreateProvider(cfg *config.Config) (LLMProvider, error) {
	return CreateProviderWithOverrides(cfg, "", "")
}

// CreateProviderWithOverrides creates a provider with optional model and provider overrides.
// If overrideModel/overrideProvider are empty, falls back to config defaults.
func CreateProviderWithOverrides(cfg *config.Config, overrideModel, overrideProvider string) (LLMProvider, error) {
	model := cfg.Agents.Defaults.Model
	if overrideModel != "" {
		model = overrideModel
	}
	provider := strings.ToLower(cfg.Agents.Defaults.Provider)
	if overrideProvider != "" {
		provider = strings.ToLower(overrideProvider)
	}

	var apiKey, apiBase string

	lowerModel := strings.ToLower(model)

	// If provider is explicitly set, use it directly instead of model prefix detection
	if provider != "" {
		switch provider {
		case "vertex":
			return NewVertexProvider(
				cfg.Providers.Vertex.CredentialsFile,
				cfg.Providers.Vertex.ProjectID,
				cfg.Providers.Vertex.Region,
			)
		case "maiarouter", "maia":
			apiKey = cfg.Providers.MAIARouter.APIKey
			if cfg.Providers.MAIARouter.APIBase != "" {
				apiBase = cfg.Providers.MAIARouter.APIBase
			} else {
				apiBase = "https://api.maiarouter.ai/v1"
			}
		case "openrouter":
			apiKey = cfg.Providers.OpenRouter.APIKey
			if cfg.Providers.OpenRouter.APIBase != "" {
				apiBase = cfg.Providers.OpenRouter.APIBase
			} else {
				apiBase = "https://openrouter.ai/api/v1"
			}
		case "anthropic":
			apiKey = cfg.Providers.Anthropic.APIKey
			apiBase = cfg.Providers.Anthropic.APIBase
			if apiBase == "" {
				apiBase = "https://api.anthropic.com/v1"
			}
		case "openai":
			apiKey = cfg.Providers.OpenAI.APIKey
			apiBase = cfg.Providers.OpenAI.APIBase
			if apiBase == "" {
				apiBase = "https://api.openai.com/v1"
			}
		case "gemini":
			apiKey = cfg.Providers.Gemini.APIKey
			apiBase = cfg.Providers.Gemini.APIBase
			if apiBase == "" {
				apiBase = "https://generativelanguage.googleapis.com/v1beta"
			}
		case "zhipu":
			apiKey = cfg.Providers.Zhipu.APIKey
			apiBase = cfg.Providers.Zhipu.APIBase
			if apiBase == "" {
				apiBase = "https://open.bigmodel.cn/api/paas/v4"
			}
		case "groq":
			apiKey = cfg.Providers.Groq.APIKey
			apiBase = cfg.Providers.Groq.APIBase
			if apiBase == "" {
				apiBase = "https://api.groq.com/openai/v1"
			}
		case "vllm":
			apiKey = cfg.Providers.VLLM.APIKey
			apiBase = cfg.Providers.VLLM.APIBase
		case "opencodego":
			return NewOpenCodeProvider(
				cfg.Providers.OpenCodeGo.APIKey,
				cfg.Providers.OpenCodeGo.APIBase,
			), nil
		default:
			return nil, fmt.Errorf("unknown provider: %s", provider)
		}

		if apiKey == "" {
			return nil, fmt.Errorf("no API key configured for provider: %s", provider)
		}
		if apiBase == "" {
			return nil, fmt.Errorf("no API base configured for provider: %s", provider)
		}
		return NewHTTPProviderFor(provider, apiKey, apiBase), nil
	}

	// Fallback: auto-detect provider from model prefix/name
	switch {
	case strings.HasPrefix(model, "vertex/"):
		return NewVertexProvider(
			cfg.Providers.Vertex.CredentialsFile,
			cfg.Providers.Vertex.ProjectID,
			cfg.Providers.Vertex.Region,
		)

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

	if apiKey == "" && !strings.HasPrefix(model, "bedrock/") && !strings.HasPrefix(model, "vertex/") {
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
