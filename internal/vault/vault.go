package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlejandroByrne/ricket/internal/config"
)

// VaultNote represents a note with its parsed content and metadata.
type VaultNote struct {
	Path         string     // relative to vault root (always forward slashes)
	AbsolutePath string
	Parsed       ParsedNote
	Name         string // filename without .md
}

// FileNoteOptions controls how fileNote moves and transforms a note.
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
	Destination     string
	GitCommitMessage string
}

// StatusResult holds vault health metrics.
type StatusResult struct {
	InboxCount  int
	TotalNotes  int
	Categories  int
}

// Vault provides operations on an Obsidian vault.
type Vault struct {
	cfg *config.RicketConfig
}

// New creates a Vault for the given config.
func New(cfg *config.RicketConfig) *Vault {
	return &Vault{cfg: cfg}
}

// ListInbox returns all notes in the inbox folder.
func (v *Vault) ListInbox() ([]VaultNote, error) {
	return v.SearchNotes(SearchOptions{Folder: v.cfg.Vault.Inbox})
}

// ReadNote reads a single note by relative path.
func (v *Vault) ReadNote(relativePath string) (VaultNote, error) {
	// Normalize path separators
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
func (v *Vault) SearchNotes(opts SearchOptions) ([]VaultNote, error) {
	searchRoot := v.cfg.VaultRoot
	if opts.Folder != "" {
		searchRoot = filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Folder))
	}

	var results []VaultNote

	err := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if d.IsDir() {
			// Skip hidden dirs
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		// Compute relative path
		rel, err := filepath.Rel(v.cfg.VaultRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		note, err := v.ReadNote(rel)
		if err != nil {
			return nil // skip unreadable notes
		}

		// Filter by tags
		if len(opts.Tags) > 0 {
			noteTags := GetTags(note.Parsed)
			if !hasAllTags(noteTags, opts.Tags) {
				return nil
			}
		}

		// Filter by query
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

// FileNote moves a note from source to destination, applying optional template,
// tags, links, and MOC update. Returns destination path and git commit message.
func (v *Vault) FileNote(opts FileNoteOptions) (FileNoteResult, error) {
	sourceAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Source))
	destAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.Destination))

	// Read source
	sourceNote, err := v.ReadNote(opts.Source)
	if err != nil {
		return FileNoteResult{}, err
	}

	// Use override content if provided
	content := opts.Content
	if content == "" {
		content = sourceNote.Parsed.Content
	}

	// Scaffold with template if specified
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

	// Parse the (potentially updated) content with existing frontmatter
	updated := ParseNote(SerializeNote(sourceNote.Parsed.Frontmatter, content))

	// Add tags
	if len(opts.Tags) > 0 {
		updated = AddFrontmatterTags(updated, opts.Tags)
	}

	// Add wikilinks to ## Links section
	if len(opts.Links) > 0 {
		updated.Content = addLinksToContent(updated.Content, opts.Links)
		updated = ParseNote(SerializeNote(updated.Frontmatter, updated.Content))
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to create destination dir: %w", err)
	}

	// Write destination
	serialized := SerializeNote(updated.Frontmatter, updated.Content)
	if err := os.WriteFile(destAbs, []byte(serialized), 0o644); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to write destination %s: %w", opts.Destination, err)
	}

	// Update MOC (best-effort)
	if opts.MOC != "" {
		mocAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(opts.MOC))
		UpdateMOCFile(mocAbs, sourceNote.Name, opts.Destination) //nolint:errcheck
	}

	// Delete source
	if err := os.Remove(sourceAbs); err != nil {
		return FileNoteResult{}, fmt.Errorf("failed to delete source %s: %w", opts.Source, err)
	}

	sourceBasename := filepath.Base(opts.Source)
	commitMsg := fmt.Sprintf("ricket: filed %s → %s", sourceBasename, opts.Destination)

	return FileNoteResult{
		Destination:     opts.Destination,
		GitCommitMessage: commitMsg,
	}, nil
}

// CreateNote creates a new note at destination with optional tags, links, and MOC update.
func (v *Vault) CreateNote(destination, content string, tags, links []string, moc string) error {
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

	// Update MOC (best-effort)
	if moc != "" {
		mocAbs := filepath.Join(v.cfg.VaultRoot, filepath.FromSlash(moc))
		noteTitle := strings.TrimSuffix(filepath.Base(destination), ".md")
		UpdateMOCFile(mocAbs, noteTitle, destination) //nolint:errcheck
	}

	return nil
}

// UpdateMOC updates a MOC file by appending a link.
func (v *Vault) UpdateMOC(mocPath, noteTitle, notePath string) error {
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
	all, err := v.SearchNotes(SearchOptions{})
	if err != nil {
		return StatusResult{}, err
	}
	return StatusResult{
		InboxCount: len(inbox),
		TotalNotes: len(all),
		Categories: len(v.cfg.Categories),
	}, nil
}

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
		// Create new section
		return content + "\n\n## Links\n" + newLinks
	}

	// Find end of ## Links section (next ## or end of content)
	afterLinks := strings.Index(content[linksIdx+1:], "\n\n##")
	if afterLinks == -1 {
		return content + "\n" + newLinks
	}
	insertPos := linksIdx + 1 + afterLinks
	return content[:insertPos] + "\n" + newLinks + content[insertPos:]
}
