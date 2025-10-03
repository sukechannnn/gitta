package ui

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// GitLogEntry represents a single git log entry
type GitLogEntry struct {
	Hash     string
	Message  string
	Author   string
	Date     string
	FullLine string // 元の表示用行
}

// GitLogView manages the git log display and navigation
type GitLogView struct {
	app             *tview.Application
	logView         *tview.TextView
	commitView      *tview.TextView
	commitFileList  *tview.TextView
	commitDiffView  *tview.TextView
	commitSplitFlex *tview.Flex
	flex            *tview.Flex
	repoRoot        string
	logEntries      []GitLogEntry
	currentLine     int
	showingCommit   bool
	onExit          func()
	scrollOffset    int // スクロールオフセット
	commitFiles     []CommitFileInfo
	selectedFile    int
	leftPaneFocused bool
	// ggコマンド用の状態
	gPressed  *bool
	lastGTime *time.Time
}

// NewGitLogView creates a new git log viewer
// 既存のfileListViewとdiffViewを再利用するバージョンも考えられるが、
// git log専用のロジックが多いため、独立したコンポーネントとして実装
func NewGitLogView(app *tview.Application, repoRoot string, onExit func()) *GitLogView {
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true)
	logView.SetBorder(true).SetTitle("Git Log (j/k: navigate, Enter: show commit, Esc: exit)")
	logView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	commitView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true)
	commitView.SetBorder(true).SetTitle("Commit Details (Esc: back to log)")
	commitView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// コミット詳細用のファイルリストと差分ビュー
	commitFileList := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true)
	commitFileList.SetBorder(true).SetTitle("Changed Files (j/k: navigate, Enter: switch to diff)")
	commitFileList.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	commitDiffView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true)
	commitDiffView.SetBorder(true).SetTitle("File Diff (Enter: back to file list)")
	commitDiffView.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// 既存のレイアウト構造を再利用
	// 縦線ボーダー
	verticalBorderLeft := tview.NewTextView()
	verticalBorderLeft.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	verticalBorderLeft.SetBorderPadding(0, 0, 0, 0)

	verticalBorder := tview.NewTextView()
	verticalBorder.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	verticalBorder.SetBorderPadding(0, 0, 0, 0)

	// コミット詳細用のunifiedViewFlexと同じ構造
	commitDiffViewFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	commitDiffViewFlex.AddItem(commitDiffView, 0, 1, false)
	commitDiffViewFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())

	// root_editor.goと同じレイアウト構造
	commitSplitFlex := tview.NewFlex()
	commitSplitFlex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	commitSplitFlex.
		AddItem(verticalBorderLeft, 1, 0, false).
		AddItem(commitFileList, 0, FileListFlexRatio, true).
		AddItem(verticalBorder, 1, 0, false).
		AddItem(commitDiffViewFlex, 0, DiffViewFlexRatio, false)

	flex := tview.NewFlex().
		AddItem(logView, 0, 1, true)

	glv := &GitLogView{
		app:             app,
		logView:         logView,
		commitView:      commitView,
		commitFileList:  commitFileList,
		commitDiffView:  commitDiffView,
		commitSplitFlex: commitSplitFlex,
		flex:            flex,
		repoRoot:        repoRoot,
		logEntries:      []GitLogEntry{},
		currentLine:     0,
		showingCommit:   false,
		onExit:          onExit,
		scrollOffset:    0,
		commitFiles:     []CommitFileInfo{},
		selectedFile:    0,
		leftPaneFocused: true,
		gPressed:        new(bool),
		lastGTime:       new(time.Time),
	}

	glv.setupKeyBindings()
	glv.loadGitLog()

	// 初期レンダリング後に再描画を予約（レイアウトが確定してから）
	go func() {
		// 少し待ってから再描画
		glv.app.QueueUpdateDraw(func() {
			glv.updateSelection()
		})
	}()

	return glv
}

// GetView returns the main flex view
func (glv *GitLogView) GetView() tview.Primitive {
	return glv.flex
}

// convertANSIToTviewColors converts ANSI escape codes to tview color tags
func convertANSIToTviewColors(text string) string {
	// ANSI escape code patterns
	patterns := []struct {
		pattern     string
		replacement string
	}{
		{`\x1b\[1;31m`, "[red]"},    // Bold Red
		{`\x1b\[31m`, "[red]"},      // Red
		{`\x1b\[1;32m`, "[green]"},  // Bold Green
		{`\x1b\[32m`, "[green]"},    // Green
		{`\x1b\[1;33m`, "[yellow]"}, // Bold Yellow
		{`\x1b\[33m`, "[yellow]"},   // Yellow
		{`\x1b\[1;34m`, "[blue]"},   // Bold Blue
		{`\x1b\[34m`, "[blue]"},     // Blue
		{`\x1b\[1;35m`, "[purple]"}, // Bold Magenta
		{`\x1b\[35m`, "[purple]"},   // Magenta
		{`\x1b\[1;36m`, "[cyan]"},   // Bold Cyan
		{`\x1b\[36m`, "[cyan]"},     // Cyan
		{`\x1b\[1;37m`, "[white]"},  // Bold White
		{`\x1b\[37m`, "[white]"},    // White
		{`\x1b\[1m`, ""},            // Bold (ignore for now)
		{`\x1b\[m`, "[-]"},          // Reset
		{`\x1b\[0m`, "[-]"},         // Reset
		{`\x1b\[(0;)?39m`, "[-]"},   // Default color
	}

	result := text
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		result = re.ReplaceAllString(result, p.replacement)
	}

	// Clean up any remaining escape codes
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	result = re.ReplaceAllString(result, "")

	return result
}

// loadGitLog executes git log and parses the output
func (glv *GitLogView) loadGitLog() {
	cmd := exec.Command("git", "log", "--graph", "--pretty=format:%C(yellow)%h%Creset - %s %Cgreen(%cr) %C(bold blue)<%an>%Creset", "--color=always", "-n", "200")
	cmd.Dir = glv.repoRoot

	output, err := cmd.Output()
	if err != nil {
		glv.logView.SetText("[red]Failed to load git log: " + err.Error())
		return
	}

	lines := strings.Split(string(output), "\n")
	glv.logEntries = []GitLogEntry{}

	// Parse git log output
	hashRegex := regexp.MustCompile(`([a-f0-9]{7,})`)

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Convert ANSI codes to tview colors
		cleanLine := convertANSIToTviewColors(line)

		// Extract commit hash if this line contains one
		var hash string
		if matches := hashRegex.FindStringSubmatch(line); len(matches) > 0 {
			hash = matches[0]
		}

		// Create log entry
		entry := GitLogEntry{
			Hash:     hash,
			FullLine: cleanLine,
		}
		glv.logEntries = append(glv.logEntries, entry)
	}

	glv.updateSelection()
}

// updateSelection updates the visual selection in the log view
func (glv *GitLogView) updateSelection() {
	if len(glv.logEntries) == 0 {
		return
	}

	// 表示可能な行数を取得（概算）
	_, _, _, height := glv.logView.GetInnerRect()
	visibleLines := height
	if visibleLines <= 0 {
		// 初期表示やレイアウト前の場合は推定値を使用
		visibleLines = 25 // より大きなデフォルト値
	}

	// カーソルが画面の境界に近づいたらスクロール
	relativePos := glv.currentLine - glv.scrollOffset

	if relativePos < 3 && glv.scrollOffset > 0 {
		// 上端に近い場合は上にスクロール
		glv.scrollOffset = glv.currentLine - 3
		if glv.scrollOffset < 0 {
			glv.scrollOffset = 0
		}
	} else if relativePos >= visibleLines-3 {
		// 下端に近い場合は下にスクロール
		glv.scrollOffset = glv.currentLine - visibleLines + 4
		if glv.scrollOffset < 0 {
			glv.scrollOffset = 0
		}
		if glv.scrollOffset >= len(glv.logEntries) {
			glv.scrollOffset = len(glv.logEntries) - 1
		}
	}

	// 初期表示時は常に最初から表示する
	if glv.currentLine == 0 && glv.scrollOffset == 0 {
		// 何もしない（先頭から表示）
	}

	// 表示範囲の計算
	endLine := glv.scrollOffset + visibleLines
	if endLine > len(glv.logEntries) {
		endLine = len(glv.logEntries)
	}

	// より多くの行を表示するため、最大値も設定
	if endLine-glv.scrollOffset < 10 && len(glv.logEntries) > 10 {
		endLine = glv.scrollOffset + min(len(glv.logEntries)-glv.scrollOffset, 30)
	}

	// 表示範囲内のエントリーでコンテンツを構築
	var content strings.Builder
	for i := glv.scrollOffset; i < endLine; i++ {
		if i >= len(glv.logEntries) {
			break
		}
		entry := glv.logEntries[i]
		if i == glv.currentLine {
			content.WriteString("[white:blue]" + entry.FullLine + "[-:-]\n")
		} else {
			content.WriteString(entry.FullLine + "\n")
		}
	}

	glv.logView.SetText(content.String())
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// loadCommitFiles loads the list of files changed in a commit
func (glv *GitLogView) loadCommitFiles(commitHash string) {
	cmd := exec.Command(
		"git",
		"-c", "core.quotepath=false",
		"show",
		"--name-status",
		"--format=",
		commitHash,
	)
	cmd.Dir = glv.repoRoot

	output, err := cmd.Output()
	if err != nil {
		glv.commitFiles = []CommitFileInfo{}
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	glv.commitFiles = []CommitFileInfo{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		rawStatus := strings.TrimSpace(parts[0])
		fileName := strings.TrimSpace(parts[len(parts)-1])
		if rawStatus == "" || fileName == "" {
			continue
		}

		fileInfo := CommitFileInfo{
			Status:   normalizeCommitStatus(rawStatus),
			FileName: fileName,
		}
		glv.commitFiles = append(glv.commitFiles, fileInfo)
	}

	glv.selectedFile = 0
	glv.updateCommitFileList()
}

// normalizeCommitStatus converts git show status codes to descriptive labels
func normalizeCommitStatus(rawStatus string) string {
	if rawStatus == "" {
		return ""
	}

	switch rawStatus[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type-changed"
	default:
		return strings.ToLower(rawStatus)
	}
}

// updateCommitFileList updates the file list display using tree structure
func (glv *GitLogView) updateCommitFileList() {
	if len(glv.commitFiles) == 0 {
		glv.commitFileList.SetText("No files changed")
		return
	}

	// Build tree from commit files
	tree := buildFileTreeFromCommitFiles(glv.commitFiles)

	var content strings.Builder
	var displayFiles []CommitFileInfo
	regionIndex := 0
	currentLine := 0
	lineNumberMap := make(map[int]int)

	renderFileTreeForCommitFiles(
		tree,
		0,
		"",
		&content,
		&displayFiles,
		&regionIndex,
		glv.selectedFile,
		glv.leftPaneFocused,
		lineNumberMap,
		&currentLine,
		glv.commitFiles,
	)

	glv.commitFileList.SetText(content.String())
}

// showFileDiff shows the diff for the selected file
func (glv *GitLogView) showFileDiff(commitHash string) {
	if glv.selectedFile >= len(glv.commitFiles) {
		glv.commitDiffView.SetText("No file selected")
		return
	}

	file := glv.commitFiles[glv.selectedFile]
	cmd := exec.Command("git", "show", "--color=always", commitHash, "--", file.FileName)
	cmd.Dir = glv.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		glv.commitDiffView.SetText("[red]Failed to show diff: " + err.Error() + "\n\nOutput:\n" + string(output))
		return
	}

	// Convert ANSI codes in the output
	cleanOutput := convertANSIToTviewColors(string(output))
	glv.commitDiffView.SetText(cleanOutput)
}

// showCommitDetails shows the details of the selected commit
func (glv *GitLogView) showCommitDetails() {
	if glv.currentLine >= len(glv.logEntries) {
		return
	}

	entry := glv.logEntries[glv.currentLine]
	if entry.Hash == "" {
		return
	}

	// Load commit files
	glv.loadCommitFiles(entry.Hash)

	// Switch to split view
	glv.flex.Clear()
	glv.flex.AddItem(glv.commitSplitFlex, 0, 1, true)
	glv.showingCommit = true
	glv.leftPaneFocused = true

	// Show first file's diff
	if len(glv.commitFiles) > 0 {
		glv.showFileDiff(entry.Hash)
	}

	glv.app.SetFocus(glv.commitFileList)
}

// backToLog returns to the log view from commit details
func (glv *GitLogView) backToLog() {
	glv.flex.Clear()
	glv.flex.AddItem(glv.logView, 0, 1, true)
	glv.showingCommit = false
	glv.app.SetFocus(glv.logView)
}

// setupKeyBindings configures keyboard navigation
func (glv *GitLogView) setupKeyBindings() {
	// Log view key bindings
	glv.logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if glv.onExit != nil {
				glv.onExit()
			}
			return nil
		case tcell.KeyEnter:
			glv.showCommitDetails()
			return nil
		}

		switch event.Rune() {
		case 'q':
			glv.quitApplication()
			return nil
		case 'j':
			if glv.currentLine < len(glv.logEntries)-1 {
				glv.currentLine++
				glv.updateSelection()
			}
			return nil
		case 'k':
			if glv.currentLine > 0 {
				glv.currentLine--
				glv.updateSelection()
			}
			return nil
		case 'g':
			// ggで最上部に移動（root_editor.goと同じロジック）
			now := time.Now()
			if *glv.gPressed && now.Sub(*glv.lastGTime) < 500*time.Millisecond {
				// 2回目のg - 最上部に移動
				glv.currentLine = 0
				glv.scrollOffset = 0
				glv.updateSelection()
				*glv.gPressed = false
			} else {
				// 1回目のg
				*glv.gPressed = true
				*glv.lastGTime = now
			}
			return nil
		}

		return event
	})

	// Commit file list key bindings
	glv.commitFileList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if !glv.showingCommit {
			return event
		}

		currentCommitHash := ""
		if glv.currentLine < len(glv.logEntries) {
			currentCommitHash = glv.logEntries[glv.currentLine].Hash
		}

		switch event.Key() {
		case tcell.KeyEsc:
			glv.backToLog()
			return nil
		case tcell.KeyEnter:
			glv.leftPaneFocused = false
			glv.updateCommitFileList()
			glv.app.SetFocus(glv.commitDiffView)
			return nil
		}

		switch event.Rune() {
		case 'q':
			glv.quitApplication()
			return nil
		case 'j':
			if glv.selectedFile < len(glv.commitFiles)-1 {
				glv.selectedFile++
				glv.updateCommitFileList()
				if currentCommitHash != "" {
					glv.showFileDiff(currentCommitHash)
				}
			}
			return nil
		case 'k':
			if glv.selectedFile > 0 {
				glv.selectedFile--
				glv.updateCommitFileList()
				if currentCommitHash != "" {
					glv.showFileDiff(currentCommitHash)
				}
			}
			return nil
		}

		return event
	})

	// Commit diff view key bindings
	glv.commitDiffView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if !glv.showingCommit {
			return event
		}

		switch event.Key() {
		case tcell.KeyEsc:
			glv.backToLog()
			return nil
		case tcell.KeyEnter:
			glv.leftPaneFocused = true
			glv.updateCommitFileList()
			glv.app.SetFocus(glv.commitFileList)
			return nil
		}

		switch event.Rune() {
		case 'q':
			glv.quitApplication()
			return nil
		}

		return event
	})
}

func (glv *GitLogView) quitApplication() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	glv.app.Stop()
}
