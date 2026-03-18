<p align="center">
  <img src="vscode-extension/icon.png" alt="Ricket" width="256" />
</p>

# ricket

Vault-powered context engine for AI coding agents.

Ricket bridges your [Obsidian](https://obsidian.md) vault and your AI coding assistant (Claude Code, GitHub Copilot, Cursor). Two jobs:

1. **Triage** — take raw inbox notes, voice transcripts, meeting dumps and file them into the right place with the right template, tags, links, and MOC entry.
2. **Context** — expose your vault's decisions, standards, concepts, and project history as structured context via MCP so your agent always knows your stack.
3. **Analysis** — detect your PKM system (PARA, Zettelkasten, LYT/ACCESS, GTD, Johnny.Decimal, and more), map frontmatter schemas, link topology, and tag taxonomy so your agent understands *how* your vault is organized.

Ricket makes **zero LLM API calls**. It is pure plumbing — the tool the LLM calls, not the other way around.

---

## How it works

Setup and ongoing use are both entirely agent-driven. There is no interactive CLI wizard.

```
1. ricket init --vscode /path/to/vault   ← one CLI command, wires up MCP
2. Open agent chat, send the first prompt ← agent takes it from here
3. Dump notes into Inbox/, ask agent to triage ← the daily workflow
```

**New vault** — agent calls `vault_analyze`, inspects the empty directory, generates a `ricket.yaml` tailored to your intended structure, scaffolds folders and templates, and writes a `VAULT_GUIDE.md` that teaches future sessions how your vault is organized.

**Existing Obsidian vault** — agent calls `vault_analyze`, reads your folder tree, tags, naming patterns, and templates, then proposes a `ricket.yaml` that maps your actual structure into ricket categories. No data is moved; only config is written.

After `ricket.yaml` exists, the server restarts into full mode and all triage tools become available.

---

## Requirements

- Go 1.22+ (for building from source)
- An AI assistant with MCP support: Claude Code, GitHub Copilot (VS Code), or GitHub Copilot (Visual Studio)

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

### 1. Wire up your agent

Run `ricket init` from inside your vault directory (existing or new). Pass a flag for your preferred agent:

```bash
cd /path/to/your/obsidian-vault

ricket init --vscode          # GitHub Copilot in VS Code  → .vscode/mcp.json
ricket init --visualstudio    # GitHub Copilot in Visual Studio → .vs/mcp.json
ricket init --claude-code     # Claude Code → ~/.claude/mcp.json (merged)
ricket init --all             # all three at once
```

ricket detects whether a `.obsidian/` folder is present and prints the right first prompt for your agent.

### 2. Open your agent and send the first prompt

**Existing Obsidian vault:**
```
Run vault_analyze and walk me through migrating my existing vault to ricket.
```

**New vault:**
```
Run vault_analyze and help me set up a new ricket vault from scratch.
```

The agent inspects your vault, proposes a config, and calls `vault_write_config` to write `ricket.yaml` and `VAULT_GUIDE.md`. Once written, restart the MCP server (reload your IDE window) — the full tool set is now available.

### 3. Verify

```bash
ricket status
```

```
Vault:       /Users/alice/obsidian-vault
Total notes: 847
Inbox:       3 notes
Categories:  8
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
```

Validate your vault configuration at any time:

```bash
ricket config validate --vault-root /path/to/vault
```

---

## MCP tools reference

### Migration tools (always available, including before `ricket.yaml` exists)

| Tool | Description |
|------|-------------|
| `vault_analyze` | Inspect vault structure — folder tree, tag frequency, naming patterns, templates, MOC files, inferred categories with confidence scores, PKM system detection, frontmatter schema, link topology, and tag taxonomy. Safe to call on any directory. |
| `vault_write_config` | Write `ricket.yaml` and `VAULT_GUIDE.md` to vault root. Accepts raw YAML config and guide markdown. Pass `scaffold: true` to also create missing folders and template stubs. |

### Triage and filing tools (available after `ricket.yaml` exists)

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

### `vault_write_config` parameters

```json
{
  "config_yaml": "vault:\n  inbox: Inbox/\n...",
  "guide_content": "# VAULT_GUIDE\n...",
  "guide_path": "VAULT_GUIDE.md",
  "overwrite": false,
  "scaffold": true
}
```

`scaffold: true` calls `ricket config scaffold` internally — creates missing folders, template stubs, and MOC files based on the config just written.

### `vault_search` parameters

```json
{
  "folder": "Areas/Engineering/decisions/",
  "tags": ["decision", "acme"],
  "query": "SQL Server"
}
```

All filters are **AND**-combined. Uses SQLite for tag/content queries; filesystem walk otherwise.

### `vault_triage_inbox` workflow

`vault_triage_inbox` is the triage planning tool. It returns:
- `proposals`: deterministic suggestions for filing Inbox notes
- `unresolved`: notes with low confidence or no category signal match

Each proposal includes `needsApproval: true`, so your agent should ask for user approval before executing the suggested moves with `vault_file_note`.

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

---

## Shell completion

```bash
ricket completion bash
ricket completion zsh
ricket completion fish
ricket completion powershell
```

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
  version: 0.3.0
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

Filter ricket vault operations:

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
  config/            ricket.yaml load/write
  vault/             core vault operations
    analyze.go       VaultAnalysis — single-pass walk, folder tree, tags, patterns, inferred categories
    pkm.go           PKM system detection (PARA, LYT, ACE, Zettelkasten, JD, GTD, BASB, Evergreen)
    notedata.go      Shared noteData struct for single-pass parsing
    scaffold.go      ScaffoldVault — create missing folders, templates, MOC files
    frontmatter.go   YAML frontmatter parse/serialize
    template.go      Templater placeholder substitution
    moc.go           Map-of-Content append
    index.go         SQLite search index (modernc.org/sqlite)
    vault.go         Vault struct — all operations
  git/               Git audit trail
  mcp/               MCP server (mark3labs/mcp-go)
    server.go        Server init; migration mode; WithInstructions (VAULT_GUIDE.md)
    tools.go         MCP tool definitions, handlers, ReadOnlyHint annotations, setup-vault prompt
testdata/vault/      Realistic test vault fixture
```
