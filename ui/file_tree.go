package ui

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/giff/git"
	"github.com/sukechannnn/giff/ui/commands"
)

// moveFileListSelection moves the file list selection.
// If currently on a directory, moves between directories. If on a file, moves between files.
func moveFileListSelection(ctx *FileListKeyContext, direction int) {
	if *ctx.currentSelection < 0 || *ctx.currentSelection >= len(*ctx.fileList) {
		return
	}
	onDirectory := (*ctx.fileList)[*ctx.currentSelection].IsDirectory

	next := *ctx.currentSelection + direction
	for next >= 0 && next < len(*ctx.fileList) {
		if (*ctx.fileList)[next].IsDirectory == onDirectory {
			*ctx.currentSelection = next
			ctx.updateFileListView()
			if !onDirectory {
				ctx.updateSelectedFileDiff()
			}
			return
		}
		next += direction
	}
}

// findParentDirectory finds the parent directory entry for the given entry
func findParentDirectory(fileList *[]FileEntry, currentIdx int) int {
	entry := (*fileList)[currentIdx]
	for i := currentIdx - 1; i >= 0; i-- {
		candidate := (*fileList)[i]
		if candidate.IsDirectory && candidate.StageStatus == entry.StageStatus &&
			strings.HasPrefix(entry.Path, candidate.Path+"/") {
			return i
		}
	}
	return -1
}

// handleFileListLeft handles left/h key (VSCode-like):
// file → parent dir, expanded dir → collapse, collapsed dir → parent dir
func handleFileListLeft(ctx *FileListKeyContext) {
	if *ctx.currentSelection < 0 || *ctx.currentSelection >= len(*ctx.fileList) {
		return
	}
	fileEntry := (*ctx.fileList)[*ctx.currentSelection]

	if fileEntry.IsDirectory && ctx.dirCollapseState != nil {
		if !ctx.dirCollapseState.IsCollapsed(fileEntry.StageStatus, fileEntry.Path) {
			// 展開中のディレクトリ → 折りたたむ
			ctx.dirCollapseState.SetCollapsed(fileEntry.StageStatus, fileEntry.Path, true)
			ctx.updateFileListView()
			if *ctx.currentSelection >= len(*ctx.fileList) {
				*ctx.currentSelection = len(*ctx.fileList) - 1
				ctx.updateFileListView()
			}
			return
		}
		// 折りたたみ済みのディレクトリ → 親ディレクトリに移動
		if parent := findParentDirectory(ctx.fileList, *ctx.currentSelection); parent >= 0 {
			*ctx.currentSelection = parent
			ctx.updateFileListView()
		}
		return
	}

	// ファイル上 → 親ディレクトリに移動
	if parent := findParentDirectory(ctx.fileList, *ctx.currentSelection); parent >= 0 {
		*ctx.currentSelection = parent
		ctx.updateFileListView()
	}
}

// handleFileListRight handles right/l key (VSCode-like):
// collapsed dir → expand, expanded dir → first child, file → no-op
func handleFileListRight(ctx *FileListKeyContext) {
	if *ctx.currentSelection < 0 || *ctx.currentSelection >= len(*ctx.fileList) {
		return
	}
	fileEntry := (*ctx.fileList)[*ctx.currentSelection]

	if !fileEntry.IsDirectory || ctx.dirCollapseState == nil {
		return
	}

	if ctx.dirCollapseState.IsCollapsed(fileEntry.StageStatus, fileEntry.Path) {
		// 折りたたまれていたら展開
		ctx.dirCollapseState.SetCollapsed(fileEntry.StageStatus, fileEntry.Path, false)
		ctx.updateFileListView()
		return
	}

	// 展開済み → 次のエントリ（最初の子）に移動
	if *ctx.currentSelection+1 < len(*ctx.fileList) {
		*ctx.currentSelection = *ctx.currentSelection + 1
		ctx.updateFileListView()
		if !(*ctx.fileList)[*ctx.currentSelection].IsDirectory {
			ctx.updateSelectedFileDiff()
		}
	}
}

// buildFileTree converts a list of file paths into a tree structure
func buildFileTree(files []git.FileInfo) *TreeNode {
	return buildFileTreeFromGitFiles(files)
}

// renderFileTree recursively renders the tree structure with proper indentation
func renderFileTree(
	node *TreeNode,
	depth int,
	sb *strings.Builder,
	fileList *[]FileEntry,
	stageStatus string,
	regionIndex *int,
	currentSelection int,
	focusedPane bool,
	lineNumberMap map[int]int,
	currentLine *int,
	fileInfos []git.FileInfo,
	collapseState *DirCollapseState,
) {
	renderFileTreeForGitFiles(
		node,
		depth,
		"",
		sb,
		fileList,
		stageStatus,
		regionIndex,
		currentSelection,
		focusedPane,
		lineNumberMap,
		currentLine,
		fileInfos,
		collapseState,
	)
}

// BuildFileListContent builds the colored file list content
func BuildFileListContent(
	stagedFiles, modifiedFiles, untrackedFiles []git.FileInfo,
	currentSelection int,
	focusedPane bool,
	fileList *[]FileEntry,
	lineNumberMap map[int]int,
	collapseState *DirCollapseState,
) string {
	// fileList を再構築
	// スライスの中身をクリア（参照は保持）
	*fileList = (*fileList)[:0]
	for k := range lineNumberMap {
		delete(lineNumberMap, k)
	}

	var coloredContent strings.Builder
	regionIndex := 0
	currentLine := 0

	// Staged ファイル
	if len(stagedFiles) > 0 {
		coloredContent.WriteString("[green]Changes to be committed:[white]\n")
		currentLine++
		tree := buildFileTree(stagedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"staged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, stagedFiles, collapseState)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 変更されたファイル（unstaged）
	if len(modifiedFiles) > 0 {
		coloredContent.WriteString("[yellow]Changes not staged for commit:[white]\n")
		currentLine++
		tree := buildFileTree(modifiedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"unstaged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, modifiedFiles, collapseState)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 未追跡ファイル
	if len(untrackedFiles) > 0 {
		coloredContent.WriteString("[red]Untracked files:[white]\n")
		currentLine++
		tree := buildFileTree(untrackedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"untracked", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, untrackedFiles, collapseState)
	}

	return coloredContent.String()
}

// BuildFileListContentForCommit builds file list content from commit file entries
func BuildFileListContentForCommit(
	commitFiles []FileEntry,
	currentSelection int,
	focusedPane bool,
	fileList *[]FileEntry,
	lineNumberMap map[int]int,
	collapseState *DirCollapseState,
) string {
	*fileList = (*fileList)[:0]
	for k := range lineNumberMap {
		delete(lineNumberMap, k)
	}

	tree := buildFileTreeFromFileEntries(commitFiles)

	var content strings.Builder
	regionIndex := 0
	currentLine := 0

	renderFileTreeForFileEntries(
		tree,
		0,
		"",
		&content,
		fileList,
		"commit",
		&regionIndex,
		currentSelection,
		focusedPane,
		lineNumberMap,
		&currentLine,
		commitFiles,
		collapseState,
	)

	return content.String()
}

// FileListKeyContext contains all the context needed for file list key bindings
type FileListKeyContext struct {
	// UI Components
	fileListView    *tview.TextView
	diffView        *tview.TextView
	beforeView      *tview.TextView
	afterView       *tview.TextView
	splitViewFlex   *tview.Flex
	unifiedViewFlex *tview.Flex
	contentFlex     *tview.Flex
	app             *tview.Application
	mainView        tview.Primitive // メインビューの参照

	// State
	currentSelection  *int
	cursorY           *int
	isSelecting       *bool
	selectStart       *int
	selectEnd         *int
	isSplitView       *bool
	leftPaneFocused   *bool
	currentFile       *string
	currentStatus     *string
	currentDiffText   *string
	preserveScrollRow *int  // ファイルリストのスクロール位置を保持
	ignoreWhitespace  *bool // Whitespace無視モード

	// Collections
	fileList *[]FileEntry

	// Directory collapse state
	dirCollapseState *DirCollapseState

	// Paths
	repoRoot string

	// Diff view context
	diffViewContext *DiffViewContext

	// Mode
	readOnly bool // true の場合、staging/discard 系を無効化

	// Callbacks
	updateFileListView     func()
	updateSelectedFileDiff func()
	refreshFileList        func()
	updateCurrentDiffText  func(string, string, string, *string, bool)
	updateGlobalStatus     func(string, string)
	updateStatusTitle      func()
	onEsc                  func() // nil でない場合、Esc キーで呼び出す
}

// SetupFileListKeyBindings sets up key bindings for file list view
func SetupFileListKeyBindings(ctx *FileListKeyContext) {
	ctx.fileListView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if ctx.onEsc != nil {
				ctx.onEsc()
				return nil
			}
			return event
		case tcell.KeyUp:
			moveFileListSelection(ctx, -1)
			return nil
		case tcell.KeyDown:
			moveFileListSelection(ctx, 1)
			return nil
		case tcell.KeyLeft:
			handleFileListLeft(ctx)
			return nil
		case tcell.KeyRight:
			handleFileListRight(ctx)
			return nil
		case tcell.KeyEnter:
			if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
				fileEntry := (*ctx.fileList)[*ctx.currentSelection]

				// ディレクトリの場合は折りたたみトグル
				if fileEntry.IsDirectory {
					if ctx.dirCollapseState != nil {
						ctx.dirCollapseState.ToggleCollapsed(fileEntry.StageStatus, fileEntry.Path)
						ctx.updateFileListView()
						// currentSelection が範囲外になった場合の調整
						if *ctx.currentSelection >= len(*ctx.fileList) {
							*ctx.currentSelection = len(*ctx.fileList) - 1
							ctx.updateFileListView()
						}
					}
					return nil
				}

				file := fileEntry.Path
				status := fileEntry.StageStatus

				// 同じファイルかどうかをチェック
				sameFile := (*ctx.currentFile == file && *ctx.currentStatus == status)

				// 現在のファイル情報を更新
				*ctx.currentFile = file
				*ctx.currentStatus = status

				// 異なるファイルの場合のみカーソルと選択をリセット
				if !sameFile {
					*ctx.cursorY = 0
					*ctx.isSelecting = false
					*ctx.selectStart = -1
					*ctx.selectEnd = -1

					ctx.updateCurrentDiffText(file, status, ctx.repoRoot, ctx.currentDiffText, *ctx.ignoreWhitespace)
				}

				// viewerを更新（カーソル表示のため）
				if *ctx.isSplitView {
					updateSplitViewWithCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.cursorY, *ctx.currentFile)
				} else {
					foldState := ctx.diffViewContext.foldState
					updateDiffViewWithCursor(ctx.diffView, *ctx.currentDiffText, *ctx.cursorY, foldState, *ctx.currentFile, ctx.repoRoot)
				}

				// viewerに移動する前にスクロール位置を保存
				if ctx.preserveScrollRow != nil {
					currentRow, _ := ctx.fileListView.GetScrollOffset()
					*ctx.preserveScrollRow = currentRow
				}

				// フォーカスを右ペインに移動
				*ctx.leftPaneFocused = false
				if restoreStatusFunc != nil {
					restoreStatusFunc()
				}
				ctx.updateFileListView()
				if *ctx.isSplitView {
					ctx.app.SetFocus(ctx.splitViewFlex)
				} else {
					ctx.app.SetFocus(ctx.diffView)
				}
			}
			return nil
		case tcell.KeyCtrlE:
			// Ctrl+E: diff view を1行下にスクロール
			if ctx.diffViewContext != nil {
				scrollDiffView(ctx.diffViewContext, 1)
			}
			return nil
		case tcell.KeyCtrlY:
			// Ctrl+Y: diff view を1行上にスクロール
			if ctx.diffViewContext != nil {
				scrollDiffView(ctx.diffViewContext, -1)
			}
			return nil
		case tcell.KeyCtrlA:
			if ctx.readOnly {
				return nil
			}
			cmd := exec.Command("git", "-c", "core.quotepath=false", "add", "--all")
			cmd.Dir = ctx.repoRoot
			if err := cmd.Run(); err != nil {
				if ctx.updateGlobalStatus != nil {
					ctx.updateGlobalStatus("Failed to stage all files", "tomato")
				}
				return nil
			}

			ctx.refreshFileList()
			ctx.updateFileListView()
			if len(*ctx.fileList) == 0 {
				*ctx.currentSelection = 0
			} else if *ctx.currentSelection >= len(*ctx.fileList) {
				*ctx.currentSelection = len(*ctx.fileList) - 1
				ctx.updateFileListView()
			}
			ctx.updateSelectedFileDiff()
			if ctx.updateGlobalStatus != nil {
				ctx.updateGlobalStatus("Staged all files", "forestgreen")
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				moveFileListSelection(ctx, -1)
				return nil
			case 'j':
				moveFileListSelection(ctx, 1)
				return nil
			case 'H':
				handleFileListLeft(ctx)
				return nil
			case 'L':
				handleFileListRight(ctx)
				return nil
			case 's':
				// Split Viewのトグル
				*ctx.isSplitView = !*ctx.isSplitView

				if *ctx.isSplitView {
					// Split Viewを表示
					updateSplitViewWithoutCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.currentFile)
					ctx.contentFlex.RemoveItem(ctx.unifiedViewFlex)
					ctx.contentFlex.AddItem(ctx.splitViewFlex, 0, DiffViewFlexRatio, false)
					// viewUpdaterをSplitView用に更新
					if ctx.diffViewContext != nil {
						ctx.diffViewContext.viewUpdater = NewSplitViewUpdater(ctx.beforeView, ctx.afterView, ctx.currentFile)
					}
				} else {
					// 通常の差分表示に戻す
					ctx.contentFlex.RemoveItem(ctx.splitViewFlex)
					ctx.contentFlex.AddItem(ctx.unifiedViewFlex, 0, DiffViewFlexRatio, false)
					foldState := ctx.diffViewContext.foldState
					updateDiffViewWithoutCursor(ctx.diffView, *ctx.currentDiffText, foldState, *ctx.currentFile, ctx.repoRoot)
					// viewUpdaterをUnifiedView用に更新
					if ctx.diffViewContext != nil {
						ctx.diffViewContext.viewUpdater = NewUnifiedViewUpdater(ctx.diffView, foldState, ctx.currentFile, ctx.repoRoot)
					}
				}
				return nil
			case 'Y': // Shift+Y でファイルパスをコピー
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]
					err := commands.CopyFilePath(fileEntry.Path)
					if ctx.updateGlobalStatus != nil {
						if err == nil {
							ctx.updateGlobalStatus("Copied path to clipboard", "forestgreen")
						} else {
							ctx.updateGlobalStatus("Failed to copy path to clipboard", "tomato")
						}
					}
				}
				return nil
			case 'w':
				// Whitespace無視モードのトグル
				*ctx.ignoreWhitespace = !*ctx.ignoreWhitespace

				// 差分を再取得
				if *ctx.currentFile != "" {
					ctx.updateCurrentDiffText(*ctx.currentFile, *ctx.currentStatus, ctx.repoRoot, ctx.currentDiffText, *ctx.ignoreWhitespace)
				}

				// 表示を更新
				if len(strings.TrimSpace(*ctx.currentDiffText)) == 0 {
					if *ctx.isSplitView {
						ctx.beforeView.SetText("")
						ctx.afterView.SetText("No differences")
					} else {
						ctx.diffView.SetText("No differences")
					}
				} else if *ctx.isSplitView {
					updateSplitViewWithoutCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.currentFile)
				} else {
					foldState := ctx.diffViewContext.foldState
					updateDiffViewWithoutCursor(ctx.diffView, *ctx.currentDiffText, foldState, *ctx.currentFile, ctx.repoRoot)
				}

				// ステータスタイトルを更新
				if ctx.updateStatusTitle != nil {
					ctx.updateStatusTitle()
				}

				if *ctx.ignoreWhitespace {
					ctx.updateGlobalStatus("Whitespace changes hidden", "forestgreen")
				} else {
					ctx.updateGlobalStatus("Whitespace changes shown", "forestgreen")
				}
				return nil
			case 'a': // 'a' で現在のファイル/ディレクトリを git add/reset
				if ctx.readOnly {
					return nil
				}
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]
					file := fileEntry.Path
					status := fileEntry.StageStatus

					// ディレクトリの場合はディレクトリごと stage/unstage
					if fileEntry.IsDirectory {
						file = fileEntry.Path + "/"
					}

					var cmd *exec.Cmd
					if status == "staged" {
						// stagedファイルをunstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", "--", file)
						cmd.Dir = ctx.repoRoot
					} else {
						// unstaged/untrackedファイルをstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", file)
						cmd.Dir = ctx.repoRoot
					}

					// Git インデックスのロック競合を考慮してリトライ
					var err error
					for retry := 0; retry < 3; retry++ {
						err = cmd.Run()
						if err == nil {
							break
						}
						// リトライ前に少し待機
						time.Sleep(50 * time.Millisecond)
						// コマンドを再作成（Cmdは一度実行すると再利用できない）
						if status == "staged" {
							cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", "--", file)
						} else {
							cmd = exec.Command("git", "-c", "core.quotepath=false", "add", file)
						}
						cmd.Dir = ctx.repoRoot
					}
					if err != nil {
						if ctx.updateGlobalStatus != nil {
							if status == "staged" {
								ctx.updateGlobalStatus("Failed to unstage file. Please retry.", "tomato")
							} else {
								ctx.updateGlobalStatus("Failed to stage file. Please retry.", "tomato")
							}
						}
						return nil
					}

					// 現在のスクロール位置を保存
					currentRow, _ := ctx.fileListView.GetScrollOffset()
					if ctx.preserveScrollRow != nil {
						*ctx.preserveScrollRow = currentRow
					}

					// 現在のカーソル位置の次の移動先を決定
					// ファイル → 次のファイル、ディレクトリ → 同じディレクトリ
					var nextTarget string
					nextIsDir := fileEntry.IsDirectory
					if fileEntry.IsDirectory {
						// ディレクトリの場合は同じディレクトリを探す
						nextTarget = fileEntry.Path
					} else {
						// ファイルの場合は次のファイルを探す
						for ni := *ctx.currentSelection + 1; ni < len(*ctx.fileList); ni++ {
							if !(*ctx.fileList)[ni].IsDirectory {
								nextTarget = (*ctx.fileList)[ni].Path
								break
							}
						}
					}

					// ファイルリストを更新
					ctx.refreshFileList()
					ctx.updateFileListView()

					// 移動先をパスで探す（ステータスは変わる可能性があるため無視）
					foundNext := false
					if nextTarget != "" {
						for i, fe := range *ctx.fileList {
							if fe.Path == nextTarget && fe.IsDirectory == nextIsDir {
								*ctx.currentSelection = i
								foundNext = true
								break
							}
						}
					}

					if !foundNext {
						if *ctx.currentSelection >= len(*ctx.fileList) {
							*ctx.currentSelection = len(*ctx.fileList) - 1
						}
					}

					// スクロール位置を保持して画面を更新
					if ctx.preserveScrollRow != nil {
						*ctx.preserveScrollRow = currentRow
					}
					ctx.updateFileListView()
					ctx.updateSelectedFileDiff()
				}
				return nil
			case 'd': // 'd' で選択したファイルの差分を破棄（untracked fileの場合は削除）
				if ctx.readOnly {
					return nil
				}
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]

					// ディレクトリの場合はスキップ
					if fileEntry.IsDirectory {
						if ctx.updateGlobalStatus != nil {
							ctx.updateGlobalStatus("Cannot discard directory. Select individual files.", "tomato")
						}
						return nil
					}

					// stagedファイルの場合はエラーメッセージを表示
					if fileEntry.StageStatus == "staged" {
						if ctx.updateGlobalStatus != nil {
							ctx.updateGlobalStatus("Cannot discard staged changes. Use 'a' to unstage first.", "tomato")
						}
						return nil
					}

					// 確認メッセージを設定
					var confirmMsg string
					var buttonLabel string
					if fileEntry.StageStatus == "untracked" {
						confirmMsg = "Delete " + fileEntry.Path + "?"
						buttonLabel = "Delete"
					} else {
						confirmMsg = "Discard changes in " + fileEntry.Path + "?"
						buttonLabel = "Discard"
					}

					// 小さい確認モーダルを作成
					modal := tview.NewModal().
						SetText(confirmMsg).
						AddButtons([]string{buttonLabel, "Cancel"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							if buttonLabel == "Discard" || buttonLabel == "Delete" {
								params := commands.CommandDParams{
									CurrentFile:   fileEntry.Path,
									CurrentStatus: fileEntry.StageStatus,
									RepoRoot:      ctx.repoRoot,
								}

								err := commands.CommandD(params)
								if err != nil {
									if ctx.updateGlobalStatus != nil {
										ctx.updateGlobalStatus(err.Error(), "tomato")
									}
								} else {
									ctx.refreshFileList()
									ctx.updateFileListView()
									ctx.updateSelectedFileDiff()
									if ctx.updateGlobalStatus != nil {
										if fileEntry.StageStatus == "untracked" {
											ctx.updateGlobalStatus("File deleted successfully!", "forestgreen")
										} else {
											ctx.updateGlobalStatus("Changes discarded successfully!", "forestgreen")
										}
									}
								}
							}
							// 元のビューに戻る
							ctx.app.SetRoot(ctx.mainView, true)
							ctx.app.SetFocus(ctx.fileListView)
						})

					// mainViewを全画面で表示し、その上にmodalを重ねて表示
					pages := tview.NewPages().
						AddPage("main", ctx.mainView, true, true).
						AddPage("modal", modal, true, true)

					ctx.app.SetRoot(pages, true)
				}
				return nil
			case 'v': // 'v' でvimでファイルを開く
				if ctx.readOnly {
					return nil
				}
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]

					// ディレクトリの場合はスキップ
					if fileEntry.IsDirectory {
						if ctx.updateGlobalStatus != nil {
							ctx.updateGlobalStatus("Cannot open directory in vim. Select a file.", "tomato")
						}
						return nil
					}

					filePath := fileEntry.Path

					// アプリケーションを一時停止してvimを起動
					ctx.app.Suspend(func() {
						cmd := exec.Command("vim", "-c", "set title titlestring=[giff]\\ %f", filePath)
						cmd.Dir = ctx.repoRoot
						cmd.Stdin = os.Stdin
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						cmd.Run()
					})

					// vimから戻ったらファイルリストを更新
					ctx.refreshFileList()
					ctx.updateFileListView()
					ctx.updateSelectedFileDiff()
				}
				return nil
			case 'c': // 'c' でVSCodeでファイルを開く
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]
					if fileEntry.IsDirectory {
						if ctx.updateGlobalStatus != nil {
							ctx.updateGlobalStatus("Cannot open directory in VSCode. Select a file.", "tomato")
						}
						return nil
					}
					cmd := exec.Command("code", fileEntry.Path)
					cmd.Dir = ctx.repoRoot
					if err := cmd.Start(); err != nil {
						if ctx.updateGlobalStatus != nil {
							ctx.updateGlobalStatus("Failed to open VSCode", "tomato")
						}
					}
				}
				return nil
			case 't': // 't' でgit logビューを表示
				if ctx.readOnly {
					return nil
				}
				// Git Log Viewを作成
				gitLogView := NewGitLogView(ctx.app, ctx.repoRoot, func() {
					// Git Log Viewを終了して元のビューに戻る
					ctx.app.SetRoot(ctx.mainView, true)
					ctx.app.SetFocus(ctx.fileListView)
				})

				// Git Log Viewに切り替え
				ctx.app.SetRoot(gitLogView.GetView(), true)
				return nil
			case 'q': // 'q' でアプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				ctx.app.Stop()
			}
		}
		return event
	})
}
