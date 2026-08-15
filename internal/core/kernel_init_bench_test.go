package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"testing"
)

// Benchmark the performance of loadMangleFiles function
func BenchmarkLoadMangleFiles(b *testing.B) {
	for i := 0; i < b.N; i++ {
		k := &RealKernel{
			facts:             make([]Fact, 0),
			cachedAtoms:       make([]ast.Atom, 0),
			factIndex:         make(map[string]struct{}),
			bootFacts:         make([]Fact, 0),
			bootIntents:       make([]HybridIntent, 0),
			bootPrompts:       make([]HybridPrompt, 0),
			store:             factstore.NewSimpleInMemoryStore(),
			loadedPolicyFiles: make(map[string]struct{}),
			policyDirty:       true,
		}
		_ = k.loadMangleFiles()
	}
}
