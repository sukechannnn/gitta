package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CommandDParams contains parameters for commandD function
type CommandDParams struct {
	CurrentFile   string
	CurrentStatus string
	RepoRoot      string
}

// CommandD handles the 'd' command for discarding changes
func CommandD(params CommandDParams) error {
	// Notify that discarding staged changes is not supported
	if params.CurrentStatus == "staged" {
		return fmt.Errorf("Cannot discard staged changes. Use 'a' to unstage first.")
	}

	// No file selected
	if params.CurrentFile == "" {
		return fmt.Errorf("No file selected")
	}

	// Delete the file if it is untracked
	if params.CurrentStatus == "untracked" {
		fullPath := filepath.Join(params.RepoRoot, params.CurrentFile)
		if err := os.RemoveAll(fullPath); err != nil {
			return fmt.Errorf("failed to delete %s: %w", params.CurrentFile, err)
		}
		return nil
	}

	// Discard changes via git checkout for unstaged files
	cmd := exec.Command("git", "checkout", "--", params.CurrentFile)
	cmd.Dir = params.RepoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("discard changes for %s failed: %w (output: %s)", params.CurrentFile, err, string(output))
	}

	return nil
}
