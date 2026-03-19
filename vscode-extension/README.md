# Ricket — VS Code Extension

**Give your AI coding agent a memory.** This extension connects your markdown notes to GitHub Copilot (and other MCP-compatible agents) so every chat session starts with your team's decisions, standards, and project history already loaded.

No Obsidian required — any folder of markdown files works.

## How it works

On activation, the extension:

1. Detects your platform and locates the bundled `ricket` binary.
2. Prompts you to select your vault folder using a native file picker (on first install).
3. Registers it as an MCP server in your VS Code settings (`mcp.servers.ricket`).
4. Your agent can immediately call ricket tools to search, read, and organize your notes.

No manual MCP config editing or path formatting needed. Install, pick a folder, reload, and start chatting.

## Getting started

1. **Install** this extension from the marketplace.
2. **Select your vault folder** — a popup appears asking you to pick the folder. Click "Select Folder" and use the file picker. You can also re-pick later: `Ctrl+Shift+P` → "Ricket: Select Vault Folder".
3. **Reload the window** — `Ctrl+Shift+P` → "Developer: Reload Window" so Copilot picks up the MCP server.
4. **Send the first prompt** to your agent:

   Starting fresh:
   ```
   Run vault_analyze and help me set up a new ricket vault from scratch.
   ```

   Existing notes:
   ```
   Run vault_analyze and walk me through migrating my existing vault to ricket.
   ```

Your agent will inspect the folder, generate a `ricket.yaml` config, scaffold folders and templates, and write a `VAULT_GUIDE.md`. Reload the window once more and the full tool set is live.

## Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `ricket.vaultRoot` | `string` | `""` | Absolute path to your vault folder. Leave empty to auto-resolve (env var → config file → cwd). |

## Commands

| Command | Description |
|---|---|
| `Ricket: Select Vault Folder` | Open a folder picker to set or change your vault root. |

## Shared standards for teams

Point ricket at shared folders so every teammate's agent reads from the same knowledge base. Add a `sources:` section to your `ricket.yaml`:

```yaml
sources:
  - name: standards
    path: ../shared-standards       # a git repo your team maintains
  - name: playbook
    path: /shared/engineering-playbook
```

Once configured, your agent automatically:
- **Searches** across shared sources alongside your notes (results tagged with source name)
- **Reads** source notes with `@standards/api-naming.md` syntax
- **Lists** all configured sources and their availability

Source notes are read-only — no one accidentally edits the shared standards through their agent. This means the whole team's agents are informed with the same decisions, conventions, and best practices.

## What ricket exposes to your agent

| Tool | What it does |
|------|-------------|
| `vault_analyze` | Deep inspection of folder structure, tags, naming patterns, PKM detection |
| `vault_write_config` | Generate `ricket.yaml` and `VAULT_GUIDE.md`, scaffold folders |
| `vault_search` | Full-text + tag + folder search across notes and shared sources |
| `vault_read_note` | Read any note's content, frontmatter, tags, links |
| `vault_triage_inbox` | Propose where to file inbox notes |
| `vault_file_note` | Move notes from inbox to destination with template + tags + links |
| `vault_create_note` | Create new notes with templates and MOC updates |
| `vault_update_note` | Edit existing notes in-place |
| `vault_status` | Quick summary — inbox count, total notes, categories |
| `vault_list_sources` | Show configured shared sources and availability |

Plus `vault_list_inbox`, `vault_get_categories`, and `vault_get_templates`.

## No Obsidian required

Ricket works with **any folder of markdown files**. If you use Obsidian, ricket detects your vault structure and works alongside it. If you don't, ricket scaffolds a clean structure for you. All you need is a directory.

## More information

- [GitHub repository](https://github.com/AlejandroByrne/ricket) — full documentation, config reference, architecture
- [Getting started guide](https://github.com/AlejandroByrne/ricket/blob/main/GETTING_STARTED.md)
