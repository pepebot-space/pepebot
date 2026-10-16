// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
)

// Notifier is how a finished task reaches a human. The tasks package knows
// nothing about chat; the agent implements this and decides what to say.
type Notifier interface {
	// TaskFinished is called once a task reaches a terminal state.
	TaskFinished(t *Task)
	// TaskStale is called at startup for work that was running when pepebot
	// stopped. Nobody will finish it, so the requester is asked whether to run
	// it again — the one thing that must not happen is silence.
	TaskStale(t *Task)
}

// Manager owns the store, the queue and the executors.
type Manager struct {
	store     *Store
	notifier  Notifier
	resultDir string

	executors map[string]Executor
	mu        sync.RWMutex

	queue   chan string
	workers int
	wg      sync.WaitGroup
	stop    chan struct{}
	once    sync.Once

	// inlineLimit is how much result text goes into the task record (and so
	// into the summary turn). Anything longer is written to disk and only
	// pointed at: a chat message is not a log file.
	inlineLimit int
}

// Options configure a Manager. Zero values pick sensible defaults.
type Options struct {
	// Workers is how many tasks run at once. Defaults to the host's thread
	// count: enough to use the machine, bounded so a burst of delegations
	// cannot start an unbounded number of subprocesses.
	Workers     int
	QueueSize   int
	InlineLimit int
}

func NewManager(workspace string, notifier Notifier, opts Options) (*Manager, error) {
	store, err := OpenStore(filepath.Join(workspace, "tasks.json"))
	if err != nil {
		return nil, err
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	queueSize := opts.QueueSize
	if queueSize <= 0 {
		queueSize = 256
	}
	inline := opts.InlineLimit
	if inline <= 0 {
		inline = 4000
	}

	m := &Manager{
		store:       store,
		notifier:    notifier,
		resultDir:   filepath.Join(workspace, "task-results"),
		executors:   map[string]Executor{},
		queue:       make(chan string, queueSize),
		workers:     workers,
		stop:        make(chan struct{}),
		inlineLimit: inline,
	}
	return m, nil
}

// Register adds an executor under a name used by Task.Executor.
func (m *Manager) Register(name string, e Executor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executors[name] = e
}

// Start brings up the worker pool and reconciles anything left running by a
// previous process.
func (m *Manager) Start(ctx context.Context) {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker(ctx)
	}
	logger.InfoCF("tasks", "Task runner started", map[string]interface{}{
		"workers": m.workers, "queue": cap(m.queue),
	})
	m.reconcile()
}

// Stop lets in-flight workers finish.
func (m *Manager) Stop() {
	m.once.Do(func() { close(m.stop) })
	m.wg.Wait()
}

// Delegate records a task and queues it.
func (m *Manager) Delegate(t *Task) (*Task, error) {
	m.mu.RLock()
	_, known := m.executors[t.Executor]
	m.mu.RUnlock()
	if !known {
		return nil, fmt.Errorf("no executor named %q; configured: %s", t.Executor, strings.Join(m.executorNames(), ", "))
	}

	created, err := m.store.Create(t)
	if err != nil {
		return nil, err
	}

	select {
	case m.queue <- created.ID:
	default:
		_, _ = m.store.Update(created.ID, func(t *Task) {
			t.Status = StatusFailed
			t.Error = "task queue is full"
			t.FinishedMS = nowMS()
		})
		return nil, fmt.Errorf("task queue is full (%d waiting)", cap(m.queue))
	}

	logger.InfoCF("tasks", "Task delegated", map[string]interface{}{
		"task": created.ID, "executor": created.Executor, "label": created.Label,
	})
	return created, nil
}

// Complete records a result that arrived from outside — the completion hook.
// It is idempotent: a worker that retries its callback does not produce a
// second answer in the chat.
func (m *Manager) Complete(id, token, status, result string) error {
	t, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("no task %s", id)
	}
	if token == "" || token != t.Token {
		return fmt.Errorf("wrong token for task %s", id)
	}
	if t.Terminal() {
		return nil
	}

	if status == "" {
		status = StatusDone
	}
	summary := m.storeResult(t, result)

	updated, err := m.store.Update(id, func(t *Task) {
		t.Status = status
		t.Result = summary
		t.FinishedMS = nowMS()
		if status == StatusFailed && t.Error == "" {
			t.Error = truncate(result, 500)
		}
	})
	if err != nil {
		return err
	}

	m.notify(updated)
	return nil
}

// Cancel stops tracking a task. Work already running elsewhere is not killed —
// pepebot cannot reach into another machine — but nothing will be reported.
func (m *Manager) Cancel(id string) error {
	t, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("no task %s", id)
	}
	if t.Terminal() {
		return fmt.Errorf("task %s already finished", id)
	}
	_, err := m.store.Update(id, func(t *Task) {
		t.Status = StatusCanceled
		t.FinishedMS = nowMS()
	})
	return err
}

// Resume re-queues a task that went stale across a restart, after the requester
// said to carry on.
func (m *Manager) Resume(id string) error {
	t, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("no task %s", id)
	}
	if t.Status != StatusStale {
		return fmt.Errorf("task %s is %s, nothing to resume", id, t.Status)
	}
	if _, err := m.store.Update(id, func(t *Task) {
		t.Status = StatusQueued
		t.Error = ""
		t.StartedMS = 0
	}); err != nil {
		return err
	}
	select {
	case m.queue <- id:
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

func (m *Manager) Get(id string) (*Task, bool) { return m.store.Get(id) }
func (m *Manager) List() []*Task               { return m.store.List() }
func (m *Manager) Active() []*Task             { return m.store.Active() }

func (m *Manager) worker(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		case id := <-m.queue:
			m.run(ctx, id)
		}
	}
}

func (m *Manager) run(ctx context.Context, id string) {
	t, ok := m.store.Get(id)
	if !ok {
		return
	}
	// Already settled: cancelled, or a callback that beat the worker to the
	// queue. Running it now would overwrite a finished task with "running" and
	// let the next callback report the same work a second time.
	if t.Terminal() {
		return
	}

	m.mu.RLock()
	exec, known := m.executors[t.Executor]
	m.mu.RUnlock()
	if !known {
		m.finish(id, StatusFailed, "", fmt.Errorf("executor %q disappeared", t.Executor))
		return
	}

	started, _ := m.store.Update(id, func(t *Task) {
		t.Status = StatusRunning
		t.StartedMS = nowMS()
	})

	result, err := exec.Run(ctx, started)
	switch {
	case err == ErrAsync:
		// Dispatched; the hook will finish it. Leave it running.
		logger.InfoCF("tasks", "Task handed to a remote worker", map[string]interface{}{
			"task": id, "target": started.Target,
		})
	case err != nil:
		m.finish(id, StatusFailed, result, err)
	default:
		m.finish(id, StatusDone, result, nil)
	}
}

func (m *Manager) finish(id, status, result string, cause error) {
	t, ok := m.store.Get(id)
	if !ok {
		return
	}
	summary := m.storeResult(t, result)

	updated, err := m.store.Update(id, func(t *Task) {
		t.Status = status
		t.Result = summary
		t.FinishedMS = nowMS()
		if cause != nil {
			t.Error = truncate(cause.Error(), 500)
		}
	})
	if err != nil {
		logger.ErrorCF("tasks", "Could not record task result", map[string]interface{}{
			"task": id, "error": err.Error(),
		})
		return
	}
	m.notify(updated)
}

// storeResult keeps the whole output on disk when it is long, and returns what
// is worth carrying into a chat message.
func (m *Manager) storeResult(t *Task, result string) string {
	if len(result) <= m.inlineLimit {
		return result
	}
	if err := os.MkdirAll(m.resultDir, 0o755); err == nil {
		path := filepath.Join(m.resultDir, t.ID+".txt")
		if err := os.WriteFile(path, []byte(result), 0o644); err == nil {
			_, _ = m.store.Update(t.ID, func(t *Task) { t.ResultPath = path })
			return result[:m.inlineLimit] + fmt.Sprintf("\n…[dipotong; hasil lengkap di %s]", path)
		}
	}
	return result[:m.inlineLimit] + "\n…[dipotong]"
}

func (m *Manager) notify(t *Task) {
	if m.notifier == nil {
		return
	}
	logger.InfoCF("tasks", "Task finished", map[string]interface{}{
		"task": t.ID, "status": t.Status, "label": t.Label,
	})
	m.notifier.TaskFinished(t)
}

// reconcile deals with tasks that were running when the process stopped.
//
// Nobody is going to finish them: an in-process executor died with the process,
// and a remote one has a callback that arrived nowhere. Marking them stale and
// telling the requester — with the choice to run it again — is the only honest
// option. Silently dropping them is what the sketch this replaces did.
func (m *Manager) reconcile() {
	for _, t := range m.store.List() {
		if t.Status != StatusRunning && t.Status != StatusQueued {
			continue
		}
		updated, err := m.store.Update(t.ID, func(t *Task) {
			t.Status = StatusStale
			t.Error = "pepebot restarted while this task was running"
		})
		if err != nil {
			continue
		}
		logger.WarnCF("tasks", "Task was interrupted by a restart", map[string]interface{}{
			"task": t.ID, "label": t.Label,
		})
		if m.notifier != nil {
			m.notifier.TaskStale(updated)
		}
	}
}

func (m *Manager) executorNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.executors))
	for n := range m.executors {
		names = append(names, n)
	}
	return names
}

func nowMS() int64 { return time.Now().UnixMilli() }
