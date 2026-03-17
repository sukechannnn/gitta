package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
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
	splitViewFlex   *tview.Flex
	unifiedViewFlex *tview.Flex
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
	currentSelection      *int
	fileList              *[]FileEntry
	preserveScrollRow     *int  // ファイルリストのスクロール位置を保持
	ignoreWhitespace      *bool // Whitespace無視モード

	// Paths
	repoRoot  string
	patchPath string

	// Key handling state
	gPressed  *bool
	lastGTime *time.Time

	// View updater
	viewUpdater DiffViewUpdater

	// Fold state
	foldState *FoldState

	// Search state
	searchQuery               *string // 現在の検索クエリ（空 = 検索なし）
	searchMatches             *[]int  // マッチした行インデックスのリスト
	searchMatchIndex          *int    // 現在のマッチインデックス（searchMatches 内の位置）
	isSearchMode              *bool   // 検索入力モード中か
	searchInput               *string // 検索入力中の文字列（未確定）
	searchCursorYBeforeSearch *int    // 検索開始前のカーソル位置

	// Callbacks
	updateFileListView    func()
	updateGlobalStatus    func(string, string)
	setGlobalStatusText   func(string) // 直接ステータステキストを設定（タイマーなし）
	refreshFileList       func()
	onUpdate              func()
	updateCurrentDiffText func(string, string, string, *string, bool)
	updateStatusTitle     func()
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
		maxLines := GetUnifiedViewLineCount(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)

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
			ctx.viewUpdater = NewSplitViewUpdater(ctx.beforeView, ctx.afterView, ctx.currentFile)
		} else {
			ctx.viewUpdater = &UnifiedViewUpdater{
				diffView:    ctx.diffView,
				foldState:   ctx.foldState,
				filePath:    ctx.currentFile,
				repoRoot:    ctx.repoRoot,
				searchQuery: ctx.searchQuery,
			}
		}
	}

	// 共通のキーハンドラー関数
	keyHandler := func(event *tcell.EventKey) *tcell.EventKey {
		// 検索モード中のキーハンドリング
		if *ctx.isSearchMode {
			switch event.Key() {
			case tcell.KeyEnter:
				// 検索確定
				*ctx.searchQuery = *ctx.searchInput
				*ctx.isSearchMode = false
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				if len(*ctx.searchMatches) > 0 {
					ctx.setGlobalStatusText(fmt.Sprintf("[white]/%s [%d/%d][-]", tview.Escape(*ctx.searchQuery), *ctx.searchMatchIndex+1, len(*ctx.searchMatches)))
				} else {
					ctx.setGlobalStatusText(fmt.Sprintf("[tomato]/%s [no match][-]", tview.Escape(*ctx.searchQuery)))
				}
			case tcell.KeyEsc:
				// 検索キャンセル: カーソルを元の位置に戻す
				*ctx.isSearchMode = false
				*ctx.searchInput = ""
				*ctx.searchQuery = ""
				*ctx.searchMatches = nil
				*ctx.searchMatchIndex = -1
				*ctx.cursorY = *ctx.searchCursorYBeforeSearch
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				ctx.setGlobalStatusText(listKeyBindingMessage)
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(*ctx.searchInput) > 0 {
					// 1文字削除
					runes := []rune(*ctx.searchInput)
					*ctx.searchInput = string(runes[:len(runes)-1])
				}
				performSearch(ctx)
			case tcell.KeyRune:
				*ctx.searchInput += string(event.Rune())
				performSearch(ctx)
			}
			return nil
		}

		switch event.Key() {
		case tcell.KeyEsc:
			// 検索結果がある場合、最初の Esc でクリア
			if *ctx.searchQuery != "" {
				*ctx.searchQuery = ""
				*ctx.searchMatches = nil
				*ctx.searchMatchIndex = -1
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				return nil
			}
			// 選択モード中なら解除
			if *ctx.isSelecting {
				*ctx.isSelecting = false
				*ctx.selectEnd = -1
				*ctx.selectStart = -1
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				return nil
			}
			// 左ペインに戻る
			*ctx.leftPaneFocused = true
			if *ctx.isSplitView {
				updateSplitViewWithoutCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.currentFile)
			} else {
				updateDiffViewWithoutCursor(ctx.diffView, *ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
			}
			ctx.updateFileListView()
			ctx.app.SetFocus(ctx.fileListView)
			return nil
		case tcell.KeyEnter:
			// 左ペインに戻る
			*ctx.isSelecting = false
			*ctx.selectStart = -1
			*ctx.selectEnd = -1
			*ctx.leftPaneFocused = true
			// diff view をカーソルなしで再描画
			if *ctx.isSplitView {
				updateSplitViewWithoutCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.currentFile)
			} else {
				updateDiffViewWithoutCursor(ctx.diffView, *ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
			}
			ctx.updateFileListView()
			ctx.app.SetFocus(ctx.fileListView)
			return nil
		case tcell.KeyCtrlE:
			// Ctrl+E: 1行下にスクロール（カーソルは最上部になったら追従）
			scrollDiffView(ctx, 1)
			return nil
		case tcell.KeyCtrlY:
			if *ctx.isSelecting && *ctx.currentFile != "" {
				// 選択中: file/path:XX-YY をコピー
				start := *ctx.selectStart
				end := *ctx.selectEnd

				// Unified viewの場合、fold indicatorを除外
				if !*ctx.isSplitView {
					displayMapping := MapUnifiedDisplayToOriginalIdx(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
					if ms, ok := displayMapping[start]; ok {
						start = ms
					}
					if me, ok := displayMapping[end]; ok {
						end = me
					}
				}

				if start > end {
					start, end = end, start
				}

				oldLineMap, newLineMap := createLineNumberMapping(*ctx.currentDiffText)

				// 選択範囲内のファイル行番号を収集（newLineMap優先、なければoldLineMap）
				startLine := -1
				endLine := -1
				for i := start; i <= end; i++ {
					num := -1
					if n, ok := newLineMap[i]; ok {
						num = n
					} else if n, ok := oldLineMap[i]; ok {
						num = n
					}
					if num >= 0 {
						if startLine == -1 || num < startLine {
							startLine = num
						}
						if num > endLine {
							endLine = num
						}
					}
				}

				if startLine >= 0 {
					var ref string
					if startLine == endLine {
						ref = fmt.Sprintf("%s:%d", *ctx.currentFile, startLine)
					} else {
						ref = fmt.Sprintf("%s:%d-%d", *ctx.currentFile, startLine, endLine)
					}
					if err := commands.CopyToClipboard(ref); err == nil {
						ctx.updateGlobalStatus("Copied reference to clipboard", "forestgreen")
					} else {
						ctx.updateGlobalStatus("Failed to copy reference", "tomato")
					}
				}

				// 選択解除
				*ctx.isSelecting = false
				*ctx.selectStart = -1
				*ctx.selectEnd = -1
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
			} else if *ctx.currentFile != "" {
				// 非選択中: ファイルパスをコピー（既存動作）
				err := commands.CopyFilePath(*ctx.currentFile)
				if err == nil {
					ctx.updateGlobalStatus("Copied path to clipboard", "forestgreen")
				} else {
					ctx.updateGlobalStatus("Failed to copy path to clipboard", "tomato")
				}
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 's':
				// Split Viewのトグル
				*ctx.isSplitView = !*ctx.isSplitView

				if *ctx.isSplitView {
					// Split Viewを表示（現在のカーソル位置を維持）
					ctx.viewUpdater = NewSplitViewUpdater(ctx.beforeView, ctx.afterView, ctx.currentFile)
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					ctx.contentFlex.RemoveItem(ctx.unifiedViewFlex)
					ctx.contentFlex.AddItem(ctx.splitViewFlex, 0, DiffViewFlexRatio, false)
					// フォーカスがdiffViewにある場合、splitViewFlexに移動
					if !*ctx.leftPaneFocused {
						ctx.app.SetFocus(ctx.splitViewFlex)
					}
				} else {
					// 通常の差分表示に戻す
					ctx.viewUpdater = &UnifiedViewUpdater{
						diffView:    ctx.diffView,
						foldState:   ctx.foldState,
						filePath:    ctx.currentFile,
						repoRoot:    ctx.repoRoot,
						searchQuery: ctx.searchQuery,
					}
					ctx.contentFlex.RemoveItem(ctx.splitViewFlex)
					ctx.contentFlex.AddItem(ctx.unifiedViewFlex, 0, DiffViewFlexRatio, false)
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					// フォーカスがsplitViewFlexにある場合、diffViewに移動
					if !*ctx.leftPaneFocused {
						ctx.app.SetFocus(ctx.diffView)
					}
				}
				return nil
			case 'w':
				// Whitespace無視モードのトグル
				*ctx.ignoreWhitespace = !*ctx.ignoreWhitespace

				// 差分を再取得
				ctx.updateCurrentDiffText(*ctx.currentFile, *ctx.currentStatus, ctx.repoRoot, ctx.currentDiffText, *ctx.ignoreWhitespace)

				// カーソル位置をリセット
				*ctx.cursorY = 0

				// 表示を更新
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}

				// ステータスタイトルを更新
				if ctx.updateStatusTitle != nil {
					ctx.updateStatusTitle()
				}

				// ステータス表示
				if *ctx.ignoreWhitespace {
					ctx.updateGlobalStatus("Whitespace changes hidden", "forestgreen")
				} else {
					ctx.updateGlobalStatus("Whitespace changes shown", "forestgreen")
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
				maxLines := 0
				if *ctx.isSplitView {
					// Split Viewの場合は有効な行数を取得
					splitViewLines := getSplitViewLineCount(*ctx.currentDiffText)
					if splitViewLines > 0 {
						maxLines = splitViewLines - 1
					}
				} else {
					// 通常ビューの場合は表示行数を取得（折りたたみを含む）
					lineCount := GetUnifiedViewLineCount(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
					if lineCount > 0 {
						maxLines = lineCount - 1
					}
				}
				*ctx.cursorY = maxLines
				if *ctx.isSelecting {
					*ctx.selectEnd = *ctx.cursorY
				}
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}
				return nil
			case 'j':
				// 下移動
				maxLines := 0
				if *ctx.isSplitView {
					// Split Viewの場合は有効な行数を取得
					splitViewLines := getSplitViewLineCount(*ctx.currentDiffText)
					if splitViewLines > 0 {
						maxLines = splitViewLines - 1
					}
				} else {
					// 通常ビューの場合は表示行数を取得（折りたたみを含む）
					if len(strings.TrimSpace(*ctx.currentDiffText)) > 0 {
						lineCount := GetUnifiedViewLineCount(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
						if lineCount > 0 {
							maxLines = lineCount - 1
						}
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
			case 'y':
				lines := getSelectableDiffLines(*ctx.currentDiffText)
				if len(lines) == 0 {
					ctx.updateGlobalStatus("No diff content to copy", "tomato")
					return nil
				}

				start := *ctx.cursorY
				end := *ctx.cursorY
				if *ctx.isSelecting && *ctx.selectStart >= 0 && *ctx.selectEnd >= 0 {
					start = *ctx.selectStart
					end = *ctx.selectEnd
				}

				// Unified viewの場合、fold indicatorを除外した実際の差分行インデックスに変換
				if !*ctx.isSplitView {
					displayMapping := MapUnifiedDisplayToOriginalIdx(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
					if mappedStart, ok := displayMapping[start]; ok {
						start = mappedStart
					}
					if mappedEnd, ok := displayMapping[end]; ok {
						end = mappedEnd
					}
				}

				if start > end {
					start, end = end, start
				}

				if start < 0 {
					start = 0
				}
				if end < 0 {
					end = 0
				}
				if start >= len(lines) {
					ctx.updateGlobalStatus("Selection is out of range", "tomato")
					return nil
				}
				if end >= len(lines) {
					end = len(lines) - 1
				}

				selected := lines[start : end+1]
				sanitized := make([]string, 0, len(selected))
				for _, line := range selected {
					sanitized = append(sanitized, stripDiffPrefix(line))
				}
				text := strings.Join(sanitized, "\n")
				if err := commands.CopyToClipboard(text); err != nil {
					ctx.updateGlobalStatus("Failed to copy diff lines", "tomato")
				} else {
					message := "Copied line to clipboard"
					if len(selected) > 1 {
						message = "Copied lines to clipboard"
					}
					ctx.updateGlobalStatus(message, "forestgreen")
				}

				if *ctx.isSelecting {
					*ctx.isSelecting = false
					*ctx.selectStart = -1
					*ctx.selectEnd = -1
				}

				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}

				return nil
			case '/':
				// 検索モード開始
				*ctx.isSearchMode = true
				*ctx.searchInput = ""
				*ctx.searchCursorYBeforeSearch = *ctx.cursorY
				ctx.setGlobalStatusText("[white]/[-]")
				return nil
			case 'n':
				// 次のマッチに移動
				if *ctx.searchQuery != "" && len(*ctx.searchMatches) > 0 {
					moveToNextMatch(ctx)
				}
				return nil
			case 'N':
				// 前のマッチに移動
				if *ctx.searchQuery != "" && len(*ctx.searchMatches) > 0 {
					moveToPrevMatch(ctx)
				}
				return nil
			case 'u':
				ctx.updateGlobalStatus("undo is not implemented!", "tomato")
			case 'v':
				// vim でファイルを開く
				if *ctx.currentFile != "" {
					ctx.app.Suspend(func() {
						cmd := exec.Command("vim", "-c", "set title titlestring=[gitta]\\ %f", *ctx.currentFile)
						cmd.Dir = ctx.repoRoot
						cmd.Stdin = os.Stdin
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						cmd.Run()
					})
					ctx.refreshFileList()
					ctx.updateFileListView()
					ctx.updateCurrentDiffText(*ctx.currentFile, *ctx.currentStatus, ctx.repoRoot, ctx.currentDiffText, *ctx.ignoreWhitespace)
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					}
				}
				return nil
			case 'e':
				// Expand/collapse fold at cursor
				if !*ctx.isSplitView && ctx.foldState != nil {
					foldID := GetFoldIDAtLine(*ctx.currentDiffText, *ctx.cursorY, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
					if foldID != "" {
						wasExpanded := ctx.foldState.IsExpanded(foldID)
						ctx.foldState.ToggleExpand(foldID)
						InvalidateUnifiedContentCache()

						// If collapsing, move cursor to the fold indicator position
						if wasExpanded {
							newPos := GetFoldIndicatorPosition(*ctx.currentDiffText, foldID, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
							*ctx.cursorY = newPos
						}

						if ctx.viewUpdater != nil {
							ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
						}
					}
				}
				return nil
			case 'a':
				// commandA関数を呼び出す
				// Unified viewの場合、fold indicatorを除外した実際の差分行インデックスに変換
				selectStart := *ctx.selectStart
				selectEnd := *ctx.selectEnd
				if !*ctx.isSplitView {
					// Fold indicatorを除外したマッピングを取得
					displayMapping := MapUnifiedDisplayToOriginalIdx(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
					// 選択範囲をfold indicatorを除外したインデックスに変換
					if mappedStart, ok := displayMapping[selectStart]; ok {
						selectStart = mappedStart
					}
					if mappedEnd, ok := displayMapping[selectEnd]; ok {
						selectEnd = mappedEnd
					}
				}

				params := commands.CommandAParams{
					SelectStart:        selectStart,
					SelectEnd:          selectEnd,
					CurrentFile:        *ctx.currentFile,
					CurrentStatus:      *ctx.currentStatus,
					CurrentDiffText:    *ctx.currentDiffText,
					RepoRoot:           ctx.repoRoot,
					UpdateGlobalStatus: ctx.updateGlobalStatus,
					IsSplitView:        *ctx.isSplitView,
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

				// 選択を解除してカーソル位置を更新
				*ctx.isSelecting = false
				*ctx.selectStart = -1
				*ctx.selectEnd = -1

				// カーソル位置の境界チェック
				newCursorPos := result.NewCursorPos
				if len(strings.TrimSpace(*ctx.currentDiffText)) > 0 {
					coloredDiff := ColorizeDiff(*ctx.currentDiffText)
					diffLines := util.SplitLines(coloredDiff)
					maxLines := len(diffLines) - 1
					if maxLines < 0 {
						maxLines = 0
					}
					if newCursorPos > maxLines {
						newCursorPos = maxLines
					}
					if newCursorPos < 0 {
						newCursorPos = 0
					}
				} else {
					newCursorPos = 0
				}
				*ctx.cursorY = newCursorPos

				// 再描画
				if ctx.viewUpdater != nil {
					ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
				}

				// ファイルリストを内部的に更新
				ctx.refreshFileList()

				// 差分が残っている場合
				if !result.ShouldUpdate {
					// 現在の選択ファイルとステータスを保存
					var currentlySelectedFile string
					var currentlySelectedStatus string
					if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
						fileEntry := (*ctx.fileList)[*ctx.currentSelection]
						currentlySelectedFile = fileEntry.Path
						currentlySelectedStatus = fileEntry.StageStatus
					}

					// ファイルリストを再描画
					ctx.updateFileListView()

					// 選択位置を復元（ファイル名とステータスの両方で検索）
					newSelection := -1
					for i, fileEntry := range *ctx.fileList {
						if fileEntry.Path == currentlySelectedFile && fileEntry.StageStatus == currentlySelectedStatus {
							newSelection = i
							break
						}
					}
					if newSelection >= 0 {
						*ctx.currentSelection = newSelection
					} else if *ctx.currentSelection >= len(*ctx.fileList) {
						*ctx.currentSelection = len(*ctx.fileList) - 1
					}

					// 選択位置が変更された場合は再度ファイルリストを更新
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

func getSelectableDiffLines(diffText string) []string {
	rawLines := util.SplitLines(diffText)
	visible := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if isUnifiedHeaderLine(line) {
			continue
		}
		visible = append(visible, line)
	}
	return visible
}

func stripDiffPrefix(line string) string {
	if len(line) == 0 {
		return line
	}
	switch line[0] {
	case '+', '-':
		return line[1:]
	default:
		return line
	}
}

// tview のカラータグを除去する正規表現
var tviewTagRegex = regexp.MustCompile(`\[("[^"]*"|[^\[\]]*)\]`)

// stripTviewTags removes tview color/region tags from text
func stripTviewTags(text string) string {
	return tviewTagRegex.ReplaceAllString(text, "")
}

// highlightSearchInTaggedText highlights occurrences of query in a tview-tagged string
func highlightSearchInTaggedText(tagged string, query string) string {
	if query == "" {
		return tagged
	}

	plain := stripTviewTags(tagged)
	plainRunes := []rune(plain)
	queryRunes := []rune(query)
	queryLen := len(queryRunes)

	if len(plainRunes) < queryLen {
		return tagged
	}

	// マッチする可視文字位置をマーク
	highlight := make([]bool, len(plainRunes))
	for i := 0; i <= len(plainRunes)-queryLen; i++ {
		match := true
		for j := 0; j < queryLen; j++ {
			if plainRunes[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			for j := 0; j < queryLen; j++ {
				highlight[i+j] = true
			}
		}
	}

	// マッチがなければそのまま返す
	anyMatch := false
	for _, h := range highlight {
		if h {
			anyMatch = true
			break
		}
	}
	if !anyMatch {
		return tagged
	}

	const hlStart = "[:#665500]"
	const hlEnd = "[:-]"

	var result strings.Builder
	runes := []rune(tagged)
	visibleIdx := 0
	inHL := false

	for i := 0; i < len(runes); {
		// tview タグの検出
		if runes[i] == '[' {
			j := i + 1
			inQuote := false
			for j < len(runes) {
				if runes[j] == '"' {
					inQuote = !inQuote
				} else if !inQuote && runes[j] == ']' {
					break
				} else if !inQuote && runes[j] == '[' {
					break
				}
				j++
			}
			if j < len(runes) && runes[j] == ']' {
				// 有効なタグ: ハイライト中ならタグの前後で再適用
				tagStr := string(runes[i : j+1])
				if inHL {
					result.WriteString(hlEnd)
					result.WriteString(tagStr)
					result.WriteString(hlStart)
				} else {
					result.WriteString(tagStr)
				}
				i = j + 1
				continue
			}
		}

		// 可視文字
		shouldHL := visibleIdx < len(highlight) && highlight[visibleIdx]

		if shouldHL && !inHL {
			result.WriteString(hlStart)
			inHL = true
		} else if !shouldHL && inHL {
			result.WriteString(hlEnd)
			inHL = false
		}

		result.WriteRune(runes[i])
		visibleIdx++
		i++
	}

	if inHL {
		result.WriteString(hlEnd)
	}

	return result.String()
}

// searchInUnifiedContent searches for query in unified view content and returns matching line indices
func searchInUnifiedContent(content *UnifiedViewContent, query string) []int {
	var matches []int
	for i, line := range content.Lines {
		plain := stripTviewTags(line.LineNumber + line.Content)
		if strings.Contains(plain, query) {
			matches = append(matches, i)
		}
	}
	return matches
}

// performSearch executes search with current searchInput and updates matches/cursor
func performSearch(ctx *DiffViewContext) {
	query := *ctx.searchInput
	if query == "" {
		*ctx.searchMatches = nil
		*ctx.searchMatchIndex = -1
		*ctx.cursorY = *ctx.searchCursorYBeforeSearch
		if ctx.viewUpdater != nil {
			ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
		}
		ctx.setGlobalStatusText("[white]/[-]")
		return
	}

	content := getCachedUnifiedContent(*ctx.currentDiffText, ctx.foldState, *ctx.currentFile, ctx.repoRoot)
	matches := searchInUnifiedContent(content, query)
	*ctx.searchMatches = matches

	if len(matches) > 0 {
		// 現在のカーソル位置から最も近い前方のマッチを探す
		matchIdx := 0
		for i, m := range matches {
			if m >= *ctx.searchCursorYBeforeSearch {
				matchIdx = i
				break
			}
		}
		*ctx.searchMatchIndex = matchIdx
		*ctx.cursorY = matches[matchIdx]
		if ctx.viewUpdater != nil {
			ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
		}
		ctx.setGlobalStatusText(fmt.Sprintf("[white]/%s [%d/%d][-]", tview.Escape(query), matchIdx+1, len(matches)))
	} else {
		*ctx.searchMatchIndex = -1
		*ctx.cursorY = *ctx.searchCursorYBeforeSearch
		if ctx.viewUpdater != nil {
			ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
		}
		ctx.setGlobalStatusText(fmt.Sprintf("[tomato]/%s [no match][-]", tview.Escape(query)))
	}
}

// moveToNextMatch moves cursor to the next search match
func moveToNextMatch(ctx *DiffViewContext) {
	matches := *ctx.searchMatches
	if len(matches) == 0 {
		return
	}
	*ctx.searchMatchIndex = (*ctx.searchMatchIndex + 1) % len(matches)
	*ctx.cursorY = matches[*ctx.searchMatchIndex]
	if ctx.viewUpdater != nil {
		ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
	}
	ctx.setGlobalStatusText(fmt.Sprintf("[white]/%s [%d/%d][-]", tview.Escape(*ctx.searchQuery), *ctx.searchMatchIndex+1, len(matches)))
}

// moveToPrevMatch moves cursor to the previous search match
func moveToPrevMatch(ctx *DiffViewContext) {
	matches := *ctx.searchMatches
	if len(matches) == 0 {
		return
	}
	*ctx.searchMatchIndex = (*ctx.searchMatchIndex - 1 + len(matches)) % len(matches)
	*ctx.cursorY = matches[*ctx.searchMatchIndex]
	if ctx.viewUpdater != nil {
		ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
	}
	ctx.setGlobalStatusText(fmt.Sprintf("[white]/%s [%d/%d][-]", tview.Escape(*ctx.searchQuery), *ctx.searchMatchIndex+1, len(matches)))
}
