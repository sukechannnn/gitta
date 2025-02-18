package ui

import (
	"strings"

	"github.com/rivo/tview"
)

// ファイル一覧を表示
func ShowFileList(app *tview.Application, modifiedFiles, untrackedFiles []string, onSelect func(file string)) {
	list := tview.NewList()

	// 差分のある既存ファイル
	if len(modifiedFiles) > 0 {
		list.AddItem("Modified Files:", "", 0, nil)
		for _, file := range modifiedFiles {
			file := strings.TrimSpace(file) // 改行や空白を取り除く
			if file != "" {                 // 空でない場合のみリストに追加
				list.AddItem(file, "", 0, func() {
					onSelect(file)
				})
			}
		}
	}

	list.AddItem("", "", 0, nil)

	// 新規ファイル
	if len(untrackedFiles) > 0 {
		list.AddItem("Untracked Files:", "", 0, nil)
		for _, file := range untrackedFiles {
			file := strings.TrimSpace(file) // 改行や空白を取り除く
			if file != "" {                 // 空でない場合のみリストに追加
				list.AddItem(file, "", 0, func() {
					onSelect(file)
				})
			}
		}
	}

	// アプリケーションのルートに設定
	app.SetRoot(list, true).Run()
}
