//go:build amd64 && simd

package mangle

import (
	"reflect"
	"testing"
)

// TestIntersectSIMD_AMD64 validates the SIMD specific intersection logic for amd64.
func TestIntersectSIMD_AMD64(t *testing.T) {
	tests := []struct {
		name string
		a    []uint64
		b    []uint64
		want []uint64
	}{
		{
			name: "identical exact match",
			a:    []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			b:    []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			want: []uint64{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "both empty",
			a:    []uint64{},
			b:    []uint64{},
			want: nil,
		},
		{
			name: "disjoint",
			a:    []uint64{1, 3, 5, 7},
			b:    []uint64{2, 4, 6, 8},
			want: nil,
		},
		{
			name: "partial overlap",
			a:    []uint64{1, 2, 3, 4, 7, 8, 9, 10},
			b:    []uint64{3, 4, 5, 6, 9, 10, 11, 12},
			want: []uint64{3, 4, 9, 10},
		},
		{
			name: "different lengths",
			a:    []uint64{1, 2, 3, 4},
			b:    []uint64{1, 2, 3, 4, 5, 6},
			want: []uint64{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectSIMD(tt.a, tt.b)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IntersectSIMD() = %v, want %v", got, tt.want)
			}
		})
	}
}
