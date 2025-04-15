package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// ファイル一覧を表示
func ShowFileList(app *tview.Application, modifiedFiles, untrackedFiles []string, onSelect func(file string)) {
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

	// 変更されたファイル
	content.WriteString("[yellow]Modified Files:[white]\n")
	for _, file := range modifiedFiles {
		file = strings.TrimSpace(file)
		if file != "" {
			regionID := fmt.Sprintf("file-%d", len(regions))
			regions = append(regions, regionID)
			fileMap[regionID] = file
			content.WriteString(fmt.Sprintf(`["file-%d"]  %s[""]`+"\n", len(regions)-1, file))
		}
	}

	// 未追跡ファイル
	if len(untrackedFiles) > 0 {
		// 空行を追加
		content.WriteString("\n")
		content.WriteString("[yellow]Untracked Files:[white]\n")
		for _, file := range untrackedFiles {
			file = strings.TrimSpace(file)
			if file != "" {
				regionID := fmt.Sprintf("file-%d", len(regions))
				regions = append(regions, regionID)
				fileMap[regionID] = file
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
				if onSelect != nil {
					onSelect(file)
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

	// アプリケーションのルートに設定
	app.SetRoot(textView, true).Run()
}
