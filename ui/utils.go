package ui

import (
	"fmt"
)

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
