package util

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
)

type ColorColde string

const (
	BackgroundColor          = ColorColde("#272A32")
	NotSelectedFileLineColor = ColorColde("#383E50")
	MainTextColor            = ColorColde("#FFFFFF")
	CommitAreaBorderColor    = ColorColde("#4A5568")
	PlaceholderColor         = ColorColde("#808080")
)

func (c ColorColde) hex() string {
	return string(c)[1:]
}

func (c ColorColde) ToTcellColor() tcell.Color {
	hexValue, _ := strconv.ParseInt(c.hex(), 16, 32)
	return tcell.NewHexColor(int32(hexValue))
}
