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
		va := archsimd.LoadFloat32x8Slice(a[i : i+8])
		vb := archsimd.LoadFloat32x8Slice(b[i : i+8])

		// Multiply
		dotVec := va.Mul(vb)
		aMagVec := va.Mul(va)
		bMagVec := vb.Mul(vb)

		// The experimental SIMD API keeps lanes opaque. Store each product to a
		// fixed array for the horizontal reduction instead of relying on the old
		// public-lane struct representation.
		var dotLanes, aMagLanes, bMagLanes [8]float32
		dotVec.Store(&dotLanes)
		aMagVec.Store(&aMagLanes)
		bMagVec.Store(&bMagLanes)
		for lane := range dotLanes {
			dotProduct += float64(dotLanes[lane])
			aMagnitude += float64(aMagLanes[lane])
			bMagnitude += float64(bMagLanes[lane])
		}
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
