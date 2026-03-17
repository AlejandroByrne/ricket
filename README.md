# ricket

Vault-powered context engine for AI coding agents.

Ricket bridges your Obsidian vault and your AI coding agent (Claude Code, GitHub Copilot, Cursor). Two jobs:

1. **Triage** — take raw notes, meeting dumps, voice transcripts and interactively classify, scaffold, tag, link, and file them into the right place in your vault
2. **Context** — expose your vault's decisions, standards, people, and project history as structured context that any coding agent can consume via MCP

Ricket is **not** an LLM. It makes zero API calls. It's plumbing — the tool that LLMs call, not the other way around.

## Install

```bash
npm install -g ricket
```

Requires Node.js 20+.

## Quick start

```bash
# Point ricket at your Obsidian vault
cd ~/obsidian-vault
ricket init

# Start the MCP server (for Claude Code / GitHub Copilot)
ricket serve
```

## How it works

Ricket runs as an [MCP server](https://modelcontextprotocol.io/) that AI agents call via tool use. When you tell Claude Code "triage my inbox" or "what did we decide about the ORM?", the agent calls ricket's tools to read, search, classify, and file notes in your vault.

```
You → "triage my inbox"
  └→ Claude Code → ricket MCP: vault_list_inbox()
  └→ Claude Code → ricket MCP: vault_read_note(path)
  └→ Claude Code (thinks): "this is a decision note"
  └→ Claude Code → you: "I propose filing as decisions/use-dapper-not-efcore.md"
  └→ You: "yes"
  └→ Claude Code → ricket MCP: vault_file_note(src, dest, template, tags)
  └→ Ricket: moves file, scaffolds template, adds tags, updates MOC, git commits
```

## MCP tools

| Tool | Description |
|---|---|
| `vault_list_inbox` | List all notes in the inbox folder |
| `vault_read_note` | Read a note with parsed frontmatter, tags, and wikilinks |
| `vault_search` | Search by folder, tags, or text content |
| `vault_get_categories` | Get the full vault category configuration |
| `vault_get_templates` | List templates with their section fields |
| `vault_file_note` | File a note: move, scaffold template, add tags/links, update MOC |
| `vault_create_note` | Create a new note from a template |
| `vault_status` | Vault stats: inbox count, total notes, categories |

## Configuration

Ricket uses a `ricket.yaml` file at your vault root. Run `ricket init` to generate one, or create it manually:

```yaml
vault:
  inbox: Inbox/
  archive: Archive/
  templates: _templates/

categories:
  - name: decision
    folder: Areas/Engineering/decisions/
    template: decision
    naming: "use-{topic}.md"
    tags: [decision]
    moc: Areas/Engineering/Engineering.md
    signals: ["we decided", "standard is", "always use"]

  - name: concept
    folder: Areas/Engineering/concepts/
    template: concept
    tags: [concept]

  # Add your own categories...
```

Each category defines where notes of that type live, what template to use, what tags to apply, and what signals (keywords) hint that a note belongs there. The LLM uses this configuration to make classification decisions.

## Connecting to Claude Code

Add to your Claude Desktop MCP config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ricket": {
      "command": "ricket",
      "args": ["serve", "--vault-root", "/path/to/your/vault"]
    }
  }
}
```

Or for Claude Code, add to `.mcp.json` in your project:

```json
{
  "mcpServers": {
    "ricket": {
      "command": "ricket",
      "args": ["serve", "--vault-root", "/path/to/your/vault"]
    }
  }
}
```

## Connecting to GitHub Copilot (VS Code)

Add to `.vscode/mcp.json` in your workspace:

```json
{
  "servers": {
    "ricket": {
      "command": "ricket",
      "args": ["serve", "--vault-root", "C:\\Users\\you\\source\\obsidian-vault"]
    }
  }
}
```

Then in Copilot Chat, select **Agent** mode and make sure the ricket tools are enabled in the tools config dial.

## Vault structure

Ricket ships with [PARA](https://fortelabs.com/blog/para/) as the default organizational system:

- **Projects/** — active work with an end date
- **Areas/** — ongoing responsibilities (engineering standards, team context)
- **Resources/** — reference material (career notes, docs, specs)
- **Archive/** — completed or inactive

Categories, folder paths, templates, and tags are all configurable in `ricket.yaml`.

## Git audit trail

Every file operation ricket performs is automatically committed with a structured message:

```
ricket: filed meeting-notes.md → Areas/Engineering/meetings/2026-03-16-sprint.md
```

This gives you a clean audit trail (`git log --grep="ricket:"`) and easy undo (`git revert`).

## CLI

```
ricket init [path]     Initialize ricket in a vault (generates ricket.yaml)
ricket serve           Start MCP server on stdio
ricket status          Show vault stats and inbox contents
ricket --help          Show all commands
```

## Who this is for

Software engineers who:
- Maintain an Obsidian vault for engineering decisions, team context, and career notes
- Use AI coding agents (Claude Code, GitHub Copilot, Cursor) daily
- Want a two-way loop: vault feeds the agent context, agent feeds the vault knowledge

## License

MIT
