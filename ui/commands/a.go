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
	SelectStart      int
	SelectEnd        int
	CurrentFile      string
	CurrentStatus    string
	CurrentDiffText  string
	RepoRoot         string
	UpdateListStatus func(string, string)
}

// CommandAResult contains the results from commandA execution
type CommandAResult struct {
	Success      bool
	NewDiffText  string
	ColoredDiff  string
	DiffLines    []string
	ShouldUpdate bool
}

// CommandA handles the 'a' command for staging selected lines
func CommandA(params CommandAParams) (*CommandAResult, error) {
	// Validate inputs
	if params.SelectStart == -1 || params.SelectEnd == -1 || params.CurrentFile == "" || params.CurrentDiffText == "" {
		return nil, nil
	}

	// Staged ファイルでは行単位のunstageは未対応
	if params.CurrentStatus == "staged" {
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Line-by-line unstaging is not implemented!", "firebrick")
		}
		return nil, nil
	}

	// ファイルパスを構築
	filePath := filepath.Join(params.RepoRoot, params.CurrentFile)

	// 現在のファイル内容を保存
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Failed to read file", "firebrick")
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
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("No changes were staged", "yellow")
		}
		// 成功として扱うが、変更はなし
		result := &CommandAResult{
			Success:      true,
			NewDiffText:  params.CurrentDiffText,
			ColoredDiff:  coloredDiff,
			DiffLines:    displayLines,
			ShouldUpdate: false,
		}
		return result, nil
	}

	modifiedContent, _, err := git.ApplySelectedChangesToFile(params.CurrentFile, params.RepoRoot, params.CurrentDiffText, start, end)
	if err != nil {
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Failed to process changes", "firebrick")
		}
		return nil, err
	}

	// 一時的に選択した変更のみのファイルに書き換え
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Failed to write file", "firebrick")
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
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Failed to stage changes", "firebrick")
			if restoreErr != nil {
				params.UpdateListStatus("Critical: Failed to restore file!", "firebrick")
			}
		}
		return nil, gitErr
	}

	if restoreErr != nil {
		if params.UpdateListStatus != nil {
			params.UpdateListStatus("Warning: Failed to restore unstaged changes", "yellow")
		}
		return nil, restoreErr
	}

	// 差分を再取得
	newDiffText, _ := git.GetFileDiff(params.CurrentFile, params.RepoRoot)

	// 成功した場合の処理
	if params.UpdateListStatus != nil {
		params.UpdateListStatus("Selected lines staged successfully!", "gold")
	}

	// ColorizeDiffで色付け
	newColoredDiff := ColorizeDiff(newDiffText)
	newDiffLines := util.SplitLines(newColoredDiff)

	// 結果を返す
	result := &CommandAResult{
		Success:      true,
		NewDiffText:  newDiffText,
		ColoredDiff:  newColoredDiff,
		DiffLines:    newDiffLines,
		ShouldUpdate: len(strings.TrimSpace(newDiffText)) == 0,
	}

	return result, nil
}
