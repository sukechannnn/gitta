package git

import (
	"fmt"
	"os/exec"
)

func Commit(message string, repoRoot string) error {
	// git commit コマンドを実行
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %s", string(output))
	}
	return nil
}
