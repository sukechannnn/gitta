package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// DiffViewUpdater interface for updating diff views
type DiffViewUpdater interface {
	UpdateWithoutCursor(diffText string)
	UpdateWithCursor(diffText string, cursorY int)
	UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool)
}

// UnifiedViewUpdater implements DiffViewUpdater for unified diff view
type UnifiedViewUpdater struct {
	diffView  *tview.TextView
	foldState *FoldState
	filePath  *string
	repoRoot  string
}

// NewUnifiedViewUpdater creates a new UnifiedViewUpdater
func NewUnifiedViewUpdater(diffView *tview.TextView, foldState *FoldState, filePath *string, repoRoot string) *UnifiedViewUpdater {
	return &UnifiedViewUpdater{
		diffView:  diffView,
		foldState: foldState,
		filePath:  filePath,
		repoRoot:  repoRoot,
	}
}

// UpdateWithoutCursor updates unified view without cursor
func (u *UnifiedViewUpdater) UpdateWithoutCursor(diffText string) {
	filePath := ""
	if u.filePath != nil {
		filePath = *u.filePath
	}
	updateDiffViewWithoutCursor(u.diffView, diffText, u.foldState, filePath, u.repoRoot)
}

// UpdateWithCursor updates unified view with cursor
func (u *UnifiedViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	filePath := ""
	if u.filePath != nil {
		filePath = *u.filePath
	}
	updateDiffViewWithCursor(u.diffView, diffText, cursorY, u.foldState, filePath, u.repoRoot)
}

// UpdateWithSelection updates unified view with selection
func (u *UnifiedViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	filePath := ""
	if u.filePath != nil {
		filePath = *u.filePath
	}
	updateDiffViewWithSelection(u.diffView, diffText, cursorY, selectStart, selectEnd, isSelecting, u.foldState, filePath, u.repoRoot)
}

// SplitViewUpdater implements DiffViewUpdater for split diff view
type SplitViewUpdater struct {
	beforeView *tview.TextView
	afterView  *tview.TextView
	filePath   *string
}

// NewSplitViewUpdater creates a new SplitViewUpdater
func NewSplitViewUpdater(beforeView, afterView *tview.TextView, filePath *string) *SplitViewUpdater {
	return &SplitViewUpdater{
		beforeView: beforeView,
		afterView:  afterView,
		filePath:   filePath,
	}
}

// UpdateWithoutCursor updates split view without cursor
func (s *SplitViewUpdater) UpdateWithoutCursor(diffText string) {
	filePath := ""
	if s.filePath != nil {
		filePath = *s.filePath
	}
	updateSplitViewWithoutCursor(s.beforeView, s.afterView, diffText, filePath)
}

// UpdateWithCursor updates split view with cursor
func (s *SplitViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	filePath := ""
	if s.filePath != nil {
		filePath = *s.filePath
	}
	updateSplitViewWithCursor(s.beforeView, s.afterView, diffText, cursorY, filePath)
}

// UpdateWithSelection updates split view with selection
func (s *SplitViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	filePath := ""
	if s.filePath != nil {
		filePath = *s.filePath
	}
	updateSplitViewWithSelection(s.beforeView, s.afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, filePath)
}

// ----------↓↓↓ unified_view_functions ↓↓↓----------

// unifiedContentCache caches the generated unified view content to avoid regeneration on cursor moves
var unifiedContentCache struct {
	diffText string
	filePath string
	content  *UnifiedViewContent
}

func getCachedUnifiedContent(diffText string, foldState *FoldState, filePath, repoRoot string) *UnifiedViewContent {
	if unifiedContentCache.diffText == diffText && unifiedContentCache.filePath == filePath && unifiedContentCache.content != nil {
		return unifiedContentCache.content
	}
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, foldState, filePath, repoRoot)
	unifiedContentCache.diffText = diffText
	unifiedContentCache.filePath = filePath
	unifiedContentCache.content = content
	return content
}

// InvalidateUnifiedContentCache clears the unified content cache (call when fold state changes)
func InvalidateUnifiedContentCache() {
	unifiedContentCache.content = nil
}

func updateDiffViewWithoutCursor(diffView *tview.TextView, diffText string, foldState *FoldState, filePath, repoRoot string) {
	renderUnifiedView(diffView, diffText, -1, -1, -1, false, foldState, filePath, repoRoot)
}

func updateDiffViewWithCursor(diffView *tview.TextView, diffText string, cursorY int, foldState *FoldState, filePath, repoRoot string) {
	renderUnifiedView(diffView, diffText, cursorY, -1, -1, false, foldState, filePath, repoRoot)
}

func updateDiffViewWithSelection(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, foldState *FoldState, filePath, repoRoot string) {
	renderUnifiedView(diffView, diffText, cursorY, selectStart, selectEnd, isSelecting, foldState, filePath, repoRoot)
}

func renderUnifiedView(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, foldState *FoldState, filePath, repoRoot string) {
	diffView.Clear()

	content := getCachedUnifiedContent(diffText, foldState, filePath, repoRoot)

	for i, line := range content.Lines {
		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行: 背景のみ dimgrey に差し替え
			highlighted := util.ReplaceBackground(line.Content, "dimgrey")
			diffView.Write([]byte("[white:dimgrey]" + line.LineNumber + "[-:-]" + highlighted + "[-:-]\n"))
		} else if i == cursorY {
			// カーソル行: 背景のみ blue に差し替え
			highlighted := util.ReplaceBackground(line.Content, "blue")
			diffView.Write([]byte("[white:blue]" + line.LineNumber + "[-:-]" + highlighted + "[-:-]\n"))
		} else {
			diffView.Write([]byte("[dimgray]" + line.LineNumber + "[-]" + line.Content + "\n"))
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

// getUnifiedViewLineCount gets valid line count for unified view
func getUnifiedViewLineCount(diffText string) int {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap, nil, "", "")
	return len(content.Lines)
}

// ----------↑↑↑ unified_view_functions ↑↑↑----------

// ----------↓↓↓ split_view_functions ↓↓↓----------

// splitContentCache caches the generated split view content to avoid regeneration on cursor moves
var splitContentCache struct {
	diffText string
	filePath string
	content  *SplitViewContent
}

func getCachedSplitContent(diffText string, filePath string) *SplitViewContent {
	if splitContentCache.diffText == diffText && splitContentCache.filePath == filePath && splitContentCache.content != nil {
		return splitContentCache.content
	}
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	content := generateSplitViewContent(diffText, oldLineMap, newLineMap, filePath)
	splitContentCache.diffText = diffText
	splitContentCache.filePath = filePath
	splitContentCache.content = content
	return content
}

// getSplitViewLineCount gets valid line count for split view
func getSplitViewLineCount(diffText string) int {
	content := getCachedSplitContent(diffText, "")
	return len(content.BeforeLines)
}

func updateSplitViewWithoutCursor(beforeView, afterView *tview.TextView, diffText string, filePath string) {
	renderSplitView(beforeView, afterView, diffText, -1, -1, -1, false, filePath)
}

// updateSplitViewWithCursor updates split view with cursor
func updateSplitViewWithCursor(beforeView, afterView *tview.TextView, diffText string, cursorY int, filePath string) {
	renderSplitView(beforeView, afterView, diffText, cursorY, -1, -1, false, filePath)
}

func updateSplitViewWithSelection(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, filePath string) {
	renderSplitView(beforeView, afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, filePath)
}

func renderSplitView(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, filePath string) {
	beforeView.Clear()
	afterView.Clear()

	content := getCachedSplitContent(diffText, filePath)
	beforeLines := content.BeforeLines
	afterLines := content.AfterLines
	beforeLineNums := content.BeforeLineNums
	afterLineNums := content.AfterLineNums

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
			// 選択行: 背景のみ dimgrey に差し替え
			highlighted := util.ReplaceBackground(line, "dimgrey")
			beforeView.Write([]byte("[white:dimgrey]" + lineNum + "[-:-]" + highlighted + "[-:-]\n"))
		} else if cursorIndex >= 0 && i == cursorIndex {
			// カーソル行: 背景のみ blue に差し替え
			highlighted := util.ReplaceBackground(line, "blue")
			beforeView.Write([]byte("[white:blue]" + lineNum + "[-:-]" + highlighted + "[-:-]\n"))
		} else {
			beforeView.Write([]byte("[dimgray]" + lineNum + "[-]" + line + "\n"))
		}
	}

	for i, line := range afterLines {
		// 行番号を追加
		lineNum := afterLineNums[i] + " │ "

		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行: 背景のみ dimgrey に差し替え
			highlighted := util.ReplaceBackground(line, "dimgrey")
			afterView.Write([]byte("[white:dimgrey]" + lineNum + "[-:-]" + highlighted + "[-:-]\n"))
		} else if cursorIndex >= 0 && i == cursorIndex {
			// カーソル行: 背景のみ blue に差し替え
			highlighted := util.ReplaceBackground(line, "blue")
			afterView.Write([]byte("[white:blue]" + lineNum + "[-:-]" + highlighted + "[-:-]\n"))
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
