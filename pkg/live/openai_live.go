// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

// OpenAILiveProvider implements LiveProvider for OpenAI Realtime API and compatible endpoints (like MAIA Router)
type OpenAILiveProvider struct {
	name    string
	apiBase string
	apiKey  string
}

// NewOpenAILiveProvider creates a new OpenAI compatible Realtime API provider
func NewOpenAILiveProvider(name, apiBase, apiKey string) (*OpenAILiveProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%s live: api_key is required", name)
	}

	if apiBase == "" {
		// Default to OpenAI if not provided
		apiBase = "https://api.openai.com/v1"
	}

	// Clean up trailing slashes
	apiBase = strings.TrimSuffix(apiBase, "/")

	logger.InfoCF("live", "OpenAI-compatible Live provider initialized", map[string]interface{}{
		"provider": name,
		"api_base": apiBase,
	})

	return &OpenAILiveProvider{
		name:    name,
		apiBase: apiBase,
		apiKey:  apiKey,
	}, nil
}

// Name returns the provider name ("openai" or "maiarouter")
func (p *OpenAILiveProvider) Name() string {
	return p.name
}

// BuildUpstreamURL constructs the WebSocket endpoint
// Standard format: wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview
func (p *OpenAILiveProvider) BuildUpstreamURL(model string) string {
	// Convert http:// to ws:// and https:// to wss://
	wsBase := p.apiBase
	if strings.HasPrefix(wsBase, "https://") {
		wsBase = "wss://" + strings.TrimPrefix(wsBase, "https://")
	} else if strings.HasPrefix(wsBase, "http://") {
		wsBase = "ws://" + strings.TrimPrefix(wsBase, "http://")
	}

	return fmt.Sprintf("%s/realtime?model=%s", wsBase, model)
}

// AuthHeaders returns the required OpenAI Realtime API headers
func (p *OpenAILiveProvider) AuthHeaders() (http.Header, error) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+p.apiKey)
	headers.Set("OpenAI-Beta", "realtime=v1")
	return headers, nil
}

// SetupMessage returns nil — OpenAI uses URL params for model selection, no setup frame needed
func (p *OpenAILiveProvider) SetupMessage(model string) []byte {
	return nil
}
