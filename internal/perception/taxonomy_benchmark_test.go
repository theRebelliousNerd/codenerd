package perception

import (
	"testing"
)

func BenchmarkGetVerbs(b *testing.B) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		b.Fatalf("failed to create engine: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.GetVerbs()
		if err != nil {
			b.Fatalf("GetVerbs failed: %v", err)
		}
	}
}
