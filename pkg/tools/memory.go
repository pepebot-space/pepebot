// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/memory"
)

// MemoryTool is how the agent curates its own long-term notes.
//
// It replaces "read_file MEMORY.md, then write_file the whole thing back",
// which rewrote an entire file to change one line, raced with itself across
// concurrent sessions, and had no ceiling.
type MemoryTool struct {
	notes   *memory.Store
	profile *memory.Store
}

func NewMemoryTool(notes, profile *memory.Store) *MemoryTool {
	return &MemoryTool{notes: notes, profile: profile}
}

func (t *MemoryTool) Name() string { return "memory" }

func (t *MemoryTool) Description() string {
	return "Curate long-term memory that is loaded into every future session. " +
		"target 'memory' holds your own notes (facts about the environment, projects, conventions); " +
		"target 'user' holds the user's profile (preferences, communication style). " +
		"Both are size-limited: when one is full the call fails and you must consolidate " +
		"or remove an entry before retrying. Save durable facts, not conversation details."
}

func (t *MemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"add", "replace", "remove"},
				"description": "add a new entry, replace an existing one, or remove one",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"memory", "user"},
				"description": "which store to write to (default: memory)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "the entry text; required for add and replace",
			},
			"old_text": map[string]interface{}{
				"type":        "string",
				"description": "substring identifying the entry to replace or remove",
			},
		},
		"required": []string{"action"},
	}
}

func (t *MemoryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	content, _ := args["content"].(string)
	oldText, _ := args["old_text"].(string)

	store, label := t.notes, "memory"
	if target, _ := args["target"].(string); strings.EqualFold(target, "user") {
		store, label = t.profile, "user profile"
	}

	var err error
	switch strings.ToLower(action) {
	case "add":
		err = store.Add(content)
		if errors.Is(err, memory.ErrDuplicate) {
			return "Already stored — nothing added.", nil
		}
	case "replace":
		err = store.Replace(oldText, content)
	case "remove":
		err = store.Remove(oldText)
	default:
		return "", fmt.Errorf("unknown action %q: use add, replace or remove", action)
	}
	if err != nil {
		return "", err
	}

	used, limit := store.Usage()
	return fmt.Sprintf("Saved to %s (%d/%d chars used).", label, used, limit), nil
}
