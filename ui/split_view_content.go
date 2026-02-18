package ui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// SplitViewContent represents the content for split view
type SplitViewContent struct {
	BeforeLines    []string
	AfterLines     []string
	BeforeLineNums []string
	AfterLineNums  []string
}

// generateSplitViewContent generates content for split view from diff text
func generateSplitViewContent(diffText string, oldLineMap, newLineMap map[int]int, filePath string) *SplitViewContent {
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

	// Collect code lines (without prefix) for tokenization
	var codeLines []string
	for _, dl := range diffLines {
		if len(dl.content) > 0 && (dl.content[0] == '-' || dl.content[0] == '+' || dl.content[0] == ' ') {
			codeLines = append(codeLines, dl.content[1:])
		} else {
			codeLines = append(codeLines, dl.content)
		}
	}

	// Tokenize
	var allTokens [][]chroma.Token
	if filePath != "" {
		allTokens = util.TokenizeCode(filePath, codeLines)
	}

	// Helper: render a code line with syntax highlighting or fallback
	renderLine := func(idx int, prefix byte, bgColor string, fallbackColor string) string {
		if allTokens != nil && len(allTokens[idx]) > 0 {
			highlighted := util.RenderHighlightedLine(allTokens[idx], bgColor)
			if bgColor != "" {
				return "[:" + bgColor + "]" + tview.Escape(string(prefix)) + "[-:-]" + highlighted
			}
			return tview.Escape(string(prefix)) + highlighted
		}
		// Fallback
		escaped := tview.Escape(diffLines[idx].content)
		if fallbackColor != "" {
			return "[" + fallbackColor + "]" + escaped + "[-]"
		}
		return escaped
	}

	// ペアリング処理：連続する-と+のグループをペアリング
	i := 0
	codeIdx := 0 // tracks index into codeLines/allTokens
	for i < len(diffLines) {
		line := diffLines[i]

		switch line.lineType {
		case "-":
			// 連続する-行を収集
			startIdx := codeIdx
			deletions := []diffLine{line}
			j := i + 1
			codeIdx++
			for j < len(diffLines) && diffLines[j].lineType == "-" {
				deletions = append(deletions, diffLines[j])
				j++
				codeIdx++
			}

			// 連続する+行を収集
			addStartIdx := codeIdx
			additions := []diffLine{}
			for j < len(diffLines) && diffLines[j].lineType == "+" {
				additions = append(additions, diffLines[j])
				j++
				codeIdx++
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
					beforeLine = renderLine(startIdx+k, '-', util.DeletedLineBg, "red")
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
					afterLine = renderLine(addStartIdx+k, '+', util.AddedLineBg, "green")
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
		case "+":
			// ペアになっていない+行（-なしで+のみ）
			content.BeforeLines = append(content.BeforeLines, "[dimgray] [-]")
			content.AfterLines = append(content.AfterLines, renderLine(codeIdx, '+', util.AddedLineBg, "green"))

			content.BeforeLineNums = append(content.BeforeLineNums, strings.Repeat(" ", maxDigits))
			if line.newLineNum >= 0 {
				content.AfterLineNums = append(content.AfterLineNums, fmt.Sprintf("%*d", maxDigits, line.newLineNum))
			} else {
				content.AfterLineNums = append(content.AfterLineNums, strings.Repeat(" ", maxDigits))
			}
			i++
			codeIdx++
		case " ":
			// 変更なし行
			contextLine := renderLine(codeIdx, ' ', "", "")
			content.BeforeLines = append(content.BeforeLines, contextLine)
			content.AfterLines = append(content.AfterLines, contextLine)

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
			codeIdx++
		default:
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
			codeIdx++
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
