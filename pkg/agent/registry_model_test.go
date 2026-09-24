// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"testing"

	"github.com/pepebot-space/pepebot/pkg/config"
)

func testConfig(t *testing.T, provider, model string) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Provider = provider
	cfg.Agents.Defaults.Model = model
	return cfg
}

func loadedRegistry(t *testing.T, cfg *config.Config) *AgentRegistry {
	t.Helper()
	ar := NewAgentRegistry(cfg.WorkspacePath())
	if err := ar.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return ar
}

// A fresh install has nothing to reconcile: config.json seeds the registry, in
// the canonical spelling.
func TestFreshInstallSeedsRegistryFromConfig(t *testing.T) {
	cfg := testConfig(t, "zai", "glm-4.5v")
	ar := loadedRegistry(t, cfg)

	if err := ar.ReconcileModel(cfg, nil); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	def := ar.Agents["default"]
	if def == nil {
		t.Fatal("no default agent was created")
	}
	if def.Model != "zai:glm-4.5v" {
		t.Errorf("model = %q, want the canonical zai:glm-4.5v", def.Model)
	}
	if def.Provider != "" {
		t.Errorf("provider = %q, want it folded into the model reference", def.Provider)
	}
}

// The registry is the source of truth, so with no terminal to ask at it wins —
// a gateway under systemd must not block on a question nobody can answer.
func TestDisagreementWithoutPromptKeepsRegistry(t *testing.T) {
	cfg := testConfig(t, "", "zai:glm-4.6")
	ar := loadedRegistry(t, cfg)
	ar.Agents["default"] = &AgentDefinition{Enabled: true, Model: "zai:glm-4.5v"}

	if err := ar.ReconcileModel(cfg, nil); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := ar.Agents["default"].Model; got != "zai:glm-4.5v" {
		t.Errorf("model = %q, want the registry value kept", got)
	}
}

func TestDisagreementAsksAndCanOverwrite(t *testing.T) {
	cfg := testConfig(t, "", "zai:glm-4.6")
	ar := loadedRegistry(t, cfg)
	ar.Agents["default"] = &AgentDefinition{Enabled: true, Model: "zai:glm-4.5v"}

	var askedRegistry, askedConfig string
	ask := func(registryModel, configModel string) (bool, error) {
		askedRegistry, askedConfig = registryModel, configModel
		return true, nil
	}
	if err := ar.ReconcileModel(cfg, ask); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if askedRegistry != "zai:glm-4.5v" || askedConfig != "zai:glm-4.6" {
		t.Errorf("prompt saw (%q, %q), want both real values", askedRegistry, askedConfig)
	}
	if got := ar.Agents["default"].Model; got != "zai:glm-4.6" {
		t.Errorf("model = %q, want the config value after saying yes", got)
	}
}

func TestDisagreementDeclinedKeepsRegistry(t *testing.T) {
	cfg := testConfig(t, "", "zai:glm-4.6")
	ar := loadedRegistry(t, cfg)
	ar.Agents["default"] = &AgentDefinition{Enabled: true, Model: "zai:glm-4.5v"}

	ask := func(string, string) (bool, error) { return false, nil }
	if err := ar.ReconcileModel(cfg, ask); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := ar.Agents["default"].Model; got != "zai:glm-4.5v" {
		t.Errorf("model = %q, want the registry value kept", got)
	}
}

// The legacy split pair and the canonical form mean the same thing, so they are
// not a disagreement — the entry is just rewritten to the canonical spelling.
func TestLegacySpellingIsMigratedNotQueried(t *testing.T) {
	cfg := testConfig(t, "maiarouter", "zai/glm-4.5v")
	ar := loadedRegistry(t, cfg)
	ar.Agents["default"] = &AgentDefinition{Enabled: true, Provider: "maiarouter", Model: "zai/glm-4.5v"}

	ask := func(string, string) (bool, error) {
		t.Error("identical models must not prompt")
		return false, nil
	}
	if err := ar.ReconcileModel(cfg, ask); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	def := ar.Agents["default"]
	if def.Model != "maiarouter:zai/glm-4.5v" || def.Provider != "" {
		t.Errorf("entry = {provider %q, model %q}, want the canonical single field", def.Provider, def.Model)
	}
}

// The reconciled value must survive a restart, or the question gets asked again
// every single run.
func TestReconciledModelIsPersisted(t *testing.T) {
	cfg := testConfig(t, "", "zai:glm-4.6")
	ar := loadedRegistry(t, cfg)
	ar.Agents["default"] = &AgentDefinition{Enabled: true, Model: "zai:glm-4.5v"}

	if err := ar.ReconcileModel(cfg, func(string, string) (bool, error) { return true, nil }); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := loadedRegistry(t, cfg).Agents["default"].Model; got != "zai:glm-4.6" {
		t.Errorf("after reload model = %q, want the saved zai:glm-4.6", got)
	}
}
