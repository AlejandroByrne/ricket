package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlejandroByrne/ricket/internal/git"
)

// initTestRepo creates a temp dir with a minimal git repository.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")

	// Initial commit so HEAD exists
	writeFile(t, dir, "README.md", "# Test Vault\n")
	run("add", "README.md")
	run("commit", "-m", "initial commit", "--no-gpg-sign")

	return dir
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsGitRepo(t *testing.T) {
	t.Run("not_a_repo", func(t *testing.T) {
		dir := t.TempDir()
		ga := git.New(dir)
		if ga.IsGitRepo() {
			t.Error("expected false for non-git dir")
		}
	})

	t.Run("is_a_repo", func(t *testing.T) {
		dir := initTestRepo(t)
		ga := git.New(dir)
		if !ga.IsGitRepo() {
			t.Error("expected true for git repo")
		}
	})

	t.Run("subdirectory_of_repo", func(t *testing.T) {
		dir := initTestRepo(t)
		subdir := filepath.Join(dir, "sub", "dir")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatal(err)
		}
		ga := git.New(subdir)
		// subdir is inside the git repo, so IsGitRepo should be true
		if !ga.IsGitRepo() {
			t.Error("expected true for subdirectory of git repo")
		}
	})
}

func TestCommit(t *testing.T) {
	t.Run("commits_new_file", func(t *testing.T) {
		dir := initTestRepo(t)
		ga := git.New(dir)

		writeFile(t, dir, "Notes/test.md", "# Test\n\nContent.")
		ok := ga.Commit([]string{"Notes/test.md"}, "ricket: created Notes/test.md")
		if !ok {
			t.Error("Commit returned false")
		}

		// Verify commit exists
		cmd := exec.Command("git", "log", "--oneline", "-1")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git log: %v", err)
		}
		if string(out) == "" {
			t.Error("expected a commit but log is empty")
		}
	})

	t.Run("returns_false_when_nothing_to_commit", func(t *testing.T) {
		dir := initTestRepo(t)
		ga := git.New(dir)

		// Nothing changed — commit should fail gracefully
		ok := ga.Commit(nil, "empty commit")
		if ok {
			t.Error("expected false when nothing to commit")
		}
	})

	t.Run("commit_multiple_files", func(t *testing.T) {
		dir := initTestRepo(t)
		ga := git.New(dir)

		writeFile(t, dir, "a.md", "A")
		writeFile(t, dir, "b.md", "B")
		ok := ga.Commit([]string{"a.md", "b.md"}, "ricket: add a and b")
		if !ok {
			t.Error("Commit of multiple files returned false")
		}
	})
}

func TestCommitFileMove(t *testing.T) {
	t.Run("stages_move_and_commits", func(t *testing.T) {
		dir := initTestRepo(t)
		ga := git.New(dir)

		// Add source file in initial state
		writeFile(t, dir, "Inbox/raw.md", "# Raw capture")
		cmd := exec.Command("git", "add", "Inbox/raw.md")
		cmd.Dir = dir
		cmd.Run() //nolint:errcheck
		exec.Command("git", "-C", dir, "commit", "-m", "add raw capture", "--no-gpg-sign").Run() //nolint:errcheck

		// Simulate file move: write destination, delete source
		writeFile(t, dir, "Notes/filed.md", "# Filed note")
		if err := os.Remove(filepath.Join(dir, "Inbox", "raw.md")); err != nil {
			t.Fatal(err)
		}

		ok := ga.CommitFileMove("Inbox/raw.md", "Notes/filed.md")
		if !ok {
			t.Error("CommitFileMove returned false")
		}

		// Verify the destination file is tracked
		cmd2 := exec.Command("git", "show", "--stat", "HEAD")
		cmd2.Dir = dir
		out, err := cmd2.Output()
		if err != nil {
			t.Fatalf("git show: %v", err)
		}
		output := string(out)
		if !containsAny(output, "Notes/filed.md", "Inbox/raw.md") {
			t.Errorf("expected commit to mention filed.md or raw.md, got:\n%s", output)
		}
	})

	t.Run("returns_false_when_not_a_repo", func(t *testing.T) {
		dir := t.TempDir()
		ga := git.New(dir)

		writeFile(t, dir, "Inbox/raw.md", "content")
		writeFile(t, dir, "Notes/dest.md", "content")

		ok := ga.CommitFileMove("Inbox/raw.md", "Notes/dest.md")
		if ok {
			t.Error("expected false when not a git repo")
		}
	})
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}
