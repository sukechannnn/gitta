package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ApplySelectedChangesToFile returns file content with only selected changes applied
// It preserves existing staged changes and applies new selections on top
func ApplySelectedChangesToFile(filePath string, repoRoot string, diffText string, selectedStart, selectedEnd int) (string, string, error) {
	// Read current file
	currentContent, err := os.ReadFile(filepath.Join(repoRoot, filePath))
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %w", err)
	}

	// Get the staged version (if any)
	stagedContent, err := GetFileContentFromIndex(filePath, repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("failed to get staged version: %w", err)
	}

	// Parse diff to understand changes
	diffLines := strings.Split(diffText, "\n")

	// Apply only selected changes on top of staged content
	result := applySelectedDiffLines(string(stagedContent), diffLines, selectedStart, selectedEnd)

	return result, string(currentContent), nil
}

// GetFileContentFromIndex returns file content from git index (staging area)
func GetFileContentFromIndex(filePath string, repoRoot string) ([]byte, error) {
	// First, check if there's a staged version
	cmd := exec.Command("git", "-c", "core.quotepath=false", "show", ":"+filePath)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		// No staged version, try HEAD
		return GetFileContentFromHEAD(filePath, repoRoot)
	}
	return output, nil
}

// GetFileContentFromHEAD returns file content at HEAD
func GetFileContentFromHEAD(filePath string, repoRoot string) ([]byte, error) {
	// Try to read the file at HEAD
	cmd := exec.Command("git", "-c", "core.quotepath=false", "show", "HEAD:"+filePath)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		// File might be new, return empty
		return []byte{}, nil
	}
	return output, nil
}

// applySelectedDiffLines applies selected diff lines to base content
func applySelectedDiffLines(baseContent string, diffLines []string, selectedStart, selectedEnd int) string {
	baseLines := strings.Split(baseContent, "\n")
	result := make([]string, 0)

	baseIdx := 0
	inHunk := false
	var currentLine int

	for i, line := range diffLines {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			// Parse line number from hunk header
			var oldStart int
			fmt.Sscanf(line, "@@ -%d", &oldStart)
			currentLine = oldStart - 1

			// Add any skipped lines
			for baseIdx < currentLine && baseIdx < len(baseLines) {
				result = append(result, baseLines[baseIdx])
				baseIdx++
			}
			continue
		}

		if !inHunk {
			continue
		}

		isSelected := i >= selectedStart && i <= selectedEnd

		if strings.HasPrefix(line, "-") {
			// Deletion
			if isSelected {
				// Skip this line (apply deletion)
				baseIdx++
			} else {
				// Keep this line (don't apply deletion)
				if baseIdx < len(baseLines) {
					result = append(result, baseLines[baseIdx])
					baseIdx++
				}
			}
		} else if strings.HasPrefix(line, "+") {
			// Addition
			if isSelected {
				// Add this line
				result = append(result, line[1:])
			}
			// Don't increment baseIdx for additions
		} else if !strings.HasPrefix(line, "\\") {
			// Context line
			if baseIdx < len(baseLines) {
				result = append(result, baseLines[baseIdx])
				baseIdx++
			}
		}
	}

	// Add any remaining lines
	for baseIdx < len(baseLines) {
		result = append(result, baseLines[baseIdx])
		baseIdx++
	}

	return strings.Join(result, "\n")
}
