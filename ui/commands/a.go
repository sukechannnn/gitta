package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/util"
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
}

// CommandAResult contains the results from commandA execution
type CommandAResult struct {
	Success       bool
	NewDiffText   string
	ColoredDiff   string
	DiffLines     []string
	ShouldUpdate  bool
	NewCursorPos  int  // ステージング後の推奨カーソル位置
}

// CommandA handles the 'a' command for staging selected lines
func CommandA(params CommandAParams) (*CommandAResult, error) {
	// Validate inputs
	if params.SelectStart == -1 || params.SelectEnd == -1 || params.CurrentFile == "" || params.CurrentDiffText == "" {
		return nil, nil
	}

	// Staged ファイルでは行単位のunstageは未対応
	if params.CurrentStatus == "staged" {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Line-by-line unstaging is not implemented!", "tomato")
		}
		return nil, nil
	}

	// ファイルパスを構築
	filePath := filepath.Join(params.RepoRoot, params.CurrentFile)

	// 現在のファイル内容を保存
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to read file", "tomato")
		}
		return nil, err
	}

	// 選択した変更のみを適用したファイル内容を取得
	mapping := MapDisplayToOriginalIdx(params.CurrentDiffText)
	start := mapping[params.SelectStart]
	end := mapping[params.SelectEnd]

	// 選択範囲に実際の変更が含まれているかチェック
	// 表示されている行を作成（ヘッダーを除外）
	coloredDiff := ColorizeDiff(params.CurrentDiffText)
	displayLines := util.SplitLines(coloredDiff)

	hasChanges := false
	diffLines := strings.Split(params.CurrentDiffText, "\n")

	for i := params.SelectStart; i <= params.SelectEnd && i < len(displayLines); i++ {
		// 元の差分行を直接チェック（色付け前）
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

	// 変更が含まれていない場合は早期リターン
	if !hasChanges {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("No changes were staged", "yellow")
		}
		// 成功として扱うが、変更はなし
		result := &CommandAResult{
			Success:       true,
			NewDiffText:   params.CurrentDiffText,
			ColoredDiff:   coloredDiff,
			DiffLines:     displayLines,
			ShouldUpdate:  false,
			NewCursorPos:  params.SelectStart, // 変更がない場合は元の位置を保持
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

	// 一時的に選択した変更のみのファイルに書き換え
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Failed to write file", "tomato")
		}
		return nil, err
	}

	// git add を実行
	// -c core.quotepath=false でマルチバイトファイル名をエスケープしないようにする
	cmd := exec.Command("git", "-c", "core.quotepath=false", "add", params.CurrentFile)
	cmd.Dir = params.RepoRoot
	_, gitErr := cmd.CombinedOutput()

	// 元のファイル内容に戻す（選択されなかった変更も含む）
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

	// 差分を再取得
	newDiffText, _ := git.GetFileDiff(params.CurrentFile, params.RepoRoot)

	// 成功した場合の処理
	if params.UpdateGlobalStatus != nil {
		params.UpdateGlobalStatus("Selected lines staged successfully!", "forestgreen")
	}

	// ColorizeDiffで色付け
	newColoredDiff := ColorizeDiff(newDiffText)
	newDiffLines := util.SplitLines(newColoredDiff)

	// ステージング後の推奨カーソル位置を計算
	newCursorPos := calculateNewCursorPosition(params.CurrentDiffText, newDiffText, params.SelectStart, params.SelectEnd)

	// 結果を返す
	result := &CommandAResult{
		Success:       true,
		NewDiffText:   newDiffText,
		ColoredDiff:   newColoredDiff,
		DiffLines:     newDiffLines,
		ShouldUpdate:  len(strings.TrimSpace(newDiffText)) == 0,
		NewCursorPos:  newCursorPos,
	}

	return result, nil
}

// calculateNewCursorPosition calculates the recommended cursor position after staging
func calculateNewCursorPosition(oldDiffText, newDiffText string, selectStart, selectEnd int) int {
	if len(strings.TrimSpace(newDiffText)) == 0 {
		return 0 // 差分がなくなった場合は先頭
	}

	newLines := strings.Split(newDiffText, "\n")
	maxLines := len(newLines) - 1

	// 基本的に元のカーソル位置（選択開始位置）を維持
	newCursorPos := selectStart

	// 新しい差分の行数が減った場合は調整
	if newCursorPos > maxLines {
		newCursorPos = maxLines
	}

	// 負の値にならないよう調整
	if newCursorPos < 0 {
		newCursorPos = 0
	}

	return newCursorPos
}
