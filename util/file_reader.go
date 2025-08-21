package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileContent ファイルの内容を読み取る
func ReadFileContent(filePath string, repoRoot string) (string, error) {
	fullPath := filepath.Join(repoRoot, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FormatAsAddedLines ファイル内容を全て追加行として差分形式にフォーマット
func FormatAsAddedLines(content string, filePath string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder

	// Git差分のヘッダー
	result.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	result.WriteString("new file mode 100644\n")
	result.WriteString("index 0000000..0000000\n")
	result.WriteString("--- /dev/null\n")
	result.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))
	result.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))

	// 各行を追加行として表示
	for _, line := range lines {
		result.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return result.String()
}
