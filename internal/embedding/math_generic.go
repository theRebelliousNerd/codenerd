//go:build !amd64 || !simd

package embedding

import (
	"fmt"
	"math"

	"codenerd/internal/logging"
)

// CosineSimilarity calculates the cosine similarity between two vectors using a generic pure-Go loop.
// Returns a value between -1 and 1, where 1 means identical, 0 means orthogonal.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		logging.Get(logging.CategoryEmbedding).Error("CosineSimilarity: vector dimension mismatch: %d != %d", len(a), len(b))
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	logging.EmbeddingDebug("Computing cosine similarity for vectors of dimension %d", len(a))

	var dotProduct, aMagnitude, bMagnitude float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		aMagnitude += float64(a[i] * a[i])
		bMagnitude += float64(b[i] * b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		logging.Get(logging.CategoryEmbedding).Warn("CosineSimilarity: zero magnitude vector detected")
		return 0, nil
	}

	result := dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude))
	logging.EmbeddingDebug("CosineSimilarity result: %.6f", result)
	return result, nil
}
