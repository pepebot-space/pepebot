// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pepebot-space/pepebot/pkg/memory"
)

func builderWithMemory(t *testing.T) (*ContextBuilder, string) {
	t.Helper()
	workspace := t.TempDir()
	cb := NewContextBuilder(workspace)
	cb.SetMemory(memory.Notes(workspace, 200), memory.Profile(workspace, 100))
	return cb, workspace
}

// The capacity header is not decoration: it is how the agent knows to
// consolidate before a write fails.
func TestMemoryPromptCarriesEntriesAndCapacity(t *testing.T) {
	cb, workspace := builderWithMemory(t)

	if err := memory.Notes(workspace, 200).Add("project utama ada di ~/Office/PEPEBOT"); err != nil {
		t.Fatalf("add note: %v", err)
	}
	if err := memory.Profile(workspace, 100).Add("user lebih suka jawaban singkat"); err != nil {
		t.Fatalf("add profile: %v", err)
	}

	prompt := cb.MemoryPrompt()
	for _, want := range []string{"PEPEBOT", "jawaban singkat", "MEMORY [", "USER PROFILE [", "/200 chars"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
}

// Bootstrap used to paste MEMORY.md and USER.md in raw and unbounded. With
// stores attached they are rendered once, by MemoryPrompt.
func TestBootstrapDoesNotAlsoInlineMemoryFiles(t *testing.T) {
	cb, workspace := builderWithMemory(t)

	const marker = "CATATAN-LAMA-TIDAK-BOLEH-DOBEL"
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "memory", "MEMORY.md"), []byte(marker), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "USER.md"), []byte("profil lama"), 0o644); err != nil {
		t.Fatalf("write user: %v", err)
	}

	if bootstrap := cb.LoadBootstrapFiles(); strings.Contains(bootstrap, marker) {
		t.Error("MEMORY.md was inlined by bootstrap as well as rendered by MemoryPrompt")
	}
	// It is still in the prompt — through the bounded path, with its header.
	if !strings.Contains(cb.MemoryPrompt(), marker) {
		t.Error("existing MEMORY.md content went missing from the prompt")
	}
}

// Nothing attached means nothing changes for callers without memory configured.
func TestMemoryPromptEmptyWithoutStores(t *testing.T) {
	if got := NewContextBuilder(t.TempDir()).MemoryPrompt(); got != "" {
		t.Errorf("MemoryPrompt = %q, want empty when no stores are attached", got)
	}
}

// Small models wrap JSON in prose or a fence no matter what the prompt says.
func TestParseReviewOpsToleratesWrapping(t *testing.T) {
	reply := "Sure!\n```json\n{\"ops\": [{\"action\": \"add\", \"target\": \"user\", \"content\": \"suka ringkas\"}]}\n```\n"

	ops, err := parseReviewOps(reply)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ops) != 1 || ops[0].Action != "add" || ops[0].Target != "user" {
		t.Fatalf("ops = %+v", ops)
	}

	if _, err := parseReviewOps("tidak ada apa-apa"); err == nil {
		t.Error("a reply with no JSON should be reported as unusable")
	}
}
