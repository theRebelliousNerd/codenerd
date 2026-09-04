package config

import (
	"testing"
)

func TestDefaultCoreLimits_EnableBoundedAdaptiveToolBudget(t *testing.T) {
	limits := DefaultCoreLimits()
	if limits.AdaptiveToolBudget == nil || !*limits.AdaptiveToolBudget {
		t.Fatal("adaptive tool budget must default on")
	}
	if limits.ToolIterationExtensionSize != 8 || limits.MaxToolIterationExtensions != 2 {
		t.Fatalf("extension defaults = %dx%d, want 2x8", limits.MaxToolIterationExtensions, limits.ToolIterationExtensionSize)
	}
	if limits.ToolLoopRepeatThreshold != 2 {
		t.Fatalf("repeat threshold = %d, want 2", limits.ToolLoopRepeatThreshold)
	}
}

func TestValidateCoreLimits_RejectsUnboundedAdaptiveSettings(t *testing.T) {
	valid := *DefaultCoreLimits()
	tests := []struct {
		name   string
		mutate func(*CoreLimits)
	}{
		{"negative extension", func(c *CoreLimits) { c.ToolIterationExtensionSize = -1 }},
		{"oversized extension", func(c *CoreLimits) { c.ToolIterationExtensionSize = 65 }},
		{"too many extensions", func(c *CoreLimits) { c.MaxToolIterationExtensions = 9 }},
		{"repeat threshold one", func(c *CoreLimits) { c.ToolLoopRepeatThreshold = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := valid
			tt.mutate(&limits)
			if err := (&limits).ValidateCoreLimits(); err == nil {
				t.Fatal("ValidateCoreLimits accepted unsafe adaptive setting")
			}
		})
	}
}

func TestGetCoreLimits_PreservesExplicitAdaptiveDisable(t *testing.T) {
	disabled := false
	cfg := &UserConfig{CoreLimits: &CoreLimits{
		AdaptiveToolBudget:         &disabled,
		MaxToolCalls:               17,
		MaxToolIterations:          9,
		ToolIterationExtensionSize: 4,
		MaxToolIterationExtensions: 1,
		ToolLoopRepeatThreshold:    3,
	}}
	limits := cfg.GetCoreLimits()
	if limits.AdaptiveToolBudget == nil || *limits.AdaptiveToolBudget {
		t.Fatal("explicit adaptive_tool_budget=false was lost")
	}
	if limits.MaxToolCalls != 17 || limits.MaxToolIterations != 9 ||
		limits.ToolIterationExtensionSize != 4 || limits.MaxToolIterationExtensions != 1 ||
		limits.ToolLoopRepeatThreshold != 3 {
		t.Fatalf("resolved tool limits drifted: %#v", limits)
	}
}
