// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexLiveProvider implements LiveProvider for Google Vertex AI Live API
type VertexLiveProvider struct {
	projectID   string
	region      string
	tokenSource oauth2.TokenSource
	mu          sync.Mutex
}

// NewVertexLiveProvider creates a Vertex AI Live provider from config
func NewVertexLiveProvider(credentialsFile, projectID, region string) (*VertexLiveProvider, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("vertex live: credentials_file is required")
	}
	if projectID == "" {
		return nil, fmt.Errorf("vertex live: project_id is required")
	}
	if region == "" {
		region = "us-central1"
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
// Special case: "global" region uses aiplatform.googleapis.com without region prefix
func (p *VertexLiveProvider) BuildUpstreamURL(model string) string {
	var host string
	if p.region == "global" {
		host = "aiplatform.googleapis.com"
	} else {
		host = fmt.Sprintf("%s-aiplatform.googleapis.com", p.region)
	}

	// The model is passed as a query parameter in the URL
	return fmt.Sprintf(
		"wss://%s/ws/google.cloud.aiplatform.v1beta1.LlmBidiService/BidiGenerateContent?key=%s",
		host, "",
	)
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
