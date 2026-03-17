// Package git provides a structured git audit trail for vault operations.
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// GitAudit commits vault file operations to git for audit trail.
type GitAudit struct {
	vaultRoot string
}

// New creates a GitAudit for the given vault root.
func New(vaultRoot string) *GitAudit {
	return &GitAudit{vaultRoot: vaultRoot}
}

// IsGitRepo returns true if the vault root is inside a git repository.
func (g *GitAudit) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = g.vaultRoot
	return cmd.Run() == nil
}

// Commit stages the given files and commits with message.
// Uses --no-gpg-sign to avoid GPG issues.
// Returns false if nothing to commit or on any error (best-effort audit).
func (g *GitAudit) Commit(files []string, message string) bool {
	if len(files) > 0 {
		args := append([]string{"add"}, files...)
		cmd := exec.Command("git", args...)
		cmd.Dir = g.vaultRoot
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	cmd := exec.Command("git", "commit", "-m", message, "--no-gpg-sign")
	cmd.Dir = g.vaultRoot
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// CommitFileMove stages source (deleted) and destination (added), then commits.
// Message format: "ricket: filed {source-basename} → {destination}"
func (g *GitAudit) CommitFileMove(source, destination string) bool {
	sourceBasename := filepath.Base(source)

	addSrc := exec.Command("git", "add", source)
	addSrc.Dir = g.vaultRoot
	addSrc.Run() //nolint:errcheck

	addDst := exec.Command("git", "add", destination)
	addDst.Dir = g.vaultRoot
	addDst.Run() //nolint:errcheck

	message := fmt.Sprintf("ricket: filed %s → %s", sourceBasename, destination)
	cmd := exec.Command("git", "commit", "-m", message, "--no-gpg-sign")
	cmd.Dir = g.vaultRoot
	return cmd.Run() == nil
}
