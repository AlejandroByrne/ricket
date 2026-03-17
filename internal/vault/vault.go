package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitpkg "github.com/AlejandroByrne/ricket/internal/git"

	"github.com/AlejandroByrne/ricket/internal/config"
)

// VaultNote represents a note with its parsed content and metadata.
type VaultNote struct {
	Path         string // relative to vault root (always forward slashes)
	AbsolutePath string
	Parsed       ParsedNote
	Name         string // filename without .md
}

// FileNoteOptions controls how FileNote moves and transforms a note.
type FileNoteOptions struct {
	Source      string   // relative path of source (typically in Inbox/)
	Destination string   // relative path of destination
	Content     string   // optional content override
	Tags        []string // tags to add to frontmatter
	Links       []string // wikilinks to add to ## Links section
	MOC         string   // MOC file to update (relative path)
	Template    string   // template name (without .md)
}

// FileNoteResult is returned by FileNote.
type FileNoteResult struct {
	Destination      string
	GitCommitMessage string
	GitCommitted     bool
}

// StatusResult holds vault health metrics.
type StatusResult struct {
	InboxCount int
	TotalNotes int
	Categories int
}

// Vault provides operations on an Obsidian vault.
type Vault struct {
	cfg        *config.RicketConfig
	index      *VaultIndex
	ga         *gitpkg.GitAudit
	indexDirty bool
}

// New creates a Vault for the given config.
// Initialises the SQLite index and git audit trail (both best-effort; failures
// result in degraded-mode operation, not a fatal error).
func New(cfg *config.RicketConfig) *Vault {
	v := &Vault{cfg: cfg, indexDirty: true}

	// SQLite index — best-effort
	idx := NewVaultIndex(cfg.VaultRoot)
	if err := idx.Init(); err == nil {
		v.index = idx
	}

	// Git audit — only attach if the vault root is inside a git repo
	ga := gitpkg.New(cfg.VaultRoot)
	if ga.IsGitRepo() {
		v.ga = ga
	}

	return v
}

// Close releases the SQLite connection (call when the vault is no longer needed).
func (v *Vault) Close() error {
	if v.index != nil {
		return v.index.Close()
	}
	return nil
}

// ── Path safety ───────────────────────────────────────────────────────────────

// validatePath rejects empty, absolute, and path-traversal inputs.
// All user-supplied paths must pass this check before being used.
func (v *Vault) validatePath(relPath string) error {
	if strings.TrimSpace(relPath) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if filepath.IsAbs(relPath) {
		return fmt.Errorf("path must be relative, got absolute path %q", relPath)
	}
	abs := filepath.Clean(filepath.Join(v.cfg.VaultRoot, relPath))
	root := filepath.Clean(v.cfg.VaultRoot)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes vault root (traversal attempt?)", relPath)
	}
	return nil
}

// ── SQLite index management ───────────────────────────────────────────────────

// ensureIndex rebuilds the SQLite index from the filesystem if the dirty flag is set.
// It is a no-op when no index is available.
func (v *Vault) ensureIndex() {
	if v.index == nil || !v.indexDirty {
		return
	}
	notes := v.walkAllNoteRecords()
	_ = v.index.Rebuild(notes) // best-effort; walk search still works on failure
	v.indexDirty = false
}

// walkAllNoteRecords walks the entire vault and returns NoteRecord slices
// suitable for indexing. Unreadable files are silently skipped.
func (v *Vault) walkAllNoteRecords() []NoteRecord {
	var records []NoteRecord
	_ = filepath.WalkDir(v.cfg.VaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(v.cfg.VaultRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		parsed := ParseNote(string(data))
		title := strings.TrimSuffix(filepath.Base(rel), ".md")
		if t, ok := parsed.Frontmatter["title"].(string); ok && t != "" {
			title = t
		}
		records = append(records, NoteRecord{
			Path:    rel,
			Title:   title,
			Tags:    GetTags(parsed),
			Content: parsed.Content,
		})
		return nil
	})
	return records
}

// markDirty marks the index as needing rebuild. Call after any mutation.
func (v *Vault) markDirty() {
	v.indexDirty = true
}

// ── Read operations ───────────────────────────────────────────────────────────

// ListInbox returns all notes in the inbox folder.
func (v *Vault) ListInbox() ([]VaultNote, error) {
	return v.searchWalk(SearchOptions{Folder: v.cfg.Vault.Inbox})
}

// ReadNote reads a single note by relative path.
func (v *Vault) ReadNote(relativePath string) (VaultNote, error) {
	if err := v.validatePath(relativePath); err != nil {
		return VaultNote{}, err
	}
	relativePath = filepath.ToSlash(relativePath)
	absPath := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(relativePath))

	data, err := os.ReadFile(absPath)
	if err != nil {
		return VaultNote{}, fmt.Errorf("failed to read note %s: %w", relativePath, err)
	}

	parsed := ParseNote(string(data))
	name := strings.TrimSuffix(filepath.Base(relativePath), ".md")

	return VaultNote{
		Path:         relativePath,
		AbsolutePath: absPath,
		Parsed:       parsed,
		Name:         name,
	}, nil
}

// SearchOptions controls SearchNotes filtering.
type SearchOptions struct {
	Folder string
	Tags   []string
	Query  string
}

// SearchNotes searches notes by folder, tags, and/or text query.
// Uses the SQLite index for tag/content queries when available; falls back to a
// filesystem walk otherwise.
func (v *Vault) SearchNotes(opts SearchOptions) ([]VaultNote, error) {
	if v.index != nil && (len(opts.Tags) > 0 || opts.Query != "") {
		return v.searchViaIndex(opts)
	}
	return v.searchWalk(opts)
}

// searchViaIndex uses the SQLite index for tag/content filtering.
func (v *Vault) searchViaIndex(opts SearchOptions) ([]VaultNote, error) {
	v.ensureIndex()

	var pathSet map[string]bool

	if len(opts.Tags) > 0 {
		results, err := v.index.SearchByTags(opts.Tags)
		if err != nil {
			return v.searchWalk(opts) // degrade gracefully
		}
		pathSet = make(map[string]bool, len(results))
		for _, r := range results {
			pathSet[r.Path] = true
		}
	}

	if opts.Query != "" {
		results, err := v.index.SearchContent(opts.Query)
		if err != nil {
			return v.searchWalk(opts)
		}
		if pathSet == nil {
			pathSet = make(map[string]bool, len(results))
			for _, r := range results {
				pathSet[r.Path] = true
			}
		} else {
			// AND: keep only paths that also match query
			filtered := make(map[string]bool)
			for _, r := range results {
				if pathSet[r.Path] {
					filtered[r.Path] = true
				}
			}
			pathSet = filtered
		}
	}

	var notes []VaultNote
	for path := range pathSet {
		// Apply folder filter
		if opts.Folder != "" {
			folder := filepath.ToSlash(filepath.Clean(opts.Folder))
			if !strings.HasPrefix(filepath.ToSlash(path), folder) {
				continue
			}
		}
		note, err := v.ReadNote(path)
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}
	return notes, nil
}

// searchWalk performs a filesystem walk with optional folder/tag/query filters.
func (v *Vault) searchWalk(opts SearchOptions) ([]VaultNote, error) {
	searchRoot := v.cfg.VaultRoot
	if opts.Folder != "" {
		searchRoot = filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Folder))
	}

	var results []VaultNote

	err := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		rel, err := filepath.Rel(v.cfg.VaultRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		note, err := v.ReadNote(rel)
		if err != nil {
			return nil
		}

		if len(opts.Tags) > 0 {
			noteTags := GetTags(note.Parsed)
			if !hasAllTags(noteTags, opts.Tags) {
				return nil
			}
		}

		if opts.Query != "" {
			title, _ := note.Parsed.Frontmatter["title"].(string)
			combined := strings.ToLower(title + " " + note.Parsed.Content)
			if !strings.Contains(combined, strings.ToLower(opts.Query)) {
				return nil
			}
		}

		results = append(results, note)
		return nil
	})

	return results, err
}

// GetCategories returns all configured categories.
func (v *Vault) GetCategories() []config.Category {
	return v.cfg.Categories
}

// GetTemplateList returns the names (without .md) of all templates.
func (v *Vault) GetTemplateList() ([]string, error) {
	templatesDir := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(v.cfg.Vault.Templates))
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// ── Write operations ──────────────────────────────────────────────────────────

// FileNote moves a note from source to destination, applying optional template,
// tags, links, and MOC update. Auto-commits to git if the vault is a git repo.
func (v *Vault) FileNote(opts FileNoteOptions) (FileNoteResult, error) {
	if err := v.validatePath(opts.Source); err != nil {
		return FileNoteResult{}, fmt.Errorf("invalid source: %w", err)
	}
	if err := v.validatePath(opts.Destination); err != nil {
		return FileNoteResult{}, fmt.Errorf("invalid destination: %w", err)
	}

	sourceAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Source))
	destAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Destination))

	sourceNote, err := v.ReadNote(opts.Source)
	if err != nil {
		return FileNoteResult{}, err
	}

	content := opts.Content
	if content == "" {
		content = sourceNote.Parsed.Content
	}

	if opts.Template != "" {
		templatesDir := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(v.cfg.Vault.Templates))
		templateContent, err := LoadTemplate(templatesDir, opts.Template)
		if err != nil {
			return FileNoteResult{}, err
		}
		vars := TemplateVars{
			Title: sourceNote.Name,
			Date:  time.Now().Format("2006-01-02"),
		}
		scaffolded := ScaffoldNote(templateContent, vars)
		content = MergeContentIntoTemplate(scaffolded, content)
	}

	updated := ParseNote(SerializeNote(sourceNote.Parsed.Frontmatter, content))

	if len(opts.Tags) > 0 {
		updated = AddFrontmatterTags(updated, opts.Tags)
	}

	if len(opts.Links) > 0 {
		updated.Content = addLinksToContent(updated.Content, opts.Links)
		updated = ParseNote(SerializeNote(updated.Frontmatter, updated.Content))
	}

	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to create destination dir: %w", err)
	}

	serialized := SerializeNote(updated.Frontmatter, updated.Content)
	if err := os.WriteFile(destAbs, []byte(serialized), 0o644); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to write destination %s: %w", opts.Destination, err)
	}

	if opts.MOC != "" {
		if mocErr := v.validatePath(opts.MOC); mocErr == nil {
			mocAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.MOC))
			UpdateMOCFile(mocAbs, sourceNote.Name, opts.Destination) //nolint:errcheck
		}
	}

	if err := os.Remove(sourceAbs); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to delete source %s: %w", opts.Source, err)
	}

	v.markDirty()

	sourceBasename := filepath.Base(opts.Source)
	commitMsg := fmt.Sprintf("ricket: filed %s → %s", sourceBasename, opts.Destination)

	committed := false
	if v.ga != nil {
		committed = v.ga.CommitFileMove(opts.Source, opts.Destination)
	}

	return FileNoteResult{
		Destination:      opts.Destination,
		GitCommitMessage: commitMsg,
		GitCommitted:     committed,
	}, nil
}

// CreateNote creates a new note at destination with optional tags, links, and MOC update.
// Auto-commits to git if the vault is a git repo.
func (v *Vault) CreateNote(destination, content string, tags, links []string, moc string) error {
	if err := v.validatePath(destination); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	destAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(destination))

	note := ParseNote(content)

	if len(tags) > 0 {
		note = AddFrontmatterTags(note, tags)
	}

	if len(links) > 0 {
		note.Content = addLinksToContent(note.Content, links)
		note = ParseNote(SerializeNote(note.Frontmatter, note.Content))
	}

	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	serialized := SerializeNote(note.Frontmatter, note.Content)
	if err := os.WriteFile(destAbs, []byte(serialized), 0o644); err != nil {
		return fmt.Errorf("failed to write note %s: %w", destination, err)
	}

	if moc != "" {
		if mocErr := v.validatePath(moc); mocErr == nil {
			mocAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(moc))
			noteTitle := strings.TrimSuffix(filepath.Base(destination), ".md")
			UpdateMOCFile(mocAbs, noteTitle, destination) //nolint:errcheck
		}
	}

	v.markDirty()

	if v.ga != nil {
		commitMsg := fmt.Sprintf("ricket: created %s", destination)
		v.ga.Commit([]string{destination}, commitMsg) //nolint:errcheck (best-effort)
	}

	return nil
}

// UpdateMOC updates a MOC file by appending a link.
func (v *Vault) UpdateMOC(mocPath, noteTitle, notePath string) error {
	if err := v.validatePath(mocPath); err != nil {
		return fmt.Errorf("invalid MOC path: %w", err)
	}
	mocAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(mocPath))
	_, err := UpdateMOCFile(mocAbs, noteTitle, notePath)
	if err != nil {
		return fmt.Errorf("failed to update MOC %s: %w", mocPath, err)
	}
	return nil
}

// Status returns inbox count, total notes, and category count.
func (v *Vault) Status() (StatusResult, error) {
	inbox, err := v.ListInbox()
	if err != nil {
		return StatusResult{}, err
	}
	all, err := v.searchWalk(SearchOptions{})
	if err != nil {
		return StatusResult{}, err
	}
	return StatusResult{
		InboxCount: len(inbox),
		TotalNotes: len(all),
		Categories: len(v.cfg.Categories),
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// addLinksToContent appends wikilinks to the ## Links section of content,
// creating the section if it doesn't exist.
func addLinksToContent(content string, links []string) string {
	linkLines := make([]string, len(links))
	for i, link := range links {
		linkLines[i] = fmt.Sprintf("- [[%s]]", link)
	}
	newLinks := strings.Join(linkLines, "\n")

	linksIdx := strings.Index(content, "## Links")
	if linksIdx == -1 {
		return content + "\n\n## Links\n" + newLinks
	}

	afterLinks := strings.Index(content[linksIdx+1:], "\n\n##")
	if afterLinks == -1 {
		return content + "\n" + newLinks
	}
	insertPos := linksIdx + 1 + afterLinks
	return content[:insertPos] + "\n" + newLinks + content[insertPos:]
}
