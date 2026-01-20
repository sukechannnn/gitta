package git

import (
	"fmt"
	"os/exec"
)

func GetFileDiff(filePath string, repoRoot string) (string, error) {
	return GetFileDiffWithOptions(filePath, repoRoot, false)
}

func GetFileDiffWithOptions(filePath string, repoRoot string, ignoreWhitespace bool) (string, error) {
	// `git diff` を実行（削除されたファイルでも動作するように -- を追加）
	// -c core.quotepath=false でマルチバイトファイル名をエスケープしないようにする
	args := []string{"-c", "core.quotepath=false", "diff"}
	if ignoreWhitespace {
		args = append(args, "-w")
	}
	args = append(args, "--", filePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff: %w", err)
	}

	// 差分を表示
	return string(output), nil
}
