package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupPRRepo makes a repo whose origin/main has moved on since the feature
// branch forked off it, with the feature branch checked out
func setupPRRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	write("a.txt", "a\n")
	run("add", ".")
	run("commit", "-m", "initial")

	run("checkout", "-b", "feature")
	write("feature.txt", "feature\n")
	run("add", ".")
	run("commit", "-m", "feature")

	run("checkout", "main")
	write("a.txt", "a changed on main\n")
	run("commit", "-am", "main moves on")
	run("update-ref", "refs/remotes/origin/main", "main")
	run("checkout", "feature")
	return repo
}

func TestPRBaseRefFallsBackToOriginMain(t *testing.T) {
	repo := setupPRRepo(t)
	base, err := PRBaseRef(repo)
	if err != nil {
		t.Fatal(err)
	}
	if base != "origin/main" {
		t.Errorf("PRBaseRef() = %q, want origin/main", base)
	}
}

func TestGetPRChangedFilesOnlyHasTheBranchChanges(t *testing.T) {
	repo := setupPRRepo(t)
	output, err := GetPRChangedFiles("origin/main", repo)
	if err != nil {
		t.Fatal(err)
	}
	// a.txt changed only on main after the fork, so it is not part of the PR
	if got := strings.TrimSpace(output); got != "A\tfeature.txt" {
		t.Errorf("GetPRChangedFiles() = %q, want %q", got, "A\tfeature.txt")
	}
}
