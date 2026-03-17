package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/config"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

// makeTestVault creates a temp dir with a minimal vault structure and config.
func makeTestVault(t *testing.T) (string, *config.RicketConfig) {
	t.Helper()
	dir := t.TempDir()

	// Create standard dirs
	for _, sub := range []string{"Inbox", "_templates", "Notes"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

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
				Name:   "note",
				Folder: "Notes/",
				Tags:   []string{"note"},
			},
		},
	}
	return dir, cfg
}

// writeNote writes a markdown file at relPath relative to dir.
func writeNote(t *testing.T, dir, relPath, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatus(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)

	// Empty vault
	status, err := v.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.InboxCount != 0 {
		t.Errorf("InboxCount = %d, want 0", status.InboxCount)
	}
	if status.Categories != 1 {
		t.Errorf("Categories = %d, want 1", status.Categories)
	}

	// Add two inbox notes
	writeNote(t, dir, "Inbox/note1.md", "# Note 1\n\nContent.")
	writeNote(t, dir, "Inbox/note2.md", "# Note 2\n\nContent.")

	status, err = v.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.InboxCount != 2 {
		t.Errorf("InboxCount = %d, want 2", status.InboxCount)
	}
	if status.TotalNotes < 2 {
		t.Errorf("TotalNotes = %d, want >= 2", status.TotalNotes)
	}
}

func TestCreateNote(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)

	t.Run("basic_create", func(t *testing.T) {
		err := v.CreateNote("Notes/my-note.md", "# My Note\n\nContent.", nil, nil, "")
		if err != nil {
			t.Fatalf("CreateNote: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "Notes", "my-note.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "Content.") {
			t.Errorf("note content missing: %s", data)
		}
	})

	t.Run("with_tags", func(t *testing.T) {
		err := v.CreateNote("Notes/tagged.md", "# Tagged\n\nBody.", []string{"foo", "bar"}, nil, "")
		if err != nil {
			t.Fatalf("CreateNote with tags: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "Notes", "tagged.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "foo") {
			t.Errorf("tag 'foo' missing from note: %s", data)
		}
	})

	t.Run("with_links", func(t *testing.T) {
		err := v.CreateNote("Notes/linked.md", "# Linked\n\nBody.", nil, []string{"OtherNote"}, "")
		if err != nil {
			t.Fatalf("CreateNote with links: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "Notes", "linked.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "[[OtherNote]]") {
			t.Errorf("wikilink missing from note: %s", data)
		}
	})

	t.Run("creates_parent_dirs", func(t *testing.T) {
		err := v.CreateNote("Projects/FCBT/new-feature.md", "Content", nil, nil, "")
		if err != nil {
			t.Fatalf("CreateNote nested: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "Projects", "FCBT", "new-feature.md")); err != nil {
			t.Errorf("expected file to exist: %v", err)
		}
	})
}

func TestFileNote(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)

	// Prepare source in Inbox
	writeNote(t, dir, "Inbox/raw-capture.md", "---\ntags: []\n---\n# Raw\n\nRaw content.")

	t.Run("basic_file", func(t *testing.T) {
		result, err := v.FileNote(vault.FileNoteOptions{
			Source:      "Inbox/raw-capture.md",
			Destination: "Notes/filed-note.md",
		})
		if err != nil {
			t.Fatalf("FileNote: %v", err)
		}
		if result.Destination != "Notes/filed-note.md" {
			t.Errorf("Destination = %q, want Notes/filed-note.md", result.Destination)
		}
		if result.GitCommitMessage == "" {
			t.Error("GitCommitMessage should not be empty")
		}
		// Source should be gone
		if _, err := os.Stat(filepath.Join(dir, "Inbox", "raw-capture.md")); !os.IsNotExist(err) {
			t.Error("source note should have been deleted")
		}
		// Destination should exist
		if _, err := os.Stat(filepath.Join(dir, "Notes", "filed-note.md")); err != nil {
			t.Errorf("destination note should exist: %v", err)
		}
	})

	t.Run("file_with_tags", func(t *testing.T) {
		writeNote(t, dir, "Inbox/to-tag.md", "# To Tag\n\nContent.")
		_, err := v.FileNote(vault.FileNoteOptions{
			Source:      "Inbox/to-tag.md",
			Destination: "Notes/tagged-note.md",
			Tags:        []string{"tagged", "important"},
		})
		if err != nil {
			t.Fatalf("FileNote with tags: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "Notes", "tagged-note.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "tagged") {
			t.Errorf("tag 'tagged' missing: %s", data)
		}
	})

	t.Run("source_not_found", func(t *testing.T) {
		_, err := v.FileNote(vault.FileNoteOptions{
			Source:      "Inbox/nonexistent.md",
			Destination: "Notes/dest.md",
		})
		if err == nil {
			t.Fatal("expected error for missing source")
		}
	})
}

func TestListInbox(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)

	notes, err := v.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox on empty inbox: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}

	writeNote(t, dir, "Inbox/a.md", "# A")
	writeNote(t, dir, "Inbox/b.md", "# B")

	notes, err = v.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestUpdateNote(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)
	defer v.Close()

	// Seed a note to update
	writeNote(t, dir, "Notes/subject.md",
		"---\ntags: [existing]\n---\n# Subject\n\nOriginal body.")

	t.Run("update_content", func(t *testing.T) {
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path:    "Notes/subject.md",
			Content: "Replaced body.",
		})
		if err != nil {
			t.Fatalf("UpdateNote content: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dir, "Notes", "subject.md"))
		if !strings.Contains(string(data), "Replaced body.") {
			t.Errorf("content not updated: %s", data)
		}
		if strings.Contains(string(data), "Original body.") {
			t.Error("old content should be gone after update")
		}
	})

	t.Run("add_tags", func(t *testing.T) {
		writeNote(t, dir, "Notes/for-tags.md", "---\ntags: [old]\n---\n# Tag test\n\nBody.")
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path: "Notes/for-tags.md",
			Tags: []string{"new1", "new2"},
		})
		if err != nil {
			t.Fatalf("UpdateNote tags: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dir, "Notes", "for-tags.md"))
		if !strings.Contains(string(data), "new1") || !strings.Contains(string(data), "new2") {
			t.Errorf("new tags missing: %s", data)
		}
		if !strings.Contains(string(data), "old") {
			t.Errorf("existing tag 'old' should be preserved: %s", data)
		}
	})

	t.Run("add_tags_deduplicates", func(t *testing.T) {
		writeNote(t, dir, "Notes/for-dedup.md", "---\ntags: [alpha]\n---\n# Dedup\n\nBody.")
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path: "Notes/for-dedup.md",
			Tags: []string{"alpha", "beta"}, // alpha already present
		})
		if err != nil {
			t.Fatal(err)
		}
		note, _ := v.ReadNote("Notes/for-dedup.md")
		tags := vault.GetTags(note.Parsed)
		alphaCount := 0
		for _, tg := range tags {
			if tg == "alpha" {
				alphaCount++
			}
		}
		if alphaCount != 1 {
			t.Errorf("expected 'alpha' exactly once, got %d times in %v", alphaCount, tags)
		}
	})

	t.Run("add_links", func(t *testing.T) {
		writeNote(t, dir, "Notes/for-links.md", "---\ntags: []\n---\n# Links test\n\nBody.")
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path:  "Notes/for-links.md",
			Links: []string{"SomeRef", "OtherRef"},
		})
		if err != nil {
			t.Fatalf("UpdateNote links: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dir, "Notes", "for-links.md"))
		if !strings.Contains(string(data), "[[SomeRef]]") {
			t.Errorf("wikilink SomeRef missing: %s", data)
		}
	})

	t.Run("returns_path", func(t *testing.T) {
		writeNote(t, dir, "Notes/path-check.md", "---\ntags: []\n---\n# X\n\nY.")
		res, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path:    "Notes/path-check.md",
			Content: "Updated.",
		})
		if err != nil {
			t.Fatalf("UpdateNote: %v", err)
		}
		if res.Path != "Notes/path-check.md" {
			t.Errorf("result.Path = %q, want Notes/path-check.md", res.Path)
		}
	})

	t.Run("missing_note", func(t *testing.T) {
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path:    "Notes/does-not-exist.md",
			Content: "content",
		})
		if err == nil {
			t.Error("expected error for missing note")
		}
	})

	t.Run("empty_update_rejected", func(t *testing.T) {
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path: "Notes/subject.md",
			// no content, tags, or links
		})
		if err == nil {
			t.Error("expected error when nothing to update")
		}
	})

	t.Run("traversal_rejected", func(t *testing.T) {
		_, err := v.UpdateNote(vault.UpdateNoteOptions{
			Path:    "../../etc/passwd",
			Content: "evil",
		})
		if err == nil {
			t.Error("expected error for traversal path")
		}
	})
}

func TestSearchNotes(t *testing.T) {
	dir, cfg := makeTestVault(t)
	v := vault.New(cfg)

	writeNote(t, dir, "Notes/alpha.md", "---\ntags: [project, active]\n---\n# Alpha\n\nAlpha content.")
	writeNote(t, dir, "Notes/beta.md", "---\ntags: [project]\n---\n# Beta\n\nBeta content.")
	writeNote(t, dir, "Inbox/raw.md", "# Raw\n\nRaw capture.")

	t.Run("by_folder", func(t *testing.T) {
		results, err := v.SearchNotes(vault.SearchOptions{Folder: "Notes/"})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2, got %d", len(results))
		}
	})

	t.Run("by_tag", func(t *testing.T) {
		results, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"active"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1, got %d", len(results))
		}
	})

	t.Run("by_query", func(t *testing.T) {
		results, err := v.SearchNotes(vault.SearchOptions{Query: "alpha content"})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1, got %d", len(results))
		}
	})

	t.Run("all_notes", func(t *testing.T) {
		results, err := v.SearchNotes(vault.SearchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) < 3 {
			t.Errorf("expected >= 3, got %d", len(results))
		}
	})
}
