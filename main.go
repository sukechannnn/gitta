package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/config" // 新しく作成した config パッケージをインポート
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui"
)

// Application はアプリケーション全体の状態と設定を保持します
type Application struct {
	App    *tview.Application
	Config *config.AppConfig // 設定構造体を含める
	// 必要に応じて他のグローバルな状態を追加
}

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	repoPath := "." // 現在のディレクトリにリポジトリがあると仮定

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := tview.NewApplication()

	// アプリケーション構造体の作成
	gittaApp := &Application{
		App:    app,
		Config: cfg,
	}

	// SIGINT (Ctrl+c) シグナルハンドラ
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		log.Println("SIGINT received, cleaning up and exiting...")
		// 設定から PatchFilePath を取得して削除
		if _, err := os.Stat(gittaApp.Config.PatchFilePath); err == nil {
			if err := os.Remove(gittaApp.Config.PatchFilePath); err != nil {
				log.Printf("Failed to remove %s: %v", gittaApp.Config.PatchFilePath, err)
			} else {
				log.Printf("Removed %s", gittaApp.Config.PatchFilePath)
			}
		}
		gittaApp.App.Stop()
		os.Exit(0)
	}()

	// アプリケーションレベルでのキー入力捕捉 (Ctrl+c ワークアラウンド)
	gittaApp.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			log.Println("Ctrl+c captured at application level, sending SIGINT...")
			gid, err := syscall.Getpgid(os.Getpid())
			if err == nil {
				syscall.Kill(-gid, syscall.SIGINT)
			}
			return nil // イベントをこれ以上伝播させない
		}
		return event // 他のイベントは通常通り処理
	})

	// 差分のあるファイルを取得
	modifiedFiles, untrackedFiles, err := git.GetModifiedFiles(repoPath)
	if err != nil {
		log.Fatalf("Failed to get modified files: %v", err)
	}

	// ファイル選択時の処理を定義（再帰的に使用するため関数として定義）
	var showFileDiff func(filePath string)
	showFileDiff = func(filePath string) {
		// ファイル差分ビューを作成し、ルートに設定
		// ShowFileDiffText に PatchFilePath を渡すように変更
		diffView := ui.ShowFileDiffText(gittaApp.App, filePath, debug, gittaApp.Config.PatchFilePath, func() {
			// 差分ビューから戻る際のコールバック
			// ファイル一覧ビューを作成し、ルートに設定
			fileListView := ui.ShowFileList(gittaApp.App, modifiedFiles, untrackedFiles, showFileDiff)
			gittaApp.App.SetRoot(fileListView, true)
		})
		gittaApp.App.SetRoot(diffView, true)
	}

	// 初期ビュー（ファイル一覧）を作成し、ルートに設定
	initialView := ui.ShowFileList(gittaApp.App, modifiedFiles, untrackedFiles, showFileDiff)
	gittaApp.App.SetRoot(initialView, true)

	// アプリケーションの実行は main で一度だけ
	if err := gittaApp.App.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
