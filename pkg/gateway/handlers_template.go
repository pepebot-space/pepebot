package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/task"
)

// handleTemplateRoutes routes /v1/task-templates and /v1/task-templates/{name}[/run]
func (gs *GatewayServer) handleTemplateRoutes(w http.ResponseWriter, r *http.Request) {
	if !gs.requireTaskStore(w) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/task-templates")
	path = strings.TrimPrefix(path, "/")

	// /v1/task-templates
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			gs.handleListTemplates(w, r)
		case http.MethodPost:
			gs.handleSaveTemplate(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		}
		return
	}

	// Split {name} and optional {action}
	parts := strings.SplitN(path, "/", 2)
	name := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if action == "run" && r.Method == http.MethodPost {
		gs.handleRunTemplate(w, r, name)
		return
	}

	if action == "" {
		switch r.Method {
		case http.MethodGet:
			gs.handleGetTemplate(w, r, name)
		case http.MethodDelete:
			gs.handleDeleteTemplate(w, r, name)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
		}
		return
	}

	writeError(w, http.StatusNotFound, "unknown action: "+action, "not_found")
}

func (gs *GatewayServer) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	loader := task.NewTemplateLoader(gs.config.WorkspacePath())
	names := loader.List()

	templates := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		tmpl, err := loader.Load(name)
		if err != nil {
			continue
		}
		templates = append(templates, map[string]interface{}{
			"name":        name,
			"description": tmpl.Description,
			"priority":    tmpl.Priority,
			"agent":       tmpl.Agent,
			"labels":      tmpl.Labels,
			"approval":    tmpl.Approval,
			"sub_tasks":   len(tmpl.SubTasks),
			"variables":   tmpl.Variables,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (gs *GatewayServer) handleGetTemplate(w http.ResponseWriter, r *http.Request, name string) {
	loader := task.NewTemplateLoader(gs.config.WorkspacePath())
	tmpl, err := loader.Load(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "not_found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tmpl)
}

func (gs *GatewayServer) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var tmpl task.TaskTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if tmpl.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "invalid_request_error")
		return
	}

	loader := task.NewTemplateLoader(gs.config.WorkspacePath())
	if err := loader.Save(tmpl.Name, &tmpl); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "template saved",
		"name":    tmpl.Name,
	})
}

func (gs *GatewayServer) handleDeleteTemplate(w http.ResponseWriter, r *http.Request, name string) {
	loader := task.NewTemplateLoader(gs.config.WorkspacePath())
	if err := loader.Delete(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "not_found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "template deleted",
	})
}

type RunTemplateRequest struct {
	Variables map[string]string `json:"variables,omitempty"`
	CreatedBy string            `json:"created_by,omitempty"`
}

func (gs *GatewayServer) handleRunTemplate(w http.ResponseWriter, r *http.Request, name string) {
	loader := task.NewTemplateLoader(gs.config.WorkspacePath())
	tmpl, err := loader.Load(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "not_found")
		return
	}

	var req RunTemplateRequest
	json.NewDecoder(r.Body).Decode(&req)

	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = "dashboard"
	}

	tasks, err := task.CreateTasksFromTemplate(gs.taskStore, tmpl, req.Variables, createdBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"created":  len(tasks),
		"task_ids": ids,
	})
}
