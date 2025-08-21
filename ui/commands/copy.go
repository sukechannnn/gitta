package commands

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CopyToClipboard copies text to system clipboard
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("clip")
	default:
		return errors.New("unsupported os")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		return err
	}

	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// CopyFileName copies just the filename (without path) to clipboard
func CopyFileName(filePath string) error {
	fileName := filepath.Base(filePath)
	return CopyToClipboard(fileName)
}

// CopyFilePath copies the full relative path to clipboard
func CopyFilePath(filePath string) error {
	return CopyToClipboard(filePath)
}
