package core

import "testing"

// Learning ids are the learnings table's integer keys (store.LearningHit).
// Declared /string, every recalled learning was refused by the kernel with a
// type error and the prompt context never saw one (48 refusals in one session,
// 2026-10-27).
func TestLearningRecallResultAcceptsIntegerLearningIDs(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.Assert(Fact{Predicate: "learning_recall_result", Args: []any{int64(1), int64(87), "prefer_table_tests", "Use table-driven tests"}}); err != nil {
		t.Fatalf("assert learning_recall_result: %v", err)
	}
	rows, err := k.Query("learning_recall_result")
	if err != nil || len(rows) != 1 {
		t.Fatalf("learning_recall_result rows=%v err=%v; want the asserted row", rows, err)
	}
}
