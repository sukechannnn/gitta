package ui

import (
	"os"
	"os/exec"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui/commands"
	"github.com/sukechannnn/gitta/util"
)

// DiffViewContext contains all the context needed for diff view key bindings
type DiffViewContext struct {
	// UI Components
	diffView        *tview.TextView
	fileListView    *tview.TextView
	beforeView      *tview.TextView
	afterView       *tview.TextView
	unifiedViewFlex *tview.Flex
	splitViewFlex   *tview.Flex
	contentFlex     *tview.Flex
	app             *tview.Application

	// State
	cursorY               *int
	selectStart           *int
	selectEnd             *int
	isSelecting           *bool
	isSplitView           *bool
	leftPaneFocused       *bool
	currentDiffText       *string
	currentFile           *string
	currentStatus         *string
	savedTargetFile       *string
	preferUnstagedSection *bool

	// Paths
	repoRoot  string
	patchPath string

	// Key handling state
	gPressed  *bool
	lastGTime *time.Time

	// View updater
	viewUpdater DiffViewUpdater

	// Callbacks
	updateFileListView func()
	updateGlobalStatus func(string, string)
	refreshFileList    func()
	onUpdate           func()
}

// scrollDiffView scrolls the diff view by the specified direction and handles cursor following
func scrollDiffView(ctx *DiffViewContext, direction int) {
	if *ctx.isSplitView {
		currentRow, _ := ctx.beforeView.GetScrollOffset()
		maxLines := getSplitViewLineCount(*ctx.currentDiffText)

		nextRow := currentRow + direction
		// スクロール位置を更新（範囲内に収める）
		if nextRow >= 0 && nextRow < maxLines {
			ctx.beforeView.ScrollTo(nextRow, 0)
			ctx.afterView.ScrollTo(nextRow, 0)

			// カーソルが画面外になったら追従
			if direction > 0 && *ctx.cursorY < nextRow {
				// 下スクロール時：カーソルが画面最上部にある場合は追従
				*ctx.cursorY = nextRow
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
				}
			} else if direction < 0 && *ctx.cursorY > nextRow+20 {
				// 上スクロール時：カーソルが画面最下部にある場合は追従（画面高さを20行と仮定）
				*ctx.cursorY = nextRow + 20
				if *ctx.cursorY >= maxLines {
					*ctx.cursorY = maxLines - 1
				}
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
				}
			}
		}
	} else {
		// Unified Viewの場合
		currentRow, _ := ctx.diffView.GetScrollOffset()
		coloredDiff := ColorizeDiff(*ctx.currentDiffText)
		maxLines := len(util.SplitLines(coloredDiff))

		nextRow := currentRow + direction
		// スクロール位置を更新（範囲内に収める）
		if nextRow >= 0 && nextRow < maxLines {
			ctx.diffView.ScrollTo(nextRow, 0)

			// カーソルが画面外になったら追従
			if direction > 0 && *ctx.cursorY < nextRow {
				// 下スクロール時：カーソルが画面最上部にある場合は追従
				*ctx.cursorY = nextRow
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
				}
			} else if direction < 0 && *ctx.cursorY > nextRow+20 {
				// 上スクロール時：カーソルが画面最下部にある場合は追従（画面高さを20行と仮定）
				*ctx.cursorY = nextRow + 20
				if *ctx.cursorY >= maxLines {
					*ctx.cursorY = maxLines - 1
				}
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
				}
			}
		}
	}
}

// SetupDiffViewKeyBindings sets up key bindings for diff view
func SetupDiffViewKeyBindings(ctx *DiffViewContext) {
	// 初期状態でviewUpdaterを設定
	if ctx.viewUpdater == nil {
		if *ctx.isSplitView {
			ctx.viewUpdater = NewSplitViewUpdater(ctx.beforeView, ctx.afterView)
		} else {
			ctx.viewUpdater = NewUnifiedViewUpdater(ctx.diffView)
		}
	}

	// 共通のキーハンドラー関数
	keyHandler := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			// 選択モードをリセット（カーソル位置は保持）
			*ctx.isSelecting = false
			*ctx.selectEnd = -1
			*ctx.selectStart = -1
			// 表示を更新
			if ctx.viewUpdater != nil {
				ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
			}
			return nil
		case tcell.KeyEnter:
			// 左ペインに戻る
			*ctx.isSelecting = false
			*ctx.selectStart = -1
			*ctx.selectEnd = -1
			// 表示を更新
			if ctx.viewUpdater != nil {
				ctx.viewUpdater.UpdateWithoutCursor(*ctx.currentDiffText)
			}
			// 左ペインにフォーカスを戻す
			*ctx.leftPaneFocused = true
			ctx.updateFileListView()
			ctx.app.SetFocus(ctx.fileListView)
			return nil
		case tcell.KeyCtrlE:
			// Ctrl+E: 1行下にスクロール（カーソルは最上部になったら追従）
			scrollDiffView(ctx, 1)
			return nil
		case tcell.KeyCtrlY:
			// Ctrl+Y: 1行上にスクロール（カーソルは最下部になったら追従）
			scrollDiffView(ctx, -1)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 's':
				// Split Viewのトグル
				*ctx.isSplitView = !*ctx.isSplitView

				if *ctx.isSplitView {
					// Split Viewを表示（現在のカーソル位置を維持）
					ctx.viewUpdater = NewSplitViewUpdater(ctx.beforeView, ctx.afterView)
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					ctx.contentFlex.RemoveItem(ctx.unifiedViewFlex)
					ctx.contentFlex.AddItem(ctx.splitViewFlex, 0, DiffViewFlexRatio, false)
					// フォーカスがdiffViewにある場合、splitViewFlexに移動
					if !*ctx.leftPaneFocused {
						ctx.app.SetFocus(ctx.splitViewFlex)
					}
				} else {
					// 通常の差分表示に戻す
					ctx.viewUpdater = NewUnifiedViewUpdater(ctx.diffView)
					ctx.contentFlex.RemoveItem(ctx.splitViewFlex)
					ctx.contentFlex.AddItem(ctx.unifiedViewFlex, 0, DiffViewFlexRatio, false)
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					// フォーカスがsplitViewFlexにある場合、diffViewに移動
					if !*ctx.leftPaneFocused {
						ctx.app.SetFocus(ctx.diffView)
					}
				}
				return nil
			case 'g':
				now := time.Now()
				if *ctx.gPressed && now.Sub(*ctx.lastGTime) < 500*time.Millisecond {
					// gg → 最上部
					*ctx.cursorY = 0
					if *ctx.isSelecting {
						*ctx.selectEnd = *ctx.cursorY
					}
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
					}
					*ctx.gPressed = false
				} else {
					// 1回目のg
					*ctx.gPressed = true
					*ctx.lastGTime = now
				}
				return nil
			case 'G': // 大文字G → 最下部へ
				coloredDiff := ColorizeDiff(*ctx.currentDiffText)
				*ctx.cursorY = len(util.SplitLines(coloredDiff)) - 1
				if *ctx.isSelecting {
					*ctx.selectEnd = *ctx.cursorY
				}
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				return nil
			case 'j':
				// 下移動
				maxLines := len(*ctx.currentDiffText) - 1
				if *ctx.isSplitView {
					// Split Viewの場合は有効な行数を取得
					splitViewLines := getSplitViewLineCount(*ctx.currentDiffText)
					if splitViewLines > 0 {
						maxLines = splitViewLines - 1
					} else {
						maxLines = 0
					}
				}

				if *ctx.cursorY < maxLines {
					(*ctx.cursorY)++
					if *ctx.isSelecting {
						*ctx.selectEnd = *ctx.cursorY
					}
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
					}
				}
				return nil
			case 'k':
				// 上移動
				if *ctx.cursorY > 0 {
					(*ctx.cursorY)--
					if *ctx.isSelecting {
						*ctx.selectEnd = *ctx.cursorY
					}
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
					}
				}
				return nil
			case 'V':
				// Shift + V で選択モード開始
				if !*ctx.isSelecting {
					*ctx.isSelecting = true
					*ctx.selectStart = *ctx.cursorY
					*ctx.selectEnd = *ctx.cursorY
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithSelection(*ctx.currentDiffText, *ctx.cursorY, *ctx.selectStart, *ctx.selectEnd, *ctx.isSelecting)
					}
				}
				return nil
			case 'u':
				ctx.updateGlobalStatus("undo is not implemented!", "tomato")
			case 'a':
				// commandA関数を呼び出す
				params := commands.CommandAParams{
					SelectStart:        *ctx.selectStart,
					SelectEnd:          *ctx.selectEnd,
					CurrentFile:        *ctx.currentFile,
					CurrentStatus:      *ctx.currentStatus,
					CurrentDiffText:    *ctx.currentDiffText,
					RepoRoot:           ctx.repoRoot,
					UpdateGlobalStatus: ctx.updateGlobalStatus,
				}

				result, err := commands.CommandA(params)
				if err != nil {
					return nil
				}
				if result == nil {
					return nil
				}

				// 結果を反映
				*ctx.currentDiffText = result.NewDiffText

				// 選択を解除してカーソルリセット
				*ctx.isSelecting = false
				*ctx.selectStart = -1
				*ctx.selectEnd = -1
				*ctx.cursorY = 0

				// 再描画
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}

				// ファイルリストを内部的に更新
				ctx.refreshFileList()

				// 差分が残っている場合
				if !result.ShouldUpdate {
					// 現在のファイルの位置を維持するため、savedTargetFileを設定
					*ctx.savedTargetFile = *ctx.currentFile
					// ファイルリストを再描画
					ctx.updateFileListView()
				} else {
					// 差分がなくなった場合は、完全に更新
					if ctx.onUpdate != nil {
						ctx.onUpdate()
					}
				}
				return nil
			case 'A':
				// 現在のファイルをステージ/アンステージ
				if *ctx.currentFile != "" {
					var cmd *exec.Cmd
					if *ctx.currentStatus == "staged" {
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", *ctx.currentFile)
					} else {
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", *ctx.currentFile)
					}
					cmd.Dir = ctx.repoRoot

					err := cmd.Run()
					if err == nil {
						wasStaged := (*ctx.currentStatus == "staged")

						if *ctx.currentStatus == "staged" {
							// unstagedになったファイルの差分を表示
							*ctx.currentStatus = "unstaged"
							newDiffText, _ := git.GetFileDiff(*ctx.currentFile, ctx.repoRoot)
							*ctx.currentDiffText = newDiffText
						} else {
							// stagedになったファイルの差分を表示
							*ctx.currentStatus = "staged"
							newDiffText, _ := git.GetStagedDiff(*ctx.currentFile, ctx.repoRoot)
							*ctx.currentDiffText = newDiffText
						}

						// カーソルと選択をリセット
						*ctx.isSelecting = false
						*ctx.selectStart = -1
						*ctx.selectEnd = -1
						*ctx.cursorY = 0

						// 再描画
						if ctx.viewUpdater != nil {
							ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
						}

						// ステータスを更新
						if wasStaged {
							ctx.updateGlobalStatus("File unstaged successfully!", "forestgreen")
						} else {
							ctx.updateGlobalStatus("File staged successfully!", "forestgreen")
						}

						// refreshFileListを呼んで最新の状態を取得
						ctx.refreshFileList()

						// カーソル位置を保存
						// 常にunstagedセクションの先頭を選択するように設定
						*ctx.preferUnstagedSection = true
						*ctx.savedTargetFile = ""

						// ファイルリストを更新
						if ctx.onUpdate != nil {
							ctx.onUpdate()
						}
					} else {
						// エラーの場合
						if *ctx.currentStatus == "staged" {
							ctx.updateGlobalStatus("Failed to unstage file", "tomato")
						} else {
							ctx.updateGlobalStatus("Failed to stage file", "tomato")
						}
					}
				}
				return nil
			case 'q':
				// アプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				ctx.app.Stop()
				return nil
			}
		}
		return event
	}

	// DiffViewとSplitViewFlexの両方に同じキーハンドラーを設定
	ctx.diffView.SetInputCapture(keyHandler)
	ctx.splitViewFlex.SetInputCapture(keyHandler)
}
