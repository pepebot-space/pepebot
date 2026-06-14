// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// GeminiLiveProvider implements LiveProvider for Google AI Studio Gemini Live API
type GeminiLiveProvider struct {
	apiKey     string
	liveConfig config.LiveConfig
}

// NewGeminiLiveProvider creates a Google AI Studio Live API provider
func NewGeminiLiveProvider(apiKey string, liveCfg config.LiveConfig) (*GeminiLiveProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini live: api_key is required")
	}

	logger.InfoC("live", "Google AI Studio Gemini Live provider initialized")

	return &GeminiLiveProvider{
		apiKey:     apiKey,
		liveConfig: liveCfg,
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
	return headers, nil
}

// SetupMessage returns a BidiGenerateContentSetup message for Gemini Live API
func (p *GeminiLiveProvider) SetupMessage(model string) []byte {
	setupInner := map[string]interface{}{
		"model": "models/" + model,
	}

	// Build generationConfig into a fresh map so injecting mediaResolution never
	// mutates the shared config across sessions.
	genCfg := map[string]interface{}{}
	if p.liveConfig.GenerationConfig != nil {
		for k, v := range p.liveConfig.GenerationConfig {
			genCfg[k] = v
		}
	} else {
		speechConfig := map[string]interface{}{
			"voiceConfig": map[string]interface{}{
				"prebuiltVoiceConfig": map[string]interface{}{
					"voiceName": "Aoede",
				},
			},
		}
		if p.liveConfig.Language != "" {
			speechConfig["languageCode"] = p.liveConfig.Language
		}
		genCfg["responseModalities"] = []string{"AUDIO"}
		genCfg["speechConfig"] = speechConfig
	}
	// Inject media resolution (lower = fewer tokens/frame, lower latency) unless
	// the config already pins it explicitly.
	if mr := p.liveConfig.MediaResolution; mr != "" {
		if _, ok := genCfg["mediaResolution"]; !ok {
			genCfg["mediaResolution"] = mr
		}
	}
	setupInner["generationConfig"] = genCfg

	if p.liveConfig.RealtimeInputConfig != nil {
		setupInner["realtimeInputConfig"] = p.liveConfig.RealtimeInputConfig
	} else {
		setupInner["realtimeInputConfig"] = map[string]interface{}{
			"automaticActivityDetection": map[string]interface{}{
				"disabled":                 false,
				"startOfSpeechSensitivity": "START_SENSITIVITY_LOW",
				"endOfSpeechSensitivity":   "END_SENSITIVITY_HIGH",
				"silenceDurationMs":        500,
			},
		}
	}

	// Set the system instruction (role/persona/task) when configured.
	if prompt := p.liveConfig.SystemPrompt; prompt != "" {
		setupInner["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": prompt}},
		}
	}

	setup := map[string]interface{}{
		"setup": setupInner,
	}
	data, _ := json.Marshal(setup)
	return data
}
