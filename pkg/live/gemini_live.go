// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"fmt"
	"net/http"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

// GeminiLiveProvider implements LiveProvider for Google AI Studio Gemini Live API
type GeminiLiveProvider struct {
	apiKey string
}

// NewGeminiLiveProvider creates a Google AI Studio Live API provider
func NewGeminiLiveProvider(apiKey string) (*GeminiLiveProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini live: api_key is required")
	}

	logger.InfoC("live", "Google AI Studio Gemini Live provider initialized")

	return &GeminiLiveProvider{
		apiKey: apiKey,
	}, nil
}

// Name returns the provider name
func (p *GeminiLiveProvider) Name() string {
	return "gemini"
}

// BuildUpstreamURL constructs the Gemini Live API WebSocket endpoint
// Format: wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent?key={api_key}
func (p *GeminiLiveProvider) BuildUpstreamURL(model string) string {
	return fmt.Sprintf(
		"wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent?key=%s",
		p.apiKey,
	)
}

// AuthHeaders returns HTTP headers (none needed for Gemini, API key is in query string)
func (p *GeminiLiveProvider) AuthHeaders() (http.Header, error) {
	headers := http.Header{}
	// Empty headers, auth is handled via the query parameter in BuildUpstreamURL
	return headers, nil
}
