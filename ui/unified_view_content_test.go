package ui

import (
	"testing"
)

func TestGenerateUnifiedViewContent(t *testing.T) {
	tests := []struct {
		name         string
		diffText     string
		oldLineMap   map[int]int
		newLineMap   map[int]int
		wantContent  []string
		wantLineNums []string
	}{
		{
			name: "削除行のみ",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,2 @@
 line1
-line2
 line3`,
			oldLineMap: map[int]int{
				0: 1,
				1: 2,
				2: 3,
			},
			newLineMap: map[int]int{
				0: 1,
				2: 2,
			},
			wantContent: []string{
				"line1",
				"[red]-line2[-]",
				"line3",
			},
			wantLineNums: []string{
				"1 │ ",
				"2 │ ",
				"2 │ ",
			},
		},
		{
			name: "追加行のみ",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -1,2 +1,3 @@
 line1
+line2
 line3`,
			oldLineMap: map[int]int{
				0: 1,
				2: 2,
			},
			newLineMap: map[int]int{
				0: 1,
				1: 2,
				2: 3,
			},
			wantContent: []string{
				"line1",
				"[green]+line2[-]",
				"line3",
			},
			wantLineNums: []string{
				"1 │ ",
				"2 │ ",
				"3 │ ",
			},
		},
		{
			name: "変更（削除と追加）",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2_modified
 line3`,
			oldLineMap: map[int]int{
				0: 1,
				1: 2,
				3: 3,
			},
			newLineMap: map[int]int{
				0: 1,
				2: 2,
				3: 3,
			},
			wantContent: []string{
				"line1",
				"[red]-line2[-]",
				"[green]+line2_modified[-]",
				"line3",
			},
			wantLineNums: []string{
				"1 │ ",
				"2 │ ",
				"2 │ ",
				"3 │ ",
			},
		},
		{
			name: "行番号の桁数が異なる場合",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -98,3 +98,3 @@
 line98
-line99
+line99_modified
 line100`,
			oldLineMap: map[int]int{
				0: 98,
				1: 99,
				3: 100,
			},
			newLineMap: map[int]int{
				0: 98,
				2: 99,
				3: 100,
			},
			wantContent: []string{
				"line98",
				"[red]-line99[-]",
				"[green]+line99_modified[-]",
				"line100",
			},
			wantLineNums: []string{
				" 98 │ ",
				" 99 │ ",
				" 99 │ ",
				"100 │ ",
			},
		},
		{
			name: "ヘッダー行の除外",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -1,1 +1,1 @@
-old
+new`,
			oldLineMap: map[int]int{
				0: 1,
			},
			newLineMap: map[int]int{
				1: 1,
			},
			wantContent: []string{
				"[red]-old[-]",
				"[green]+new[-]",
			},
			wantLineNums: []string{
				"1 │ ",
				"1 │ ",
			},
		},
		{
			name: "ブラケットを含むテキストのエスケープ",
			diffText: `diff --git a/test.go b/test.go
index 123..456 789
--- a/test.go
+++ b/test.go
@@ -1,2 +1,2 @@
-var foo [int]string
+var foo [white]string`,
			oldLineMap: map[int]int{
				0: 1,
			},
			newLineMap: map[int]int{
				1: 1,
			},
			wantContent: []string{
				"[red]-var foo [int[]string[-]",
				"[green]+var foo [white[]string[-]",
			},
			wantLineNums: []string{
				"1 │ ",
				"1 │ ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := generateUnifiedViewContent(tt.diffText, tt.oldLineMap, tt.newLineMap)

			// Check number of lines
			if len(content.Lines) != len(tt.wantContent) {
				t.Errorf("Lines count mismatch: got %d, want %d", len(content.Lines), len(tt.wantContent))
			}

			// Check content and line numbers
			for i := range tt.wantContent {
				if i < len(content.Lines) {
					if content.Lines[i].Content != tt.wantContent[i] {
						t.Errorf("Line[%d].Content: got %q, want %q", i, content.Lines[i].Content, tt.wantContent[i])
					}
					if content.Lines[i].LineNumber != tt.wantLineNums[i] {
						t.Errorf("Line[%d].LineNumber: got %q, want %q", i, content.Lines[i].LineNumber, tt.wantLineNums[i])
					}
				}
			}
		})
	}
}

func TestColorizeDiff(t *testing.T) {
	tests := []struct {
		name     string
		diffText string
		want     string
	}{
		{
			name: "基本的な色付け",
			diffText: ` line1
-deleted
+added`,
			want: "line1\n[red]-deleted[-]\n[green]+added[-]\n",
		},
		{
			name: "ヘッダー行の除外",
			diffText: `diff --git a/test.txt b/test.txt
index 123..456 789
--- a/test.txt
+++ b/test.txt
@@ -1,1 +1,1 @@
-old
+new`,
			want: "[red]-old[-]\n[green]+new[-]\n",
		},
		{
			name:     "空のdiff",
			diffText: "",
			want:     "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorizeDiff(tt.diffText)
			if got != tt.want {
				t.Errorf("ColorizeDiff() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsUnifiedHeaderLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"diff --git", "diff --git a/file b/file", true},
		{"index", "index 123..456 789", true},
		{"---", "--- a/file", true},
		{"+++", "+++ b/file", true},
		{"ハンクヘッダー", "@@ -1,3 +1,3 @@", true},
		{"通常の行", " normal line", false},
		{"削除行", "-deleted line", false},
		{"追加行", "+added line", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnifiedHeaderLine(tt.line); got != tt.want {
				t.Errorf("isUnifiedHeaderLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestColorizeLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"削除行", "-deleted", "[red]-deleted[-]"},
		{"追加行", "+added", "[green]+added[-]"},
		{"通常の行", " normal", "normal"},
		{"空行", "", ""},
		{"その他の行", "other", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := colorizeLine(tt.line); got != tt.want {
				t.Errorf("colorizeLine(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestGenerateLineNumber(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		index      int
		maxDigits  int
		oldLineMap map[int]int
		newLineMap map[int]int
		want       string
	}{
		{
			name:       "削除行",
			line:       "[red]-deleted[-]",
			index:      0,
			maxDigits:  2,
			oldLineMap: map[int]int{0: 10},
			newLineMap: map[int]int{},
			want:       "10 │ ",
		},
		{
			name:       "追加行",
			line:       "[green]+added[-]",
			index:      1,
			maxDigits:  2,
			oldLineMap: map[int]int{},
			newLineMap: map[int]int{1: 11},
			want:       "11 │ ",
		},
		{
			name:       "共通行（新しい行番号）",
			line:       "common",
			index:      2,
			maxDigits:  3,
			oldLineMap: map[int]int{},
			newLineMap: map[int]int{2: 100},
			want:       "100 │ ",
		},
		{
			name:       "共通行（古い行番号）",
			line:       "common",
			index:      3,
			maxDigits:  3,
			oldLineMap: map[int]int{3: 101},
			newLineMap: map[int]int{},
			want:       "101 │ ",
		},
		{
			name:       "行番号なし",
			line:       "no number",
			index:      4,
			maxDigits:  2,
			oldLineMap: map[int]int{},
			newLineMap: map[int]int{},
			want:       "   │ ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateLineNumber(tt.line, tt.index, tt.maxDigits, tt.oldLineMap, tt.newLineMap)
			if got != tt.want {
				t.Errorf("generateLineNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}
