package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// ファイル一覧を表示
func ShowFileList(app *tview.Application, stagedFiles, modifiedFiles, untrackedFiles []string, repoRoot string, onSelect func(file string, status string), onUpdate func()) tview.Primitive {
	// テキストビューを作成
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	textView.SetBackgroundColor(util.MyColor.BackgroundColor)

	// ファイル一覧を構築
	var content strings.Builder
	var regions []string
	var fileMap = make(map[string]string)
	var fileStatusMap = make(map[string]string) // ファイルの状態を保存

	// Staged ファイル
	if len(stagedFiles) > 0 {
		content.WriteString("[green]Changes to be committed:[white]\n")
		for _, file := range stagedFiles {
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
	if len(modifiedFiles) > 0 {
		content.WriteString("[yellow]Changes not staged for commit:[white]\n")
		for _, file := range modifiedFiles {
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
	if len(untrackedFiles) > 0 {
		content.WriteString("[red]Untracked files:[white]\n")
		for _, file := range untrackedFiles {
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

	// テキストビューにファイル一覧を表示
	textView.SetText(content.String())

	// 現在選択されているリージョンのインデックス
	currentSelection := 0
	if len(regions) > 0 {
		textView.Highlight(regions[currentSelection])
	}

	// キー入力の処理
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if currentSelection > 0 {
				currentSelection--
				textView.Highlight(regions[currentSelection])
				textView.ScrollToHighlight()
			}
			return nil
		case tcell.KeyDown:
			if currentSelection < len(regions)-1 {
				currentSelection++
				textView.Highlight(regions[currentSelection])
				textView.ScrollToHighlight()
			}
			return nil
		case tcell.KeyEnter:
			if currentSelection >= 0 && currentSelection < len(regions) {
				regionID := regions[currentSelection]
				file := fileMap[regionID]
				status := fileStatusMap[regionID]
				if onSelect != nil {
					onSelect(file, status)
				}
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				if currentSelection > 0 {
					currentSelection--
					textView.Highlight(regions[currentSelection])
					textView.ScrollToHighlight()
				}
				return nil
			case 'j':
				if currentSelection < len(regions)-1 {
					currentSelection++
					textView.Highlight(regions[currentSelection])
					textView.ScrollToHighlight()
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

	return textView
}
