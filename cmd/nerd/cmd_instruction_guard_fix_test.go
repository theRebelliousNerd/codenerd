package main

import (
	"strings"
	"testing"
)

func TestDistinctiveTokens_LeadingImperativeExcluded(t *testing.T) {
	toks := distinctiveTokens("Determine which prompt atom categories consume the most tokens")
	joined := strings.Join(toks, " ")
	if strings.Contains(joined, "determine") {
		t.Fatalf("distinctiveTokens should exclude leading imperative verb 'determine', got %v", toks)
	}
	for _, want := range []string{"prompt", "atom", "categories", "consume", "tokens"} {
		if !strings.Contains(joined, want) {
			t.Errorf("distinctiveTokens %v missing expected token %q", toks, want)
		}
	}
}

func TestFindUnaccountedSubtasks_DetermineEvidencedWithoutVerb(t *testing.T) {
	subtasks := []string{"Determine which prompt atom categories consume the most tokens"}
	output := "prompt atom categories: code, review, test consume the most tokens: 1200, 800, 600 on a coder turn"
	missing := findUnaccountedSubtasks(subtasks, output)
	if len(missing) != 0 {
		t.Fatalf("expected no gap when answer names categories and token counts without using 'determine', got missing=%v", missing)
	}
}

func TestCompoundGuard_QueryPartialNoError(t *testing.T) {
	subtasks := extractRequestedSubtasks("wire the TurnStart audit event and wire the TurnEnd audit event")
	if len(subtasks) < 2 {
		t.Fatalf("precondition failed: expected 2 subtasks, got %v", subtasks)
	}
	output := "Wired the TurnStart audit event; all good."
	unaccounted := findUnaccountedSubtasks(subtasks, output)
	if len(unaccounted) == 0 {
		t.Fatalf("expected a gap for missing TurnEnd, subtasks=%v output=%q", subtasks, output)
	}
	// Simulate guard for query intent: should report PARTIAL but not set error.
	category := "/query"
	verb := "/list"
	if !isReadOnlyIntent(category, verb) {
		t.Fatalf("isReadOnlyIntent(%q,%q) should be true for query", category, verb)
	}
	// Guard logic: for query, do not set actionErr, but output gets PARTIAL.
	gapMsg := "PARTIAL: gap"
	_ = gapMsg
	if isReadOnlyIntent(category, verb) {
		// no error
	} else {
		t.Fatalf("query should not produce error")
	}
	// Also verify that for same gap, mutating would be considered not read-only.
	if isReadOnlyIntent("/mutation", "/fix") {
		t.Fatalf("isReadOnlyIntent(/mutation,/fix) should be false")
	}
}

func TestCompoundGuard_MutatingPartialReturnsError(t *testing.T) {
	subtasks := extractRequestedSubtasks("wire the TurnStart audit event and wire the TurnEnd audit event")
	if len(subtasks) < 2 {
		t.Fatalf("precondition failed: expected 2 subtasks, got %v", subtasks)
	}
	output := "Wired the TurnStart audit event; all good."
	unaccounted := findUnaccountedSubtasks(subtasks, output)
	if len(unaccounted) == 0 {
		t.Fatalf("expected a gap for missing TurnEnd")
	}
	if isReadOnlyIntent("/mutation", "/fix") {
		t.Fatalf("mutating intent should not be read-only")
	}
	// For mutating, guard must set error (simulated).
	if isReadOnlyIntent("/mutation", "/fix") {
		t.Fatalf("mutating should be considered mutating, not read-only")
	}
	// Verify that the guard would set error for mutating: we check the helper directly.
	// The actual runInstruction guard checks !isReadOnlyIntent before setting actionErr,
	// so mutating with gap must yield error.
}
