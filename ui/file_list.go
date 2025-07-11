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

// 保持するカーソル情報
var savedCursorPosition int = -1
var savedTargetFile string = ""
var preferUnstagedSection bool = false

// ファイル一覧を表示
func ShowFileList(app *tview.Application, stagedFiles, modifiedFiles, untrackedFiles []string, repoRoot string, onSelect func(file string, status string), onUpdate func()) tview.Primitive {
	// ファイルリストを更新するための参照を保持
	stagedFilesPtr := &stagedFiles
	modifiedFilesPtr := &modifiedFiles
	untrackedFilesPtr := &untrackedFiles
	// フレックスレイアウトを作成（左右分割）
	flex := tview.NewFlex()

	// 左ペイン（ファイルリスト）のテキストビューを作成
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	textView.SetBackgroundColor(util.MyColor.BackgroundColor)

	// 右ペイン（差分表示）のテキストビューを作成
	diffView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	diffView.SetBackgroundColor(util.MyColor.BackgroundColor)

	// 現在のファイル情報を保持
	var currentFile string
	var currentStatus string
	var diffLines []string
	var currentDiffText string // 生の差分テキストを保持
	var cursorY int = 0
	var selectStart int = -1
	var selectEnd int = -1
	var isSelecting bool = false
	var currentSelection int = 0
	var leftPaneFocused bool = true

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

	// 表示更新関数の宣言
	var updateFileListView func()
	var refreshFileList func()

	// ファイル一覧を構築するための変数
	var regions []string
	var fileMap = make(map[string]string)
	var fileStatusMap = make(map[string]string)

	// 右ペインのキー入力処理を設定（file_view.goと同じ動作）
	diffView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			// 選択モードを解除するか、左ペインに戻る
			if isSelecting {
				isSelecting = false
				selectStart = -1
				selectEnd = -1
				updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
			} else {
				app.SetFocus(textView)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'w':
				// 'w'キーで左ペインに戻る
				// カーソルと選択モードをリセット
				cursorY = 0
				isSelecting = false
				selectStart = -1
				selectEnd = -1
				// 表示を更新（カーソルなし）
				updateDiffViewWithoutCursor(diffView, diffLines)
				// 左ペインにフォーカスを戻す
				leftPaneFocused = true
				updateFileListView()
				app.SetFocus(textView)
				return nil
			case 'j':
				// 下移動
				if cursorY < len(diffLines)-1 {
					cursorY++
					if isSelecting {
						selectEnd = cursorY
					}
					updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
				}
				return nil
			case 'k':
				// 上移動
				if cursorY > 0 {
					cursorY--
					if isSelecting {
						selectEnd = cursorY
					}
					updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
				}
				return nil
			case 'V':
				// Shift + V で選択モード開始
				if !isSelecting {
					isSelecting = true
					selectStart = cursorY
					selectEnd = cursorY
					updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
				}
				return nil
			case 'a':
				// 選択した行をステージング
				if selectStart != -1 && selectEnd != -1 && currentFile != "" && currentDiffText != "" {
					if currentStatus == "staged" {
						// Staged ファイルでは行単位のunstageは未対応
						return nil
					}

					// パッチファイルのパスを生成
					patchPath := "/tmp/gitta_selected.patch"

					// パッチを生成（file_view.goと同じロジック）
					mapping := mapDisplayToOriginalIdx(currentDiffText)
					start := mapping[selectStart]
					end := mapping[selectEnd]

					fileHeader := extractFileHdr(currentDiffText, start)
					patch := GenerateMinimalPatch(currentDiffText, start, end, fileHeader, nil)

					// パッチファイルに書き込む
					if err := os.WriteFile(patchPath, []byte(patch), 0644); err == nil {
						// git applyを実行
						cmd := exec.Command("git", "apply", "--cached", patchPath)
						cmd.Dir = repoRoot
						_, err := cmd.CombinedOutput()
						if err == nil {
							// 差分を再取得
							var newDiffText string
							if currentStatus == "staged" {
								newDiffText, _ = git.GetStagedDiff(currentFile, repoRoot)
							} else {
								newDiffText, _ = git.GetFileDiff(currentFile, repoRoot)
							}
							currentDiffText = newDiffText

							// ColorizeDiffで色付け
							coloredDiff := ColorizeDiff(currentDiffText)
							diffLines = SplitLines(coloredDiff)

							// 選択を解除してカーソルリセット
							isSelecting = false
							selectStart = -1
							selectEnd = -1
							cursorY = 0

							// 再描画
							updateDiffView(diffView, diffLines, cursorY)

							// 現在のファイルを保存
							savedFile := currentFile

							// ファイルリストを内部的に更新
							refreshFileList()

							// 差分が残っている場合
							if len(strings.TrimSpace(newDiffText)) > 0 {
								// 同じファイルのインデックスを探す
								foundIndex := -1
								allFiles := []string{}

								// 全ファイルリストを作成
								for _, file := range *stagedFilesPtr {
									file = strings.TrimSpace(file)
									if file != "" {
										allFiles = append(allFiles, file)
										if file == savedFile {
											foundIndex = len(allFiles) - 1
										}
									}
								}
								for _, file := range *modifiedFilesPtr {
									file = strings.TrimSpace(file)
									if file != "" {
										allFiles = append(allFiles, file)
										if file == savedFile {
											foundIndex = len(allFiles) - 1
										}
									}
								}
								for _, file := range *untrackedFilesPtr {
									file = strings.TrimSpace(file)
									if file != "" {
										allFiles = append(allFiles, file)
										if file == savedFile {
											foundIndex = len(allFiles) - 1
										}
									}
								}

								// ファイルが見つかった場合はカーソルを設定
								if foundIndex != -1 {
									currentSelection = foundIndex
								}

								// ファイルリストを再描画
								updateFileListView()
							} else {
								// 差分がなくなった場合は、完全に更新
								if onUpdate != nil {
									onUpdate()
								}
							}
						} else if err != nil {
							// エラーの場合でもパッチファイルは削除（エラーは表示しない）
						}
						os.Remove(patchPath)
					}
				}
				return nil
			case 'A':
				// 現在のファイルをステージ/アンステージ
				if currentFile != "" {
					var cmd *exec.Cmd
					if currentStatus == "staged" {
						cmd = exec.Command("git", "reset", "HEAD", currentFile)
					} else {
						cmd = exec.Command("git", "add", currentFile)
					}
					cmd.Dir = repoRoot

					err := cmd.Run()
					if err == nil {
						wasStaged := (currentStatus == "staged")

						if currentStatus == "staged" {
							// unstagedになったファイルの差分を表示
							currentStatus = "unstaged"
							newDiffText, _ := git.GetFileDiff(currentFile, repoRoot)
							currentDiffText = newDiffText
						} else {
							// stagedになったファイルの差分を表示
							currentStatus = "staged"
							newDiffText, _ := git.GetStagedDiff(currentFile, repoRoot)
							currentDiffText = newDiffText
						}

						// 差分を更新して表示
						coloredDiff := ColorizeDiff(currentDiffText)
						diffLines = SplitLines(coloredDiff)

						// カーソルと選択をリセット
						isSelecting = false
						selectStart = -1
						selectEnd = -1
						cursorY = 0

						// 再描画
						updateDiffView(diffView, diffLines, cursorY)

						// refreshFileListを呼んで最新の状態を取得
						refreshFileList()

						// カーソル位置を保存
						if wasStaged || len(*modifiedFilesPtr) > 0 {
							// staged -> unstaged の場合、または unstaged ファイルが残っている場合
							// unstagedセクションの先頭を選択するように設定
							preferUnstagedSection = true
							savedTargetFile = ""
						} else {
							// unstagedファイルがない場合は、通常の動作
							preferUnstagedSection = false
							savedTargetFile = ""
						}

						// ファイルリストを更新
						if onUpdate != nil {
							onUpdate()
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
				app.Stop()
				return nil
			}
		}
		return event
	})

	// 縦線を作成
	border := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// 縦線を描画
			for i := y; i < y+height; i++ {
				screen.SetContent(x, i, '│', nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
			}
			return x, y, width, height
		})
	border.SetBackgroundColor(util.MyColor.BackgroundColor)

	// 左右のペインをフレックスに追加（左:縦線:右 = 1:0:2）
	flex.AddItem(textView, 0, 1, true).
		AddItem(border, 1, 0, false).
		AddItem(diffView, 0, 2, false)

	// ファイル一覧を構築
	var content strings.Builder

	// Staged ファイル
	if len(*stagedFilesPtr) > 0 {
		content.WriteString("[green]Changes to be committed:[white]\n")
		for _, file := range *stagedFilesPtr {
			file = strings.TrimSpace(file)
			if file != "" {
				regionID := fmt.Sprintf("file-%d", len(regions))
				regions = append(regions, regionID)
				fileMap[regionID] = file
				fileStatusMap[regionID] = "staged"
				content.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", len(regions)-1, file))
			}
		}
		content.WriteString("\n")
	}

	// 変更されたファイル（unstaged）
	if len(*modifiedFilesPtr) > 0 {
		content.WriteString("[yellow]Changes not staged for commit:[white]\n")
		for _, file := range *modifiedFilesPtr {
			file = strings.TrimSpace(file)
			if file != "" {
				regionID := fmt.Sprintf("file-%d", len(regions))
				regions = append(regions, regionID)
				fileMap[regionID] = file
				fileStatusMap[regionID] = "unstaged"
				content.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", len(regions)-1, file))
			}
		}
		content.WriteString("\n")
	}

	// 未追跡ファイル
	if len(*untrackedFilesPtr) > 0 {
		content.WriteString("[red]Untracked files:[white]\n")
		for _, file := range *untrackedFilesPtr {
			file = strings.TrimSpace(file)
			if file != "" {
				regionID := fmt.Sprintf("file-%d", len(regions))
				regions = append(regions, regionID)
				fileMap[regionID] = file
				fileStatusMap[regionID] = "untracked"
				content.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", len(regions)-1, file))
			}
		}
	}

	// ファイル一覧の内容を構築（色付き）
	buildFileListContent := func(focusedPane bool) string {
		// regions と fileMap を再構築
		regions = []string{}
		fileMap = make(map[string]string)
		fileStatusMap = make(map[string]string)

		var coloredContent strings.Builder
		regionIndex := 0

		// Staged ファイル
		if len(*stagedFilesPtr) > 0 {
			coloredContent.WriteString("[green]Changes to be committed:[white]\n")
			for _, file := range *stagedFilesPtr {
				file = strings.TrimSpace(file)
				if file != "" {
					regionID := fmt.Sprintf("file-%d", regionIndex)
					regions = append(regions, regionID)
					fileMap[regionID] = file
					fileStatusMap[regionID] = "staged"

					if focusedPane && regionIndex == currentSelection {
						// フォーカス時は青色ハイライト
						coloredContent.WriteString(fmt.Sprintf(`[white:blue]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else if !focusedPane && regionIndex == currentSelection {
						// 非フォーカス時は白色ハイライト
						coloredContent.WriteString(fmt.Sprintf(`[black:white]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else {
						coloredContent.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", regionIndex, file))
					}
					regionIndex++
				}
			}
			coloredContent.WriteString("\n")
		}

		// 変更されたファイル（unstaged）
		if len(*modifiedFilesPtr) > 0 {
			coloredContent.WriteString("[yellow]Changes not staged for commit:[white]\n")
			for _, file := range *modifiedFilesPtr {
				file = strings.TrimSpace(file)
				if file != "" {
					regionID := fmt.Sprintf("file-%d", regionIndex)
					regions = append(regions, regionID)
					fileMap[regionID] = file
					fileStatusMap[regionID] = "unstaged"

					if focusedPane && regionIndex == currentSelection {
						coloredContent.WriteString(fmt.Sprintf(`[white:blue]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else if !focusedPane && regionIndex == currentSelection {
						coloredContent.WriteString(fmt.Sprintf(`[black:white]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else {
						coloredContent.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", regionIndex, file))
					}
					regionIndex++
				}
			}
			coloredContent.WriteString("\n")
		}

		// 未追跡ファイル
		if len(*untrackedFilesPtr) > 0 {
			coloredContent.WriteString("[red]Untracked files:[white]\n")
			for _, file := range *untrackedFilesPtr {
				file = strings.TrimSpace(file)
				if file != "" {
					regionID := fmt.Sprintf("file-%d", regionIndex)
					regions = append(regions, regionID)
					fileMap[regionID] = file
					fileStatusMap[regionID] = "untracked"

					if focusedPane && regionIndex == currentSelection {
						coloredContent.WriteString(fmt.Sprintf(`[white:blue]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else if !focusedPane && regionIndex == currentSelection {
						coloredContent.WriteString(fmt.Sprintf(`[black:white]["file-%d"]  %s[""][-:-]`+"\n", regionIndex, file))
					} else {
						coloredContent.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", regionIndex, file))
					}
					regionIndex++
				}
			}
		}

		return coloredContent.String()
	}

	// 初期表示を更新
	updateFileListView = func() {
		textView.Clear()
		textView.SetText(buildFileListContent(leftPaneFocused))
	}

	// ファイルリストを内部的に更新する関数
	refreshFileList = func() {
		// 新しいファイルリストを取得
		newStaged, newModified, newUntracked, err := git.GetChangedFiles(repoRoot)
		if err == nil {
			*stagedFilesPtr = newStaged
			*modifiedFilesPtr = newModified
			*untrackedFilesPtr = newUntracked
		}
	}

	updateFileListView()

	// キー入力の処理
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if currentSelection > 0 {
				currentSelection--
				updateFileListView()
			}
			return nil
		case tcell.KeyDown:
			if currentSelection < len(regions)-1 {
				currentSelection++
				updateFileListView()
			}
			return nil
		case tcell.KeyEnter:
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

				// 右ペインに差分を表示
				diffLines = ShowDiffInPane(diffView, file, status, repoRoot, cursorY, &currentDiffText)

				// フォーカスを右ペインに移動
				leftPaneFocused = false
				updateFileListView()
				app.SetFocus(diffView)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				if currentSelection > 0 {
					currentSelection--
					updateFileListView()
				}
				return nil
			case 'j':
				if currentSelection < len(regions)-1 {
					currentSelection++
					updateFileListView()
				}
				return nil
			case 'A': // 'A' で現在のファイルを git add/reset
				if currentSelection >= 0 && currentSelection < len(regions) {
					regionID := regions[currentSelection]
					file := fileMap[regionID]
					status := fileStatusMap[regionID]

					var cmd *exec.Cmd
					if status == "staged" {
						// stagedファイルをunstageする
						cmd = exec.Command("git", "reset", "HEAD", file)
						cmd.Dir = repoRoot
					} else {
						// unstaged/untrackedファイルをstageする
						cmd = exec.Command("git", "add", file)
						cmd.Dir = repoRoot
					}

					err := cmd.Run()
					if err != nil {
						// エラーハンドリング（ここでは簡単にスキップ）
						return nil
					}

					// ファイルリストを更新
					if onUpdate != nil {
						onUpdate()
					}
				}
				return nil
			case 'q': // 'q' でアプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				app.Stop()
			}
		}
		return event
	})

	return flex
}

// 右ペインに差分を表示する関数
func ShowDiffInPane(diffView *tview.TextView, filePath string, status string, repoRoot string, cursorY int, diffTextPtr *string) []string {
	// ファイルの差分を取得
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
	if diffTextPtr != nil {
		*diffTextPtr = diffText
	}

	// ColorizeDiffを使って色付けとヘッダー除外
	coloredDiff := ColorizeDiff(diffText)

	// 表示用の行を返す（カーソル表示のため）- file_view.goと同じsplitLinesを使用
	lines := SplitLines(coloredDiff)

	// カーソル付きで表示（SetTextは不要、updateDiffViewが処理する）
	updateDiffView(diffView, lines, cursorY)

	return lines
}

// 差分ビューを更新する関数
func updateDiffView(diffView *tview.TextView, lines []string, cursorY int) {
	updateDiffViewWithSelection(diffView, lines, cursorY, -1, -1, false)
}

// 選択範囲付きで差分ビューを更新する関数
func updateDiffViewWithSelection(diffView *tview.TextView, lines []string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	diffView.Clear()

	for i, line := range lines {
		if isSelecting && isLineSelected(i, selectStart, selectEnd) {
			// 選択行を黄色でハイライト
			diffView.Write([]byte("[black:yellow]" + line + "[-:-]\n"))
		} else if i == cursorY {
			// カーソル行を青でハイライト
			diffView.Write([]byte("[white:blue]" + line + "[-:-]\n"))
		} else {
			diffView.Write([]byte(line + "\n"))
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

// カーソルなしで差分ビューを更新する関数
func updateDiffViewWithoutCursor(diffView *tview.TextView, lines []string) {
	diffView.Clear()

	for _, line := range lines {
		diffView.Write([]byte(line + "\n"))
	}

	// スクロール位置を先頭に
	diffView.ScrollTo(0, 0)
}

// 行が選択範囲内かどうかを判定
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

// file_view.goから必要な関数を移植（名前を変更して重複を回避）
func mapDisplayToOriginalIdx(diff string) map[int]int {
	lines := SplitLines(diff)
	displayIndex := 0
	mapping := make(map[int]int)

	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}

		mapping[displayIndex] = i
		displayIndex++
	}

	return mapping
}

func extractFileHdr(diff string, startLine int) string {
	lines := SplitLines(diff)
	var header []string

	for i := startLine; i >= 0; i-- {
		line := lines[i]
		if strings.HasPrefix(line, "diff --git ") {
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
