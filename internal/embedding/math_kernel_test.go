package embedding

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestCosineSumsMatchesScalarReference checks the build's kernel (the AVX2
// one under GOEXPERIMENT=simd on amd64, otherwise the scalar one) against the
// float64 reference across lengths that do and do not fill whole lanes.
func TestCosineSumsMatchesScalarReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17, 100, 768, 769, 1536, 3072} {
		a := make([]float32, n)
		b := make([]float32, n)
		for i := range a {
			a[i] = rng.Float32()*2 - 1
			b[i] = rng.Float32()*2 - 1
		}
		gd, ga, gb := cosineSums(a, b)
		wd, wa, wb := cosineSumsScalar(a, b)
		for _, pair := range [][2]float64{{gd, wd}, {ga, wa}, {gb, wb}} {
			if diff := math.Abs(pair[0] - pair[1]); diff > 1e-4*math.Max(1, math.Abs(pair[1])) {
				t.Fatalf("n=%d: kernel %v, reference %v (diff %g)", n, pair[0], pair[1], diff)
			}
		}
	}
}

func BenchmarkCosineSimilarity768(b *testing.B) {
	rng := rand.New(rand.NewPCG(3, 4))
	x := make([]float32, 768)
	y := make([]float32, 768)
	for i := range x {
		x[i] = rng.Float32()
		y[i] = rng.Float32()
	}
	for b.Loop() {
		if _, err := CosineSimilarity(x, y); err != nil {
			b.Fatal(err)
		}
	}
}
