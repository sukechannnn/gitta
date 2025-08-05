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
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		// 次の階層のためのプレフィックス
		childPrefix := prefix + "│ "
		if isLast {
			childPrefix = prefix + "  "
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
	var isSplitView bool = false // Split Viewモードのフラグ
	var oldLineMap map[int]int   // 表示行 -> 元ファイルの行番号
	var newLineMap map[int]int   // 表示行 -> 新ファイルの行番号

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
			// 左ペインに戻る
			// 選択モードをリセット（カーソル位置は保持）
			isSelecting = false
			selectStart = -1
			selectEnd = -1
			// 表示を更新（カーソルなし）
			if isSplitView {
				// Split Viewもカーソルなしで更新
				updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
			} else {
				// 通常の差分表示もカーソルなし
				updateDiffViewWithoutCursorAndMapping(diffView, diffLines, oldLineMap, newLineMap)
			}
			// 左ペインにフォーカスを戻す
			leftPaneFocused = true
			updateFileListView()
			app.SetFocus(fileListView)
			return nil
		case tcell.KeyEnter:
			// 左ペインに戻る
			// 選択モードをリセット（カーソル位置は保持）
			isSelecting = false
			selectStart = -1
			selectEnd = -1
			// 表示を更新
			if isSplitView {
				// Split Viewはカーソル付きで更新
				updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
			} else {
				// 通常の差分表示はカーソルなし
				updateDiffViewWithoutCursorAndMapping(diffView, diffLines, oldLineMap, newLineMap)
			}
			// 左ペインにフォーカスを戻す
			leftPaneFocused = true
			updateFileListView()
			app.SetFocus(fileListView)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'w':
				// 左ペインに戻る
				// 選択モードをリセット（カーソル位置は保持）
				isSelecting = false
				selectStart = -1
				selectEnd = -1
				// 表示を更新
				if isSplitView {
					// Split Viewはカーソル付きで更新
					updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
				} else {
					// 通常の差分表示はカーソルなし
					updateDiffViewWithoutCursorAndMapping(diffView, diffLines, oldLineMap, newLineMap)
				}
				// 左ペインにフォーカスを戻す
				leftPaneFocused = true
				updateFileListView()
				app.SetFocus(fileListView)
				return nil
			case 's':
				// Split Viewのトグル
				isSplitView = !isSplitView

				if isSplitView {
					// Split Viewを表示（現在のカーソル位置を維持）
					updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
					contentFlex.RemoveItem(diffView)
					contentFlex.AddItem(splitViewFlex, 0, 4, false)
					// フォーカスがdiffViewにある場合、splitViewFlexに移動
					if !leftPaneFocused {
						app.SetFocus(splitViewFlex)
					}
				} else {
					// 通常の差分表示に戻す
					contentFlex.RemoveItem(splitViewFlex)
					contentFlex.AddItem(diffView, 0, 4, false)
					updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
					// フォーカスがsplitViewFlexにある場合、diffViewに移動
					if !leftPaneFocused {
						app.SetFocus(diffView)
					}
				}
				return nil
			case 'g':
				now := time.Now()
				if gPressed && now.Sub(lastGTime) < 500*time.Millisecond {
					// gg → 最上部
					cursorY = 0
					if isSelecting {
						selectEnd = cursorY
					}
					if isSplitView {
						updateSplitViewWithSelection(beforeView, afterView, currentDiffText, cursorY, selectStart, selectEnd, isSelecting)
					} else {
						updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
					}
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
				if isSplitView {
					updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
				} else {
					updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
				}
				return nil
			case 'j':
				// 下移動
				maxLines := len(diffLines) - 1
				if isSplitView {
					// Split Viewの場合は有効な行数を取得
					splitViewLines := getSplitViewLineCount(currentDiffText)
					if splitViewLines > 0 {
						maxLines = splitViewLines - 1
					} else {
						maxLines = 0
					}
				}

				if cursorY < maxLines {
					cursorY++
					if isSelecting {
						selectEnd = cursorY
					}
					if isSplitView {
						updateSplitViewWithSelection(beforeView, afterView, currentDiffText, cursorY, selectStart, selectEnd, isSelecting)
					} else {
						updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
					}
				}
				return nil
			case 'k':
				// 上移動
				if cursorY > 0 {
					cursorY--
					if isSelecting {
						selectEnd = cursorY
					}
					if isSplitView {
						updateSplitViewWithSelection(beforeView, afterView, currentDiffText, cursorY, selectStart, selectEnd, isSelecting)
					} else {
						updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
					}
				}
				return nil
			case 'V':
				// Shift + V で選択モード開始
				if !isSelecting {
					isSelecting = true
					selectStart = cursorY
					selectEnd = cursorY
					if isSplitView {
						updateSplitViewWithSelection(beforeView, afterView, currentDiffText, cursorY, selectStart, selectEnd, isSelecting)
					} else {
						updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
					}
				}
				return nil
			case 'u':
				// パッチファイルが存在するか確認
				if _, err := os.Stat(patchPath); os.IsNotExist(err) {
					updateListStatus("No patch to undo", "yellow")
					return nil
				}

				cmd := exec.Command("git", "-c", "core.quotepath=false", "apply", "-R", "--cached", patchPath)
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
					updateDiffViewWithMapping(diffView, diffLines, cursorY, oldLineMap, newLineMap)

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
				if isSplitView {
					updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
				} else {
					updateDiffViewWithMapping(diffView, diffLines, cursorY, oldLineMap, newLineMap)
				}

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
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", currentFile)
					} else {
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", currentFile)
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
						updateDiffViewWithMapping(diffView, diffLines, cursorY, oldLineMap, newLineMap)

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

	// Split View用のキー入力処理を設定
	splitViewFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			// 左ペインに戻る
			// 選択モードをリセット（カーソル位置は保持）
			isSelecting = false
			selectStart = -1
			selectEnd = -1
			// 表示を更新（カーソルなし）
			updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
			// 左ペインにフォーカスを戻す
			leftPaneFocused = true
			updateFileListView()
			app.SetFocus(fileListView)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'w':
				// 左ペインに戻る
				// 選択モードをリセット（カーソル位置は保持）
				isSelecting = false
				selectStart = -1
				selectEnd = -1
				// 表示を更新（カーソルなし）
				updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
				// 左ペインにフォーカスを戻す
				leftPaneFocused = true
				updateFileListView()
				app.SetFocus(fileListView)
				return nil
			default:
				// その他のキーはdiffViewの処理を呼び出す
				return diffView.GetInputCapture()(event)
			}
		default:
			// その他のキーはdiffViewの処理を呼び出す
			return diffView.GetInputCapture()(event)
		}
	})

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
			// 行番号マッピングを作成
			oldLineMap, newLineMap = createLineNumberMapping(currentDiffText)
			if isSplitView {
				updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
			} else {
				updateDiffViewWithoutCursorAndMapping(diffView, diffLines, oldLineMap, newLineMap)
			}
		}
	}

	updateFileListView()

	// 初期表示時に最初のファイルの差分を表示
	updateSelectedFileDiff()

	// キー入力の処理
	fileListView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

				// Split Viewの場合はカーソル付きで更新
				if isSplitView {
					updateSplitViewWithCursor(beforeView, afterView, currentDiffText, cursorY)
				}

				// フォーカスを右ペインに移動
				leftPaneFocused = false
				updateFileListView()
				if isSplitView {
					app.SetFocus(splitViewFlex)
				} else {
					app.SetFocus(diffView)
				}
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
			case 'h':
				// 左にスクロール
				currentRow, currentCol := fileListView.GetScrollOffset()
				if currentCol > 0 {
					fileListView.ScrollTo(currentRow, currentCol-1)
				}
				return nil
			case 'l':
				// 右にスクロール
				currentRow, currentCol := fileListView.GetScrollOffset()
				fileListView.ScrollTo(currentRow, currentCol+1)
				return nil
			case 's':
				// Split Viewのトグル
				isSplitView = !isSplitView

				if isSplitView {
					// Split Viewを表示
					updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
					contentFlex.RemoveItem(diffView)
					contentFlex.AddItem(splitViewFlex, 0, 4, false)
				} else {
					// 通常の差分表示に戻す
					contentFlex.RemoveItem(splitViewFlex)
					contentFlex.AddItem(diffView, 0, 4, false)
					updateDiffViewWithoutCursorAndMapping(diffView, diffLines, oldLineMap, newLineMap)
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
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", file)
						cmd.Dir = repoRoot
					} else {
						// unstaged/untrackedファイルをstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", file)
						cmd.Dir = repoRoot
					}

					err := cmd.Run()
					if err != nil {
						// エラーハンドリング（ここでは簡単にスキップ）
						return nil
					}

					// 現在のカーソル位置の次のファイルを探す
					var nextFile string
					var nextStatus string
					if currentSelection < len(regions)-1 {
						nextRegionID := regions[currentSelection+1]
						nextFile = fileMap[nextRegionID]
						nextStatus = fileStatusMap[nextRegionID]
					}

					// ファイルリストを更新
					refreshFileList()

					// カーソル位置を復元
					if nextFile != "" {
						// 次のファイルを探す
						for i, regionID := range regions {
							if fileMap[regionID] == nextFile && fileStatusMap[regionID] == nextStatus {
								currentSelection = i
								break
							}
						}
					} else if currentSelection >= len(regions) {
						// 最後のファイルだった場合
						currentSelection = len(regions) - 1
					}

					// 画面を更新
					updateFileListView()
					updateSelectedFileDiff()
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

								// Split Viewモードの場合はSplit View更新、そうでなければ通常の更新
								if isSplitView {
									updateSplitViewWithoutCursor(beforeView, afterView, currentDiffText)
								} else {
									updateDiffViewWithSelectionAndMapping(diffView, diffLines, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
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
	oldMap, newMap := createLineNumberMapping(diffText)
	updateDiffViewWithMapping(diffView, lines, cursorY, oldMap, newMap)

	return lines
}

// 差分ビューを更新する関数（行番号マッピング付き）
func updateDiffViewWithMapping(diffView *tview.TextView, lines []string, cursorY int, oldLineMap, newLineMap map[int]int) {
	updateDiffViewWithSelectionAndMapping(diffView, lines, cursorY, -1, -1, false, oldLineMap, newLineMap)
}

// diffテキストから行番号マッピングを作成
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

// 選択範囲と行番号マッピング付きで差分ビューを更新する関数
func updateDiffViewWithSelectionAndMapping(diffView *tview.TextView, lines []string, cursorY int, selectStart int, selectEnd int, isSelecting bool, oldLineMap, newLineMap map[int]int) {
	diffView.Clear()

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

// カーソルなしで差分ビューを更新する関数（行番号マッピング付き）
func updateDiffViewWithoutCursorAndMapping(diffView *tview.TextView, lines []string, oldLineMap, newLineMap map[int]int) {
	diffView.Clear()

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

		diffView.Write([]byte("[dimgray]" + lineNum + "[-]" + line + "\n"))
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

// カーソルなしでSplit Viewを更新する関数
func updateSplitViewWithoutCursor(beforeView, afterView *tview.TextView, diffText string) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, -1, -1, -1, false, oldLineMap, newLineMap)
}

// Split View用の有効な行数を取得
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

// カーソル付きでSplit Viewを更新する関数
func updateSplitViewWithCursor(beforeView, afterView *tview.TextView, diffText string, cursorY int) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, -1, -1, false, oldLineMap, newLineMap)
}

// 選択範囲付きでSplit Viewを更新する関数
func updateSplitViewWithSelection(beforeView, afterView *tview.TextView, diffText string, cursorY int, selectStart int, selectEnd int, isSelecting bool) {
	// 行番号マッピングを作成
	oldLineMap, newLineMap := createLineNumberMapping(diffText)
	updateSplitViewWithSelectionAndMapping(beforeView, afterView, diffText, cursorY, selectStart, selectEnd, isSelecting, oldLineMap, newLineMap)
}

// 選択範囲と行番号マッピング付きでSplit Viewを更新する関数
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
