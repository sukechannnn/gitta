package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/util"
)

// 保持するカーソル情報
var savedTargetFile string = ""
var preferUnstagedSection bool = false

// パッチファイルのパス
var patchPath = "/tmp/gitta_selected.patch"

// listStatusView をグローバルに定義
var listStatusView *tview.TextView
var listKeyBindingMessage = "Press 'w' to switch panes, 'q' to quit, 'a' to stage selected lines, 'A' to stage/unstage file, 'V' to select lines, and 'j/k' to navigate."

func updateListStatus(message string, color string) {
	if listStatusView != nil {
		listStatusView.SetText(fmt.Sprintf("[%s]%s[-]", color, message))
		go func() {
			time.Sleep(5 * time.Second)
			listStatusView.SetText(listKeyBindingMessage)
		}()
	}
}

// ファイルの差分を取得
func updateCurrentDiffText(filePath string, status string, repoRoot string, currentDiffText *string) {
	var diffText string
	var err error

	if status == "staged" {
		// stagedファイルの場合はstaged差分を取得
		diffText, err = git.GetStagedDiff(filePath, repoRoot)
	} else {
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

// ファイル一覧を表示
func ShowFileList(app *tview.Application, stagedFiles, modifiedFiles, untrackedFiles []string, repoRoot string, onSelect func(file string, status string), onUpdate func(), enableAutoRefresh bool) tview.Primitive {
	// ファイルリストを更新するための参照を保持
	stagedFilesPtr := &stagedFiles
	modifiedFilesPtr := &modifiedFiles
	untrackedFilesPtr := &untrackedFiles

	// listStatusView を作成
	listStatusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)
	listStatusView.SetBorder(true)
	listStatusView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// フレックスレイアウトを作成（上下分割、その下に左右分割）
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	// 左右分割のフレックス
	contentFlex := tview.NewFlex()
	// contentFlex.SetBorder(true)
	contentFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

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
		AddItem(afterView, 0, 1, false)
	splitViewFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// 現在のファイル情報を保持
	var currentFile string
	var currentStatus string
	var currentDiffText string // 生の差分テキストを保持
	var cursorY int = 0
	var selectStart int = -1
	var selectEnd int = -1
	var isSelecting bool = false
	var currentSelection int = 0
	var leftPaneFocused bool = true
	var gPressed bool = false
	var lastGTime time.Time
	var isSplitView bool = false // Split Viewモードのフラグ

	// 保存されたカーソル位置を復元
	if preferUnstagedSection || savedTargetFile != "" {
		// カーソル位置を計算
		targetSelection := 0
		foundTarget := false

		// 全ファイルを走査
		for _, file := range *stagedFilesPtr {
			if strings.TrimSpace(file) != "" {
				if !preferUnstagedSection && file == savedTargetFile {
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

			for _, file := range *modifiedFilesPtr {
				if strings.TrimSpace(file) != "" {
					if preferUnstagedSection && targetSelection == unstagedStart {
						// unstagedセクションの最初のファイル
						currentSelection = targetSelection
						foundTarget = true
						break
					} else if !preferUnstagedSection && file == savedTargetFile {
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
	var regions []string
	var fileMap = make(map[string]string)
	var fileStatusMap = make(map[string]string)
	var lineNumberMap = make(map[int]int)

	// ボーダーを作成
	verticalBorder := CreateVerticalBorder()
	horizontalTopBorder := CreateHorizontalTopBorder()
	horizontalBottomBorder := CreateHorizontalBottomBorder()

	// 左右のペインをフレックスに追加（左:縦線:右 = 1:0:4）
	verticalBorderLeft := CreateVerticalBorder()
	contentFlex.
		AddItem(verticalBorderLeft, 1, 0, false).
		AddItem(fileListView, 0, 1, true).
		AddItem(verticalBorder, 1, 0, false).
		AddItem(diffView, 0, 4, false)

	// ファイル一覧の内容を構築（色付き）
	buildFileListContent := func(focusedPane bool) string {
		return BuildFileListContent(
			*stagedFilesPtr,
			*modifiedFilesPtr,
			*untrackedFilesPtr,
			currentSelection,
			focusedPane,
			&regions,
			fileMap,
			fileStatusMap,
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
		if currentSelection >= 0 && currentSelection < len(regions) {
			regionID := regions[currentSelection]
			file := fileMap[regionID]
			status := fileStatusMap[regionID]

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
				updateDiffViewWithoutCursor(diffView, currentDiffText)
			}
		}
	}

	updateFileListView()

	// 初期表示時に最初のファイルの差分を表示
	updateSelectedFileDiff()

	// 初期メッセージを設定
	listStatusView.SetText(listKeyBindingMessage)

	// ファイルリストのキーバインディングを設定
	fileListKeyContext := &FileListKeyContext{
		// UI Components
		FileListView:  fileListView,
		DiffView:      diffView,
		BeforeView:    beforeView,
		AfterView:     afterView,
		SplitViewFlex: splitViewFlex,
		ContentFlex:   contentFlex,
		App:           app,

		// State
		CurrentSelection: &currentSelection,
		CursorY:          &cursorY,
		IsSelecting:      &isSelecting,
		SelectStart:      &selectStart,
		SelectEnd:        &selectEnd,
		IsSplitView:      &isSplitView,
		LeftPaneFocused:  &leftPaneFocused,
		CurrentFile:      &currentFile,
		CurrentStatus:    &currentStatus,
		CurrentDiffText:  &currentDiffText,

		// Collections
		Regions:       &regions,
		FileMap:       fileMap,
		FileStatusMap: fileStatusMap,

		// Paths
		RepoRoot: repoRoot,

		// Callbacks
		UpdateFileListView:     updateFileListView,
		UpdateSelectedFileDiff: updateSelectedFileDiff,
		RefreshFileList:        refreshFileList,
		UpdateCurrentDiffText:  updateCurrentDiffText,
	}
	SetupFileListKeyBindings(fileListKeyContext)

	// 右ペインのキー入力処理を設定（file_view.goと同じ動作）
	// diffViewのキーバインディングを設定
	diffViewContext := &DiffViewContext{
		// UI Components
		DiffView:      diffView,
		FileListView:  fileListView,
		BeforeView:    beforeView,
		AfterView:     afterView,
		SplitViewFlex: splitViewFlex,
		ContentFlex:   contentFlex,
		App:           app,

		// State
		CursorY:               &cursorY,
		SelectStart:           &selectStart,
		SelectEnd:             &selectEnd,
		IsSelecting:           &isSelecting,
		IsSplitView:           &isSplitView,
		LeftPaneFocused:       &leftPaneFocused,
		CurrentDiffText:       &currentDiffText,
		CurrentFile:           &currentFile,
		CurrentStatus:         &currentStatus,
		SavedTargetFile:       &savedTargetFile,
		PreferUnstagedSection: &preferUnstagedSection,

		// Paths
		RepoRoot:  repoRoot,
		PatchPath: patchPath,

		// Key handling state
		GPressed:  &gPressed,
		LastGTime: &lastGTime,

		// View updater
		ViewUpdater: NewUnifiedViewUpdater(diffView),

		// Callbacks
		UpdateFileListView: updateFileListView,
		UpdateListStatus:   updateListStatus,
		RefreshFileList:    refreshFileList,
		OnUpdate:           onUpdate,
	}
	SetupDiffViewKeyBindings(diffViewContext)

	// 自動リフレッシュが有効な場合のみゴルーチンを開始
	if enableAutoRefresh {
		stopRefresh := make(chan bool)
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					app.QueueUpdateDraw(func() {
						// 現在の選択ファイルとステータスを保存
						var currentlySelectedFile string
						var currentlySelectedStatus string
						if currentSelection >= 0 && currentSelection < len(regions) {
							regionID := regions[currentSelection]
							currentlySelectedFile = fileMap[regionID]
							currentlySelectedStatus = fileStatusMap[regionID]
						}

						// ファイルリストを更新
						refreshFileList()

						// 選択位置を復元（ファイル名とステータスの両方で検索）
						newSelection := -1
						for i, regionID := range regions {
							if fileMap[regionID] == currentlySelectedFile && fileStatusMap[regionID] == currentlySelectedStatus {
								newSelection = i
								break
							}
						}
						if newSelection >= 0 {
							currentSelection = newSelection
						} else if currentSelection >= len(regions) {
							currentSelection = len(regions) - 1
						}

						// 表示を更新
						updateFileListView()

						// 右ペインの差分も更新
						if leftPaneFocused {
							// 左ペインにフォーカスがある場合は通常の更新
							updateSelectedFileDiff()
						} else if currentFile != "" {
							// 右ペインにフォーカスがある場合は現在のカーソル位置を保持して更新
							var newDiffText string
							if currentStatus == "staged" {
								newDiffText, _ = git.GetStagedDiff(currentFile, repoRoot)
							} else {
								newDiffText, _ = git.GetFileDiff(currentFile, repoRoot)
							}

							// 差分が変更された場合のみ更新
							if newDiffText != currentDiffText {
								currentDiffText = newDiffText
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
									updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
								} else {
									updateDiffViewWithCursor(diffView, currentDiffText, cursorY)
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

	// mainFlex にステータスビューとコンテンツを追加
	mainFlex.AddItem(listStatusView, 5, 0, false).
		AddItem(horizontalTopBorder, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(horizontalBottomBorder, 1, 0, false)

	return mainFlex
}
