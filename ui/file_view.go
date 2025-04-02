package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func ShowFileDiffText(app *tview.Application, diffText string, onExit func()) {
	coloredDiff := colorizeDiff(diffText)

	textView := tview.NewTextView().
		SetText(coloredDiff).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true).
		SetRegions(true)

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
		AddItem(debugView, 10, 1, false)

	textView.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			onExit()
		}
	})

	// 現在のカーソル位置
	cursorY := 0
	selectStart := -1
	selectEnd := -1
	isSelecting := false
	currentFocus := 0

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
					// パッチを抽出
					fileHeader := extractFileHeader(diffText, selectStart)
					patch := generateMinimalPatch(diffText, selectStart, selectEnd, fileHeader)
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
					}
					// os.Remove(patchFile) // 処理後に削除

					isSelecting = false
					selectStart = -1
					selectEnd = -1
				}
			case 'w': // 'w' でファイル一覧に戻る
				onExit() // ファイル一覧に戻る
			case 'q': // 'q' でアプリ終了
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
	lines := splitLines(diff) // 複数行に分割
	for _, line := range lines {
		if len(line) > 0 {
			switch line[0] {
			case '-': // 赤色
				result += "[red]" + line + "[-]\n"
			case '+': // 緑色
				result += "[green]" + line + "[-]\n"
			default: // 通常の色
				result += line + "\n"
			}
		} else {
			result += "\n"
		}
	}
	return result
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

func extractAddedLines(diff string, selectStart, selectEnd int) ([]string, int) {
	lines := strings.Split(diff, "\n")
	var addedLines []string
	startLine := -1

	for i := selectStart; i <= selectEnd && i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if startLine == -1 {
				startLine = i
			}
			addedLines = append(addedLines, line)
		}
	}

	return addedLines, startLine
}

func generateMinimalHunkHeader(addedStartLineInDiff int, addedLineCount int) string {
	// 元ファイルの行は変わらないので -N,0
	// 新ファイルの行数が必要
	return fmt.Sprintf("@@ -%d,0 +%d,%d @@", addedStartLineInDiff, addedStartLineInDiff, addedLineCount)
}

func generateMinimalPatch(diffText string, selectStart, selectEnd int, fileHeader string) string {
	addedLines, addedStart := extractAddedLines(diffText, selectStart, selectEnd)
	if len(addedLines) == 0 || addedStart == -1 {
		return ""
	}

	header := generateMinimalHunkHeader(addedStart, len(addedLines))
	body := strings.Join(addedLines, "\n")
	return fileHeader + "\n" + header + "\n" + body + "\n"
}
