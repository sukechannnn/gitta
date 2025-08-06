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
	"github.com/sukechannnn/gitta/util"
)

// FileEntry represents a file entry in the file list with ID, path and status
type FileEntry struct {
	ID          string
	Path        string
	StageStatus string // "staged", "unstaged", "untracked"
}

// formatFileWithStatus adds status decoration to filename
func formatFileWithStatus(filename string, status string) string {
	switch status {
	case "added", "untracked":
		return filename + " " + "(+)"
	case "deleted":
		return filename + " " + "(-)"
	case "modified":
		return filename + " " + "(•)"
	default:
		return filename
	}
}

// TreeNode represents a node in the file tree structure
type TreeNode struct {
	Name     string
	IsFile   bool
	Children map[string]*TreeNode
	FullPath string // ファイルの場合のみ使用
}

// buildFileTree converts a list of file paths into a tree structure
func buildFileTree(files []git.FileInfo) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsFile:   false,
		Children: make(map[string]*TreeNode),
	}

	for _, fileInfo := range files {
		file := strings.TrimSpace(fileInfo.Path)
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
func renderFileTree(
	node *TreeNode,
	depth int,
	sb *strings.Builder,
	fileList *[]FileEntry,
	stageStatus string,
	regionIndex *int,
	currentSelection int,
	focusedPane bool,
	lineNumberMap map[int]int,
	currentLine *int,
	fileInfos []git.FileInfo,
) {
	renderFileTreeWithPrefix(
		node,
		depth,
		"",
		sb,
		fileList,
		stageStatus,
		regionIndex,
		currentSelection,
		focusedPane,
		lineNumberMap,
		currentLine,
		fileInfos,
	)
}

// renderFileTreeWithPrefix renders the tree with proper line prefixes
func renderFileTreeWithPrefix(
	node *TreeNode,
	depth int,
	prefix string,
	sb *strings.Builder,
	fileList *[]FileEntry,
	stageStatus string,
	regionIndex *int,
	currentSelection int,
	focusedPane bool,
	lineNumberMap map[int]int,
	currentLine *int,
	fileInfos []git.FileInfo,
) {
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
			*fileList = append(*fileList, FileEntry{
				ID:          regionID,
				Path:        child.FullPath,
				StageStatus: stageStatus,
			})

			// ファイル名に装飾を追加
			displayName := child.Name
			// ファイルのステータスを検索して装飾を追加
			for _, fileInfo := range fileInfos {
				if fileInfo.Path == child.FullPath {
					displayName = formatFileWithStatus(child.Name, fileInfo.ChangeStatus)
					break
				}
			}

			if focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[white:blue]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, displayName))
			} else if !focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[black:white]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, displayName))
			} else {
				sb.WriteString(fmt.Sprintf(`%s[white:%s]["file-%d"]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, *regionIndex, connector, displayName))
			}
			lineNumberMap[*regionIndex] = *currentLine
			(*regionIndex)++
			(*currentLine)++
		} else {
			// ディレクトリの場合
			sb.WriteString(fmt.Sprintf("%s%s%s/\n", prefix, connector, child.Name))
			(*currentLine)++
			renderFileTreeWithPrefix(child, depth+1, childPrefix, sb, fileList,
				stageStatus, regionIndex, currentSelection, focusedPane, lineNumberMap, currentLine, fileInfos)
		}
	}
}

// BuildFileListContent builds the colored file list content
func BuildFileListContent(
	stagedFiles, modifiedFiles, untrackedFiles []git.FileInfo,
	currentSelection int,
	focusedPane bool,
	fileList *[]FileEntry,
	lineNumberMap map[int]int,
) string {
	// fileList を再構築
	// スライスの中身をクリア（参照は保持）
	*fileList = (*fileList)[:0]
	for k := range lineNumberMap {
		delete(lineNumberMap, k)
	}

	var coloredContent strings.Builder
	regionIndex := 0
	currentLine := 0

	// Staged ファイル
	if len(stagedFiles) > 0 {
		coloredContent.WriteString("[green]Changes to be committed:[white]\n")
		currentLine++
		tree := buildFileTree(stagedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"staged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, stagedFiles)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 変更されたファイル（unstaged）
	if len(modifiedFiles) > 0 {
		coloredContent.WriteString("[yellow]Changes not staged for commit:[white]\n")
		currentLine++
		tree := buildFileTree(modifiedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"unstaged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, modifiedFiles)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 未追跡ファイル
	if len(untrackedFiles) > 0 {
		coloredContent.WriteString("[red]Untracked files:[white]\n")
		currentLine++
		tree := buildFileTree(untrackedFiles)
		renderFileTree(tree, 1, &coloredContent, fileList,
			"untracked", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine, untrackedFiles)
	}

	return coloredContent.String()
}

// FileListKeyContext contains all the context needed for file list key bindings
type FileListKeyContext struct {
	// UI Components
	fileListView    *tview.TextView
	diffView        *tview.TextView
	beforeView      *tview.TextView
	afterView       *tview.TextView
	splitViewFlex   *tview.Flex
	unifiedViewFlex *tview.Flex
	contentFlex     *tview.Flex
	app             *tview.Application

	// State
	currentSelection *int
	cursorY          *int
	isSelecting      *bool
	selectStart      *int
	selectEnd        *int
	isSplitView      *bool
	leftPaneFocused  *bool
	currentFile      *string
	currentStatus    *string
	currentDiffText  *string

	// Collections
	fileList *[]FileEntry

	// Paths
	repoRoot string

	// Callbacks
	updateFileListView     func()
	updateSelectedFileDiff func()
	refreshFileList        func()
	updateCurrentDiffText  func(string, string, string, *string)
}

// SetupFileListKeyBindings sets up key bindings for file list view
func SetupFileListKeyBindings(ctx *FileListKeyContext) {
	ctx.fileListView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if *ctx.currentSelection > 0 {
				(*ctx.currentSelection)--
				ctx.updateFileListView()
				ctx.updateSelectedFileDiff()
			}
			return nil
		case tcell.KeyDown:
			if *ctx.currentSelection < len(*ctx.fileList)-1 {
				(*ctx.currentSelection)++
				ctx.updateFileListView()
				ctx.updateSelectedFileDiff()
			}
			return nil
		case tcell.KeyEnter:
			if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
				fileEntry := (*ctx.fileList)[*ctx.currentSelection]
				file := fileEntry.Path
				status := fileEntry.StageStatus

				// 現在のファイル情報を更新
				*ctx.currentFile = file
				*ctx.currentStatus = status

				// カーソルと選択をリセット
				*ctx.cursorY = 0
				*ctx.isSelecting = false
				*ctx.selectStart = -1
				*ctx.selectEnd = -1

				ctx.updateCurrentDiffText(file, status, ctx.repoRoot, ctx.currentDiffText)

				// Split Viewの場合はカーソル付きで更新
				if *ctx.isSplitView {
					updateSplitViewWithCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText, *ctx.cursorY)
				} else {
					updateDiffViewWithCursor(ctx.diffView, *ctx.currentDiffText, *ctx.cursorY)
				}

				// フォーカスを右ペインに移動
				*ctx.leftPaneFocused = false
				ctx.updateFileListView()
				if *ctx.isSplitView {
					ctx.app.SetFocus(ctx.splitViewFlex)
				} else {
					ctx.app.SetFocus(ctx.diffView)
				}
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				if *ctx.currentSelection > 0 {
					(*ctx.currentSelection)--
					ctx.updateFileListView()
					ctx.updateSelectedFileDiff()
				}
				return nil
			case 'j':
				if *ctx.currentSelection < len(*ctx.fileList)-1 {
					(*ctx.currentSelection)++
					ctx.updateFileListView()
					ctx.updateSelectedFileDiff()
				}
				return nil
			case 'h':
				// 左にスクロール
				currentRow, currentCol := ctx.fileListView.GetScrollOffset()
				if currentCol > 0 {
					ctx.fileListView.ScrollTo(currentRow, currentCol-1)
				}
				return nil
			case 'l':
				// 右にスクロール
				currentRow, currentCol := ctx.fileListView.GetScrollOffset()
				ctx.fileListView.ScrollTo(currentRow, currentCol+1)
				return nil
			case 's':
				// Split Viewのトグル
				*ctx.isSplitView = !*ctx.isSplitView

				if *ctx.isSplitView {
					// Split Viewを表示
					updateSplitViewWithoutCursor(ctx.beforeView, ctx.afterView, *ctx.currentDiffText)
					ctx.contentFlex.RemoveItem(ctx.unifiedViewFlex)
					ctx.contentFlex.AddItem(ctx.splitViewFlex, 0, 4, false)
				} else {
					// 通常の差分表示に戻す
					ctx.contentFlex.RemoveItem(ctx.splitViewFlex)
					ctx.contentFlex.AddItem(ctx.unifiedViewFlex, 0, 4, false)
					updateDiffViewWithoutCursor(ctx.diffView, *ctx.currentDiffText)
				}
				return nil
			case 'A': // 'A' で現在のファイルを git add/reset
				if *ctx.currentSelection >= 0 && *ctx.currentSelection < len(*ctx.fileList) {
					fileEntry := (*ctx.fileList)[*ctx.currentSelection]
					file := fileEntry.Path
					status := fileEntry.StageStatus

					var cmd *exec.Cmd
					if status == "staged" {
						// stagedファイルをunstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", file)
						cmd.Dir = ctx.repoRoot
					} else {
						// unstaged/untrackedファイルをstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", file)
						cmd.Dir = ctx.repoRoot
					}

					err := cmd.Run()
					if err != nil {
						// エラーハンドリング（ここでは簡単にスキップ）
						return nil
					}

					// 現在のカーソル位置の次のファイルを探す
					var nextFile string
					var nextStatus string
					if *ctx.currentSelection < len(*ctx.fileList)-1 {
						nextFileEntry := (*ctx.fileList)[*ctx.currentSelection+1]
						nextFile = nextFileEntry.Path
						nextStatus = nextFileEntry.StageStatus
					}

					// ファイルリストを更新
					ctx.refreshFileList()

					// カーソル位置を復元（UpdateFileListViewの前に実行）
					foundNext := false
					if nextFile != "" {
						// UpdateFileListViewを呼ぶ前に一時的に選択を保存
						tempSelection := -1

						// ファイルリストを再構築（UpdateFileListViewを呼ぶ）
						ctx.updateFileListView()

						// 次のファイルを探す
						for i, fileEntry := range *ctx.fileList {
							if fileEntry.Path == nextFile && fileEntry.StageStatus == nextStatus {
								tempSelection = i
								foundNext = true
								break
							}
						}

						if foundNext {
							*ctx.currentSelection = tempSelection
						}
					} else {
						// nextFileがない場合は通常通り更新
						ctx.updateFileListView()
					}

					if !foundNext {
						// 次のファイルが見つからない場合
						if *ctx.currentSelection >= len(*ctx.fileList) {
							// 最後のファイルだった場合
							*ctx.currentSelection = len(*ctx.fileList) - 1
						}
					}

					// 画面を再度更新して選択位置を反映
					ctx.updateFileListView()
					ctx.updateSelectedFileDiff()
				}
				return nil
			case 'q': // 'q' でアプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				ctx.app.Stop()
			}
		}
		return event
	})
}
