package ui

import (
	"strings"

	"github.com/sukechannnn/gitta/util"
)

// colorizeDiff は Diff を色付けします
func ColorizeDiff(diff string) string {
	var result string
	lines := util.SplitLines(diff)
	for _, line := range lines {
		// 🎯 ここでスキップしたいヘッダー行を除外
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue // ← 表示しない
		}

		// 色付け処理（+/-）
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
