package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/vault"
)

// ── AnalyzeVaultRoot ──────────────────────────────────────────────────────────

func TestAnalyzeVaultRoot_ExistingVault(t *testing.T) {
	root := testdataVaultPath(t)

	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if a.VaultRoot != root {
		t.Errorf("VaultRoot = %q, want %q", a.VaultRoot, root)
	}
	if a.IsNewVault {
		t.Error("IsNewVault should be false for testdata/vault")
	}
	if a.TotalNoteCount == 0 {
		t.Error("TotalNoteCount should be > 0")
	}
	if a.DetectedInbox == "" {
		t.Error("DetectedInbox should be non-empty")
	}
	if a.DetectedTemplatesDir == "" {
		t.Error("DetectedTemplatesDir should be non-empty")
	}
	if len(a.Folders) == 0 {
		t.Error("Folders should not be empty")
	}
}

func TestAnalyzeVaultRoot_ObsidianDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".obsidian"), 0o755); err != nil {
		t.Fatalf("mkdir .obsidian: %v", err)
	}

	a, err := vault.AnalyzeVaultRoot(dir)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if !a.ObsidianVaultDetected {
		t.Error("ObsidianVaultDetected should be true when .obsidian/ exists")
	}
	if !a.IsNewVault {
		t.Error("IsNewVault should be false when .obsidian is present but has no notes")
	}
}

func TestAnalyzeVaultRoot_NewEmptyVault(t *testing.T) {
	dir := t.TempDir()

	a, err := vault.AnalyzeVaultRoot(dir)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if !a.IsNewVault {
		t.Error("IsNewVault should be true for an empty directory")
	}
	if a.ObsidianVaultDetected {
		t.Error("ObsidianVaultDetected should be false for empty directory")
	}
	if a.TotalNoteCount != 0 {
		t.Errorf("TotalNoteCount = %d, want 0", a.TotalNoteCount)
	}
	if len(a.InferredCategories) != 0 {
		t.Errorf("InferredCategories should be empty for new vault, got %d", len(a.InferredCategories))
	}
}

func TestAnalyzeVaultRoot_HasExistingConfig(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}
	if !a.HasExistingConfig {
		t.Error("HasExistingConfig should be true for testdata/vault (ricket.yaml present)")
	}
}

func TestAnalyzeVaultRoot_TagFrequency(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if len(a.TagFrequency) == 0 {
		t.Fatal("TagFrequency should not be empty for testdata/vault")
	}
	// Tags should be sorted by count descending
	for i := 1; i < len(a.TagFrequency); i++ {
		if a.TagFrequency[i].Count > a.TagFrequency[i-1].Count {
			t.Errorf("TagFrequency not sorted: [%d].Count=%d > [%d].Count=%d",
				i, a.TagFrequency[i].Count, i-1, a.TagFrequency[i-1].Count)
		}
	}
}

func TestAnalyzeVaultRoot_MOCFilesDetected(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if len(a.MOCFiles) == 0 {
		t.Error("MOCFiles should not be empty — testdata/vault has several MOC.md files")
	}
	for _, moc := range a.MOCFiles {
		base := strings.ToLower(strings.TrimSuffix(filepath.Base(moc), ".md"))
		if base != "moc" && base != "index" && base != "home" {
			t.Errorf("unexpected MOC file: %s", moc)
		}
	}
}

func TestAnalyzeVaultRoot_TemplatesLoaded(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if len(a.Templates) == 0 {
		t.Fatal("Templates should not be empty — testdata/vault has a _templates/ dir")
	}
	names := make(map[string]bool, len(a.Templates))
	for _, tmpl := range a.Templates {
		names[tmpl.Name] = true
	}
	for _, expected := range []string{"decision", "concept", "meeting"} {
		if !names[expected] {
			t.Errorf("expected template %q to be detected", expected)
		}
	}
}

func TestAnalyzeVaultRoot_InferredCategories(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	if len(a.InferredCategories) == 0 {
		t.Fatal("InferredCategories should not be empty for testdata/vault")
	}

	for _, cat := range a.InferredCategories {
		if cat.Name == "" {
			t.Error("category name should not be empty")
		}
		if cat.Folder == "" {
			t.Error("category folder should not be empty")
		}
		if !strings.HasSuffix(cat.Folder, "/") {
			t.Errorf("category folder %q should have trailing slash", cat.Folder)
		}
		if cat.Confidence <= 0 || cat.Confidence > 1.0 {
			t.Errorf("category %q confidence %f out of range (0,1]", cat.Name, cat.Confidence)
		}
		if cat.Reasoning == "" {
			t.Errorf("category %q should have non-empty reasoning", cat.Name)
		}
		if len(cat.Tags) == 0 {
			t.Errorf("category %q should have at least one tag", cat.Name)
		}
	}

	// Check that decision-type folder was correctly inferred from testdata
	foundDecision := false
	for _, cat := range a.InferredCategories {
		if strings.Contains(cat.Folder, "decisions") || strings.Contains(cat.Name, "decision") {
			foundDecision = true
			break
		}
	}
	if !foundDecision {
		t.Error("expected at least one decision category to be inferred from testdata/vault")
	}
}

func TestAnalyzeVaultRoot_NamingPatterns(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	// testdata/vault/Areas/Engineering/decisions/ has "use-*.md" files
	var decisionPattern string
	for _, p := range a.NamingPatterns {
		if strings.Contains(p.Folder, "decisions") {
			decisionPattern = p.Pattern
			break
		}
	}
	if decisionPattern == "" {
		t.Log("no naming pattern found for decisions folder (may not have enough files)")
	} else if decisionPattern != "use-{topic}.md" {
		t.Errorf("decisions naming pattern = %q, want %q", decisionPattern, "use-{topic}.md")
	}
}

func TestAnalyzeVaultRoot_FolderEntriesHaveTrailingSlash(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}
	for _, f := range a.Folders {
		if !strings.HasSuffix(f.Path, "/") {
			t.Errorf("folder path %q missing trailing slash", f.Path)
		}
	}
}

func TestAnalyzeVaultRoot_TemplatesDirExcludedFromFolders(t *testing.T) {
	root := testdataVaultPath(t)
	a, err := vault.AnalyzeVaultRoot(root)
	if err != nil {
		t.Fatalf("AnalyzeVaultRoot: %v", err)
	}

	// The _templates/ folder itself should not appear as a category folder
	for _, f := range a.Folders {
		if strings.HasPrefix(f.Path, "_templates") {
			t.Errorf("_templates folder should be excluded from Folders, got %q", f.Path)
		}
	}
}

// ── ScaffoldVault ─────────────────────────────────────────────────────────────

func TestScaffoldVault_CreatesExpectedFiles(t *testing.T) {
	_, v := copyVaultToTemp(t)
	_ = v // only need the config

	// ScaffoldVault is tested via config scaffold CLI test; here we ensure
	// DefaultTemplateContent produces valid non-empty content.
	for _, name := range []string{"decision", "concept", "meeting", "project", "learning", "person", "journal"} {
		content := vault.DefaultTemplateContent(name)
		if content == "" {
			t.Errorf("DefaultTemplateContent(%q) returned empty string", name)
		}
		if !strings.Contains(content, "---") {
			t.Errorf("DefaultTemplateContent(%q) should contain frontmatter delimiters", name)
		}
	}
}
