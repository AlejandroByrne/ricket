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

func TestLoadConfig_Sources(t *testing.T) {
	t.Run("absolute_path", func(t *testing.T) {
		srcDir := t.TempDir()
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
sources:
  - name: standards
    path: `+srcDir+`
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("len(Sources) = %d, want 1", len(cfg.Sources))
		}
		if cfg.Sources[0].Name != "standards" {
			t.Errorf("Name = %q, want standards", cfg.Sources[0].Name)
		}
		if cfg.Sources[0].ResolvedPath != filepath.Clean(srcDir) {
			t.Errorf("ResolvedPath = %q, want %q", cfg.Sources[0].ResolvedPath, filepath.Clean(srcDir))
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
sources:
  - name: shared
    path: ../shared-standards
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("len(Sources) = %d, want 1", len(cfg.Sources))
		}
		want := filepath.Clean(filepath.Join(dir, "..", "shared-standards"))
		if cfg.Sources[0].ResolvedPath != want {
			t.Errorf("ResolvedPath = %q, want %q", cfg.Sources[0].ResolvedPath, want)
		}
	})

	t.Run("skip_empty_name_or_path", func(t *testing.T) {
		dir := t.TempDir()
		writeYAML(t, dir, `
categories:
  - name: note
    folder: Notes/
    tags: [note]
sources:
  - name: ""
    path: /some/path
  - name: valid
    path: ""
`)
		cfg, err := config.LoadConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Sources) != 0 {
			t.Errorf("len(Sources) = %d, want 0 (both should be skipped)", len(cfg.Sources))
		}
	})

	t.Run("no_sources", func(t *testing.T) {
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
		if len(cfg.Sources) != 0 {
			t.Errorf("len(Sources) = %d, want 0", len(cfg.Sources))
		}
	})
}

func TestWriteConfig_Sources(t *testing.T) {
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
			{Name: "note", Folder: "Notes/", Tags: []string{"note"}},
		},
		Sources: []config.Source{
			{Name: "standards", Path: "/shared/standards", ResolvedPath: "/shared/standards"},
		},
	}
	if err := config.WriteConfig(cfg, dir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	loaded, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig after WriteConfig: %v", err)
	}
	if len(loaded.Sources) != 1 {
		t.Fatalf("round-trip sources: got %d, want 1", len(loaded.Sources))
	}
	if loaded.Sources[0].Name != "standards" {
		t.Errorf("round-trip Name = %q, want standards", loaded.Sources[0].Name)
	}
	if loaded.Sources[0].Path != "/shared/standards" {
		t.Errorf("round-trip Path = %q, want /shared/standards", loaded.Sources[0].Path)
	}
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
