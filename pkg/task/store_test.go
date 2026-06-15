package task

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// runStoreTests runs the full test suite against any TaskStore implementation.
func runStoreTests(t *testing.T, newStore func(t *testing.T) TaskStore) {
	t.Run("CreateAndGet", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title:     "Test task",
			CreatedBy: "tester",
			Priority:  TaskPriorityHigh,
			Labels:    []string{"code", "test"},
			Metadata:  map[string]string{"env": "staging"},
		}

		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
		if task.ID == "" {
			t.Fatal("expected ID to be set")
		}

		got, err := s.Get(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Test task" {
			t.Fatalf("title mismatch: %s", got.Title)
		}
		if got.Priority != TaskPriorityHigh {
			t.Fatalf("priority mismatch: %s", got.Priority)
		}
		if got.Status != TaskStatusBacklog {
			t.Fatalf("expected backlog status, got: %s", got.Status)
		}
		if len(got.Labels) != 2 || got.Labels[0] != "code" {
			t.Fatalf("labels mismatch: %v", got.Labels)
		}
		if got.Metadata["env"] != "staging" {
			t.Fatalf("metadata mismatch: %v", got.Metadata)
		}
	})

	t.Run("CreateAutoID", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Auto ID", CreatedBy: "tester"}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
		if task.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
	})

	t.Run("CreateValidationError", func(t *testing.T) {
		s := newStore(t)
		task := &Task{CreatedBy: "tester"} // missing title
		if err := s.Create(task); err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s := newStore(t)
		_, err := s.Get("nonexistent")
		if err != ErrTaskNotFound {
			t.Fatalf("expected ErrTaskNotFound, got: %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Original", CreatedBy: "tester"}
		s.Create(task)

		task.Title = "Updated"
		if err := s.Update(task); err != nil {
			t.Fatal(err)
		}

		got, _ := s.Get(task.ID)
		if got.Title != "Updated" {
			t.Fatalf("expected 'Updated', got: %s", got.Title)
		}
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		s := newStore(t)
		task := &Task{ID: "nonexistent", Title: "X", CreatedBy: "tester"}
		err := s.Update(task)
		if err != ErrTaskNotFound {
			t.Fatalf("expected ErrTaskNotFound, got: %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "To delete", CreatedBy: "tester"}
		s.Create(task)

		if err := s.Delete(task.ID); err != nil {
			t.Fatal(err)
		}

		_, err := s.Get(task.ID)
		if err != ErrTaskNotFound {
			t.Fatalf("expected ErrTaskNotFound after delete, got: %v", err)
		}
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := newStore(t)
		err := s.Delete("nonexistent")
		if err != ErrTaskNotFound {
			t.Fatalf("expected ErrTaskNotFound, got: %v", err)
		}
	})

	t.Run("ListAll", func(t *testing.T) {
		s := newStore(t)
		for i := 0; i < 5; i++ {
			s.Create(&Task{Title: "Task", CreatedBy: "tester"})
			time.Sleep(time.Millisecond) // ensure unique IDs
		}

		result, err := s.List(TaskFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 5 {
			t.Fatalf("expected 5 tasks, got: %d", len(result))
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		s := newStore(t)
		s.Create(&Task{Title: "Backlog 1", CreatedBy: "tester", Status: TaskStatusBacklog})
		s.Create(&Task{Title: "Todo 1", CreatedBy: "tester", Status: TaskStatusTodo})
		s.Create(&Task{Title: "Todo 2", CreatedBy: "tester", Status: TaskStatusTodo})
		time.Sleep(time.Millisecond)

		status := TaskStatusTodo
		result, _ := s.List(TaskFilter{Status: &status})
		if len(result) != 2 {
			t.Fatalf("expected 2 todo tasks, got: %d", len(result))
		}
	})

	t.Run("ListByAgent", func(t *testing.T) {
		s := newStore(t)
		t1 := &Task{Title: "A", CreatedBy: "tester", Status: TaskStatusInProgress, AssignedAgent: "coder"}
		t2 := &Task{Title: "B", CreatedBy: "tester", Status: TaskStatusInProgress, AssignedAgent: "researcher"}
		s.Create(t1)
		s.Create(t2)
		time.Sleep(time.Millisecond)

		agent := "coder"
		result, _ := s.List(TaskFilter{AssignedAgent: &agent})
		if len(result) != 1 || result[0].AssignedAgent != "coder" {
			t.Fatalf("expected 1 task for coder, got: %d", len(result))
		}
	})

	t.Run("ListByPriority", func(t *testing.T) {
		s := newStore(t)
		s.Create(&Task{Title: "Low", CreatedBy: "tester", Priority: TaskPriorityLow})
		s.Create(&Task{Title: "High", CreatedBy: "tester", Priority: TaskPriorityHigh})
		time.Sleep(time.Millisecond)

		prio := TaskPriorityHigh
		result, _ := s.List(TaskFilter{Priority: &prio})
		if len(result) != 1 || result[0].Title != "High" {
			t.Fatalf("expected 1 high priority task, got: %d", len(result))
		}
	})

	t.Run("ListByParentID", func(t *testing.T) {
		s := newStore(t)
		parent := &Task{Title: "Parent", CreatedBy: "tester"}
		s.Create(parent)
		s.Create(&Task{Title: "Child 1", CreatedBy: "tester", ParentID: parent.ID})
		s.Create(&Task{Title: "Child 2", CreatedBy: "tester", ParentID: parent.ID})
		s.Create(&Task{Title: "Unrelated", CreatedBy: "tester"})
		time.Sleep(time.Millisecond)

		pid := parent.ID
		result, _ := s.List(TaskFilter{ParentID: &pid})
		if len(result) != 2 {
			t.Fatalf("expected 2 children, got: %d", len(result))
		}
	})

	t.Run("ListPagination", func(t *testing.T) {
		s := newStore(t)
		for i := 0; i < 10; i++ {
			s.Create(&Task{Title: "Task", CreatedBy: "tester"})
			time.Sleep(time.Millisecond)
		}

		result, _ := s.List(TaskFilter{Limit: 3, Offset: 2})
		if len(result) != 3 {
			t.Fatalf("expected 3 tasks, got: %d", len(result))
		}
	})

	t.Run("ListPriorityOrder", func(t *testing.T) {
		s := newStore(t)
		s.Create(&Task{Title: "Low", CreatedBy: "tester", Priority: TaskPriorityLow})
		time.Sleep(time.Millisecond)
		s.Create(&Task{Title: "Critical", CreatedBy: "tester", Priority: TaskPriorityCritical})
		time.Sleep(time.Millisecond)
		s.Create(&Task{Title: "Medium", CreatedBy: "tester", Priority: TaskPriorityMedium})
		time.Sleep(time.Millisecond)

		result, _ := s.List(TaskFilter{})
		if result[0].Title != "Critical" {
			t.Fatalf("expected Critical first, got: %s", result[0].Title)
		}
	})

	t.Run("CheckoutSpecificTask", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Checkout me", CreatedBy: "tester", Status: TaskStatusTodo}
		s.Create(task)

		got, err := s.Checkout(task.ID, "agent-1", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != TaskStatusInProgress {
			t.Fatalf("expected in_progress, got: %s", got.Status)
		}
		if got.AssignedAgent != "agent-1" {
			t.Fatalf("expected agent-1, got: %s", got.AssignedAgent)
		}
		if got.StartedAt == nil {
			t.Fatal("expected started_at to be set")
		}
	})

	t.Run("CheckoutNextAvailable", func(t *testing.T) {
		s := newStore(t)
		s.Create(&Task{Title: "Low prio", CreatedBy: "tester", Status: TaskStatusTodo, Priority: TaskPriorityLow})
		time.Sleep(time.Millisecond)
		s.Create(&Task{Title: "Critical", CreatedBy: "tester", Status: TaskStatusTodo, Priority: TaskPriorityCritical})
		time.Sleep(time.Millisecond)

		got, err := s.Checkout("", "agent-1", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Critical" {
			t.Fatalf("expected Critical task, got: %s", got.Title)
		}
	})

	t.Run("CheckoutAlreadyClaimed", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Claimed", CreatedBy: "tester", Status: TaskStatusTodo}
		s.Create(task)

		_, err := s.Checkout(task.ID, "agent-1", nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = s.Checkout(task.ID, "agent-2", nil)
		if err != ErrTaskAlreadyClaimed {
			t.Fatalf("expected ErrTaskAlreadyClaimed, got: %v", err)
		}
	})

	t.Run("CheckoutWrongStatus", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Backlog", CreatedBy: "tester", Status: TaskStatusBacklog}
		s.Create(task)

		_, err := s.Checkout(task.ID, "agent-1", nil)
		if err == nil {
			t.Fatal("expected error for non-todo task")
		}
	})

	t.Run("CheckoutLabelPermission", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title: "Code task", CreatedBy: "tester",
			Status: TaskStatusTodo, Labels: []string{"code"},
		}
		s.Create(task)

		// Agent with non-matching labels
		_, err := s.Checkout(task.ID, "researcher", []string{"research"})
		if err != ErrLabelMismatch {
			t.Fatalf("expected ErrLabelMismatch, got: %v", err)
		}

		// Agent with matching labels
		got, err := s.Checkout(task.ID, "coder", []string{"code", "review"})
		if err != nil {
			t.Fatal(err)
		}
		if got.AssignedAgent != "coder" {
			t.Fatalf("expected coder, got: %s", got.AssignedAgent)
		}
	})

	t.Run("CheckoutEmptyLabelsAllowed", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title: "Any task", CreatedBy: "tester",
			Status: TaskStatusTodo, Labels: []string{"code"},
		}
		s.Create(task)

		// Agent with no label restrictions
		got, err := s.Checkout(task.ID, "default", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.AssignedAgent != "default" {
			t.Fatalf("expected default, got: %s", got.AssignedAgent)
		}
	})

	t.Run("MoveValidTransition", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Move me", CreatedBy: "tester", Status: TaskStatusBacklog}
		s.Create(task)

		if err := s.Move(task.ID, TaskStatusTodo); err != nil {
			t.Fatal(err)
		}

		got, _ := s.Get(task.ID)
		if got.Status != TaskStatusTodo {
			t.Fatalf("expected todo, got: %s", got.Status)
		}
	})

	t.Run("MoveInvalidTransition", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Invalid", CreatedBy: "tester", Status: TaskStatusBacklog}
		s.Create(task)

		err := s.Move(task.ID, TaskStatusDone)
		if err != ErrInvalidTransition {
			t.Fatalf("expected ErrInvalidTransition, got: %v", err)
		}
	})

	t.Run("MoveApprovalBlock", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title: "Needs approval", CreatedBy: "tester",
			Status: TaskStatusTodo, Approval: true,
		}
		s.Create(task)

		s.Move(task.ID, TaskStatusInProgress)
		s.Move(task.ID, TaskStatusReview)

		err := s.Move(task.ID, TaskStatusDone)
		if err != ErrApprovalRequired {
			t.Fatalf("expected ErrApprovalRequired, got: %v", err)
		}
	})

	t.Run("MoveCompletedAt", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Complete", CreatedBy: "tester", Status: TaskStatusTodo}
		s.Create(task)
		s.Move(task.ID, TaskStatusInProgress)
		s.Move(task.ID, TaskStatusDone)

		got, _ := s.Get(task.ID)
		if got.CompletedAt == nil {
			t.Fatal("expected completed_at to be set")
		}
	})

	t.Run("Approve", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title: "Approve me", CreatedBy: "tester",
			Status: TaskStatusTodo, Approval: true,
		}
		s.Create(task)
		s.Move(task.ID, TaskStatusInProgress)
		s.Move(task.ID, TaskStatusReview)

		if err := s.Approve(task.ID, "admin", "Looks good"); err != nil {
			t.Fatal(err)
		}

		got, _ := s.Get(task.ID)
		if got.Status != TaskStatusDone {
			t.Fatalf("expected done, got: %s", got.Status)
		}
		if got.ApprovedBy != "admin" {
			t.Fatalf("expected admin, got: %s", got.ApprovedBy)
		}
		if got.ApprovalNote != "Looks good" {
			t.Fatalf("expected note, got: %s", got.ApprovalNote)
		}
		if got.ApprovedAt == nil {
			t.Fatal("expected approved_at to be set")
		}
		if got.CompletedAt == nil {
			t.Fatal("expected completed_at to be set")
		}
	})

	t.Run("ApproveNotInReview", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Wrong status", CreatedBy: "tester", Status: TaskStatusTodo, Approval: true}
		s.Create(task)

		err := s.Approve(task.ID, "admin", "nope")
		if err == nil {
			t.Fatal("expected error for non-review task")
		}
	})

	t.Run("ApproveNotRequired", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "No approval", CreatedBy: "tester", Status: TaskStatusTodo}
		s.Create(task)
		s.Move(task.ID, TaskStatusInProgress)
		s.Move(task.ID, TaskStatusReview)

		err := s.Approve(task.ID, "admin", "hmm")
		if err == nil {
			t.Fatal("expected error for task without approval flag")
		}
	})

	t.Run("Reject", func(t *testing.T) {
		s := newStore(t)
		task := &Task{
			Title: "Reject me", CreatedBy: "tester",
			Status: TaskStatusTodo, Approval: true,
		}
		s.Create(task)
		s.Move(task.ID, TaskStatusInProgress)
		s.Move(task.ID, TaskStatusReview)

		if err := s.Reject(task.ID, "admin", "Needs more work"); err != nil {
			t.Fatal(err)
		}

		got, _ := s.Get(task.ID)
		if got.Status != TaskStatusRejected {
			t.Fatalf("expected rejected, got: %s", got.Status)
		}
		if got.AssignedAgent != "" {
			t.Fatalf("expected empty agent, got: %s", got.AssignedAgent)
		}
		if got.ApprovalNote != "Needs more work" {
			t.Fatalf("expected note, got: %s", got.ApprovalNote)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		s := newStore(t)
		s.Create(&Task{Title: "A", CreatedBy: "tester", Status: TaskStatusBacklog})
		s.Create(&Task{Title: "B", CreatedBy: "tester", Status: TaskStatusBacklog})
		s.Create(&Task{Title: "C", CreatedBy: "tester", Status: TaskStatusTodo})
		s.Create(&Task{Title: "D", CreatedBy: "tester", Status: TaskStatusDone})
		time.Sleep(time.Millisecond)

		stats, err := s.Stats()
		if err != nil {
			t.Fatal(err)
		}
		if stats[TaskStatusBacklog] != 2 {
			t.Fatalf("expected 2 backlog, got: %d", stats[TaskStatusBacklog])
		}
		if stats[TaskStatusTodo] != 1 {
			t.Fatalf("expected 1 todo, got: %d", stats[TaskStatusTodo])
		}
		if stats[TaskStatusDone] != 1 {
			t.Fatalf("expected 1 done, got: %d", stats[TaskStatusDone])
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		s := newStore(t)

		// Old done task (should be cleaned)
		old := time.Now().Add(-40 * 24 * time.Hour)
		t1 := &Task{Title: "Old done", CreatedBy: "tester", Status: TaskStatusDone, CompletedAt: &old}
		s.Create(t1)

		// Recent done task (should stay)
		recent := time.Now().Add(-5 * 24 * time.Hour)
		t2 := &Task{Title: "Recent done", CreatedBy: "tester", Status: TaskStatusDone, CompletedAt: &recent}
		s.Create(t2)

		// Old failed task (should be cleaned)
		oldFailed := time.Now().Add(-10 * 24 * time.Hour)
		t3 := &Task{Title: "Old failed", CreatedBy: "tester", Status: TaskStatusFailed, CompletedAt: &oldFailed}
		s.Create(t3)

		time.Sleep(time.Millisecond)

		deleted, err := s.Cleanup(30, 7)
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 2 {
			t.Fatalf("expected 2 deleted, got: %d", deleted)
		}

		// Verify t2 still exists
		_, err = s.Get(t2.ID)
		if err != nil {
			t.Fatal("recent done task should still exist")
		}

		// Verify t1 is gone
		_, err = s.Get(t1.ID)
		if err != ErrTaskNotFound {
			t.Fatal("old done task should be deleted")
		}
	})

	t.Run("ConcurrentCheckout", func(t *testing.T) {
		s := newStore(t)
		task := &Task{Title: "Race me", CreatedBy: "tester", Status: TaskStatusTodo}
		s.Create(task)

		var successes atomic.Int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				_, err := s.Checkout(task.ID, fmt.Sprintf("agent-%d", n), nil)
				if err == nil {
					successes.Add(1)
				}
			}(i)
		}

		wg.Wait()

		if successes.Load() != 1 {
			t.Fatalf("expected exactly 1 successful checkout, got: %d", successes.Load())
		}
	})
}

func TestJSONTaskStore(t *testing.T) {
	runStoreTests(t, func(t *testing.T) TaskStore {
		dir := filepath.Join(t.TempDir(), "tasks")
		store, err := NewJSONTaskStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { store.Close() })
		return store
	})
}

func TestSQLiteTaskStore(t *testing.T) {
	runStoreTests(t, func(t *testing.T) TaskStore {
		dir := t.TempDir()
		store, err := NewSQLiteTaskStore(filepath.Join(dir, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { store.Close() })
		return store
	})
}

func TestJSONPersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")

	// Create store and add tasks
	store1, err := NewJSONTaskStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	task := &Task{Title: "Persistent", CreatedBy: "tester"}
	store1.Create(task)
	taskID := task.ID
	store1.Close()

	// Reopen from same directory
	store2, err := NewJSONTaskStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	got, err := store2.Get(taskID)
	if err != nil {
		t.Fatal("task should persist across store restarts")
	}
	if got.Title != "Persistent" {
		t.Fatalf("expected 'Persistent', got: %s", got.Title)
	}
}

// Ensure fmt is used.
var _ = fmt.Sprintf
