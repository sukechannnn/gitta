package main

import (
	"log"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitadd/git"
	"github.com/sukechannnn/gitadd/ui"
)

func main() {
	repoPath := "." // 現在のディレクトリにリポジトリがあると仮定
	app := tview.NewApplication()

	// 差分のあるファイルを取得
	modifiedFiles, untrackedFiles, err := git.GetModifiedFiles(repoPath)
	if err != nil {
		log.Fatalf("Failed to get modified files: %v", err)
	}

	// ファイル選択時の処理を定義（再帰的に使用するため関数として定義）
	var showFileDiff func(file string)
	showFileDiff = func(file string) {
		// ファイル内容を取得して表示
		content, err := git.GetFileDiff(repoPath, file)
		if err != nil {
			log.Fatalf("Failed to get file diff: %v", err)
		}

		ui.ShowFileDiffText(app, content, func() {
			// ファイル一覧に戻る（同じコールバック関数を渡す）
			ui.ShowFileList(app, modifiedFiles, untrackedFiles, showFileDiff)
		})
	}

	// ファイル一覧を表示（初期表示）
	ui.ShowFileList(app, modifiedFiles, untrackedFiles, showFileDiff)
}
