# Getting Started with Ricket

This guide walks you through setting up Ricket from scratch: installing the binary, creating your vault, integrating it with GitHub Copilot in VS Code, and running your first triage session.

## Prerequisites

- Go 1.22+ (for building and installing from source)
- Obsidian (with default settings; community plugins optional)
- GitHub Copilot extension in VS Code
- Git CLI

## Step 1: Install or Upgrade Ricket

### Uninstall previous version (if upgrading)

```bash
# Remove the old binary from your Go bin directory
rm $(go env GOPATH)/bin/ricket
# or on Windows:
del %USERPROFILE%\go\bin\ricket.exe
```

### Install the latest version

```bash
go install github.com/AlejandroByrne/ricket/cmd/ricket@latest
```

Verify installation:

```bash
ricket --version
# Output: ricket version 0.2.0
```

If the `ricket` command is not found, ensure `$(go env GOPATH)/bin` is on your `PATH`:

**On macOS/Linux:**
```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

**On Windows:**
Add `%USERPROFILE%\go\bin` to your `PATH` environment variable via System Properties → Environment Variables.

---

## Step 2: Create Your Vault Directory

Create a dedicated folder for your Obsidian vault:

```bash
mkdir ~/my-vault
cd ~/my-vault
```

(Replace `~/my-vault` with your preferred location.)

---

## Step 3: Initialize a Git Repository

Ricket commits every filing action to git, so initialize a repo:

```bash
git init
git config user.email "your-email@example.com"
git config user.name "Your Name"
```

---

## Step 4: Run the Ricket Setup Wizard

From inside your vault directory, run:

```bash
ricket init .
```

The wizard will ask:
1. **Vault directory** → accept the default (`.` for current directory)
2. **Folder names** → accept defaults (`Inbox`, `Archive`, `_templates`)
3. **Number of organisations** → `1` for simplicity
4. **Organisation name** → e.g., `Personal` or `Acme`
5. **Tag prefix** → e.g., `personal` (no spaces)
6. **Is this an employer/client?** → `n` for a personal vault (or `y` for work)
7. **Note categories** → choose at least `decisions`, `concepts`, `meetings`
8. **Inbox signals** → accept defaults
9. **Set as default vault?** → `y` to save this vault path for future `ricket` commands
10. **Create .vscode/mcp.json for GitHub Copilot?** → `y`

After completion, you'll see:

```
✓ ricket.yaml written to /path/to/my-vault/ricket.yaml
✓ Vault folders/templates scaffolded
```

---

## Step 5: Verify Vault Scaffolding

Check that all required folders and templates were created:

```bash
ricket config validate --vault-root .
```

Expected output:

```
Vault: /path/to/my-vault

  OK    inbox directory exists: Inbox/
  OK    archive directory exists: Archive/
  OK    templates directory exists: _templates/
  OK    category "personal-decision" → Areas/Personal/decisions/
  OK    category "personal-concept" → Areas/Personal/concepts/
  ...
Vault configuration looks good.
```

If any folders are missing, run:

```bash
ricket init . --vault-root .
```

(The wizard will skip existing `ricket.yaml` but you can re-run the setup to add missing pieces.)

---

## Step 6: Verify Status

List your vault's current state:

```bash
ricket status --vault-root .
```

Expected output:

```
Vault:       /path/to/my-vault
Total notes: 0
Inbox:       0 notes
Categories:  5
```

---

## Step 7: Add Ricket to GitHub Copilot in VS Code

Open your vault in VS Code:

```bash
code .
```

The `.vscode/mcp.json` file was created automatically during `ricket init`. Verify its contents:

```bash
cat .vscode/mcp.json
```

It should look like:

```json
{
  "servers": {
    "ricket": {
      "type": "stdio",
      "command": "/absolute/path/to/ricket",
      "args": ["serve"],
      "env": {
        "RICKET_VAULT_ROOT": "/path/to/my-vault"
      }
    }
  }
}
```

If the paths are wrong (e.g., on Windows), regenerate it:

```bash
ricket mcp init-vscode . --vault-root .
```

---

## Step 8: Restart GitHub Copilot

In VS Code:
1. Open the Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`)
2. Run: `Developer: Reload Window`
3. Wait for VS Code to reload and GitHub Copilot to reconnect

You should see a "ricket" MCP server listed in the Copilot panel once it reconnects (look for the MCP servers indicator in the chat).

---

## Step 9: Add a Test Note to Your Inbox

Using VS Code or Obsidian, create a new note in `Inbox/`:

**File:** `Inbox/test-capture.md`

```markdown
---
tags: []
---

# Test Capture

I want to decide whether to use SQLite or PostgreSQL for my project database. We need:
- Fast query performance
- ACID compliance
- Simple deployment for small team

Also mentioned was caching strategy with Redis.
```

Save the file.

---

## Step 10: Run Triage Analysis in GitHub Copilot

In VS Code, open the GitHub Copilot chat and send:

```
Run vault_triage_inbox and show me the proposals
```

Copilot will call the `vault_triage_inbox` MCP tool, which analyzes your inbox and returns:

```json
{
  "generatedAt": "2026-03-17T...",
  "proposals": [
    {
      "source": "Inbox/test-capture.md",
      "category": "personal-decision",
      "destination": "Areas/Personal/decisions/use-sqlite-or-postgres.md",
      "template": "decision",
      "tags": ["decision", "personal"],
      "confidence": 0.85,
      "matchedSignals": ["decide", "decision"],
      "needsApproval": true
    }
  ],
  "unresolved": []
}
```

---

## Step 11: File the Note (Approval Workflow)

Ask Copilot to approve and file it:

```
The proposal for test-capture.md looks good. Please file it using vault_file_note 
with the exact parameters from the proposal.
```

Copilot will:
1. Move `Inbox/test-capture.md` → `Areas/Personal/decisions/use-sqlite-or-postgres.md`
2. Apply the decision template
3. Add tags `decision` and `personal`
4. Commit to git

Verify the filing succeeded:

```bash
ricket status --vault-root .
```

You should now see:

```
Inbox:       0 notes
Total notes: 1
```

And the note file exists:

```bash
cat Areas/Personal/decisions/use-sqlite-or-postgres.md
```

---

## Step 12: Continuous Workflow

Now that ricket is set up, the typical workflow is:

1. **Capture** → Add raw notes, voice transcripts, meeting dumps to `Inbox/`
2. **Analyze** → Ask Copilot: "Run `vault_triage_inbox` and show proposals"
3. **Approve** → Review each proposal's category, destination, and confidence
4. **File** → Ask Copilot to call `vault_file_note` for each approved note
5. **Link** → Copilot can call `vault_update_note` and `vault_read_note` to cross-link notes and update MOC files

---

## Troubleshooting

### `spawn ricket ENOENT` error in VS Code

Make sure `.vscode/mcp.json` has an **absolute** path to the ricket binary:

```bash
ricket mcp init-vscode . --vault-root .
```

Then reload VS Code.

### Templates are missing

Run the wizard again:

```bash
rm ricket.yaml
ricket init .
```

### Vault status shows "configuration looks good" but init didn't create folders

Make sure you're in the correct directory and run:

```bash
ricket init . --vault-root $(pwd)
```

### GitHub Copilot doesn't show ricket tools

1. Verify `.vscode/mcp.json` is valid JSON (use an online validator)
2. Check that the `ricket serve` command works in a terminal:
   ```bash
   ricket serve --vault-root .
   ```
   It should output `ricket MCP server running...` and wait for stdin.
3. Reload VS Code and check the Copilot logs (Developer Console)

---

## Next Steps

- Read [README.md](README.md) for detailed MCP tool reference
- Explore the vault: open it in Obsidian, install **Obsidian Git**, **Templater**, **Dataview** plugins
- Customize `ricket.yaml` with more categories and organisations
- Use `ricket completion bash/zsh/fish/powershell` to enable shell autocomplete

Happy triaging!
