package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// JSONTaskStore implements TaskStore using one JSON file per task.
type JSONTaskStore struct {
	tasks   map[string]*Task
	mu      sync.RWMutex
	dataDir string
}

// NewJSONTaskStore creates a JSON file-based task store.
func NewJSONTaskStore(dataDir string) (*JSONTaskStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	s := &JSONTaskStore{
		tasks:   make(map[string]*Task),
		dataDir: dataDir,
	}

	if err := s.loadAll(); err != nil {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}

	return s, nil
}

func (s *JSONTaskStore) loadAll() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue
		}

		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}

		if t.ID != "" {
			s.tasks[t.ID] = &t
		}
	}

	return nil
}

func (s *JSONTaskStore) saveTask(t *Task) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataDir, t.ID+".json"), data, 0644)
}

func (s *JSONTaskStore) deleteFile(id string) error {
	return os.Remove(filepath.Join(s.dataDir, id+".json"))
}

func copyTask(t *Task) *Task {
	cp := *t
	if t.Labels != nil {
		cp.Labels = make([]string, len(t.Labels))
		copy(cp.Labels, t.Labels)
	}
	if t.Metadata != nil {
		cp.Metadata = make(map[string]string, len(t.Metadata))
		for k, v := range t.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp
}

// Create adds a new task to the store.
func (s *JSONTaskStore) Create(t *Task) error {
	if t.ID == "" {
		t.ID = generateID()
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = TaskStatusBacklog
	}
	if t.Priority == "" {
		t.Priority = TaskPriorityMedium
	}
	if t.CreatedBy == "" {
		t.CreatedBy = "system"
	}

	if err := t.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[t.ID]; exists {
		return fmt.Errorf("task '%s' already exists", t.ID)
	}

	stored := copyTask(t)
	s.tasks[t.ID] = stored

	if err := s.saveTask(stored); err != nil {
		delete(s.tasks, t.ID)
		return err
	}

	return nil
}

// Get retrieves a task by ID.
func (s *JSONTaskStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return copyTask(t), nil
}

// Update replaces a task in the store.
func (s *JSONTaskStore) Update(t *Task) error {
	if err := t.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[t.ID]; !ok {
		return ErrTaskNotFound
	}

	t.UpdatedAt = time.Now()
	stored := copyTask(t)
	s.tasks[t.ID] = stored

	if err := s.saveTask(stored); err != nil {
		return err
	}

	return nil
}

// Delete removes a task from the store.
func (s *JSONTaskStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrTaskNotFound
	}

	delete(s.tasks, id)
	_ = s.deleteFile(id) // best-effort file removal
	return nil
}

// List returns tasks matching the filter criteria.
func (s *JSONTaskStore) List(filter TaskFilter) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if !matchFilter(t, filter) {
			continue
		}
		result = append(result, copyTask(t))
	}

	// Sort by priority (critical first), then by created_at (oldest first)
	sort.Slice(result, func(i, j int) bool {
		pi := PrioritySortOrder(result[i].Priority)
		pj := PrioritySortOrder(result[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	// Apply offset
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*Task{}, nil
		}
		result = result[filter.Offset:]
	}

	// Apply limit
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// Checkout atomically claims a task for an agent.
func (s *JSONTaskStore) Checkout(taskID, agentName string, agentLabels []string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var t *Task
	if taskID != "" {
		var ok bool
		t, ok = s.tasks[taskID]
		if !ok {
			return nil, ErrTaskNotFound
		}
	} else {
		t = s.findNextTodo()
		if t == nil {
			return nil, ErrTaskNotFound
		}
	}

	if t.Status != TaskStatusTodo || t.AssignedAgent != "" {
		return nil, ErrTaskAlreadyClaimed
	}

	// Check label permissions
	if len(agentLabels) > 0 && len(t.Labels) > 0 {
		if !HasLabelIntersection(agentLabels, t.Labels) {
			return nil, ErrLabelMismatch
		}
	}

	now := time.Now()
	t.Status = TaskStatusInProgress
	t.AssignedAgent = agentName
	t.StartedAt = &now
	t.UpdatedAt = now

	if err := s.saveTask(t); err != nil {
		// Rollback
		t.Status = TaskStatusTodo
		t.AssignedAgent = ""
		t.StartedAt = nil
		return nil, err
	}

	return copyTask(t), nil
}

// findNextTodo returns the highest-priority unassigned todo task.
func (s *JSONTaskStore) findNextTodo() *Task {
	var best *Task
	for _, t := range s.tasks {
		if t.Status != TaskStatusTodo || t.AssignedAgent != "" {
			continue
		}
		if best == nil {
			best = t
			continue
		}
		bp := PrioritySortOrder(best.Priority)
		tp := PrioritySortOrder(t.Priority)
		if tp < bp || (tp == bp && t.CreatedAt.Before(best.CreatedAt)) {
			best = t
		}
	}
	return best
}

// Move changes a task's status with transition validation.
func (s *JSONTaskStore) Move(taskID string, newStatus TaskStatus) error {
	if !IsValidStatus(newStatus) {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	if !IsValidTransition(t.Status, newStatus) {
		return ErrInvalidTransition
	}

	// Block review→done if approval is required
	if t.Approval && t.Status == TaskStatusReview && newStatus == TaskStatusDone {
		return ErrApprovalRequired
	}

	now := time.Now()
	t.Status = newStatus
	t.UpdatedAt = now

	if newStatus == TaskStatusDone || newStatus == TaskStatusFailed {
		t.CompletedAt = &now
	}
	if newStatus == TaskStatusInProgress && t.StartedAt == nil {
		t.StartedAt = &now
	}

	return s.saveTask(t)
}

// Approve marks an approval-required task as done.
func (s *JSONTaskStore) Approve(taskID, approvedBy, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	if t.Status != TaskStatusReview {
		return fmt.Errorf("task must be in review status to approve (current: %s)", t.Status)
	}
	if !t.Approval {
		return fmt.Errorf("task does not require approval")
	}

	now := time.Now()
	t.Status = TaskStatusDone
	t.ApprovedBy = approvedBy
	t.ApprovedAt = &now
	t.ApprovalNote = note
	t.CompletedAt = &now
	t.UpdatedAt = now

	return s.saveTask(t)
}

// Reject marks a task as rejected, returning it to todo.
func (s *JSONTaskStore) Reject(taskID, rejectedBy, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	if t.Status != TaskStatusReview {
		return fmt.Errorf("task must be in review status to reject (current: %s)", t.Status)
	}

	now := time.Now()
	t.Status = TaskStatusRejected
	t.ApprovedBy = rejectedBy
	t.ApprovalNote = note
	t.AssignedAgent = ""
	t.UpdatedAt = now

	return s.saveTask(t)
}

// Stats returns task counts grouped by status.
func (s *JSONTaskStore) Stats() (map[TaskStatus]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[TaskStatus]int)
	for _, t := range s.tasks {
		stats[t.Status]++
	}
	return stats, nil
}

// Cleanup removes expired done/failed tasks based on TTL.
func (s *JSONTaskStore) Cleanup(doneTTLDays, failedTTLDays int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, t := range s.tasks {
		if t.CompletedAt == nil {
			continue
		}
		age := now.Sub(*t.CompletedAt)
		expired := false

		if t.Status == TaskStatusDone && doneTTLDays > 0 {
			expired = age > time.Duration(doneTTLDays)*24*time.Hour
		}
		if t.Status == TaskStatusFailed && failedTTLDays > 0 {
			expired = age > time.Duration(failedTTLDays)*24*time.Hour
		}

		if expired {
			delete(s.tasks, id)
			_ = s.deleteFile(id)
			deleted++
		}
	}

	return deleted, nil
}

// Close is a no-op for the JSON backend.
func (s *JSONTaskStore) Close() error {
	return nil
}

func matchFilter(t *Task, f TaskFilter) bool {
	if f.Status != nil && t.Status != *f.Status {
		return false
	}
	if f.AssignedAgent != nil && t.AssignedAgent != *f.AssignedAgent {
		return false
	}
	if f.Priority != nil && t.Priority != *f.Priority {
		return false
	}
	if f.ParentID != nil && t.ParentID != *f.ParentID {
		return false
	}
	if len(f.Labels) > 0 && !HasLabelIntersection(f.Labels, t.Labels) {
		return false
	}
	return true
}
