//go:build amd64 && simd

package embedding

import (
	"fmt"
	"math"
	"simd/archsimd"

	"codenerd/internal/logging"
)

// CosineSimilarity calculates the cosine similarity using AVX2 SIMD operations.
// Returns a value between -1 and 1, where 1 means identical, 0 means orthogonal.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		logging.Get(logging.CategoryEmbedding).Error("CosineSimilarity: vector dimension mismatch: %d != %d", len(a), len(b))
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	logging.EmbeddingDebug("Computing cosine similarity for vectors of dimension %d", len(a))

	var dotProduct, aMagnitude, bMagnitude float64
	i := 0

	// Process 8 float32s at a time (256-bit AVX2)
	for ; i+8 <= len(a); i += 8 {
		va := archsimd.Float32x8{a[i], a[i+1], a[i+2], a[i+3], a[i+4], a[i+5], a[i+6], a[i+7]}
		vb := archsimd.Float32x8{b[i], b[i+1], b[i+2], b[i+3], b[i+4], b[i+5], b[i+6], b[i+7]}

		// Multiply
		dotVec := va.Mul(vb)
		aMagVec := va.Mul(va)
		bMagVec := vb.Mul(vb)

		// Sum the 8 results
		dotProduct += float64(dotVec[0] + dotVec[1] + dotVec[2] + dotVec[3] + dotVec[4] + dotVec[5] + dotVec[6] + dotVec[7])
		aMagnitude += float64(aMagVec[0] + aMagVec[1] + aMagVec[2] + aMagVec[3] + aMagVec[4] + aMagVec[5] + aMagVec[6] + aMagVec[7])
		bMagnitude += float64(bMagVec[0] + bMagVec[1] + bMagVec[2] + bMagVec[3] + bMagVec[4] + bMagVec[5] + bMagVec[6] + bMagVec[7])
	}

	// Handle the remainder
	for ; i < len(a); i++ {
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
