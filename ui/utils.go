package ui

import (
	"fmt"
	"os"
	"path/filepath"
)

// missingFromWorkTree reports whether a file shown by a read-only view (a
// commit or the pull request) is absent from the working tree, where an
// editor would open an empty buffer instead.
func missingFromWorkTree(repoRoot, path string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, path))
	return err != nil
}

// calculateMaxLineNumberDigits calculates the maximum number of digits needed for line numbers
// across both old and new line maps
func calculateMaxLineNumberDigits(oldLineMap, newLineMap map[int]int) int {
	maxOldLine := 0
	maxNewLine := 0

	for _, lineNum := range oldLineMap {
		if lineNum > maxOldLine {
			maxOldLine = lineNum
		}
	}
	for _, lineNum := range newLineMap {
		if lineNum > maxNewLine {
			maxNewLine = lineNum
		}
	}

	maxDigits := len(fmt.Sprintf("%d", maxNewLine))
	if len(fmt.Sprintf("%d", maxOldLine)) > maxDigits {
		maxDigits = len(fmt.Sprintf("%d", maxOldLine))
	}

	return maxDigits
}
