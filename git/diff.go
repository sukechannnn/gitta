package git

import (
	"fmt"
	"log"
	"os/exec"
)

func GetFileDiff(repoPath, filePath string) (string, error) {
	// `git diff` を実行
	cmd := exec.Command("git", "diff", filePath)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to execute git diff: %v", err)
	}

	// 差分を表示
	return fmt.Sprint(string(output)), err
}
