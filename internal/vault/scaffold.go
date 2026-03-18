package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlejandroByrne/ricket/internal/config"
)

// ScaffoldVault creates any missing folders, template stubs, and MOC files
// defined in cfg. Safe to call on an existing vault — skips files that already exist.
func ScaffoldVault(cfg *config.RicketConfig) error {
	for _, d := range []string{cfg.Vault.Inbox, cfg.Vault.Archive, cfg.Vault.Templates} {
		if err := os.MkdirAll(filepath.Join(cfg.VaultRoot, filepath.FromSlash(d)), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", d, err)
		}
	}

	for _, c := range cfg.Categories {
		if err := os.MkdirAll(filepath.Join(cfg.VaultRoot, filepath.FromSlash(c.Folder)), 0o755); err != nil {
			return fmt.Errorf("failed to create category folder %s: %w", c.Folder, err)
		}

		if c.Template != "" {
			tmplPath := filepath.Join(cfg.VaultRoot, filepath.FromSlash(cfg.Vault.Templates), c.Template+".md")
			if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
				if err := os.WriteFile(tmplPath, []byte(DefaultTemplateContent(c.Template)), 0o644); err != nil {
					return fmt.Errorf("failed to write template %s: %w", c.Template, err)
				}
			}
		}

		if c.MOC != "" {
			mocPath := filepath.Join(cfg.VaultRoot, filepath.FromSlash(c.MOC))
			if err := os.MkdirAll(filepath.Dir(mocPath), 0o755); err != nil {
				return fmt.Errorf("failed to create MOC folder for %s: %w", c.MOC, err)
			}
			if _, err := os.Stat(mocPath); os.IsNotExist(err) {
				title := strings.TrimSuffix(filepath.Base(c.MOC), ".md")
				if title == "" {
					title = "MOC"
				}
				if err := os.WriteFile(mocPath, []byte("# "+title+"\n\n"), 0o644); err != nil {
					return fmt.Errorf("failed to write MOC %s: %w", c.MOC, err)
				}
			}
		}
	}

	return nil
}

// DefaultTemplateContent returns starter Obsidian Templater content for a
// named template type. Returns a generic template for unknown names.
func DefaultTemplateContent(name string) string {
	switch name {
	case "decision":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [decision]\nstatus: active\n---\n\n# <% tp.file.title %>\n\n## Decision\n\n## Rationale\n\n## Consequences\n\n## Alternatives Considered\n\n## Links\n"
	case "concept":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [concept]\n---\n\n# <% tp.file.title %>\n\n## What It Is\n\n## How We Use It\n\n## Examples\n\n## Links\n"
	case "meeting":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [meeting]\nattendees: []\n---\n\n# <% tp.file.title %>\n\n## Agenda\n\n## Notes\n\n## Action Items\n\n## Decisions Made\n\n## Links\n"
	case "project":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [project]\nstatus: active\n---\n\n# <% tp.file.title %>\n\n## Goal\n\n## Scope\n\n## Progress\n\n## Decisions\n\n## Links\n"
	case "learning":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [learning]\n---\n\n# <% tp.file.title %>\n\n## Summary\n\n## Key Concepts\n\n## How I'll Use This\n\n## Links\n"
	case "person":
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [person]\nrole: \"\"\nteam: \"\"\n---\n\n# <% tp.file.title %>\n\n## Who They Are\n\n## Context\n\n## Current Work\n\n## Notes\n\n## Links\n"
	case "journal":
		return "---\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\ntags: [journal]\n---\n\n# <% tp.date.now(\"YYYY-MM-DD\") %>\n\n## Today\n\n## Notes\n\n## Links\n"
	default:
		return "---\ntitle: <% tp.file.title %>\ndate: <% tp.date.now(\"YYYY-MM-DD\") %>\n---\n\n# <% tp.file.title %>\n\n## Notes\n"
	}
}
