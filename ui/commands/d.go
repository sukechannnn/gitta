package commands

import (
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CommandDParams contains parameters for commandD function
type CommandDParams struct {
	CurrentFile        string
	CurrentStatus      string
	RepoRoot           string
	UpdateGlobalStatus func(string, string)
}

// CommandD handles the 'd' command for discarding changes
func CommandD(params CommandDParams, app *tview.Application) error {
	// stagedファイルの場合はサポートされていないことを通知
	if params.CurrentStatus == "staged" {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("Cannot discard staged changes. Use 'u' to unstage first.", "tomato")
		}
		return nil
	}

	// ファイルが選択されていない場合
	if params.CurrentFile == "" {
		if params.UpdateGlobalStatus != nil {
			params.UpdateGlobalStatus("No file selected", "tomato")
		}
		return nil
	}

	// 確認ダイアログを表示
	modal := tview.NewModal().
		SetText("Discard all changes in " + params.CurrentFile + "?\n\nThis action cannot be undone!").
		AddButtons([]string{"Discard", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Discard" {
				// git checkout -- <file> を実行
				cmd := exec.Command("git", "checkout", "--", params.CurrentFile)
				cmd.Dir = params.RepoRoot
				_, err := cmd.CombinedOutput()

				if err != nil {
					if params.UpdateGlobalStatus != nil {
						params.UpdateGlobalStatus("Failed to discard changes", "tomato")
					}
				} else {
					if params.UpdateGlobalStatus != nil {
						params.UpdateGlobalStatus("Changes discarded successfully!", "forestgreen")
					}
				}
			}
			// メインページに戻る
			app.SetRoot(app.GetFocus(), true)
		})

	// 確認ダイアログの背景色を設定
	modal.SetBackgroundColor(tcell.ColorDefault)

	// 確認ダイアログを表示
	app.SetRoot(modal, true)

	return nil
}