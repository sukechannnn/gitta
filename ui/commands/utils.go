package commands

import (
	"github.com/sukechannnn/giff/util"
)

// ColorizeDiff colorizes a diff string
func ColorizeDiff(diff string) string {
	var result string
	lines := util.SplitLines(diff)
	headers := util.DiffHeaderMask(lines)
	for i, line := range lines {
		// Filter out header lines to skip
		if headers[i] {
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
	headers := util.DiffHeaderMask(lines)

	for i := range lines {
		if headers[i] {
			continue
		}

		mapping[displayIndex] = i
		displayIndex++
	}

	return mapping
}
