package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
)

var statusView *tview.TextView

func updateStatus(message string, color string) {
	if statusView != nil {
		statusView.SetText(fmt.Sprintf("[%s]%s[-]", color, message))
	}
}

func ShowFileDiffText(app *tview.Application, filePath string, debug bool, onExit func()) {
	// ファイル内容を取得して表示
	diffText, err := git.GetFileDiff(filePath)
	if err != nil {
		log.Fatalf("Failed to get file diff: %v", err)
	}

	coloredDiff := colorizeDiff(diffText)

	textView := tview.NewTextView().
		SetText(coloredDiff).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true).
		SetRegions(true)

	statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	statusView.SetBorder(false)

	// デバッグ用ウィジェット
	debugView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)

	views := []*tview.TextView{
		textView,
		debugView,
	}

	debugView.SetBorder(true).
		SetTitle("Debug view").
		SetTitleAlign(tview.AlignLeft)

	// デバッグ情報を更新する関数
	updateDebug := func(message string) {
		debugView.SetText(debugView.GetText(false) + message + "\n")
	}

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).
		AddItem(debugView, 20, 1, false)
		AddItem(statusView, 5, 0, false)

	textView.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			onExit()
		}
	})

	cursorY := 0
	selectStart := -1
	selectEnd := -1
	isSelecting := false
	currentFocus := 0

	resetCursor := func() {
		cursorY = 0
		selectStart = -1
		selectEnd = -1
		isSelecting = false
		currentFocus = 0
	}

	// テキストを描画する関数
	updateTextView := func() {
		// テキストを行ごとに分割
		lines := splitLines(coloredDiff)
		textView.Clear()

		for i, line := range lines {
			if isSelected(i, selectStart, selectEnd) {
				// 選択済みの行をハイライト
				line = "[black:yellow]" + line + "[-:-]"
			} else if i == cursorY {
				// カーソル位置の行をハイライト
				line = "[white:blue]" + line + "[-:-]"
			}
			textView.Write([]byte(line + "\n"))
		}
	}

	// キー入力の処理
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// 次のビューにフォーカス
			currentFocus = (currentFocus + 1) % len(views)
			app.SetFocus(views[currentFocus])
			return nil
		case tcell.KeyUp: // 上矢印キー
			if cursorY > 0 {
				cursorY--
			}
		case tcell.KeyDown: // 下矢印キー
			if cursorY < len(splitLines(coloredDiff))-1 {
				cursorY++
			}
		case tcell.KeyEscape: // Esc で選択モード解除
			isSelecting = false
			selectStart = -1
			selectEnd = -1
		case tcell.KeyRune: // その他のキー
			switch event.Rune() {
			case 'j': // 下移動
				if cursorY < len(splitLines(coloredDiff))-1 {
					cursorY++
					if isSelecting {
						selectEnd = cursorY
					}
				}
			case 'k': // 上移動
				if cursorY > 0 {
					cursorY--
					if isSelecting {
						selectEnd = cursorY
					}
				}
			case 'V': // Shift + V で選択モード切り替え
				if !isSelecting {
					isSelecting = true
					selectStart = cursorY
					selectEnd = cursorY
				}
			case 'U':
				if selectStart != -1 && selectEnd != -1 {
					mapping := mapDisplayIndexToOriginalIndex(diffText)
					start := mapping[selectStart]
					end := mapping[selectEnd]
					// パッチを抽出
					fileHeader := extractFileHeader(diffText, start)
					patch := generateMinimalPatch(diffText, start, end, fileHeader, updateDebug)
					updateDebug("Generated Patch:\n" + patch)

					// パッチを一時ファイルに保存
					patchFile := "selected.patch"
					if err := os.WriteFile(patchFile, []byte(patch), 0644); err != nil {
						fmt.Println("Failed to write patch file:", err)
						onExit() // ファイル一覧に戻る
					}

					// git apply を実行
					cmd := exec.Command("git", "apply", "--cached", patchFile)
					output, err := cmd.CombinedOutput()
					if err != nil {
						updateDebug(fmt.Sprintf("Failed to apply patch:\n%s", string(output)))
					} else {
						updateDebug("Patch applied successfully!")
						diffText, err = git.GetFileDiff(filePath)
						if err != nil {
							log.Fatalf("Failed to get file diff: %v", err)
						}
						coloredDiff = colorizeDiff(diffText)
						resetCursor()
						updateTextView()
					}
					// os.Remove(patchFile) // 処理後にパッチファイルを削除

					resetCursor()
				}
			case 'w': // 'w' でファイル一覧に戻る
				onExit() // ファイル一覧に戻る
			case 'q': // 'q' でアプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				app.Stop()
			}
		}

		updateTextView()
		return nil
	})

	debugScrollY := 0
	debugView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// 次のビューにフォーカス
			currentFocus = (currentFocus + 1) % len(views)
			app.SetFocus(views[currentFocus])
			return nil
		case tcell.KeyRune: // j/k キーでスクロール
			switch event.Rune() {
			case 'j':
				debugScrollY++
				debugView.ScrollTo(debugScrollY, 0)
			case 'k':
				if debugScrollY > 0 {
					debugScrollY--
				}
				debugView.ScrollTo(debugScrollY, 0)
			}
		}

		return nil
	})

	// 初期描画
	updateTextView()
	app.SetRoot(flex, true).Run()
}

// colorizeDiff は Diff を色付けします
func colorizeDiff(diff string) string {
	var result string
	lines := splitLines(diff)
	for _, line := range lines {
		// 🎯 ここでスキップしたいヘッダー行を除外
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue // ← 表示しない
		}

		// 色付け処理（+/-）
		if len(line) > 0 {
			switch line[0] {
			case '-':
				result += "[red]" + line + "[-]\n"
			case '+':
				result += "[green]" + line + "[-]\n"
			default:
				result += line + "\n"
			}
		} else {
			result += "\n"
		}
	}
	return result
}

func mapDisplayIndexToOriginalIndex(diff string) map[int]int {
	lines := splitLines(diff)
	displayIndex := 0
	mapping := make(map[int]int) // displayIndex -> originalIndex

	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue // 表示に含めない
		}

		mapping[displayIndex] = i
		displayIndex++
	}

	return mapping
}

// splitLines は文字列を改行で分割します
func splitLines(input string) []string {
	lines := []string{}
	currentLine := ""
	for _, r := range input {
		if r == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(r)
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

// isSelected は指定した行が選択範囲内かどうかを判定します
func isSelected(line, start, end int) bool {
	if start == -1 || end == -1 {
		return false
	}
	if start > end {
		start, end = end, start
	}
	return line >= start && line <= end
}

func extractFileHeader(diff string, startLine int) string {
	lines := strings.Split(diff, "\n")
	var header []string

	// 対象行より前を逆順にたどって、diff ヘッダーを見つける
	for i := startLine; i >= 0; i-- {
		line := lines[i]
		if strings.HasPrefix(line, "diff --git ") {
			// ヘッダーの先頭見つけたら、そこから3〜4行分取り出す
			for j := 0; j < 5 && i+j < len(lines); j++ {
				hline := lines[i+j]
				if strings.HasPrefix(hline, "index ") || strings.HasPrefix(hline, "--- ") || strings.HasPrefix(hline, "+++ ") || strings.HasPrefix(hline, "diff --git ") {
					header = append(header, hline)
				} else {
					break
				}
			}
			break
		}
	}
	return strings.Join(header, "\n")
}

type PatchLine struct {
	Line     string
	Original int
}

func generateMinimalPatch(diffText string, selectStart, selectEnd int, fileHeader string, updateDebug func(message string)) string {
	lines, start := extractSelectedLinesWithContext(diffText, selectStart, selectEnd)
	if len(lines) == 0 || start == -1 {
		return ""
	}

	allLines := splitLines(diffText)
	startLine := findHunkStartLineInFile(allLines, start)
	if startLine == -1 {
		updateDebug("Could not find hunk header for selected lines")
		return ""
	}

	header := generateFullHunkHeader(startLine, lines)

	var body strings.Builder
	for _, pl := range lines {
		body.WriteString(pl.Line + "\n")
	}

	return fileHeader + "\n" + header + "\n" + body.String()
}

// 選択行の上下に最大3行ずつ context (" ") 行を含めてパッチ化する
func extractSelectedLinesWithContext(diff string, selectStart, selectEnd int) ([]PatchLine, int) {
	lines := splitLines(diff)
	var result []PatchLine
	firstLine := -1
	seen := make(map[int]bool) // 重複防止

	// 上方向の context 行（最大3行）
	contextLines := 3
	count := 0
	for i := selectStart - 1; i >= 0 && count < contextLines; i-- {
		if strings.HasPrefix(lines[i], " ") || lines[i] == "" {
			result = append([]PatchLine{{Line: lines[i], Original: i}}, result...) // 先頭に追加
			seen[i] = true
			firstLine = i
			count++
		} else if strings.HasPrefix(lines[i], "@@") || strings.HasPrefix(lines[i], "diff --git") {
			break // hunk 跨ぎ禁止
		}
	}

	// 選択された範囲の + / - 行
	for i := selectStart; i <= selectEnd && i < len(lines); i++ {
		result = append(result, PatchLine{Line: lines[i], Original: i})
		seen[i] = true
		if firstLine == -1 {
			firstLine = i
		}
	}

	// 下方向の context 行（最大3行）
	count = 0
	for i := selectEnd + 1; i < len(lines) && count < contextLines; i++ {
		if strings.HasPrefix(lines[i], " ") || lines[i] == "" {
			if seen[i] {
				continue
			}
			result = append(result, PatchLine{Line: lines[i], Original: i})
			count++
		} else if strings.HasPrefix(lines[i], "@@") || strings.HasPrefix(lines[i], "diff --git") {
			break
		}
	}

	return result, firstLine
}

func generateFullHunkHeader(startLine int, selected []PatchLine) string {
	delCount := 0
	addCount := 0

	for _, pl := range selected {
		switch {
		case strings.HasPrefix(pl.Line, "-") && !strings.HasPrefix(pl.Line, "---"):
			delCount++
		case strings.HasPrefix(pl.Line, "+") && !strings.HasPrefix(pl.Line, "+++"):
			addCount++
		case strings.HasPrefix(pl.Line, " ") || pl.Line == "":
			delCount++
			addCount++
		}
	}

	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", startLine, delCount, startLine, addCount)
}

func findHunkStartLineInFile(diffLines []string, targetIndex int) int {
	hunkRegex := regexp.MustCompile(`@@ -(\d+),\d+ \+\d+,\d+ @@`)

	for i := targetIndex; i >= 0; i-- {
		if strings.HasPrefix(diffLines[i], "@@") {
			match := hunkRegex.FindStringSubmatch(diffLines[i])
			if len(match) == 2 {
				if line, err := strconv.Atoi(match[1]); err == nil {
					return line
				}
			}
			break
		}
	}
	return -1
}
