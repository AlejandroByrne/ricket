package vault

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// NoteRecord is a row stored in the SQLite index.
type NoteRecord struct {
	Path    string
	Title   string
	Tags    []string
	Content string
}

// SearchResult is returned from content/tag searches.
type SearchResult struct {
	Path    string
	Title   string
	Snippet string // empty unless content search
}

// FolderResult is returned from folder-based lookups.
type FolderResult struct {
	Path  string
	Title string
	Tags  []string
}

// VaultIndex is a SQLite-backed search index for vault notes.
type VaultIndex struct {
	vaultRoot string
	indexPath string
	db        *sql.DB
}

// NewVaultIndex creates a VaultIndex for the given vault root.
// Call Init() before use.
func NewVaultIndex(vaultRoot string) *VaultIndex {
	return &VaultIndex{
		vaultRoot: vaultRoot,
		indexPath: filepath.Join(vaultRoot, ".ricket", "index.db"),
	}
}

// Init opens or creates the SQLite database at .ricket/index.db.
func (idx *VaultIndex) Init() error {
	// Ensure directory exists
	dir := filepath.Dir(idx.indexPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create .ricket dir: %w", err)
	}

	db, err := sql.Open("sqlite", idx.indexPath)
	if err != nil {
		return fmt.Errorf("failed to open index db: %w", err)
	}
	idx.db = db
	return idx.createSchema()
}

func (idx *VaultIndex) createSchema() error {
	_, err := idx.db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			path       TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			tags       TEXT,
			content    TEXT,
			folder     TEXT,
			updated_at TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_folder ON notes(folder);
	`)
	return err
}

// Rebuild replaces the entire index with the provided notes.
func (idx *VaultIndex) Rebuild(notes []NoteRecord) error {
	if idx.db == nil {
		return fmt.Errorf("index not initialized")
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec("DELETE FROM notes"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO notes (path, title, tags, content, folder, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, n := range notes {
		tagsJSON, _ := json.Marshal(n.Tags)
		folder := extractFolder(n.Path)
		if _, err := stmt.Exec(n.Path, n.Title, string(tagsJSON), n.Content, folder, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SearchByTags returns notes that contain ALL specified tags.
func (idx *VaultIndex) SearchByTags(tags []string) ([]SearchResult, error) {
	if idx.db == nil {
		return nil, fmt.Errorf("index not initialized")
	}
	if len(tags) == 0 {
		return nil, nil
	}

	rows, err := idx.db.Query("SELECT path, title, tags FROM notes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var path, title, tagsJSON string
		if err := rows.Scan(&path, &title, &tagsJSON); err != nil {
			continue
		}
		var noteTags []string
		if err := json.Unmarshal([]byte(tagsJSON), &noteTags); err != nil {
			continue
		}
		if hasAllTags(noteTags, tags) {
			results = append(results, SearchResult{Path: path, Title: title})
		}
	}
	return results, rows.Err()
}

// SearchContent returns notes whose content contains query, with a 100-char snippet.
func (idx *VaultIndex) SearchContent(query string) ([]SearchResult, error) {
	if idx.db == nil {
		return nil, fmt.Errorf("index not initialized")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	rows, err := idx.db.Query(
		"SELECT path, title, content FROM notes WHERE content LIKE ?",
		"%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	lowerQuery := strings.ToLower(query)
	for rows.Next() {
		var path, title, content string
		if err := rows.Scan(&path, &title, &content); err != nil {
			continue
		}
		snippet := buildSnippet(content, lowerQuery)
		results = append(results, SearchResult{Path: path, Title: title, Snippet: snippet})
	}
	return results, rows.Err()
}

// GetByFolder returns all notes in folder (prefix match).
func (idx *VaultIndex) GetByFolder(folder string) ([]FolderResult, error) {
	if idx.db == nil {
		return nil, fmt.Errorf("index not initialized")
	}

	searchFolder := folder
	if !strings.HasSuffix(searchFolder, "/") {
		searchFolder += "/"
	}

	rows, err := idx.db.Query(
		"SELECT path, title, tags FROM notes WHERE folder LIKE ?",
		searchFolder+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FolderResult
	for rows.Next() {
		var path, title, tagsJSON string
		if err := rows.Scan(&path, &title, &tagsJSON); err != nil {
			continue
		}
		var tags []string
		json.Unmarshal([]byte(tagsJSON), &tags) //nolint:errcheck
		results = append(results, FolderResult{Path: path, Title: title, Tags: tags})
	}
	return results, rows.Err()
}

// Close closes the database connection.
func (idx *VaultIndex) Close() error {
	if idx.db != nil {
		err := idx.db.Close()
		idx.db = nil
		return err
	}
	return nil
}

// extractFolder returns the folder portion of a path (e.g., "Projects/FCBT/MOC.md" → "Projects/FCBT/")
func extractFolder(notePath string) string {
	dir := filepath.ToSlash(filepath.Dir(notePath))
	if dir == "." {
		return ""
	}
	return dir + "/"
}

func hasAllTags(noteTags, required []string) bool {
	tagSet := make(map[string]bool, len(noteTags))
	for _, t := range noteTags {
		tagSet[t] = true
	}
	for _, t := range required {
		if !tagSet[t] {
			return false
		}
	}
	return true
}

func buildSnippet(content, lowerQuery string) string {
	lowerContent := strings.ToLower(content)
	idx := strings.Index(lowerContent, lowerQuery)
	if idx == -1 {
		return ""
	}
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + 50
	if end > len(content) {
		end = len(content)
	}
	snippet := strings.TrimSpace(content[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}
