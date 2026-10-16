// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store keeps tasks on disk.
//
// On disk, not in a map, because the whole point of delegation is that the work
// outlives the message that asked for it — and a process restart in the middle
// is the normal case, not the exception. The sketch this replaces kept tasks in
// a map and lost every one of them on restart, silently.
type Store struct {
	path  string
	mu    sync.RWMutex
	tasks map[string]*Task
	// loadedMS is the file's modification time as of the last read. Another
	// pepebot process — the CLI delegating while the gateway serves the
	// completion hook — writes the same file, and a reader that never looks
	// again answers "no such task" for work it simply has not seen yet.
	//
	// This makes readers current. It does not make concurrent writers safe:
	// one workspace is meant to have one writer, and two processes delegating
	// at the same moment can still lose one of the two records.
	loadedMS int64
}

// OpenStore reads the task file, tolerating one that is missing or empty. An
// empty file is a store with no tasks: a crash between create and write leaves
// exactly that, and treating it as corrupt took down a service for five months
// once already.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, tasks: map[string]*Task{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}

	var list []*Task
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("task store %s is unreadable: %w", path, err)
	}
	for _, t := range list {
		s.tasks[t.ID] = t
	}
	return s, nil
}

// Create registers a new task. The caller gets it back with its id and token.
func (s *Store) Create(t *Task) (*Task, error) {
	if t.Depth > MaxDepth {
		return nil, fmt.Errorf("task nesting is capped at %d levels; this one would be %d", MaxDepth, t.Depth)
	}
	s.mu.Lock()
	t.ID = newID()
	t.Token = newToken()
	t.Status = StatusQueued
	t.CreatedMS = nowMS()
	s.tasks[t.ID] = t
	s.mu.Unlock()

	return t, s.save()
}

// Get returns a copy-by-pointer of one task, re-reading the file first if
// another process has written it since.
func (s *Store) Get(id string) (*Task, bool) {
	s.refresh()
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

// refresh re-reads the file when its modification time has moved. Cheap: the
// store is one small JSON document, and the alternative is a hook that rejects
// perfectly valid work.
func (s *Store) refresh() {
	info, err := os.Stat(s.path)
	if err != nil {
		return
	}
	modMS := info.ModTime().UnixMilli()

	s.mu.RLock()
	current := s.loadedMS
	s.mu.RUnlock()
	if modMS == current {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return
	}
	var list []*Task
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range list {
		// Keep whatever this process already has in flight: its own in-memory
		// task is newer than the file for tasks it is running right now.
		if existing, ok := s.tasks[t.ID]; ok && !existing.Terminal() && t.Terminal() {
			continue
		}
		if _, ok := s.tasks[t.ID]; !ok {
			s.tasks[t.ID] = t
		}
	}
	s.loadedMS = modMS
}

// Update applies a change under the lock and persists it.
func (s *Store) Update(id string, mutate func(*Task)) (*Task, error) {
	s.mu.Lock()
	t, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("no task %s", id)
	}
	mutate(t)
	s.mu.Unlock()
	return t, s.save()
}

// List returns tasks newest first.
func (s *Store) List() []*Task {
	s.refresh()
	s.mu.RLock()
	out := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedMS > out[j].CreatedMS })
	return out
}

// Active returns tasks that have not finished.
func (s *Store) Active() []*Task {
	var out []*Task
	for _, t := range s.List() {
		if !t.Terminal() && t.Status != StatusStale {
			out = append(out, t)
		}
	}
	return out
}

// Prune drops finished tasks beyond the newest keep, so the file does not grow
// forever. Unfinished tasks are never pruned.
func (s *Store) Prune(keep int) error {
	s.mu.Lock()
	var finished []*Task
	for _, t := range s.tasks {
		if t.Terminal() {
			finished = append(finished, t)
		}
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].FinishedMS > finished[j].FinishedMS })
	for i, t := range finished {
		if i >= keep {
			delete(s.tasks, t.ID)
		}
	}
	s.mu.Unlock()
	return s.save()
}

func (s *Store) save() error {
	s.mu.RLock()
	list := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		list = append(list, t)
	}
	s.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool { return list[i].CreatedMS < list[j].CreatedMS })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	// Write through a temp file: a half-written task store read at the next
	// start is worse than no store at all.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	if info, err := os.Stat(s.path); err == nil {
		s.mu.Lock()
		s.loadedMS = info.ModTime().UnixMilli()
		s.mu.Unlock()
	}
	return nil
}
