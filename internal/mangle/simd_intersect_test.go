package mangle

import (
	"reflect"
	"testing"
)

func TestIntersectSIMD(t *testing.T) {
	tests := []struct {
		name string
		a    []uint64
		b    []uint64
		want []uint64
	}{
		{
			name: "both empty",
			a:    []uint64{},
			b:    []uint64{},
			want: nil,
		},
		{
			name: "a empty",
			a:    []uint64{},
			b:    []uint64{1, 2, 3},
			want: nil,
		},
		{
			name: "b empty",
			a:    []uint64{1, 2, 3},
			b:    []uint64{},
			want: nil,
		},
		{
			name: "disjoint",
			a:    []uint64{1, 3, 5},
			b:    []uint64{2, 4, 6},
			want: nil,
		},
		{
			name: "identical",
			a:    []uint64{1, 2, 3},
			b:    []uint64{1, 2, 3},
			want: []uint64{1, 2, 3},
		},
		{
			name: "partial overlap",
			a:    []uint64{1, 3, 5, 7, 9},
			b:    []uint64{3, 4, 5, 9, 10},
			want: []uint64{3, 5, 9},
		},
		{
			name: "a is subset of b",
			a:    []uint64{2, 4},
			b:    []uint64{1, 2, 3, 4, 5},
			want: []uint64{2, 4},
		},
		{
			name: "b is subset of a",
			a:    []uint64{1, 2, 3, 4, 5},
			b:    []uint64{2, 4},
			want: []uint64{2, 4},
		},
		{
			name: "large datasets matching blocks",
			a:    []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			b:    []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			want: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name: "large datasets partial blocks",
			a:    []uint64{1, 2, 3, 4, 7, 8, 9, 10},
			b:    []uint64{3, 4, 5, 6, 9, 10, 11, 12},
			want: []uint64{3, 4, 9, 10},
		},
		{
			name: "nil slices",
			a:    nil,
			b:    nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectSIMD(tt.a, tt.b)
			if len(got) == 0 && len(tt.want) == 0 {
				return // Both are empty/nil, which is correct
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IntersectSIMD() = %v, want %v", got, tt.want)
			}
		})
	}
}
