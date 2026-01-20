package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// UnifiedViewLine represents a single line in unified view
type UnifiedViewLine struct {
	Content         string
	LineNumber      string
	IsFoldIndicator bool   // True if this is a fold indicator line (not a real diff line)
	FoldID          string // Fold identifier (empty if not a fold indicator)
}

// UnifiedViewContent represents the content for unified view
type UnifiedViewContent struct {
	Lines []UnifiedViewLine
}

// FoldableRange represents a range of lines that can be folded
type FoldableRange struct {
	StartLine  int    // File line number where fold starts
	EndLine    int    // File line number where fold ends
	InsertAt   int    // Display index where fold indicator should be inserted
	LineCount  int    // Number of lines in the fold
	ID         string // Unique identifier for this fold
}

// GetUnifiedViewLineCount returns the actual line count including fold indicators
func GetUnifiedViewLineCount(diffText string, foldState *FoldState, filePath, repoRoot string) int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, foldState, filePath, repoRoot)
	return len(content.Lines)
}

// MapUnifiedDisplayToOriginalIdx maps unified view display indices (including fold indicators)
// to original diff line indices (excluding headers)
func MapUnifiedDisplayToOriginalIdx(diffText string, foldState *FoldState, filePath, repoRoot string) map[int]int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, foldState, filePath, repoRoot)

	mapping := make(map[int]int)
	originalIdx := 0

	for displayIdx, line := range content.Lines {
		// Skip fold indicator lines and expanded fold content (both have FoldID set)
		if line.FoldID != "" {
			continue
		}
		// This is a real diff line, map it to the original index
		mapping[displayIdx] = originalIdx
		originalIdx++
	}

	return mapping
}

// detectFoldableRanges detects ranges that can be folded
// totalLines is the total number of lines in the file (0 if unknown, which disables top/bottom folds)
func detectFoldableRanges(oldLineMap, newLineMap map[int]int, minGap int, totalLines int) []FoldableRange {
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

	// Check for lines before the first diff line (top fold)
	if len(displayIndices) > 0 {
		firstDisplayIdx := displayIndices[0]
		firstLineNum := lineMap[firstDisplayIdx]
		if firstLineNum > 1 {
			topGap := firstLineNum - 1
			if topGap >= minGap {
				ranges = append(ranges, FoldableRange{
					StartLine: 1,
					EndLine:   firstLineNum - 1,
					InsertAt:  -1, // Special value: insert at the beginning
					LineCount: topGap,
					ID:        fmt.Sprintf("fold-top-1-%d", firstLineNum-1),
				})
			}
		}
	}

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
				ID:        fmt.Sprintf("fold-%d-%d", currentLineNum+1, nextLineNum-1),
			})
		}
	}

	// Check for lines after the last diff line (bottom fold)
	if totalLines > 0 && len(displayIndices) > 0 {
		lastDisplayIdx := displayIndices[len(displayIndices)-1]
		lastLineNum := lineMap[lastDisplayIdx]
		if lastLineNum < totalLines {
			bottomGap := totalLines - lastLineNum
			if bottomGap >= minGap {
				ranges = append(ranges, FoldableRange{
					StartLine: lastLineNum + 1,
					EndLine:   totalLines,
					InsertAt:  -2, // Special value: insert at the end
					LineCount: bottomGap,
					ID:        fmt.Sprintf("fold-bottom-%d-%d", lastLineNum+1, totalLines),
				})
			}
		}
	}

	return ranges
}

// generateUnifiedViewContent generates content for unified view from diff text
func generateUnifiedViewContent(diffText string, oldLineMap, newLineMap map[int]int, foldState *FoldState, filePath, repoRoot string) *UnifiedViewContent {
	// First colorize the diff
	coloredLines := colorizeDiff(diffText)

	// Calculate max digits for line numbers
	maxDigits := calculateMaxLineNumberDigits(oldLineMap, newLineMap)

	// Get total lines in file for top/bottom fold detection
	totalLines := getFileTotalLines(filePath, repoRoot)

	// Detect foldable ranges (minimum 3 lines gap)
	foldableRanges := detectFoldableRanges(oldLineMap, newLineMap, 3, totalLines)

	// Create maps for quick lookup
	foldMap := make(map[int]*FoldableRange)
	var topFold, bottomFold *FoldableRange
	for i := range foldableRanges {
		fold := &foldableRanges[i]
		if fold.InsertAt == -1 {
			topFold = fold
		} else if fold.InsertAt == -2 {
			bottomFold = fold
		} else {
			foldMap[fold.InsertAt] = fold
		}
	}

	// Build content with line numbers
	content := &UnifiedViewContent{
		Lines: []UnifiedViewLine{},
	}

	// Insert top fold indicator/content at the beginning
	if topFold != nil {
		appendFoldContent(content, topFold, foldState, filePath, repoRoot, maxDigits)
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
			appendFoldContent(content, fold, foldState, filePath, repoRoot, maxDigits)
		}
	}

	// Insert bottom fold indicator/content at the end
	if bottomFold != nil {
		appendFoldContent(content, bottomFold, foldState, filePath, repoRoot, maxDigits)
	}

	return content
}

// appendFoldContent appends fold indicator or expanded content to the unified view
func appendFoldContent(content *UnifiedViewContent, fold *FoldableRange, foldState *FoldState, filePath, repoRoot string, maxDigits int) {
	if foldState != nil && foldState.IsExpanded(fold.ID) {
		// Expanded: show actual file content
		expandedLines := readFileLines(filePath, repoRoot, fold.StartLine, fold.EndLine)
		for lineIdx, expandedLine := range expandedLines {
			actualLineNum := fold.StartLine + lineIdx
			lineNumStr := fmt.Sprintf("[dimgray]%*d │ [-]", maxDigits, actualLineNum)
			// Add leading space to align with +/- prefixed lines
			content.Lines = append(content.Lines, UnifiedViewLine{
				Content:         "[dimgray] " + tview.Escape(expandedLine) + "[-]",
				LineNumber:      lineNumStr,
				IsFoldIndicator: false,
				FoldID:          fold.ID,
			})
		}
	} else {
		// Collapsed: show fold indicator
		foldIndicator := fmt.Sprintf("[dimgray]... %d lines hidden (press 'e' to expand) ...[-]", fold.LineCount)
		content.Lines = append(content.Lines, UnifiedViewLine{
			Content:         foldIndicator,
			LineNumber:      strings.Repeat(" ", maxDigits) + " │ ",
			IsFoldIndicator: true,
			FoldID:          fold.ID,
		})
	}
}

// getFileTotalLines returns the total number of lines in a file
func getFileTotalLines(filePath, repoRoot string) int {
	if filePath == "" || repoRoot == "" {
		return 0
	}
	fullPath := filepath.Join(repoRoot, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return 0
	}
	lines := strings.Split(string(content), "\n")
	return len(lines)
}

// GetFoldIDAtLine returns the fold ID at the given display line index, or empty string if none
func GetFoldIDAtLine(diffText string, lineIndex int, foldState *FoldState, filePath, repoRoot string) string {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, foldState, filePath, repoRoot)

	if lineIndex >= 0 && lineIndex < len(content.Lines) {
		return content.Lines[lineIndex].FoldID
	}
	return ""
}

// GetFoldIndicatorPosition returns the display line index of a fold indicator after toggling
// This is used to move cursor to the fold indicator when collapsing
func GetFoldIndicatorPosition(diffText string, foldID string, foldState *FoldState, filePath, repoRoot string) int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, foldState, filePath, repoRoot)

	for i, line := range content.Lines {
		if line.FoldID == foldID {
			return i
		}
	}
	return 0
}

// readFileLines reads lines from a file within the specified range (1-indexed, inclusive)
func readFileLines(filePath, repoRoot string, startLine, endLine int) []string {
	fullPath := filepath.Join(repoRoot, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return []string{fmt.Sprintf("Error reading file: %v", err)}
	}

	lines := strings.Split(string(content), "\n")

	// Convert to 0-indexed
	start := startLine - 1
	end := endLine // endLine is inclusive, so we use it as the slice end

	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start >= len(lines) {
		return []string{}
	}

	return lines[start:end]
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
