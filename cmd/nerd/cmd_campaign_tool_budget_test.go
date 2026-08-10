package main

import (
	"testing"

	"codenerd/internal/config"
)

func TestApplyCampaignExecutorBudget_PropagatesAdaptivePolicy(t *testing.T) {
	disabled := false
	appCfg := &config.UserConfig{CoreLimits: &config.CoreLimits{
		MaxToolCalls:               31,
		MaxToolIterations:          12,
		AdaptiveToolBudget:         &disabled,
		ToolIterationExtensionSize: 5,
		MaxToolIterationExtensions: 3,
		ToolLoopRepeatThreshold:    4,
	}}

	got := applyCampaignExecutorBudget(appCfg, t.TempDir(), nil, nil)
	if got.MaxToolCalls != 31 || got.MaxToolIterations != 12 {
		t.Fatalf("base budget = %d/%d, want 31/12", got.MaxToolCalls, got.MaxToolIterations)
	}
	if got.AdaptiveToolBudget {
		t.Fatal("campaign lost explicit adaptive disable")
	}
	if got.ToolIterationExtensionSize != 5 || got.MaxToolIterationExtensions != 3 || got.ToolLoopRepeatThreshold != 4 {
		t.Fatalf("adaptive policy drifted: %#v", got)
	}
}
