package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sukechannnn/giff/git"
	"github.com/sukechannnn/giff/util"
)

// CommandAParams contains parameters for commandA function
type CommandAParams struct {
	SelectStart        int
	SelectEnd          int
	CurrentFile        string
	CurrentStatus      string
	CurrentDiffText    string
	RepoRoot           string
	UpdateGlobalStatus func(string, string)
	IsSplitView        bool // Whether the view is split view or unified view
}

// CommandAResult contains the results from commandA execution
type CommandAResult struct {
	Success      bool
	NewDiffText  string
	ColoredDiff  string
	DiffLines    []string
	ShouldUpdate bool
	NewCursorPos int // Recommended cursor position after staging
}

// CommandA handles the 'a' command for staging/unstaging selected lines
func CommandA(params CommandAParams) (*CommandAResult, error) {
	// Validate inputs
	if params.SelectStart == -1 || params.SelectEnd == -1 || params.CurrentFile == "" || params.CurrentDiffText == "" {
		return nil, nil
	}

	// Unstage if the file is staged
	if params.CurrentStatus == "staged" {
		return commandAUnstage(params)
	}

	// Build file path
	filePath := filepath.Join(params.RepoRoot, params.CurrentFile)

	// Save current file content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to read file", "tomato")
		}
		return nil, err
	}

	// Get file content with only selected changes applied
	mapping := MapDisplayToOriginalIdx(params.CurrentDiffText)
	start := mapping[params.SelectStart]
	end := mapping[params.SelectEnd]

	// Check if the selection contains actual changes
	// Create display lines (excluding headers)
	coloredDiff := ColorizeDiff(params.CurrentDiffText)
	displayLines := util.SplitLines(coloredDiff)

	hasChanges := false
	diffLines := strings.Split(params.CurrentDiffText, "\n")

	for i := params.SelectStart; i <= params.SelectEnd && i < len(displayLines); i++ {
		// Check original diff line directly (before colorization)
		if originalIdx, ok := mapping[i]; ok {
			if originalIdx < len(diffLines) {
				originalLine := diffLines[originalIdx]
				if strings.HasPrefix(originalLine, "+") || strings.HasPrefix(originalLine, "-") {
					hasChanges = true
					break
				}
			}
		}
	}

	// Early return if no changes are included
	if !hasChanges {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("No changes were staged", "yellow")
		}
		// Treat as success but with no changes
		result := &CommandAResult{
			Success:      true,
			NewDiffText:  params.CurrentDiffText,
			ColoredDiff:  coloredDiff,
			DiffLines:    displayLines,
			ShouldUpdate: false,
			NewCursorPos: params.SelectStart, // Keep original position when there are no changes
		}
		return result, nil
	}

	modifiedContent, _, err := git.ApplySelectedChangesToFile(params.CurrentFile, params.RepoRoot, params.CurrentDiffText, start, end)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to process changes", "tomato")
		}
		return nil, err
	}

	// Temporarily overwrite file with only selected changes
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to write file", "tomato")
		}
		return nil, err
	}

	// Run git add
	// -c core.quotepath=false prevents escaping of multibyte filenames
	cmd := exec.Command("git", "-c", "core.quotepath=false", "add", params.CurrentFile)
	cmd.Dir = params.RepoRoot
	_, gitErr := cmd.CombinedOutput()

	// Restore original file content (including unselected changes)
	restoreErr := os.WriteFile(filePath, originalContent, 0644)

	if gitErr != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to stage changes", "tomato")
			if restoreErr != nil {
				params.UpdateGlobalStatus("Critical: Failed to restore file!", "tomato")
			}
		}
		return nil, gitErr
	}

	if restoreErr != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Warning: Failed to restore unstaged changes", "yellow")
		}
		return nil, restoreErr
	}

	// Re-fetch the diff
	newDiffText, _ := git.GetFileDiff(params.CurrentFile, params.RepoRoot)

	// Handle success
	if params.UpdateGlobalStatus != nil {
		params.UpdateGlobalStatus("Selected lines staged successfully!", "forestgreen")
	}

	// Colorize the diff
	newColoredDiff := ColorizeDiff(newDiffText)
	newDiffLines := util.SplitLines(newColoredDiff)

	// Calculate recommended cursor position after staging
	newCursorPos := calculateNewCursorPosition(params.CurrentDiffText, newDiffText, params.SelectStart, params.SelectEnd)

	// Return result
	result := &CommandAResult{
		Success:      true,
		NewDiffText:  newDiffText,
		ColoredDiff:  newColoredDiff,
		DiffLines:    newDiffLines,
		ShouldUpdate: len(strings.TrimSpace(newDiffText)) == 0,
		NewCursorPos: newCursorPos,
	}

	return result, nil
}

// calculateNewCursorPosition calculates the recommended cursor position after staging
func calculateNewCursorPosition(oldDiffText, newDiffText string, selectStart, selectEnd int) int {
	if len(strings.TrimSpace(newDiffText)) == 0 {
		return 0 // Return to the beginning if diff is empty
	}

	newLines := strings.Split(newDiffText, "\n")
	maxLines := len(newLines) - 1

	// Basically keep the original cursor position (selection start)
	newCursorPos := selectStart

	// Adjust if the new diff has fewer lines
	if newCursorPos > maxLines {
		newCursorPos = maxLines
	}

	// Ensure non-negative value
	if newCursorPos < 0 {
		newCursorPos = 0
	}

	return newCursorPos
}

// commandAUnstage handles unstaging selected lines from staged diff
func commandAUnstage(params CommandAParams) (*CommandAResult, error) {
	// Build file path
	filePath := filepath.Join(params.RepoRoot, params.CurrentFile)

	// Save current file content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to read file", "tomato")
		}
		return nil, err
	}

	// Get mapping for the selection range
	mapping := MapDisplayToOriginalIdx(params.CurrentDiffText)
	start := mapping[params.SelectStart]
	end := mapping[params.SelectEnd]

	// Check if the selection contains actual changes
	coloredDiff := ColorizeDiff(params.CurrentDiffText)
	displayLines := util.SplitLines(coloredDiff)

	hasChanges := false
	diffLines := strings.Split(params.CurrentDiffText, "\n")

	for i := params.SelectStart; i <= params.SelectEnd && i < len(displayLines); i++ {
		if originalIdx, ok := mapping[i]; ok {
			if originalIdx < len(diffLines) {
				originalLine := diffLines[originalIdx]
				if strings.HasPrefix(originalLine, "+") || strings.HasPrefix(originalLine, "-") {
					hasChanges = true
					break
				}
			}
		}
	}

	// Early return if no changes are included
	if !hasChanges {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("No changes were unstaged", "yellow")
		}
		result := &CommandAResult{
			Success:      true,
			NewDiffText:  params.CurrentDiffText,
			ColoredDiff:  coloredDiff,
			DiffLines:    displayLines,
			ShouldUpdate: false,
			NewCursorPos: params.SelectStart,
		}
		return result, nil
	}

	// Get file content with selected changes excluded
	modifiedContent, err := git.RevertSelectedChangesFromStaged(params.CurrentFile, params.RepoRoot, params.CurrentDiffText, start, end)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to process changes", "tomato")
		}
		return nil, err
	}

	// Temporarily overwrite file with selected changes excluded
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to write file", "tomato")
		}
		return nil, err
	}

	// Run git add (only unselected changes remain staged = selected changes become unstaged)
	cmd := exec.Command("git", "-c", "core.quotepath=false", "add", params.CurrentFile)
	cmd.Dir = params.RepoRoot
	_, gitErr := cmd.CombinedOutput()

	// Restore original file content
	restoreErr := os.WriteFile(filePath, originalContent, 0644)

	if gitErr != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to unstage changes", "tomato")
			if restoreErr != nil {
				params.UpdateGlobalStatus("Critical: Failed to restore file!", "tomato")
			}
		}
		return nil, gitErr
	}

	if restoreErr != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Warning: Failed to restore file", "yellow")
		}
		return nil, restoreErr
	}

	// Re-fetch the staged diff
	newDiffText, _ := git.GetStagedDiff(params.CurrentFile, params.RepoRoot)

	// Handle success
	if params.UpdateGlobalStatus != nil {
		params.UpdateGlobalStatus("Selected lines unstaged successfully!", "forestgreen")
	}

	// Colorize the diff
	newColoredDiff := ColorizeDiff(newDiffText)
	newDiffLines := util.SplitLines(newColoredDiff)

	// Calculate recommended cursor position after unstaging
	newCursorPos := calculateNewCursorPosition(params.CurrentDiffText, newDiffText, params.SelectStart, params.SelectEnd)

	// Return result
	result := &CommandAResult{
		Success:      true,
		NewDiffText:  newDiffText,
		ColoredDiff:  newColoredDiff,
		DiffLines:    newDiffLines,
		ShouldUpdate: len(strings.TrimSpace(newDiffText)) == 0,
		NewCursorPos: newCursorPos,
	}

	return result, nil
}
