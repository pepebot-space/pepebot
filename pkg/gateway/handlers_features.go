package gateway

import (
	"encoding/json"
	"net/http"
)

type FeaturesResponse struct {
	Orchestration FeatureStatus `json:"orchestration"`
	Live          FeatureStatus `json:"live"`
}

type FeatureStatus struct {
	Enabled bool   `json:"enabled"`
	Backend string `json:"backend,omitempty"`
}

func (gs *GatewayServer) handleFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	resp := FeaturesResponse{
		Orchestration: FeatureStatus{
			Enabled: gs.taskStore != nil,
		},
		Live: FeatureStatus{
			Enabled: gs.liveServer != nil,
		},
	}

	if gs.taskStore != nil {
		resp.Orchestration.Backend = gs.config.Orchestration.Backend
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
