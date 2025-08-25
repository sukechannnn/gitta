package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// UnifiedViewLine represents a single line in unified view
type UnifiedViewLine struct {
	Content    string
	LineNumber string
}

// UnifiedViewContent represents the content for unified view
type UnifiedViewContent struct {
	Lines []UnifiedViewLine
}

// generateUnifiedViewContent generates content for unified view from diff text
func generateUnifiedViewContent(diffText string, oldLineMap, newLineMap map[int]int) *UnifiedViewContent {
	// First colorize the diff
	coloredLines := colorizeDiff(diffText)

	// Calculate max digits for line numbers
	maxDigits := calculateMaxLineNumberDigits(oldLineMap, newLineMap)

	// Build content with line numbers
	content := &UnifiedViewContent{
		Lines: []UnifiedViewLine{},
	}

	for i, line := range coloredLines {
		lineNum := generateLineNumber(line, i, maxDigits, oldLineMap, newLineMap)
		content.Lines = append(content.Lines, UnifiedViewLine{
			Content:    line,
			LineNumber: lineNum,
		})
	}

	return content
}

// colorizeDiff colorizes diff text and filters out headers
func colorizeDiff(diff string) []string {
	var result []string
	lines := util.SplitLines(diff)

	for _, line := range lines {
		// Skip header lines
		if isUnifiedHeaderLine(line) {
			continue
		}

		// Colorize the line
		coloredLine := colorizeLine(line)
		result = append(result, coloredLine)
	}

	return result
}

// ColorizeDiff is a public wrapper for backward compatibility
// This will be deprecated once all callers are updated
func ColorizeDiff(diff string) string {
	lines := colorizeDiff(diff)
	return strings.Join(lines, "\n") + "\n"
}

// isUnifiedHeaderLine checks if the line is a header that should be skipped
func isUnifiedHeaderLine(line string) bool {
	return strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "--- ") ||
		strings.HasPrefix(line, "+++ ") ||
		strings.HasPrefix(line, "@@")
}

// colorizeLine adds color tags to a single line based on its type
func colorizeLine(line string) string {
	if len(line) > 0 {
		switch line[0] {
		case '-':
			// Escape the line content to prevent tview from interpreting brackets as color tags
			return "[red]" + tview.Escape(line) + "[-]"
		case '+':
			// Escape the line content to prevent tview from interpreting brackets as color tags
			return "[green]" + tview.Escape(line) + "[-]"
		case ' ':
			return tview.Escape(line)
		default:
			return tview.Escape(line)
		}
	}
	return ""
}

// generateLineNumber generates the line number string for a given line
func generateLineNumber(line string, index int, maxDigits int, oldLineMap, newLineMap map[int]int) string {
	var lineNum string

	if strings.HasPrefix(line, "[red]") || (len(line) > 0 && line[0] == '-') {
		// Deletion line
		if num, ok := oldLineMap[index]; ok {
			lineNum = fmt.Sprintf("%*d", maxDigits, num)
		} else {
			lineNum = strings.Repeat(" ", maxDigits)
		}
		lineNum += " │ "
	} else if strings.HasPrefix(line, "[green]") || (len(line) > 0 && line[0] == '+') {
		// Addition line
		if num, ok := newLineMap[index]; ok {
			lineNum = fmt.Sprintf("%*d", maxDigits, num)
		} else {
			lineNum = strings.Repeat(" ", maxDigits)
		}
		lineNum += " │ "
	} else {
		// Common line
		if num, ok := newLineMap[index]; ok {
			lineNum = fmt.Sprintf("%*d", maxDigits, num)
		} else if num, ok := oldLineMap[index]; ok {
			lineNum = fmt.Sprintf("%*d", maxDigits, num)
		} else {
			lineNum = strings.Repeat(" ", maxDigits)
		}
		lineNum += " │ "
	}

	return lineNum
}
