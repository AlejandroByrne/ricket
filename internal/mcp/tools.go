package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AlejandroByrne/ricket/internal/vault"
)

// registerTools registers all ricket MCP tools on srv.
func registerTools(srv *mcpserver.MCPServer, s *RicketMCPServer) {
	srv.AddTool(toolListInbox(), handleVaultListInbox(s))
	srv.AddTool(toolTriageInbox(), handleVaultTriageInbox(s))
	srv.AddTool(toolReadNote(), handleVaultReadNote(s))
	srv.AddTool(toolSearch(), handleVaultSearch(s))
	srv.AddTool(toolGetCategories(), handleVaultGetCategories(s))
	srv.AddTool(toolGetTemplates(), handleVaultGetTemplates(s))
	srv.AddTool(toolFileNote(), handleVaultFileNote(s))
	srv.AddTool(toolCreateNote(), handleVaultCreateNote(s))
	srv.AddTool(toolUpdateNote(), handleVaultUpdateNote(s))
	srv.AddTool(toolStatus(), handleVaultStatus(s))
}

// ── Tool definitions ──────────────────────────────────────────────────────────

func toolListInbox() mcplib.Tool {
	return mcplib.NewTool("vault_list_inbox",
		mcplib.WithDescription("List all notes in the vault inbox. Returns path, name, and a 200-character preview for each note."),
	)
}

func toolTriageInbox() mcplib.Tool {
	return mcplib.NewTool("vault_triage_inbox",
		mcplib.WithDescription("Analyze Inbox notes and propose filing actions with category, destination path, confidence, and matched signals. Does not move files."),
	)
}

func toolReadNote() mcplib.Tool {
	return mcplib.NewTool("vault_read_note",
		mcplib.WithDescription("Read a single note by its relative vault path. Returns path, name, frontmatter, content, tags, and wikilinks."),
		mcplib.WithString("path",
			mcplib.Required(),
			mcplib.Description("Relative path of the note within the vault (e.g. 'Inbox/my-note.md')"),
		),
	)
}

func toolSearch() mcplib.Tool {
	t := mcplib.NewTool("vault_search",
		mcplib.WithDescription("Search notes by folder, tags, and/or full-text query. All filters are combined (AND logic)."),
		mcplib.WithString("folder",
			mcplib.Description("Restrict search to this folder (relative path, optional)"),
		),
		mcplib.WithString("query",
			mcplib.Description("Full-text search query (optional)"),
		),
	)
	t.InputSchema.Properties["tags"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Filter by tags — note must have ALL listed tags (optional)",
	}
	return t
}

func toolGetCategories() mcplib.Tool {
	return mcplib.NewTool("vault_get_categories",
		mcplib.WithDescription("Return all configured note categories with their folders, templates, tags, MOC paths, and classification signals."),
	)
}

func toolGetTemplates() mcplib.Tool {
	return mcplib.NewTool("vault_get_templates",
		mcplib.WithDescription("Return all available templates with their names and section headings (## fields)."),
	)
}

func toolFileNote() mcplib.Tool {
	t := mcplib.NewTool("vault_file_note",
		mcplib.WithDescription("Move a note from source to destination, optionally applying a template, tags, links, and a MOC update. Returns the new path and a suggested git commit message."),
		mcplib.WithString("source",
			mcplib.Required(),
			mcplib.Description("Relative path of the source note (typically in Inbox/)"),
		),
		mcplib.WithString("destination",
			mcplib.Required(),
			mcplib.Description("Relative destination path within the vault"),
		),
		mcplib.WithString("content",
			mcplib.Description("Content override for the note (optional)"),
		),
		mcplib.WithString("template",
			mcplib.Description("Template name to apply (without .md extension, optional)"),
		),
		mcplib.WithString("moc",
			mcplib.Description("Relative path of a MOC file to update with a link to this note (optional)"),
		),
	)
	t.InputSchema.Properties["tags"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Tags to add to the note's frontmatter (optional)",
	}
	t.InputSchema.Properties["links"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Wikilinks to append to the ## Links section (optional)",
	}
	return t
}

func toolCreateNote() mcplib.Tool {
	t := mcplib.NewTool("vault_create_note",
		mcplib.WithDescription("Create a new note at the given path with optional tags, links, and MOC update."),
		mcplib.WithString("path",
			mcplib.Required(),
			mcplib.Description("Relative destination path within the vault"),
		),
		mcplib.WithString("content",
			mcplib.Required(),
			mcplib.Description("Markdown content for the new note"),
		),
		mcplib.WithString("moc",
			mcplib.Description("Relative path of a MOC file to update (optional)"),
		),
	)
	t.InputSchema.Properties["tags"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Tags to add to the note's frontmatter (optional)",
	}
	t.InputSchema.Properties["links"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Wikilinks to append to the ## Links section (optional)",
	}
	return t
}

func toolUpdateNote() mcplib.Tool {
	t := mcplib.NewTool("vault_update_note",
		mcplib.WithDescription("Update an existing note's content, tags, and/or links in-place. At least one of content, tags, or links must be provided."),
		mcplib.WithString("path",
			mcplib.Required(),
			mcplib.Description("Relative path of the note to update within the vault"),
		),
		mcplib.WithString("content",
			mcplib.Description("New body content to replace the existing note body (optional)"),
		),
	)
	t.InputSchema.Properties["tags"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Tags to add to the note's frontmatter (additive, optional)",
	}
	t.InputSchema.Properties["links"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "Wikilinks to append to the ## Links section (optional)",
	}
	return t
}

func toolStatus() mcplib.Tool {
	return mcplib.NewTool("vault_status",
		mcplib.WithDescription("Return a summary of vault health: inbox count, total notes, and number of categories."),
	)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func handleVaultListInbox(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		notes, err := s.vault.ListInbox()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type item struct {
			Path    string `json:"path"`
			Name    string `json:"name"`
			Preview string `json:"preview"`
		}
		result := make([]item, 0, len(notes))
		for _, n := range notes {
			preview := n.Parsed.Content
			if len([]rune(preview)) > 200 {
				preview = string([]rune(preview)[:200])
			}
			result = append(result, item{Path: n.Path, Name: n.Name, Preview: preview})
		}

		out, _ := json.MarshalIndent(result, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultTriageInbox(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		plan, err := s.vault.PlanInboxTriage()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		out, _ := json.MarshalIndent(plan, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultReadNote(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		note, err := s.vault.ReadNote(path)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type result struct {
			Path        string                 `json:"path"`
			Name        string                 `json:"name"`
			Frontmatter map[string]interface{} `json:"frontmatter"`
			Content     string                 `json:"content"`
			Tags        []string               `json:"tags"`
			Links       []string               `json:"links"`
		}
		r := result{
			Path:        note.Path,
			Name:        note.Name,
			Frontmatter: note.Parsed.Frontmatter,
			Content:     note.Parsed.Content,
			Tags:        vault.GetTags(note.Parsed),
			Links:       vault.ExtractWikilinks(note.Parsed.Content),
		}

		out, _ := json.MarshalIndent(r, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultSearch(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		folder := req.GetString("folder", "")
		query := req.GetString("query", "")
		tags := req.GetStringSlice("tags", nil)

		notes, err := s.vault.SearchNotes(vault.SearchOptions{
			Folder: folder,
			Tags:   tags,
			Query:  query,
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type item struct {
			Path    string   `json:"path"`
			Name    string   `json:"name"`
			Tags    []string `json:"tags"`
			Preview string   `json:"preview"`
		}
		result := make([]item, 0, len(notes))
		for _, n := range notes {
			preview := n.Parsed.Content
			if len([]rune(preview)) > 200 {
				preview = string([]rune(preview)[:200])
			}
			result = append(result, item{
				Path:    n.Path,
				Name:    n.Name,
				Tags:    vault.GetTags(n.Parsed),
				Preview: preview,
			})
		}

		out, _ := json.MarshalIndent(result, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultGetCategories(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		categories := s.vault.GetCategories()
		out, _ := json.MarshalIndent(categories, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultGetTemplates(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		names, err := s.vault.GetTemplateList()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type templateInfo struct {
			Name   string   `json:"name"`
			Fields []string `json:"fields"`
		}

		templatesDir := filepath.Join(s.cfg.VaultRoot, filepath.FromSlash(s.cfg.Vault.Templates))

		result := make([]templateInfo, 0, len(names))
		for _, name := range names {
			info := templateInfo{Name: name}
			data, readErr := os.ReadFile(filepath.Join(templatesDir, name+".md"))
			if readErr == nil {
				info.Fields = extractTemplateFields(string(data))
			}
			result = append(result, info)
		}

		out, _ := json.MarshalIndent(result, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultFileNote(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		source, err := req.RequireString("source")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		destination, err := req.RequireString("destination")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		opts := vault.FileNoteOptions{
			Source:      source,
			Destination: destination,
			Content:     req.GetString("content", ""),
			Template:    req.GetString("template", ""),
			MOC:         req.GetString("moc", ""),
			Tags:        req.GetStringSlice("tags", nil),
			Links:       req.GetStringSlice("links", nil),
		}

		fileResult, err := s.vault.FileNote(opts)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type result struct {
			Destination      string `json:"destination"`
			GitCommitMessage string `json:"gitCommitMessage"`
			GitCommitted     bool   `json:"gitCommitted"`
		}
		r := result{
			Destination:      fileResult.Destination,
			GitCommitMessage: fileResult.GitCommitMessage,
			GitCommitted:     fileResult.GitCommitted,
		}

		out, _ := json.MarshalIndent(r, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultCreateNote(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		tags := req.GetStringSlice("tags", nil)
		links := req.GetStringSlice("links", nil)
		moc := req.GetString("moc", "")

		if err := s.vault.CreateNote(path, content, tags, links, moc); err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type result struct {
			Path string `json:"path"`
		}
		out, _ := json.MarshalIndent(result{Path: path}, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultUpdateNote(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		path, err := req.RequireString("path")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		opts := vault.UpdateNoteOptions{
			Path:    path,
			Content: req.GetString("content", ""),
			Tags:    req.GetStringSlice("tags", nil),
			Links:   req.GetStringSlice("links", nil),
		}

		updateResult, err := s.vault.UpdateNote(opts)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type result struct {
			Path         string `json:"path"`
			GitCommitted bool   `json:"gitCommitted"`
		}
		out, _ := json.MarshalIndent(result{
			Path:         updateResult.Path,
			GitCommitted: updateResult.GitCommitted,
		}, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultStatus(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		status, err := s.vault.Status()
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		type result struct {
			InboxCount int `json:"inboxCount"`
			TotalNotes int `json:"totalNotes"`
			Categories int `json:"categories"`
		}
		r := result{
			InboxCount: status.InboxCount,
			TotalNotes: status.TotalNotes,
			Categories: status.Categories,
		}

		out, _ := json.MarshalIndent(r, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

// extractTemplateFields returns the ## heading names from template content.
func extractTemplateFields(content string) []string {
	var fields []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			fields = append(fields, strings.TrimPrefix(line, "## "))
		}
	}
	return fields
}
