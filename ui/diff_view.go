package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui/commands"
	"github.com/sukechannnn/gitta/util"
)

// createLineNumberMapping creates line number mapping from diff text
func createLineNumberMapping(diffText string) (map[int]int, map[int]int) {
	oldLineMap := make(map[int]int)
	newLineMap := make(map[int]int)

	lines := strings.Split(diffText, "\n")
	displayLine := 0
	var oldLineNum, newLineNum int
	inHunk := false

	for _, line := range lines {
		// ヘッダー行をスキップ（ColorizeDiffと同じロジック）
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			// ハンクヘッダーから行番号を取得
			if strings.HasPrefix(line, "@@") {
				// @@ -oldStart,oldCount +newStart,newCount @@
				var oldStart, newStart int
				fmt.Sscanf(line, "@@ -%d", &oldStart)
				parts := strings.Split(line, " +")
				if len(parts) >= 2 {
					fmt.Sscanf(parts[1], "%d", &newStart)
				}
				oldLineNum = oldStart
				newLineNum = newStart
				inHunk = true
			}
			continue
		}

		if !inHunk {
			continue
		}

		// 実際の差分行（ColorizeDiffで表示される行のみカウント）
		if strings.HasPrefix(line, "-") {
			oldLineMap[displayLine] = oldLineNum
			oldLineNum++
		} else if strings.HasPrefix(line, "+") {
			newLineMap[displayLine] = newLineNum
			newLineNum++
		} else {
			// スペースで始まる行またはそれ以外の行（コンテキスト行）
			oldLineMap[displayLine] = oldLineNum
			newLineMap[displayLine] = newLineNum
			oldLineNum++
			newLineNum++
		}

		displayLine++
	}

	return oldLineMap, newLineMap
}

// ----------↓↓↓ unified_view_functions ↓↓↓----------
func getColorizeDiffLines(diffText string) []string {
	// ColorizeDiffを使って色付けとヘッダー除外
	coloredDiff := ColorizeDiff(diffText)

	// 表示用の行を返す（カーソル表示のため）
	lines := util.SplitLines(coloredDiff)

	return lines
}

func updateDiffViewWithoutCursor(diffView *tview.TextView, diffText string) {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMapping(diffView, diffText, -1, -1, -1, false, oldLineMap, newLineMap)
}

func updateDiffViewWithCursor(diffView *tview.TextView, diffText string, cursorY int) {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMapping(diffView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap)
}

func updateDiffViewWithSelection(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMapping(diffView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
}

// updateDiffViewWithSelectionAndMapping updates diff view with selection and line mapping
func updateDiffViewWithSelectionAndMapping(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int) {
	diffView.Clear()

	lines := getColorizeDiffLines(diffText)

	// 行番号の最大桁数を計算
	maxOldLine := 0
	maxNewLine := 0
	for _, lineNum := range oldLineMap {
		if lineNum > maxOldLine {
			maxOldLine = lineNum
		}
	}
	for _, lineNum := range newLineMap {
		if lineNum > maxNewLine {
			maxNewLine = lineNum
		}
	}
	maxDigits := len(fmt.Sprintf("%d", maxNewLine))
	if len(fmt.Sprintf("%d", maxOldLine)) > maxDigits {
		maxDigits = len(fmt.Sprintf("%d", maxOldLine))
	}

	for i, line := range lines {
		// 実際の行番号を取得
		var lineNum string
		if strings.HasPrefix(line, "[red]") || (len(line) > 0 && line[0] == '-') {
			// 削除行
			if num, ok := oldLineMap[i]; ok {
				lineNum = fmt.Sprintf("%*d", maxDigits, num)
			} else {
				lineNum = strings.Repeat(" ", maxDigits)
			}
			lineNum += " -│ "
		} else if strings.HasPrefix(line, "[green]") || (len(line) > 0 && line[0] == '+') {
			// 追加行
			if num, ok := newLineMap[i]; ok {
				lineNum = fmt.Sprintf("%*d", maxDigits, num)
			} else {
				lineNum = strings.Repeat(" ", maxDigits)
			}
			lineNum += " +│ "
		} else {
			// 共通行
			if num, ok := newLineMap[i]; ok {
				lineNum = fmt.Sprintf("%*d", maxDigits, num)
			} else if num, ok := oldLineMap[i]; ok {
				lineNum = fmt.Sprintf("%*d", maxDigits, num)
			} else {
				lineNum = strings.Repeat(" ", maxDigits)
			}
			lineNum += "  │ "
		}

		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行を黄色でハイライト
			diffView.Write([]byte("[black:yellow]" + lineNum + line + "[-:-]\n"))
		} else if i == cursorY {
			// カーソル行を青でハイライト
			diffView.Write([]byte("[white:blue]" + lineNum + line + "[-:-]\n"))
		} else {
			diffView.Write([]byte("[dimgray]" + lineNum + "[-]" + line + "\n"))
		}
	}

	// スクロール位置を調整（カーソルが見える範囲に）
	_, _, _, height := diffView.GetInnerRect()
	currentRow, _ := diffView.GetScrollOffset()

	// カーソルが画面より下にある場合
	if cursorY >= currentRow+height-1 {
		diffView.ScrollTo(cursorY-height+2, 0)
	}
	// カーソルが画面より上にある場合
	if cursorY < currentRow {
		diffView.ScrollTo(cursorY, 0)
	}
}

// ----------↑↑↑ unified_view_functions ↑↑↑----------

// ----------↓↓↓ split_view_functions ↓↓↓----------

// getSplitViewLineCount gets valid line count for split view
func getSplitViewLineCount(diffText string) int {
	lines := strings.Split(diffText, "\n")
	count := 0
	inHunk := false

	for _, line := range lines {
		// ヘッダー行をスキップ
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}

		if inHunk {
			count++
		}
	}

	return count
}

func updateSplitViewWithoutCursor(beforeView, afterView *tview.TextView, diffText string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, -1, -1, -1, false, oldLineMap, newLineMap)
}

// updateSplitViewWithCursor updates split view with cursor
func updateSplitViewWithCursor(beforeView, afterView *tview.TextView, diffText string, cursorY int) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap)
}

func updateSplitViewWithSelection(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
}

// updateSplitViewWithSelectionAndMapping updates split view with selection and line mapping
func updateSplitViewWithSelectionAndMapping(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int) {
	beforeView.Clear()
	afterView.Clear()

	lines := strings.Split(diffText, "\n")
	var beforeLines []string
	var afterLines []string
	var beforeLineNums []string
	var afterLineNums []string
	var inHunk bool = false
	displayLine := 0

	// 行番号の最大桁数を計算
	maxOldLine := 0
	maxNewLine := 0
	for _, lineNum := range oldLineMap {
		if lineNum > maxOldLine {
			maxOldLine = lineNum
		}
	}
	for _, lineNum := range newLineMap {
		if lineNum > maxNewLine {
			maxNewLine = lineNum
		}
	}
	maxDigits := len(fmt.Sprintf("%d", maxNewLine))
	if len(fmt.Sprintf("%d", maxOldLine)) > maxDigits {
		maxDigits = len(fmt.Sprintf("%d", maxOldLine))
	}

	for _, line := range lines {
		// ヘッダー行を非表示にする
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			// ハンクヘッダー（非表示にする）
			inHunk = true
			continue
		} else if inHunk {
			if strings.HasPrefix(line, "-") {
				// 削除行（左側のみに表示、- 記号を含める）
				beforeLines = append(beforeLines, "[red]"+line+"[-]")
				afterLines = append(afterLines, "[dimgray] [-]") // 右側には左側の行数と合わせるためのスペースを表示
				// 左側に実際の行番号、右側は空
				if num, ok := oldLineMap[displayLine]; ok {
					beforeLineNums = append(beforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					beforeLineNums = append(beforeLineNums, strings.Repeat(" ", maxDigits))
				}
				afterLineNums = append(afterLineNums, strings.Repeat(" ", maxDigits))
			} else if strings.HasPrefix(line, "+") {
				// 追加行（右側のみに表示、+ 記号を含める）
				beforeLines = append(beforeLines, "[dimgray] [-]") // 左側には右側の行数と合わせるためのスペースを表示
				afterLines = append(afterLines, "[green]"+line+"[-]")
				// 左側は空、右側に実際の行番号
				beforeLineNums = append(beforeLineNums, strings.Repeat(" ", maxDigits))
				if num, ok := newLineMap[displayLine]; ok {
					afterLineNums = append(afterLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					afterLineNums = append(afterLineNums, strings.Repeat(" ", maxDigits))
				}
			} else if strings.HasPrefix(line, " ") {
				// 変更なし行（両側に表示、先頭のスペースを保持）
				beforeLines = append(beforeLines, " "+line[1:])
				afterLines = append(afterLines, " "+line[1:])
				// 両側に実際の行番号
				if num, ok := oldLineMap[displayLine]; ok {
					beforeLineNums = append(beforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					beforeLineNums = append(beforeLineNums, strings.Repeat(" ", maxDigits))
				}
				if num, ok := newLineMap[displayLine]; ok {
					afterLineNums = append(afterLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					afterLineNums = append(afterLineNums, strings.Repeat(" ", maxDigits))
				}
			} else {
				// その他の行
				beforeLines = append(beforeLines, " "+line)
				afterLines = append(afterLines, " "+line)
				// 両側に実際の行番号
				if num, ok := oldLineMap[displayLine]; ok {
					beforeLineNums = append(beforeLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					beforeLineNums = append(beforeLineNums, strings.Repeat(" ", maxDigits))
				}
				if num, ok := newLineMap[displayLine]; ok {
					afterLineNums = append(afterLineNums, fmt.Sprintf("%*d", maxDigits, num))
				} else {
					afterLineNums = append(afterLineNums, strings.Repeat(" ", maxDigits))
				}
			}
			displayLine++
		}
	}

	// カーソル行の実際のインデックスを取得（単純化）
	// cursorYは表示行のインデックスとして扱う
	cursorIndex := -1
	if cursorY >= 0 && cursorY < len(beforeLines) {
		cursorIndex = cursorY
	}

	// 表示を更新
	for i, line := range beforeLines {
		// 行番号を追加
		lineNum := beforeLineNums[i] + " │ "

		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行を黄色でハイライト
			beforeView.Write([]byte("[black:yellow]" + lineNum + line + "[-:-]\n"))
		} else if cursorIndex >= 0 && i == cursorIndex {
			// カーソル行を青でハイライト
			beforeView.Write([]byte("[white:blue]" + lineNum + line + "[-:-]\n"))
		} else {
			beforeView.Write([]byte("[dimgray]" + lineNum + "[-]" + line + "\n"))
		}
	}

	for i, line := range afterLines {
		// 行番号を追加
		lineNum := afterLineNums[i] + " │ "

		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行を黄色でハイライト
			afterView.Write([]byte("[black:yellow]" + lineNum + line + "[-:-]\n"))
		} else if cursorIndex >= 0 && i == cursorIndex {
			// カーソル行を青でハイライト
			afterView.Write([]byte("[white:blue]" + lineNum + line + "[-:-]\n"))
		} else {
			afterView.Write([]byte("[dimgray]" + lineNum + "[-]" + line + "\n"))
		}
	}

	// スクロール位置を同期
	if cursorIndex >= 0 {
		_, _, _, height := beforeView.GetInnerRect()
		currentRow, _ := beforeView.GetScrollOffset()

		// カーソルが画面より下にある場合
		if cursorIndex >= currentRow+height-1 {
			scrollPos := cursorIndex - height + 2
			beforeView.ScrollTo(scrollPos, 0)
			afterView.ScrollTo(scrollPos, 0)
		}
		// カーソルが画面より上にある場合
		if cursorIndex < currentRow {
			beforeView.ScrollTo(cursorIndex, 0)
			afterView.ScrollTo(cursorIndex, 0)
		}
	} else {
		// カーソルなしの場合は先頭に
		beforeView.ScrollTo(0, 0)
		afterView.ScrollTo(0, 0)
	}
}

// ----------↑↑↑ split_view_functions ↑↑↑----------

// DiffViewUpdater interface for updating diff views
type DiffViewUpdater interface {
	UpdateWithoutCursor(diffText string)
	UpdateWithCursor(diffText string, cursorY int)
	UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool)
}

// UnifiedViewUpdater implements DiffViewUpdater for unified diff view
type UnifiedViewUpdater struct {
	diffView *tview.TextView
}

// NewUnifiedViewUpdater creates a new UnifiedViewUpdater
func NewUnifiedViewUpdater(diffView *tview.TextView) *UnifiedViewUpdater {
	return &UnifiedViewUpdater{
		diffView: diffView,
	}
}

// UpdateWithoutCursor updates unified view without cursor
func (u *UnifiedViewUpdater) UpdateWithoutCursor(diffText string) {
	updateDiffViewWithoutCursor(u.diffView, diffText)
}

// UpdateWithCursor updates unified view with cursor
func (u *UnifiedViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	updateDiffViewWithCursor(u.diffView, diffText, cursorY)
}

// UpdateWithSelection updates unified view with selection
func (u *UnifiedViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	updateDiffViewWithSelection(u.diffView, diffText, cursorY, selectStart, selectEnd, isSelecting)
}

// SplitViewUpdater implements DiffViewUpdater for split diff view
type SplitViewUpdater struct {
	beforeView *tview.TextView
	afterView  *tview.TextView
}

// NewSplitViewUpdater creates a new SplitViewUpdater
func NewSplitViewUpdater(beforeView, afterView *tview.TextView) *SplitViewUpdater {
	return &SplitViewUpdater{
		beforeView: beforeView,
		afterView:  afterView,
	}
}

// UpdateWithoutCursor updates split view without cursor
func (s *SplitViewUpdater) UpdateWithoutCursor(diffText string) {
	updateSplitViewWithoutCursor(s.beforeView, s.afterView, diffText)
}

// UpdateWithCursor updates split view with cursor
func (s *SplitViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	updateSplitViewWithCursor(s.beforeView, s.afterView, diffText, cursorY)
}

// UpdateWithSelection updates split view with selection
func (s *SplitViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	updateSplitViewWithSelection(s.beforeView, s.afterView, diffText, cursorY, selectStart, selectEnd, isSelecting)
}

// isLineSelected checks if line is in selection range
func isLineSelected(index, start, end int) bool {
	if start == -1 || end == -1 {
		return false
	}
	min := start
	max := end
	if min > max {
		min, max = max, min
	}
	return index >= min && index <= max
}

// DiffViewContext contains all the context needed for diff view key bindings
type DiffViewContext struct {
	// UI Components
	diffView      *tview.TextView
	fileListView  *tview.TextView
	beforeView    *tview.TextView
	afterView     *tview.TextView
	splitViewFlex *tview.Flex
	contentFlex   *tview.Flex
	app           *tview.Application

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
	updateListStatus   func(string, string)
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
	// 共通のキーハンドラー関数
	keyHandler := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			// 左ペインに戻る
			// 選択モードをリセット（カーソル位置は保持）
			*ctx.isSelecting = false
			*ctx.selectEnd = -1
			*ctx.selectStart = -1
			// 表示を更新（カーソルなし）
			if ctx.viewUpdater != nil {
				ctx.viewUpdater.UpdateWithoutCursor(*ctx.currentDiffText)
			}
			// 左ペインにフォーカスを戻す
			*ctx.leftPaneFocused = true
			ctx.updateFileListView()
			ctx.app.SetFocus(ctx.fileListView)
			return nil
		case tcell.KeyEnter:
			// 左ペインに戻る
			// 選択モードをリセット（カーソル位置は保持）
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
					ctx.contentFlex.RemoveItem(ctx.diffView)
					ctx.contentFlex.AddItem(ctx.splitViewFlex, 0, 4, false)
					// フォーカスがdiffViewにある場合、splitViewFlexに移動
					if !*ctx.leftPaneFocused {
						ctx.app.SetFocus(ctx.splitViewFlex)
					}
				} else {
					// 通常の差分表示に戻す
					ctx.viewUpdater = NewUnifiedViewUpdater(ctx.diffView)
					ctx.contentFlex.RemoveItem(ctx.splitViewFlex)
					ctx.contentFlex.AddItem(ctx.diffView, 0, 4, false)
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
				// パッチファイルが存在するか確認
				if _, err := os.Stat(ctx.patchPath); os.IsNotExist(err) {
					ctx.updateListStatus("No patch to undo", "yellow")
					return nil
				}

				cmd := exec.Command("git", "-c", "core.quotepath=false", "apply", "-R", "--cached", ctx.patchPath)
				cmd.Dir = ctx.repoRoot
				_, err := cmd.CombinedOutput()
				if err != nil {
					ctx.updateListStatus("Undo failed!", "firebrick")
				} else {
					ctx.updateListStatus("Undo successful!", "gold")

					// 差分を再取得
					var newDiffText string
					if *ctx.currentStatus == "staged" {
						newDiffText, _ = git.GetStagedDiff(*ctx.currentFile, ctx.repoRoot)
					} else {
						newDiffText, _ = git.GetFileDiff(*ctx.currentFile, ctx.repoRoot)
					}
					*ctx.currentDiffText = newDiffText

					// 再描画
					if ctx.viewUpdater != nil {
						ctx.viewUpdater.UpdateWithCursor(*ctx.currentDiffText, *ctx.cursorY)
					}

					// ファイルリストを更新
					ctx.refreshFileList()
					ctx.updateFileListView()
				}
			case 'a':
				// commandA関数を呼び出す
				params := commands.CommandAParams{
					SelectStart:      *ctx.selectStart,
					SelectEnd:        *ctx.selectEnd,
					CurrentFile:      *ctx.currentFile,
					CurrentStatus:    *ctx.currentStatus,
					CurrentDiffText:  *ctx.currentDiffText,
					RepoRoot:         ctx.repoRoot,
					UpdateListStatus: ctx.updateListStatus,
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
							ctx.updateListStatus("File unstaged successfully!", "gold")
						} else {
							ctx.updateListStatus("File staged successfully!", "gold")
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
							ctx.updateListStatus("Failed to unstage file", "firebrick")
						} else {
							ctx.updateListStatus("Failed to stage file", "firebrick")
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
