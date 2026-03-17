// Package mcp implements the ricket MCP server.
package mcp

import (
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AlejandroByrne/ricket/internal/config"
	"github.com/AlejandroByrne/ricket/internal/vault"
)

// RicketMCPServer wraps the vault and config for MCP tool handlers.
type RicketMCPServer struct {
	vaultRoot string
	vault     *vault.Vault
	cfg       *config.RicketConfig
}

// New creates a RicketMCPServer for the given vault root.
func New(vaultRoot string) *RicketMCPServer {
	return &RicketMCPServer{vaultRoot: vaultRoot}
}

// Start loads config, initialises the vault, registers tools, then
// serves over stdio until the process is signalled.
func (s *RicketMCPServer) Start() error {
	cfg, err := config.LoadConfig(s.vaultRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	s.cfg = cfg
	s.vault = vault.New(cfg)
	defer s.vault.Close() //nolint:errcheck — best-effort on shutdown

	name := "ricket"
	version := "0.1.0"
	if cfg.MCP != nil {
		if cfg.MCP.Name != "" {
			name = cfg.MCP.Name
		}
		if cfg.MCP.Version != "" {
			version = cfg.MCP.Version
		}
	}

	srv := mcpserver.NewMCPServer(name, version)
	registerTools(srv, s)

	fmt.Fprintf(os.Stderr, "ricket MCP server running (vault: %s)\n", s.vaultRoot)
	return mcpserver.ServeStdio(srv)
}
