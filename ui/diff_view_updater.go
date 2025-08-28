package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// DiffViewUpdater interface for updating diff views
type DiffViewUpdater interface {
	UpdateWithoutCursor(diffText string)
	UpdateWithCursor(diffText string, cursorY int)
	UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool)
}

// UnifiedViewUpdater implements DiffViewUpdater for unified diff view
type UnifiedViewUpdater struct {
	diffView    *tview.TextView
	currentFile *string
}

// NewUnifiedViewUpdater creates a new UnifiedViewUpdater
func NewUnifiedViewUpdater(diffView *tview.TextView) *UnifiedViewUpdater {
	return &UnifiedViewUpdater{
		diffView: diffView,
	}
}

// SetCurrentFile sets the current file for syntax highlighting
func (u *UnifiedViewUpdater) SetCurrentFile(file *string) {
	u.currentFile = file
}

// UpdateWithoutCursor updates unified view without cursor
func (u *UnifiedViewUpdater) UpdateWithoutCursor(diffText string) {
	filePath := ""
	if u.currentFile != nil {
		filePath = *u.currentFile
	}
	updateDiffViewWithoutCursorWithSyntax(u.diffView, diffText, filePath)
}

// UpdateWithCursor updates unified view with cursor
func (u *UnifiedViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	filePath := ""
	if u.currentFile != nil {
		filePath = *u.currentFile
	}
	updateDiffViewWithCursorWithSyntax(u.diffView, diffText, cursorY, filePath)
}

// UpdateWithSelection updates unified view with selection
func (u *UnifiedViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	filePath := ""
	if u.currentFile != nil {
		filePath = *u.currentFile
	}
	updateDiffViewWithSelectionWithSyntax(u.diffView, diffText, cursorY, selectStart, selectEnd, isSelecting, filePath)
}

// SplitViewUpdater implements DiffViewUpdater for split diff view
type SplitViewUpdater struct {
	beforeView  *tview.TextView
	afterView   *tview.TextView
	currentFile *string
}

// NewSplitViewUpdater creates a new SplitViewUpdater
func NewSplitViewUpdater(beforeView, afterView *tview.TextView) *SplitViewUpdater {
	return &SplitViewUpdater{
		beforeView: beforeView,
		afterView:  afterView,
	}
}

// SetCurrentFile sets the current file for syntax highlighting
func (s *SplitViewUpdater) SetCurrentFile(file *string) {
	s.currentFile = file
}

// UpdateWithoutCursor updates split view without cursor
func (s *SplitViewUpdater) UpdateWithoutCursor(diffText string) {
	filePath := ""
	if s.currentFile != nil {
		filePath = *s.currentFile
	}
	updateSplitViewWithoutCursorWithSyntax(s.beforeView, s.afterView, diffText, filePath)
}

// UpdateWithCursor updates split view with cursor
func (s *SplitViewUpdater) UpdateWithCursor(diffText string, cursorY int) {
	filePath := ""
	if s.currentFile != nil {
		filePath = *s.currentFile
	}
	updateSplitViewWithCursorWithSyntax(s.beforeView, s.afterView, diffText, cursorY, filePath)
}

// UpdateWithSelection updates split view with selection
func (s *SplitViewUpdater) UpdateWithSelection(diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	filePath := ""
	if s.currentFile != nil {
		filePath = *s.currentFile
	}
	updateSplitViewWithSelectionWithSyntax(s.beforeView, s.afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, filePath)
}

// ----------↓↓↓ unified_view_functions ↓↓↓----------
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

// With syntax highlighting versions
func updateDiffViewWithoutCursorWithSyntax(diffView *tview.TextView, diffText string, filePath string) {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMappingWithSyntax(diffView, diffText, -1, -1, -1, false, oldLineMap, newLineMap, filePath)
}

func updateDiffViewWithCursorWithSyntax(diffView *tview.TextView, diffText string, cursorY int, filePath string) {
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMappingWithSyntax(diffView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap, filePath)
}

func updateDiffViewWithSelectionWithSyntax(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, filePath string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateDiffViewWithSelectionAndMappingWithSyntax(diffView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap, filePath)
}

// updateDiffViewWithSelectionAndMapping updates diff view with selection and line mapping
func updateDiffViewWithSelectionAndMapping(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int) {
	diffView.Clear()

	// Generate content using the new function
	content := generateUnifiedViewContent(diffText, oldLineMap, newLineMap)

	renderUnifiedViewContent(diffView, content, cursorY, selectStart, selectEnd, isSelecting)
}

// updateDiffViewWithSelectionAndMappingWithSyntax updates diff view with selection, line mapping and syntax highlighting
func updateDiffViewWithSelectionAndMappingWithSyntax(diffView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int, filePath string) {
	diffView.Clear()

	// Generate content with syntax highlighting
	content := generateUnifiedViewContentWithSyntax(diffText, oldLineMap, newLineMap, filePath)

	renderUnifiedViewContent(diffView, content, cursorY, selectStart, selectEnd, isSelecting)
}

// renderUnifiedViewContent renders the unified view content
func renderUnifiedViewContent(diffView *tview.TextView, content *UnifiedViewContent, cursorY int, selectStart int, selectEnd int, isSelecting bool) {

	for i, line := range content.Lines {
		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行を黄色でハイライト
			diffView.Write([]byte("[black:dimgrey]" + line.LineNumber + line.Content + "[-:-]\n"))
		} else if i == cursorY {
			// カーソル行を青でハイライト
			diffView.Write([]byte("[white:blue]" + line.LineNumber + line.Content + "[-:-]\n"))
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
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, -1, -1, -1, false, oldLineMap, newLineMap, "", false)
}

// updateSplitViewWithCursor updates split view with cursor
func updateSplitViewWithCursor(beforeView, afterView *tview.TextView, diffText string, cursorY int) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap, "", false)
}

func updateSplitViewWithSelection(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap, "", false)
}

// シンタックスハイライト版の関数
func updateSplitViewWithoutCursorWithSyntax(beforeView, afterView *tview.TextView, diffText string, filePath string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, -1, -1, -1, false, oldLineMap, newLineMap, filePath, true)
}

func updateSplitViewWithCursorWithSyntax(beforeView, afterView *tview.TextView, diffText string, cursorY int, filePath string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap, filePath, true)
}

func updateSplitViewWithSelectionWithSyntax(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, filePath string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap, filePath, true)
}

// updateSplitViewWithSelectionAndMapping updates split view with selection and line mapping
func updateSplitViewWithSelectionAndMapping(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int, filePath string, useSyntax bool) {
	beforeView.Clear()
	afterView.Clear()

	// Generate content using the new function
	var content *SplitViewContent
	if useSyntax && filePath != "" {
		content = generateSplitViewContentWithSyntax(diffText, oldLineMap, newLineMap, filePath)
	} else {
		content = generateSplitViewContent(diffText, oldLineMap, newLineMap)
	}
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
