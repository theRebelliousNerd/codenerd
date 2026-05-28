//go:build !amd64 || !simd

package mangle

// IntersectSIMD computes the intersection of two sorted slices of uint64.
// This is the generic pure-Go fallback implementation.
func IntersectSIMD(a, b []uint64) []uint64 {
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
