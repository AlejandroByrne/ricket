// Package vault_test — integration tests exercising full triage workflows,
// git audit trails, SQLite index behaviour, and edge cases against the
// testdata/vault fixture.
package vault_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/config"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// testdataVaultPath returns the absolute path to testdata/vault relative to
// this test file (which is in internal/vault/).
func testdataVaultPath(t *testing.T) string {
	t.Helper()
	// __file__ → internal/vault/integration_test.go
	// testdata is at repo-root/testdata/vault
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "vault"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// loadTestVault loads the config for testdata/vault and returns a ready Vault.
func loadTestVault(t *testing.T) *vault.Vault {
	t.Helper()
	cfgPath := testdataVaultPath(t)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig(testdata/vault): %v", err)
	}
	v := vault.New(cfg)
	t.Cleanup(func() { v.Close() })
	return v
}

// copyVaultToTemp copies the testdata/vault into a fresh temp dir so mutation
// tests don't modify the tracked fixture.
func copyVaultToTemp(t *testing.T) (string, *vault.Vault) {
	t.Helper()
	src := testdataVaultPath(t)
	dst := t.TempDir()

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	cfg, err := config.LoadConfig(dst)
	if err != nil {
		t.Fatalf("LoadConfig(copy): %v", err)
	}
	v := vault.New(cfg)
	t.Cleanup(func() { v.Close() })
	return dst, v
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// initGitRepo turns a directory into a git repo with a single initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("add", "-A")
	run("commit", "-m", "initial vault snapshot", "--no-gpg-sign")
}

// ── testdata/vault fixture read-only tests ────────────────────────────────────

func TestFixture_LoadConfig(t *testing.T) {
	_ = loadTestVault(t) // just ensure it loads without error
}

func TestFixture_Status(t *testing.T) {
	v := loadTestVault(t)
	status, err := v.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.InboxCount != 3 {
		t.Errorf("InboxCount = %d, want 3", status.InboxCount)
	}
	if status.TotalNotes < 10 {
		t.Errorf("TotalNotes = %d, want >= 10", status.TotalNotes)
	}
	if status.Categories != 5 {
		t.Errorf("Categories = %d, want 5", status.Categories)
	}
}

func TestFixture_ReadNote(t *testing.T) {
	v := loadTestVault(t)

	note, err := v.ReadNote("Areas/Engineering/decisions/use-sqlite-for-index.md")
	if err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if note.Name != "use-sqlite-for-index" {
		t.Errorf("Name = %q, want use-sqlite-for-index", note.Name)
	}
	tags := vault.GetTags(note.Parsed)
	if !containsStr(tags, "decision") {
		t.Errorf("expected 'decision' tag, got %v", tags)
	}
}

func TestFixture_ReadNote_PathTraversal(t *testing.T) {
	v := loadTestVault(t)

	cases := []string{
		"../../etc/passwd",
		"../../../tmp/evil",
		"/etc/passwd",
		"Inbox/../../../etc/passwd",
	}
	for _, p := range cases {
		_, err := v.ReadNote(p)
		if err == nil {
			t.Errorf("ReadNote(%q): expected error for traversal, got nil", p)
		}
	}
}

func TestFixture_ListInbox(t *testing.T) {
	v := loadTestVault(t)

	notes, err := v.ListInbox()
	if err != nil {
		t.Fatalf("ListInbox: %v", err)
	}
	if len(notes) != 3 {
		t.Errorf("ListInbox = %d notes, want 3", len(notes))
	}
	// All paths should be under Inbox/
	for _, n := range notes {
		if !strings.HasPrefix(n.Path, "Inbox/") {
			t.Errorf("inbox note path %q doesn't start with Inbox/", n.Path)
		}
	}
}

func TestFixture_SearchByFolder(t *testing.T) {
	v := loadTestVault(t)

	results, err := v.SearchNotes(vault.SearchOptions{Folder: "Areas/Engineering/decisions/"})
	if err != nil {
		t.Fatalf("SearchNotes by folder: %v", err)
	}
	// Should have MOC + 2 decision notes
	if len(results) < 2 {
		t.Errorf("expected >= 2 notes in decisions folder, got %d", len(results))
	}
}

func TestFixture_SearchByTags(t *testing.T) {
	v := loadTestVault(t)

	results, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"decision", "acme"}})
	if err != nil {
		t.Fatalf("SearchNotes by tags: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected >= 2 decision+acme notes, got %d", len(results))
	}
	for _, n := range results {
		tags := vault.GetTags(n.Parsed)
		if !containsStr(tags, "decision") || !containsStr(tags, "acme") {
			t.Errorf("note %q missing expected tags (got %v)", n.Path, tags)
		}
	}
}

func TestFixture_SearchByQuery(t *testing.T) {
	v := loadTestVault(t)

	results, err := v.SearchNotes(vault.SearchOptions{Query: "SQLite"})
	if err != nil {
		t.Fatalf("SearchNotes by query: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result for query 'SQLite'")
	}
}

func TestFixture_GetTemplateList(t *testing.T) {
	v := loadTestVault(t)

	names, err := v.GetTemplateList()
	if err != nil {
		t.Fatalf("GetTemplateList: %v", err)
	}
	if len(names) < 4 {
		t.Errorf("expected >= 4 templates, got %d: %v", len(names), names)
	}
	want := []string{"decision", "concept", "meeting", "project", "learning"}
	for _, w := range want {
		if !containsStr(names, w) {
			t.Errorf("expected template %q in list %v", w, names)
		}
	}
}

func TestFixture_GetCategories(t *testing.T) {
	v := loadTestVault(t)
	cats := v.GetCategories()
	if len(cats) != 5 {
		t.Errorf("expected 5 categories, got %d", len(cats))
	}
}

// ── mutation tests (use temp copy) ───────────────────────────────────────────

func TestTriage_FileRawCapture(t *testing.T) {
	_, v := copyVaultToTemp(t)

	result, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
		Tags:        []string{"decision", "acme"},
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}
	if result.Destination != "Areas/Engineering/decisions/use-query-builder.md" {
		t.Errorf("Destination = %q", result.Destination)
	}
	if result.GitCommitMessage == "" {
		t.Error("GitCommitMessage should not be empty")
	}
}

func TestTriage_FileMeetingDraft_WithTemplate(t *testing.T) {
	_, v := copyVaultToTemp(t)

	result, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/meeting-draft.md",
		Destination: "Areas/Engineering/meetings/2026-03-17-sprint-planning.md",
		Template:    "meeting",
		Tags:        []string{"meeting", "acme"},
		MOC:         "", // no MOC for this test
	})
	if err != nil {
		t.Fatalf("FileNote with template: %v", err)
	}

	// Re-read the filed note
	filed, err := v.ReadNote(result.Destination)
	if err != nil {
		t.Fatalf("ReadNote after FileNote: %v", err)
	}

	// Tags should be present
	tags := vault.GetTags(filed.Parsed)
	if !containsStr(tags, "meeting") || !containsStr(tags, "acme") {
		t.Errorf("expected meeting+acme tags, got %v", tags)
	}

	// Original meeting content should be merged (Action Items section exists in draft)
	if !strings.Contains(filed.Parsed.Content, "alice") {
		t.Errorf("expected original content to be merged, got: %s", filed.Parsed.Content[:200])
	}
}

func TestTriage_FileLearningNote_WithLinks(t *testing.T) {
	_, v := copyVaultToTemp(t)

	result, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/learning-golang-channels.md",
		Destination: "Areas/Personal Development/golang-channels.md",
		Template:    "learning",
		Tags:        []string{"learning", "golang"},
		Links:       []string{"Goroutines and Concurrency", "Go Concurrency Patterns"},
	})
	if err != nil {
		t.Fatalf("FileNote learning: %v", err)
	}

	filed, err := v.ReadNote(result.Destination)
	if err != nil {
		t.Fatalf("ReadNote: %v", err)
	}

	links := vault.ExtractWikilinks(filed.Parsed.Content)
	if !containsStr(links, "Goroutines and Concurrency") {
		t.Errorf("expected wikilink 'Goroutines and Concurrency', found %v", links)
	}
	if !containsStr(links, "Go Concurrency Patterns") {
		t.Errorf("expected wikilink 'Go Concurrency Patterns', found %v", links)
	}
}

func TestTriage_SourceGoneAfterFile(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}

	srcAbs := filepath.Join(dir, "Inbox", "raw-capture.md")
	if _, err := os.Stat(srcAbs); !os.IsNotExist(err) {
		t.Error("source note should have been deleted after FileNote")
	}
}

func TestTriage_DestinationCreatedAfterFile(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}

	dstAbs := filepath.Join(dir, "Areas", "Engineering", "decisions", "use-query-builder.md")
	if _, err := os.Stat(dstAbs); err != nil {
		t.Errorf("destination note should exist: %v", err)
	}
}

func TestTriage_InboxCountDecreases(t *testing.T) {
	_, v := copyVaultToTemp(t)

	before, err := v.ListInbox()
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}

	after, err := v.ListInbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)-1 {
		t.Errorf("inbox: before=%d after=%d, expected decrease by 1", len(before), len(after))
	}
}

func TestTriage_MOCUpdated(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	mocPath := "Areas/Engineering/decisions/MOC.md"
	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
		MOC:         mocPath,
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}

	mocData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(mocPath)))
	if err != nil {
		t.Fatalf("ReadFile MOC: %v", err)
	}
	if !strings.Contains(string(mocData), "use-query-builder") &&
		!strings.Contains(string(mocData), "raw-capture") {
		t.Errorf("MOC should mention the new note, got:\n%s", mocData)
	}
}

func TestTriage_FileNote_MissingSource(t *testing.T) {
	_, v := copyVaultToTemp(t)

	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/does-not-exist.md",
		Destination: "Notes/dest.md",
	})
	if err == nil {
		t.Error("expected error for missing source note")
	}
}

func TestTriage_FileNote_TraversalInSource(t *testing.T) {
	_, v := copyVaultToTemp(t)

	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "../../etc/passwd",
		Destination: "Notes/dest.md",
	})
	if err == nil {
		t.Error("expected error for traversal source path")
	}
}

func TestTriage_FileNote_TraversalInDestination(t *testing.T) {
	_, v := copyVaultToTemp(t)

	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "../../etc/evil.md",
	})
	if err == nil {
		t.Error("expected error for traversal destination path")
	}
}

func TestTriage_CreateNote_ThenSearch(t *testing.T) {
	_, v := copyVaultToTemp(t)

	err := v.CreateNote(
		"Areas/Engineering/concepts/opentelemetry.md",
		"---\ntags: [concept, acme]\n---\n# OpenTelemetry\n\nVendor-neutral observability framework.",
		nil, nil, "",
	)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	// Index is now dirty; SearchNotes should rebuild and find the new note
	results, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"concept", "acme"}})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	if !containsStr(paths, "Areas/Engineering/concepts/opentelemetry.md") {
		t.Errorf("expected opentelemetry.md in search results after create, got %v", paths)
	}
}

func TestTriage_IndexDirtyAfterFileNote(t *testing.T) {
	_, v := copyVaultToTemp(t)

	// First search to build index
	before, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"decision"}})
	if err != nil {
		t.Fatalf("SearchNotes before: %v", err)
	}

	// File a decision note
	_, err = v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
		Tags:        []string{"decision", "acme"},
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}

	// Index should be dirty; next search rebuilds and finds the new note
	after, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"decision"}})
	if err != nil {
		t.Fatalf("SearchNotes after: %v", err)
	}
	if len(after) <= len(before) {
		t.Errorf("expected more decision notes after filing: before=%d after=%d", len(before), len(after))
	}
}

// ── UpdateNote integration ─────────────────────────────────────────────────────

func TestUpdateNote_ContentPersisted(t *testing.T) {
	_, v := copyVaultToTemp(t)

	result, err := v.UpdateNote(vault.UpdateNoteOptions{
		Path:    "Areas/Engineering/decisions/use-sqlite-for-index.md",
		Content: "# Rewritten\n\nFully rewritten content.",
	})
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	if result.Path != "Areas/Engineering/decisions/use-sqlite-for-index.md" {
		t.Errorf("result.Path = %q", result.Path)
	}

	note, err := v.ReadNote("Areas/Engineering/decisions/use-sqlite-for-index.md")
	if err != nil {
		t.Fatalf("ReadNote after update: %v", err)
	}
	if !strings.Contains(note.Parsed.Content, "Fully rewritten content.") {
		t.Errorf("content not persisted: %q", note.Parsed.Content)
	}
}

func TestUpdateNote_IndexDirtyAfterUpdate(t *testing.T) {
	_, v := copyVaultToTemp(t)

	// Seed index
	before, err := v.SearchNotes(vault.SearchOptions{Query: "uniquekeyword99"})
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("expected 0 results before update, got %d", len(before))
	}

	_, err = v.UpdateNote(vault.UpdateNoteOptions{
		Path:    "Areas/Engineering/decisions/use-sqlite-for-index.md",
		Content: "# Updated\n\nThis note now contains uniquekeyword99.",
	})
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	// Index should be dirty; rebuild should find the updated content
	after, err := v.SearchNotes(vault.SearchOptions{Query: "uniquekeyword99"})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) == 0 {
		t.Error("expected note to be findable after content update + index rebuild")
	}
}

func TestGitAudit_UpdateNoteCommits(t *testing.T) {
	dir, v := copyVaultToTemp(t)
	initGitRepo(t, dir)

	v.Close()
	cfg, _ := config.LoadConfig(dir)
	v = vault.New(cfg)
	defer v.Close()

	result, err := v.UpdateNote(vault.UpdateNoteOptions{
		Path:    "Areas/Engineering/decisions/use-sqlite-for-index.md",
		Content: "# Updated\n\nNew content.",
	})
	if err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}
	if !result.GitCommitted {
		t.Error("GitCommitted should be true when vault is a git repo")
	}

	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "ricket") {
		t.Errorf("expected ricket commit in log, got: %s", out)
	}
}

// ── git audit integration ─────────────────────────────────────────────────────

func TestGitAudit_FileNoteCommits(t *testing.T) {
	dir, v := copyVaultToTemp(t)
	initGitRepo(t, dir)

	// Close and re-open vault so git.IsGitRepo() picks up the new repo
	v.Close()
	cfg, _ := config.LoadConfig(dir)
	v = vault.New(cfg)
	defer v.Close()

	result, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
		Tags:        []string{"decision", "acme"},
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}
	if !result.GitCommitted {
		t.Error("GitCommitted should be true when vault is a git repo")
	}

	// Verify the commit is in the log
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "ricket") {
		t.Errorf("expected 'ricket' in commit message, got: %s", out)
	}
}

func TestGitAudit_CreateNoteCommits(t *testing.T) {
	dir, v := copyVaultToTemp(t)
	initGitRepo(t, dir)

	v.Close()
	cfg, _ := config.LoadConfig(dir)
	v = vault.New(cfg)
	defer v.Close()

	err := v.CreateNote(
		"Areas/Engineering/concepts/opentelemetry.md",
		"# OpenTelemetry\n\nVendor-neutral observability.",
		[]string{"concept", "acme"}, nil, "",
	)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	// Verify commit exists
	cmd := exec.Command("git", "log", "--oneline", "-2")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	logOutput := string(out)
	if !strings.Contains(logOutput, "ricket") {
		t.Errorf("expected ricket commit, got:\n%s", logOutput)
	}
}

func TestGitAudit_FileNote_NoRepoReturnsNotCommitted(t *testing.T) {
	_, v := copyVaultToTemp(t) // no git init

	result, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Areas/Engineering/decisions/use-query-builder.md",
	})
	if err != nil {
		t.Fatalf("FileNote: %v", err)
	}
	if result.GitCommitted {
		t.Error("GitCommitted should be false when no git repo")
	}
}

// ── edge cases ────────────────────────────────────────────────────────────────

func TestEdge_ReadNote_NonMarkdownExtension(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	// Write a .txt file — should be readable but won't appear in searches
	if err := os.WriteFile(filepath.Join(dir, "Inbox", "data.txt"), []byte("not markdown"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ReadNote on .txt should still work (no extension filter in ReadNote itself)
	_, err := v.ReadNote("Inbox/data.txt")
	if err != nil {
		t.Errorf("ReadNote on .txt: unexpected error: %v", err)
	}

	// But it should NOT appear in ListInbox
	notes, _ := v.ListInbox()
	for _, n := range notes {
		if strings.HasSuffix(n.Path, ".txt") {
			t.Errorf("non-.md file %q appeared in ListInbox", n.Path)
		}
	}
}

func TestEdge_FileNote_DestinationAlreadyExists(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	// Pre-create destination
	dstAbs := filepath.Join(dir, "Notes", "collision.md")
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstAbs, []byte("# Existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// FileNote to same destination — should overwrite (intentional: AI is filing)
	_, err := v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/raw-capture.md",
		Destination: "Notes/collision.md",
	})
	if err != nil {
		t.Fatalf("FileNote to existing destination: %v", err)
	}
	data, _ := os.ReadFile(dstAbs)
	if strings.Contains(string(data), "Existing content") {
		t.Error("expected destination to be overwritten")
	}
}

func TestEdge_MalformedFrontmatter(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	// Write a note with broken YAML frontmatter
	bad := "---\n{ invalid yaml: [\n---\n# Body\n\nContent."
	if err := os.WriteFile(filepath.Join(dir, "Inbox", "malformed.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	// ReadNote should not crash — it returns empty frontmatter
	note, err := v.ReadNote("Inbox/malformed.md")
	if err != nil {
		t.Fatalf("ReadNote malformed: unexpected error %v", err)
	}
	if note.Parsed.Content == "" && note.Parsed.Raw == "" {
		t.Error("malformed note should have non-empty content or raw")
	}
}

func TestEdge_VeryLongContent(t *testing.T) {
	dir, v := copyVaultToTemp(t)

	// Create a note with 100k words
	var sb strings.Builder
	sb.WriteString("---\ntags: [bigfile]\n---\n# Big Note\n\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString("The quick brown fox jumps over the lazy dog. ")
	}
	bigContent := sb.String()

	if err := os.WriteFile(filepath.Join(dir, "Inbox", "big.md"), []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	note, err := v.ReadNote("Inbox/big.md")
	if err != nil {
		t.Fatalf("ReadNote big file: %v", err)
	}
	if len(note.Parsed.Content) < 100 {
		t.Error("expected large content to be preserved")
	}

	// File it — should work even with large content
	_, err = v.FileNote(vault.FileNoteOptions{
		Source:      "Inbox/big.md",
		Destination: "Notes/filed-big.md",
	})
	if err != nil {
		t.Fatalf("FileNote big file: %v", err)
	}
}

func TestEdge_EmptyVaultSearch(t *testing.T) {
	dir := t.TempDir()

	// Minimal config for a vault with no notes at all
	if err := os.MkdirAll(filepath.Join(dir, "Inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.RicketConfig{
		VaultRoot: dir,
		Vault: config.VaultConfig{
			Root:      dir,
			Inbox:     "Inbox/",
			Archive:   "Archive/",
			Templates: "_templates/",
		},
		Categories: []config.Category{{Name: "note", Folder: "Notes/", Tags: []string{"note"}}},
	}
	v := vault.New(cfg)
	defer v.Close()

	status, err := v.Status()
	if err != nil {
		t.Fatalf("Status on empty vault: %v", err)
	}
	if status.InboxCount != 0 || status.TotalNotes != 0 {
		t.Errorf("empty vault: InboxCount=%d TotalNotes=%d", status.InboxCount, status.TotalNotes)
	}

	notes, err := v.SearchNotes(vault.SearchOptions{Tags: []string{"any"}})
	if err != nil {
		t.Fatalf("SearchNotes empty vault: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

func TestEdge_CreateNote_InvalidPaths(t *testing.T) {
	_, v := copyVaultToTemp(t)

	cases := []string{
		"",
		"../../escape.md",
		"/absolute/path.md",
	}
	for _, p := range cases {
		err := v.CreateNote(p, "content", nil, nil, "")
		if err == nil {
			t.Errorf("CreateNote(%q): expected error for invalid path", p)
		}
	}
}

func TestEdge_SearchWithCombinedFilters(t *testing.T) {
	v := loadTestVault(t)

	results, err := v.SearchNotes(vault.SearchOptions{
		Folder: "Areas/Engineering/",
		Tags:   []string{"acme"},
		Query:  "Dapper",
	})
	if err != nil {
		t.Fatalf("SearchNotes combined: %v", err)
	}
	// Should find use-dapper-not-efcore.md
	if len(results) == 0 {
		t.Error("expected at least one result for combined folder+tag+query search")
	}
	for _, r := range results {
		if !strings.HasPrefix(r.Path, "Areas/Engineering/") {
			t.Errorf("result %q outside expected folder", r.Path)
		}
	}
}

// ── helper ───────────────────────────────────────────────────────────────────

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
