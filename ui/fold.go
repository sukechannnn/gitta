package ui

// FoldState manages the expansion state of foldable ranges
type FoldState struct {
	expandedLines map[int]bool // Map of display line index -> expanded state
}

// NewFoldState creates a new FoldState
func NewFoldState() *FoldState {
	return &FoldState{
		expandedLines: make(map[int]bool),
	}
}

// IsExpanded checks if a fold at the given line is expanded
func (fs *FoldState) IsExpanded(lineIndex int) bool {
	return fs.expandedLines[lineIndex]
}

// ToggleExpand toggles the expansion state of a fold
func (fs *FoldState) ToggleExpand(lineIndex int) {
	fs.expandedLines[lineIndex] = !fs.expandedLines[lineIndex]
}

// Reset clears all expansion state
func (fs *FoldState) Reset() {
	fs.expandedLines = make(map[int]bool)
}
