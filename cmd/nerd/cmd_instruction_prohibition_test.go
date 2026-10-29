package main

import (
	"strings"
	"testing"
)

// Prohibition constraints must not be counted as deliverables.
// A prohibition is satisfied by absence, so requiring lexical evidence of it
// would fail the runs that complied best. This is the live hollow-failure
// mirror described in the fix.

func TestExtractRequestedSubtasks_ProhibitionConstraintIsFiltered(t *testing.T) {
	// Form: "<deliverable A>, and <deliverable B>. Do not change any files."
	// The prohibition must not appear as a subtask; only the two deliverables remain.
	in := "wire the TurnStart audit event and wire the TurnEnd audit event. Do not change any files."
	got := extractRequestedSubtasks(in)
	if len(got) != 2 {
		t.Fatalf("extractRequestedSubtasks(%q) = %v (len %d); want exactly 2 deliverables, prohibition must be filtered", in, got, len(got))
	}
	for _, s := range got {
		if isProhibitionClause(s) {
			t.Errorf("extracted subtask %q is a prohibition clause; should have been filtered", s)
		}
		if strings.Contains(strings.ToLower(s), "do not change") {
			t.Errorf("extracted subtask %q still contains prohibition text", s)
		}
	}
	// Verify both deliverables are present via distinctive tokens.
	joined := strings.ToLower(strings.Join(got, " "))
	if !strings.Contains(joined, "turnstart") {
		t.Errorf("expected TurnStart deliverable to be preserved, got %v", got)
	}
	if !strings.Contains(joined, "turnend") {
		t.Errorf("expected TurnEnd deliverable to be preserved, got %v", got)
	}
}

func TestFindUnaccountedSubtasks_ProhibitionNeedsNoEvidence(t *testing.T) {
	// Live failure: an instruction with two deliverables plus a trailing
	// prohibition. An output that evidences both deliverables and never
	// mentions the prohibition must be considered complete (zero gaps).
	// Against the old code the prohibition was a third subtask requiring
	// evidence, so 1 of 3 was always missing.
	in := "wire the TurnStart audit event and wire the TurnEnd audit event. Do not change any files."
	subtasks := extractRequestedSubtasks(in)
	if len(subtasks) != 2 {
		t.Fatalf("precondition failed: expected 2 subtasks after filtering, got %d (%v)", len(subtasks), subtasks)
	}
	output := "Wired the TurnStart audit event in session/executor.go and the TurnEnd audit event alongside it. Both are emitted on every turn."
	missing := findUnaccountedSubtasks(subtasks, output)
	if len(missing) != 0 {
		t.Fatalf("findUnaccountedSubtasks with prohibition filtered should report no gaps when both deliverables are evidenced, got missing=%v subtasks=%v output=%q", missing, subtasks, output)
	}
}

func TestFindUnaccountedSubtasks_GenuineMissingDeliverableStillReported(t *testing.T) {
	// The prohibition fix must not weaken the guard: a real missing
	// deliverable must still be flagged.
	subtasks := extractRequestedSubtasks("wire the TurnStart audit event and wire the TurnEnd audit event")
	if len(subtasks) < 2 {
		t.Fatalf("precondition failed: expected 2 subtasks, got %d (%v)", len(subtasks), subtasks)
	}
	// Output evidences only TurnStart.
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

func TestExtractRequestedSubtasks_SingleDeliverablePlusProhibitionIsNotCompound(t *testing.T) {
	// After filtering, one real deliverable plus a prohibition falls below the
	// two-subtask threshold and must not be gated at all.
	in := "wire the TurnStart audit event. Do not change any files."
	got := extractRequestedSubtasks(in)
	if len(got) >= 2 {
		t.Errorf("extractRequestedSubtasks(%q) = %v (len %d); want fewer than 2 after filtering — single deliverable plus prohibition is not compound", in, got, len(got))
	}
}
