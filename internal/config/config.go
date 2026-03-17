// Package config handles loading and generating ricket.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RicketConfig is the top-level config loaded from ricket.yaml.
type RicketConfig struct {
	Vault      VaultConfig  `yaml:"vault"`
	Categories []Category   `yaml:"categories"`
	MCP        *MCPConfig   `yaml:"mcp,omitempty"`
	// Resolved at load time — not in YAML
	VaultRoot string `yaml:"-"`
}

// VaultConfig holds folder paths relative to vault root.
type VaultConfig struct {
	Root      string `yaml:"root,omitempty"` // optional explicit root
	Inbox     string `yaml:"inbox"`
	Archive   string `yaml:"archive"`
	Templates string `yaml:"templates"`
}

// Category defines a note category with its folder, template, tags, and MOC.
type Category struct {
	Name     string   `yaml:"name"`
	Folder   string   `yaml:"folder"`
	Template string   `yaml:"template,omitempty"`
	Naming   string   `yaml:"naming,omitempty"`
	Tags     []string `yaml:"tags"`
	MOC      string   `yaml:"moc,omitempty"`
	Signals  []string `yaml:"signals,omitempty"`
}

// MCPConfig holds optional MCP server metadata.
type MCPConfig struct {
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version,omitempty"`
}

// rawConfig mirrors the YAML structure for parsing (before defaults are applied).
type rawConfig struct {
	Vault struct {
		Root      string `yaml:"root"`
		Inbox     string `yaml:"inbox"`
		Archive   string `yaml:"archive"`
		Templates string `yaml:"templates"`
	} `yaml:"vault"`
	Categories []struct {
		Name     string   `yaml:"name"`
		Folder   string   `yaml:"folder"`
		Template string   `yaml:"template"`
		Naming   string   `yaml:"naming"`
		Tags     []string `yaml:"tags"`
		MOC      string   `yaml:"moc"`
		Signals  []string `yaml:"signals"`
	} `yaml:"categories"`
	MCP *MCPConfig `yaml:"mcp"`
}

// LoadConfig reads ricket.yaml from vaultRoot, validates it, and returns a RicketConfig.
// vault.root in the YAML is optional — if absent, vaultRoot is used.
func LoadConfig(vaultRoot string) (*RicketConfig, error) {
	configPath := filepath.Join(vaultRoot, "ricket.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ricket.yaml not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read ricket.yaml: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid YAML in ricket.yaml: %w", err)
	}

	// Resolve vault root
	resolvedRoot := vaultRoot
	if raw.Vault.Root != "" {
		resolvedRoot, err = filepath.Abs(raw.Vault.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve vault.root: %w", err)
		}
	}

	// Apply defaults for vault sub-paths
	inbox := raw.Vault.Inbox
	if inbox == "" {
		inbox = "Inbox/"
	}
	archive := raw.Vault.Archive
	if archive == "" {
		archive = "Archive/"
	}
	templates := raw.Vault.Templates
	if templates == "" {
		templates = "_templates/"
	}

	cfg := &RicketConfig{
		VaultRoot: resolvedRoot,
		Vault: VaultConfig{
			Root:      resolvedRoot,
			Inbox:     inbox,
			Archive:   archive,
			Templates: templates,
		},
		MCP: raw.MCP,
	}

	// Validate and copy categories
	if len(raw.Categories) == 0 {
		return nil, fmt.Errorf("ricket.yaml must have a \"categories\" array")
	}
	for _, c := range raw.Categories {
		if c.Name == "" {
			return nil, fmt.Errorf("each category must have a \"name\" string")
		}
		if c.Folder == "" {
			return nil, fmt.Errorf("category %q must have a \"folder\" string", c.Name)
		}
		if c.Tags == nil {
			return nil, fmt.Errorf("category %q must have a \"tags\" array", c.Name)
		}
		cfg.Categories = append(cfg.Categories, Category{
			Name:     c.Name,
			Folder:   c.Folder,
			Template: c.Template,
			Naming:   c.Naming,
			Tags:     c.Tags,
			MOC:      c.MOC,
			Signals:  c.Signals,
		})
	}

	return cfg, nil
}

// GenerateDefaultConfig creates a PARA-based default config for vaultRoot.
func GenerateDefaultConfig(vaultRoot string) *RicketConfig {
	resolvedRoot, _ := filepath.Abs(vaultRoot)

	return &RicketConfig{
		VaultRoot: resolvedRoot,
		Vault: VaultConfig{
			Root:      resolvedRoot,
			Inbox:     "Inbox/",
			Archive:   "Archive/",
			Templates: "_templates/",
		},
		Categories: []Category{
			{
				Name:     "decision",
				Folder:   "Areas/FCBT Engineering/decisions/",
				Template: "decision",
				Naming:   "use-{topic}.md",
				Tags:     []string{"decision", "fcbt"},
				MOC:      "Areas/FCBT Engineering/decisions/MOC.md",
				Signals:  []string{"decision", "standard", "convention", "rule"},
			},
			{
				Name:     "concept",
				Folder:   "Areas/FCBT Engineering/concepts/",
				Template: "concept",
				Naming:   "{topic}.md",
				Tags:     []string{"concept", "fcbt"},
				MOC:      "Areas/FCBT Engineering/concepts/MOC.md",
				Signals:  []string{"concept", "explain", "understand", "definition"},
			},
			{
				Name:     "project",
				Folder:   "Projects/FCBT/",
				Template: "project",
				Naming:   "{topic}.md",
				Tags:     []string{"project", "fcbt"},
				MOC:      "Projects/FCBT/MOC.md",
				Signals:  []string{"project", "task", "feature", "initiative"},
			},
			{
				Name:     "meeting",
				Folder:   "Areas/FCBT Engineering/meetings/",
				Template: "meeting",
				Naming:   "YYYY-MM-DD-{topic}.md",
				Tags:     []string{"meeting", "fcbt"},
				Signals:  []string{"meeting", "standup", "sync", "planning"},
			},
			{
				Name:     "learning",
				Folder:   "Areas/Personal Development/",
				Template: "learning",
				Naming:   "{topic}.md",
				Tags:     []string{"learning", "personal"},
				Signals:  []string{"learning", "skill", "training", "course"},
			},
		},
		MCP: &MCPConfig{
			Name:    "ricket",
			Version: "0.2.0",
		},
	}
}

// WriteConfig serializes cfg to ricket.yaml at vaultRoot.
func WriteConfig(cfg *RicketConfig, vaultRoot string) error {
	configPath := filepath.Join(vaultRoot, "ricket.yaml")

	type outConfig struct {
		Vault struct {
			Root      string `yaml:"root,omitempty"`
			Inbox     string `yaml:"inbox"`
			Archive   string `yaml:"archive"`
			Templates string `yaml:"templates"`
		} `yaml:"vault"`
		Categories []Category `yaml:"categories"`
		MCP        *MCPConfig `yaml:"mcp,omitempty"`
	}

	out := outConfig{}
	out.Vault.Root = cfg.Vault.Root
	out.Vault.Inbox = cfg.Vault.Inbox
	out.Vault.Archive = cfg.Vault.Archive
	out.Vault.Templates = cfg.Vault.Templates
	out.Categories = cfg.Categories
	if cfg.MCP != nil && (cfg.MCP.Name != "" || cfg.MCP.Version != "") {
		out.MCP = cfg.MCP
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write ricket.yaml: %w", err)
	}

	return nil
}
