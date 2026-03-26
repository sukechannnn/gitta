package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/giff/util"
)

// CreateVerticalBorder creates a vertical border box
func CreateVerticalBorder() *tview.Box {
	verticalBorder := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// Draw vertical line
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
			// Draw horizontal line
			style := tcell.StyleDefault.
				Foreground(tcell.ColorWhite).
				Background(util.BackgroundColor.ToTcellColor())

			// First draw the horizontal line across the full width
			for i := x; i < x+width; i++ {
				screen.SetContent(i, y, '─', nil, style)
			}

			leftBorderPos, middleBorderPos, rightBorderPos := calcBorderPos(width)

			// Draw intersection characters
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
			// Draw horizontal line
			style := tcell.StyleDefault.
				Foreground(tcell.ColorWhite).
				Background(util.BackgroundColor.ToTcellColor())

			// First draw the horizontal line across the full width
			for i := x; i < x+width; i++ {
				screen.SetContent(i, y, '─', nil, style)
			}

			leftBorderPos, middleBorderPos, rightBorderPos := calcBorderPos(width)

			// Draw intersection characters (for bottom border)
			screen.SetContent(x+leftBorderPos, y, '└', nil, style)
			screen.SetContent(x+middleBorderPos, y, '┴', nil, style)
			screen.SetContent(x+rightBorderPos, y, '┘', nil, style)

			return x, y, width, height
		})
	horizontalBorder.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	return horizontalBorder
}

func calcBorderPos(width int) (int, int, int) {
	// Calculate vertical line positions
	// Layout: verticalLine(1) + textView(ratio FileListFlexRatio) + verticalLine(1) + diffView(ratio DiffViewFlexRatio) + verticalLine(1)
	totalFlexWidth := width - 3                  // Exclude the width of 3 vertical lines
	unitWidth := totalFlexWidth / TotalFlexRatio // Total ratio

	// Position of each vertical line
	leftBorderPos := 0                                 // Left edge vertical line
	middleBorderPos := 1 + unitWidth*FileListFlexRatio // Left line(1) + textView(unitWidth*FileListFlexRatio)
	rightBorderPos := width - 1                        // Right edge vertical line

	return leftBorderPos, middleBorderPos, rightBorderPos
}
