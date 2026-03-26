package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileContent reads the contents of a file
func ReadFileContent(filePath string, repoRoot string) (string, error) {
	fullPath := filepath.Join(repoRoot, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FormatAsAddedLines formats file content as all-added lines in diff format
func FormatAsAddedLines(content string, filePath string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	// Git diff header
	result.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	result.WriteString("new file mode 100644\n")
	result.WriteString("index 0000000..0000000\n")
	result.WriteString("--- /dev/null\n")
	result.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))
	result.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))

	// Display each line as an added line
	for _, line := range lines {
		result.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return result.String()
}
