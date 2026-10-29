package main

import (
	"strings"
	"testing"
)

// TestExtractRequestedSubtasks_JITLiveContextFiltered pins the live failure
// where a context sentence describing where to look was counted as a
// deliverable. The three-sentence instruction contains two imperative
// deliverables and one declarative context sentence. After filtering, only
// imperatives remain.
func TestExtractRequestedSubtasks_JITLiveContextFiltered(t *testing.T) {
	input := "Profile the JIT prompt compiler's token spend. The compilation_complete log lines in .nerd/logs/*_jit.log record per-category token counts for every compiled prompt. Determine which prompt atom categories consume the most tokens on a coder turn, then judge which three categories deliver the least value for their cost and justify each judgement with evidence from the atoms themselves rather than from their size alone."
	got := extractRequestedSubtasks(input)
	if len(got) == 0 {
		t.Fatalf("extractRequestedSubtasks(%q) = %v; want at least 2 imperatives after filtering", input, got)
	}
	joined := strings.ToLower(strings.Join(got, " "))
	if strings.Contains(joined, "compilation_complete") {
		t.Errorf("extracted subtasks %v still contain declarative context 'compilation_complete'; should have been filtered", got)
	}
	if strings.Contains(joined, "record per-category") {
		t.Errorf("extracted subtasks %v still contain declarative context; should have been filtered", got)
	}
	if !strings.Contains(joined, "profile") {
		t.Errorf("expected imperative 'Profile' to be preserved, got %v", got)
	}
	if !strings.Contains(joined, "determine") {
		t.Errorf("expected imperative 'Determine' to be preserved, got %v", got)
	}
	// The context sentence must be gone, so every remaining clause must start
	// with an imperative, not a declarative starter like "the".
	for _, s := range got {
		if isDeclarativeContextClause(s) {
			t.Errorf("extracted subtask %q is declarative context; should have been filtered", s)
		}
	}
	// After filtering the single declarative, the remaining imperatives are
	// Profile, Determine, judge, justify — four, not three. The key is that the
	// context sentence is not among them.
	if len(got) != 4 {
		t.Errorf("extractRequestedSubtasks(context) = %v (len %d); want 4 imperatives (Profile, Determine, judge, justify) after filtering context", got, len(got))
	}
}

// TestFindUnaccountedSubtasks_DeclarativeFilterDoesNotWeakenGuard ensures the
// declarative filter does not hide a genuinely missing imperative deliverable.
func TestFindUnaccountedSubtasks_DeclarativeFilterDoesNotWeakenGuard(t *testing.T) {
	subtasks := extractRequestedSubtasks("wire the TurnStart audit event and wire the TurnEnd audit event")
	if len(subtasks) < 2 {
		t.Fatalf("precondition failed: expected 2 subtasks, got %d (%v)", len(subtasks), subtasks)
	}
	// Output evidences only the first.
	output := "Wired the TurnStart audit event; all good."
	missing := findUnaccountedSubtasks(subtasks, output)
	if len(missing) == 0 {
		t.Fatalf("expected a gap when one deliverable is not evidenced, subtasks=%v output=%q", subtasks, output)
	}
	joined := strings.ToLower(strings.Join(missing, " "))
	if !strings.Contains(joined, "turnend") {
		t.Errorf("expected TurnEnd subtask to be flagged as missing, got %v", missing)
	}
	if strings.Contains(joined, "turnstart") {
		t.Errorf("TurnStart was evidenced but was flagged as missing: %v", missing)
	}
}

// TestExtractRequestedSubtasks_LeadingAdverbIsStillDeliverable checks that a
// clause beginning with a leading adverb like "then" is not mistaken for
// declarative context.
func TestExtractRequestedSubtasks_LeadingAdverbIsStillDeliverable(t *testing.T) {
	input := "wire the TurnStart audit event and then determine the cost"
	got := extractRequestedSubtasks(input)
	if len(got) < 2 {
		t.Fatalf("extractRequestedSubtasks(%q) = %v (len %d); want at least 2 — leading adverb should not cause filtering", input, got, len(got))
	}
	joined := strings.ToLower(strings.Join(got, " "))
	if !strings.Contains(joined, "turnstart") {
		t.Errorf("expected TurnStart deliverable, got %v", got)
	}
	if !strings.Contains(joined, "determine") {
		t.Errorf("expected 'determine the cost' to be preserved as deliverable despite leading 'then', got %v", got)
	}
	for _, s := range got {
		if isDeclarativeContextClause(s) {
			t.Errorf("subtask %q incorrectly classified as declarative", s)
		}
	}
}

// TestExtractRequestedSubtasks_SingleImperativePlusTwoContextNotCompound verifies
// that an instruction with one real deliverable plus two context sentences falls
// below the two-subtask threshold and is therefore not gated.
func TestExtractRequestedSubtasks_SingleImperativePlusTwoContextNotCompound(t *testing.T) {
	input := "Profile the JIT prompt compiler's token spend. The compilation_complete log lines are in .nerd/logs. There are three categories to consider."
	got := extractRequestedSubtasks(input)
	if len(got) >= 2 {
		t.Errorf("extractRequestedSubtasks(%q) = %v (len %d); want fewer than 2 after filtering — single imperative plus two contexts is not compound", input, got, len(got))
	}
}
