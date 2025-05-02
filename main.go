package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	repoPath := "." // 現在のディレクトリにリポジトリがあると仮定
	app := tview.NewApplication()

	// SIGINT (Ctrl+c) シグナルを捕捉するためのチャネルを作成
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 別ゴルーチンでシグナルを待機し、受信したらクリーンアップ処理を実行して終了
	go func() {
		<-sigChan
		log.Println("SIGINT received, cleaning up...")

		patchFilePath := "selected.patch" // 実際のパスに修正が必要
		if _, err := os.Stat(patchFilePath); err == nil {
			// ファイルが存在する場合のみ削除
			if err := os.Remove(patchFilePath); err != nil {
				log.Printf("Failed to remove %s: %v", patchFilePath, err)
			} else {
				log.Printf("Removed %s", patchFilePath)
			}
		}
		app.Stop() // tview アプリケーションを停止
		os.Exit(0) // プロセスを終了
	}()

	// 差分のあるファイルを取得
	modifiedFiles, untrackedFiles, err := git.GetModifiedFiles(repoPath)
	if err != nil {
		log.Fatalf("Failed to get modified files: %v", err)
	}

	// ビュー切り替え関数を定義
	var showFileDiff func(filePath string)
	showFileDiff = func(filePath string) {
		// ファイル差分ビューを作成し、ルートに設定
		diffView := ui.ShowFileDiffText(app, filePath, debug, func() {
			// 差分ビューから戻る際のコールバック
			// ファイル一覧ビューを作成し、ルートに設定
			fileListView := ui.ShowFileList(app, modifiedFiles, untrackedFiles, showFileDiff)
			app.SetRoot(fileListView, true)
		})
		app.SetRoot(diffView, true)
	}

	// 初期ビュー（ファイル一覧）を作成し、ルートに設定
	initialView := ui.ShowFileList(app, modifiedFiles, untrackedFiles, showFileDiff)
	app.SetRoot(initialView, true)

	// アプリケーションの実行は main で一度だけ
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
