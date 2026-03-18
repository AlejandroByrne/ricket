# Ricket — VS Code Extension

Bridges your Obsidian vault with AI coding agents (GitHub Copilot, Claude, Cursor) by auto-registering the **ricket** MCP server in VS Code.

## What it does

On activation (workspace open) the extension:

1. Locates the bundled `ricket` binary for your platform (`bin/ricket-<os>-<arch>`).
2. Registers it as an MCP server in your VS Code user settings under `mcp.servers.ricket`.
3. Passes `--vault-root` when you set `ricket.vaultRoot` in settings.

No manual MCP config editing required.

## Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `ricket.vaultRoot` | `string` | `""` | Absolute path to your Obsidian vault. Leave empty to let ricket auto-resolve. |

## Multi-source (shared standards)

Ricket supports read-only reference sources — shared team standards, style guides, or any external note directories. Configure them in your vault's `ricket.yaml`:

```yaml
sources:
  - name: standards
    path: /path/to/shared-standards
```

Once configured, your AI agent can:
- **Search** across sources automatically (results include a `source` field)
- **Read** source notes with `@standards/api-naming.md` syntax
- **List** all configured sources and their availability

Source notes are read-only and cannot be modified through ricket.

## Development

```bash
cd vscode-extension
npm install
npm run compile   # or: npm run watch
```

Press **F5** in VS Code to launch an Extension Development Host for testing.

## Packaging

```bash
npm install -g @vscode/vsce
cd vscode-extension
vsce package          # produces ricket-0.3.0.vsix
```

## Binary layout

The extension expects platform-specific Go binaries under `bin/`:

```
bin/
  ricket-linux-x64
  ricket-linux-arm64
  ricket-darwin-x64
  ricket-darwin-arm64
  ricket-win32-x64.exe
  ricket-win32-arm64.exe
```

These are built from the repo root with cross-compilation (see main README for CI details).
