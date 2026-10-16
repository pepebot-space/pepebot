// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Learning was invisible: it changed files and said so only in a server log.
// This is the listing that answers "how do I know it is learning".
func TestLearnedListsOnlyAgentSkillsNewestFirst(t *testing.T) {
	ws := t.TempDir()
	author := NewAgentAuthor(ws)
	if err := os.MkdirAll(filepath.Join(ws, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := author.Create("lama", "Yang lebih dulu.", "isi"); err != nil {
		t.Fatalf("create: %v", err)
	}
	time.Sleep(1100 * time.Millisecond) // created_at has second resolution
	if err := author.Create("baru", "Yang terbaru.", "isi"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// A hand-installed skill must not appear: the listing is about what the
	// agent taught itself, not what someone installed for it.
	foreign := filepath.Join(ws, "skills", "dipasang")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"),
		[]byte("---\nname: dipasang\ndescription: Dari hub.\n---\n\nIsi.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := Learned(ws)
	if len(got) != 2 {
		t.Fatalf("got %d learned skills, want 2: %+v", len(got), got)
	}
	if got[0].Name != "baru" || got[1].Name != "lama" {
		t.Errorf("order = %s, %s; want newest first", got[0].Name, got[1].Name)
	}
	if got[0].Description != "Yang terbaru." {
		t.Errorf("description = %q", got[0].Description)
	}
	if got[0].CreatedAt.IsZero() {
		t.Error("created_at missing")
	}
}

func TestLearnedOnEmptyWorkspace(t *testing.T) {
	if got := Learned(t.TempDir()); len(got) != 0 {
		t.Errorf("got %+v, want nothing", got)
	}
}
