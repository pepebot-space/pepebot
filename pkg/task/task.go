package task

import (
	"errors"
	"fmt"
	"time"
)

// TaskStatus represents the current state of a task in the Kanban board.
type TaskStatus string

const (
	TaskStatusBacklog    TaskStatus = "backlog"
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusApproved   TaskStatus = "approved"
	TaskStatusRejected   TaskStatus = "rejected"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

// AllStatuses returns all valid task statuses.
func AllStatuses() []TaskStatus {
	return []TaskStatus{
		TaskStatusBacklog, TaskStatusTodo, TaskStatusInProgress,
		TaskStatusReview, TaskStatusApproved, TaskStatusRejected,
		TaskStatusDone, TaskStatusFailed,
	}
}

// IsValidStatus checks if a status value is known.
func IsValidStatus(s TaskStatus) bool {
	for _, v := range AllStatuses() {
		if v == s {
			return true
		}
	}
	return false
}

// TaskPriority represents task urgency.
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// IsValidPriority checks if a priority value is known.
func IsValidPriority(p TaskPriority) bool {
	switch p {
	case TaskPriorityLow, TaskPriorityMedium, TaskPriorityHigh, TaskPriorityCritical:
		return true
	}
	return false
}

// PrioritySortOrder returns a numeric value for sorting (lower = higher priority).
func PrioritySortOrder(p TaskPriority) int {
	switch p {
	case TaskPriorityCritical:
		return 0
	case TaskPriorityHigh:
		return 1
	case TaskPriorityMedium:
		return 2
	case TaskPriorityLow:
		return 3
	default:
		return 4
	}
}

// Task represents a work item in the orchestration system.
type Task struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description,omitempty"`
	Status        TaskStatus        `json:"status"`
	Priority      TaskPriority      `json:"priority"`
	AssignedAgent string            `json:"assigned_agent,omitempty"`
	CreatedBy     string            `json:"created_by"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	Result        string            `json:"result,omitempty"`
	Error         string            `json:"error,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	ParentID      string            `json:"parent_id,omitempty"`
	TTLDays       *int              `json:"ttl_days,omitempty"`
	TemplateID    string            `json:"template_id,omitempty"`
	Approval      bool              `json:"approval,omitempty"`
	ApprovedBy    string            `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time        `json:"approved_at,omitempty"`
	ApprovalNote  string            `json:"approval_note,omitempty"`
}

// TaskFilter specifies criteria for listing tasks.
type TaskFilter struct {
	Status        *TaskStatus   `json:"status,omitempty"`
	AssignedAgent *string       `json:"assigned_agent,omitempty"`
	Priority      *TaskPriority `json:"priority,omitempty"`
	Labels        []string      `json:"labels,omitempty"`
	ParentID      *string       `json:"parent_id,omitempty"`
	Limit         int           `json:"limit,omitempty"`
	Offset        int           `json:"offset,omitempty"`
}

// Sentinel errors.
var (
	ErrOrchestrationDisabled = errors.New("orchestration is not enabled")
	ErrTaskNotFound          = errors.New("task not found")
	ErrTaskAlreadyClaimed    = errors.New("task is already claimed by another agent")
	ErrInvalidTransition     = errors.New("invalid status transition")
	ErrApprovalRequired      = errors.New("task requires approval before moving to done")
	ErrLabelMismatch         = errors.New("agent labels do not match task labels")
)

// ValidTransitions defines which status transitions are allowed.
var ValidTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusBacklog:    {TaskStatusTodo},
	TaskStatusTodo:       {TaskStatusInProgress, TaskStatusBacklog},
	TaskStatusInProgress: {TaskStatusReview, TaskStatusDone, TaskStatusFailed, TaskStatusTodo},
	TaskStatusReview:     {TaskStatusDone, TaskStatusApproved, TaskStatusRejected, TaskStatusInProgress, TaskStatusTodo},
	TaskStatusApproved:   {TaskStatusDone},
	TaskStatusRejected:   {TaskStatusTodo},
	TaskStatusDone:       {TaskStatusTodo},
	TaskStatusFailed:     {TaskStatusTodo},
}

// IsValidTransition checks if a status transition is allowed.
func IsValidTransition(from, to TaskStatus) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// Validate checks that the task has all required fields and valid values.
func (t *Task) Validate() error {
	if t.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if t.Status != "" && !IsValidStatus(t.Status) {
		return fmt.Errorf("invalid task status: %s", t.Status)
	}
	if t.Priority != "" && !IsValidPriority(t.Priority) {
		return fmt.Errorf("invalid task priority: %s", t.Priority)
	}
	return nil
}

// HasLabelIntersection returns true if slices a and b share at least one element.
func HasLabelIntersection(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

// generateID produces a unique task ID using nanosecond timestamp.
func generateID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
