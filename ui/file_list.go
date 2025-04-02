package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ファイル一覧を表示
func ShowFileList(app *tview.Application, modifiedFiles, untrackedFiles []string, onSelect func(file string)) {
	// カスタムテキストビューを作成
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	// ファイル一覧を構築
	var fileList []string

	// 変更されたファイル
	if len(modifiedFiles) > 0 {
		fileList = append(fileList, "[yellow]Modified Files:[white]")
		for _, file := range modifiedFiles {
			file = strings.TrimSpace(file)
			if file != "" {
				fileList = append(fileList, " [white]"+file)
			}
		}
	}

	// 未追跡ファイル
	if len(untrackedFiles) > 0 {
		// 空行を追加
		fileList = append(fileList, "")
		fileList = append(fileList, "[yellow]Untracked Files:[white]")
		for _, file := range untrackedFiles {
			file = strings.TrimSpace(file)
			if file != "" {
				fileList = append(fileList, " [white]"+file)
			}
		}
	}

	// テキストビューにファイル一覧を表示
	content := strings.Join(fileList, "\n")
	textView.SetText(content)

	// キー入力の処理
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			// 現在の行を取得
			_, y := textView.GetScrollOffset()
			if y < len(fileList) && y > 0 && !strings.Contains(fileList[y], "Files:") {
				// ヘッダー行でなければファイル名を取得して選択
				file := strings.TrimSpace(strings.TrimPrefix(fileList[y], " [white]"))
				if file != "" && onSelect != nil {
					onSelect(file)
				}
			}
		}
		return event
	})

	// アプリケーションのルートに設定
	app.SetRoot(textView, true).Run()
}
