package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AlejandroByrne/ricket/internal/config"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

// registerTools registers all ricket MCP tools on srv (normal mode).
func registerTools(srv *mcpserver.MCPServer, s *RicketMCPServer) {
	// Setup / migration tools — available in all modes
	registerMigrationTools(srv, s)

	// Vault operation tools — require a loaded config
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
	srv.AddTool(toolListSources(), handleVaultListSources(s))

	// Prompts
	registerPrompts(srv, s)
}

// registerMigrationTools registers only the setup tools used before ricket.yaml exists.
func registerMigrationTools(srv *mcpserver.MCPServer, s *RicketMCPServer) {
	srv.AddTool(toolAnalyze(), handleVaultAnalyze(s))
	srv.AddTool(toolWriteConfig(), handleVaultWriteConfig(s))

	// Prompts available in both modes
	registerPrompts(srv, s)
}

// ── Tool definitions ──────────────────────────────────────────────────────────

func toolListInbox() mcplib.Tool {
	return mcplib.NewTool("vault_list_inbox",
		mcplib.WithDescription("List all notes in the vault inbox. Returns path, name, and a 200-character preview for each note."),
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolTriageInbox() mcplib.Tool {
	return mcplib.NewTool("vault_triage_inbox",
		mcplib.WithDescription("Analyze Inbox notes and propose filing actions with category, destination path, confidence, and matched signals. Does not move files."),
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolReadNote() mcplib.Tool {
	return mcplib.NewTool("vault_read_note",
		mcplib.WithDescription("Read a single note by its relative vault path. Returns path, name, frontmatter, content, tags, and wikilinks. For notes from reference sources, use @source-name/path format."),
		mcplib.WithReadOnlyHintAnnotation(true),
		mcplib.WithString("path",
			mcplib.Required(),
			mcplib.Description("Relative path of the note within the vault (e.g. 'Inbox/my-note.md') or a source reference (e.g. '@standards/adr-001.md')"),
		),
	)
}

func toolSearch() mcplib.Tool {
	t := mcplib.NewTool("vault_search",
		mcplib.WithDescription("Search notes by folder, tags, and/or full-text query. All filters are combined (AND logic). When a text query is provided, also searches configured reference sources and returns results with a 'source' field."),
		mcplib.WithReadOnlyHintAnnotation(true),
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
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolGetTemplates() mcplib.Tool {
	return mcplib.NewTool("vault_get_templates",
		mcplib.WithDescription("Return all available templates with their names and section headings (## fields)."),
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolFileNote() mcplib.Tool {
	t := mcplib.NewTool("vault_file_note",
		mcplib.WithDescription("Move a note from source to destination, optionally applying a template, tags, links, and a MOC update. Use source_action to decide what happens to the original inbox note. Returns the new path and a suggested git commit message."),
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
		mcplib.WithString("source_action",
			mcplib.Description("What to do with the source note after filing: 'delete' (default), 'archive', or 'keep'"),
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
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolListSources() mcplib.Tool {
	return mcplib.NewTool("vault_list_sources",
		mcplib.WithDescription("List configured reference sources. Each source is a read-only directory whose notes appear in search results and can be read with @source-name/path."),
		mcplib.WithReadOnlyHintAnnotation(true),
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

		var note vault.VaultNote
		if strings.HasPrefix(path, "@") {
			// Source reference: @source-name/relative/path.md
			rest := strings.TrimPrefix(path, "@")
			idx := strings.Index(rest, "/")
			if idx < 0 {
				return mcplib.NewToolResultError("invalid source path: expected @source-name/path"), nil
			}
			srcName := rest[:idx]
			srcPath := rest[idx+1:]
			note, err = s.vault.ReadSourceNote(srcName, srcPath)
		} else {
			note, err = s.vault.ReadNote(path)
		}
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
			Source  string   `json:"source,omitempty"`
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

		// Also search reference sources when a text query is provided
		if query != "" {
			for _, sn := range s.vault.SearchSources(query) {
				preview := sn.Parsed.Content
				if len([]rune(preview)) > 200 {
					preview = string([]rune(preview)[:200])
				}
				result = append(result, item{
					Path:    sn.VaultNote.Path,
					Name:    sn.Name,
					Tags:    vault.GetTags(sn.Parsed),
					Preview: preview,
					Source:  sn.Source,
				})
			}
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
			SourceAction: req.GetString("source_action", ""),
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

func handleVaultListSources(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		type sourceInfo struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			Available bool   `json:"available"`
		}
		var result []sourceInfo
		for _, src := range s.cfg.Sources {
			_, err := os.Stat(src.ResolvedPath)
			result = append(result, sourceInfo{
				Name:      src.Name,
				Path:      src.ResolvedPath,
				Available: err == nil,
			})
		}
		out, _ := json.MarshalIndent(result, "", "  ")
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

// ── Setup / migration tools ───────────────────────────────────────────────────

func toolAnalyze() mcplib.Tool {
	return mcplib.NewTool("vault_analyze",
		mcplib.WithDescription(`Analyze vault structure without requiring ricket.yaml. Scans folder tree, frontmatter tags, naming conventions, and templates to produce a complete picture of the vault. Returns inferred categories with confidence scores and reasoning strings. Use this as the first step of the migration or new-vault setup flow, then call vault_write_config with the agent-generated config.`),
		mcplib.WithReadOnlyHintAnnotation(true),
	)
}

func toolWriteConfig() mcplib.Tool {
	return mcplib.NewTool("vault_write_config",
		mcplib.WithDescription(`Write ricket.yaml and VAULT_GUIDE.md to the vault root. Requires overwrite:true if ricket.yaml already exists. Set scaffold:true to create any missing category folders, template stubs, and MOC files after writing. After this call succeeds, restart the MCP server to load the new config.`),
		mcplib.WithString("config_yaml",
			mcplib.Required(),
			mcplib.Description("Full ricket.yaml content. May include YAML comments (# lines) explaining category reasoning. Must follow the ricket.yaml schema (vault, categories, mcp sections)."),
		),
		mcplib.WithString("guide_content",
			mcplib.Required(),
			mcplib.Description("Markdown content for VAULT_GUIDE.md — explains vault structure, naming conventions, tagging rules, and filing guidelines so AI agents understand this vault."),
		),
		mcplib.WithString("guide_path",
			mcplib.Description(`Relative path for the guide file within the vault (default: "VAULT_GUIDE.md")`),
		),
		mcplib.WithBoolean("overwrite",
			mcplib.Description("Allow overwriting an existing ricket.yaml (default: false). Required when ricket.yaml already exists."),
		),
		mcplib.WithBoolean("scaffold",
			mcplib.Description("After writing ricket.yaml, create any missing category folders, template stub files, and MOC files (default: false). Recommended for new vaults."),
		),
	)
}

func handleVaultAnalyze(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		analysis, err := vault.AnalyzeVaultRoot(s.vaultRoot)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		out, _ := json.MarshalIndent(analysis, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func handleVaultWriteConfig(s *RicketMCPServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		configYAML, err := req.RequireString("config_yaml")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		guideContent, err := req.RequireString("guide_content")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		guidePath := req.GetString("guide_path", "VAULT_GUIDE.md")
		overwrite := req.GetBool("overwrite", false)
		scaffold := req.GetBool("scaffold", false)

		if err := validateRelPath(guidePath); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("invalid guide_path: %v", err)), nil
		}

		configPath := filepath.Join(s.vaultRoot, "ricket.yaml")
		if _, statErr := os.Stat(configPath); statErr == nil && !overwrite {
			return mcplib.NewToolResultError("ricket.yaml already exists — set overwrite:true to replace it"), nil
		}

		if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to write ricket.yaml: %v", err)), nil
		}

		guideAbsPath := filepath.Join(s.vaultRoot, filepath.FromSlash(guidePath))
		if err := os.MkdirAll(filepath.Dir(guideAbsPath), 0o755); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to create guide directory: %v", err)), nil
		}
		if err := os.WriteFile(guideAbsPath, []byte(guideContent), 0o644); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to write %s: %v", guidePath, err)), nil
		}

		scaffolded := false
		if scaffold {
			cfg, loadErr := config.LoadConfig(s.vaultRoot)
			if loadErr == nil {
				if scaffoldErr := vault.ScaffoldVault(cfg); scaffoldErr != nil {
					return mcplib.NewToolResultError(fmt.Sprintf("config written but scaffold failed: %v", scaffoldErr)), nil
				}
				scaffolded = true
			}
		}

		type writeResult struct {
			ConfigPath string `json:"configPath"`
			GuidePath  string `json:"guidePath"`
			Scaffolded bool   `json:"scaffolded"`
			NextStep   string `json:"nextStep"`
		}
		out, _ := json.MarshalIndent(writeResult{
			ConfigPath: "ricket.yaml",
			GuidePath:  guidePath,
			Scaffolded: scaffolded,
			NextStep:   "Restart the MCP server (reload your editor window) to load the new config and enable all vault tools.",
		}, "", "  ")
		return mcplib.NewToolResultText(string(out)), nil
	}
}

// validateRelPath rejects empty, absolute, and traversal paths for guide_path.
func validateRelPath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if filepath.IsAbs(filepath.FromSlash(p)) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return fmt.Errorf("path must be relative")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path must not contain path traversal (..)")
	}
	return nil
}

// ── Prompts ─────────────────────────────────────────────────────────────────

func registerPrompts(srv *mcpserver.MCPServer, s *RicketMCPServer) {
	srv.AddPrompt(mcplib.Prompt{
		Name:        "setup-vault",
		Description: "Initialize or reconfigure your vault with ricket. Analyzes vault structure, detects PKM system, and generates ricket.yaml configuration.",
	}, handleSetupVaultPrompt(s))
}

func handleSetupVaultPrompt(s *RicketMCPServer) mcpserver.PromptHandlerFunc {
	return func(ctx context.Context, req mcplib.GetPromptRequest) (*mcplib.GetPromptResult, error) {
		return &mcplib.GetPromptResult{
			Description: "Ricket vault setup flow",
			Messages: []mcplib.PromptMessage{
				{
					Role: mcplib.RoleUser,
					Content: mcplib.TextContent{
						Type: "text",
						Text: `Set up my vault with ricket. Follow these steps:

1. Call vault_analyze to inspect the vault structure, detect the PKM system, and identify categories.
2. Review the analysis results — especially the detected PKM system, inferred categories, tag taxonomy, and link structure.
3. Generate a ricket.yaml configuration that preserves my existing organizational patterns.
4. Write a VAULT_GUIDE.md that explains my vault's structure and filing rules for AI agents.
5. Call vault_write_config with the generated config and guide. Set scaffold:true if this is a new vault.
6. Tell me to reload the editor window so the MCP server picks up the new config.`,
					},
				},
			},
		}, nil
	}
}
