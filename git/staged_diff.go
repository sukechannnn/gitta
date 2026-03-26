package git

import (
	"fmt"
	"os/exec"
)

func GetStagedDiff(filePath string, repoRoot string) (string, error) {
	return GetStagedDiffWithOptions(filePath, repoRoot, false)
}

func GetStagedDiffWithOptions(filePath string, repoRoot string, ignoreWhitespace bool) (string, error) {
	// Run `git diff --cached` (add -- so it works even for deleted files)
	// -c core.quotepath=false prevents escaping of multibyte filenames
	args := []string{"-c", "core.quotepath=false", "diff", "--cached"}
	if ignoreWhitespace {
		args = append(args, "-w")
	}
	args = append(args, "--", filePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff --cached: %w", err)
	}

	// Return the diff
	return string(output), nil
}
