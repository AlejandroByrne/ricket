package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/AlejandroByrne/ricket/internal/config"
	ricketmcp "github.com/AlejandroByrne/ricket/internal/mcp"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

var vaultRoot string

const plainRicketBanner = `
██████╗ ██╗ ██████╗██╗  ██╗███████╗████████╗
██╔══██╗██║██╔════╝██║ ██╔╝██╔════╝╚══██╔══╝
██████╔╝██║██║     █████╔╝ █████╗     ██║
██╔══██╗██║██║     ██╔═██╗ ██╔══╝     ██║
██║  ██║██║╚██████╗██║  ██╗███████╗   ██║
╚═╝  ╚═╝╚═╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝
`

func main() {
	root := &cobra.Command{
		Use:     "ricket",
		Short:   "Vault-powered context engine for AI coding agents",
		Version: "0.3.0",
		RunE: func(cmd *cobra.Command, args []string) error {
			printBanner(os.Stdout)
			return cmd.Usage()
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printBanner(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout())
		_ = cmd.Root().Usage()
	})
	root.PersistentFlags().StringVarP(&vaultRoot, "vault-root", "r", "",
		"Vault root directory (overrides RICKET_VAULT_ROOT env var and ~/.config/ricket/config.yaml)")

	root.AddCommand(initCmd(), serveCmd(), statusCmd(), configCmd(), mcpCmd(), completionCmd(root))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func printBanner(out io.Writer) {
	if out == nil || !shouldShowBanner() {
		return
	}
	if supportsANSI() {
		fmt.Fprint(out, coloredRicketBanner())
		fmt.Fprintln(out)
		fmt.Fprintln(out, coloredRicketTagline())
		return
	}
	fmt.Fprint(out, plainRicketBanner)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Inbox -> Triage -> Filed -> Committed")
}

func shouldShowBanner() bool {
	if os.Getenv("RICKET_NO_ART") != "" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func supportsANSI() bool {
	term := strings.TrimSpace(strings.ToLower(os.Getenv("TERM")))
	if term == "dumb" {
		return false
	}
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	return os.Getenv("ANSICON") != "" || os.Getenv("ConEmuANSI") == "ON"
}

func coloredRicketBanner() string {
	reset := "\x1b[0m"
	shadow := "\x1b[38;2;24;24;23m"
	deep := "\x1b[38;2;56;83;47m"
	main := "\x1b[38;2;77;114;63m"
	highlight := "\x1b[38;2;103;135;85m"

	lines := []string{
		shadow + "██████╗ ██╗ ██████╗██╗  ██╗███████╗████████╗" + reset,
		deep + "██╔══██╗██║██╔════╝██║ ██╔╝██╔════╝╚══██╔══╝" + reset,
		main + "██████╔╝██║██║     █████╔╝ █████╗     ██║   " + reset,
		main + "██╔══██╗██║██║     ██╔═██╗ ██╔══╝     ██║   " + reset,
		highlight + "██║  ██║██║╚██████╗██║  ██╗███████╗   ██║   " + reset,
		highlight + "╚═╝  ╚═╝╚═╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   " + reset,
	}
	return strings.Join(lines, "\n")
}

func coloredRicketTagline() string {
	reset := "\x1b[0m"
	accent := "\x1b[38;2;201;183;104m"
	muted := "\x1b[38;2;118;125;133m"
	return muted + "Inbox" + reset + accent + " -> " + reset + muted + "Triage" + reset + accent + " -> " + reset + muted + "Filed" + reset + accent + " -> " + reset + muted + "Committed" + reset
}

// resolveRoot returns the vault root using this precedence:
//  1. --vault-root CLI flag
//  2. RICKET_VAULT_ROOT environment variable
//  3. default_vault in ~/.config/ricket/config.yaml
//  4. Current working directory
func resolveRoot() (string, error) {
	if vaultRoot != "" {
		clean := filepath.Clean(filepath.FromSlash(vaultRoot))
		if filepath.IsAbs(clean) {
			return clean, nil
		}
		return filepath.Abs(clean)
	}
	if env := os.Getenv("RICKET_VAULT_ROOT"); env != "" {
		clean := filepath.Clean(filepath.FromSlash(env))
		if filepath.IsAbs(clean) {
			return clean, nil
		}
		return filepath.Abs(clean)
	}
	if home, err := os.UserHomeDir(); err == nil {
		cfgPath := filepath.Join(home, ".config", "ricket", "config.yaml")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var uc struct {
				DefaultVault string `yaml:"default_vault"`
			}
			if err := yaml.Unmarshal(data, &uc); err == nil && uc.DefaultVault != "" {
				clean := filepath.Clean(filepath.FromSlash(uc.DefaultVault))
				if filepath.IsAbs(clean) {
					return clean, nil
				}
				return filepath.Abs(clean)
			}
		}
	}
	return os.Getwd()
}

// writeUserConfig writes ~/.config/ricket/config.yaml with the given vault path.
func writeUserConfig(vaultPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "ricket")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	content := fmt.Sprintf("# ricket user configuration\ndefault_vault: %s\n", vaultPath)
	return os.WriteFile(cfgPath, []byte(content), 0o644)
}

// ── init ──────────────────────────────────────────────────────────────────────

// initCmd bootstraps ricket by registering it with the user's AI agent(s).
// It does NOT run an interactive config wizard — that flow is handled by the
// agent via vault_analyze + vault_write_config after the MCP server is running.
func initCmd() *cobra.Command {
	var flagVSCode bool
	var flagVisualStudio bool
	var flagClaudeCode bool
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "init [vault-path]",
		Short: "Bootstrap ricket — register the MCP server with your AI agent(s)",
		Long: `Register ricket with your AI agent(s) and print the first prompt to kick off
vault setup or migration.

Vault detection:
  - If a .obsidian/ folder or existing notes are found at vault-path, ricket
    starts in migration mode: the agent will call vault_analyze and walk you
    through generating ricket.yaml for your existing vault.
  - If the vault appears to be new/empty, the agent will scaffold a fresh
    PARA-based vault for you.

MCP config is written to:
  --vscode         .vscode/mcp.json  (GitHub Copilot in VS Code)
  --visualstudio   .vs/mcp.json      (GitHub Copilot in Visual Studio)
  --claude-code    ~/.claude/mcp.json (Claude Code — global)
  --all            all of the above

If no flag is given, you are prompted to choose interactively.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve vault path
			var vaultPath string
			var err error
			if vaultRoot != "" {
				vaultPath, err = filepath.Abs(vaultRoot)
			} else if len(args) == 1 {
				vaultPath, err = filepath.Abs(args[0])
			} else {
				vaultPath, err = os.Getwd()
			}
			if err != nil {
				return fmt.Errorf("invalid vault path: %w", err)
			}

			// Detect whether this is a new or existing vault
			isExisting := isExistingVault(vaultPath)

			printBanner(os.Stderr)
			fmt.Fprintln(os.Stderr)

			if isExisting {
				fmt.Fprintln(os.Stderr, "Existing Obsidian vault detected.")
			} else {
				fmt.Fprintln(os.Stderr, "New vault directory.")
			}

			// Determine which agents to configure
			if !flagVSCode && !flagVisualStudio && !flagClaudeCode && !flagAll {
				flagVSCode, flagVisualStudio, flagClaudeCode = promptAgentChoice(os.Stdin, os.Stderr)
			}
			if flagAll {
				flagVSCode = true
				flagVisualStudio = true
				flagClaudeCode = true
			}

			wrote := false

			if flagVSCode {
				if err := writeVSCodeMCPConfig(vaultPath, vaultPath); err != nil {
					fmt.Fprintf(os.Stderr, "  WARN  .vscode/mcp.json: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "  ✓  wrote %s\n", filepath.Join(vaultPath, ".vscode", "mcp.json"))
					wrote = true
				}
			}
			if flagVisualStudio {
				if err := writeVisualStudioMCPConfig(vaultPath, vaultPath); err != nil {
					fmt.Fprintf(os.Stderr, "  WARN  .vs/mcp.json: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "  ✓  wrote %s\n", filepath.Join(vaultPath, ".vs", "mcp.json"))
					wrote = true
				}
			}
			if flagClaudeCode {
				if err := writeClaudeCodeMCPConfig(vaultPath); err != nil {
					fmt.Fprintf(os.Stderr, "  WARN  ~/.claude/mcp.json: %v\n", err)
				} else {
					home, _ := os.UserHomeDir()
					fmt.Fprintf(os.Stderr, "  ✓  wrote %s\n", filepath.Join(home, ".claude", "mcp.json"))
					wrote = true
				}
			}

			if !wrote {
				fmt.Fprintln(os.Stderr, "\nNo MCP config written. Run with --vscode, --claude-code, or --all.")
				return nil
			}

			// Save vault as default
			if err := writeUserConfig(vaultPath); err != nil {
				fmt.Fprintf(os.Stderr, "  WARN  could not update ~/.config/ricket/config.yaml: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  ✓  default vault set to %s\n", vaultPath)
			}

			printFirstPrompt(os.Stderr, isExisting)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagVSCode, "vscode", false, "Write .vscode/mcp.json for GitHub Copilot in VS Code")
	cmd.Flags().BoolVar(&flagVisualStudio, "visualstudio", false, "Write .vs/mcp.json for GitHub Copilot in Visual Studio")
	cmd.Flags().BoolVar(&flagClaudeCode, "claude-code", false, "Write to ~/.claude/mcp.json for Claude Code (global)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Write all supported MCP configs")
	return cmd
}

// isExistingVault returns true when vaultPath contains a .obsidian folder or
// has at least 3 markdown files (indicating a vault already in use).
func isExistingVault(vaultPath string) bool {
	if _, err := os.Stat(filepath.Join(vaultPath, ".obsidian")); err == nil {
		return true
	}
	count := 0
	_ = filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			count++
			if count >= 3 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return count >= 3
}

// printFirstPrompt prints the agent prompt the user should paste to kick off setup.
func printFirstPrompt(out io.Writer, isExisting bool) {
	fmt.Fprintln(out, "\n────────────────────────────────────────────────────")
	fmt.Fprintln(out, "Next steps:")
	if isExisting {
		fmt.Fprintln(out, "  1. Open your agent inside the vault folder")
		fmt.Fprintln(out, "     (VS Code: code /path/to/vault  |  Claude Code: cd /path/to/vault && claude)")
		fmt.Fprintln(out, "  2. Send the agent this prompt:")
	fmt.Fprintln(out)
		fmt.Fprintln(out, `     "Run vault_analyze and walk me through migrating my existing vault to ricket."`)
	} else {
		fmt.Fprintln(out, "  1. Open your agent inside the vault folder")
		fmt.Fprintln(out, "     (VS Code: code /path/to/vault  |  Claude Code: cd /path/to/vault && claude)")
		fmt.Fprintln(out, "  2. Send the agent this prompt:")
	fmt.Fprintln(out)
		fmt.Fprintln(out, `     "Run vault_analyze and help me set up a new ricket vault from scratch."`)
	}
	fmt.Fprintln(out, "\n  The agent will guide you through the rest.")
	fmt.Fprintln(out, "────────────────────────────────────────────────────")
}

// promptAgentChoice asks the user which agent(s) to configure.
func promptAgentChoice(in io.Reader, out io.Writer) (vscode, visualstudio, claudeCode bool) {
	r := bufio.NewReader(in)
	fmt.Fprintln(out, "\nWhich AI agent(s) should ricket be added to?")
	fmt.Fprintln(out, "  [1] GitHub Copilot in VS Code (.vscode/mcp.json)")
	fmt.Fprintln(out, "  [2] GitHub Copilot in Visual Studio (.vs/mcp.json)")
	fmt.Fprintln(out, "  [3] Claude Code (~/.claude/mcp.json)")
	fmt.Fprintln(out, "  [4] All of the above")
	fmt.Fprint(out, "  Choice [1]: ")
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		line = "1"
	}
	switch line {
	case "2":
		return false, true, false
	case "3":
		return false, false, true
	case "4":
		return true, true, true
	default:
		return true, false, false
	}
}

// ── MCP config writers ────────────────────────────────────────────────────────

func writeVSCodeMCPConfig(workspacePath, vaultPath string) error {
	command := resolveRicketCommand()

	type vscodeServer struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	type vscodeMCPConfig struct {
		Servers map[string]vscodeServer `json:"servers"`
	}

	cfg := vscodeMCPConfig{
		Servers: map[string]vscodeServer{
			"ricket": {
				Type:    "stdio",
				Command: command,
				Args:    []string{"serve"},
				Env: map[string]string{
					"RICKET_VAULT_ROOT": vaultPath,
				},
			},
		},
	}

	if err := os.MkdirAll(filepath.Join(workspacePath, ".vscode"), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(workspacePath, ".vscode", "mcp.json"), data, 0o644)
}

func writeVisualStudioMCPConfig(solutionPath, vaultPath string) error {
	command := resolveRicketCommand()

	type visualStudioServer struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	type visualStudioMCPConfig struct {
		Servers map[string]visualStudioServer `json:"servers"`
	}

	cfg := visualStudioMCPConfig{
		Servers: map[string]visualStudioServer{
			"ricket": {
				Type:    "stdio",
				Command: command,
				Args:    []string{"serve"},
				Env: map[string]string{
					"RICKET_VAULT_ROOT": vaultPath,
				},
			},
		},
	}

	if err := os.MkdirAll(filepath.Join(solutionPath, ".vs"), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(solutionPath, ".vs", "mcp.json"), data, 0o644)
}

// writeClaudeCodeMCPConfig merges a ricket entry into ~/.claude/mcp.json.
// Creates the file if it does not exist; preserves existing server entries.
func writeClaudeCodeMCPConfig(vaultPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}

	configPath := filepath.Join(claudeDir, "mcp.json")

	// Load existing config or start fresh
	existing := make(map[string]any)
	if data, err := os.ReadFile(configPath); err == nil {
		if jsonErr := json.Unmarshal(data, &existing); jsonErr != nil {
			existing = make(map[string]any) // corrupt file — start fresh
		}
	}

	// Get or create the mcpServers map
	servers, _ := existing["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
	}

	servers["ricket"] = map[string]any{
		"command": resolveRicketCommand(),
		"args":    []string{"serve"},
		"env": map[string]string{
			"RICKET_VAULT_ROOT": vaultPath,
		},
	}
	existing["mcpServers"] = servers

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(configPath, data, 0o644)
}

func resolveRicketCommand() string {
	if p, err := exec.LookPath("ricket"); err == nil {
		if abs, absErr := filepath.Abs(p); absErr == nil {
			return abs
		}
		return p
	}
	if abs, err := filepath.Abs(os.Args[0]); err == nil {
		return abs
	}
	return "ricket"
}

// ── serve ─────────────────────────────────────────────────────────────────────

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the ricket MCP server over stdio",
		Long: `Start the ricket MCP server over stdio (JSON-RPC 2.0).

If ricket.yaml is not found, the server starts in migration mode: only
vault_analyze and vault_write_config are available. After writing config,
restart the server (reload your editor window) to enable all tools.

Vault root resolution order:
  1. --vault-root flag
  2. RICKET_VAULT_ROOT environment variable
  3. default_vault in ~/.config/ricket/config.yaml
  4. Current working directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			return ricketmcp.New(root).Start()
		},
	}
}

// ── status ────────────────────────────────────────────────────────────────────

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print vault statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}

			cfg, err := config.LoadConfig(root)
			if err != nil {
				return err
			}

			v := vault.New(cfg)
			defer v.Close() //nolint:errcheck
			status, err := v.Status()
			if err != nil {
				return err
			}

			fmt.Printf("Vault:       %s\n", cfg.VaultRoot)
			fmt.Printf("Total notes: %d\n", status.TotalNotes)
			fmt.Printf("Inbox:       %d notes\n", status.InboxCount)
			fmt.Printf("Categories:  %d\n", status.Categories)

			if status.InboxCount > 0 {
				inbox, err := v.ListInbox()
				if err == nil {
					fmt.Println("\nInbox:")
					for _, n := range inbox {
						tags := vault.GetTags(n.Parsed)
						if len(tags) > 0 {
							fmt.Printf("  - %s  [%s]\n", n.Path, strings.Join(tags, ", "))
						} else {
							fmt.Printf("  - %s\n", n.Path)
						}
					}
				}
			}

			return nil
		},
	}
}

// ── config ────────────────────────────────────────────────────────────────────

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ricket user configuration",
	}

	setDefault := &cobra.Command{
		Use:   "set-default [vault-path]",
		Short: "Set the default vault in ~/.config/ricket/config.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var vaultPath string
			if len(args) == 1 {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				vaultPath = abs
			} else {
				root, err := resolveRoot()
				if err != nil {
					return err
				}
				vaultPath = root
			}
			if err := writeUserConfig(vaultPath); err != nil {
				return err
			}
			fmt.Printf("Default vault set to: %s\n", vaultPath)
			return nil
		},
	}

	showPath := &cobra.Command{
		Use:   "path",
		Short: "Show the resolved vault root",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			fmt.Println(root)
			return nil
		},
	}

	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate vault configuration and directory structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}

			cfg, err := config.LoadConfig(root)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}

			ok := true
			warn := func(format string, a ...any) {
				fmt.Fprintf(os.Stderr, "  WARN  "+format+"\n", a...)
				ok = false
			}
			info := func(format string, a ...any) {
				fmt.Printf("  OK    "+format+"\n", a...)
			}

			fmt.Printf("Vault: %s\n\n", cfg.VaultRoot)

			for _, sub := range []struct{ name, path string }{
				{"inbox", cfg.Vault.Inbox},
				{"archive", cfg.Vault.Archive},
				{"templates", cfg.Vault.Templates},
			} {
				absDir := filepath.Join(cfg.VaultRoot, filepath.FromSlash(sub.path))
				if _, err := os.Stat(absDir); err != nil {
					warn("%s directory missing: %s", sub.name, sub.path)
				} else {
					info("%s directory exists: %s", sub.name, sub.path)
				}
			}

			fmt.Printf("\nCategories (%d):\n", len(cfg.Categories))
			for _, cat := range cfg.Categories {
				absFolder := filepath.Join(cfg.VaultRoot, filepath.FromSlash(cat.Folder))
				if _, err := os.Stat(absFolder); err != nil {
					warn("category %q folder missing: %s (will be created on first use)", cat.Name, cat.Folder)
				} else {
					info("category %q → %s", cat.Name, cat.Folder)
				}
				if cat.Template != "" {
					tmplAbs := filepath.Join(cfg.VaultRoot, filepath.FromSlash(cfg.Vault.Templates), cat.Template+".md")
					if _, err := os.Stat(tmplAbs); err != nil {
						warn("category %q references missing template: %s", cat.Name, cat.Template)
					}
				}
				if cat.MOC != "" {
					mocAbs := filepath.Join(cfg.VaultRoot, filepath.FromSlash(cat.MOC))
					if _, err := os.Stat(mocAbs); err != nil {
						warn("category %q MOC file missing: %s", cat.Name, cat.MOC)
					}
				}
			}

			fmt.Println()
			if ok {
				fmt.Println("Vault configuration looks good.")
			} else {
				fmt.Fprintln(os.Stderr, "\nValidation completed with warnings.")
			}
			return nil
		},
	}

	scaffold := &cobra.Command{
		Use:   "scaffold",
		Short: "Create any missing vault folders, templates, and MOC files from ricket.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(root)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			if err := vault.ScaffoldVault(cfg); err != nil {
				return err
			}
			fmt.Println("Scaffolding complete.")
			return nil
		},
	}

	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate ricket.yaml to newer defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadConfig(root)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			addPeople, _ := cmd.Flags().GetBool("add-people")
			if !addPeople {
				fmt.Println("No migrations selected. Use --add-people.")
				return nil
			}
			added := ensurePeopleCategories(cfg)
			if added == 0 {
				fmt.Println("No people categories needed. ricket.yaml unchanged.")
				return nil
			}
			if err := config.WriteConfig(cfg, root); err != nil {
				return err
			}
			fmt.Printf("Added %d people categories to ricket.yaml.\n", added)
			return nil
		},
	}
	migrate.Flags().Bool("add-people", false, "Add missing per-organization people categories")

	cmd.AddCommand(setDefault, showPath, validate, scaffold, migrate)
	return cmd
}

func ensurePeopleCategories(cfg *config.RicketConfig) int {
	existingByName := map[string]bool{}
	for _, c := range cfg.Categories {
		existingByName[c.Name] = true
	}

	type orgInfo struct {
		Tag  string
		Area string
	}
	orgs := map[string]orgInfo{}

	for _, c := range cfg.Categories {
		tag := categoryTagPrefix(c.Name)
		if tag == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(c.Name), "-people") {
			continue
		}
		if area := inferAreaRoot(c.Folder); area != "" {
			if _, ok := orgs[tag]; !ok {
				orgs[tag] = orgInfo{Tag: tag, Area: area}
			}
		}
	}

	added := 0
	for _, org := range orgs {
		name := org.Tag + "-people"
		if existingByName[name] {
			continue
		}
		cfg.Categories = append(cfg.Categories, config.Category{
			Name:     name,
			Folder:   org.Area + "people/",
			Template: "person",
			Naming:   "{topic}.md",
			Tags:     []string{"person", org.Tag},
			MOC:      org.Area + "people/MOC.md",
			Signals:  []string{"person", "people", "stakeholder", "owner", "contact", "manager", "teammate"},
		})
		added++
	}

	return added
}

func categoryTagPrefix(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	suffixes := []string{"-decision", "-concept", "-meeting", "-project", "-people"}
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return strings.TrimSuffix(name, s)
		}
	}
	return ""
}

func inferAreaRoot(folder string) string {
	f := filepath.ToSlash(strings.TrimSpace(folder))
	if strings.HasPrefix(f, "Areas/") {
		parts := strings.Split(f, "/")
		if len(parts) >= 2 && parts[1] != "" {
			return "Areas/" + parts[1] + "/"
		}
	}
	return ""
}

// ── mcp ───────────────────────────────────────────────────────────────────────

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Generate MCP client configuration files",
	}

	vscode := &cobra.Command{
		Use:   "init-vscode [workspace-path]",
		Short: "Write .vscode/mcp.json for GitHub Copilot in VS Code",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace := "."
			if len(args) == 1 {
				workspace = args[0]
			}
			workspaceAbs, err := filepath.Abs(workspace)
			if err != nil {
				return err
			}
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if err := writeVSCodeMCPConfig(workspaceAbs, root); err != nil {
				return err
			}
			fmt.Printf("Wrote %s\n", filepath.Join(workspaceAbs, ".vscode", "mcp.json"))
			return nil
		},
	}

	visualStudio := &cobra.Command{
		Use:   "init-visualstudio [solution-path]",
		Short: "Write .vs/mcp.json for GitHub Copilot in Visual Studio",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			solution := "."
			if len(args) == 1 {
				solution = args[0]
			}
			solutionAbs, err := filepath.Abs(solution)
			if err != nil {
				return err
			}
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if err := writeVisualStudioMCPConfig(solutionAbs, root); err != nil {
				return err
			}
			fmt.Printf("Wrote %s\n", filepath.Join(solutionAbs, ".vs", "mcp.json"))
			return nil
		},
	}

	claudeCode := &cobra.Command{
		Use:   "init-claude-code",
		Short: "Add ricket to ~/.claude/mcp.json for Claude Code (global)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if err := writeClaudeCodeMCPConfig(root); err != nil {
				return err
			}
			home, _ := os.UserHomeDir()
			fmt.Printf("Wrote ricket entry to %s\n", filepath.Join(home, ".claude", "mcp.json"))
			return nil
		},
	}

	cmd.AddCommand(vscode, visualStudio, claudeCode)
	return cmd
}

// ── completion ────────────────────────────────────────────────────────────────

func completionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(args[0]) {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
}
