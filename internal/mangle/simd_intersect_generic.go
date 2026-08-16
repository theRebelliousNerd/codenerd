package mangle

// IntersectSIMD computes the intersection of two sorted slices of uint64.
//
// The previous amd64/simd variant was removed because it performed no vector
// operations — it constructed archsimd.Uint64x4 values and then compared them
// element-by-element with scalar code (vA[0]==vB[0] && ...), plus a scalar
// merge loop — and it never compiled against Go 1.26's archsimd (implicit
// assignment to unexported field in struct literal). This generic
// implementation is now the single unconditional implementation for all
// architectures and build tags.

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
