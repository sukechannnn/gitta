package commands

import (
	"strings"

	"github.com/sukechannnn/giff/util"
)

// ColorizeDiff colorizes a diff string
func ColorizeDiff(diff string) string {
	var result string
	lines := util.SplitLines(diff)
	for _, line := range lines {
		// Filter out header lines to skip
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue // Skip display
		}

		// Colorize lines (+/-)
		if len(line) > 0 {
			switch line[0] {
			case '-':
				result += "[red]" + line + "[-]\n"
			case '+':
				result += "[green]" + line + "[-]\n"
			default:
				result += line + "\n"
			}
		} else {
			result += "\n"
		}
	}
	return result
}

// MapDisplayToOriginalIdx maps display indices to original diff line indices
func MapDisplayToOriginalIdx(diff string) map[int]int {
	lines := util.SplitLines(diff)
	displayIndex := 0
	mapping := make(map[int]int)

	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}

		mapping[displayIndex] = i
		displayIndex++
	}

	return mapping
}
