// Package mcp — white-box tests for MCP tool handlers.
// Tests call handler functions directly (no subprocess needed) using the
// testdata/vault fixture.
package mcp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AlejandroByrne/ricket/internal/config"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func testdataPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "vault"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// makeServer wires up a RicketMCPServer against testdata/vault.
func makeServer(t *testing.T) *RicketMCPServer {
	t.Helper()
	vaultPath := testdataPath(t)
	cfg, err := config.LoadConfig(vaultPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	v := vault.New(cfg)
	s := &RicketMCPServer{
		vaultRoot: vaultPath,
		vault:     v,
		cfg:       cfg,
	}
	t.Cleanup(func() { s.vault.Close() })
	return s
}

// callHandler invokes a handler and returns the JSON-decoded response body.
// Fails the test if the handler returns a Go error. Returns (obj, isError).
func callHandler(t *testing.T, h mcpserver.ToolHandlerFunc, args map[string]any) (map[string]any, bool) {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("handler returned empty content")
	}
	tc, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("first content item is not TextContent: %T", result.Content[0])
	}
	if result.IsError {
		return nil, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		// Might be a JSON array — wrap it
		var arr []any
		if err2 := json.Unmarshal([]byte(tc.Text), &arr); err2 != nil {
			t.Fatalf("unmarshal response: %v\nbody: %s", err, tc.Text)
		}
		return map[string]any{"_array": arr}, false
	}
	return out, false
}

// callHandlerArray calls the handler and returns the response as a JSON array.
func callHandlerArray(t *testing.T, h mcpserver.ToolHandlerFunc, args map[string]any) []any {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	tc, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("not TextContent: %T", result.Content[0])
	}
	var arr []any
	if err := json.Unmarshal([]byte(tc.Text), &arr); err != nil {
		t.Fatalf("unmarshal array: %v\nbody: %s", err, tc.Text)
	}
	return arr
}

// isErrorResult returns true if the handler result has IsError set.
func isErrorResult(t *testing.T, h mcpserver.ToolHandlerFunc, args map[string]any) bool {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	result, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	return result.IsError
}

// ── vault_list_inbox ──────────────────────────────────────────────────────────

func TestHandler_ListInbox(t *testing.T) {
	s := makeServer(t)
	h := handleVaultListInbox(s)

	items := callHandlerArray(t, h, nil)
	if len(items) != 3 {
		t.Errorf("expected 3 inbox notes, got %d", len(items))
	}

	// Each item should have path, name, preview
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("item is not object: %T", raw)
		}
		if item["path"] == "" {
			t.Error("item missing path")
		}
		if item["name"] == "" {
			t.Error("item missing name")
		}
		_, hasPreview := item["preview"]
		if !hasPreview {
			t.Error("item missing preview key")
		}
		// Preview must not exceed 200 runes
		preview, _ := item["preview"].(string)
		if len([]rune(preview)) > 200 {
			t.Errorf("preview exceeds 200 runes: %d", len([]rune(preview)))
		}
	}
}

func TestHandler_TriageInbox(t *testing.T) {
	s := makeServer(t)
	h := handleVaultTriageInbox(s)

	resp, isErr := callHandler(t, h, nil)
	if isErr {
		t.Fatal("unexpected error result")
	}

	proposals, ok := resp["proposals"].([]any)
	if !ok {
		t.Fatalf("expected proposals array, got %T", resp["proposals"])
	}
	unresolved, ok := resp["unresolved"].([]any)
	if !ok {
		t.Fatalf("expected unresolved array, got %T", resp["unresolved"])
	}
	if len(proposals)+len(unresolved) != 3 {
		t.Errorf("expected 3 total triage outcomes, got proposals=%d unresolved=%d", len(proposals), len(unresolved))
	}
	if _, ok := resp["generatedAt"].(string); !ok {
		t.Fatalf("expected generatedAt timestamp in response")
	}
}

// ── vault_read_note ───────────────────────────────────────────────────────────

func TestHandler_ReadNote_Valid(t *testing.T) {
	s := makeServer(t)
	h := handleVaultReadNote(s)

	resp, isErr := callHandler(t, h, map[string]any{
		"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
	})
	if isErr {
		t.Fatal("unexpected error result")
	}

	if resp["path"] == "" {
		t.Error("missing path in response")
	}
	if resp["name"] != "use-sqlite-for-index" {
		t.Errorf("name = %q", resp["name"])
	}
	if resp["content"] == "" {
		t.Error("missing content")
	}

	tags, ok := resp["tags"].([]any)
	if !ok || len(tags) == 0 {
		t.Errorf("expected tags array, got %T %v", resp["tags"], resp["tags"])
	}

	links, ok := resp["links"].([]any)
	if !ok {
		t.Errorf("expected links array, got %T", resp["links"])
	}
	_ = links // may be empty or populated
}

func TestHandler_ReadNote_Missing(t *testing.T) {
	s := makeServer(t)
	h := handleVaultReadNote(s)

	if !isErrorResult(t, h, map[string]any{"path": "Inbox/does-not-exist.md"}) {
		t.Error("expected error result for missing note")
	}
}

func TestHandler_ReadNote_TraversalPath(t *testing.T) {
	s := makeServer(t)
	h := handleVaultReadNote(s)

	for _, p := range []string{"../../etc/passwd", "/etc/passwd"} {
		if !isErrorResult(t, h, map[string]any{"path": p}) {
			t.Errorf("expected error result for traversal path %q", p)
		}
	}
}

func TestHandler_ReadNote_MissingRequiredArg(t *testing.T) {
	s := makeServer(t)
	h := handleVaultReadNote(s)

	if !isErrorResult(t, h, map[string]any{}) {
		t.Error("expected error when 'path' arg is missing")
	}
}

// ── vault_search ──────────────────────────────────────────────────────────────

func TestHandler_Search_ByFolder(t *testing.T) {
	s := makeServer(t)
	h := handleVaultSearch(s)

	items := callHandlerArray(t, h, map[string]any{
		"folder": "Areas/Engineering/decisions/",
	})
	if len(items) < 2 {
		t.Errorf("expected >= 2 results, got %d", len(items))
	}
}

func TestHandler_Search_ByTags(t *testing.T) {
	s := makeServer(t)
	h := handleVaultSearch(s)

	items := callHandlerArray(t, h, map[string]any{
		"tags": []any{"decision", "acme"},
	})
	if len(items) < 2 {
		t.Errorf("expected >= 2 results for decision+acme, got %d", len(items))
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		tags, _ := item["tags"].([]any)
		hasDecision, hasAcme := false, false
		for _, tg := range tags {
			switch tg.(string) {
			case "decision":
				hasDecision = true
			case "acme":
				hasAcme = true
			}
		}
		if !hasDecision || !hasAcme {
			t.Errorf("note %q missing expected tags, got %v", item["path"], tags)
		}
	}
}

func TestHandler_Search_ByQuery(t *testing.T) {
	s := makeServer(t)
	h := handleVaultSearch(s)

	items := callHandlerArray(t, h, map[string]any{"query": "SQLite"})
	if len(items) == 0 {
		t.Error("expected results for query 'SQLite'")
	}
}

func TestHandler_Search_NoResults(t *testing.T) {
	s := makeServer(t)
	h := handleVaultSearch(s)

	items := callHandlerArray(t, h, map[string]any{"query": "xyznonexistentterm12345"})
	if len(items) != 0 {
		t.Errorf("expected 0 results, got %d", len(items))
	}
}

func TestHandler_Search_FolderPrefixSafety(t *testing.T) {
	// "Areas/Engineering" should NOT match "Areas/Engineering2/" if it existed.
	// Verify all returned paths are strictly inside the requested folder.
	s := makeServer(t)
	h := handleVaultSearch(s)

	items := callHandlerArray(t, h, map[string]any{
		"folder": "Areas/Engineering/decisions/",
	})
	for _, raw := range items {
		item := raw.(map[string]any)
		path, _ := item["path"].(string)
		if !strings.HasPrefix(path, "Areas/Engineering/decisions/") {
			t.Errorf("search result %q outside requested folder", path)
		}
	}
}

// ── vault_get_categories ──────────────────────────────────────────────────────

func TestHandler_GetCategories(t *testing.T) {
	s := makeServer(t)
	h := handleVaultGetCategories(s)

	cats := callHandlerArray(t, h, nil)
	if len(cats) != 5 {
		t.Errorf("expected 5 categories, got %d", len(cats))
	}
	names := make([]string, 0, len(cats))
	for _, raw := range cats {
		cat, _ := raw.(map[string]any)
		// Category struct has no json tags, so fields are marshaled with Go names (capitalized)
		n, _ := cat["Name"].(string)
		names = append(names, n)
	}
	for _, want := range []string{"acme-decision", "acme-concept", "acme-meeting", "acme-project", "learning"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("category %q not found in %v", want, names)
		}
	}
}

// ── vault_get_templates ───────────────────────────────────────────────────────

func TestHandler_GetTemplates(t *testing.T) {
	s := makeServer(t)
	h := handleVaultGetTemplates(s)

	templates := callHandlerArray(t, h, nil)
	if len(templates) < 4 {
		t.Errorf("expected >= 4 templates, got %d", len(templates))
	}

	for _, raw := range templates {
		tmpl, _ := raw.(map[string]any)
		if tmpl["name"] == "" {
			t.Error("template missing name")
		}
		fields, ok := tmpl["fields"].([]any)
		if !ok {
			t.Errorf("template %q missing fields array", tmpl["name"])
		}
		if len(fields) == 0 {
			t.Errorf("template %q has no fields (expected ## headings)", tmpl["name"])
		}
	}
}

// ── vault_create_note ─────────────────────────────────────────────────────────

func TestHandler_CreateNote_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	if err := copyDirForTest(testdataPath(t), tmpDir); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s := &RicketMCPServer{vaultRoot: tmpDir, vault: vault.New(cfg), cfg: cfg}
	t.Cleanup(func() { s.vault.Close() })

	h := handleVaultCreateNote(s)
	resp, isErr := callHandler(t, h, map[string]any{
		"path":    "Notes/new-note.md",
		"content": "# New Note\n\nSome content.",
		"tags":    []any{"note", "test"},
	})
	if isErr {
		t.Fatal("unexpected error result")
	}
	if resp["path"] != "Notes/new-note.md" {
		t.Errorf("path = %q", resp["path"])
	}
}

func TestHandler_CreateNote_MissingPath(t *testing.T) {
	s := makeServer(t)
	h := handleVaultCreateNote(s)
	if !isErrorResult(t, h, map[string]any{"content": "hi"}) {
		t.Error("expected error when path missing")
	}
}

func TestHandler_CreateNote_MissingContent(t *testing.T) {
	s := makeServer(t)
	h := handleVaultCreateNote(s)
	if !isErrorResult(t, h, map[string]any{"path": "Notes/x.md"}) {
		t.Error("expected error when content missing")
	}
}

func TestHandler_CreateNote_TraversalPath(t *testing.T) {
	s := makeServer(t)
	h := handleVaultCreateNote(s)
	if !isErrorResult(t, h, map[string]any{
		"path":    "../../evil.md",
		"content": "bad",
	}) {
		t.Error("expected error for traversal path")
	}
}

// ── vault_file_note ───────────────────────────────────────────────────────────

func TestHandler_FileNote_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	if err := copyDirForTest(testdataPath(t), tmpDir); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s := &RicketMCPServer{vaultRoot: tmpDir, vault: vault.New(cfg), cfg: cfg}
	t.Cleanup(func() { s.vault.Close() })

	h := handleVaultFileNote(s)
	resp, isErr := callHandler(t, h, map[string]any{
		"source":      "Inbox/raw-capture.md",
		"destination": "Areas/Engineering/decisions/use-query-builder.md",
		"tags":        []any{"decision", "acme"},
	})
	if isErr {
		t.Fatal("unexpected error result")
	}

	if resp["destination"] != "Areas/Engineering/decisions/use-query-builder.md" {
		t.Errorf("destination = %q", resp["destination"])
	}
	if resp["gitCommitMessage"] == "" {
		t.Error("gitCommitMessage should not be empty")
	}
	// gitCommitted key should exist (bool, may be false if no git repo)
	if _, exists := resp["gitCommitted"]; !exists {
		t.Error("response missing gitCommitted field")
	}
}

func TestHandler_FileNote_MissingSource(t *testing.T) {
	s := makeServer(t)
	h := handleVaultFileNote(s)
	if !isErrorResult(t, h, map[string]any{
		"source":      "Inbox/does-not-exist.md",
		"destination": "Notes/x.md",
	}) {
		t.Error("expected error for missing source")
	}
}

func TestHandler_FileNote_TraversalSource(t *testing.T) {
	s := makeServer(t)
	h := handleVaultFileNote(s)
	if !isErrorResult(t, h, map[string]any{
		"source":      "../../etc/passwd",
		"destination": "Notes/x.md",
	}) {
		t.Error("expected error for traversal source")
	}
}

func TestHandler_FileNote_MissingRequiredArgs(t *testing.T) {
	s := makeServer(t)
	h := handleVaultFileNote(s)
	// Missing destination
	if !isErrorResult(t, h, map[string]any{"source": "Inbox/x.md"}) {
		t.Error("expected error when destination missing")
	}
	// Missing source
	if !isErrorResult(t, h, map[string]any{"destination": "Notes/x.md"}) {
		t.Error("expected error when source missing")
	}
}

// ── vault_update_note ─────────────────────────────────────────────────────────

func TestHandler_UpdateNote_Content(t *testing.T) {
	tmpDir := t.TempDir()
	if err := copyDirForTest(testdataPath(t), tmpDir); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s := &RicketMCPServer{vaultRoot: tmpDir, vault: vault.New(cfg), cfg: cfg}
	t.Cleanup(func() { s.vault.Close() })

	h := handleVaultUpdateNote(s)
	resp, isErr := callHandler(t, h, map[string]any{
		"path":    "Areas/Engineering/decisions/use-sqlite-for-index.md",
		"content": "# Updated\n\nNew body content.",
	})
	if isErr {
		t.Fatal("unexpected error result")
	}
	if resp["path"] != "Areas/Engineering/decisions/use-sqlite-for-index.md" {
		t.Errorf("path = %q", resp["path"])
	}
	if _, exists := resp["gitCommitted"]; !exists {
		t.Error("response missing gitCommitted field")
	}

	// Verify the note was actually updated
	rh := handleVaultReadNote(s)
	readResp, readErr := callHandler(t, rh, map[string]any{
		"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
	})
	if readErr {
		t.Fatal("read after update failed")
	}
	content, _ := readResp["content"].(string)
	if !strings.Contains(content, "New body content.") {
		t.Errorf("updated content not persisted, got: %q", content)
	}
}

func TestHandler_UpdateNote_AddTags(t *testing.T) {
	tmpDir := t.TempDir()
	if err := copyDirForTest(testdataPath(t), tmpDir); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s := &RicketMCPServer{vaultRoot: tmpDir, vault: vault.New(cfg), cfg: cfg}
	t.Cleanup(func() { s.vault.Close() })

	h := handleVaultUpdateNote(s)
	_, isErr := callHandler(t, h, map[string]any{
		"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
		"tags": []any{"reviewed", "pinned"},
	})
	if isErr {
		t.Fatal("unexpected error result")
	}

	// Verify tags were added
	rh := handleVaultReadNote(s)
	readResp, _ := callHandler(t, rh, map[string]any{
		"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
	})
	tags, _ := readResp["tags"].([]any)
	hasReviewed := false
	for _, tg := range tags {
		if tg.(string) == "reviewed" {
			hasReviewed = true
		}
	}
	if !hasReviewed {
		t.Errorf("'reviewed' tag not found after update, got: %v", tags)
	}
}

func TestHandler_UpdateNote_MissingPath(t *testing.T) {
	s := makeServer(t)
	h := handleVaultUpdateNote(s)
	if !isErrorResult(t, h, map[string]any{"content": "hi"}) {
		t.Error("expected error when path missing")
	}
}

func TestHandler_UpdateNote_NothingProvided(t *testing.T) {
	s := makeServer(t)
	h := handleVaultUpdateNote(s)
	if !isErrorResult(t, h, map[string]any{
		"path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
	}) {
		t.Error("expected error when no content/tags/links provided")
	}
}

func TestHandler_UpdateNote_MissingNote(t *testing.T) {
	s := makeServer(t)
	h := handleVaultUpdateNote(s)
	if !isErrorResult(t, h, map[string]any{
		"path":    "Inbox/does-not-exist.md",
		"content": "new content",
	}) {
		t.Error("expected error for missing note")
	}
}

func TestHandler_UpdateNote_TraversalPath(t *testing.T) {
	s := makeServer(t)
	h := handleVaultUpdateNote(s)
	if !isErrorResult(t, h, map[string]any{
		"path":    "../../etc/passwd",
		"content": "bad",
	}) {
		t.Error("expected error for traversal path")
	}
}

// ── vault_status ──────────────────────────────────────────────────────────────

func TestHandler_Status(t *testing.T) {
	s := makeServer(t)
	h := handleVaultStatus(s)

	resp, isErr := callHandler(t, h, nil)
	if isErr {
		t.Fatal("unexpected error result")
	}

	inbox, ok := resp["inboxCount"].(float64)
	if !ok {
		t.Fatalf("inboxCount type: %T %v", resp["inboxCount"], resp["inboxCount"])
	}
	if int(inbox) != 3 {
		t.Errorf("inboxCount = %d, want 3", int(inbox))
	}

	total, _ := resp["totalNotes"].(float64)
	if int(total) < 10 {
		t.Errorf("totalNotes = %d, want >= 10", int(total))
	}

	cats, _ := resp["categories"].(float64)
	if int(cats) != 5 {
		t.Errorf("categories = %d, want 5", int(cats))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func copyDirForTest(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
