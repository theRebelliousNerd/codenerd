//go:build goexperiment.simd && amd64

package embedding

import "simd/archsimd"

// hasAVX2FMA is read once: the kernel below needs both 256-bit lanes and
// fused multiply-add.
var hasAVX2FMA = archsimd.X86.AVX2() && archsimd.X86.FMA()

// cosineSums accumulates the three sums in eight float32 lanes with fused
// multiply-add and reduces the lanes once, in float64, at the end. The tail
// shorter than a lane goes through the scalar kernel.
//
// Accumulating in float32 lanes differs from the float64 reference by
// rounding only: over a 768-dimension embedding each lane sums 96 products.
func cosineSums(a, b []float32) (dot, aa, bb float64) {
	if !hasAVX2FMA || len(a) < 8 {
		return cosineSumsScalar(a, b)
	}
	b = b[:len(a)]
	var vdot, vaa, vbb archsimd.Float32x8
	i := 0
	for ; i+8 <= len(a); i += 8 {
		va := archsimd.LoadFloat32x8(a[i : i+8])
		vb := archsimd.LoadFloat32x8(b[i : i+8])
		vdot = va.MulAdd(vb, vdot)
		vaa = va.MulAdd(va, vaa)
		vbb = vb.MulAdd(vb, vbb)
	}
	var ld, la, lb [8]float32
	vdot.StoreArray(&ld)
	vaa.StoreArray(&la)
	vbb.StoreArray(&lb)
	for lane := range ld {
		dot += float64(ld[lane])
		aa += float64(la[lane])
		bb += float64(lb[lane])
	}
	td, ta, tb := cosineSumsScalar(a[i:], b[i:])
	return dot + td, aa + ta, bb + tb
}
