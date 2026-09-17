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
)

// The system prompt carries what skills exist, never their full text. Injecting every
// SKILL.md put ~335k tokens in front of every request on a deployment with 89 skills,
// which no context window survives — and paid for all of them to use at most one.
func TestSkillsPromptCarriesSummaryNotBodies(t *testing.T) {
	workspace := t.TempDir()

	const marker = "LANGKAH-RAHASIA-TIDAK-BOLEH-MASUK-PROMPT"
	skillDir := filepath.Join(workspace, "skills", "kopi")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
name: kopi
description: Cara menyeduh kopi V60 dengan rasio yang benar.
---

# Kopi

` + marker + `

Rasio 1:16, air 92 derajat, bloom 30 detik.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	prompt := NewContextBuilder(workspace).SkillsPrompt()

	if strings.Contains(prompt, marker) {
		t.Error("the skill body reached the prompt; only the summary belongs there")
	}
	for _, want := range []string{"kopi", "menyeduh kopi V60", "SKILL.md"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	// Without this the model has a catalogue it does not know how to open.
	if !strings.Contains(prompt, "read_file") {
		t.Errorf("prompt never tells the agent how to open a skill:\n%s", prompt)
	}
}

func TestSkillsPromptEmptyWithoutSkills(t *testing.T) {
	if got := NewContextBuilder(t.TempDir()).SkillsPrompt(); got != "" {
		t.Errorf("expected nothing for a workspace with no skills, got %q", got)
	}
}
