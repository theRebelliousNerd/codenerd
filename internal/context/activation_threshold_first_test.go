package context

import (
	"reflect"
	"testing"

	"codenerd/internal/core"
)

func thresholdFixture(n int) ([]core.Fact, *core.Fact) {
	facts := namedEntityFacts(n)
	facts = append(facts,
		core.Fact{Predicate: "user_intent", Args: []any{"/current_intent", "/mutation", "/fix", "internal/pkg001/file00001.go", "none"}},
		core.Fact{Predicate: "focus_resolution", Args: []any{"the kernel file", "internal/pkg002/file00002.go", "", int64(90)}},
		core.Fact{Predicate: "test_state", Args: []any{"/failing"}},
		core.Fact{Predicate: "test_state", Args: []any{"/failing"}}, // an equal fact: ties keep input order
	)
	intent := facts[len(facts)-4]
	return facts, &intent
}

// What clears the threshold, and its order, is what sorting every scored fact
// and filtering afterwards gave.
func TestApplyIntentActivation_MatchesSortThenFilter(t *testing.T) {
	facts, intent := thresholdFixture(8000)

	reference := NewActivationEngine(DefaultConfig())
	reference.mu.Lock()
	reference.updateFocusedPathsLocked(facts)
	want := reference.FilterByThreshold(reference.scoreFactsLocked(facts, intent))
	reference.mu.Unlock()

	got := NewActivationEngine(DefaultConfig()).ApplyIntentActivation(facts, intent)

	if len(want) == 0 || len(want) == len(facts) {
		t.Fatalf("the fixture proves nothing: %d of %d facts clear the threshold", len(want), len(facts))
	}
	if len(got) != len(want) {
		t.Fatalf("%d facts selected, sort-then-filter selects %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i].Fact, want[i].Fact) || got[i].Score != want[i].Score {
			t.Fatalf("position %d: got %s (%.1f), want %s (%.1f)", i, got[i].Fact.String(), got[i].Score, want[i].Fact.String(), want[i].Score)
		}
	}
}

func BenchmarkApplyIntentActivation(b *testing.B) {
	facts, intent := thresholdFixture(300000)
	b.Run("sort-then-filter", func(b *testing.B) {
		b.ReportAllocs()
		ae := NewActivationEngine(DefaultConfig())
		for b.Loop() {
			ae.mu.Lock()
			ae.updateFocusedPathsLocked(facts)
			_ = ae.FilterByThreshold(ae.scoreFactsLocked(facts, intent))
			ae.mu.Unlock()
		}
	})
	b.Run("filter-then-sort", func(b *testing.B) {
		b.ReportAllocs()
		ae := NewActivationEngine(DefaultConfig())
		for b.Loop() {
			_ = ae.ApplyIntentActivation(facts, intent)
		}
	})
}
