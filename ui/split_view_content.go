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

	// 削除行と追加行をペアリングするための前処理
	type diffLine struct {
		content      string
		displayIndex int
		oldLineNum   int
		newLineNum   int
		lineType     string // "-", "+", " ", "other"
	}

	var diffLines []diffLine

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
			lineType := "other"
			if strings.HasPrefix(line, "-") {
				lineType = "-"
			} else if strings.HasPrefix(line, "+") {
				lineType = "+"
			} else if strings.HasPrefix(line, " ") {
				lineType = " "
			}

			oldNum := -1
			newNum := -1
			if num, ok := oldLineMap[displayLine]; ok {
				oldNum = num
			}
			if num, ok := newLineMap[displayLine]; ok {
				newNum = num
			}

			diffLines = append(diffLines, diffLine{
				content:      line,
				displayIndex: displayLine,
				oldLineNum:   oldNum,
				newLineNum:   newNum,
				lineType:     lineType,
			})
			displayLine++
		}
	}

	// ペアリング処理：連続する-と+のグループをペアリング
	i := 0
	for i < len(diffLines) {
		line := diffLines[i]

		if line.lineType == "-" {
			// 連続する-行を収集
			deletions := []diffLine{line}
			j := i + 1
			for j < len(diffLines) && diffLines[j].lineType == "-" {
				deletions = append(deletions, diffLines[j])
				j++
			}

			// 連続する+行を収集
			additions := []diffLine{}
			for j < len(diffLines) && diffLines[j].lineType == "+" {
				additions = append(additions, diffLines[j])
				j++
			}

			// ペアリング：削除と追加をペアにする
			maxLen := len(deletions)
			if len(additions) > maxLen {
				maxLen = len(additions)
			}

			for k := 0; k < maxLen; k++ {
				beforeLine := ""
				beforeLineNum := ""
				afterLine := ""
				afterLineNum := ""

				if k < len(deletions) {
					beforeLine = "[red]" + tview.Escape(deletions[k].content) + "[-]"
					if deletions[k].oldLineNum >= 0 {
						beforeLineNum = fmt.Sprintf("%*d", maxDigits, deletions[k].oldLineNum)
					} else {
						beforeLineNum = strings.Repeat(" ", maxDigits)
					}
				} else {
					beforeLine = "[dimgray] [-]"
					beforeLineNum = strings.Repeat(" ", maxDigits)
				}

				if k < len(additions) {
					afterLine = "[green]" + tview.Escape(additions[k].content) + "[-]"
					if additions[k].newLineNum >= 0 {
						afterLineNum = fmt.Sprintf("%*d", maxDigits, additions[k].newLineNum)
					} else {
						afterLineNum = strings.Repeat(" ", maxDigits)
					}
				} else {
					afterLine = "[dimgray] [-]"
					afterLineNum = strings.Repeat(" ", maxDigits)
				}

				content.BeforeLines = append(content.BeforeLines, beforeLine)
				content.AfterLines = append(content.AfterLines, afterLine)
				content.BeforeLineNums = append(content.BeforeLineNums, beforeLineNum)
				content.AfterLineNums = append(content.AfterLineNums, afterLineNum)
			}

			i = j
		} else if line.lineType == "+" {
			// ペアになっていない+行（-なしで+のみ）
			content.BeforeLines = append(content.BeforeLines, "[dimgray] [-]")
			content.AfterLines = append(content.AfterLines, "[green]"+tview.Escape(line.content)+"[-]")

			content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
			if line.newLineNum >= 0 {
				content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, line.newLineNum))
			} else {
				content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
			}
			i++
		} else if line.lineType == " " {
			// 変更なし行
			escapedLine := tview.Escape(line.content[1:])
			content.BeforeLines = append(content.BeforeLines, " "+escapedLine)
			content.AfterLines = append(content.AfterLines, " "+escapedLine)

			if line.oldLineNum >= 0 {
				content.BeforeLineNums = append(content.BeforeLineNums, fmt.Sprintf("%*d", maxDigits, line.oldLineNum))
			} else {
				content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
			}
			if line.newLineNum >= 0 {
				content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, line.newLineNum))
			} else {
				content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
			}
			i++
		} else {
			// その他の行
			escapedLine := tview.Escape(line.content)
			content.BeforeLines = append(content.BeforeLines, " "+escapedLine)
			content.AfterLines = append(content.AfterLines, " "+escapedLine)

			if line.oldLineNum >= 0 {
				content.BeforeLineNums = append(content.BeforeLineNums, fmt.Sprintf("%*d", maxDigits, line.oldLineNum))
			} else {
				content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
			}
			if line.newLineNum >= 0 {
				content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, line.newLineNum))
			} else {
				content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
			}
			i++
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
