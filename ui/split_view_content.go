package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// SplitViewContent represents the content for split view
type SplitViewContent struct {
	BeforeLines    []string
	AfterLines     []string
	BeforeLineNums []string
	AfterLineNums  []string
}

// generateSplitViewContent generates content for split view from diff text
func generateSplitViewContent(diffText string, oldLineMap, newLineMap map[int]int) *SplitViewContent {
	lines := strings.Split(diffText, "\n")
	content := &SplitViewContent{
		BeforeLines:    []string{},
		AfterLines:     []string{},
		BeforeLineNums: []string{},
		AfterLineNums:  []string{},
	}

	var inHunk bool = false
	displayLine := 0

	// 行番号の最大桁数を計算
	maxDigits := calculateMaxLineNumberDigits(oldLineMap, newLineMap)

	for _, line := range lines {
		// ヘッダー行を非表示にする
		if isHeaderLine(line) {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			// ハンクヘッダー（非表示にする）
			inHunk = true
			continue
		} else if inHunk {
			processHunkLine(line, displayLine, maxDigits, oldLineMap, newLineMap, content)
			displayLine++
		}
	}

	return content
}

// isHeaderLine checks if the line is a header line that should be hidden
func isHeaderLine(line string) bool {
	return strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ")
}

// processHunkLine processes a single line within a hunk
func processHunkLine(line string, displayLine, maxDigits int, oldLineMap, newLineMap map[int]int, content *SplitViewContent) {
	if strings.HasPrefix(line, "-") {
		// 削除行（左側のみに表示、- 記号を含める）
		// Escape the line content to prevent tview from interpreting brackets as color tags
		content.BeforeLines = append(content.BeforeLines, "[red]"+tview.Escape(line)+"[-]")
		content.AfterLines = append(content.AfterLines, "[dimgray] [-]") // 右側には左側の行数と合わせるためのスペースを表示

		// 左側に実際の行番号、右側は空
		if num, ok := oldLineMap[displayLine]; ok {
			content.BeforeLineNums = append(content.BeforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
		}
		content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
	} else if strings.HasPrefix(line, "+") {
		// 追加行（右側のみに表示、+ 記号を含める）
		content.BeforeLines = append(content.BeforeLines, "[dimgray] [-]") // 左側には右側の行数と合わせるためのスペースを表示
		// Escape the line content to prevent tview from interpreting brackets as color tags
		content.AfterLines = append(content.AfterLines, "[green]"+tview.Escape(line)+"[-]")

		// 左側は空、右側に実際の行番号
		content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
		if num, ok := newLineMap[displayLine]; ok {
			content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
		}
	} else if strings.HasPrefix(line, " ") {
		// 変更なし行（両側に表示、先頭のスペースを保持）
		// Escape the line content to prevent tview from interpreting brackets as color tags
		escapedLine := tview.Escape(line[1:])
		content.BeforeLines = append(content.BeforeLines, " "+escapedLine)
		content.AfterLines = append(content.AfterLines, " "+escapedLine)

		// 両側に実際の行番号
		if num, ok := oldLineMap[displayLine]; ok {
			content.BeforeLineNums = append(content.BeforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
		}
		if num, ok := newLineMap[displayLine]; ok {
			content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
		}
	} else {
		// その他の行
		// Escape the line content to prevent tview from interpreting brackets as color tags
		escapedLine := tview.Escape(line)
		content.BeforeLines = append(content.BeforeLines, " "+escapedLine)
		content.AfterLines = append(content.AfterLines, " "+escapedLine)

		// 両側に実際の行番号
		if num, ok := oldLineMap[displayLine]; ok {
			content.BeforeLineNums = append(content.BeforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
		}
		if num, ok := newLineMap[displayLine]; ok {
			content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, num))
		} else {
			content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
		}
	}
}
