// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/tasks"
)

// The completion hook: how work finished somewhere else gets back into the
// conversation that asked for it.
//
// Authentication is the per-task token, not a global key. A worker is told one
// task's id and one task's token, and that is all it can ever report on — a
// leaked token finishes one job and nothing else.

type taskResultRequest struct {
	Status string `json:"status"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// handleTaskRoutes serves /v1/tasks/{id} and /v1/tasks/{id}/result.
func (gs *GatewayServer) handleTaskRoutes(w http.ResponseWriter, r *http.Request) {
	manager := gs.agentManager.Tasks()
	if manager == nil {
		http.Error(w, `{"error":"tasks are disabled"}`, http.StatusNotFound)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	id, rest, _ := strings.Cut(path, "/")
	if id == "" {
		http.Error(w, `{"error":"task id missing"}`, http.StatusBadRequest)
		return
	}

	switch {
	case rest == "result" && r.Method == http.MethodPost:
		gs.completeTask(w, r, manager, id)
	case rest == "" && r.Method == http.MethodGet:
		task, ok := manager.Get(id)
		if !ok {
			http.Error(w, `{"error":"no such task"}`, http.StatusNotFound)
			return
		}
		writeTask(w, task)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (gs *GatewayServer) completeTask(w http.ResponseWriter, r *http.Request, manager *tasks.Manager, id string) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		token = r.Header.Get("X-Task-Token")
	}

	var req taskResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"body must be JSON"}`, http.StatusBadRequest)
		return
	}

	status := req.Status
	if status == "" {
		status = tasks.StatusDone
	}
	result := req.Result
	if req.Error != "" {
		status = tasks.StatusFailed
		if result == "" {
			result = req.Error
		}
	}

	if err := manager.Complete(id, token, status, result); err != nil {
		// A wrong token and an unknown id are the same answer on purpose: this
		// endpoint is reachable by whoever can route to the gateway, and it
		// should not confirm which task ids exist.
		logger.WarnCF("tasks", "Rejected a task callback", map[string]interface{}{
			"task": id, "error": err.Error(), "peer": r.RemoteAddr,
		})
		http.Error(w, `{"error":"unknown task or token"}`, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// handleListTasks answers GET /v1/tasks.
func (gs *GatewayServer) handleListTasks(w http.ResponseWriter, r *http.Request) {
	manager := gs.agentManager.Tasks()
	if manager == nil {
		http.Error(w, `{"error":"tasks are disabled"}`, http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	list := manager.List()
	out := make([]map[string]interface{}, 0, len(list))
	for _, t := range list {
		out = append(out, publicTask(t))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": out})
}

func writeTask(w http.ResponseWriter, t *tasks.Task) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(publicTask(t))
}

// publicTask is the task without its callback token. The token is a credential
// for one job; it goes out with the dispatch and never comes back in a listing.
func publicTask(t *tasks.Task) map[string]interface{} {
	return map[string]interface{}{
		"id":          t.ID,
		"label":       t.Label,
		"status":      t.Status,
		"executor":    t.Executor,
		"created_ms":  t.CreatedMS,
		"finished_ms": t.FinishedMS,
		"result":      t.Result,
		"error":       t.Error,
	}
}
