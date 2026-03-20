// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexLiveProvider implements LiveProvider for Google Vertex AI Live API
type VertexLiveProvider struct {
	projectID   string
	region      string
	liveConfig  config.LiveConfig
	tokenSource oauth2.TokenSource
	mu          sync.Mutex
}

// NewVertexLiveProvider creates a Vertex AI Live provider from config
func NewVertexLiveProvider(credentialsFile, projectID, region string, liveCfg config.LiveConfig) (*VertexLiveProvider, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("vertex live: credentials_file is required")
	}
	if projectID == "" {
		return nil, fmt.Errorf("vertex live: project_id is required")
	}
	if region == "" || region == "global" {
		region = "us-central1" // Live API requires a regional endpoint
	}

	credBytes, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("vertex live: failed to read credentials file %s: %w", credentialsFile, err)
	}

	creds, err := google.CredentialsFromJSON(context.Background(), credBytes,
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, fmt.Errorf("vertex live: failed to parse credentials: %w", err)
	}

	logger.InfoCF("live", "Vertex AI Live provider initialized", map[string]interface{}{
		"project_id": projectID,
		"region":     region,
	})

	return &VertexLiveProvider{
		projectID:   projectID,
		region:      region,
		liveConfig:  liveCfg,
		tokenSource: creds.TokenSource,
	}, nil
}

// Name returns the provider name
func (p *VertexLiveProvider) Name() string {
	return "vertex"
}

// BuildUpstreamURL constructs the Vertex AI Live API WebSocket endpoint
//
// Format: wss://{region}-aiplatform.googleapis.com/ws/google.cloud.aiplatform.v1beta1.LlmBidiService/BidiGenerateContent
func (p *VertexLiveProvider) BuildUpstreamURL(model string) string {
	host := fmt.Sprintf("%s-aiplatform.googleapis.com", p.region)

	return fmt.Sprintf(
		"wss://%s/ws/google.cloud.aiplatform.v1beta1.LlmBidiService/BidiGenerateContent",
		host,
	)
}

// SetupMessage returns the BidiGenerateContentSetup message.
// Uses generation_config and realtime_input_config from config.json if set, otherwise uses defaults.
func (p *VertexLiveProvider) SetupMessage(model string) []byte {
	modelResource := fmt.Sprintf(
		"projects/%s/locations/%s/publishers/google/models/%s",
		p.projectID, p.region, model,
	)

	setupInner := map[string]interface{}{
		"model": modelResource,
	}

	// Use config-provided generationConfig, or defaults
	if p.liveConfig.GenerationConfig != nil {
		setupInner["generationConfig"] = withVideoResponseModalities(p.liveConfig.GenerationConfig, p.liveConfig.Video)
	} else {
		responseModalities := []string{"AUDIO"}
		if p.liveConfig.Video {
			responseModalities = []string{"AUDIO", "VIDEO"}
		}

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

		setupInner["generationConfig"] = map[string]interface{}{
			"responseModalities": responseModalities,
			"speechConfig":       speechConfig,
		}
	}

	// Use config-provided realtimeInputConfig, or defaults
	if p.liveConfig.RealtimeInputConfig != nil {
		setupInner["realtimeInputConfig"] = p.liveConfig.RealtimeInputConfig
	} else {
		setupInner["realtimeInputConfig"] = map[string]interface{}{
			"automaticActivityDetection": map[string]interface{}{
				"disabled":                 false,
				"startOfSpeechSensitivity": "START_SENSITIVITY_LOW",
				"endOfSpeechSensitivity":   "END_SENSITIVITY_LOW",
			},
		}
	}

	setup := map[string]interface{}{
		"setup": setupInner,
	}

	data, _ := json.Marshal(setup)
	return data
}

// AuthHeaders returns OAuth2 Bearer token headers for the WebSocket upgrade request
func (p *VertexLiveProvider) AuthHeaders() (http.Header, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	token, err := p.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+token.AccessToken)

	return headers, nil
}
