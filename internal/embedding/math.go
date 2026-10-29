package embedding

import (
	"fmt"
	"math"

	"codenerd/internal/logging"
)

// CosineSimilarity returns the cosine similarity of a and b, between -1 and 1:
// 1 means identical direction, 0 orthogonal. A zero-magnitude vector has no
// direction and scores 0.
//
// Every brute-force vector search (prompt atoms, retrieval, the semantic
// classifier, the vector store fallback, northstar docs) calls this once per
// candidate, so it logs nothing on the success path: a per-call debug line
// was one log write per atom per search.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		logging.Get(logging.CategoryEmbedding).Error("CosineSimilarity: vector dimension mismatch: %d != %d", len(a), len(b))
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}
	dot, aa, bb := cosineSums(a, b)
	if aa == 0 || bb == 0 {
		return 0, nil
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb)), nil
}

// cosineSumsScalar is the reference kernel: the dot product and both squared
// magnitudes, accumulated in float64. The vector kernel falls back to it on
// CPUs without AVX2+FMA and uses it for the tail that does not fill a lane.
func cosineSumsScalar(a, b []float32) (dot, aa, bb float64) {
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		aa += x * x
		bb += y * y
	}
	return dot, aa, bb
}
