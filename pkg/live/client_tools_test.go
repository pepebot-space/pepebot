// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"strings"
	"testing"
)

func TestNamespaceClientTools(t *testing.T) {
	gateway := map[string]bool{"exec": true, "read_file": true}

	defs, names, err := namespaceClientTools("rover", []map[string]interface{}{
		{"name": "take_photo", "description": "Take a photo", "parameters": map[string]interface{}{"type": "object"}},
		{"name": "drive"},
	}, gateway)
	if err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}

	if defs[0]["name"] != "rover-take_photo" {
		t.Errorf("name = %v, want rover-take_photo", defs[0]["name"])
	}
	if defs[0]["type"] != "function" || defs[0]["description"] != "Take a photo" || defs[0]["parameters"] == nil {
		t.Errorf("definition lost fields: %#v", defs[0])
	}
	// A tool with no description or parameters must not carry empty keys upstream.
	if _, has := defs[1]["description"]; has {
		t.Errorf("empty description should be omitted: %#v", defs[1])
	}

	if names["rover-take_photo"] != "take_photo" || names["rover-drive"] != "drive" {
		t.Errorf("name map = %#v", names)
	}

	// The separator makes collisions structurally impossible against today's gateway
	// tools — they are all snake_case, so "read-file" cannot become "read_file". The
	// guard exists for a future gateway tool that does contain a hyphen, so test it
	// against exactly that rather than pretending the two forms collide.
	if _, _, err := namespaceClientTools("rover", []map[string]interface{}{{"name": "drive"}},
		map[string]bool{"rover-drive": true}); err == nil {
		t.Error("expected a name already taken by a gateway tool to be rejected")
	}
}

func TestNamespaceClientTools_Rejects(t *testing.T) {
	cases := map[string]struct {
		app   string
		tools []map[string]interface{}
		want  string
	}{
		"no app name":         {"", []map[string]interface{}{{"name": "ok"}}, "setup.app"},
		"app with separator":  {"my-app", []map[string]interface{}{{"name": "ok"}}, "setup.app"},
		"tool with separator": {"rover", []map[string]interface{}{{"name": "take-photo"}}, "must match"},
		"nameless tool":       {"rover", []map[string]interface{}{{"description": "x"}}, "must match"},
		"duplicate tool":      {"rover", []map[string]interface{}{{"name": "drive"}, {"name": "drive"}}, "declared twice"},
	}

	for label, c := range cases {
		_, _, err := namespaceClientTools(c.app, c.tools, nil)
		if err == nil {
			t.Errorf("%s: expected an error", label)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not mention %q", label, err, c.want)
		}
	}
}

func TestNamespaceClientTools_Empty(t *testing.T) {
	// No tools declared is the normal case and must not require an app name.
	defs, names, err := namespaceClientTools("", nil, nil)
	if err != nil || defs != nil || names != nil {
		t.Errorf("expected a clean no-op, got %v / %v / %v", defs, names, err)
	}
}

func TestSessionClientToolLookup(t *testing.T) {
	s := &LiveSession{clientTools: map[string]string{"rover-take_photo": "take_photo"}}

	if bare, ok := s.clientTool("rover-take_photo"); !ok || bare != "take_photo" {
		t.Errorf("client tool lookup = %q, %v", bare, ok)
	}
	// Gateway tools must not be mistaken for client ones.
	if _, ok := s.clientTool("exec"); ok {
		t.Error("exec should not be treated as a client tool")
	}
}
