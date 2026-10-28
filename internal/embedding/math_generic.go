//go:build !(goexperiment.simd && amd64)

package embedding

func cosineSums(a, b []float32) (dot, aa, bb float64) {
	return cosineSumsScalar(a, b)
}
