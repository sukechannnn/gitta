package git

import (
	"fmt"
	"os/exec"
)

func Commit(message string, repoRoot string) error {
	// Execute git commit command
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %s", string(output))
	}
	return nil
}

func CommitAmend(message string, repoRoot string) error {
	// Execute git commit --amend command
	cmd := exec.Command("git", "commit", "--amend", "-m", message)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to amend commit: %s", string(output))
	}
	return nil
}
