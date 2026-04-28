package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/task"
)

// ManageTaskTool allows agents to create and manage tasks via tool calls.
type ManageTaskTool struct {
	store     task.TaskStore
	workspace string
}

func NewManageTaskTool(store task.TaskStore, workspace string) *ManageTaskTool {
	return &ManageTaskTool{store: store, workspace: workspace}
}

func (t *ManageTaskTool) Name() string {
	return "manage_task"
}

func (t *ManageTaskTool) Description() string {
	return "Manage orchestration tasks: create, list, get, checkout, complete, fail, move. Tasks appear on the Kanban board and can be assigned to agents."
}

func (t *ManageTaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"create", "list", "get", "checkout", "complete", "fail", "move", "stats", "create_from_template"},
				"description": "Action to perform",
			},
			"template": map[string]interface{}{
				"type":        "string",
				"description": "Template name (for create_from_template action)",
			},
			"variables": map[string]interface{}{
				"type":        "object",
				"description": "Template variables (for create_from_template action)",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Task ID (required for get, complete, fail, move; optional for checkout — empty means auto-assign next)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Task title (required for create)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Task description (optional for create)",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"backlog", "todo", "in_progress", "review", "done", "failed"},
				"description": "Task status (for create initial status, or move target status)",
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"low", "medium", "high", "critical"},
				"description": "Task priority (for create, default: medium)",
			},
			"agent": map[string]interface{}{
				"type":        "string",
				"description": "Agent name (for checkout, or assign on create)",
			},
			"labels": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Task labels (for create or list filter)",
			},
			"parent_id": map[string]interface{}{
				"type":        "string",
				"description": "Parent task ID (for creating sub-tasks)",
			},
			"approval": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether task requires explicit approval before completion",
			},
			"result": map[string]interface{}{
				"type":        "string",
				"description": "Result text (for complete action)",
			},
			"error": map[string]interface{}{
				"type":        "string",
				"description": "Error text (for fail action)",
			},
			"filter_status": map[string]interface{}{
				"type":        "string",
				"description": "Filter by status (for list action)",
			},
			"filter_agent": map[string]interface{}{
				"type":        "string",
				"description": "Filter by assigned agent (for list action)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ManageTaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)

	switch action {
	case "create":
		return t.create(ctx, args)
	case "list":
		return t.list(ctx, args)
	case "get":
		return t.get(ctx, args)
	case "checkout":
		return t.checkout(ctx, args)
	case "complete":
		return t.complete(ctx, args)
	case "fail":
		return t.fail(ctx, args)
	case "move":
		return t.move(ctx, args)
	case "stats":
		return t.stats(ctx, args)
	case "create_from_template":
		return t.createFromTemplate(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (t *ManageTaskTool) create(_ context.Context, args map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required for create")
	}

	tk := &task.Task{
		Title:     title,
		CreatedBy: "agent",
	}

	if v, ok := args["description"].(string); ok {
		tk.Description = v
	}
	if v, ok := args["status"].(string); ok {
		tk.Status = task.TaskStatus(v)
	}
	if v, ok := args["priority"].(string); ok {
		tk.Priority = task.TaskPriority(v)
	}
	if v, ok := args["agent"].(string); ok {
		tk.AssignedAgent = v
	}
	if v, ok := args["parent_id"].(string); ok {
		tk.ParentID = v
	}
	if v, ok := args["approval"].(bool); ok {
		tk.Approval = v
	}
	if v, ok := args["labels"].([]interface{}); ok {
		for _, l := range v {
			if s, ok := l.(string); ok {
				tk.Labels = append(tk.Labels, s)
			}
		}
	}

	if err := t.store.Create(tk); err != nil {
		return "", err
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"task_id": tk.ID,
		"title":   tk.Title,
		"status":  tk.Status,
		"message": fmt.Sprintf("Task '%s' created", tk.Title),
	})
}

func (t *ManageTaskTool) list(_ context.Context, args map[string]interface{}) (string, error) {
	filter := task.TaskFilter{Limit: 20}

	if v, ok := args["filter_status"].(string); ok && v != "" {
		s := task.TaskStatus(v)
		filter.Status = &s
	}
	if v, ok := args["filter_agent"].(string); ok && v != "" {
		filter.AssignedAgent = &v
	}
	if v, ok := args["labels"].([]interface{}); ok {
		for _, l := range v {
			if s, ok := l.(string); ok {
				filter.Labels = append(filter.Labels, s)
			}
		}
	}

	tasks, err := t.store.List(filter)
	if err != nil {
		return "", err
	}

	// Compact summary for agent consumption
	items := make([]map[string]interface{}, 0, len(tasks))
	for _, tk := range tasks {
		item := map[string]interface{}{
			"id":       tk.ID,
			"title":    tk.Title,
			"status":   tk.Status,
			"priority": tk.Priority,
		}
		if tk.AssignedAgent != "" {
			item["agent"] = tk.AssignedAgent
		}
		if len(tk.Labels) > 0 {
			item["labels"] = tk.Labels
		}
		items = append(items, item)
	}

	return jsonResult(map[string]interface{}{
		"tasks": items,
		"total": len(items),
	})
}

func (t *ManageTaskTool) get(_ context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for get")
	}

	tk, err := t.store.Get(id)
	if err != nil {
		return "", err
	}

	data, _ := json.Marshal(tk)
	return string(data), nil
}

func (t *ManageTaskTool) checkout(ctx context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	agent, _ := args["agent"].(string)
	if agent == "" {
		agent = "default"
	}

	var agentLabels []string
	if v, ok := args["labels"].([]interface{}); ok {
		for _, l := range v {
			if s, ok := l.(string); ok {
				agentLabels = append(agentLabels, s)
			}
		}
	}

	tk, err := t.store.Checkout(id, agent, agentLabels)
	if err != nil {
		return "", err
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"task_id": tk.ID,
		"title":   tk.Title,
		"agent":   tk.AssignedAgent,
		"message": fmt.Sprintf("Task '%s' checked out by %s", tk.Title, agent),
	})
}

func (t *ManageTaskTool) complete(_ context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for complete")
	}

	result, _ := args["result"].(string)

	// Try moving to done first
	err := t.store.Move(id, task.TaskStatusDone)
	if err != nil {
		// If approval required, move to review instead
		if err == task.ErrApprovalRequired || strings.Contains(err.Error(), "approval") {
			moveErr := t.store.Move(id, task.TaskStatusReview)
			if moveErr != nil {
				return "", moveErr
			}
			return jsonResult(map[string]interface{}{
				"success": true,
				"task_id": id,
				"status":  "review",
				"message": "Task moved to review (requires approval)",
			})
		}
		return "", err
	}

	if result != "" {
		tk, _ := t.store.Get(id)
		if tk != nil {
			tk.Result = result
			t.store.Update(tk)
		}
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"task_id": id,
		"status":  "done",
		"message": "Task completed",
	})
}

func (t *ManageTaskTool) fail(_ context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for fail")
	}

	errMsg, _ := args["error"].(string)

	if err := t.store.Move(id, task.TaskStatusFailed); err != nil {
		return "", err
	}

	if errMsg != "" {
		tk, _ := t.store.Get(id)
		if tk != nil {
			tk.Error = errMsg
			t.store.Update(tk)
		}
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"task_id": id,
		"status":  "failed",
		"message": "Task marked as failed",
	})
}

func (t *ManageTaskTool) move(_ context.Context, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for move")
	}

	status, _ := args["status"].(string)
	if status == "" {
		return "", fmt.Errorf("status is required for move")
	}

	if err := t.store.Move(id, task.TaskStatus(status)); err != nil {
		return "", err
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"task_id": id,
		"status":  status,
		"message": fmt.Sprintf("Task moved to %s", status),
	})
}

func (t *ManageTaskTool) stats(_ context.Context, _ map[string]interface{}) (string, error) {
	stats, err := t.store.Stats()
	if err != nil {
		return "", err
	}

	result := make(map[string]interface{})
	total := 0
	for status, count := range stats {
		result[string(status)] = count
		total += count
	}
	result["total"] = total

	return jsonResult(result)
}

func (t *ManageTaskTool) createFromTemplate(_ context.Context, args map[string]interface{}) (string, error) {
	tmplName, _ := args["template"].(string)
	if tmplName == "" {
		return "", fmt.Errorf("template name is required for create_from_template")
	}

	loader := task.NewTemplateLoader(t.workspace)
	tmpl, err := loader.Load(tmplName)
	if err != nil {
		return "", err
	}

	// Parse variables
	vars := make(map[string]string)
	if v, ok := args["variables"].(map[string]interface{}); ok {
		for key, val := range v {
			if s, ok := val.(string); ok {
				vars[key] = s
			}
		}
	}

	tasks, err := task.CreateTasksFromTemplate(t.store, tmpl, vars, "agent")
	if err != nil {
		return "", err
	}

	ids := make([]string, 0, len(tasks))
	for _, tk := range tasks {
		ids = append(ids, tk.ID)
	}

	return jsonResult(map[string]interface{}{
		"success":  true,
		"template": tmplName,
		"created":  len(tasks),
		"task_ids": ids,
		"message":  fmt.Sprintf("Created %d tasks from template '%s'", len(tasks), tmplName),
	})
}

func jsonResult(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
