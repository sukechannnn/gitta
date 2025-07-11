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
	"github.com/sukechannnn/gitta/util"
)

var gPressed bool
var lastGTime time.Time

var statusView *tview.TextView
var keyBindingMessage = "Press 'w' to go back to the file list, 'q' to quit, 'u' to undo, 'a' to apply patch, 'A' to apply all changes, 'V' to select lines, and 'j/k' to scroll up/down."

func updateStatus(message string, color string) {
	if statusView != nil {
		statusView.SetText(fmt.Sprintf("[%s]%s[-]", color, message))
		go func() {
			time.Sleep(5 * time.Second)
			statusView.SetText(keyBindingMessage)
		}()
	}
}

func ShowFileDiffText(app *tview.Application, filePath string, status string, debug bool, patchFilePath string, repoRoot string, onExit func()) tview.Primitive {
	// ファイル内容を取得して表示
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

	coloredDiff := ColorizeDiff(diffText)
	lines := SplitLines(diffText)
	lineLength := len(lines)

	textView := tview.NewTextView().
		SetText(coloredDiff).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true).
		SetRegions(true)
	textView.SetBackgroundColor(util.MyColor.BackgroundColor)

	statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)
	statusView.SetBorder(true)
	statusView.SetBackgroundColor(util.MyColor.BackgroundColor)

	debugView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	debugView.SetBorder(true).
		SetTitle("Debug view").
		SetTitleAlign(tview.AlignLeft)

	views := []*tview.TextView{textView}
	if debug {
		views = append(views, debugView)
	}

	updateDebug := func(message string) {
		debugView.SetText(debugView.GetText(false) + message + "\n")
	}

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusView, 5, 0, false).
		AddItem(textView, 0, 1, true)

	if debug {
		flex.AddItem(debugView, 20, 1, false)
	}

	textView.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			onExit()
		}
	})

	statusView.SetText(keyBindingMessage)

	cursorY := 0
	scrollY := 0
	selectStart := -1
	selectEnd := -1
	isSelecting := false
	currentFocus := 0

	resetCursor := func() {
		if cursorY > lineLength {
			cursorY = lineLength - 1
			scrollY = scrollY - (cursorY - lineLength + 1)
		}
		selectStart = -1
		selectEnd = -1
		isSelecting = false
		currentFocus = 0
	}

	// テキストを描画する関数
	updateTextView := func() {
		// テキストを行ごとに分割
		lines := SplitLines(coloredDiff)
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
		lineLength = len(lines)
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
			if cursorY < len(SplitLines(coloredDiff))-1 {
				cursorY++
			}
		case tcell.KeyEscape: // Esc で選択モード解除
			isSelecting = false
			selectStart = -1
			selectEnd = -1
		case tcell.KeyRune: // その他のキー
			switch event.Rune() {
			case 'j': // 下移動
				if cursorY < len(SplitLines(coloredDiff))-1 {
					cursorY++
					if isSelecting {
						selectEnd = cursorY
					}
				}

				_, _, _, height := textView.GetInnerRect()
				if cursorY >= scrollY+height-1 {
					scrollY = cursorY - height + 2 // 1〜2 行余裕をもたせる
				}
				textView.ScrollTo(scrollY, 0)
			case 'k': // 上移動
				if cursorY > 0 {
					cursorY--
					if isSelecting {
						selectEnd = cursorY
					}
				}

				if cursorY < scrollY {
					scrollY = cursorY
				}
				textView.ScrollTo(scrollY, 0)
			case 'g':
				now := time.Now()
				if gPressed && now.Sub(lastGTime) < 500*time.Millisecond {
					// gg → 最上部
					cursorY = 0
					scrollY = 0
					textView.ScrollTo(scrollY, 0)
					gPressed = false
				} else {
					// 1回目のg
					gPressed = true
					lastGTime = now
				}
			case 'G': // 大文字G → 最下部へ
				cursorY = len(SplitLines(coloredDiff)) - 1
				_, _, _, height := textView.GetInnerRect()
				scrollY = cursorY - height + 2
				if scrollY < 0 {
					scrollY = 0
				}
				textView.ScrollTo(scrollY, 0)
			case 'V': // Shift + V で選択モード切り替え
				if !isSelecting {
					isSelecting = true
					selectStart = cursorY
					selectEnd = cursorY
				}
			case 'u':
				cmd := exec.Command("git", "apply", "-R", "--cached", patchFilePath)
				cmd.Dir = repoRoot
				output, err := cmd.CombinedOutput()
				if err != nil {
					message := "Undo failed!" + "\n" + "Please use debug mode to see more details: gitta --debug"
					updateStatus(message, "firebrick")
					updateDebug(fmt.Sprintf("[firebrick]Failed to undo patch:\n%s[-]", string(output)))
				} else {
					updateStatus("Undo successful!", "gold")

					// 再描画用に diff 更新
					diffText, err = git.GetFileDiff(filePath, repoRoot)
					if err != nil {
						updateDebug("Failed to get file diff after undo: " + err.Error())
					} else {
						coloredDiff = ColorizeDiff(diffText)
						updateTextView()
						resetCursor()
					}
				}
			case 'a':
				if status == "staged" {
					// Staged ファイルでは行単位のunstageは複雑なため、現在未対応
					updateStatus("Line-by-line unstaging not supported for staged files. Use 'A' to unstage entire file.", "firebrick")
				} else {
					// Unstaged ファイルでは通常の行単位ステージング
					if selectStart != -1 && selectEnd != -1 {
						mapping := mapDisplayIndexToOriginalIndex(diffText)
						start := mapping[selectStart]
						end := mapping[selectEnd]
						// パッチを抽出
						fileHeader := extractFileHeader(diffText, start)
						patch := GenerateMinimalPatch(diffText, start, end, fileHeader, updateDebug)
						updateDebug("Generated Patch:\n" + patch)

						if err := os.WriteFile(patchFilePath, []byte(patch), 0644); err != nil {
							fmt.Println("Failed to write patch file:", err)
							onExit() // ファイル一覧に戻る
						}

						// git apply を実行
						cmd := exec.Command("git", "apply", "--cached", patchFilePath)
						cmd.Dir = repoRoot
						output, err := cmd.CombinedOutput()
						if err != nil {
							message := fmt.Sprintf("Failed to apply patch:\n%s", string(output)+"\n"+"Please use debug mode to see more details: gitta --debug")
							updateStatus(message, "firebrick")
							updateDebug(fmt.Sprintf("Failed to apply patch:\n%s", string(output)))
						} else {
							updateStatus("Patch applied successfully!", "gold")
							if status == "staged" {
								diffText, err = git.GetStagedDiff(filePath, repoRoot)
							} else {
								diffText, err = git.GetFileDiff(filePath, repoRoot)
							}
							if err != nil {
								diffText = fmt.Sprintf("Error getting updated diff for %s: %v", filePath, err)
							}
							coloredDiff = ColorizeDiff(diffText)
							updateTextView()
							resetCursor()
						}
						resetCursor()
					}
				}
			case 'A':
				if status == "staged" {
					// Staged ファイルの場合はunstageする
					cmd := exec.Command("git", "reset", "HEAD", filePath)
					cmd.Dir = repoRoot
					output, err := cmd.CombinedOutput()
					if err != nil {
						message := fmt.Sprintf("Failed to unstage file:\n%s", string(output))
						updateStatus(message, "firebrick")
						updateDebug(fmt.Sprintf("Failed to unstage file:\n%s", string(output)))
					} else {
						updateStatus("File unstaged successfully!", "gold")
						// ファイル一覧に戻る
						onExit()
						os.Remove(patchFilePath)
					}
				} else {
					// Unstaged ファイルの場合はstageする
					cmd := exec.Command("git", "add", filePath)
					cmd.Dir = repoRoot
					output, err := cmd.CombinedOutput()
					if err != nil {
						message := fmt.Sprintf("Failed to apply all changes:\n%s", string(output))
						updateStatus(message, "firebrick")
						updateDebug(fmt.Sprintf("Failed to apply all changes:\n%s", string(output)))
					} else {
						updateStatus("All changes applied successfully!", "gold")
						// ファイル一覧に戻る
						onExit()
						os.Remove(patchFilePath)
					}
				}
			// case 'C': // Shift + c
			// 	ShowCommitScreen(app, filePath, func() {
			// 		onExit() // ファイル一覧に戻る
			// 		os.Remove(patchFilePath)
			// 	})
			case 'w': // 'w' でファイル一覧に戻る
				onExit() // ファイル一覧に戻る
				os.Remove(patchFilePath)
			case 'q': // 'q' でアプリ終了
				os.Remove(patchFilePath)
				app.Stop() // tview アプリケーションを停止
				os.Exit(0) // プロセスを終了
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
	return flex
}

func mapDisplayIndexToOriginalIndex(diff string) map[int]int {
	lines := SplitLines(diff)
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

