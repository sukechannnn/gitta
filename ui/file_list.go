package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/ui/commands"
	"github.com/sukechannnn/gitta/util"
)

// 保持するカーソル情報
var savedTargetFile string = ""
var preferUnstagedSection bool = false

// パッチファイルのパス
var patchPath = "/tmp/gitta_selected.patch"

// TreeNode represents a node in the file tree structure
type TreeNode struct {
	Name     string
	IsFile   bool
	Children map[string]*TreeNode
	FullPath string // ファイルの場合のみ使用
}

// buildFileTree converts a list of file paths into a tree structure
func buildFileTree(files []string) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsFile:   false,
		Children: make(map[string]*TreeNode),
	}

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		parts := strings.Split(file, "/")
		currentNode := root

		for i, part := range parts {
			isLastPart := i == len(parts)-1

			if _, exists := currentNode.Children[part]; !exists {
				newNode := &TreeNode{
					Name:     part,
					IsFile:   isLastPart,
					Children: make(map[string]*TreeNode),
				}
				if isLastPart {
					newNode.FullPath = file
				}
				currentNode.Children[part] = newNode
			}

			currentNode = currentNode.Children[part]
		}
	}

	return root
}

// renderFileTree recursively renders the tree structure with proper indentation
func renderFileTree(node *TreeNode, depth int, sb *strings.Builder,
	regions *[]string, fileMap map[string]string, fileStatusMap map[string]string,
	status string, regionIndex *int, currentSelection int, focusedPane bool, lineNumberMap map[int]int, currentLine *int) {

	renderFileTreeWithPrefix(node, depth, "", sb, regions, fileMap, fileStatusMap,
		status, regionIndex, currentSelection, focusedPane, lineNumberMap, currentLine)
}

// renderFileTreeWithPrefix renders the tree with proper line prefixes
func renderFileTreeWithPrefix(node *TreeNode, depth int, prefix string, sb *strings.Builder,
	regions *[]string, fileMap map[string]string, fileStatusMap map[string]string,
	status string, regionIndex *int, currentSelection int, focusedPane bool, lineNumberMap map[int]int, currentLine *int) {

	// Sort children for consistent ordering
	var sortedKeys []string
	for key := range node.Children {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// ディレクトリとファイルを分離
	var directories []string
	var files []string

	for _, key := range sortedKeys {
		child := node.Children[key]
		if child.IsFile {
			files = append(files, key)
		} else {
			directories = append(directories, key)
		}
	}

	// 全ての要素（ディレクトリ＋ファイル）を処理
	allItems := append(directories, files...)

	for i, key := range allItems {
		isLast := i == len(allItems)-1
		child := node.Children[key]

		// 現在の要素の接続記号
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// 次の階層のためのプレフィックス
		childPrefix := prefix + "│   "
		if isLast {
			childPrefix = prefix + "    "
		}

		if child.IsFile {
			// ファイルの場合
			regionID := fmt.Sprintf("file-%d", *regionIndex)
			*regions = append(*regions, regionID)
			fileMap[regionID] = child.FullPath
			fileStatusMap[regionID] = status

			if focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[white:blue]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, child.Name))
			} else if !focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[black:white]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, child.Name))
			} else {
				sb.WriteString(fmt.Sprintf(`%s[white:%s]["file-%d"]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, *regionIndex, connector, child.Name))
			}
			lineNumberMap[*regionIndex] = *currentLine
			(*regionIndex)++
			(*currentLine)++
		} else {
			// ディレクトリの場合
			sb.WriteString(fmt.Sprintf("%s%s%s/\n", prefix, connector, child.Name))
			(*currentLine)++
			renderFileTreeWithPrefix(child, depth+1, childPrefix, sb, regions, fileMap, fileStatusMap,
				status, regionIndex, currentSelection, focusedPane, lineNumberMap, currentLine)
		}
	}
}

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
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	textView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// 右ペイン（差分表示）のテキストビューを作成
	diffView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	diffView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

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
	var gPressed bool = false
	var lastGTime time.Time

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
	var lineNumberMap = make(map[int]int)

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
			case 'g':
				now := time.Now()
				if gPressed && now.Sub(lastGTime) < 500*time.Millisecond {
					// gg → 最上部
					cursorY = 0
					if isSelecting {
						selectEnd = cursorY
					}
					updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
					gPressed = false
				} else {
					// 1回目のg
					gPressed = true
					lastGTime = now
				}
				return nil
			case 'G': // 大文字G → 最下部へ
				coloredDiff := ColorizeDiff(currentDiffText)
				cursorY = len(util.SplitLines(coloredDiff)) - 1
				if isSelecting {
					selectEnd = cursorY
				}
				updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
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
			case 'u':
				// パッチファイルが存在するか確認
				if _, err := os.Stat(patchPath); os.IsNotExist(err) {
					updateListStatus("No patch to undo", "yellow")
					return nil
				}

				cmd := exec.Command("git", "apply", "-R", "--cached", patchPath)
				cmd.Dir = repoRoot
				_, err := cmd.CombinedOutput()
				if err != nil {
					updateListStatus("Undo failed!", "firebrick")
				} else {
					updateListStatus("Undo successful!", "gold")

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
					diffLines = util.SplitLines(coloredDiff)

					// 再描画
					updateDiffView(diffView, diffLines, cursorY)

					// ファイルリストを更新
					refreshFileList()
					updateFileListView()
				}
			case 'a':
				// commandA関数を呼び出す
				params := commands.CommandAParams{
					SelectStart:      selectStart,
					SelectEnd:        selectEnd,
					CurrentFile:      currentFile,
					CurrentStatus:    currentStatus,
					CurrentDiffText:  currentDiffText,
					RepoRoot:         repoRoot,
					UpdateListStatus: updateListStatus,
				}

				result, err := commands.CommandA(params)
				if err != nil {
					return nil
				}
				if result == nil {
					return nil
				}

				// 結果を反映
				currentDiffText = result.NewDiffText
				diffLines = result.DiffLines

				// 選択を解除してカーソルリセット
				isSelecting = false
				selectStart = -1
				selectEnd = -1
				cursorY = 0

				// 再描画
				updateDiffView(diffView, diffLines, cursorY)

				// ファイルリストを内部的に更新
				refreshFileList()

				// 差分が残っている場合
				if !result.ShouldUpdate {
					// 現在のファイルの位置を維持するため、savedTargetFileを設定
					savedTargetFile = currentFile
					// ファイルリストを再描画
					updateFileListView()
				} else {
					// 差分がなくなった場合は、完全に更新
					if onUpdate != nil {
						onUpdate()
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
						diffLines = util.SplitLines(coloredDiff)

						// カーソルと選択をリセット
						isSelecting = false
						selectStart = -1
						selectEnd = -1
						cursorY = 0

						// 再描画
						updateDiffView(diffView, diffLines, cursorY)

						// ステータスを更新
						if wasStaged {
							updateListStatus("File unstaged successfully!", "gold")
						} else {
							updateListStatus("File staged successfully!", "gold")
						}

						// refreshFileListを呼んで最新の状態を取得
						refreshFileList()

						// カーソル位置を保存
						// 常にunstagedセクションの先頭を選択するように設定
						preferUnstagedSection = true
						savedTargetFile = ""

						// ファイルリストを更新
						if onUpdate != nil {
							onUpdate()
						}
					} else {
						// エラーの場合
						if currentStatus == "staged" {
							updateListStatus("Failed to unstage file", "firebrick")
						} else {
							updateListStatus("Failed to stage file", "firebrick")
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

	// ボーダーを作成
	verticalBorder := CreateVerticalBorder()
	horizontalTopBorder := CreateHorizontalTopBorder()
	horizontalBottomBorder := CreateHorizontalBottomBorder()

	// 左右のペインをフレックスに追加（左:縦線:右 = 1:0:2）
	contentFlex.
		AddItem(verticalBorder, 1, 0, false).
		AddItem(textView, 0, 1, true).
		AddItem(verticalBorder, 1, 0, false).
		AddItem(diffView, 0, 2, false).
		AddItem(verticalBorder, 1, 0, false)

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
		lineNumberMap = make(map[int]int)

		var coloredContent strings.Builder
		regionIndex := 0
		currentLine := 0

		// Staged ファイル
		if len(*stagedFilesPtr) > 0 {
			coloredContent.WriteString("[green]Changes to be committed:[white]\n")
			currentLine++
			tree := buildFileTree(*stagedFilesPtr)
			renderFileTree(tree, 1, &coloredContent, &regions, fileMap, fileStatusMap,
				"staged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
			coloredContent.WriteString("\n")
			currentLine++
		}

		// 変更されたファイル（unstaged）
		if len(*modifiedFilesPtr) > 0 {
			coloredContent.WriteString("[yellow]Changes not staged for commit:[white]\n")
			currentLine++
			tree := buildFileTree(*modifiedFilesPtr)
			renderFileTree(tree, 1, &coloredContent, &regions, fileMap, fileStatusMap,
				"unstaged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
			coloredContent.WriteString("\n")
			currentLine++
		}

		// 未追跡ファイル
		if len(*untrackedFilesPtr) > 0 {
			coloredContent.WriteString("[red]Untracked files:[white]\n")
			currentLine++
			tree := buildFileTree(*untrackedFilesPtr)
			renderFileTree(tree, 1, &coloredContent, &regions, fileMap, fileStatusMap,
				"untracked", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
		}

		return coloredContent.String()
	}

	// 初期表示を更新
	updateFileListView = func() {
		textView.Clear()
		textView.SetText(buildFileListContent(leftPaneFocused))

		// 最初のファイルが選択されている場合は一番上にスクロール
		if currentSelection == 0 {
			textView.ScrollTo(0, 0)
			return
		}

		// スクロール位置を調整（選択行が見える範囲に）
		if actualLine, exists := lineNumberMap[currentSelection]; exists {
			_, _, _, height := textView.GetInnerRect()
			currentRow, _ := textView.GetScrollOffset()

			// 選択行が画面より下にある場合
			if actualLine >= currentRow+height-1 {
				textView.ScrollTo(actualLine-height+2, 0)
			}
			// 選択行が画面より上にある場合
			if actualLine < currentRow {
				textView.ScrollTo(actualLine, 0)
			}
		}
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

			// 右ペインに差分を表示（カーソルなし）
			diffLines = ShowDiffInPane(diffView, file, status, repoRoot, cursorY, &currentDiffText)
			updateDiffViewWithoutCursor(diffView, diffLines)
		}
	}

	updateFileListView()

	// 初期表示時に最初のファイルの差分を表示
	updateSelectedFileDiff()

	// キー入力の処理
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if currentSelection > 0 {
				currentSelection--
				updateFileListView()
				updateSelectedFileDiff()
			}
			return nil
		case tcell.KeyDown:
			if currentSelection < len(regions)-1 {
				currentSelection++
				updateFileListView()
				updateSelectedFileDiff()
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
					updateSelectedFileDiff()
				}
				return nil
			case 'j':
				if currentSelection < len(regions)-1 {
					currentSelection++
					updateFileListView()
					updateSelectedFileDiff()
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

					// カーソル位置を保存
					// 常にunstagedセクションの先頭を選択するように設定
					preferUnstagedSection = true
					savedTargetFile = ""

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

	// 初期メッセージを設定
	listStatusView.SetText(listKeyBindingMessage)

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
						// 現在の選択ファイルを保存
						var currentlySelectedFile string
						if currentSelection >= 0 && currentSelection < len(regions) {
							currentlySelectedFile = fileMap[regions[currentSelection]]
						}

						// ファイルリストを更新
						refreshFileList()

						// 選択位置を復元
						newSelection := -1
						for i, regionID := range regions {
							if fileMap[regionID] == currentlySelectedFile {
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
								diffLines = util.SplitLines(coloredDiff)

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

								updateDiffViewWithSelection(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting)
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

	// 表示用の行を返す（カーソル表示のため）- file_view.goと同じutil.splitLinesを使用
	lines := util.SplitLines(coloredDiff)

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
