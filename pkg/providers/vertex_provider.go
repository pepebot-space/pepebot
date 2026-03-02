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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexProvider implements LLMProvider for Google Vertex AI using service account credentials
type VertexProvider struct {
	projectID       string
	region          string
	credentialsFile string
	httpClient      *http.Client
	tokenSource     oauth2.TokenSource
	mu              sync.Mutex
}

// NewVertexProvider creates a Vertex AI provider with service account authentication
func NewVertexProvider(credentialsFile, projectID, region string) (*VertexProvider, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("vertex: credentials_file is required (path to service account JSON)")
	}
	if projectID == "" {
		return nil, fmt.Errorf("vertex: project_id is required")
	}
	if region == "" {
		region = "us-central1"
	}

	// Read the service account JSON file
	credBytes, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to read credentials file %s: %w", credentialsFile, err)
	}

	// Create OAuth2 token source with Vertex AI scope
	creds, err := google.CredentialsFromJSON(context.Background(), credBytes,
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to parse credentials: %w", err)
	}

	return &VertexProvider{
		projectID:       projectID,
		region:          region,
		credentialsFile: credentialsFile,
		httpClient: &http.Client{
			Timeout: 0,
		},
		tokenSource: creds.TokenSource,
	}, nil
}

// Chat sends a chat completion request to Vertex AI
func (p *VertexProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	// Strip "vertex/" prefix from model name
	modelName := strings.TrimPrefix(model, "vertex/")

	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		toolNames = append(toolNames, t.Function.Name)
	}

	logger.DebugCF("provider", "Vertex AI chat request", map[string]interface{}{
		"model":       modelName,
		"project_id":  p.projectID,
		"region":      p.region,
		"messages":    len(messages),
		"tools":       len(tools),
		"tool_names":  toolNames,
		"temperature": options["temperature"],
	})

	// Build the Vertex AI request body
	requestBody := p.buildGenerateContentRequest(messages, tools, options)

	// Build the endpoint URL
	endpoint := p.buildEndpointURL(modelName, false)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to marshal request: %w", err)
	}

	// Get OAuth2 token
	token, err := p.getToken()
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to get auth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vertex API error (status %d): %s", resp.StatusCode, string(body))
	}

	parsed, err := p.parseResponse(body)
	if err != nil {
		return nil, err
	}

	respToolNames := make([]string, 0, len(parsed.ToolCalls))
	for _, tc := range parsed.ToolCalls {
		respToolNames = append(respToolNames, tc.Name)
	}

	logger.DebugCF("provider", "Vertex AI chat response", map[string]interface{}{
		"finish_reason":   parsed.FinishReason,
		"content_len":     len(parsed.Content),
		"content_preview": truncateString(parsed.Content, 120),
		"tool_calls":      len(parsed.ToolCalls),
		"tool_names":      respToolNames,
	})

	return parsed, nil
}

// ChatStream sends a streaming chat completion request to Vertex AI
func (p *VertexProvider) ChatStream(ctx context.Context, messages []Message, model string, options map[string]interface{}, callback StreamCallback) error {
	modelName := strings.TrimPrefix(model, "vertex/")

	requestBody := p.buildGenerateContentRequest(messages, nil, options)

	endpoint := p.buildEndpointURL(modelName, true)

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("vertex: failed to marshal request: %w", err)
	}

	token, err := p.getToken()
	if err != nil {
		return fmt.Errorf("vertex: failed to get auth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("vertex: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vertex: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vertex API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Vertex AI streaming returns newline-delimited JSON array chunks
	scanner := bufio.NewScanner(resp.Body)
	// Increase scanner buffer for large responses
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var inArray bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || line == "[" {
			inArray = true
			continue
		}
		if line == "]" {
			break
		}

		// Remove trailing comma if present (JSON array elements)
		line = strings.TrimSuffix(line, ",")

		if !inArray {
			// Try to parse as single JSON object (non-array streaming)
			if strings.HasPrefix(line, "data: ") {
				line = strings.TrimPrefix(line, "data: ")
				if line == "[DONE]" {
					callback(StreamChunk{Done: true})
					return nil
				}
			}
		}

		if line == "" {
			continue
		}

		var chunk vertexResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					callback(StreamChunk{Content: part.Text})
				}
			}
			if candidate.FinishReason == "STOP" || candidate.FinishReason == "MAX_TOKENS" {
				callback(StreamChunk{Done: true})
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("vertex: error reading stream: %w", err)
	}

	callback(StreamChunk{Done: true})
	return nil
}

// GetDefaultModel returns the default model for Vertex AI
func (p *VertexProvider) GetDefaultModel() string {
	return "vertex/gemini-2.0-flash"
}

// buildEndpointURL constructs the Vertex AI API endpoint
func (p *VertexProvider) buildEndpointURL(model string, stream bool) string {
	action := "generateContent"
	if stream {
		action = "streamGenerateContent?alt=sse"
	}

	// For "global" region, use the base hostname without region prefix
	var host string
	if p.region == "global" {
		host = "aiplatform.googleapis.com"
	} else {
		host = fmt.Sprintf("%s-aiplatform.googleapis.com", p.region)
	}

	return fmt.Sprintf(
		"https://%s/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
		host, p.projectID, p.region, model, action,
	)
}

// getToken retrieves a valid OAuth2 access token
func (p *VertexProvider) getToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	token, err := p.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	return token.AccessToken, nil
}

// vertexResponse represents the Vertex AI generateContent response
type vertexResponse struct {
	Candidates    []vertexCandidate    `json:"candidates"`
	UsageMetadata *vertexUsageMetadata `json:"usageMetadata,omitempty"`
}

type vertexCandidate struct {
	Content      vertexContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type vertexContent struct {
	Role  string       `json:"role"`
	Parts []vertexPart `json:"parts"`
}

type vertexPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *vertexFunctionCall `json:"functionCall,omitempty"`
	InlineData   *vertexInlineData   `json:"inlineData,omitempty"`
}

type vertexFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type vertexInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type vertexUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// buildGenerateContentRequest builds the Vertex AI request body from pepebot messages
func (p *VertexProvider) buildGenerateContentRequest(messages []Message, tools []ToolDefinition, options map[string]interface{}) map[string]interface{} {
	request := map[string]interface{}{}

	// Convert messages to Vertex AI format
	var systemInstruction *map[string]interface{}
	vertexContents := []map[string]interface{}{}

	for _, msg := range messages {
		if msg.Role == "system" {
			// System messages become systemInstruction
			contentStr := getMessageContentString(msg.Content)
			si := map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": contentStr},
				},
			}
			systemInstruction = &si
			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		parts := []map[string]interface{}{}

		// Handle tool call results
		if msg.Role == "tool" && msg.ToolCallID != "" {
			contentStr := getMessageContentString(msg.Content)
			parts = append(parts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name": msg.ToolCallID,
					"response": map[string]interface{}{
						"content": contentStr,
					},
				},
			})
		} else if len(msg.ToolCalls) > 0 {
			// Handle assistant messages with tool calls
			contentStr := getMessageContentString(msg.Content)
			if contentStr != "" {
				parts = append(parts, map[string]interface{}{"text": contentStr})
			}
			for _, tc := range msg.ToolCalls {
				args := tc.Arguments
				if args == nil {
					args = map[string]interface{}{}
				}
				name := tc.Name
				if name == "" && tc.Function != nil {
					name = tc.Function.Name
					if tc.Function.Arguments != "" {
						json.Unmarshal([]byte(tc.Function.Arguments), &args)
					}
				}
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": name,
						"args": args,
					},
				})
			}
		} else {
			// Regular content
			switch content := msg.Content.(type) {
			case string:
				if content != "" {
					parts = append(parts, map[string]interface{}{"text": content})
				}
			case []interface{}:
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						switch blockMap["type"] {
						case "text":
							if text, ok := blockMap["text"].(string); ok && text != "" {
								parts = append(parts, map[string]interface{}{"text": text})
							}
						case "image_url":
							if imgURL, ok := blockMap["image_url"].(map[string]interface{}); ok {
								if url, ok := imgURL["url"].(string); ok {
									if strings.HasPrefix(url, "data:") {
										// Inline base64 data
										mimeType, data := parseDataURL(url)
										parts = append(parts, map[string]interface{}{
											"inlineData": map[string]interface{}{
												"mimeType": mimeType,
												"data":     data,
											},
										})
									}
								}
							}
						}
					}
				}
			}
		}

		if len(parts) > 0 {
			vertexContents = append(vertexContents, map[string]interface{}{
				"role":  role,
				"parts": parts,
			})
		}
	}

	request["contents"] = vertexContents

	if systemInstruction != nil {
		request["systemInstruction"] = *systemInstruction
	}

	// Convert tools to Vertex AI format
	if len(tools) > 0 {
		vertexFunctions := []map[string]interface{}{}
		for _, tool := range tools {
			fn := map[string]interface{}{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
			}
			if len(tool.Function.Parameters) > 0 {
				fn["parameters"] = tool.Function.Parameters
			}
			vertexFunctions = append(vertexFunctions, fn)
		}
		request["tools"] = []map[string]interface{}{
			{"functionDeclarations": vertexFunctions},
		}
	}

	// Generation config
	genConfig := map[string]interface{}{}
	if maxTokens, ok := options["max_tokens"].(int); ok {
		genConfig["maxOutputTokens"] = maxTokens
	}
	if temperature, ok := options["temperature"].(float64); ok {
		genConfig["temperature"] = temperature
	}
	if len(genConfig) > 0 {
		request["generationConfig"] = genConfig
	}

	return request
}

// parseResponse parses Vertex AI response into LLMResponse
func (p *VertexProvider) parseResponse(body []byte) (*LLMResponse, error) {
	var resp vertexResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("vertex: failed to parse response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return &LLMResponse{
			Content:      "",
			FinishReason: "stop",
		}, nil
	}

	candidate := resp.Candidates[0]

	var contentParts []string
	var toolCalls []ToolCall

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			contentParts = append(contentParts, part.Text)
		}
		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, time.Now().UnixNano()),
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}

	finishReason := "stop"
	switch candidate.FinishReason {
	case "STOP":
		finishReason = "stop"
	case "MAX_TOKENS":
		finishReason = "length"
	case "SAFETY":
		finishReason = "content_filter"
	case "TOOL_USE":
		finishReason = "tool_calls"
	}

	var usage *UsageInfo
	if resp.UsageMetadata != nil {
		usage = &UsageInfo{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return &LLMResponse{
		Content:      strings.Join(contentParts, ""),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

// getMessageContentString extracts string content from a Message.Content field
func getMessageContentString(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, block := range c {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if text, ok := blockMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", content)
	}
}

// parseDataURL parses a data URL into mime type and base64 data
func parseDataURL(dataURL string) (string, string) {
	// Format: data:mime/type;base64,....
	if !strings.HasPrefix(dataURL, "data:") {
		return "", ""
	}
	rest := strings.TrimPrefix(dataURL, "data:")
	parts := strings.SplitN(rest, ",", 2)
	if len(parts) != 2 {
		return "", ""
	}
	meta := parts[0]
	data := parts[1]
	mimeType := strings.TrimSuffix(meta, ";base64")
	return mimeType, data
}
