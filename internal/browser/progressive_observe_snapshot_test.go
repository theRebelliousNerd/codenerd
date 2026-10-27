package browser

import (
	"testing"

	"codenerd/internal/mangle"
)

func TestProgressiveSnapshotWithoutRetractionWithholdsSameGenerationCandidates(t *testing.T) {
	sink := &testEngineSinkLocal{}
	manager := NewSessionManagerWithSink(DefaultConfig(), sink)
	registry := NewElementRegistry()
	firstFacts := []mangle.Fact{{Predicate: "interactive", Args: []any{"session", "e1_1", "button", "Continue", "click"}}}

	stale, err := manager.replaceProgressiveControlSnapshot("session", registry.Generation(), registry, firstFacts)
	if err != nil || stale {
		t.Fatalf("first observation stale=%v, err=%v; want fresh", stale, err)
	}
	stale, err = manager.replaceProgressiveControlSnapshot("session", registry.Generation(), registry, firstFacts)
	if err != nil || !stale {
		t.Fatalf("same-generation replacement stale=%v, err=%v; want stale without a retractor", stale, err)
	}
	stale, err = manager.replaceProgressiveControlSnapshot("session", registry.Generation(), registry, firstFacts)
	if err != nil || !stale {
		t.Fatalf("later same-generation replacement stale=%v, err=%v; stale must persist for this epoch", stale, err)
	}

	registry.Clear()
	stale, err = manager.replaceProgressiveControlSnapshot("session", registry.Generation(), registry, firstFacts)
	if err != nil || stale {
		t.Fatalf("new generation stale=%v, err=%v; old refs cannot match current-generation refs", stale, err)
	}
	if got := len(sink.findFactsByPredicate("interactive")); got != 4 {
		t.Fatalf("sink stored %d source snapshots, want all four (sink has no retraction)", got)
	}
}
