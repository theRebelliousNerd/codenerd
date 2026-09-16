package core

import (
	"testing"
)

// TestIntentCorpus_BootFactsQueryable pins the loader-to-query path end to
// end: the 14 embedded intent modules must land in bootFacts at boot and be
// queryable after Evaluate. Unit tests cover the predicate set; this covers
// the behavior (files present, parseable, admitted by the allow-list).
func TestIntentCorpus_BootFactsQueryable(t *testing.T) {
	k := setupMockKernel(t)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, pred := range []string{"intent_definition", "valid_action_type", "valid_semantic_type", "intent_category"} {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatalf("Query(%s): %v", pred, err)
		}
		if len(rows) == 0 {
			t.Fatalf("Query(%s) returned 0 rows, want boot corpus facts", pred)
		}
	}
}
