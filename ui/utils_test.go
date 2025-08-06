package ui

import (
	"testing"
)

func TestCalculateMaxLineNumberDigits(t *testing.T) {
	tests := []struct {
		name       string
		oldLineMap map[int]int
		newLineMap map[int]int
		want       int
	}{
		{
			name:       "1桁",
			oldLineMap: map[int]int{0: 1, 1: 2},
			newLineMap: map[int]int{0: 1, 1: 2},
			want:       1,
		},
		{
			name:       "2桁",
			oldLineMap: map[int]int{0: 10, 1: 99},
			newLineMap: map[int]int{0: 10, 1: 99},
			want:       2,
		},
		{
			name:       "3桁",
			oldLineMap: map[int]int{0: 100, 1: 999},
			newLineMap: map[int]int{0: 100, 1: 999},
			want:       3,
		},
		{
			name:       "oldが大きい",
			oldLineMap: map[int]int{0: 1000},
			newLineMap: map[int]int{0: 99},
			want:       4,
		},
		{
			name:       "newが大きい",
			oldLineMap: map[int]int{0: 99},
			newLineMap: map[int]int{0: 1000},
			want:       4,
		},
		{
			name:       "空のマップ",
			oldLineMap: map[int]int{},
			newLineMap: map[int]int{},
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateMaxLineNumberDigits(tt.oldLineMap, tt.newLineMap)
			if got != tt.want {
				t.Errorf("calculateMaxLineNumberDigits() = %d, want %d", got, tt.want)
			}
		})
	}
}
