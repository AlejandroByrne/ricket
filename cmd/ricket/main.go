package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/AlejandroByrne/ricket/internal/config"
	ricketmcp "github.com/AlejandroByrne/ricket/internal/mcp"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

var vaultRoot string

func main() {
	root := &cobra.Command{
		Use:     "ricket",
		Short:   "Vault-powered context engine for AI coding agents",
		Version: "0.1.0",
	}
	root.PersistentFlags().StringVarP(&vaultRoot, "vault-root", "r", "",
		"Vault root directory (overrides RICKET_VAULT_ROOT env var and ~/.config/ricket/config.yaml)")

	root.AddCommand(initCmd(), serveCmd(), statusCmd(), configCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveRoot returns the vault root using this precedence:
//  1. --vault-root CLI flag
//  2. RICKET_VAULT_ROOT environment variable
//  3. default_vault in ~/.config/ricket/config.yaml
//  4. Current working directory
func resolveRoot() (string, error) {
	if vaultRoot != "" {
		return filepath.Abs(vaultRoot)
	}
	if env := os.Getenv("RICKET_VAULT_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	if home, err := os.UserHomeDir(); err == nil {
		cfgPath := filepath.Join(home, ".config", "ricket", "config.yaml")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var uc struct {
				DefaultVault string `yaml:"default_vault"`
			}
			if err := yaml.Unmarshal(data, &uc); err == nil && uc.DefaultVault != "" {
				return filepath.Abs(uc.DefaultVault)
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

// ── init (interactive wizard) ─────────────────────────────────────────────────

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Interactive setup wizard — creates ricket.yaml for your vault",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := func() (string, error) {
				if vaultRoot != "" {
					return filepath.Abs(vaultRoot)
				}
				if len(args) == 1 {
					return filepath.Abs(args[0])
				}
				wd, err := os.Getwd()
				return wd, err
			}()
			if err != nil {
				return err
			}

			if _, err := os.Stat(filepath.Join(root, "ricket.yaml")); err == nil {
				return fmt.Errorf("ricket.yaml already exists at %s — delete it first to reinitialise", root)
			}

			return runWizard(root)
		},
	}
}

// runWizard runs the interactive setup wizard and writes ricket.yaml.
func runWizard(defaultRoot string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintln(os.Stderr, "\n╔══════════════════════════════════════╗")
	fmt.Fprintln(os.Stderr, "║  ricket setup wizard                 ║")
	fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════╝")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Press Enter to accept defaults shown in [brackets].")
	fmt.Fprintln(os.Stderr, "")

	// ── Vault location ──────────────────────────────────────────────────
	vaultPath := prompt(reader, "Vault directory", defaultRoot)
	abs, err := filepath.Abs(vaultPath)
	if err != nil {
		return fmt.Errorf("invalid vault path: %w", err)
	}
	vaultPath = abs

	// ── Folder names ────────────────────────────────────────────────────
	fmt.Fprintln(os.Stderr, "\n── Vault folders ──")
	inboxFolder := prompt(reader, "Inbox folder", "Inbox")
	archiveFolder := prompt(reader, "Archive folder", "Archive")
	templatesFolder := prompt(reader, "_templates folder", "_templates")

	// ── Organisations ───────────────────────────────────────────────────
	fmt.Fprintln(os.Stderr, "\n── Organisations ──")
	fmt.Fprintln(os.Stderr, "How many organisations/workplaces does this vault cover?")
	fmt.Fprintln(os.Stderr, "(1 = just me, 2 = work + personal side projects, etc.)")
	numOrgsStr := prompt(reader, "Number of organisations", "1")
	numOrgs := 1
	fmt.Sscanf(numOrgsStr, "%d", &numOrgs)
	if numOrgs < 1 {
		numOrgs = 1
	}
	if numOrgs > 8 {
		numOrgs = 8
	}

	orgs := make([]orgEntry, 0, numOrgs)
	for i := 0; i < numOrgs; i++ {
		fmt.Fprintf(os.Stderr, "\nOrganisation %d:\n", i+1)
		orgName := prompt(reader, "  Name (e.g. Acme Corp, Personal)", fmt.Sprintf("Org%d", i+1))
		tagDefault := strings.ToLower(strings.ReplaceAll(orgName, " ", "-"))
		tagDefault = sanitiseTag(tagDefault)
		orgTag := prompt(reader, "  Tag prefix (used in frontmatter)", tagDefault)
		orgTag = sanitiseTag(orgTag)
		isWork := promptBool(reader, "  Is this an employer/client (not personal)?", true)
		orgs = append(orgs, orgEntry{Name: orgName, Tag: orgTag, IsWork: isWork})
	}

	// ── Category choices ────────────────────────────────────────────────
	fmt.Fprintln(os.Stderr, "\n── Note categories ──")
	wantDecisions := false
	wantConcepts := false
	wantMeetings := false
	wantProjects := false
	wantLearning := false
	wantResources := false
	wantJournal := false

	for _, o := range orgs {
		if o.IsWork {
			wantDecisions = wantDecisions || promptBool(reader,
				fmt.Sprintf("  Include decision/standards notes for %s?", o.Name), true)
			wantConcepts = wantConcepts || promptBool(reader,
				fmt.Sprintf("  Include concept/explanation notes for %s?", o.Name), true)
			wantMeetings = wantMeetings || promptBool(reader,
				fmt.Sprintf("  Include meeting notes for %s?", o.Name), true)
			wantProjects = wantProjects || promptBool(reader,
				fmt.Sprintf("  Include project notes for %s?", o.Name), true)
		}
	}
	wantLearning = promptBool(reader, "  Include personal learning notes?", true)
	wantResources = promptBool(reader, "  Include reference/resource notes?", false)
	wantJournal = promptBool(reader, "  Include daily journal entries?", false)

	// ── Inbox signal hints ──────────────────────────────────────────────
	fmt.Fprintln(os.Stderr, "\n── Inbox signals ──")
	fmt.Fprintln(os.Stderr, "What typically ends up in your Inbox? (used to help AI classify captures)")
	fmt.Fprintln(os.Stderr, "Select all that apply — press Enter to toggle defaults:")
	captureVoice := promptBool(reader, "  Voice/quick captures?", true)
	captureMeetings := promptBool(reader, "  Draft meeting notes?", true)
	captureClippings := promptBool(reader, "  Web clippings / links?", false)
	captureCode := promptBool(reader, "  Code snippets / technical captures?", false)

	// ── Build config ─────────────────────────────────────────────────────
	cfg := buildConfigFromWizard(wizardAnswers{
		VaultRoot:       vaultPath,
		InboxFolder:     normaliseFolder(inboxFolder),
		ArchiveFolder:   normaliseFolder(archiveFolder),
		TemplatesFolder: normaliseFolder(templatesFolder),
		Orgs:            orgs,
		WantDecisions:   wantDecisions,
		WantConcepts:    wantConcepts,
		WantMeetings:    wantMeetings,
		WantProjects:    wantProjects,
		WantLearning:    wantLearning,
		WantResources:   wantResources,
		WantJournal:     wantJournal,
		InboxSignals: buildInboxSignals(
			captureVoice, captureMeetings, captureClippings, captureCode),
	})

	// ── Write files ──────────────────────────────────────────────────────
	if err := os.MkdirAll(filepath.Join(vaultPath, ".ricket"), 0o755); err != nil {
		return fmt.Errorf("failed to create .ricket/: %w", err)
	}

	if err := config.WriteConfig(cfg, vaultPath); err != nil {
		return err
	}

	// Offer to set as default vault
	setDefault := promptBool(reader, "\nSet this as your default vault in ~/.config/ricket/config.yaml?", true)
	if setDefault {
		if err := writeUserConfig(vaultPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not write user config: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  ✓ ~/.config/ricket/config.yaml updated\n")
		}
	}

	fmt.Fprintf(os.Stderr, "\n✓ ricket.yaml written to %s\n", filepath.Join(vaultPath, "ricket.yaml"))
	fmt.Fprintln(os.Stderr, "\nNext steps:")
	fmt.Fprintln(os.Stderr, "  1. Review and edit ricket.yaml to fine-tune categories")
	fmt.Fprintln(os.Stderr, "  2. Add ricket to your MCP client — see README.md")
	fmt.Fprintf(os.Stderr, "  3. Run: ricket status --vault-root %s\n", vaultPath)
	return nil
}

type orgEntry struct {
	Name   string
	Tag    string
	IsWork bool
}

type wizardAnswers struct {
	VaultRoot       string
	InboxFolder     string
	ArchiveFolder   string
	TemplatesFolder string
	Orgs            []orgEntry
	WantDecisions   bool
	WantConcepts    bool
	WantMeetings    bool
	WantProjects    bool
	WantLearning    bool
	WantResources   bool
	WantJournal     bool
	InboxSignals    []string
}

func buildConfigFromWizard(a wizardAnswers) *config.RicketConfig {
	cfg := &config.RicketConfig{
		VaultRoot: a.VaultRoot,
		Vault: config.VaultConfig{
			Root:      a.VaultRoot,
			Inbox:     a.InboxFolder,
			Archive:   a.ArchiveFolder,
			Templates: a.TemplatesFolder,
		},
		MCP: &config.MCPConfig{
			Name:    "ricket",
			Version: "0.1.0",
		},
	}

	for _, org := range a.Orgs {
		orgArea := "Areas/" + org.Name + "/"
		orgProjects := "Projects/" + org.Name + "/"

		if org.IsWork {
			if a.WantDecisions {
				cfg.Categories = append(cfg.Categories, config.Category{
					Name:     org.Tag + "-decision",
					Folder:   orgArea + "decisions/",
					Template: "decision",
					Naming:   "use-{topic}.md",
					Tags:     []string{"decision", org.Tag},
					MOC:      orgArea + "decisions/MOC.md",
					Signals:  append(a.InboxSignals, "decision", "standard", "convention", "rule", "architecture"),
				})
			}
			if a.WantConcepts {
				cfg.Categories = append(cfg.Categories, config.Category{
					Name:     org.Tag + "-concept",
					Folder:   orgArea + "concepts/",
					Template: "concept",
					Naming:   "{topic}.md",
					Tags:     []string{"concept", org.Tag},
					MOC:      orgArea + "concepts/MOC.md",
					Signals:  []string{"concept", "explain", "understand", "definition", "how"},
				})
			}
			if a.WantMeetings {
				cfg.Categories = append(cfg.Categories, config.Category{
					Name:     org.Tag + "-meeting",
					Folder:   orgArea + "meetings/",
					Template: "meeting",
					Naming:   "YYYY-MM-DD-{topic}.md",
					Tags:     []string{"meeting", org.Tag},
					Signals:  []string{"meeting", "standup", "sync", "planning", "retro", "review"},
				})
			}
		}

		if a.WantProjects {
			cfg.Categories = append(cfg.Categories, config.Category{
				Name:     org.Tag + "-project",
				Folder:   orgProjects,
				Template: "project",
				Naming:   "{topic}.md",
				Tags:     []string{"project", org.Tag},
				MOC:      orgProjects + "MOC.md",
				Signals:  []string{"project", "task", "feature", "initiative", "epic", "ticket"},
			})
		}
	}

	if a.WantLearning {
		cfg.Categories = append(cfg.Categories, config.Category{
			Name:     "learning",
			Folder:   "Areas/Personal Development/",
			Template: "learning",
			Naming:   "{topic}.md",
			Tags:     []string{"learning"},
			MOC:      "Areas/Personal Development/MOC.md",
			Signals:  []string{"learning", "skill", "training", "course", "book", "reading"},
		})
	}

	if a.WantResources {
		cfg.Categories = append(cfg.Categories, config.Category{
			Name:    "resource",
			Folder:  "Resources/",
			Naming:  "{topic}.md",
			Tags:    []string{"resource"},
			Signals: []string{"reference", "resource", "link", "doc", "documentation"},
		})
	}

	if a.WantJournal {
		cfg.Categories = append(cfg.Categories, config.Category{
			Name:     "journal",
			Folder:   "Journal/",
			Template: "journal",
			Naming:   "YYYY-MM-DD.md",
			Tags:     []string{"journal"},
			Signals:  []string{"journal", "daily", "log", "standup", "today"},
		})
	}

	// Fallback: ensure at least one category exists
	if len(cfg.Categories) == 0 {
		cfg.Categories = []config.Category{
			{
				Name:    "note",
				Folder:  "Notes/",
				Tags:    []string{"note"},
				Signals: []string{"note", "capture"},
			},
		}
	}

	return cfg
}

func buildInboxSignals(voice, meetings, clippings, code bool) []string {
	var s []string
	if voice {
		s = append(s, "capture", "quick note", "voice")
	}
	if meetings {
		s = append(s, "meeting", "sync", "discussion")
	}
	if clippings {
		s = append(s, "link", "article", "clipping")
	}
	if code {
		s = append(s, "snippet", "gist", "code")
	}
	return s
}

func normaliseFolder(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Inbox/"
	}
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

func sanitiseTag(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ── serve ─────────────────────────────────────────────────────────────────────

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the ricket MCP server over stdio",
		Long: `Start the ricket MCP server over stdio (JSON-RPC 2.0).

Vault root resolution order:
  1. --vault-root flag
  2. RICKET_VAULT_ROOT environment variable
  3. default_vault in ~/.config/ricket/config.yaml
  4. Current working directory

Claude Code (~/.claude/mcp.json):
  {
    "mcpServers": {
      "ricket": {
        "command": "ricket",
        "args": ["serve"],
        "env": { "RICKET_VAULT_ROOT": "/path/to/vault" }
      }
    }
  }

GitHub Copilot (.vscode/mcp.json):
  {
    "servers": {
      "ricket": {
        "type": "stdio",
        "command": "ricket",
        "args": ["serve"],
        "env": { "RICKET_VAULT_ROOT": "/path/to/vault" }
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(root, "ricket.yaml")); os.IsNotExist(err) {
				return fmt.Errorf("ricket.yaml not found in %s\nRun: ricket init %s", root, root)
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

	cmd.AddCommand(setDefault, showPath)
	return cmd
}

// ── Prompt helpers ────────────────────────────────────────────────────────────

func prompt(r *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "  %s [%s]: ", question, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "  %s: ", question)
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return defaultVal
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptBool(r *bufio.Reader, question string, defaultVal bool) bool {
	hint := "Y/n"
	if !defaultVal {
		hint = "y/N"
	}
	fmt.Fprintf(os.Stderr, "  %s [%s]: ", question, hint)
	line, err := r.ReadString('\n')
	if err != nil {
		return defaultVal
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultVal
	}
	return line == "y" || line == "yes"
}
