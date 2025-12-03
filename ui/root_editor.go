package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/util"
)

// 保持するカーソル情報
var savedTargetFile string = ""
var preferUnstagedSection bool = false

// globalStatusView をグローバルに定義
var globalStatusView *tview.TextView
var listKeyBindingMessage = "Press 'Enter' to switch panes, 'q' to quit, 'a' to stage selected lines, 'A' to stage/unstage file, 'V' to select lines, 'Ctrl+A' to stage all files, 'Ctrl+K' to commit, 'Ctrl+J' to amend, and 'j/k' to navigate."

func updateGlobalStatus(message string, color string) {
	if globalStatusView != nil {
		globalStatusView.SetText(fmt.Sprintf("[%s]%s[-]", color, message))
		go func() {
			time.Sleep(5 * time.Second)
			globalStatusView.SetText(listKeyBindingMessage)
		}()
	}
}

// ファイルの差分を取得
func updateCurrentDiffText(filePath string, status string, repoRoot string, currentDiffText *string) {
	var diffText string
	var err error

	// ディレクトリかどうかをチェック
	fullPath := filepath.Join(repoRoot, filePath)
	fileInfo, statErr := os.Stat(fullPath)
	if statErr == nil && fileInfo.IsDir() {
		// ディレクトリの場合は何もしない
		return
	}

	switch status {
	case "staged":
		// stagedファイルの場合はstaged差分を取得
		diffText, err = git.GetStagedDiff(filePath, repoRoot)
	case "untracked":
		// untrackedファイルの場合はファイル内容を読み取って全て追加として表示
		content, readErr := util.ReadFileContent(filePath, repoRoot)
		if readErr != nil {
			err = readErr
		} else {
			diffText = util.FormatAsAddedLines(content, filePath)
		}
	default:
		// unstagedファイルの場合は通常の差分を取得
		diffText, err = git.GetFileDiff(filePath, repoRoot)
	}

	if err != nil {
		// エラーが発生した場合でも適切なメッセージを表示
		diffText = fmt.Sprintf("Error getting diff for %s: %v\n\nThis might be a deleted file or there might be an issue with git.", filePath, err)
	}

	// 生の差分テキストを保存
	if currentDiffText != nil {
		*currentDiffText = diffText
	}
}

func RootEditor(app *tview.Application, stagedFiles, modifiedFiles, untrackedFiles []git.FileInfo, repoRoot string, patchFilePath string, onUpdate func(), enableAutoRefresh bool) tview.Primitive {
	// ファイルリストを更新するための参照を保持
	stagedFilesPtr := &stagedFiles
	modifiedFilesPtr := &modifiedFiles
	untrackedFilesPtr := &untrackedFiles

	// コミット関連の状態
	var isCommitMode bool = false
	var isAmendMode bool = false
	var commitMessage string = ""
	// 現在のファイル情報を保持
	var currentFile string
	var currentStatus string
	var currentDiffText string
	var cursorY int = 0
	var selectStart int = -1
	var selectEnd int = -1
	var isSelecting bool = false
	var currentSelection int = 0
	var leftPaneFocused bool = true
	var gPressed bool = false
	var lastGTime time.Time
	var isSplitView bool = false // Split Viewモードのフラグ

	// Fold state for managing expandable ranges
	foldState := NewFoldState()

	// listStatusView を作成
	globalStatusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)
	globalStatusView.SetBorder(true)
	globalStatusView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// フレックスレイアウトを作成（上下分割、その下に左右分割）
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	// 左ペイン（ファイルリスト）のテキストビューを作成
	fileListView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	fileListView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// 右ペイン（差分表示）のテキストビューを作成
	diffView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	diffView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// Unified View用のフレックスコンテナ（diffViewと右端の縦線を含む）
	unifiedViewFlex := tview.NewFlex().
		AddItem(diffView, 0, 1, false).
		AddItem(CreateVerticalBorder(), 1, 0, false)
	unifiedViewFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// Split View用のテキストビューを作成
	beforeView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	beforeView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	afterView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	afterView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// Split View用のフレックスコンテナ
	splitViewFlex := tview.NewFlex().
		AddItem(beforeView, 0, 1, false).
		AddItem(CreateVerticalBorder(), 1, 0, false).
		AddItem(afterView, 0, 1, false).
		AddItem(CreateVerticalBorder(), 1, 0, false)
	splitViewFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// 保存されたカーソル位置を復元
	if preferUnstagedSection || savedTargetFile != "" {
		// カーソル位置を計算
		targetSelection := 0
		foundTarget := false

		// 全ファイルを走査
		for _, fileInfo := range *stagedFilesPtr {
			if strings.TrimSpace(fileInfo.Path) != "" {
				if !preferUnstagedSection && fileInfo.Path == savedTargetFile {
					currentSelection = targetSelection
					foundTarget = true
					break
				}
				targetSelection++
			}
		}

		if !foundTarget {
			// unstagedセクションの開始位置
			unstagedStart := targetSelection

			for _, fileInfo := range *modifiedFilesPtr {
				if strings.TrimSpace(fileInfo.Path) != "" {
					if preferUnstagedSection && targetSelection == unstagedStart {
						// unstagedセクションの最初のファイル
						currentSelection = targetSelection
						foundTarget = true
						break
					} else if !preferUnstagedSection && fileInfo.Path == savedTargetFile {
						currentSelection = targetSelection
						foundTarget = true
						break
					}
					targetSelection++
				}
			}
		}

		// カーソル情報をリセット
		preferUnstagedSection = false
		savedTargetFile = ""
	}

	// ファイル一覧を構築するための変数
	var fileList []FileEntry
	var lineNumberMap = make(map[int]int)

	// ボーダーを作成
	verticalBorder := CreateVerticalBorder()
	horizontalTopBorder := CreateHorizontalTopBorder()
	horizontalBottomBorder := CreateHorizontalBottomBorder()
	verticalBorderLeft := CreateVerticalBorder()

	// コミットメッセージ入力エリア
	commitTextArea := tview.NewTextArea().
		SetPlaceholder("Enter commit message (Press Option+Enter to commit, Ctrl+L to focus file list, Esc to cancel)")

	// TextAreaのスタイル設定
	// テキストスタイル（入力される文字）
	commitTextArea.SetTextStyle(tcell.StyleDefault.
		Foreground(util.MainTextColor.ToTcellColor()).
		Background(util.BackgroundColor.ToTcellColor()))

	// プレースホルダーのスタイル
	commitTextArea.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(util.PlaceholderColor.ToTcellColor()).
		Background(util.BackgroundColor.ToTcellColor()))

	// 背景色とボーダー設定
	commitTextArea.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	commitTextArea.SetBorder(true)
	commitTextArea.SetBorderColor(util.CommitAreaBorderColor.ToTcellColor())
	commitTextArea.SetTitle("Commit Message")
	commitTextArea.SetTitleAlign(tview.AlignLeft)
	commitTextArea.SetTitleColor(tcell.ColorWhite)

	// 左右分割のフレックス
	contentFlex := tview.NewFlex()
	contentFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// レイアウト設定
	// 左右のペインをフレックスに追加（左:縦線:右 = FileListFlexRatio:0:DiffViewFlexRatio）
	// 右側の縦線は unifiedViewFlex と splitViewFlex で定義している
	contentFlex.
		AddItem(verticalBorderLeft, 1, 0, false).
		AddItem(fileListView, 0, FileListFlexRatio, true).
		AddItem(verticalBorder, 1, 0, false).
		AddItem(unifiedViewFlex, 0, DiffViewFlexRatio, false)

	// ファイル一覧の内容を構築（色付き）
	buildFileListContent := func(focusedPane bool) string {
		return BuildFileListContent(
			*stagedFilesPtr,
			*modifiedFilesPtr,
			*untrackedFilesPtr,
			currentSelection,
			focusedPane,
			&fileList,
			lineNumberMap,
		)
	}

	// 初期表示を更新
	updateFileListView := func() {
		// 現在の横スクロール位置を保存
		_, currentCol := fileListView.GetScrollOffset()

		fileListView.Clear()
		fileListView.SetText(buildFileListContent(leftPaneFocused))

		// 最初のファイルが選択されている場合は一番上にスクロール（横スクロール位置は維持）
		if currentSelection == 0 {
			fileListView.ScrollTo(0, currentCol)
			return
		}

		// スクロール位置を調整（選択行が見える範囲に）
		if actualLine, exists := lineNumberMap[currentSelection]; exists {
			_, _, _, height := fileListView.GetInnerRect()
			currentRow, _ := fileListView.GetScrollOffset()

			// 選択行が画面より下にある場合
			if actualLine >= currentRow+height-1 {
				fileListView.ScrollTo(actualLine-height+2, currentCol)
			}
			// 選択行が画面より上にある場合
			if actualLine < currentRow {
				fileListView.ScrollTo(actualLine, currentCol)
			}
		}
	}

	// ファイルリストを内部的に更新する関数
	refreshFileList := func() {
		// 新しいファイルリストを取得
		newStaged, newModified, newUntracked, err := git.GetChangedFiles(repoRoot)
		if err == nil {
			*stagedFilesPtr = newStaged
			*modifiedFilesPtr = newModified
			*untrackedFilesPtr = newUntracked
		}
	}

	// 選択されているファイルの差分を更新する関数
	updateSelectedFileDiff := func() {
		// 選択範囲を調整
		if len(fileList) == 0 {
			// ファイルリストが空の場合
			currentDiffText = "No file content ✨"
			currentFile = ""
			currentStatus = ""
			if isSplitView {
				beforeView.SetText("")
				afterView.SetText("No file content ✨")
			} else {
				diffView.SetText("No file content ✨")
			}
			return
		}

		// currentSelectionが範囲外の場合は調整
		if currentSelection < 0 {
			currentSelection = 0
		} else if currentSelection >= len(fileList) {
			currentSelection = len(fileList) - 1
		}

		fileEntry := fileList[currentSelection]
		file := fileEntry.Path
		status := fileEntry.StageStatus

		// 現在のファイル情報を更新
		currentFile = file
		currentStatus = status

		// カーソルと選択をリセット
		cursorY = 0
		isSelecting = false
		selectStart = -1
		selectEnd = -1

		updateCurrentDiffText(file, status, repoRoot, &currentDiffText)

		if isSplitView {
			updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
		} else {
			updateDiffViewWithoutCursor(diffView, currentDiffText, foldState, currentFile, repoRoot)
		}
	}

	updateFileListView()

	// 初期表示時に最初のファイルの差分を表示
	updateSelectedFileDiff()

	// 初期メッセージを設定
	globalStatusView.SetText(listKeyBindingMessage)

	// 右ペインのキー入力処理を設定（file_view.goと同じ動作）
	// diffViewのキーバインディングを設定
	diffViewContext := &DiffViewContext{
		// UI Components
		diffView:        diffView,
		fileListView:    fileListView,
		beforeView:      beforeView,
		afterView:       afterView,
		splitViewFlex:   splitViewFlex,
		unifiedViewFlex: unifiedViewFlex,
		contentFlex:     contentFlex,
		app:             app,

		// State
		cursorY:               &cursorY,
		selectStart:           &selectStart,
		selectEnd:             &selectEnd,
		isSelecting:           &isSelecting,
		isSplitView:           &isSplitView,
		leftPaneFocused:       &leftPaneFocused,
		currentDiffText:       &currentDiffText,
		currentFile:           &currentFile,
		currentStatus:         &currentStatus,
		savedTargetFile:       &savedTargetFile,
		preferUnstagedSection: &preferUnstagedSection,
		currentSelection:      &currentSelection,
		fileList:              &fileList,

		// Paths
		repoRoot:  repoRoot,
		patchPath: patchFilePath,

		// Key handling state
		gPressed:  &gPressed,
		lastGTime: &lastGTime,

		// View updater
		viewUpdater: NewUnifiedViewUpdater(diffView, foldState, &currentFile, repoRoot),

		// Fold state
		foldState: foldState,

		// Callbacks
		updateFileListView: updateFileListView,
		updateGlobalStatus: updateGlobalStatus,
		refreshFileList:    refreshFileList,
		onUpdate:           onUpdate,
	}
	SetupDiffViewKeyBindings(diffViewContext)

	// ファイルリストのキーバインディングを設定（一時的にnilを設定）
	fileListKeyContext := &FileListKeyContext{
		// UI Components
		fileListView:    fileListView,
		diffView:        diffView,
		beforeView:      beforeView,
		afterView:       afterView,
		splitViewFlex:   splitViewFlex,
		unifiedViewFlex: unifiedViewFlex,
		contentFlex:     contentFlex,
		app:             app,
		mainView:        mainFlex, // メインビューの参照を追加

		// State
		currentSelection: &currentSelection,
		cursorY:          &cursorY,
		isSelecting:      &isSelecting,
		selectStart:      &selectStart,
		selectEnd:        &selectEnd,
		isSplitView:      &isSplitView,
		leftPaneFocused:  &leftPaneFocused,
		currentFile:      &currentFile,
		currentStatus:    &currentStatus,
		currentDiffText:  &currentDiffText,

		// Collections
		fileList: &fileList,

		// Paths
		repoRoot: repoRoot,

		// Diff view context
		diffViewContext: diffViewContext, // 後で設定

		// Callbacks
		updateFileListView:     updateFileListView,
		updateSelectedFileDiff: updateSelectedFileDiff,
		refreshFileList:        refreshFileList,
		updateCurrentDiffText:  updateCurrentDiffText,
		updateGlobalStatus:     updateGlobalStatus,
	}
	SetupFileListKeyBindings(fileListKeyContext)

	// 自動リフレッシュが有効な場合のみゴルーチンを開始
	if enableAutoRefresh {
		stopRefresh := make(chan bool)

		// 前回のファイルリストをキャッシュするための変数
		var lastStagedFiles, lastModifiedFiles, lastUntrackedFiles []git.FileInfo

		// ファイルリストが変更されたかどうかを判定する関数
		hasFileListChanged := func(newStaged, newModified, newUntracked []git.FileInfo) bool {
			if len(lastStagedFiles) != len(newStaged) ||
				len(lastModifiedFiles) != len(newModified) ||
				len(lastUntrackedFiles) != len(newUntracked) {
				return true
			}

			// 各ファイルリストの内容を比較
			for i := range newStaged {
				if lastStagedFiles[i].Path != newStaged[i].Path ||
					lastStagedFiles[i].ChangeStatus != newStaged[i].ChangeStatus {
					return true
				}
			}

			for i := range newModified {
				if lastModifiedFiles[i].Path != newModified[i].Path ||
					lastModifiedFiles[i].ChangeStatus != newModified[i].ChangeStatus {
					return true
				}
			}

			for i := range newUntracked {
				if lastUntrackedFiles[i].Path != newUntracked[i].Path ||
					lastUntrackedFiles[i].ChangeStatus != newUntracked[i].ChangeStatus {
					return true
				}
			}

			return false
		}

		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					// 新しいファイルリストを取得
					newStaged, newModified, newUntracked, err := git.GetChangedFiles(repoRoot)
					if err != nil {
						continue // エラーが発生した場合は何もしない
					}

					// 現在選択中のファイルの差分変更もチェック
					var currentFileDiffChanged bool = false
					var newDiffText string
					if currentFile != "" && !leftPaneFocused {
						if currentStatus == "staged" {
							newDiffText, _ = git.GetStagedDiff(currentFile, repoRoot)
						} else if currentStatus == "untracked" {
							content, readErr := util.ReadFileContent(currentFile, repoRoot)
							if readErr == nil {
								newDiffText = util.FormatAsAddedLines(content, currentFile)
							}
						} else {
							newDiffText, _ = git.GetFileDiff(currentFile, repoRoot)
						}
						currentFileDiffChanged = (newDiffText != currentDiffText)
					}

					// ファイルリストに変更があるかチェック
					fileListChanged := hasFileListChanged(newStaged, newModified, newUntracked)

					// ファイルリストの変更も現在のファイルの差分変更もない場合は何もしない
					if !fileListChanged && !currentFileDiffChanged {
						continue
					}

					app.QueueUpdateDraw(func() {

						// 現在の選択ファイルとステータスを保存
						var currentlySelectedFile string
						var currentlySelectedStatus string
						if currentSelection >= 0 && currentSelection < len(fileList) {
							fileEntry := fileList[currentSelection]
							currentlySelectedFile = fileEntry.Path
							currentlySelectedStatus = fileEntry.StageStatus
						}

						// ファイルリストが変更された場合のみキャッシュとリストを更新
						if fileListChanged {
							// キャッシュを更新
							lastStagedFiles = newStaged
							lastModifiedFiles = newModified
							lastUntrackedFiles = newUntracked

							// ファイルリストを更新
							*stagedFilesPtr = newStaged
							*modifiedFilesPtr = newModified
							*untrackedFilesPtr = newUntracked

							// 選択位置を復元（ファイル名とステータスの両方で検索）
							newSelection := -1
							for i, fileEntry := range fileList {
								if fileEntry.Path == currentlySelectedFile && fileEntry.StageStatus == currentlySelectedStatus {
									newSelection = i
									break
								}
							}
							if newSelection >= 0 {
								currentSelection = newSelection
							} else if currentSelection >= len(fileList) {
								currentSelection = len(fileList) - 1
							}

							// 表示を更新
							updateFileListView()
						}

						// 右ペインの差分更新
						if leftPaneFocused {
							// 左ペインにフォーカスがある場合は通常の更新（ファイルリストが変更された場合のみ）
							if fileListChanged {
								updateSelectedFileDiff()
							}
						} else if currentFile != "" {
							// 右ペインにフォーカスがある場合は現在のカーソル位置を保持して更新
							var newDiffText string
							if currentStatus == "staged" {
								newDiffText, _ = git.GetStagedDiff(currentFile, repoRoot)
							} else if currentStatus == "untracked" {
								content, readErr := util.ReadFileContent(currentFile, repoRoot)
								if readErr == nil {
									newDiffText = util.FormatAsAddedLines(content, currentFile)
								}
							} else {
								newDiffText, _ = git.GetFileDiff(currentFile, repoRoot)
							}

							// 差分が変更された場合のみ更新
							if newDiffText != currentDiffText {
								currentDiffText = newDiffText

								// 空の差分の場合の処理
								if len(currentDiffText) == 0 {
									if isSplitView {
										beforeView.SetText("")
										afterView.SetText("No differences")
									} else {
										diffView.SetText("No differences")
									}
									return
								}

								coloredDiff := ColorizeDiff(currentDiffText)
								diffLines := util.SplitLines(coloredDiff)

								// カーソル位置を調整（差分の行数が減った場合）
								if cursorY >= len(diffLines) {
									cursorY = len(diffLines) - 1
									if cursorY < 0 {
										cursorY = 0
									}
								}

								// 選択範囲も調整
								if isSelecting {
									if selectStart >= len(diffLines) {
										selectStart = len(diffLines) - 1
									}
									if selectEnd >= len(diffLines) {
										selectEnd = len(diffLines) - 1
									}
								}

								// Split Viewモードの場合はSplit View更新、そうでなければ通常の更新
								if isSplitView {
									updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
								} else {
									updateDiffViewWithCursor(diffView, currentDiffText, cursorY, foldState, currentFile, repoRoot)
								}
							}
						}
					})
				case <-stopRefresh:
					return
				}
			}
		}()
	}

	exitCommitMode := func() {
		isCommitMode = false
		isAmendMode = false
		leftPaneFocused = true
		commitTextArea.SetText("", false)
		commitTextArea.SetTitle("Commit Message")
		mainFlex.RemoveItem(commitTextArea)
		app.SetFocus(fileListView)
	}

	commitTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			// Option+Enter (Alt+Enter) でコミット実行
			if event.Modifiers()&tcell.ModAlt != 0 {
				commitMessage = commitTextArea.GetText()
				if commitMessage == "" {
					updateGlobalStatus("Commit message cannot be empty", "tomato")
					return nil
				}

				var err error
				if isAmendMode {
					err = git.CommitAmend(commitMessage, repoRoot)
				} else {
					err = git.Commit(commitMessage, repoRoot)
				}
				if err != nil {
					if isAmendMode {
						updateGlobalStatus("Failed to amend commit: "+err.Error(), "tomato")
					} else {
						updateGlobalStatus("Failed to commit: "+err.Error(), "tomato")
					}
					// エラー時はコミットモードを終了せず、メッセージを保持したまま再試行できるようにする
					return nil
				}

				if isAmendMode {
					updateGlobalStatus("Successfully amended commit", "forestgreen")
				} else {
					updateGlobalStatus("Successfully committed", "forestgreen")
				}
				// コミット後にファイルリストを更新
				refreshFileList()

				// ファイルリストが更新された後、選択位置を調整
				// fileListの再構築のためにupdateFileListViewを呼ぶ
				updateFileListView()

				// 選択位置が範囲外の場合は調整
				if len(fileList) > 0 {
					// ファイルがまだ残っている場合
					if currentSelection >= len(fileList) {
						currentSelection = len(fileList) - 1
					}
					if currentSelection < 0 {
						currentSelection = 0
					}
				} else {
					// ファイルリストが空になった場合
					currentSelection = 0
				}

				// 再度ビューを更新して正しい選択位置を反映
				updateFileListView()
				updateSelectedFileDiff()

				exitCommitMode()
				return nil
			}
			// 通常のEnterは改行として処理
			return event
		case tcell.KeyCtrlL:
			app.SetFocus(fileListView)
			return nil
		case tcell.KeyEsc:
			exitCommitMode()
			return nil
		}
		return event
	})

	// mainFlex にステータスビューとコンテンツを追加
	mainFlex.AddItem(globalStatusView, 5, 0, false).
		AddItem(horizontalTopBorder, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(horizontalBottomBorder, 1, 0, false)

	mainFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlK {
			// staged な差分があるかチェック
			if len(*stagedFilesPtr) == 0 {
				updateGlobalStatus("No changes are staged for commit", "tomato")
				return nil
			}

			if !isCommitMode {
				isCommitMode = true
				isAmendMode = false
				mainFlex.AddItem(commitTextArea, 7, 0, true) // TextAreaは高さを7に増やして複数行に対応
				app.SetFocus(commitTextArea)
			} else {
				app.SetFocus(commitTextArea)
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlJ {
			// 最新のコミットメッセージを取得
			cmd := exec.Command("git", "log", "-1", "--pretty=%B")
			cmd.Dir = repoRoot
			output, err := cmd.Output()
			var lastCommitMsg string
			if err == nil {
				lastCommitMsg = strings.TrimSpace(string(output))
			}

			if !isCommitMode {
				isCommitMode = true
				isAmendMode = true
				commitTextArea.SetTitle("Commit Message (Amend)")
				commitTextArea.SetText(lastCommitMsg, false)
				mainFlex.AddItem(commitTextArea, 7, 0, true)
				app.SetFocus(commitTextArea)
			} else {
				app.SetFocus(commitTextArea)
			}
			return nil
		}
		return event
	})

	return mainFlex
}
