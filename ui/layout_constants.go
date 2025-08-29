package ui

// レイアウト比率の定数定義
const (
	// Flexレイアウトの比率
	// FileList : DiffView = 2 : 6
	FileListFlexRatio = 2
	DiffViewFlexRatio = 6

	// 比率の合計（計算で使用）
	TotalFlexRatio = FileListFlexRatio + DiffViewFlexRatio
)
