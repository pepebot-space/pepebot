package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/task"
)

// Task API request types

type CreateTaskRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status,omitempty"`
	Priority    string            `json:"priority,omitempty"`
	Agent       string            `json:"assigned_agent,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Approval    bool              `json:"approval,omitempty"`
}

type MoveTaskRequest struct {
	Status string `json:"status"`
}

type CheckoutTaskRequest struct {
	Agent       string   `json:"agent"`
	AgentLabels []string `json:"agent_labels,omitempty"`
}

type CompleteTaskRequest struct {
	Result string `json:"result,omitempty"`
}

type FailTaskRequest struct {
	Error string `json:"error,omitempty"`
}

type ApproveTaskRequest struct {
	ApprovedBy string `json:"approved_by"`
	Note       string `json:"note,omitempty"`
}

type RejectTaskRequest struct {
	RejectedBy string `json:"rejected_by"`
	Note       string `json:"note,omitempty"`
}

// requireTaskStore checks if orchestration is enabled, returns false and writes error if not.
func (gs *GatewayServer) requireTaskStore(w http.ResponseWriter) bool {
	if gs.taskStore == nil {
		writeError(w, http.StatusNotFound, "orchestration not enabled", "not_found")
		return false
	}
	return true
}

// handleTaskRoutes routes /v1/tasks and /v1/tasks/{id}[/action]
func (gs *GatewayServer) handleTaskRoutes(w http.ResponseWriter, r *http.Request) {
	if !gs.requireTaskStore(w) {
		return
	}

	// Parse path: /v1/tasks, /v1/tasks/stats, /v1/tasks/{id}, /v1/tasks/{id}/action
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks")
	path = strings.TrimPrefix(path, "/")

	// /v1/tasks or /v1/tasks/
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			gs.handleListTasks(w, r)
		case http.MethodPost:
			gs.handleCreateTask(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		}
		return
	}

	// /v1/tasks/stats
	if path == "stats" && r.Method == http.MethodGet {
		gs.handleTaskStats(w, r)
		return
	}

	// Split into {id} and optional {action}
	parts := strings.SplitN(path, "/", 2)
	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if action == "" {
		// /v1/tasks/{id}
		switch r.Method {
		case http.MethodGet:
			gs.handleGetTask(w, r, taskID)
		case http.MethodPut:
			gs.handleUpdateTask(w, r, taskID)
		case http.MethodDelete:
			gs.handleDeleteTask(w, r, taskID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		}
		return
	}

	// /v1/tasks/{id}/{action} — all POST
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		return
	}

	switch action {
	case "move":
		gs.handleMoveTask(w, r, taskID)
	case "checkout":
		gs.handleCheckoutTask(w, r, taskID)
	case "complete":
		gs.handleCompleteTask(w, r, taskID)
	case "fail":
		gs.handleFailTask(w, r, taskID)
	case "approve":
		gs.handleApproveTask(w, r, taskID)
	case "reject":
		gs.handleRejectTask(w, r, taskID)
	default:
		writeError(w, http.StatusNotFound, "unknown action: "+action, "not_found")
	}
}

func (gs *GatewayServer) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := task.TaskFilter{}

	if s := q.Get("status"); s != "" {
		status := task.TaskStatus(s)
		filter.Status = &status
	}
	if a := q.Get("agent"); a != "" {
		filter.AssignedAgent = &a
	}
	if p := q.Get("priority"); p != "" {
		prio := task.TaskPriority(p)
		filter.Priority = &prio
	}
	if pid := q.Get("parent_id"); pid != "" {
		filter.ParentID = &pid
	}
	if labels := q.Get("labels"); labels != "" {
		filter.Labels = strings.Split(labels, ",")
	}

	tasks, err := gs.taskStore.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	if tasks == nil {
		tasks = []*task.Task{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (gs *GatewayServer) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required", "invalid_request_error")
		return
	}

	t := &task.Task{
		Title:       req.Title,
		Description: req.Description,
		Priority:    task.TaskPriority(req.Priority),
		Labels:      req.Labels,
		Metadata:    req.Metadata,
		ParentID:    req.ParentID,
		Approval:    req.Approval,
		CreatedBy:   req.CreatedBy,
	}

	if req.Status != "" {
		t.Status = task.TaskStatus(req.Status)
	}
	if req.Agent != "" {
		t.AssignedAgent = req.Agent
	}

	if err := gs.taskStore.Create(t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	gs.emitTaskEvent("task.created", t)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleGetTask(w http.ResponseWriter, r *http.Request, id string) {
	t, err := gs.taskStore.Get(id)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleUpdateTask(w http.ResponseWriter, r *http.Request, id string) {
	existing, err := gs.taskStore.Get(id)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	// Decode partial update
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if v, ok := updates["title"].(string); ok {
		existing.Title = v
	}
	if v, ok := updates["description"].(string); ok {
		existing.Description = v
	}
	if v, ok := updates["priority"].(string); ok {
		existing.Priority = task.TaskPriority(v)
	}
	if v, ok := updates["assigned_agent"].(string); ok {
		existing.AssignedAgent = v
	}
	if v, ok := updates["result"].(string); ok {
		existing.Result = v
	}
	if v, ok := updates["error"].(string); ok {
		existing.Error = v
	}
	if v, ok := updates["approval"].(bool); ok {
		existing.Approval = v
	}
	if v, ok := updates["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(v))
		for _, l := range v {
			if s, ok := l.(string); ok {
				labels = append(labels, s)
			}
		}
		existing.Labels = labels
	}

	if err := gs.taskStore.Update(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (gs *GatewayServer) handleDeleteTask(w http.ResponseWriter, r *http.Request, id string) {
	err := gs.taskStore.Delete(id)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "task deleted",
	})
}

func (gs *GatewayServer) handleMoveTask(w http.ResponseWriter, r *http.Request, id string) {
	var req MoveTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required", "invalid_request_error")
		return
	}

	err := gs.taskStore.Move(id, task.TaskStatus(req.Status))
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err == task.ErrInvalidTransition {
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	if err == task.ErrApprovalRequired {
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	t, _ := gs.taskStore.Get(id)
	gs.emitTaskEvent("task.moved", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleCheckoutTask(w http.ResponseWriter, r *http.Request, id string) {
	var req CheckoutTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if req.Agent == "" {
		writeError(w, http.StatusBadRequest, "agent is required", "invalid_request_error")
		return
	}

	t, err := gs.taskStore.Checkout(id, req.Agent, req.AgentLabels)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found or not available", "not_found")
		return
	}
	if err == task.ErrTaskAlreadyClaimed {
		writeError(w, http.StatusConflict, err.Error(), "conflict")
		return
	}
	if err == task.ErrLabelMismatch {
		writeError(w, http.StatusForbidden, err.Error(), "forbidden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	gs.emitTaskEvent("task.checkout", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleCompleteTask(w http.ResponseWriter, r *http.Request, id string) {
	var req CompleteTaskRequest
	json.NewDecoder(r.Body).Decode(&req)

	// Move to done
	if err := gs.taskStore.Move(id, task.TaskStatusDone); err != nil {
		if err == task.ErrTaskNotFound {
			writeError(w, http.StatusNotFound, "task not found", "not_found")
			return
		}
		if err == task.ErrApprovalRequired {
			writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	// Set result if provided
	if req.Result != "" {
		t, _ := gs.taskStore.Get(id)
		if t != nil {
			t.Result = req.Result
			gs.taskStore.Update(t)
		}
	}

	t, _ := gs.taskStore.Get(id)
	gs.emitTaskEvent("task.completed", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleFailTask(w http.ResponseWriter, r *http.Request, id string) {
	var req FailTaskRequest
	json.NewDecoder(r.Body).Decode(&req)

	if err := gs.taskStore.Move(id, task.TaskStatusFailed); err != nil {
		if err == task.ErrTaskNotFound {
			writeError(w, http.StatusNotFound, "task not found", "not_found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	if req.Error != "" {
		t, _ := gs.taskStore.Get(id)
		if t != nil {
			t.Error = req.Error
			gs.taskStore.Update(t)
		}
	}

	t, _ := gs.taskStore.Get(id)
	gs.emitTaskEvent("task.failed", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleApproveTask(w http.ResponseWriter, r *http.Request, id string) {
	var req ApproveTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if req.ApprovedBy == "" {
		req.ApprovedBy = "dashboard"
	}

	err := gs.taskStore.Approve(id, req.ApprovedBy, req.Note)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	t, _ := gs.taskStore.Get(id)
	gs.emitTaskEvent("task.approved", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleRejectTask(w http.ResponseWriter, r *http.Request, id string) {
	var req RejectTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if req.RejectedBy == "" {
		req.RejectedBy = "dashboard"
	}

	err := gs.taskStore.Reject(id, req.RejectedBy, req.Note)
	if err == task.ErrTaskNotFound {
		writeError(w, http.StatusNotFound, "task not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	t, _ := gs.taskStore.Get(id)
	gs.emitTaskEvent("task.rejected", t)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (gs *GatewayServer) handleTaskStats(w http.ResponseWriter, r *http.Request) {
	stats, err := gs.taskStore.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	// Ensure all statuses are represented
	response := map[string]interface{}{
		"backlog":     stats[task.TaskStatusBacklog],
		"todo":        stats[task.TaskStatusTodo],
		"in_progress": stats[task.TaskStatusInProgress],
		"review":      stats[task.TaskStatusReview],
		"approved":    stats[task.TaskStatusApproved],
		"rejected":    stats[task.TaskStatusRejected],
		"done":        stats[task.TaskStatusDone],
		"failed":      stats[task.TaskStatusFailed],
	}

	total := 0
	for _, v := range stats {
		total += v
	}
	response["total"] = total

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

