// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

// Package tasks is pepebot's delegation: work that is too slow to answer inline,
// handed to someone else, tracked by id, and reported back to the chat that
// asked for it.
//
// "Someone else" is deliberately vague. An executor can be pepebot's own agent
// loop, a shell command, or an HTTP worker on another machine — the package
// knows none of their names. A task that goes to Claude on another host is a
// shell command in a config file, not code in here.
package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Status values. A task is only ever in one of these.
const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusTimeout  = "timeout"
	StatusCanceled = "canceled"
	// StatusStale is a task that was running when pepebot stopped. Nobody is
	// going to finish it, and it must not quietly disappear — the requester is
	// told and asked whether to run it again.
	StatusStale = "stale"
)

// MaxDepth stops a task from spawning a task from spawning a task forever.
// Three levels is enough for "do this, which needs that, which needs one lookup"
// and short of anything that could run away.
const MaxDepth = 3

// Origin remembers who asked, so the answer can find its way back.
type Origin struct {
	Channel    string `json:"channel"`
	ChatID     string `json:"chat_id"`
	SessionKey string `json:"session_key"`
	Requester  string `json:"requester,omitempty"`
}

// Task is one delegated piece of work.
type Task struct {
	ID       string `json:"id"`
	Label    string `json:"label,omitempty"`
	Prompt   string `json:"prompt"`
	Executor string `json:"executor"`
	// Target is what the executor needs beyond the prompt: a command line, a
	// worker URL, an agent name. Its meaning belongs to the executor.
	Target string `json:"target,omitempty"`

	Origin Origin `json:"origin"`
	Status string `json:"status"`
	Depth  int    `json:"depth"`

	// Result is the summary that reaches the chat. A long result is written to
	// ResultPath and only summarised here, because a chat message is not a log
	// file.
	Result     string `json:"result,omitempty"`
	ResultPath string `json:"result_path,omitempty"`
	Error      string `json:"error,omitempty"`

	// Token authenticates the completion callback for this one task. A remote
	// worker gets it with the job and sends it back; it is accepted once.
	Token string `json:"token"`

	CreatedMS  int64 `json:"created_ms"`
	StartedMS  int64 `json:"started_ms,omitempty"`
	FinishedMS int64 `json:"finished_ms,omitempty"`
}

// Terminal reports whether the task has finished, one way or another.
func (t *Task) Terminal() bool {
	switch t.Status {
	case StatusDone, StatusFailed, StatusTimeout, StatusCanceled:
		return true
	}
	return false
}

// Age is how long the task has been alive, for the status listing.
func (t *Task) Age() time.Duration {
	return time.Since(time.UnixMilli(t.CreatedMS))
}

// Describe is the one-line form used in chat and in `pepebot tasks`.
func (t *Task) Describe() string {
	name := t.Label
	if name == "" {
		name = truncate(t.Prompt, 48)
	}
	return fmt.Sprintf("%s [%s] %s", t.ID, t.Status, name)
}

// newID is short enough to type into a chat message and long enough not to
// collide: "t_a1b2c3d4".
func newID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("t_%d", time.Now().UnixNano())
	}
	return "t_" + hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tok_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
