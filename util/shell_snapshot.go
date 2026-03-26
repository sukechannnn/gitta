package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var snapshotPath string

// CreateShellSnapshot captures the current shell environment (aliases, functions)
// by sourcing the user's shell config and saving a snapshot file.
func CreateShellSnapshot() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Determine which config file to source
	var rcFile string
	if strings.HasSuffix(shell, "zsh") {
		rcFile = filepath.Join(homeDir, ".zshrc")
	} else if strings.HasSuffix(shell, "bash") {
		rcFile = filepath.Join(homeDir, ".bashrc")
	} else {
		return
	}

	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		return
	}

	// Create snapshot file in temp directory
	tmpFile, err := os.CreateTemp("", "giff-snapshot-*.sh")
	if err != nil {
		return
	}
	snapshotPath = tmpFile.Name()
	tmpFile.Close()

	// Build script to source rc file and dump aliases/functions
	var script string
	if strings.HasSuffix(shell, "zsh") {
		script = fmt.Sprintf(`
source "%s" < /dev/null 2>/dev/null

{
  echo "# giff shell snapshot"
  echo "unalias -a 2>/dev/null || true"
  echo ""

  # Dump aliases
  alias | while IFS= read -r line; do
    echo "$line"
  done
  echo ""

  # Dump functions (exclude internal ones)
  typeset -f | grep -E '^\S+ \(\) \{' | while IFS= read -r line; do
    fname="${line%% *}"
    case "$fname" in
      _*|prompt_*|compdef|compinit|bashcompinit) continue ;;
    esac
    echo "$(typeset -f "$fname")"
    echo ""
  done
} > "%s"
`, rcFile, snapshotPath)
	} else {
		script = fmt.Sprintf(`
source "%s" 2>/dev/null

{
  echo "# giff shell snapshot"
  echo "unalias -a 2>/dev/null || true"
  echo ""

  # Dump aliases
  alias -p 2>/dev/null
  echo ""
} > "%s"
`, rcFile, snapshotPath)
	}

	cmd := exec.Command(shell, "-c", script)
	cmd.Run()
}

// GetSnapshotPath returns the path to the snapshot file, or empty string if none.
func GetSnapshotPath() string {
	return snapshotPath
}

// CleanupShellSnapshot removes the snapshot file.
func CleanupShellSnapshot() {
	if snapshotPath != "" {
		os.Remove(snapshotPath)
		snapshotPath = ""
	}
}
