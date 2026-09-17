// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// SKILL.md frontmatter is YAML across several lines. Two bugs stopped it ever being
// read: the extraction regex could not span lines, and what it did extract was handed
// to json.Unmarshal. Between them every skill came back with an empty description,
// `requires` never gated availability, and declared MCP servers were never registered.
func TestSkillFrontmatterIsParsed(t *testing.T) {
	workspace := t.TempDir()
	skillDir := filepath.Join(workspace, "skills", "kopi")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	body := `---
name: kopi
description: Menyeduh V60 dengan rasio yang benar.
requires:
  bins:
    - definitely-not-a-real-binary-9f3a
mcp:
  - name: kopi-api
    transport: sse
    url: https://example.invalid/sse
---

# Kopi

Rasio 1:16.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := NewSkillsLoader(workspace, "")
	all := loader.ListSkills(false)
	if len(all) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(all))
	}

	got := all[0]
	if got.Description != "Menyeduh V60 dengan rasio yang benar." {
		t.Errorf("description = %q, want the frontmatter text", got.Description)
	}

	// requires.bins names a binary that cannot exist, so the skill is unavailable —
	// which also proves the nested structure parsed, not just the scalars.
	if got.Available {
		t.Error("skill requiring a missing binary reported as available")
	}
	if got.Missing == "" {
		t.Error("nothing reported as missing for an unmet requirement")
	}

	// And filtering actually uses it.
	if available := loader.ListSkills(true); len(available) != 0 {
		t.Errorf("unavailable skill survived filtering: %+v", available)
	}
}

func TestSkillWithoutFrontmatterStillLists(t *testing.T) {
	workspace := t.TempDir()
	skillDir := filepath.Join(workspace, "skills", "polos")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Polos\n\nTanpa frontmatter.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	all := NewSkillsLoader(workspace, "").ListSkills(true)
	if len(all) != 1 || all[0].Name != "polos" {
		t.Fatalf("a skill with no frontmatter should still be listed, got %+v", all)
	}
}
