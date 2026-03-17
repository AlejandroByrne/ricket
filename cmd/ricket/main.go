package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
	root.PersistentFlags().StringVarP(&vaultRoot, "vault-root", "r", "", "Vault root directory (default: current directory)")

	root.AddCommand(initCmd(), serveCmd(), statusCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveRoot returns the effective vault root (flag > cwd).
func resolveRoot() (string, error) {
	if vaultRoot != "" {
		return filepath.Abs(vaultRoot)
	}
	return os.Getwd()
}

// ── init ─────────────────────────────────────────────────────────────────────

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Generate a default ricket.yaml for a vault",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				root, err = filepath.Abs(args[0])
				if err != nil {
					return err
				}
			}

			configPath := filepath.Join(root, "ricket.yaml")
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("ricket.yaml already exists at %s", configPath)
			}

			// Scan for PARA folders
			detected := []string{}
			for _, folder := range []string{"Projects", "Areas", "Resources", "Archive"} {
				if _, err := os.Stat(filepath.Join(root, folder)); err == nil {
					detected = append(detected, folder)
				}
			}

			hasTemplates := false
			if _, err := os.Stat(filepath.Join(root, "_templates")); err == nil {
				hasTemplates = true
			}

			// Create .ricket directory
			ricketDir := filepath.Join(root, ".ricket")
			if err := os.MkdirAll(ricketDir, 0o755); err != nil {
				return fmt.Errorf("failed to create .ricket/: %w", err)
			}

			cfg := config.GenerateDefaultConfig(root)
			if err := config.WriteConfig(cfg, root); err != nil {
				return err
			}

			fmt.Printf("Initialized ricket at %s\n", root)
			if len(detected) > 0 {
				fmt.Printf("Detected PARA folders: %v\n", detected)
			}
			if hasTemplates {
				fmt.Println("Detected _templates/ directory")
			}
			fmt.Printf("Wrote %s\n", configPath)
			fmt.Println("Edit ricket.yaml to customize categories for your vault.")
			return nil
		},
	}
}

// ── serve ─────────────────────────────────────────────────────────────────────

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the ricket MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}

			if _, err := os.Stat(filepath.Join(root, "ricket.yaml")); os.IsNotExist(err) {
				return fmt.Errorf("ricket.yaml not found in %s — run 'ricket init' first", root)
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
						fmt.Printf("  - %s\n", n.Path)
					}
				}
			}

			return nil
		},
	}
}
