package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// TaskStore defines the storage interface for task orchestration.
type TaskStore interface {
	Create(task *Task) error
	Get(id string) (*Task, error)
	Update(task *Task) error
	Delete(id string) error
	List(filter TaskFilter) ([]*Task, error)

	// Checkout atomically claims a task for an agent.
	// If taskID is empty, the highest-priority todo task is claimed.
	// agentLabels restricts checkout to tasks with matching labels (empty = no restriction).
	Checkout(taskID, agentName string, agentLabels []string) (*Task, error)

	// Move changes a task's status with transition validation.
	Move(taskID string, newStatus TaskStatus) error

	// Approve marks an approval-required task as approved and done.
	Approve(taskID, approvedBy, note string) error

	// Reject marks an approval-required task as rejected (returns to todo).
	Reject(taskID, rejectedBy, note string) error

	// Stats returns task counts grouped by status.
	Stats() (map[TaskStatus]int, error)

	// Cleanup removes expired done/failed tasks based on TTL.
	// Returns the number of tasks deleted.
	Cleanup(doneTTLDays, failedTTLDays int) (int, error)

	// Close releases any resources held by the store.
	Close() error
}

// NewTaskStore creates a TaskStore based on the orchestration configuration.
// Returns ErrOrchestrationDisabled if orchestration is not enabled.
func NewTaskStore(cfg *config.OrchestrationConfig) (TaskStore, error) {
	if !cfg.Enabled {
		return nil, ErrOrchestrationDisabled
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))

	switch backend {
	case "sqlite":
		return NewSQLiteTaskStore(expandHome(cfg.DBPath))
	case "json":
		return NewJSONTaskStore(expandHome(cfg.TasksDir))
	case "auto", "":
		dbPath := expandHome(cfg.DBPath)
		store, err := NewSQLiteTaskStore(dbPath)
		if err != nil {
			logger.WarnCF("task", "SQLite backend failed, falling back to JSON", map[string]interface{}{
				"error": err.Error(),
			})
			return NewJSONTaskStore(expandHome(cfg.TasksDir))
		}
		logger.InfoC("task", "Using SQLite backend")
		return store, nil
	default:
		return nil, fmt.Errorf("unknown orchestration backend: %s", backend)
	}
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return filepath.Join(home, path[2:])
		}
		return home
	}
	return path
}
