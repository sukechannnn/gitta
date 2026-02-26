package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/git"
	"github.com/sukechannnn/gitta/util"
)

// FileEntry represents a file entry in the file list with ID, path and status
type FileEntry struct {
	ID           string
	Path         string
	StageStatus  string // "staged", "unstaged", "untracked", "commit" (for git log view)
	ChangeStatus string // "added", "modified", "deleted", "untracked", "renamed", etc.
	IsDirectory  bool
}

// DirCollapseState manages the collapse state of directories in the file tree
type DirCollapseState struct {
	collapsed map[string]bool // key: "stageStatus:dirPath"
}

// NewDirCollapseState creates a new DirCollapseState
func NewDirCollapseState() *DirCollapseState {
	return &DirCollapseState{
		collapsed: make(map[string]bool),
	}
}

// IsCollapsed returns whether the directory is collapsed
func (s *DirCollapseState) IsCollapsed(stageStatus, dirPath string) bool {
	return s.collapsed[stageStatus+":"+dirPath]
}

// ToggleCollapsed toggles the collapse state of a directory
func (s *DirCollapseState) ToggleCollapsed(stageStatus, dirPath string) {
	key := stageStatus + ":" + dirPath
	s.collapsed[key] = !s.collapsed[key]
}

// SetCollapsed sets the collapse state of a directory
func (s *DirCollapseState) SetCollapsed(stageStatus, dirPath string, collapsed bool) {
	s.collapsed[stageStatus+":"+dirPath] = collapsed
}

// TreeNode represents a node in the file tree structure
type TreeNode struct {
	Name     string
	IsFile   bool
	Children map[string]*TreeNode
	FullPath string // ファイルおよびディレクトリのパス
}

// buildFileTreeFromGitFiles converts a list of git.FileInfo into a tree structure
func buildFileTreeFromGitFiles(files []git.FileInfo) *TreeNode {
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
				} else {
					newNode.FullPath = strings.Join(parts[:i+1], "/")
				}
				currentNode.Children[part] = newNode
			}

			currentNode = currentNode.Children[part]
		}
	}

	return root
}

// buildFileTreeFromFileEntries converts a list of FileEntry into a tree structure
func buildFileTreeFromFileEntries(files []FileEntry) *TreeNode {
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
				} else {
					newNode.FullPath = strings.Join(parts[:i+1], "/")
				}
				currentNode.Children[part] = newNode
			}

			currentNode = currentNode.Children[part]
		}
	}

	return root
}

// renderFileTreeForGitFiles renders the tree structure for git.FileInfo
func renderFileTreeForGitFiles(
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
	collapseState *DirCollapseState,
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

			// tviewの色タグをエスケープ
			escapedDisplayName := escapeTviewTags(displayName)

			if focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[white:blue]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, escapedDisplayName))
			} else if !focusedPane && *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[black:white]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, escapedDisplayName))
			} else {
				sb.WriteString(fmt.Sprintf(`%s[white:%s]["file-%d"]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, *regionIndex, connector, escapedDisplayName))
			}
			lineNumberMap[*regionIndex] = *currentLine
			(*regionIndex)++
			(*currentLine)++
		} else {
			// ディレクトリの場合
			collapsed := collapseState != nil && collapseState.IsCollapsed(stageStatus, child.FullPath)
			escapedDirName := escapeTviewTags(child.Name)
			dirDisplay := escapedDirName + "/"

			selectable := hasDirectFileChildren(child)

			if selectable {
				// 直下にファイルがあるディレクトリ: 選択可能
				regionID := fmt.Sprintf("file-%d", *regionIndex)
				*fileList = append(*fileList, FileEntry{
					ID:          regionID,
					Path:        child.FullPath,
					StageStatus: stageStatus,
					IsDirectory: true,
				})

				if focusedPane && *regionIndex == currentSelection {
					sb.WriteString(fmt.Sprintf(`%s[white:blue]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, dirDisplay))
				} else if !focusedPane && *regionIndex == currentSelection {
					sb.WriteString(fmt.Sprintf(`%s[black:white]["file-%d"]%s%s[""][-:-]`+"\n", prefix, *regionIndex, connector, dirDisplay))
				} else {
					sb.WriteString(fmt.Sprintf(`%s[white:%s]["file-%d"]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, *regionIndex, connector, dirDisplay))
				}
				lineNumberMap[*regionIndex] = *currentLine
				(*regionIndex)++
			} else {
				// 直下にファイルがないディレクトリ: ラベルのみ（選択不可）
				sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, dirDisplay))
			}
			(*currentLine)++

			// 折りたたまれていない場合のみ子要素を描画
			if !collapsed {
				renderFileTreeForGitFiles(child, depth+1, childPrefix, sb, fileList,
					stageStatus, regionIndex, currentSelection, focusedPane, lineNumberMap, currentLine, fileInfos, collapseState)
			}
		}
	}
}

// renderFileTreeForFileEntries renders the tree structure for FileEntry (used by git log view)
func renderFileTreeForFileEntries(
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
	fileEntries []FileEntry,
	collapseState *DirCollapseState,
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
			displayName := child.Name

			// ファイルのステータスを検索して装飾を追加
			for _, fileInfo := range fileEntries {
				if fileInfo.Path == child.FullPath {
					displayName = formatFileWithStatus(child.Name, fileInfo.ChangeStatus)
					break
				}
			}

			// tviewの色タグをエスケープ
			escapedDisplayName := escapeTviewTags(displayName)

			if *regionIndex == currentSelection && focusedPane {
				sb.WriteString(fmt.Sprintf(`%s[white:blue]%s%s[""][-:-]`+"\n", prefix, connector, escapedDisplayName))
			} else if *regionIndex == currentSelection {
				sb.WriteString(fmt.Sprintf(`%s[black:white]%s%s[""][-:-]`+"\n", prefix, connector, escapedDisplayName))
			} else {
				sb.WriteString(fmt.Sprintf(`%s[white:%s]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, connector, escapedDisplayName))
			}
			lineNumberMap[*regionIndex] = *currentLine
			(*regionIndex)++
			(*currentLine)++
		} else {
			// ディレクトリの場合
			collapsed := collapseState != nil && collapseState.IsCollapsed(stageStatus, child.FullPath)
			escapedDirName := escapeTviewTags(child.Name)
			dirDisplay := escapedDirName + "/"

			selectable := hasDirectFileChildren(child)

			if selectable {
				// 直下にファイルがあるディレクトリ: 選択可能
				regionID := fmt.Sprintf("file-%d", *regionIndex)
				if fileList != nil {
					*fileList = append(*fileList, FileEntry{
						ID:          regionID,
						Path:        child.FullPath,
						StageStatus: stageStatus,
						IsDirectory: true,
					})
				}

				if focusedPane && *regionIndex == currentSelection {
					sb.WriteString(fmt.Sprintf(`%s[white:blue]%s%s[""][-:-]`+"\n", prefix, connector, dirDisplay))
				} else if !focusedPane && *regionIndex == currentSelection {
					sb.WriteString(fmt.Sprintf(`%s[black:white]%s%s[""][-:-]`+"\n", prefix, connector, dirDisplay))
				} else {
					sb.WriteString(fmt.Sprintf(`%s[white:%s]%s%s[""][-:-]`+"\n", prefix, util.NotSelectedFileLineColor, connector, dirDisplay))
				}
				lineNumberMap[*regionIndex] = *currentLine
				(*regionIndex)++
			} else {
				// 直下にファイルがないディレクトリ: ラベルのみ（選択不可）
				sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, dirDisplay))
			}
			(*currentLine)++

			// 折りたたまれていない場合のみ子要素を描画
			if !collapsed {
				renderFileTreeForFileEntries(child, depth+1, childPrefix, sb, fileList,
					stageStatus, regionIndex, currentSelection, focusedPane, lineNumberMap, currentLine, fileEntries, collapseState)
			}
		}
	}
}

// collectPathsInTreeOrder collects file paths in the same order as tree rendering
// (directories first, then files, both sorted alphabetically at each level)
// Collapsed directories' children are skipped.
func collectPathsInTreeOrder(node *TreeNode, collapseState *DirCollapseState, stageStatus string) []string {
	var sortedKeys []string
	for key := range node.Children {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

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

	var result []string
	allItems := append(directories, files...)
	for _, key := range allItems {
		child := node.Children[key]
		if child.IsFile {
			result = append(result, child.FullPath)
		} else {
			// 直下にファイルがあるディレクトリのみリストに追加
			if hasDirectFileChildren(child) {
				result = append(result, child.FullPath)
			}
			// 折りたたまれていない場合のみ子要素を追加
			collapsed := collapseState != nil && collapseState.IsCollapsed(stageStatus, child.FullPath)
			if !collapsed {
				result = append(result, collectPathsInTreeOrder(child, collapseState, stageStatus)...)
			}
		}
	}
	return result
}

// hasDirectFileChildren returns true if the node has at least one direct file child
func hasDirectFileChildren(node *TreeNode) bool {
	for _, child := range node.Children {
		if child.IsFile {
			return true
		}
	}
	return false
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

// escapeTviewTags escapes tview color tag characters in text
func escapeTviewTags(text string) string {
	return tview.Escape(text)
}
