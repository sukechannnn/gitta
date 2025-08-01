package git

import (
	"fmt"
	"os/exec"
)

func GetStagedDiff(filePath string, repoRoot string) (string, error) {
	// `git diff` を実行（削除されたファイルでも動作するように -- を追加）
	cmd := exec.Command("git", "diff", "--cached", "--", filePath)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff --cached: %w", err)
	}

	// 差分を表示
	return string(output), nil
}
