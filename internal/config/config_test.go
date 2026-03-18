package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/config"
)

// writeYAML writes content to ricket.yaml in dir.
func writeYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ricket.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid_minimal", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
vault:
  inbox: Inbox/
  archive: Archive/
  templates: _templates/
categories:
  - name: decision
    folder: Areas/decisions/
    tags: [decision]
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.VaultRoot != dir {
			t.Errorf("VaultRoot = %q, want %q", cfg.VaultRoot, dir)
		}
		if len(cfg.Categories) != 1 {
			t.Errorf("len(Categories) = %d, want 1", len(cfg.Categories))
		}
		if cfg.Categories[0].Name != "decision" {
			t.Errorf("Category[0].Name = %q, want decision", cfg.Categories[0].Name)
		}
	})

	t.Run("defaults_applied", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Vault.Inbox != "Inbox/" {
			t.Errorf("default Inbox = %q, want Inbox/", cfg.Vault.Inbox)
		}
		if cfg.Vault.Archive != "Archive/" {
			t.Errorf("default Archive = %q, want Archive/", cfg.Vault.Archive)
		}
		if cfg.Vault.Templates != "_templates/" {
			t.Errorf("default Templates = %q, want _templates/", cfg.Vault.Templates)
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		dir := t.TempDir()
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for missing ricket.yaml")
		}
	})

	t.Run("invalid_yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `{{{not valid yaml`)
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("no_categories", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
vault:
  inbox: Inbox/
`)
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for missing categories")
		}
	})

	t.Run("category_missing_name", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - folder: Notes/
    tags: [note]
`)
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for category with empty name")
		}
	})

	t.Run("category_missing_folder", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    tags: [note]
`)
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for category with empty folder")
		}
	})

	t.Run("category_missing_tags", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
`)
		_, err := config.LoadConfig(dir)
		if err == nil {
			t.Fatal("expected error for category with nil tags")
		}
	})

	t.Run("mcp_metadata", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
mcp:
  name: myvault
  version: 1.0.0
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MCP == nil {
			t.Fatal("expected MCP config to be set")
		}
		if cfg.MCP.Name != "myvault" {
			t.Errorf("MCP.Name = %q, want myvault", cfg.MCP.Name)
		}
		if !cfg.MCP.RequireTriageApproval() {
			t.Error("RequireTriageApproval() = false, want true by default")
		}
	})

	t.Run("mcp_needs_approval_false", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
mcp:
  name: myvault
  version: 1.0.0
  needsApproval: false
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MCP == nil {
			t.Fatal("expected MCP config to be set")
		}
		if cfg.MCP.RequireTriageApproval() {
			t.Error("RequireTriageApproval() = true, want false")
		}
	})
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.RicketConfig{
		VaultRoot: dir,
		Vault: config.VaultConfig{
			Root:      dir,
			Inbox:     "Inbox/",
			Archive:   "Archive/",
			Templates: "_templates/",
		},
		Categories: []config.Category{
			{
				Name:    "test-decision",
				Folder:  "Areas/decisions/",
				Tags:    []string{"decision"},
				Signals: []string{"decision", "standard"},
			},
		},
	}

	if err := config.WriteConfig(cfg, dir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Round-trip: load what was written
	loaded, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig after WriteConfig: %v", err)
	}
	if len(loaded.Categories) != len(cfg.Categories) {
		t.Errorf("round-trip categories: got %d, want %d", len(loaded.Categories), len(cfg.Categories))
	}
}
