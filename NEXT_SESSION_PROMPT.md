# ricket Go Rewrite — Handoff Prompt

## Context

You are continuing a Go rewrite of **ricket** — a CLI + MCP server tool that manages Obsidian
vaults for AI coding agents. The TypeScript v0.1.0 is the reference implementation in `src/`.
The Go rewrite lives alongside it in the same repo (`github.com/AlejandroByrne/ricket`).

**ricket makes zero LLM API calls.** It is purely plumbing — the tool that LLMs call via MCP.

---

## What Has Been Written (Previous Session)

The following Go source files have been written and are on `origin/main`:

| File | Status | Notes |
|------|--------|-------|
| `go.mod` | ✅ Written | Module `github.com/AlejandroByrne/ricket`, go 1.22, deps listed but NOT yet resolved (`go mod tidy` not run) |
| `internal/config/config.go` | ✅ Written | LoadConfig, GenerateDefaultConfig, WriteConfig |
| `internal/vault/frontmatter.go` | ✅ Written | ParseNote, SerializeNote, MergeTags, AddFrontmatterTags, ExtractWikilinks, GetTags |
| `internal/vault/template.go` | ✅ Written | LoadTemplate, ScaffoldNote, MergeContentIntoTemplate |
| `internal/vault/moc.go` | ✅ Written | UpdateMOCFile |
| `internal/vault/index.go` | ✅ Written | VaultIndex with SQLite (modernc.org/sqlite), Init/Rebuild/SearchByTags/SearchContent/GetByFolder/Close |
| `internal/vault/vault.go` | ✅ Written | Vault struct, ListInbox, ReadNote, SearchNotes, FileNote, CreateNote, UpdateMOC, Status |
| `internal/git/git.go` | ✅ Written | GitAudit, IsGitRepo, Commit, CommitFileMove |

---

## What Still Needs to Be Written

### 1. `internal/mcp/server.go` — MCP server setup

```go
package mcp

import (
    "github.com/mark3labs/mcp-go/server"
    "github.com/AlejandroByrne/ricket/internal/config"
    "github.com/AlejandroByrne/ricket/internal/vault"
)

type RicketMCPServer struct {
    vaultRoot string
    vault     *vault.Vault
    cfg       *config.RicketConfig
}

func New(vaultRoot string) *RicketMCPServer

// Start loads config, inits vault, registers all 8 tools, then calls server.ServeStdio
func (s *RicketMCPServer) Start() error
```

### 2. `internal/mcp/tools.go` — All 8 MCP tool definitions and handlers

The 8 tools (names and schemas must match TypeScript exactly):

1. **`vault_list_inbox`** — no params → `[]{ path, name, preview(200chars) }`
2. **`vault_read_note`** — `path:string(required)` → `{ path, name, frontmatter, content, tags, links }`
3. **`vault_search`** — `folder?:string, tags?:string[], query?:string` → `[]{ path, name, tags, preview }`
4. **`vault_get_categories`** — no params → `[]Category`
5. **`vault_get_templates`** — no params → `[]{ name, fields:string[] }` (fields = ## headings)
6. **`vault_file_note`** — `source:string(req), destination:string(req), content?, tags?, links?, moc?, template?` → `{ destination, gitCommitMessage }`
7. **`vault_create_note`** — `path:string(req), content:string(req), tags?, links?, moc?` → `{ path }`
8. **`vault_status`** — no params → `{ inboxCount, totalNotes, categories }`

**For the mcp-go API**, use `github.com/mark3labs/mcp-go v0.32.0`. Key functions:
```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

s := server.NewMCPServer("ricket", "0.1.0")

tool := mcp.NewTool("vault_list_inbox",
    mcp.WithDescription("..."),
)
// For string params:
mcp.WithString("path", mcp.Required(), mcp.Description("..."))
// For optional string params:
mcp.WithString("folder", mcp.Description("..."))

s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Access args: req.Params.Arguments["path"].(string)
    result, _ := json.MarshalIndent(data, "", "  ")
    return mcp.NewToolResultText(string(result)), nil
})

// Start serving:
if err := server.ServeStdio(s); err != nil {
    return err
}
```

**For arrays** (tags, links), since mcp-go may not have WithArray in v0.32.0,
accept them as `[]interface{}` in the handler and convert:
```go
func argsToStringSlice(args map[string]any, key string) []string {
    val, ok := args[key]
    if !ok { return nil }
    if arr, ok := val.([]interface{}); ok {
        result := make([]string, 0, len(arr))
        for _, v := range arr { if s, ok := v.(string); ok { result = append(result, s) } }
        return result
    }
    return nil
}
```

For the tool schema's array properties, after creating the tool with `mcp.NewTool(...)`,
you may need to add array properties directly to `tool.InputSchema.Properties` if
`mcp.WithArray` doesn't exist:
```go
if tool.InputSchema.Properties == nil {
    tool.InputSchema.Properties = map[string]interface{}{}
}
tool.InputSchema.Properties["tags"] = map[string]interface{}{
    "type": "array",
    "items": map[string]interface{}{"type": "string"},
    "description": "Tags to filter by",
}
```

### 3. `cmd/ricket/main.go` — CLI entrypoint

Three commands using `github.com/spf13/cobra`:

```
ricket init [path]      — generate default ricket.yaml, create .ricket dir
ricket serve            — start MCP server on stdio
ricket status           — print vault stats
```

Global flag: `-r, --vault-root <path>` (default: current directory)

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "github.com/spf13/cobra"
    "github.com/AlejandroByrne/ricket/internal/config"
    "github.com/AlejandroByrne/ricket/internal/vault"
    ricketmcp "github.com/AlejandroByrne/ricket/internal/mcp"
)

var vaultRoot string

func main() {
    root := &cobra.Command{
        Use:     "ricket",
        Short:   "Vault-powered context engine for AI coding agents",
        Version: "0.1.0",
    }
    root.PersistentFlags().StringVarP(&vaultRoot, "vault-root", "r", "", "Vault root (default: cwd)")

    root.AddCommand(initCmd(), serveCmd(), statusCmd())
    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

**init command**: Check ricket.yaml doesn't exist, scan for PARA folders (Projects/Areas/Resources/Archive), check _templates, generate default config, create .ricket/, write ricket.yaml.

**serve command**: Check ricket.yaml exists, print `ricket MCP server running (vault: ...)` to stderr, call `ricketmcp.New(root).Start()`.

**status command**: LoadConfig, Vault.Status(), print stats, list inbox files if any.

### 4. Tests

Write table-driven Go tests for:

- `internal/config/config_test.go` — LoadConfig (valid, invalid, missing), GenerateDefaultConfig
- `internal/vault/frontmatter_test.go` — ParseNote (with/without FM), SerializeNote, MergeTags, AddFrontmatterTags, ExtractWikilinks
- `internal/vault/template_test.go` — ScaffoldNote variable substitution, MergeContentIntoTemplate sections
- `internal/vault/vault_test.go` — FileNote (mock fs using os.TempDir), CreateNote, Status

Use `testing` + `os.TempDir()` for filesystem tests. No external test dependencies needed.

### 5. Build infrastructure

**Makefile**:
```makefile
GO := go
BINARY := bin/ricket

.PHONY: build test lint clean

build:
	$(GO) build -o $(BINARY) ./cmd/ricket

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

clean:
	rm -rf bin/ .ricket/

release:
	goreleaser release --clean
```

**.github/workflows/ci.yml**:
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: ["1.22"]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
      - run: go mod download
      - run: go test ./...
      - run: go build ./cmd/ricket
```

**.goreleaser.yml**:
```yaml
project_name: ricket
version: 2
builds:
  - id: ricket
    main: ./cmd/ricket
    binary: ricket
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    env: [CGO_ENABLED=0]
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
checksum:
  name_template: "checksums.txt"
```

---

## After Writing All Files

Run these commands in order:
```bash
export PATH="/opt/homebrew/bin:$PATH"  # Go is at /opt/homebrew/bin/go (v1.26.1)
cd /Users/ale/ricket
go mod tidy                             # resolves all deps, generates go.sum
go build ./cmd/ricket                  # should produce ./ricket binary
go test ./...                          # should pass all tests
```

If `mcp-go` API doesn't match (e.g., `mcp.WithArray` missing, `server.ServeStdio` signature different),
check the actual installed source at `$(go env GOPATH)/pkg/mod/github.com/mark3labs/`.

---

## Key Architecture Notes

- **Config**: `vault.root` in YAML is OPTIONAL — if absent, use the directory containing ricket.yaml
- **Paths**: Always use `filepath.Join` internally, convert to forward slashes for MCP responses
- **Tags in frontmatter**: YAML arrays under `tags` key — merge, never overwrite
- **MOC update**: best-effort (don't fail the whole operation if MOC doesn't exist)
- **Git commits**: best-effort (don't fail if not a git repo)
- **SQLite**: uses `modernc.org/sqlite` pure Go driver — import with `_ "modernc.org/sqlite"`, driver name is `"sqlite"`
- **Template vars**: `<% tp.file.title %>` → title, `<% tp.date.now("YYYY-MM-DD") %>` → today's date
- **Wikilinks**: `[[Note Name]]` or `[[Note Name|display]]` — extract the part before `|`

---

## Go is installed

```
/opt/homebrew/bin/go  — version 1.26.1 darwin/arm64
```

Always prefix PATH: `export PATH="/opt/homebrew/bin:$PATH"` before running Go commands.

---

## Permissions

`.claude/settings.local.json` has `"defaultMode": "bypassPermissions"` — no approval prompts needed.

---

## Commit and Push Protocol

After completing each major package:
```bash
git add -A
git commit -m "feat: <description> (AI-assisted)"
git push origin main
```
