package util

import "github.com/gdamore/tcell/v2"

type MyColorType struct {
	BackgroundColor tcell.Color
}

var MyColor = MyColorType{
	BackgroundColor: tcell.NewHexColor(0x272A32),
}
