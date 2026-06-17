package mangle

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func TestIntersectSIMD_Basic(t *testing.T) {
	tests := []struct {
		name string
		a    []uint64
		b    []uint64
		want []uint64
	}{
		{
			name: "empty slices",
			a:    []uint64{},
			b:    []uint64{},
			want: nil,
		},
		{
			name: "one empty slice",
			a:    []uint64{1, 2, 3},
			b:    []uint64{},
			want: nil,
		},
		{
			name: "no overlap",
			a:    []uint64{1, 2, 3},
			b:    []uint64{4, 5, 6},
			want: nil,
		},
		{
			name: "exact match",
			a:    []uint64{1, 2, 3},
			b:    []uint64{1, 2, 3},
			want: []uint64{1, 2, 3},
		},
		{
			name: "partial overlap",
			a:    []uint64{1, 3, 5, 7, 9},
			b:    []uint64{2, 3, 4, 5, 6, 9},
			want: []uint64{3, 5, 9},
		},
		{
			name: "subset",
			a:    []uint64{2, 4},
			b:    []uint64{1, 2, 3, 4, 5},
			want: []uint64{2, 4},
		},
		{
			name: "blocks overlap exact (SIMD fast path)",
			a:    []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			b:    []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			want: []uint64{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "blocks partial overlap (SIMD slow path)",
			a:    []uint64{1, 3, 5, 7, 9, 11, 13, 15},
			b:    []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			want: []uint64{1, 3, 5, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectSIMD(tt.a, tt.b)
			if len(got) == 0 && len(tt.want) == 0 {
				return // Both empty is fine
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IntersectSIMD() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntersectSIMD_Large(t *testing.T) {
	// Generate large slices to hit multiple SIMD blocks
	var a, b []uint64
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			a = append(a, uint64(i))
		}
		if i%3 == 0 {
			b = append(b, uint64(i))
		}
	}

	var want []uint64
	for i := 0; i < 1000; i++ {
		if i%2 == 0 && i%3 == 0 {
			want = append(want, uint64(i))
		}
	}

	got := IntersectSIMD(a, b)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IntersectSIMD large slice mismatch")
	}
}

func naiveIntersect(a, b []uint64) []uint64 {
	var result []uint64
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			result = append(result, a[i])
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return result
}

func TestIntersectSIMD_Random(t *testing.T) {
	// Fuzz test with random arrays
	r := rand.New(rand.NewSource(42))
	for iter := 0; iter < 100; iter++ {
		lenA := r.Intn(200)
		lenB := r.Intn(200)

		var a, b []int
		for i := 0; i < lenA; i++ {
			a = append(a, r.Intn(1000))
		}
		for i := 0; i < lenB; i++ {
			b = append(b, r.Intn(1000))
		}

		sort.Ints(a)
		sort.Ints(b)

		// Remove duplicates to make them proper sets (as expected by SIMD)
		var aUniq, bUniq []uint64
		if len(a) > 0 {
			aUniq = append(aUniq, uint64(a[0]))
			for i := 1; i < len(a); i++ {
				if a[i] != a[i-1] {
					aUniq = append(aUniq, uint64(a[i]))
				}
			}
		}
		if len(b) > 0 {
			bUniq = append(bUniq, uint64(b[0]))
			for i := 1; i < len(b); i++ {
				if b[i] != b[i-1] {
					bUniq = append(bUniq, uint64(b[i]))
				}
			}
		}

		want := naiveIntersect(aUniq, bUniq)
		got := IntersectSIMD(aUniq, bUniq)

		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Random iter %d mismatch:\na=%v\nb=%v\ngot=%v\nwant=%v", iter, aUniq, bUniq, got, want)

		}
	}
}
