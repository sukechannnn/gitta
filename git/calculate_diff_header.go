package git

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
)

// CalculateGitHash calculates the Git-style hash for a file.
func CalculateGitHash(repoPath, filePath string) (string, error) {
	// Get the hash value at HEAD
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	// Get the HEAD commit
	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", err
	}
	currentEntry, _ := headTree.FindEntry(filePath)

	headHash := "0000000"
	if currentEntry != nil {
		headHash = currentEntry.Hash.String()[:7]
	}

	// Generate a hash value from the retrieved diff
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read the file contents
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()
	content := make([]byte, fileSize)
	_, err = file.Read(content)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Prepare data following Git's format
	header := fmt.Sprintf("blob %d\000", fileSize) // `blob <size>\0`
	data := append([]byte(header), content...)

	// Calculate SHA-1 hash
	hash := sha1.Sum(data)

	// Convert hash to string and return
	currentHash := fmt.Sprintf("%x", hash)

	fileMode := stat.Mode()

	return fmt.Sprintf(`diff --git a/%s b/%s
index %s..%s %s
--- a/%s
+++ b/%s
`, filePath, filePath, headHash, currentHash, fileMode, filePath, filePath), nil
}
