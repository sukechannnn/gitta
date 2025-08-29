package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/gitta/util"
)

// CreateVerticalBorder creates a vertical border box
func CreateVerticalBorder() *tview.Box {
	verticalBorder := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// 縦線を描画
			style := tcell.StyleDefault.
				Foreground(tcell.ColorWhite).
				Background(util.BackgroundColor.ToTcellColor())
			for i := y; i < y+height; i++ {
				screen.SetContent(x, i, '│', nil, style)
			}
			return x, y, width, height
		})
	verticalBorder.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	return verticalBorder
}

// CreateHorizontalTopBorder creates a horizontal border box with intersection characters
// The positions of vertical lines are calculated based on the layout ratio
func CreateHorizontalTopBorder() *tview.Box {
	horizontalBorder := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// 横線を描画
			style := tcell.StyleDefault.
				Foreground(tcell.ColorWhite).
				Background(util.BackgroundColor.ToTcellColor())

			// まず横線を全体に描画
			for i := x; i < x+width; i++ {
				screen.SetContent(i, y, '─', nil, style)
			}

			leftBorderPos, middleBorderPos, rightBorderPos := calcBorderPos(width)

			// 交差部分を描画
			screen.SetContent(x+leftBorderPos, y, '┌', nil, style)
			screen.SetContent(x+middleBorderPos, y, '┬', nil, style)
			screen.SetContent(x+rightBorderPos, y, '┐', nil, style)

			return x, y, width, height
		})
	horizontalBorder.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	return horizontalBorder
}

// CreateHorizontalBottomBorder creates a horizontal border box for bottom with intersection characters
func CreateHorizontalBottomBorder() *tview.Box {
	horizontalBorder := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// 横線を描画
			style := tcell.StyleDefault.
				Foreground(tcell.ColorWhite).
				Background(util.BackgroundColor.ToTcellColor())

			// まず横線を全体に描画
			for i := x; i < x+width; i++ {
				screen.SetContent(i, y, '─', nil, style)
			}

			leftBorderPos, middleBorderPos, rightBorderPos := calcBorderPos(width)

			// 交差部分を描画（下線用）
			screen.SetContent(x+leftBorderPos, y, '└', nil, style)
			screen.SetContent(x+middleBorderPos, y, '┴', nil, style)
			screen.SetContent(x+rightBorderPos, y, '┘', nil, style)

			return x, y, width, height
		})
	horizontalBorder.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	return horizontalBorder
}

func calcBorderPos(width int) (int, int, int) {
	// 縦線の位置を計算
	// レイアウト: 縦線(1) + textView(比率FileListFlexRatio) + 縦線(1) + diffView(比率DiffViewFlexRatio) + 縦線(1)
	totalFlexWidth := width - 3                  // 3つの縦線の幅を除く
	unitWidth := totalFlexWidth / TotalFlexRatio // 比率の合計

	// 各縦線の位置
	leftBorderPos := 0                                 // 左端の縦線
	middleBorderPos := 1 + unitWidth*FileListFlexRatio // 左縦線(1) + textView(unitWidth*FileListFlexRatio)
	rightBorderPos := width - 1                        // 右端の縦線

	return leftBorderPos, middleBorderPos, rightBorderPos
}
