package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		startCol int
		maxCols  int
		want     string
	}{
		{
			name:     "ASCII - Start 0, Full Width",
			input:    "Hello",
			startCol: 0,
			maxCols:  10,
			want:     "Hello",
		},
		{
			name:     "ASCII - Start 1",
			input:    "Hello",
			startCol: 1,
			maxCols:  10,
			want:     "ello",
		},
		{
			name:     "ASCII - Start > Length",
			input:    "Hello",
			startCol: 10,
			maxCols:  5,
			want:     "",
		},
		{
			name:     "ASCII - Max Width Limit",
			input:    "Hello World",
			startCol: 0,
			maxCols:  5,
			want:     "Hello",
		},
		{
			name:     "ASCII - Start + Max Width Limit",
			input:    "Hello World",
			startCol: 6,
			maxCols:  5,
			want:     "World",
		},
		{
			name:     "Wide Char - Simple",
			input:    "你好世界",
			startCol: 0,
			maxCols:  4, // 2 chars (width 2 each)
			want:     "你好",
		},
		{
			name:     "Wide Char - Offset 2",
			input:    "你好世界",
			startCol: 2, // Skip '你' (width 2)
			maxCols:  4,
			want:     "好世",
		},
		{
			name:     "Wide Char - Offset 1 (Partial Skip)",
			input:    "你好世界",
			startCol: 1, // Skip partial '你' -> skips whole '你'
			maxCols:  4,
			want:     "好世",
		},
		{
			name:     "Mixed - ASCII and Wide",
			input:    "A你好B",
			startCol: 0,
			maxCols:  3, // A (1) + 你 (2) = 3
			want:     "A你",
		},
		{
			name:     "Mixed - Offset 1 (Limit 4)",
			input:    "A你好B",
			startCol: 1, // Skip 'A'
			maxCols:  4,
			want:     "你好", // Width 4. "B" excluded.
		},
		{
			name:     "Mixed - Offset 1 (Limit 5)",
			input:    "A你好B",
			startCol: 1, // Skip 'A'
			maxCols:  5,
			want:     "你好B", // Width 5.
		},
		{
			name:     "Mixed - Offset 2 (Partial Wide)",
			input:    "A你好B",
			startCol: 2, // Skip 'A' (1) and partial '你' (starts at 1)
			maxCols:  4,
			want:     "好B",
		},
		{
			name:     "Negative Start",
			input:    "Hello",
			startCol: -5,
			maxCols:  5,
			want:     "Hello",
		},
		{
			name:     "Zero Max Width",
			input:    "Hello",
			startCol: 0,
			maxCols:  0,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceString(tt.input, tt.startCol, tt.maxCols)
			assert.Equal(t, tt.want, got)
		})
	}
}
