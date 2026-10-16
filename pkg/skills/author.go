// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Writing skills is how the agent keeps what it worked out. Memory holds short
// facts that ride in every prompt; a skill holds a procedure that is only read
// when it is needed — so the expensive knowledge lives where it costs nothing
// until it is used.
//
// Everything written here is marked `created_by: agent`, which is what makes it
// reversible: `pepebot skills list --agent` finds them, and removing one is a
// directory delete. Nothing in this file can touch a skill the agent did not
// write.

// maxSkillBytes caps one skill. A procedure worth keeping is a page or two; a
// model that produces more than this has misunderstood the job, and the file
// would go on to cost context in every skills summary.
const maxSkillBytes = 32 * 1024

// skillNamePattern is deliberately narrow: the name becomes a directory under
// the workspace, so anything that could climb out of it is rejected rather than
// cleaned up.
var skillNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])$`)

// AgentAuthor writes and removes agent-created skills.
type AgentAuthor struct {
	dir string
}

// NewAgentAuthor writes into the same workspace directory the loader reads.
func NewAgentAuthor(workspace string) *AgentAuthor {
	return &AgentAuthor{dir: filepath.Join(workspace, "skills")}
}

// Create writes a new skill. It refuses to overwrite an existing one: replacing
// a hand-written or installed skill is not something an autonomous turn should
// be able to do by picking a name that happens to collide.
func (a *AgentAuthor) Create(name, description, body string) error {
	if err := validName(name); err != nil {
		return err
	}
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("a skill needs a description: it is the only thing the model sees before deciding to open it")
	}
	if err := scanSkill(description + "\n" + body); err != nil {
		return err
	}

	path := filepath.Join(a.dir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("skill %q already exists; patch it instead", name)
	}

	content := renderSkill(name, description, body)
	if len(content) > maxSkillBytes {
		return fmt.Errorf("skill is %d bytes, over the %d byte limit; keep it to the procedure itself", len(content), maxSkillBytes)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644)
}

// Patch replaces the body of a skill the agent wrote. A skill it did not write
// is left alone — improving an installed skill in place would be silently
// undone the next time that skill is updated from its source.
func (a *AgentAuthor) Patch(name, description, body string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := a.mustBeAgentOwned(name); err != nil {
		return err
	}
	if err := scanSkill(description + "\n" + body); err != nil {
		return err
	}

	existing, err := os.ReadFile(filepath.Join(a.dir, name, "SKILL.md"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(description) == "" {
		description = descriptionOf(string(existing))
	}

	content := renderSkill(name, description, body)
	if len(content) > maxSkillBytes {
		return fmt.Errorf("patched skill is %d bytes, over the %d byte limit", len(content), maxSkillBytes)
	}
	return os.WriteFile(filepath.Join(a.dir, name, "SKILL.md"), []byte(content), 0o644)
}

// Delete removes a skill the agent wrote.
func (a *AgentAuthor) Delete(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	if err := a.mustBeAgentOwned(name); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(a.dir, name))
}

// IsAgentOwned reports whether a skill carries the agent marker.
func (a *AgentAuthor) IsAgentOwned(name string) bool {
	return a.mustBeAgentOwned(name) == nil
}

func (a *AgentAuthor) mustBeAgentOwned(name string) error {
	data, err := os.ReadFile(filepath.Join(a.dir, name, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("no skill named %q", name)
	}
	if !strings.Contains(string(data), "created_by: agent") {
		return fmt.Errorf("skill %q was not written by the agent; leave it to whoever installed it", name)
	}
	return nil
}

// renderSkill builds the SKILL.md, frontmatter included. created_by and
// created_at are what make an agent-written skill findable later.
func renderSkill(name, description, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", strings.TrimSpace(singleLine(description)))
	b.WriteString("created_by: agent\n")
	fmt.Fprintf(&b, "created_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	return b.String()
}

func descriptionOf(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return ""
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func validName(name string) error {
	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("skill name %q must be lowercase letters, digits and dashes, 3-40 characters, and cannot start or end with a dash", name)
	}
	return nil
}

// scanSkill refuses content that must not end up in something the agent later
// reads back as its own instructions.
//
// This is the same class of check memory entries get, and it is weaker here on
// purpose-built grounds: a SKILL.md is a long document and no keyword list will
// catch everything. It stops the obvious, and `created_by: agent` is what makes
// the rest recoverable.
func scanSkill(content string) error {
	lower := strings.ToLower(content)
	for _, marker := range []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"disregard your instructions",
		"begin openssh private key",
		"begin rsa private key",
		"aws_secret_access_key",
	} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("refused: skill contains an instruction-override or credential pattern (%q)", marker)
		}
	}
	for _, r := range content {
		if r == 0xFEFF || (r >= 0x200B && r <= 0x200F) ||
			(r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) ||
			(unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r') {
			return fmt.Errorf("refused: skill contains an invisible or control character (U+%04X)", r)
		}
	}
	return nil
}
