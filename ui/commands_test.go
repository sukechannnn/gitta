package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandA(t *testing.T) {
	t.Run("returns nil when selectStart is -1", func(t *testing.T) {
		params := CommandAParams{
			SelectStart: -1,
			SelectEnd:   5,
		}
		result, err := commandA(params)
		if result != nil || err != nil {
			t.Errorf("Expected nil result and nil error, got result=%v, err=%v", result, err)
		}
	})

	t.Run("returns nil when currentFile is empty", func(t *testing.T) {
		params := CommandAParams{
			SelectStart:     0,
			SelectEnd:       5,
			CurrentFile:     "",
			CurrentDiffText: "some diff",
		}
		result, err := commandA(params)
		if result != nil || err != nil {
			t.Errorf("Expected nil result and nil error, got result=%v, err=%v", result, err)
		}
	})

	t.Run("returns nil when currentStatus is staged", func(t *testing.T) {
		params := CommandAParams{
			SelectStart:     0,
			SelectEnd:       5,
			CurrentFile:     "test.txt",
			CurrentStatus:   "staged",
			CurrentDiffText: "some diff",
		}
		result, err := commandA(params)
		if result != nil || err != nil {
			t.Errorf("Expected nil result and nil error for staged file, got result=%v, err=%v", result, err)
		}
	})

	t.Run("handles file read error", func(t *testing.T) {
		statusMessages := []string{}
		params := CommandAParams{
			SelectStart:     0,
			SelectEnd:       5,
			CurrentFile:     "nonexistent.txt",
			CurrentStatus:   "unstaged",
			CurrentDiffText: "some diff",
			RepoRoot:        "/tmp",
			UpdateListStatus: func(msg, color string) {
				statusMessages = append(statusMessages, msg)
			},
		}
		result, err := commandA(params)
		if result != nil {
			t.Errorf("Expected nil result for file read error, got %v", result)
		}
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
		if len(statusMessages) == 0 || !strings.Contains(statusMessages[0], "Failed to read file") {
			t.Errorf("Expected 'Failed to read file' status message, got %v", statusMessages)
		}
	})
}

func TestCommandA_Integration(t *testing.T) {
	// Create a temporary test directory
	tmpDir, err := os.MkdirTemp("", "gitta-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatal("Failed to initialize git repo:", err)
	}

	// Configure git user
	configCmd1 := exec.Command("git", "config", "user.email", "test@example.com")
	configCmd1.Dir = tmpDir
	configCmd1.Run()

	configCmd2 := exec.Command("git", "config", "user.name", "Test User")
	configCmd2.Dir = tmpDir
	configCmd2.Run()

	// Create initial file and commit
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = tmpDir
	if err := addCmd.Run(); err != nil {
		t.Fatal("Failed to add file:", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "initial")
	commitCmd.Dir = tmpDir
	if err := commitCmd.Run(); err != nil {
		t.Fatal("Failed to commit:", err)
	}

	// Modify the file
	modifiedContent := "line1\nmodified line2\nline3\nnew line4\n"
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Get the diff
	diffCmd := exec.Command("git", "diff", "test.txt")
	diffCmd.Dir = tmpDir
	diffOutput, err := diffCmd.Output()
	if err != nil {
		t.Fatal("Failed to get diff:", err)
	}

	diffText := string(diffOutput)

	t.Run("stages changes successfully when actual changes are selected", func(t *testing.T) {
		statusMessages := []string{}
		params := CommandAParams{
			SelectStart:     2,
			SelectEnd:       3,
			CurrentFile:     "test.txt",
			CurrentStatus:   "unstaged",
			CurrentDiffText: diffText,
			RepoRoot:        tmpDir,
			UpdateListStatus: func(msg, color string) {
				statusMessages = append(statusMessages, msg)
			},
		}

		result, err := commandA(params)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Error("Expected result, got nil")
		} else if !result.Success {
			t.Error("Expected success=true")
		}

		// Check that success message was sent
		found := false
		for _, msg := range statusMessages {
			if strings.Contains(msg, "Selected lines staged successfully") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected 'Selected lines staged successfully' message, got %v", statusMessages)
		}
	})

	t.Run("reports no changes when only context lines are selected", func(t *testing.T) {
		statusMessages := []string{}
		params := CommandAParams{
			SelectStart:     0, // " line1" - コンテキスト行
			SelectEnd:       0, // " line1" - コンテキスト行
			CurrentFile:     "test.txt",
			CurrentStatus:   "unstaged",
			CurrentDiffText: diffText,
			RepoRoot:        tmpDir,
			UpdateListStatus: func(msg, color string) {
				statusMessages = append(statusMessages, msg)
			},
		}

		result, err := commandA(params)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Error("Expected result, got nil")
		} else if !result.Success {
			t.Error("Expected success=true")
		}

		// Check that "no changes" message was sent
		found := false
		for _, msg := range statusMessages {
			if strings.Contains(msg, "No changes were staged") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected 'No changes were staged' message, got %v", statusMessages)
		}
	})

	t.Run("stages only selected lines correctly", func(t *testing.T) {
		// Create another temporary directory for this specific test
		tmpDir2, err := os.MkdirTemp("", "gitta-test2")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir2)

		// Initialize git repo
		initCmd := exec.Command("git", "init")
		initCmd.Dir = tmpDir2
		if err := initCmd.Run(); err != nil {
			t.Fatal("Failed to initialize git repo:", err)
		}

		// Configure git user
		emailCmd := exec.Command("git", "config", "user.email", "test@example.com")
		emailCmd.Dir = tmpDir2
		if err := emailCmd.Run(); err != nil {
			t.Fatal("Failed to set git email:", err)
		}
		nameCmd := exec.Command("git", "config", "user.name", "Test User")
		nameCmd.Dir = tmpDir2
		if err := nameCmd.Run(); err != nil {
			t.Fatal("Failed to set git name:", err)
		}

		// Create initial file with multiple lines
		testFile := filepath.Join(tmpDir2, "test.txt")
		initialContent := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Initial commit
		addCmd := exec.Command("git", "add", "test.txt")
		addCmd.Dir = tmpDir2
		if err := addCmd.Run(); err != nil {
			t.Fatal("Failed to add file:", err)
		}
		commitCmd := exec.Command("git", "commit", "-m", "initial")
		commitCmd.Dir = tmpDir2
		if err := commitCmd.Run(); err != nil {
			t.Fatal("Failed to commit:", err)
		}

		// Modify multiple lines
		modifiedContent := "line1\nmodified line2\nline3\nmodified line4\nline5\n"
		if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Get the diff
		diffCmd := exec.Command("git", "diff", "test.txt")
		diffCmd.Dir = tmpDir2
		diffOutput, err := diffCmd.Output()
		if err != nil {
			t.Fatal("Failed to get diff:", err)
		}
		diffText := string(diffOutput)

		// Stage only the first modification (line2)
		params := CommandAParams{
			SelectStart:      2,
			SelectEnd:        3,
			CurrentFile:      "test.txt",
			CurrentStatus:    "unstaged",
			CurrentDiffText:  diffText,
			RepoRoot:         tmpDir2,
			UpdateListStatus: func(msg, color string) {},
		}

		result, err := commandA(params)
		if err != nil {
			t.Fatalf("commandA failed: %v", err)
		}
		if result == nil || !result.Success {
			t.Fatal("Expected successful result")
		}

		// Check staged content - should only have line2 modification
		stagedDiffCmd := exec.Command("git", "diff", "--cached", "test.txt")
		stagedDiffCmd.Dir = tmpDir2
		stagedOutput, err := stagedDiffCmd.Output()
		if err != nil {
			t.Fatal("Failed to get staged diff:", err)
		}
		stagedDiff := string(stagedOutput)

		if !strings.Contains(stagedDiff, "modified line2") {
			t.Error("Staged diff should contain 'modified line2'")
		}
		if strings.Contains(stagedDiff, "modified line4") {
			t.Error("Staged diff should NOT contain 'modified line4'")
		}

		// Check unstaged content - should still have line4 modification
		unstagedDiffCmd := exec.Command("git", "diff", "test.txt")
		unstagedDiffCmd.Dir = tmpDir2
		unstagedOutput, err := unstagedDiffCmd.Output()
		if err != nil {
			t.Fatal("Failed to get unstaged diff:", err)
		}
		unstagedDiff := string(unstagedOutput)

		// The unstaged diff should not contain the "-line2" to "+modified line2" change
		// but it may show "modified line2" as a context line
		if strings.Contains(unstagedDiff, "-line2") && strings.Contains(unstagedDiff, "+modified line2") {
			t.Error("Unstaged diff should NOT contain the line2 modification")
		}
		if !strings.Contains(unstagedDiff, "-line4") || !strings.Contains(unstagedDiff, "+modified line4") {
			t.Error("Unstaged diff should still contain the complete line4 modification")
		}
	})
}
