package core

import (
	"testing"

	"codenerd/internal/types"
)

// TestYoloModeSuppressesAmbiguityQuestions proves the kernel half of yolo
// autonomy: with yolo_mode asserted, ambiguity-driven clarification rules
// stop deriving, and retracting it restores them. The policy guards are
// ground negations over an absent-by-default fact, so non-yolo behavior is
// untouched.
func TestYoloModeSuppressesAmbiguityQuestions(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	trigger := Fact{Predicate: "intent_unknown", Args: []any{"do the thing", types.MangleAtom("/heuristic_low")}}
	if err := kernel.Assert(trigger); err != nil {
		t.Fatalf("assert trigger: %v", err)
	}
	questions := func() int {
		t.Helper()
		rows, err := kernel.Query("clarification_question")
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		return len(rows)
	}

	if got := questions(); got != 1 {
		t.Fatalf("without yolo: want 1 clarification question, got %d", got)
	}
	if err := kernel.Assert(Fact{Predicate: "yolo_mode"}); err != nil {
		t.Fatalf("assert yolo_mode: %v", err)
	}
	if got := questions(); got != 0 {
		t.Fatalf("with yolo: want 0 clarification questions, got %d", got)
	}
	if err := kernel.Retract("yolo_mode"); err != nil {
		t.Fatalf("retract yolo_mode: %v", err)
	}
	if got := questions(); got != 1 {
		t.Fatalf("after retract: want 1 clarification question, got %d", got)
	}
}

// TestYoloModePreservesErrorNotices proves yolo never hides infrastructure
// failure: the model-unreachable notice derives with or without yolo_mode,
// because its rule deliberately carries no yolo guard.
func TestYoloModePreservesErrorNotices(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := kernel.Assert(Fact{
		Predicate: "intent_unknown",
		Args:      []any{"do the thing", types.MangleAtom("/llm_unavailable")},
	}); err != nil {
		t.Fatalf("assert trigger: %v", err)
	}
	if err := kernel.Assert(Fact{Predicate: "yolo_mode"}); err != nil {
		t.Fatalf("assert yolo_mode: %v", err)
	}
	rows, err := kernel.Query("clarification_question")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("error notice must survive yolo: want 1 question, got %d", len(rows))
	}
}
