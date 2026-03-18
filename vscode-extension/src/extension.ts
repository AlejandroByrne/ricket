import * as vscode from "vscode";
import * as path from "path";
import * as fs from "fs";

/** Name used for the MCP server registration in VS Code settings. */
const MCP_SERVER_NAME = "ricket";

export function activate(context: vscode.ExtensionContext): void {
  const binaryPath = getBundledBinaryPath(context);
  if (!binaryPath) {
    vscode.window.showWarningMessage(
      "Ricket: bundled binary not found for this platform. Install ricket manually and configure the MCP server."
    );
    return;
  }

  ensureMcpServerRegistered(binaryPath);

  // Re-register when the user changes ricket.vaultRoot.
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("ricket.vaultRoot")) {
        ensureMcpServerRegistered(binaryPath);
      }
    })
  );
}

export function deactivate(): void {
  // Nothing to clean up — VS Code manages the MCP server lifecycle.
}

// ---------------------------------------------------------------------------
// Binary resolution
// ---------------------------------------------------------------------------

/**
 * Returns the absolute path to the bundled ricket binary for the current
 * platform, or undefined if no binary is bundled.
 *
 * Expected layout inside the extension:
 *   bin/ricket-<os>-<arch>[.exe]
 * where <os> is linux | darwin | win32 and <arch> is x64 | arm64.
 */
function getBundledBinaryPath(
  context: vscode.ExtensionContext
): string | undefined {
  const platform = process.platform; // linux | darwin | win32
  const arch = process.arch; // x64 | arm64
  const ext = platform === "win32" ? ".exe" : "";
  const name = `ricket-${platform}-${arch}${ext}`;
  const candidate = path.join(context.extensionPath, "bin", name);
  return fs.existsSync(candidate) ? candidate : undefined;
}

// ---------------------------------------------------------------------------
// MCP server auto-registration
// ---------------------------------------------------------------------------

/**
 * Writes (or updates) the ricket MCP server entry in VS Code user settings
 * under `mcp.servers`. This lets GitHub Copilot (and other MCP-aware
 * extensions) discover the running ricket server automatically.
 */
function ensureMcpServerRegistered(binaryPath: string): void {
  const config = vscode.workspace.getConfiguration("mcp");
  const servers: Record<string, unknown> =
    (config.get<Record<string, unknown>>("servers") as Record<string, unknown>) ?? {};

  const args = buildArgs();

  const desired: Record<string, unknown> = {
    type: "stdio",
    command: binaryPath,
    args,
  };

  // Only write if something actually changed.
  const existing = servers[MCP_SERVER_NAME] as Record<string, unknown> | undefined;
  if (
    existing &&
    existing["command"] === desired.command &&
    JSON.stringify(existing["args"]) === JSON.stringify(desired.args)
  ) {
    return;
  }

  servers[MCP_SERVER_NAME] = desired;
  config.update("servers", servers, vscode.ConfigurationTarget.Global);
}

/**
 * Builds the CLI args for the ricket binary, injecting --vault-root when the
 * user has configured `ricket.vaultRoot`.
 */
function buildArgs(): string[] {
  const vaultRoot = vscode.workspace
    .getConfiguration("ricket")
    .get<string>("vaultRoot", "");
  if (vaultRoot) {
    return ["--vault-root", vaultRoot];
  }
  return [];
}
