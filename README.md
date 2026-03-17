```text
██████╗ ██╗ ██████╗██╗  ██╗███████╗████████╗
██╔══██╗██║██╔════╝██║ ██╔╝██╔════╝╚══██╔══╝
██████╔╝██║██║     █████╔╝ █████╗     ██║
██╔══██╗██║██║     ██╔═██╗ ██╔══╝     ██║
██║  ██║██║╚██████╗██║  ██╗███████╗   ██║
╚═╝  ╚═╝╚═╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝

Inbox -> Triage -> Filed -> Committed
```

# ricket

Vault-powered context engine for AI coding agents.

Ricket bridges your [Obsidian](https://obsidian.md) vault and your AI coding assistant (Claude Code, GitHub Copilot, Cursor). Two jobs:

1. **Triage** — take raw inbox notes, voice transcripts, meeting dumps and file them into the right place with the right template, tags, links, and MOC entry.
2. **Context** — expose your vault's decisions, standards, concepts, and project history as structured context via MCP so your agent always knows your stack.

Ricket makes **zero LLM API calls**. It is pure plumbing — the tool the LLM calls, not the other way around.

---

## Requirements

- Go 1.22+ (for building from source)
- An Obsidian vault organised with [PARA](https://fortelabs.com/blog/para/) or similar
- Recommended Obsidian plugins: **Templater**, **Dataview**, **Obsidian Git**, **Tag Wrangler**, **Omnisearch**

---

## Installation

```bash
go install github.com/AlejandroByrne/ricket/cmd/ricket@latest
```

Or build from source:

```bash
git clone https://github.com/AlejandroByrne/ricket
cd ricket
make build          # → bin/ricket
```

---

## Setup

### 1. Initialise your vault

```bash
ricket init /path/to/your/obsidian-vault
```

This runs an interactive wizard that asks about your organisations, note categories, and inbox habits, then writes `ricket.yaml` and offers to set the vault as your default.

`ricket init` now also scaffolds your vault structure on first run:
- Creates vault folders (`Inbox/`, `Archive/`, `_templates/` by default)
- Creates each configured category folder
- Creates missing category templates in `_templates/`
- Creates missing category MOC files
- Optionally writes `.vscode/mcp.json` in the selected vault for GitHub Copilot

### 2. Verify

```bash
ricket status
```

```
Vault:       /Users/alice/obsidian-vault
Total notes: 847
Inbox:       3 notes
Categories:  8

Inbox:
  - Inbox/2026-03-17-sync.md  [meeting, acme]
  - Inbox/raw-capture.md
  - Inbox/learning-rust.md    [learning]
```

---

## Vault root resolution

`ricket serve` resolves the vault root in this order:

| Priority | Source |
|----------|--------|
| 1 | `--vault-root` CLI flag |
| 2 | `RICKET_VAULT_ROOT` environment variable |
| 3 | `default_vault` in `~/.config/ricket/config.yaml` |
| 4 | Current working directory |

Set the default once:

```bash
ricket config set-default /path/to/vault
# or
ricket init  # wizard offers to set it at the end
```

Validate your vault configuration at any time:

```bash
ricket config validate --vault-root /path/to/vault
```

This checks that all required directories exist and that every category's template and MOC file are in place.

---

## Adding to Claude Code

Add to `~/.claude/mcp.json` (global) or `.claude/mcp.json` (project):

```json
{
  "mcpServers": {
    "ricket": {
      "command": "ricket",
      "args": ["serve"],
      "env": {
        "RICKET_VAULT_ROOT": "/absolute/path/to/vault"
      }
    }
  }
}
```

If you've set a default vault via `ricket config set-default`, you can omit the env var:

```json
{
  "mcpServers": {
    "ricket": {
      "command": "ricket",
      "args": ["serve"]
    }
  }
}
```

---

## Adding to GitHub Copilot (VS Code)

Recommended: generate it automatically:

```bash
ricket mcp init-vscode /path/to/your/code-workspace --vault-root /path/to/vault
```

This writes `.vscode/mcp.json` with an absolute `command` path, which avoids common VS Code errors like `spawn ricket ENOENT` when `ricket` is not on PATH in the extension host process.

Manual config (if needed):

```json
{
  "servers": {
    "ricket": {
      "type": "stdio",
      "command": "ricket",
      "args": ["serve"],
      "env": {
        "RICKET_VAULT_ROOT": "/absolute/path/to/vault"
      }
    }
  }
}
```

If you see `spawn ricket ENOENT`:
- Run `ricket mcp init-vscode ...` so the config uses an absolute command path.
- Or replace `"command": "ricket"` with the full path to your binary (for example, `"C:/Users/alice/go/bin/ricket.exe"` on Windows).
- Verify the binary exists with `ricket --version` in your terminal.

---

## Adding to GitHub Copilot (Visual Studio)

Generate a solution-local MCP config:

```bash
ricket mcp init-visualstudio /path/to/your/solution --vault-root /path/to/vault
```

This writes `.vs/mcp.json` with an absolute `command` path and `RICKET_VAULT_ROOT` environment variable.

The generated server definition matches the same stdio MCP shape used for VS Code (`type`, `command`, `args`, `env`). Feature parity in the chat UI is determined by the Visual Studio Copilot build/channel you are using.

---

## MCP tools reference

| Tool | Description |
|------|-------------|
| `vault_list_inbox` | List all notes in Inbox — path, name, 200-char preview |
| `vault_triage_inbox` | Analyze Inbox notes and propose filing actions (category, destination, confidence, signals); does not mutate files |
| `vault_read_note` | Read a note by path — frontmatter, content, tags, wikilinks |
| `vault_search` | Search by folder, tags (AND), and/or full-text query |
| `vault_get_categories` | Return all configured categories with signals |
| `vault_get_templates` | Return all templates with their section headings |
| `vault_file_note` | Move a note from Inbox to destination, apply template/tags/links/MOC |
| `vault_create_note` | Create a new note with optional tags, links, and MOC update |
| `vault_update_note` | Update an existing note's content, tags, and/or links in-place |
| `vault_status` | Inbox count, total notes, category count |

### `vault_search` parameters

```json
{
  "folder": "Areas/Engineering/decisions/",
  "tags": ["decision", "acme"],
  "query": "SQL Server"
}
```

All filters are **AND**-combined. Uses SQLite for tag/content queries; filesystem walk otherwise.

### `vault_update_note` parameters

```json
{
  "path": "Areas/Engineering/decisions/use-sqlite-for-index.md",
  "content": "# Revised decision\n\nUpdated rationale...",
  "tags": ["reviewed", "stable"],
  "links": ["observability-strategy"]
}
```

At least one of `content`, `tags`, or `links` must be provided. Tags are additive (merged with existing tags, deduplicated). Returns `{ path, gitCommitted }`.

### `vault_file_note` parameters

```json
{
  "source": "Inbox/meeting-notes.md",
  "destination": "Areas/Engineering/meetings/2026-03-17-sprint-planning.md",
  "template": "meeting",
  "tags": ["meeting", "acme"],
  "links": ["sprint-backlog", "auth-regression"],
  "moc": "Areas/Engineering/meetings/MOC.md"
}
```

Returns `{ destination, gitCommitMessage, gitCommitted }`. If the vault is a git repository, ricket auto-commits the change.

### `vault_triage_inbox` workflow

`vault_triage_inbox` is the triage planning tool. It returns:
- `proposals`: deterministic suggestions for filing Inbox notes
- `unresolved`: notes with low confidence or no category signal match

Each proposal includes `needsApproval: true`, so your MCP client/agent should ask for user approval before executing the suggested moves with `vault_file_note`.

---

## Shell completion

Generate completion scripts:

```bash
ricket completion bash
ricket completion zsh
ricket completion fish
ricket completion powershell
```

`ricket completion` only outputs shell completion scripts for the CLI itself; it does not configure MCP clients.

---

## ricket.yaml reference

```yaml
vault:
  inbox: Inbox/          # where unprocessed notes land
  archive: Archive/      # where old notes go
  templates: _templates/ # Obsidian Templater templates

categories:
  - name: acme-decision
    folder: Areas/Engineering/decisions/
    template: decision          # template to apply when filing
    naming: use-{topic}.md      # suggested filename pattern
    tags: [decision, acme]      # tags to add on filing
    moc: Areas/Engineering/decisions/MOC.md  # MOC to update
    signals:                    # keywords that hint this category
      - decision
      - standard
      - convention

mcp:
  name: ricket      # name shown in MCP client
  version: 0.2.0
  needsApproval: true # set false to skip triage approval prompts
```

### Signal hints

`signals` are keywords that help the AI agent decide which category a note belongs to. When the agent calls `vault_get_categories`, it sees these signals and can match them against the note's content before calling `vault_file_note`.

---

## Git audit trail

If your vault is a git repository (recommended — use the **Obsidian Git** plugin), ricket automatically commits every `vault_file_note` and `vault_create_note` operation:

```
ricket: filed meeting-notes.md → Areas/Engineering/meetings/2026-03-17-sprint-planning.md
ricket: created Areas/Engineering/concepts/opentelemetry.md
```

Filter AI-assisted vault operations:

```bash
git log --oneline --grep="ricket:"
```

---

## Recommended Obsidian plugin setup

| Plugin | Why |
|--------|-----|
| **Obsidian Git** | Auto-syncs vault to remote; ricket commits appear here |
| **Templater** | Templates use `<% tp.file.title %>` and `<% tp.date.now(...) %>` — ricket resolves these |
| **Dataview** | MOC files can use Dataview queries to auto-list notes by tag |
| **Tag Wrangler** | Rename tags across vault when standards change |
| **Omnisearch** | Fast full-text search in the Obsidian UI (complements ricket's SQLite index) |

---

## Development

```bash
make build    # compile to bin/ricket
make test     # go test ./...
make lint     # go vet ./...
make clean    # remove bin/ and .ricket/
```

### Test vault

`testdata/vault/` contains a realistic PARA vault used for integration tests. It has:
- 3 inbox captures (raw, meeting draft, learning note)
- 5 templates (decision, concept, meeting, project, learning)
- Filed notes across decisions, concepts, projects, personal development
- `ricket.yaml` configured for a single work org ("acme") + personal learning

---

## Architecture

```
cmd/ricket/          CLI (init, serve, status, config)
internal/
  config/            ricket.yaml load/generate/write
  vault/             core vault operations
    frontmatter.go   YAML frontmatter parse/serialize
    template.go      Templater placeholder substitution
    moc.go           Map-of-Content append
    index.go         SQLite search index (modernc.org/sqlite)
    vault.go         Vault struct — all operations
  git/               Git audit trail
  mcp/               MCP server (mark3labs/mcp-go)
    server.go        Server init and stdio serve
    tools.go         MCP tool definitions and handlers
testdata/vault/      Realistic test vault fixture
```
