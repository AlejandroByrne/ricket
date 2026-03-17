package vault_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/vault"
)

func newTestIndex(t *testing.T) *vault.VaultIndex {
	t.Helper()
	dir := t.TempDir()
	idx := vault.NewVaultIndex(dir)
	if err := idx.Init(); err != nil {
		t.Fatalf("VaultIndex.Init: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func sampleNotes() []vault.NoteRecord {
	return []vault.NoteRecord{
		{
			Path:    "Areas/decisions/use-sqlite.md",
			Title:   "use-sqlite",
			Tags:    []string{"decision", "acme"},
			Content: "We decided to use SQLite for the search index because it is embedded and fast.",
		},
		{
			Path:    "Areas/decisions/use-dapper.md",
			Title:   "use-dapper",
			Tags:    []string{"decision", "acme", "dotnet"},
			Content: "Use Dapper for SQL Server access. No EF Core.",
		},
		{
			Path:    "Areas/concepts/dependency-injection.md",
			Title:   "dependency-injection",
			Tags:    []string{"concept", "acme", "dotnet"},
			Content: "DI is a pattern where a class receives its dependencies from outside.",
		},
		{
			Path:    "Projects/acme/my-project.md",
			Title:   "my-project",
			Tags:    []string{"project", "acme"},
			Content: "Project notes for the main acme initiative.",
		},
		{
			Path:    "Inbox/raw.md",
			Title:   "raw",
			Tags:    nil,
			Content: "An untagged raw capture from the inbox.",
		},
	}
}

func TestVaultIndex_Rebuild(t *testing.T) {
	idx := newTestIndex(t)

	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Rebuild again should replace, not append
	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
}

func TestVaultIndex_SearchByTags(t *testing.T) {
	idx := newTestIndex(t)
	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		tags      []string
		wantCount int
	}{
		{"single tag", []string{"acme"}, 4},
		{"two tags AND", []string{"decision", "acme"}, 2},
		{"three tags AND", []string{"decision", "acme", "dotnet"}, 1},
		{"no matches", []string{"nonexistent"}, 0},
		{"empty tags", []string{}, 0},
		{"tag with no index hit", []string{"dotnet"}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.SearchByTags(tc.tags)
			if err != nil {
				t.Fatalf("SearchByTags: %v", err)
			}
			if len(results) != tc.wantCount {
				t.Errorf("SearchByTags(%v) = %d results, want %d", tc.tags, len(results), tc.wantCount)
			}
		})
	}
}

func TestVaultIndex_SearchContent(t *testing.T) {
	idx := newTestIndex(t)
	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantSnip  bool // at least one result should have a snippet
	}{
		{"unique word", "SQLite", 1, true},
		{"common word", "acme", 1, true}, // only my-project.md has "acme" in its content body
		{"no match", "xyznonexistent", 0, false},
		{"empty query", "", 0, false},
		{"case insensitive", "sqlite", 1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := idx.SearchContent(tc.query)
			if err != nil {
				t.Fatalf("SearchContent: %v", err)
			}
			if len(results) != tc.wantCount {
				t.Errorf("SearchContent(%q) = %d results, want %d", tc.query, len(results), tc.wantCount)
			}
			if tc.wantSnip && len(results) > 0 && results[0].Snippet == "" {
				t.Errorf("expected non-empty snippet for query %q", tc.query)
			}
		})
	}
}

func TestVaultIndex_GetByFolder(t *testing.T) {
	idx := newTestIndex(t)
	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		folder    string
		wantCount int
	}{
		{"Areas/decisions/", 2},
		{"Areas/", 3},
		{"Projects/", 1},
		{"Inbox/", 1},
		{"nonexistent/", 0},
	}

	for _, tc := range tests {
		t.Run(tc.folder, func(t *testing.T) {
			results, err := idx.GetByFolder(tc.folder)
			if err != nil {
				t.Fatalf("GetByFolder(%q): %v", tc.folder, err)
			}
			if len(results) != tc.wantCount {
				t.Errorf("GetByFolder(%q) = %d, want %d", tc.folder, len(results), tc.wantCount)
			}
		})
	}
}

func TestVaultIndex_RebuildEmpty(t *testing.T) {
	idx := newTestIndex(t)

	// Rebuild with data, then rebuild with empty
	if err := idx.Rebuild(sampleNotes()); err != nil {
		t.Fatal(err)
	}
	if err := idx.Rebuild([]vault.NoteRecord{}); err != nil {
		t.Fatalf("Rebuild empty: %v", err)
	}

	results, err := idx.SearchByTags([]string{"acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results after rebuild with empty set, got %d", len(results))
	}
}

func TestVaultIndex_IndexFileExists(t *testing.T) {
	dir := t.TempDir()
	idx := vault.NewVaultIndex(dir)
	if err := idx.Init(); err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	// .ricket/index.db should exist
	dbPath := filepath.Join(dir, ".ricket", "index.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected index.db to exist at %s: %v", dbPath, err)
	}
}
