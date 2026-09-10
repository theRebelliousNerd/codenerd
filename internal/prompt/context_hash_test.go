package prompt

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompilationContext_Hash(t *testing.T) {
	t.Run("nil context returns nil string", func(t *testing.T) {
		var cc *CompilationContext
		assert.Equal(t, "nil", cc.Hash())
	})

	t.Run("same context produces same hash", func(t *testing.T) {
		cc1 := &CompilationContext{
			OperationalMode: "/active",
			Language:        "/go",
			TokenBudget:     1000,
		}
		cc2 := &CompilationContext{
			OperationalMode: "/active",
			Language:        "/go",
			TokenBudget:     1000,
		}
		assert.Equal(t, cc1.Hash(), cc2.Hash())
	})

	t.Run("different contexts produce different hashes", func(t *testing.T) {
		cc1 := &CompilationContext{
			OperationalMode: "/active",
		}
		cc2 := &CompilationContext{
			OperationalMode: "/passive",
		}
		assert.NotEqual(t, cc1.Hash(), cc2.Hash())
	})

	t.Run("retry and rendered capability fields change identity", func(t *testing.T) {
		base := NewCompilationContext()
		base.AvailableTools = []string{"read_file"}

		retry := base.Clone()
		retry.PreviousAttemptNoToolCall = true
		assert.NotEqual(t, base.Hash(), retry.Hash())

		withWrite := base.Clone()
		withWrite.AvailableTools = []string{"read_file", "write_file"}
		assert.NotEqual(t, base.Hash(), withWrite.Hash())

		withSpecialist := base.Clone()
		withSpecialist.AvailableSpecialists = "- security-auditor"
		assert.NotEqual(t, base.Hash(), withSpecialist.Hash())
	})

	t.Run("set-like fields are canonical", func(t *testing.T) {
		left := NewCompilationContext()
		left.Frameworks = []string{"/gin", "/bubbletea", "/gin"}
		left.AvailableTools = []string{"write_file", "read_file", "read_file"}

		right := NewCompilationContext()
		right.Frameworks = []string{"/bubbletea", "/gin"}
		right.AvailableTools = []string{"read_file", "write_file"}

		assert.Equal(t, left.Hash(), right.Hash())
		assert.Equal(t, []string{"/gin", "/bubbletea", "/gin"}, left.Frameworks)
		assert.Equal(t, []string{"write_file", "read_file", "read_file"}, left.AvailableTools)
	})

	t.Run("budget and search fields change identity", func(t *testing.T) {
		base := NewCompilationContext()

		reserved := base.Clone()
		reserved.ReservedTokens++
		assert.NotEqual(t, base.Hash(), reserved.Hash())

		topK := base.Clone()
		topK.SemanticTopK++
		assert.NotEqual(t, base.Hash(), topK.Hash())

		newFiles := base.Clone()
		newFiles.HasNewFiles = true
		assert.NotEqual(t, base.Hash(), newFiles.Hash())

		highChurn := base.Clone()
		highChurn.IsHighChurn = true
		assert.NotEqual(t, base.Hash(), highChurn.Hash())
	})
}

// ActivatedFacts is documented as a per-turn selection input and is not yet
// populated by production. Two hazards used to sit behind wiring it up, and
// both fail silently, so they are pinned here rather than left to be
// rediscovered by whoever populates the field.

func TestHashIncludesActivatedFacts(t *testing.T) {
	base := NewCompilationContext()
	base.IntentVerb = "/fix"

	hot := base.Clone()
	hot.ActivatedFacts = map[string]float64{"fix_applied(/auth.go)": 0.9}

	cold := base.Clone()
	cold.ActivatedFacts = map[string]float64{"test_state(/failing)": 0.9}

	if hot.Hash() == cold.Hash() {
		t.Error("two contexts with entirely different activated facts hash identically; " +
			"the second turn would be served the first's compiled prompt, with no error")
	}
	if hot.Hash() == base.Hash() {
		t.Error("adding activated facts did not change cache identity")
	}
}

func TestHashIsStableAcrossMapIterationOrder(t *testing.T) {
	// Go randomizes map iteration. An unsorted hash would make the same context
	// hash differently between two calls, which is a cache that never hits
	// rather than one that hits wrongly — quieter, and just as broken.
	facts := map[string]float64{
		"a": 0.1, "b": 0.2, "c": 0.3, "d": 0.4, "e": 0.5,
		"f": 0.6, "g": 0.7, "h": 0.8, "i": 0.9, "j": 1.0,
	}

	first := NewCompilationContext()
	first.ActivatedFacts = facts

	want := first.Hash()
	for i := 0; i < 50; i++ {
		if got := first.Hash(); got != want {
			t.Fatalf("Hash() is not stable across calls: %s != %s", got, want)
		}
	}

	// A separately-built map with the same contents must agree.
	other := NewCompilationContext()
	other.ActivatedFacts = make(map[string]float64, len(facts))
	for k, v := range facts {
		other.ActivatedFacts[k] = v
	}
	if other.Hash() != want {
		t.Error("two contexts with equal activated facts hashed differently")
	}
}

func TestHashDistinguishesActivationScores(t *testing.T) {
	// Same fact, different heat, is a different selection input.
	a := NewCompilationContext()
	a.ActivatedFacts = map[string]float64{"modified(/parser.go)": 0.2}

	b := NewCompilationContext()
	b.ActivatedFacts = map[string]float64{"modified(/parser.go)": 0.95}

	if a.Hash() == b.Hash() {
		t.Error("activation scores are absent from cache identity")
	}
}

func TestCloneDeepCopiesActivatedFacts(t *testing.T) {
	original := NewCompilationContext()
	original.ActivatedFacts = map[string]float64{"shared": 1.0}

	clone := original.Clone()
	clone.ActivatedFacts["shared"] = 0.0
	clone.ActivatedFacts["added-by-clone"] = 0.5

	if got := original.ActivatedFacts["shared"]; got != 1.0 {
		t.Errorf("mutating the clone changed the original: shared = %v, want 1.0", got)
	}
	if _, ok := original.ActivatedFacts["added-by-clone"]; ok {
		t.Error("the clone and the original share one map; compilation runs under " +
			"singleflight with an errgroup, so that is a data race")
	}
}

func TestCloneHandlesNilActivatedFacts(t *testing.T) {
	original := NewCompilationContext()
	if clone := original.Clone(); clone.ActivatedFacts != nil {
		t.Errorf("Clone() invented a map for a nil field: %v", clone.ActivatedFacts)
	}
}

func TestConcurrentCloneAndHashAreRaceFree(t *testing.T) {
	// Run under -race: this is the shape the singleflight compile path takes.
	original := NewCompilationContext()
	original.ActivatedFacts = map[string]float64{"a": 0.1, "b": 0.2, "c": 0.3}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c := original.Clone()
			c.ActivatedFacts[fmt.Sprintf("worker-%d", n)] = float64(n)
			_ = c.Hash()
			_ = original.Hash()
		}(i)
	}
	wg.Wait()

	if len(original.ActivatedFacts) != 3 {
		t.Errorf("concurrent clones wrote through to the original: %d facts, want 3",
			len(original.ActivatedFacts))
	}
}
