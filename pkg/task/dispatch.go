package task

import (
	"context"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

// AgentInfo holds the info needed for task dispatch.
type AgentInfo struct {
	Name       string
	Enabled    bool
	TaskLabels []string
}

// AgentProvider returns available agents for dispatch.
type AgentProvider interface {
	ListEnabledAgentsForDispatch() []AgentInfo
}

// TaskProcessor processes a task message with a specific agent.
type TaskProcessor interface {
	ProcessDirect(ctx context.Context, content string, media []string, sessionKey, agentName string) (string, error)
}

// Dispatcher assigns unassigned "todo" tasks to available agents and optionally
// processes them. It is designed to be called periodically from the heartbeat.
type Dispatcher struct {
	store     TaskStore
	agents    AgentProvider
	processor TaskProcessor
	notifier  *Notifier
}

// NewDispatcher creates a new task dispatcher.
func NewDispatcher(store TaskStore, agents AgentProvider, processor TaskProcessor) *Dispatcher {
	return &Dispatcher{
		store:     store,
		agents:    agents,
		processor: processor,
	}
}

// SetNotifier sets the notification handler for task events.
func (d *Dispatcher) SetNotifier(n *Notifier) {
	d.notifier = n
}

// Dispatch assigns unassigned todo tasks to available agents.
// It tries to match agent task_labels with task labels.
// Returns the number of tasks dispatched.
func (d *Dispatcher) Dispatch(ctx context.Context) int {
	if d.store == nil {
		return 0
	}

	status := TaskStatusTodo
	todoTasks, err := d.store.List(TaskFilter{Status: &status, Limit: 50})
	if err != nil {
		logger.WarnCF("task", "Dispatch: failed to list todo tasks", map[string]interface{}{
			"error": err.Error(),
		})
		return 0
	}

	agents := d.agents.ListEnabledAgentsForDispatch()
	if len(agents) == 0 {
		return 0
	}

	dispatched := 0
	for _, t := range todoTasks {
		if t.AssignedAgent != "" {
			continue // already assigned, just not started
		}

		// Find a matching agent
		agent := findMatchingAgent(agents, t.Labels)
		if agent == nil {
			continue
		}

		// Checkout
		checkedOut, err := d.store.Checkout(t.ID, agent.Name, agent.TaskLabels)
		if err != nil {
			continue // someone else claimed it, or label mismatch
		}

		dispatched++

		logger.InfoCF("task", "Dispatched task to agent", map[string]interface{}{
			"task_id": checkedOut.ID,
			"title":   checkedOut.Title,
			"agent":   agent.Name,
		})

		// If processor is available, execute the task
		if d.processor != nil {
			go d.executeTask(ctx, checkedOut, agent.Name)
		}
	}

	return dispatched
}

func (d *Dispatcher) executeTask(ctx context.Context, t *Task, agentName string) {
	prompt := t.Title
	if t.Description != "" {
		prompt += "\n\n" + t.Description
	}

	sessionKey := "task:" + t.ID

	response, err := d.processor.ProcessDirect(ctx, prompt, nil, sessionKey, agentName)
	if err != nil {
		// Mark as failed
		d.store.Move(t.ID, TaskStatusFailed)
		updated, _ := d.store.Get(t.ID)
		if updated != nil {
			updated.Error = err.Error()
			d.store.Update(updated)
		}
		if d.notifier != nil {
			failed, _ := d.store.Get(t.ID)
			if failed != nil {
				d.notifier.NotifyTaskFailed(failed)
			}
		}
		logger.WarnCF("task", "Task execution failed", map[string]interface{}{
			"task_id": t.ID,
			"agent":   agentName,
			"error":   err.Error(),
		})
		return
	}

	// If task requires approval, move to review
	if t.Approval {
		d.store.Move(t.ID, TaskStatusReview)
	} else {
		d.store.Move(t.ID, TaskStatusDone)
	}

	updated, _ := d.store.Get(t.ID)
	if updated != nil {
		updated.Result = response
		d.store.Update(updated)

		// Send notifications
		if d.notifier != nil {
			if updated.Status == TaskStatusReview && updated.Approval {
				d.notifier.NotifyApprovalNeeded(updated)
			} else if updated.Status == TaskStatusDone {
				d.notifier.NotifyTaskCompleted(updated)
			}
		}
	}

	logger.InfoCF("task", "Task completed by agent", map[string]interface{}{
		"task_id": t.ID,
		"agent":   agentName,
		"status":  updated.Status,
	})
}

// findMatchingAgent finds the first agent whose task_labels match the task's labels.
// If an agent has empty task_labels, it can work on any task.
func findMatchingAgent(agents []AgentInfo, taskLabels []string) *AgentInfo {
	// Prefer agents with matching labels
	for i := range agents {
		if len(agents[i].TaskLabels) == 0 {
			continue // skip "any" agents for now
		}
		if len(taskLabels) == 0 || HasLabelIntersection(agents[i].TaskLabels, taskLabels) {
			return &agents[i]
		}
	}

	// Fall back to agents with no label restrictions
	for i := range agents {
		if len(agents[i].TaskLabels) == 0 {
			return &agents[i]
		}
	}

	return nil
}
