package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// UnifiedViewLine represents a single line in unified view
type UnifiedViewLine struct {
	Content         string
	LineNumber      string
	IsFoldIndicator bool // True if this is a fold indicator line (not a real diff line)
}

// UnifiedViewContent represents the content for unified view
type UnifiedViewContent struct {
	Lines []UnifiedViewLine
}

// FoldableRange represents a range of lines that can be folded
type FoldableRange struct {
	StartLine  int // File line number where fold starts
	EndLine    int // File line number where fold ends
	InsertAt   int // Display index where fold indicator should be inserted
	LineCount  int // Number of lines in the fold
}

// GetUnifiedViewLineCount returns the actual line count including fold indicators
func GetUnifiedViewLineCount(diffText string) int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap)
	return len(content.Lines)
}

// MapUnifiedDisplayToOriginalIdx maps unified view display indices (including fold indicators)
// to original diff line indices (excluding headers)
func MapUnifiedDisplayToOriginalIdx(diffText string) map[int]int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap)

	mapping := make(map[int]int)
	originalIdx := 0

	for displayIdx, line := range content.Lines {
		if !line.IsFoldIndicator {
			// This is a real diff line, map it to the original index
			mapping[displayIdx] = originalIdx
			originalIdx++
		}
		// Fold indicator lines are not mapped (they don't correspond to any original line)
	}

	return mapping
}

// detectFoldableRanges detects ranges that can be folded
func detectFoldableRanges(oldLineMap, newLineMap map[int]int, minGap int) []FoldableRange {
	// Use newLineMap primarily (additions/context), fall back to oldLineMap for deletions
	lineMap := newLineMap
	if len(lineMap) == 0 {
		lineMap = oldLineMap
	}
	if len(lineMap) == 0 {
		return nil
	}

	// Get sorted display indices
	var displayIndices []int
	for idx := range lineMap {
		displayIndices = append(displayIndices, idx)
	}

	// Simple bubble sort
	for i := 0; i < len(displayIndices)-1; i++ {
		for j := i + 1; j < len(displayIndices); j++ {
			if displayIndices[i] > displayIndices[j] {
				displayIndices[i], displayIndices[j] = displayIndices[j], displayIndices[i]
			}
		}
	}

	var ranges []FoldableRange

	// Check gaps between consecutive diff lines
	for i := 0; i < len(displayIndices)-1; i++ {
		currentDisplayIdx := displayIndices[i]
		nextDisplayIdx := displayIndices[i+1]

		currentLineNum := lineMap[currentDisplayIdx]
		nextLineNum := lineMap[nextDisplayIdx]

		gap := nextLineNum - currentLineNum - 1
		if gap >= minGap {
			ranges = append(ranges, FoldableRange{
				StartLine: currentLineNum + 1,
				EndLine:   nextLineNum - 1,
				InsertAt:  currentDisplayIdx, // Insert after current line
				LineCount: gap,
			})
		}
	}

	return ranges
}

// generateUnifiedViewContent generates content for unified view from diff text
func generateUnifiedViewContent(diffText string, oldLineMap, newLineMap map[int]int) *UnifiedViewContent {
	// First colorize the diff
	coloredLines := colorizeDiff(diffText)

	// Calculate max digits for line numbers
	maxDigits := calculateMaxLineNumberDigits(oldLineMap, newLineMap)

	// Detect foldable ranges (minimum 3 lines gap)
	foldableRanges := detectFoldableRanges(oldLineMap, newLineMap, 3)

	// Create a map for quick lookup
	foldMap := make(map[int]*FoldableRange)
	for i := range foldableRanges {
		foldMap[foldableRanges[i].InsertAt] = &foldableRanges[i]
	}

	// Build content with line numbers
	content := &UnifiedViewContent{
		Lines: []UnifiedViewLine{},
	}

	for i, line := range coloredLines {
		lineNum := generateLineNumber(line, i, maxDigits, oldLineMap, newLineMap)
		content.Lines = append(content.Lines, UnifiedViewLine{
			Content:         line,
			LineNumber:      lineNum,
			IsFoldIndicator: false,
		})

		// Check if we should insert a fold indicator after this line
		if fold, exists := foldMap[i]; exists {
			foldIndicator := fmt.Sprintf("[dimgray]... %d lines hidden (press 'e' to expand) ...[-]", fold.LineCount)
			content.Lines = append(content.Lines, UnifiedViewLine{
				Content:         foldIndicator,
				LineNumber:      strings.Repeat(" ", maxDigits) + " │ ",
				IsFoldIndicator: true,
			})
		}
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
