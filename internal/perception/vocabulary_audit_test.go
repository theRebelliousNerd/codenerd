package perception

import (
	"context"
	"slices"
	"testing"

	"codenerd/internal/core"
)

// The routing vocabulary check is derived by the kernel
// (understanding_vocab_miss in perception_routing.mg) from the facts the
// transducer asserts, and surfaced on Routing.VocabularyMisses. It used to be
// a Go validate() that nothing called, so an out-of-vocabulary field handed
// routing back to the LLM's suggestion with no trace.
func TestUnderstand_SurfacesKernelDerivedVocabularyMisses(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	// semantic_type and scope are in the vocabulary; domain and the
	// suggested mode are not.
	envelope := `{"understanding": {"primary_intent": "fix", "semantic_type": "causation",
	  "action_type": "investigate", "domain": "quantum_widgets", "scope": {"level": "file", "target": "x.go"},
	  "confidence": 0.8, "signals": {}, "suggested_approach": {"mode": "hyperdrive", "primary_shard": "coder"}},
	  "surface_response": "Looking."}`
	tr := NewLLMTransducer(&cannedJSONClient{response: envelope}, NewRealKernelRouter(kernel), "system")

	u, err := tr.Understand(context.Background(), "why is x.go failing", nil, nil, nil, "")
	if err != nil {
		t.Fatalf("Understand: %v", err)
	}
	if u.Routing == nil {
		t.Fatal("Routing is nil with kernel wired")
	}
	want := []string{"domain=quantum_widgets", "mode=hyperdrive"}
	if !slices.Equal(u.Routing.VocabularyMisses, want) {
		t.Fatalf("VocabularyMisses = %v, want %v", u.Routing.VocabularyMisses, want)
	}

	// The misses are a kernel derivation, not a Go computation: the relation
	// itself holds them.
	facts, err := kernel.Query("understanding_vocab_miss")
	if err != nil {
		t.Fatalf("query understanding_vocab_miss: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("understanding_vocab_miss has %d facts, want 2: %v", len(facts), facts)
	}
}

func TestUnderstand_InVocabularyUnderstandingHasNoMisses(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	envelope := `{"understanding": {"primary_intent": "fix", "semantic_type": "causation",
	  "action_type": "investigate", "domain": "testing", "scope": {"level": "function", "target": "x.go"},
	  "confidence": 0.8, "signals": {}, "suggested_approach": {"mode": "debug", "primary_shard": "coder"}},
	  "surface_response": "Looking."}`
	tr := NewLLMTransducer(&cannedJSONClient{response: envelope}, NewRealKernelRouter(kernel), "system")

	u, err := tr.Understand(context.Background(), "why does the test fail", nil, nil, nil, "")
	if err != nil {
		t.Fatalf("Understand: %v", err)
	}
	if len(u.Routing.VocabularyMisses) != 0 {
		t.Fatalf("in-vocabulary understanding reported misses: %v", u.Routing.VocabularyMisses)
	}
}
