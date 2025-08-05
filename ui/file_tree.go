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
	"github.com/sukechannnn/gitta/util"
)

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

// BuildFileListContent builds the colored file list content
func BuildFileListContent(
	stagedFiles, modifiedFiles, untrackedFiles []string,
	currentSelection int,
	focusedPane bool,
	regions *[]string,
	fileMap map[string]string,
	fileStatusMap map[string]string,
	lineNumberMap map[int]int,
) string {
	// regions と fileMap を再構築
	// スライスの中身をクリア（参照は保持）
	*regions = (*regions)[:0]
	for k := range fileMap {
		delete(fileMap, k)
	}
	for k := range fileStatusMap {
		delete(fileStatusMap, k)
	}
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
		renderFileTree(tree, 1, &coloredContent, regions, fileMap, fileStatusMap,
			"staged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 変更されたファイル（unstaged）
	if len(modifiedFiles) > 0 {
		coloredContent.WriteString("[yellow]Changes not staged for commit:[white]\n")
		currentLine++
		tree := buildFileTree(modifiedFiles)
		renderFileTree(tree, 1, &coloredContent, regions, fileMap, fileStatusMap,
			"unstaged", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
		coloredContent.WriteString("\n")
		currentLine++
	}

	// 未追跡ファイル
	if len(untrackedFiles) > 0 {
		coloredContent.WriteString("[red]Untracked files:[white]\n")
		currentLine++
		tree := buildFileTree(untrackedFiles)
		renderFileTree(tree, 1, &coloredContent, regions, fileMap, fileStatusMap,
			"untracked", &regionIndex, currentSelection, focusedPane, lineNumberMap, &currentLine)
	}

	return coloredContent.String()
}

// FileListKeyContext contains all the context needed for file list key bindings
type FileListKeyContext struct {
	// UI Components
	FileListView  *tview.TextView
	DiffView      *tview.TextView
	BeforeView    *tview.TextView
	AfterView     *tview.TextView
	SplitViewFlex *tview.Flex
	ContentFlex   *tview.Flex
	App           *tview.Application

	// State
	CurrentSelection *int
	CursorY          *int
	IsSelecting      *bool
	SelectStart      *int
	SelectEnd        *int
	IsSplitView      *bool
	LeftPaneFocused  *bool
	CurrentFile      *string
	CurrentStatus    *string
	CurrentDiffText  *string

	// Collections
	Regions       *[]string
	FileMap       map[string]string
	FileStatusMap map[string]string

	// Paths
	RepoRoot string

	// Callbacks
	UpdateFileListView     func()
	UpdateSelectedFileDiff func()
	RefreshFileList        func()
	UpdateCurrentDiffText  func(string, string, string, *string)
}

// SetupFileListKeyBindings sets up key bindings for file list view
func SetupFileListKeyBindings(ctx *FileListKeyContext) {
	ctx.FileListView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if *ctx.CurrentSelection > 0 {
				(*ctx.CurrentSelection)--
				ctx.UpdateFileListView()
				ctx.UpdateSelectedFileDiff()
			}
			return nil
		case tcell.KeyDown:
			if *ctx.CurrentSelection < len(*ctx.Regions)-1 {
				(*ctx.CurrentSelection)++
				ctx.UpdateFileListView()
				ctx.UpdateSelectedFileDiff()
			}
			return nil
		case tcell.KeyEnter:
			if *ctx.CurrentSelection >= 0 && *ctx.CurrentSelection < len(*ctx.Regions) {
				regionID := (*ctx.Regions)[*ctx.CurrentSelection]
				file := ctx.FileMap[regionID]
				status := ctx.FileStatusMap[regionID]

				// 現在のファイル情報を更新
				*ctx.CurrentFile = file
				*ctx.CurrentStatus = status

				// カーソルと選択をリセット
				*ctx.CursorY = 0
				*ctx.IsSelecting = false
				*ctx.SelectStart = -1
				*ctx.SelectEnd = -1

				ctx.UpdateCurrentDiffText(file, status, ctx.RepoRoot, ctx.CurrentDiffText)

				// Split Viewの場合はカーソル付きで更新
				if *ctx.IsSplitView {
					updateSplitViewWithCursor(ctx.BeforeView, ctx.AfterView, *ctx.CurrentDiffText, *ctx.CursorY)
				} else {
					updateDiffViewWithCursor(ctx.DiffView, *ctx.CurrentDiffText, *ctx.CursorY)
				}

				// フォーカスを右ペインに移動
				*ctx.LeftPaneFocused = false
				ctx.UpdateFileListView()
				if *ctx.IsSplitView {
					ctx.App.SetFocus(ctx.SplitViewFlex)
				} else {
					ctx.App.SetFocus(ctx.DiffView)
				}
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				if *ctx.CurrentSelection > 0 {
					(*ctx.CurrentSelection)--
					ctx.UpdateFileListView()
					ctx.UpdateSelectedFileDiff()
				}
				return nil
			case 'j':
				if *ctx.CurrentSelection < len(*ctx.Regions)-1 {
					(*ctx.CurrentSelection)++
					ctx.UpdateFileListView()
					ctx.UpdateSelectedFileDiff()
				}
				return nil
			case 'h':
				// 左にスクロール
				currentRow, currentCol := ctx.FileListView.GetScrollOffset()
				if currentCol > 0 {
					ctx.FileListView.ScrollTo(currentRow, currentCol-1)
				}
				return nil
			case 'l':
				// 右にスクロール
				currentRow, currentCol := ctx.FileListView.GetScrollOffset()
				ctx.FileListView.ScrollTo(currentRow, currentCol+1)
				return nil
			case 's':
				// Split Viewのトグル
				*ctx.IsSplitView = !*ctx.IsSplitView

				if *ctx.IsSplitView {
					// Split Viewを表示
					updateSplitViewWithoutCursor(ctx.BeforeView, ctx.AfterView, *ctx.CurrentDiffText)
					ctx.ContentFlex.RemoveItem(ctx.DiffView)
					ctx.ContentFlex.AddItem(ctx.SplitViewFlex, 0, 4, false)
				} else {
					// 通常の差分表示に戻す
					ctx.ContentFlex.RemoveItem(ctx.SplitViewFlex)
					ctx.ContentFlex.AddItem(ctx.DiffView, 0, 4, false)
					updateDiffViewWithoutCursor(ctx.DiffView, *ctx.CurrentDiffText)
				}
				return nil
			case 'A': // 'A' で現在のファイルを git add/reset
				if *ctx.CurrentSelection >= 0 && *ctx.CurrentSelection < len(*ctx.Regions) {
					regionID := (*ctx.Regions)[*ctx.CurrentSelection]
					file := ctx.FileMap[regionID]
					status := ctx.FileStatusMap[regionID]

					var cmd *exec.Cmd
					if status == "staged" {
						// stagedファイルをunstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "reset", "HEAD", file)
						cmd.Dir = ctx.RepoRoot
					} else {
						// unstaged/untrackedファイルをstageする
						cmd = exec.Command("git", "-c", "core.quotepath=false", "add", file)
						cmd.Dir = ctx.RepoRoot
					}

					err := cmd.Run()
					if err != nil {
						// エラーハンドリング（ここでは簡単にスキップ）
						return nil
					}

					// 現在のカーソル位置の次のファイルを探す
					var nextFile string
					var nextStatus string
					if *ctx.CurrentSelection < len(*ctx.Regions)-1 {
						nextRegionID := (*ctx.Regions)[*ctx.CurrentSelection+1]
						nextFile = ctx.FileMap[nextRegionID]
						nextStatus = ctx.FileStatusMap[nextRegionID]
					}

					// ファイルリストを更新
					ctx.RefreshFileList()

					// カーソル位置を復元（UpdateFileListViewの前に実行）
					foundNext := false
					if nextFile != "" {
						// UpdateFileListViewを呼ぶ前に一時的に選択を保存
						tempSelection := -1

						// ファイルリストを再構築（UpdateFileListViewを呼ぶ）
						ctx.UpdateFileListView()

						// 次のファイルを探す
						for i, regionID := range *ctx.Regions {
							if ctx.FileMap[regionID] == nextFile && ctx.FileStatusMap[regionID] == nextStatus {
								tempSelection = i
								foundNext = true
								break
							}
						}

						if foundNext {
							*ctx.CurrentSelection = tempSelection
						}
					} else {
						// nextFileがない場合は通常通り更新
						ctx.UpdateFileListView()
					}

					if !foundNext {
						// 次のファイルが見つからない場合
						if *ctx.CurrentSelection >= len(*ctx.Regions) {
							// 最後のファイルだった場合
							*ctx.CurrentSelection = len(*ctx.Regions) - 1
						}
					}

					// 画面を再度更新して選択位置を反映
					ctx.UpdateFileListView()
					ctx.UpdateSelectedFileDiff()
				}
				return nil
			case 'q': // 'q' でアプリ終了
				go func() {
					time.Sleep(100 * time.Millisecond)
					os.Exit(0)
				}()
				ctx.App.Stop()
			}
		}
		return event
	})
}
