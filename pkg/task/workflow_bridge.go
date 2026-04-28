package task

import (
	"fmt"
	"time"
)

// WorkflowBridge implements workflow.WorkflowTaskExecutor using a TaskStore.
type WorkflowBridge struct {
	store TaskStore
}

// NewWorkflowBridge creates a bridge between workflows and the task store.
func NewWorkflowBridge(store TaskStore) *WorkflowBridge {
	return &WorkflowBridge{store: store}
}

// CreateTask creates a task and returns its ID.
func (b *WorkflowBridge) CreateTask(title, description, agent, priority string, labels []string, approval bool) (string, error) {
	t := &Task{
		Title:       title,
		Description: description,
		Status:      TaskStatusTodo,
		Priority:    TaskPriority(priority),
		Labels:      labels,
		Approval:    approval,
		CreatedBy:   "workflow",
	}
	if t.Priority == "" {
		t.Priority = TaskPriorityMedium
	}
	if agent != "" {
		t.AssignedAgent = agent
	}

	if err := b.store.Create(t); err != nil {
		return "", err
	}
	return t.ID, nil
}

// WaitForTask polls the task store until the task reaches a terminal state or timeout.
// Returns (status, result, error).
func (b *WorkflowBridge) WaitForTask(id string, timeoutSeconds int) (string, string, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		t, err := b.store.Get(id)
		if err != nil {
			return "", "", fmt.Errorf("task not found: %w", err)
		}

		switch t.Status {
		case TaskStatusDone:
			return string(t.Status), t.Result, nil
		case TaskStatusFailed:
			return string(t.Status), t.Error, fmt.Errorf("task failed: %s", t.Error)
		case TaskStatusRejected:
			return string(t.Status), t.ApprovalNote, fmt.Errorf("task rejected: %s", t.ApprovalNote)
		}

		time.Sleep(pollInterval)
	}

	return "timeout", "", fmt.Errorf("task %s did not complete within %d seconds", id, timeoutSeconds)
}
