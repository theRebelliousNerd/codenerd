package core

import "testing"

// review_rejection_rate_high was an input "computed by Go" that nothing
// computed, so the high_rejection_rate suspicion of reviewer.mg could never
// fire. It is derived from the accuracy row the feedback loop keeps
// (review_accuracy: Total, Accepted, Rejected, Score): more than half rejected.
// The suspicion it feeds is what CheckReviewNeedsValidation reads.
func TestReviewRejectionRateIsJudgedFromTheAccuracyRow(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	rows := []Fact{
		{Predicate: "review_accuracy", Args: []any{"mostly_rejected", int64(4), int64(1), int64(3), int64(25)}},
		{Predicate: "review_accuracy", Args: []any{"half_rejected", int64(4), int64(2), int64(2), int64(50)}},
		{Predicate: "review_accuracy", Args: []any{"mostly_accepted", int64(4), int64(3), int64(1), int64(75)}},
		{Predicate: "review_accuracy", Args: []any{"tiny", int64(2), int64(0), int64(2), int64(0)}},
	}
	if err := k.AssertBatch(rows); err != nil {
		t.Fatal(err)
	}
	high := map[string]bool{}
	got, err := k.Query("review_rejection_rate_high")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		high[f.Args[0].(string)] = true
	}
	if !high["mostly_rejected"] || !high["tiny"] {
		t.Errorf("more than half rejected is a high rate: %v", high)
	}
	if high["half_rejected"] || high["mostly_accepted"] {
		t.Errorf("exactly half, or mostly accepted, is not: %v", high)
	}

	// The suspicion needs more than two judged findings, so a tiny review is
	// high-rate but not suspect.
	suspect, err := k.Query("review_suspect")
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]bool{}
	for _, f := range suspect {
		if len(f.Args) >= 2 && f.Args[1] == "high_rejection_rate" {
			flagged[f.Args[0].(string)] = true
		}
	}
	if !flagged["mostly_rejected"] || flagged["tiny"] || flagged["mostly_accepted"] {
		t.Errorf("suspect reviews = %v, want only mostly_rejected", flagged)
	}
}
