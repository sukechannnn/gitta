package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SplitLines splits a string by newline characters
func SplitLines(input string) []string {
	lines := []string{}
	currentLine := ""
	for _, r := range input {
		if r == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(r)
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

// PatchLine represents a line in a patch with its original index
type PatchLine struct {
	Line     string
	Original int
}

// GenerateMinimalPatch generates a minimal patch for the selected lines
func GenerateMinimalPatch(diffText string, selectStart, selectEnd int, fileHeader string, updateDebug func(message string)) string {
	lines, start := ExtractSelectedLinesWithContext(diffText, selectStart, selectEnd)
	if len(lines) == 0 || start == -1 {
		return ""
	}

	allLines := SplitLines(diffText)
	startLine := FindHunkStartLineInFile(allLines, start)
	if startLine == -1 {
		if updateDebug != nil {
			updateDebug("Could not find hunk header for selected lines")
		}
		return ""
	}

	header := GenerateFullHunkHeader(startLine, lines)

	var body strings.Builder
	for _, pl := range lines {
		body.WriteString(pl.Line + "\n")
	}

	return fileHeader + "\n" + header + "\n" + body.String()
}

// ExtractSelectedLinesWithContext extracts selected lines with up to 3 context lines
func ExtractSelectedLinesWithContext(diff string, selectStart, selectEnd int) ([]PatchLine, int) {
	lines := SplitLines(diff)
	var result []PatchLine
	firstLine := -1

	seen := make(map[int]bool)

	// Context lines above (max 3)
	contextLines := 3
	count := 0
	for i := selectStart - 1; i >= 0 && count < contextLines; i-- {
		if strings.HasPrefix(lines[i], " ") || lines[i] == "" {
			result = append([]PatchLine{{Line: lines[i], Original: i}}, result...)
			seen[i] = true
			firstLine = i
			count++
		} else if strings.HasPrefix(lines[i], "@@") || strings.HasPrefix(lines[i], "diff --git") {
			break
		}
	}

	// Selected lines
	for i := selectStart; i <= selectEnd && i < len(lines); i++ {
		result = append(result, PatchLine{Line: lines[i], Original: i})
		seen[i] = true
		if firstLine == -1 {
			firstLine = i
		}
	}

	// Context lines below (max 3)
	count = 0
	for i := selectEnd + 1; i < len(lines) && count < contextLines; i++ {
		if strings.HasPrefix(lines[i], " ") || lines[i] == "" {
			if seen[i] {
				continue
			}
			result = append(result, PatchLine{Line: lines[i], Original: i})
			count++
		} else if strings.HasPrefix(lines[i], "@@") || strings.HasPrefix(lines[i], "diff --git") {
			break
		}
	}

	return result, firstLine
}

// GenerateFullHunkHeader generates a hunk header for the patch
func GenerateFullHunkHeader(startLine int, selected []PatchLine) string {
	delCount := 0
	addCount := 0

	for _, pl := range selected {
		switch {
		case strings.HasPrefix(pl.Line, "-") && !strings.HasPrefix(pl.Line, "---"):
			delCount++
		case strings.HasPrefix(pl.Line, "+") && !strings.HasPrefix(pl.Line, "+++"):
			addCount++
		case strings.HasPrefix(pl.Line, " ") || pl.Line == "":
			delCount++
			addCount++
		}
	}

	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", startLine, delCount, startLine, addCount)
}

// FindHunkStartLineInFile finds the start line number from hunk header
func FindHunkStartLineInFile(diffLines []string, targetIndex int) int {
	hunkRegex := regexp.MustCompile(`@@ -(\d+),\d+ \+\d+,\d+ @@`)

	for i := targetIndex; i >= 0; i-- {
		if strings.HasPrefix(diffLines[i], "@@") {
			match := hunkRegex.FindStringSubmatch(diffLines[i])
			if len(match) == 2 {
				if line, err := strconv.Atoi(match[1]); err == nil {
					return line
				}
			}
			break
		}
	}
	return -1
}