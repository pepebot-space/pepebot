// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

// Package memory is the agent's curated long-term notes: a small set of entries
// that ride along in every system prompt.
//
// The point of the package is the limit. MEMORY.md was previously rewritten
// wholesale with write_file and had no ceiling, so on a live deployment it grew
// to 14.5k characters — ~3.6k tokens paid on every single request, whether or
// not any of it was relevant. A store that is full returns an error instead of
// silently dropping the oldest entry: the agent has to consolidate or remove
// something itself, which is what keeps the notes curated rather than merely
// long.
package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// delimiter separates entries on disk. A legacy file without it is simply one
// entry, so existing MEMORY.md content keeps working untouched.
const delimiter = "\n§\n"

// ErrDuplicate reports that the content is already stored verbatim. It is not a
// failure — the caller reports it as a no-op.
var ErrDuplicate = errors.New("entry already present")

// Store is one bounded memory file.
type Store struct {
	path  string
	limit int
	label string
}

// NewStore describes a memory file. Nothing is read until it is used, so a
// store for a file that does not exist yet is valid.
func NewStore(path string, limit int, label string) *Store {
	return &Store{path: path, limit: limit, label: label}
}

func (s *Store) read() string {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Entries returns the stored entries, oldest first.
func (s *Store) Entries() []string {
	content := s.read()
	if content == "" {
		return nil
	}
	var out []string
	for _, e := range strings.Split(content, "§") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// Usage reports characters stored against the limit.
func (s *Store) Usage() (used, limit int) {
	return len([]rune(s.read())), s.limit
}

// Render is what goes into the system prompt: the entries plus a capacity
// header. The agent is told how full it is so it can consolidate before it is
// forced to, rather than discovering the ceiling only when a write fails.
func (s *Store) Render() string {
	entries := s.Entries()
	if len(entries) == 0 {
		return ""
	}

	used, limit := s.Usage()
	pct := 0
	if limit > 0 {
		pct = used * 100 / limit
	}
	header := fmt.Sprintf("%s [%d%% — %d/%d chars]", s.label, pct, used, limit)
	if used > limit {
		header += " OVER LIMIT — consolidate entries before adding more"
	}

	return header + "\n" + strings.Repeat("─", 46) + "\n" + strings.Join(entries, "\n§\n")
}

// Add appends an entry.
func (s *Store) Add(entry string) error {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return errors.New("nothing to add")
	}
	if err := scan(entry); err != nil {
		return err
	}

	entries := s.Entries()
	for _, e := range entries {
		if e == entry {
			return ErrDuplicate
		}
	}
	return s.write(append(entries, entry))
}

// Replace swaps the first entry containing oldText. Matching is by substring so
// the agent can name an entry by a fragment instead of repeating it in full.
func (s *Store) Replace(oldText, newEntry string) error {
	newEntry = strings.TrimSpace(newEntry)
	if newEntry == "" {
		return errors.New("replacement content is required")
	}
	if err := scan(newEntry); err != nil {
		return err
	}

	entries := s.Entries()
	i := indexOf(entries, oldText)
	if i < 0 {
		return fmt.Errorf("no entry contains %q", truncate(oldText, 60))
	}
	entries[i] = newEntry
	return s.write(entries)
}

// Remove drops the first entry containing oldText.
func (s *Store) Remove(oldText string) error {
	entries := s.Entries()
	i := indexOf(entries, oldText)
	if i < 0 {
		return fmt.Errorf("no entry contains %q", truncate(oldText, 60))
	}
	return s.write(append(entries[:i], entries[i+1:]...))
}

// write persists entries, refusing anything past the limit.
//
// Deliberately no auto-compaction: dropping the oldest entry to make room would
// throw away something the user asked to be remembered, silently and at the
// worst possible moment. An error hands the decision back to the agent, which
// can consolidate two entries into one in the same turn and retry.
func (s *Store) write(entries []string) error {
	content := strings.Join(entries, delimiter)
	// A store that is already over the limit — every deployment that predates
	// this package — must still be able to shrink, or it would be frozen at its
	// current size forever with no way for the agent to dig out.
	if n, before := len([]rune(content)), len([]rune(s.read())); n > s.limit && n >= before {
		return fmt.Errorf("%s is full (%d/%d chars). Consolidate or remove an entry, then retry",
			s.label, n, s.limit)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte(content+"\n"), 0o644)
}

func indexOf(entries []string, needle string) int {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return -1
	}
	for i, e := range entries {
		if strings.Contains(e, needle) {
			return i
		}
	}
	return -1
}

// scan rejects content that must never reach a system prompt.
//
// Memory is written from conversation, and conversation carries whatever a
// channel delivered — so an attacker who can message the bot could otherwise
// park instructions in a file the agent reads as its own notes on every later
// turn. That is a persistent prompt injection, so this check is at the write
// boundary rather than at render time.
var injectionMarkers = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"disregard your instructions",
	"disregard all prior",
	"you are now",
	"begin openssh private key",
	"begin rsa private key",
	"authorized_keys",
}

func scan(entry string) error {
	lower := strings.ToLower(entry)
	for _, m := range injectionMarkers {
		if strings.Contains(lower, m) {
			return fmt.Errorf("refused: entry contains an instruction-override or credential pattern (%q)", m)
		}
	}
	for _, r := range entry {
		// Zero-width and bidi-override runes hide text from whoever reviews the
		// file while the model still reads it.
		if r == 0xFEFF || (r >= 0x200B && r <= 0x200F) ||
			(r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) ||
			(unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r') {
			return fmt.Errorf("refused: entry contains an invisible or control character (U+%04X)", r)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Default limits, in characters. Roughly 800 and 500 tokens: enough for the
// handful of facts worth carrying into every session, small enough that nobody
// notices them in the bill.
const (
	DefaultNotesLimit   = 2200
	DefaultProfileLimit = 1375
)

// Notes is the agent's own long-term notes, at the path pepebot has always used.
func Notes(workspace string, limit int) *Store {
	if limit <= 0 {
		limit = DefaultNotesLimit
	}
	return NewStore(filepath.Join(workspace, "memory", "MEMORY.md"), limit, "MEMORY")
}

// Profile is the user profile, likewise where it already lives.
func Profile(workspace string, limit int) *Store {
	if limit <= 0 {
		limit = DefaultProfileLimit
	}
	return NewStore(filepath.Join(workspace, "USER.md"), limit, "USER PROFILE")
}
