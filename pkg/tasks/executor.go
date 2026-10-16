// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Executor runs one task. Two shapes exist and the difference matters:
//
//   - in-process executors (internal, exec) return a result from Run
//   - remote executors (http) return ErrAsync: the work is somewhere else and
//     the answer arrives later through the completion hook
//
// Nothing here knows what Claude, Hermes or any other system is. Handing a task
// to one of them is a command line or a URL in config.
type Executor interface {
	Run(ctx context.Context, t *Task) (result string, err error)
}

// ErrAsync says the task was dispatched and will report back through the hook.
var ErrAsync = fmt.Errorf("task dispatched; result will arrive via callback")

// ExecExecutor runs a shell command. This is the general-purpose door: a task
// that should go to Claude on another host is
//
//	ssh herdr 'claude -p {{prompt}} --output-format json'
//
// and pepebot needs to know nothing about it.
type ExecExecutor struct {
	// Shell runs the command; empty means "sh -c".
	Shell   string
	Timeout time.Duration
	// MaxOutput caps what is read back, so a runaway job cannot fill memory.
	MaxOutput int
}

func (e *ExecExecutor) Run(ctx context.Context, t *Task) (string, error) {
	if strings.TrimSpace(t.Target) == "" {
		return "", fmt.Errorf("exec task needs a command in target")
	}

	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell := e.Shell
	if shell == "" {
		shell = "sh"
	}

	// The prompt is passed in the environment, never interpolated into the
	// command string: a task prompt comes from a chat message, and splicing
	// that into a shell line is how a question becomes a command.
	cmd := exec.CommandContext(ctx, shell, "-c", t.Target)
	cmd.Env = append(cmd.Environ(),
		"PEPEBOT_TASK_ID="+t.ID,
		"PEPEBOT_TASK_PROMPT="+t.Prompt,
		"PEPEBOT_TASK_LABEL="+t.Label,
	)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	limit := e.MaxOutput
	if limit <= 0 {
		limit = 256 * 1024
	}

	result := strings.TrimSpace(out.String())
	if len(result) > limit {
		result = result[:limit] + "\n…[dipotong]"
	}
	if err != nil {
		detail := strings.TrimSpace(errOut.String())
		if detail == "" {
			detail = result
		}
		return result, fmt.Errorf("%v: %s", err, truncate(detail, 500))
	}
	return result, nil
}

// HTTPExecutor hands the task to a worker somewhere else and returns ErrAsync.
// The worker is told where to send the answer and which token proves it is the
// one that was asked.
type HTTPExecutor struct {
	Client *http.Client
	// CallbackURL is pepebot's own address as the worker will see it. Pepebot
	// cannot guess this — a gateway bound to 127.0.0.1 is unreachable from
	// another host — so it is configured, and dispatch fails loudly when it is
	// missing rather than sending work that can never report back.
	CallbackURL string
}

func (e *HTTPExecutor) Run(ctx context.Context, t *Task) (string, error) {
	if strings.TrimSpace(t.Target) == "" {
		return "", fmt.Errorf("http task needs a worker URL in target")
	}
	if strings.TrimSpace(e.CallbackURL) == "" {
		return "", fmt.Errorf("tasks.callback_url is not set: a remote worker would have nowhere to report back to")
	}

	payload, _ := json.Marshal(map[string]string{
		"task_id":  t.ID,
		"label":    t.Label,
		"prompt":   t.Prompt,
		"callback": strings.TrimRight(e.CallbackURL, "/") + "/v1/tasks/" + t.ID + "/result",
		"token":    t.Token,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", t.Target, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach worker %s: %w", t.Target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("worker %s refused the task: HTTP %d", t.Target, resp.StatusCode)
	}
	return "", ErrAsync
}

// FuncExecutor adapts a plain function, which is how the agent package supplies
// the in-process executor without this package importing it.
type FuncExecutor func(ctx context.Context, t *Task) (string, error)

func (f FuncExecutor) Run(ctx context.Context, t *Task) (string, error) { return f(ctx, t) }
