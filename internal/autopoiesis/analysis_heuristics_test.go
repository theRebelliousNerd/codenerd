package autopoiesis

import (
	"context"
	"testing"
)

func TestComplexityAnalyzeHeuristics(t *testing.T) {
	ca := NewComplexityAnalyzer(nil) // heuristic path uses no LLM
	ctx := context.Background()

	if r := ca.Analyze(ctx, "fix a typo in the readme", ""); r.Level != ComplexitySimple || r.NeedsCampaign {
		t.Errorf("simple task misclassified: level=%v needsCampaign=%v", r.Level, r.NeedsCampaign)
	}
	epic := ca.Analyze(ctx, "implement a complete feature for billing", "")
	if epic.Level != ComplexityEpic || !epic.NeedsCampaign || epic.Score < 0.9 {
		t.Errorf("epic task misclassified: %+v", epic)
	}
	mod := ca.Analyze(ctx, "add a new component to the dashboard", "")
	if mod.Level != ComplexityModerate {
		t.Errorf("moderate task misclassified: level=%v", mod.Level)
	}
}

func TestPersistenceAnalyzeHeuristics(t *testing.T) {
	pa := NewPersistenceAnalyzer(nil)
	ctx := context.Background()

	if r := pa.Analyze(ctx, "fix a typo"); r.NeedsPersistent {
		t.Errorf("simple task should not need a persistent agent: %+v", r)
	}
	learn := pa.Analyze(ctx, "remember my preference for tabs over spaces")
	if !learn.NeedsPersistent || len(learn.Needs) == 0 {
		t.Errorf("learning task should need a persistent agent: %+v", learn)
	}
}

func TestShouldTriggerCampaign(t *testing.T) {
	o := &Orchestrator{complexity: NewComplexityAnalyzer(nil)}
	ctx := context.Background()

	ok, reason := o.ShouldTriggerCampaign(ctx, "implement a complete feature for billing", "")
	if !ok || reason == "" {
		t.Errorf("epic task should trigger a campaign with a reason, got (%v,%q)", ok, reason)
	}
	if ok, reason := o.ShouldTriggerCampaign(ctx, "fix a typo", ""); ok || reason != "" {
		t.Errorf("simple task should not trigger a campaign, got (%v,%q)", ok, reason)
	}
}

func TestShouldCreatePersistentAgent(t *testing.T) {
	o := &Orchestrator{persistence: NewPersistenceAnalyzer(nil)}
	ctx := context.Background()

	ok, need := o.ShouldCreatePersistentAgent(ctx, "remember my preference for tabs over spaces")
	if !ok || need == nil {
		t.Errorf("learning task should create a persistent agent, got (%v,%v)", ok, need)
	}
	if ok, need := o.ShouldCreatePersistentAgent(ctx, "fix a typo"); ok || need != nil {
		t.Errorf("simple task should not create a persistent agent, got (%v,%v)", ok, need)
	}
}
