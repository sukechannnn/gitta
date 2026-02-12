package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	dbg "runtime/debug"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/config"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui"
)

var version = "dev"

// Application はアプリケーション全体の状態と設定を保持
type Application struct {
	App    *tview.Application
	Config *config.AppConfig
}

func main() {
	var debug bool
	var autoRefresh bool
	var showVersion bool
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&autoRefresh, "watch", false, "Watch for file changes and auto-refresh")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.Parse()

	if showVersion {
		if version == "dev" {
			if info, ok := dbg.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}
		}
		fmt.Printf("%s\n", version)
		return
	}

	// Git リポジトリのルートを検出
	repoPath, err := git.FindGitRoot(".")
	if err != nil {
		log.Fatalf("Git repository not found: %v", err)
	}

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
	stagedFiles, modifiedFiles, untrackedFiles, err := git.GetChangedFiles(repoPath)
	if err != nil {
		log.Fatalf("Failed to get modified files: %v", err)
	}

	// ファイル選択時の処理を定義（再帰的に使用するため関数として定義）
	var updateFileList func()

	updateFileList = func() {
		updatedStagedFiles, updatedModifiedFiles, updatedUntrackedFiles, err := git.GetChangedFiles(repoPath)
		if err != nil {
			log.Fatalf("Failed to get updated files: %v", err)
		}
		rootEditor := ui.RootEditor(
			gittaApp.App,
			updatedStagedFiles,
			updatedModifiedFiles,
			updatedUntrackedFiles,
			repoPath,
			gittaApp.Config.PatchFilePath,
			updateFileList,
			autoRefresh,
		)
		gittaApp.App.SetRoot(rootEditor, true)
	}

	// 初期ビュー（ファイル一覧）を作成し、ルートに設定
	// onSelect パラメータは現在使用されていないため nil を渡す
	initialView := ui.RootEditor(gittaApp.App, stagedFiles, modifiedFiles, untrackedFiles, repoPath, gittaApp.Config.PatchFilePath, updateFileList, autoRefresh)
	gittaApp.App.SetRoot(initialView, true)

	// アプリケーションの実行は main で一度だけ
	if err := gittaApp.App.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}
