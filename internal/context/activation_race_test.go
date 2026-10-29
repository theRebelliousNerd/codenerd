package context

import (
	"sync"
	"testing"

	"codenerd/internal/core"
)

// TestActivationEngine_ConcurrentScoreNoRace reproduces the production crash
// (concurrent map read/write on reverseDependencies) under the race detector.
func TestActivationEngine_ConcurrentScoreNoRace(t *testing.T) {
	ae := NewActivationEngine(DefaultConfig())
	facts := make([]core.Fact, 0, 200)
	for i := 0; i < 100; i++ {
		facts = append(facts, core.Fact{
			Predicate: "dependency_link",
			Args:      []any{"pkgA", "pkgB"},
		})
		facts = append(facts, core.Fact{
			Predicate: "symbol_graph",
			Args:      []any{"Sym", "func", "pub", "file.go", "sig"},
		})
	}
	intent := &core.Fact{Predicate: "user_intent", Args: []any{"/query", "x", "/review"}}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = ae.ScoreFacts(facts, intent)
				_ = ae.GetHighActivationFacts(facts, intent, 8000)
				ae.AddDependency(
					core.Fact{Predicate: "a", Args: []any{"1"}},
					core.Fact{Predicate: "b", Args: []any{"2"}},
				)
			}
		}()
	}
	wg.Wait()
}
