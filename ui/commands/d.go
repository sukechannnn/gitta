package commands

import (
	"fmt"
	"os/exec"
)

// CommandDParams contains parameters for commandD function
type CommandDParams struct {
	CurrentFile   string
	CurrentStatus string
	RepoRoot      string
}

// CommandD handles the 'd' command for discarding changes
func CommandD(params CommandDParams) error {
	// stagedファイルの場合はサポートされていないことを通知
	if params.CurrentStatus == "staged" {
		return fmt.Errorf("Cannot discard staged changes. Use 'u' to unstage first.")
	}

	// ファイルが選択されていない場合
	if params.CurrentFile == "" {
		return fmt.Errorf("No file selected")
	}

	cmd := exec.Command("git", "checkout", "--", params.CurrentFile)
	cmd.Dir = params.RepoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("discard changes for %s failed: %w (output: %s)", params.CurrentFile, err, string(output))
	}

	return nil
}
