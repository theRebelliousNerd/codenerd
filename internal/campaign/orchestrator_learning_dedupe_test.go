package campaign

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// campaign-learning-receipt-fidelity #2/#3: the promotion is derived state
// and comes back on every later task's query. Each (kind, value) is recorded
// once, and only a /phase_success promotion is labeled a success pattern.
func TestApplyLearnings_RecordsEachPromotionOnceUnderItsOwnKind(t *testing.T) {
	stub := &promotionStubKernel{facts: []core.Fact{
		{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/phase_success"), "/fix"}},
		{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/edit_pattern"), "/rename"}},
	}}
	o := &Orchestrator{kernel: stub, campaign: &Campaign{ID: "/campaign_learn"}}
	for i := 0; i < 3; i++ {
		o.applyLearnings(context.Background(), &Task{ID: "/t", Description: "task"}, nil)
	}
	if got := len(o.campaign.Learnings); got != 2 {
		t.Fatalf("learnings = %d (%+v), want each promotion once", got, o.campaign.Learnings)
	}
	byKind := map[string]Learning{}
	for _, l := range o.campaign.Learnings {
		byKind[l.Pattern] = l
	}
	if l := byKind["/phase_success"]; l.Type != "/success_pattern" || l.Value != "/fix" {
		t.Fatalf("phase success learning = %+v", l)
	}
	if l := byKind["/edit_pattern"]; l.Type != "/edit_pattern" || l.Value != "/rename" {
		t.Fatalf("an edit-pattern promotion must not be labeled a success pattern: %+v", l)
	}
}
