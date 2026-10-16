// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LearnedSkill is a skill the agent wrote for itself, with when it did.
type LearnedSkill struct {
	Name        string
	Description string
	CreatedAt   time.Time
	Path        string
}

// Learned lists what the agent has taught itself, newest first.
//
// It exists because learning was otherwise invisible: memory and skills changed
// on disk and said so only in a server log line. "How do I know it is learning?"
// deserves an answer that is not `journalctl | grep`.
func Learned(workspace string) []LearnedSkill {
	dir := filepath.Join(workspace, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var out []LearnedSkill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "created_by: agent") {
			continue
		}

		s := LearnedSkill{Name: e.Name(), Path: path}
		for _, line := range strings.Split(content, "\n") {
			switch {
			case strings.HasPrefix(line, "description:"):
				s.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			case strings.HasPrefix(line, "created_at:"):
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(strings.TrimPrefix(line, "created_at:"))); err == nil {
					s.CreatedAt = t
				}
			case line == "---" && s.Description != "":
				// End of frontmatter; nothing useful below it for a listing.
			}
		}
		// A skill written before created_at existed still counts — fall back to
		// the file's own timestamp rather than showing a zero date.
		if s.CreatedAt.IsZero() {
			if info, err := os.Stat(path); err == nil {
				s.CreatedAt = info.ModTime()
			}
		}
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
