// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package cron

import (
	"os"
	"path/filepath"
	"testing"
)

// A crash between creating the file and writing to it leaves zero bytes. That
// is a store with no jobs; treating it as corrupt took the cron service down on
// every single start with "unexpected end of JSON input".
func TestLoadStoreAcceptsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	for _, content := range []string{"", "   \n\t "} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		cs := &CronService{storePath: path}
		if err := cs.loadStore(); err != nil {
			t.Fatalf("empty store reported as broken: %v", err)
		}
		if cs.store == nil || len(cs.store.Jobs) != 0 {
			t.Fatalf("store = %+v, want an empty job list", cs.store)
		}
	}
}

func TestLoadStoreStillRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	if err := os.WriteFile(path, []byte("{bukan json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := (&CronService{storePath: path}).loadStore(); err == nil {
		t.Error("a truly corrupt store should still be reported")
	}
}

func TestLoadStoreMissingFileIsFine(t *testing.T) {
	cs := &CronService{storePath: filepath.Join(t.TempDir(), "belum-ada.json")}
	if err := cs.loadStore(); err != nil {
		t.Fatalf("missing store: %v", err)
	}
}
