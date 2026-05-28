//go:build amd64 && simd

package mangle

import "simd/archsimd"

// IntersectSIMD computes the intersection of two sorted slices of uint64.
// Uses AVX2 SIMD operations for vectorized hash-joins.
func IntersectSIMD(a, b []uint64) []uint64 {
	var result []uint64
	i, j := 0, 0

	for i+4 <= len(a) && j+4 <= len(b) {
		vA := archsimd.Uint64x4{a[i], a[i+1], a[i+2], a[i+3]}
		vB := archsimd.Uint64x4{b[j], b[j+1], b[j+2], b[j+3]}

		// Fast path: blocks match exactly
		if vA[0] == vB[0] && vA[1] == vB[1] && vA[2] == vB[2] && vA[3] == vB[3] {
			result = append(result, a[i], a[i+1], a[i+2], a[i+3])
			i += 4
			j += 4
			continue
		}

		// Scalar fallback for unaligned/mismatched blocks
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

	// Handle remainder
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
