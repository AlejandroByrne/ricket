package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	ricketmcp "github.com/AlejandroByrne/ricket/internal/mcp"
)

var vaultRoot string

func main() {
	root := &cobra.Command{
		Use:     "ricket",
		Short:   "Vault-powered MCP server for AI coding agents",
		Version: "0.5.0",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			return ricketmcp.New(root).Start()
		},
	}
	root.PersistentFlags().StringVarP(&vaultRoot, "vault-root", "r", "",
		"Vault root directory (overrides RICKET_VAULT_ROOT env var and ~/.config/ricket/config.yaml)")

	root.AddCommand(serveCmd())

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

// serveCmd is an alias for the root command, kept for backward compatibility.
func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the ricket MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot()
			if err != nil {
				return err
			}
			return ricketmcp.New(root).Start()
		},
	}
}
