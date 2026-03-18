# Getting Started with Ricket

This guide walks you through installing ricket, wiring it up to your AI agent, and running your first triage session. The entire setup — new vault or existing Obsidian vault — is handled by your agent in chat. There is no CLI wizard.

## Prerequisites

- Go 1.22+ (for installing from source)
- An AI assistant with MCP support: Claude Code, GitHub Copilot (VS Code), or GitHub Copilot (Visual Studio)
- Git CLI (recommended — ricket auto-commits every filing action)

---

## Step 1: Install Ricket

```bash
go install github.com/AlejandroByrne/ricket/cmd/ricket@latest
```

Verify installation:

```bash
ricket --version
# Output: ricket version 0.3.0
```

If the `ricket` command is not found, ensure `$(go env GOPATH)/bin` is on your `PATH`:

**macOS / Linux:**
```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

**Windows:**
Add `%USERPROFILE%\go\bin` to your `PATH` via System Properties → Environment Variables.

---

## Step 2: Point ricket at your vault

Navigate to your vault directory. This can be an existing Obsidian vault or an empty folder for a new one.

```bash
cd /path/to/your/vault
```

If starting fresh, initialize a git repository (recommended):

```bash
git init
git config user.email "you@example.com"
git config user.name "Your Name"
```

---

## Step 3: Wire up your agent

Run `ricket init` with a flag for your preferred agent:

```bash
ricket init --vscode          # GitHub Copilot in VS Code  → .vscode/mcp.json
ricket init --visualstudio    # GitHub Copilot in Visual Studio → .vs/mcp.json
ricket init --claude-code     # Claude Code → ~/.claude/mcp.json (merged)
ricket init --all             # all three at once
```

ricket detects whether a `.obsidian/` folder exists and prints the correct first prompt for your situation.

Example output (existing vault):

```
Existing Obsidian vault detected at /path/to/your/vault

✓ .vscode/mcp.json written

Next steps:
  1. Open VS Code in this directory:  code .
  2. Send the agent this prompt:

     Run vault_analyze and walk me through migrating my existing vault to ricket.
```

Example output (new vault):

```
✓ .vscode/mcp.json written

Next steps:
  1. Open VS Code in this directory:  code .
  2. Send the agent this prompt:

     Run vault_analyze and help me set up a new ricket vault from scratch.
```

---

## Step 4: Open your agent and send the first prompt

### GitHub Copilot (VS Code)

Open VS Code in your vault directory:

```bash
code .
```

Reload the window so Copilot picks up the new MCP config (`Ctrl+Shift+P` → `Developer: Reload Window`). Open the Copilot chat and send the prompt ricket printed in Step 3.

### Claude Code

Open Claude Code from inside your vault directory. The `~/.claude/mcp.json` entry ricket added will be picked up automatically. Send the prompt ricket printed in Step 3.

---

## Step 5: Agent-driven setup

The agent calls `vault_analyze`, which performs a single-pass scan of your vault and returns:
- **Folder tree** with note counts and sample filenames
- **Tag frequency** and **tag taxonomy** (nested prefixes, @context tags)
- **Naming patterns** with type classification (zettelkasten-uid, johnny-decimal, date-topic, etc.)
- **Link analysis** — wikilink density, hub notes, MOC-like notes, orphan count
- **Frontmatter schema** — key frequency and notable key combinations (LYT, Zettelkasten, BASB signals)
- **PKM system detection** — identifies your methodology (PARA, LYT/ACCESS, ACE, Zettelkasten, Johnny.Decimal, GTD, BASB, Evergreen) with confidence scores and evidence, including hybrid detection
- **Inferred categories** with confidence boosted by PKM system alignment

**For an existing Obsidian vault**, the agent proposes a `ricket.yaml` that maps your actual folders, tags, and templates into ricket categories — no data is moved or renamed. It also drafts a `VAULT_GUIDE.md` that teaches future agent sessions how your vault is organized.

**For a new vault**, the agent walks you through choosing a structure (PARA or custom), picks folder names, category definitions, and templates suited to your use case.

When you confirm, the agent calls `vault_write_config` to write both files and scaffold any missing folders.

---

## Step 6: Reload and verify

After `vault_write_config` completes, reload your IDE window so the MCP server restarts. It will now start in full mode with all triage tools available.

Verify:

```bash
ricket status
```

```
Vault:       /path/to/your/vault
Total notes: 847
Inbox:       3 notes
Categories:  8
```

Validate that all folders and templates are in place:

```bash
ricket config validate
```

---

## Step 7: Your first triage session

Add a raw note to your `Inbox/` folder. It can be anything: voice-to-text, meeting notes, a quick idea, a copied snippet.

**Example:** `Inbox/raw-capture.md`

```markdown
---
tags: []
---

Should we switch the API layer from REST to GraphQL?
Main reasons: frontend keeps requesting nested data, lots of over-fetching.
Counter-argument: team is more familiar with REST, migration cost is high.
```

Then ask your agent:

```
Run vault_triage_inbox and show me the proposals.
```

The agent calls `vault_triage_inbox`, which returns a proposal for each inbox note:

```json
{
  "proposals": [
    {
      "source": "Inbox/raw-capture.md",
      "category": "engineering-decision",
      "destination": "Areas/Engineering/decisions/use-graphql-or-rest.md",
      "template": "decision",
      "tags": ["decision", "engineering"],
      "confidence": 0.85,
      "matchedSignals": ["decide", "decision"],
      "needsApproval": true
    }
  ],
  "unresolved": []
}
```

Review the proposal, then approve it:

```
The proposal looks good. File it.
```

The agent calls `vault_file_note`, which moves the note, applies the template, adds tags, updates the MOC file, and commits to git.

---

## Step 8: Ongoing workflow

```
1. Capture  → Drop raw notes into Inbox/ (voice transcripts, meeting dumps, quick ideas)
2. Triage   → "Run vault_triage_inbox and show me the proposals"
3. Approve  → Review each proposal's category, destination, and confidence
4. File     → "File them" (agent calls vault_file_note for each approved note)
5. Link     → "Update the MOC and cross-link related notes" (vault_update_note)
```

That's the full loop. You never need to decide where a note goes, what to tag it, or what to link it to — that's the agent's job.

---

## Troubleshooting

### `spawn ricket ENOENT` in VS Code

The extension host may not inherit your shell PATH. Regenerate the config so the command uses an absolute path:

```bash
ricket mcp init-vscode --vault-root /path/to/vault
```

Then reload VS Code.

### Agent doesn't see ricket tools

1. Confirm the MCP config file was written:
   - VS Code: `.vscode/mcp.json` in your vault directory
   - Claude Code: `~/.claude/mcp.json`
2. Test the server directly: `ricket serve --vault-root .` should block waiting for stdin, not error.
3. Reload your IDE window.

### Templates are missing after setup

Ask your agent:

```
Run vault_write_config with scaffold: true to create any missing folders and templates.
```

Or from the CLI:

```bash
ricket config scaffold
```

### Server starts in "migration mode" (only vault_analyze available)

This means `ricket.yaml` does not exist yet. Send the setup prompt to your agent — it will call `vault_analyze` and `vault_write_config` to generate the config, then the server needs a reload.

---

## Next steps

- Read [README.md](README.md) for the full MCP tool reference and `ricket.yaml` schema
- Open your vault in Obsidian and install: **Obsidian Git**, **Templater**, **Dataview**
- Use `ricket completion bash/zsh/fish/powershell` to enable shell autocomplete
