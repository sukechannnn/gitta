package ui

// FoldState manages the expansion state of foldable ranges
type FoldState struct {
	expandedFolds map[string]bool // Map of fold ID -> expanded state
}

// NewFoldState creates a new FoldState
func NewFoldState() *FoldState {
	return &FoldState{
		expandedFolds: make(map[string]bool),
	}
}

// IsExpanded checks if a fold with the given ID is expanded
func (fs *FoldState) IsExpanded(foldID string) bool {
	return fs.expandedFolds[foldID]
}

// ToggleExpand toggles the expansion state of a fold
func (fs *FoldState) ToggleExpand(foldID string) {
	fs.expandedFolds[foldID] = !fs.expandedFolds[foldID]
}

// Reset clears all expansion state
func (fs *FoldState) Reset() {
	fs.expandedFolds = make(map[string]bool)
}
