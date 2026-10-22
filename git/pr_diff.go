package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// PRBaseRef returns the ref the current branch's pull request is compared
// against: the PR's base branch when the GitHub CLI knows the PR, otherwise
// the remote's default branch.
func PRBaseRef(repoRoot string) (string, error) {
	if base := ghPRBaseBranch(repoRoot); base != "" {
		for _, ref := range []string{"origin/" + base, base} {
			if refExists(ref, repoRoot) {
				return ref, nil
			}
		}
	}

	cmd := exec.Command("git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	cmd.Dir = repoRoot
	if output, err := cmd.Output(); err == nil {
		if ref := strings.TrimSpace(string(output)); ref != "" {
			return ref, nil
		}
	}

	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		if refExists(ref, repoRoot) {
			return ref, nil
		}
	}
	return "", errors.New("could not find the base branch (no origin/HEAD, main or master)")
}

// ghPRBaseBranch returns the base branch of the current branch's pull request,
// or "" when the GitHub CLI is missing or there is no pull request
func ghPRBaseBranch(repoRoot string) string {
	cmd := exec.Command("gh", "pr", "view", "--json", "baseRefName", "--jq", ".baseRefName")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func refExists(ref string, repoRoot string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

// PRDiffArgs returns the git arguments printing the pull request diff, the
// changes from where the branch forked off base up to HEAD (as GitHub shows
// them), followed by extra arguments
func PRDiffArgs(base string, extra ...string) []string {
	args := []string{"-c", "core.quotepath=false", "diff", base + "...HEAD"}
	return append(args, extra...)
}

// GetPRChangedFiles returns `git diff --name-status` output for the pull request diff
func GetPRChangedFiles(base string, repoRoot string) (string, error) {
	cmd := exec.Command("git", PRDiffArgs(base, "--name-status")...)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff %s...HEAD: %w", base, err)
	}
	return string(output), nil
}
