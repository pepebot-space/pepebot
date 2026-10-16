// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/tasks"
)

// TaskTool is how the agent hands work off and keeps track of it.
//
// The point is the chat does not have to wait. A job that takes minutes gets an
// id immediately, runs somewhere else, and comes back as a message in this same
// conversation when it is done.
type TaskTool struct {
	manager  *tasks.Manager
	routes   map[string]string // label -> "executor:target", from config
	defaultX string
}

// TaskRoute names a place work can be sent. The agent picks a route by name;
// what is behind it — a shell command, a worker URL, pepebot itself — is the
// operator's business, not the model's.
type TaskRoute struct {
	Executor string
	Target   string
}

func NewTaskTool(manager *tasks.Manager, routes map[string]TaskRoute, defaultExecutor string) *TaskTool {
	flat := map[string]string{}
	for name, r := range routes {
		flat[name] = r.Executor + "\x00" + r.Target
	}
	if defaultExecutor == "" {
		defaultExecutor = "internal"
	}
	return &TaskTool{manager: manager, routes: flat, defaultX: defaultExecutor}
}

func (t *TaskTool) Name() string { return "task" }

func (t *TaskTool) Description() string {
	routes := t.routeNames()
	where := "Routes: " + strings.Join(routes, ", ") + ". "
	if len(routes) == 0 {
		where = ""
	}
	return "Delegate work that takes too long to answer inline, then get on with the conversation. " +
		"action=delegate returns a task id immediately and the result arrives in this chat when it is done; " +
		"action=status checks one or lists the active ones; action=cancel drops one; " +
		"action=resume re-runs a task that a restart interrupted. " + where +
		"Use it for jobs measured in minutes, not for questions you can answer now."
}

func (t *TaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type": "string", "enum": []string{"delegate", "status", "cancel", "resume"},
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "what the worker should do, written so it makes sense without this conversation",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "a few words naming the job, shown to the user with its id",
			},
			"route": map[string]interface{}{
				"type":        "string",
				"description": "where to send it; omit for the default. " + strings.Join(t.routeNames(), ", "),
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "for status, cancel and resume",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	id, _ := args["task_id"].(string)

	switch strings.ToLower(action) {
	case "delegate":
		return t.delegate(ctx, args)

	case "status":
		if id != "" {
			task, ok := t.manager.Get(id)
			if !ok {
				return "", fmt.Errorf("no task %s", id)
			}
			return describe(task), nil
		}
		active := t.manager.Active()
		if len(active) == 0 {
			return "No tasks running.", nil
		}
		var b strings.Builder
		for _, task := range active {
			fmt.Fprintf(&b, "%s\n", task.Describe())
		}
		return b.String(), nil

	case "cancel":
		if err := t.manager.Cancel(id); err != nil {
			return "", err
		}
		return fmt.Sprintf("Task %s cancelled. Work already running elsewhere is not killed, but nothing will be reported.", id), nil

	case "resume":
		if err := t.manager.Resume(id); err != nil {
			return "", err
		}
		return fmt.Sprintf("Task %s is running again.", id), nil

	default:
		return "", fmt.Errorf("unknown action %q: use delegate, status, cancel or resume", action)
	}
}

func (t *TaskTool) delegate(ctx context.Context, args map[string]interface{}) (string, error) {
	prompt, _ := args["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("a task needs a prompt")
	}
	label, _ := args["label"].(string)

	executor, target := t.defaultX, ""
	if route, _ := args["route"].(string); route != "" {
		spec, ok := t.routes[route]
		if !ok {
			return "", fmt.Errorf("no route named %q; available: %s", route, strings.Join(t.routeNames(), ", "))
		}
		parts := strings.SplitN(spec, "\x00", 2)
		executor, target = parts[0], parts[1]
	}

	// The origin travels with the task: it is the only way the answer finds its
	// way back to this conversation once the turn that asked for it is over.
	// A session key is "<channel>:<chat id>", which is all the routing needs.
	sessionKey := SessionKeyFromContext(ctx)
	channel, chatID, _ := strings.Cut(sessionKey, ":")

	// A task delegated from inside another task runs one level deeper. Reading
	// the parent's depth here is what keeps the cap honest without threading a
	// counter through every call.
	depth := 0
	if parentID, ok := strings.CutPrefix(sessionKey, "task:"); ok {
		if parent, found := t.manager.Get(parentID); found {
			depth = parent.Depth
		}
		channel, chatID = "", ""
		if parent, found := t.manager.Get(parentID); found {
			channel, chatID = parent.Origin.Channel, parent.Origin.ChatID
		}
	}

	created, err := t.manager.Delegate(&tasks.Task{
		Prompt:   prompt,
		Label:    label,
		Executor: executor,
		Target:   target,
		Origin: tasks.Origin{
			Channel:    channel,
			ChatID:     chatID,
			SessionKey: sessionKey,
		},
		Depth: depth + 1,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Delegated as `%s`%s. It runs in the background; I will report here when it finishes. "+
		"Tell the user the id so they can ask about it.",
		created.ID, labelSuffix(label)), nil
}

func (t *TaskTool) routeNames() []string {
	names := make([]string, 0, len(t.routes))
	for n := range t.routes {
		names = append(names, n)
	}
	return names
}

func describe(t *tasks.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nstatus: %s, age: %s", t.Describe(), t.Status, t.Age().Round(1e9))
	if t.Error != "" {
		fmt.Fprintf(&b, "\nerror: %s", t.Error)
	}
	if t.Result != "" {
		fmt.Fprintf(&b, "\nresult: %s", t.Result)
	}
	return b.String()
}

func labelSuffix(label string) string {
	if label == "" {
		return ""
	}
	return " (" + label + ")"
}
