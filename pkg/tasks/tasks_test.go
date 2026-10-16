// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu       sync.Mutex
	finished []*Task
	stale    []*Task
}

func (r *recorder) TaskFinished(t *Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finished = append(r.finished, t)
}

func (r *recorder) TaskStale(t *Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stale = append(r.stale, t)
}

func (r *recorder) waitFinished(t *testing.T, n int) []*Task {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		got := len(r.finished)
		r.mu.Unlock()
		if got >= n {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Task(nil), r.finished...)
}

func newManager(t *testing.T, rec *recorder) (*Manager, string) {
	t.Helper()
	ws := t.TempDir()
	m, err := NewManager(ws, rec, Options{Workers: 2, InlineLimit: 100})
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	return m, ws
}

func TestDelegateRunsAndReportsBack(t *testing.T) {
	rec := &recorder{}
	m, _ := newManager(t, rec)
	m.Register("test", FuncExecutor(func(ctx context.Context, task *Task) (string, error) {
		return "selesai: " + task.Prompt, nil
	}))
	m.Start(context.Background())
	defer m.Stop()

	created, err := m.Delegate(&Task{Prompt: "hitung sesuatu", Executor: "test",
		Origin: Origin{Channel: "discord", ChatID: "42"}})
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	if !strings.HasPrefix(created.ID, "t_") || created.Token == "" {
		t.Fatalf("task = %+v, want an id and a callback token", created)
	}

	finished := rec.waitFinished(t, 1)
	if len(finished) != 1 {
		t.Fatalf("nobody was told the task finished")
	}
	if finished[0].Status != StatusDone || !strings.Contains(finished[0].Result, "hitung sesuatu") {
		t.Errorf("task = %+v", finished[0])
	}
	if finished[0].Origin.ChatID != "42" {
		t.Errorf("origin lost: %+v", finished[0].Origin)
	}
}

func TestFailureIsReportedNotSwallowed(t *testing.T) {
	rec := &recorder{}
	m, _ := newManager(t, rec)
	m.Register("boom", FuncExecutor(func(ctx context.Context, task *Task) (string, error) {
		return "", errors.New("worker meledak")
	}))
	m.Start(context.Background())
	defer m.Stop()

	if _, err := m.Delegate(&Task{Prompt: "x", Executor: "boom"}); err != nil {
		t.Fatalf("delegate: %v", err)
	}
	finished := rec.waitFinished(t, 1)
	if len(finished) != 1 || finished[0].Status != StatusFailed {
		t.Fatalf("failure not reported: %+v", finished)
	}
	if !strings.Contains(finished[0].Error, "meledak") {
		t.Errorf("error lost: %q", finished[0].Error)
	}
}

// A remote worker returns ErrAsync and answers later through the hook. The same
// callback arriving twice must not produce two answers in the chat.
func TestCallbackCompletesOnceAndNeedsTheToken(t *testing.T) {
	rec := &recorder{}
	m, _ := newManager(t, rec)
	m.Register("remote", FuncExecutor(func(ctx context.Context, task *Task) (string, error) {
		return "", ErrAsync
	}))
	m.Start(context.Background())
	defer m.Stop()

	created, err := m.Delegate(&Task{Prompt: "kerja jauh", Executor: "remote"})
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}

	if err := m.Complete(created.ID, "token-palsu", StatusDone, "hasil"); err == nil {
		t.Error("a wrong token was accepted")
	}
	if err := m.Complete(created.ID, created.Token, StatusDone, "hasilnya ini"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	// Retried callback.
	if err := m.Complete(created.ID, created.Token, StatusDone, "hasilnya ini"); err != nil {
		t.Fatalf("second complete should be a no-op, got %v", err)
	}

	finished := rec.waitFinished(t, 1)
	if len(finished) != 1 {
		t.Fatalf("got %d notifications, want exactly 1", len(finished))
	}
}

// Work that was running when pepebot stopped has nobody to finish it. It must
// surface as a question to the requester, not vanish. Written as a store left
// behind by a dead process, which is exactly what a restart finds.
func TestRestartTurnsRunningTasksIntoStaleAndAsks(t *testing.T) {
	ws := t.TempDir()
	left := `[{"id":"t_tertinggal","label":"impor data","prompt":"impor 10rb baris",
	  "executor":"slow","status":"running","token":"tok","created_ms":1,"started_ms":2,
	  "origin":{"channel":"discord","chat_id":"42"}}]`
	if err := os.WriteFile(filepath.Join(ws, "tasks.json"), []byte(left), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	rec := &recorder{}
	m, err := NewManager(ws, rec, Options{Workers: 1})
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	m.Register("slow", FuncExecutor(func(ctx context.Context, task *Task) (string, error) {
		return "akhirnya selesai", nil
	}))
	m.Start(context.Background())
	defer m.Stop()

	rec.mu.Lock()
	stale := len(rec.stale)
	rec.mu.Unlock()
	if stale != 1 {
		t.Fatalf("got %d stale notifications, want 1 — an interrupted task must not disappear", stale)
	}
	if cur, _ := m.Get("t_tertinggal"); cur.Status != StatusStale {
		t.Fatalf("status = %s, want stale", cur.Status)
	}

	// And the requester can say "carry on".
	if err := m.Resume("t_tertinggal"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	finished := rec.waitFinished(t, 1)
	if len(finished) != 1 || finished[0].Status != StatusDone {
		t.Errorf("resumed task did not complete: %+v", finished)
	}
}

func TestDepthIsCapped(t *testing.T) {
	m, _ := newManager(t, &recorder{})
	m.Register("test", FuncExecutor(func(ctx context.Context, task *Task) (string, error) { return "", nil }))
	if _, err := m.Delegate(&Task{Prompt: "x", Executor: "test", Depth: MaxDepth + 1}); err == nil {
		t.Error("a task nested past the cap was accepted")
	}
}

func TestUnknownExecutorIsRefusedWithTheList(t *testing.T) {
	m, _ := newManager(t, &recorder{})
	m.Register("exec", FuncExecutor(func(ctx context.Context, task *Task) (string, error) { return "", nil }))
	_, err := m.Delegate(&Task{Prompt: "x", Executor: "tidakada"})
	if err == nil || !strings.Contains(err.Error(), "exec") {
		t.Errorf("error = %v, want it to name what executors exist", err)
	}
}

// A long result belongs on disk; the chat gets a summary pointing at it.
func TestLongResultSpillsToDisk(t *testing.T) {
	rec := &recorder{}
	m, ws := newManager(t, rec)
	long := strings.Repeat("x", 5000)
	m.Register("big", FuncExecutor(func(ctx context.Context, task *Task) (string, error) { return long, nil }))
	m.Start(context.Background())
	defer m.Stop()

	if _, err := m.Delegate(&Task{Prompt: "x", Executor: "big"}); err != nil {
		t.Fatalf("delegate: %v", err)
	}
	finished := rec.waitFinished(t, 1)
	if len(finished) != 1 {
		t.Fatal("no notification")
	}
	if len(finished[0].Result) > 400 {
		t.Errorf("summary is %d chars; it should be trimmed", len(finished[0].Result))
	}
	files, _ := os.ReadDir(filepath.Join(ws, "task-results"))
	if len(files) != 1 {
		t.Errorf("full result was not written to disk: %v", files)
	}
}

func TestStoreSurvivesEmptyFile(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "tasks.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := OpenStore(path); err != nil {
		t.Errorf("an empty store should be empty, not broken: %v", err)
	}
}
