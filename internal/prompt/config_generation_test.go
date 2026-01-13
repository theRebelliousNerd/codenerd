package prompt

import (
	"context"
	"testing"
)

func TestConfigGeneration_StandardIntents(t *testing.T) {
	registry := NewSimpleRegistry()
	RegisterDefaultConfigAtoms(registry)
	factory := NewConfigFactory(registry)

	ctx := context.Background()
	result := &CompilationResult{Prompt: "Test Prompt"}

	// Test Coder
	coderCfg, err := factory.Generate(ctx, result, "/coder")
	if err != nil {
		t.Fatalf("Failed to generate coder config: %v", err)
	}
	if len(coderCfg.Tools.AllowedTools) == 0 {
		t.Errorf("Coder config has no tools")
	}
	expectedCoderPolicies := []string{
		"base.mg",
		"policy/coder_classification.mg",
		"policy/coder_language.mg",
		"policy/coder_impact.mg",
		"policy/coder_safety.mg",
		"policy/coder_diagnostics.mg",
		"policy/coder_workflow.mg",
		"policy/coder_context.mg",
		"policy/coder_tdd.mg",
		"policy/coder_quality.mg",
		"policy/coder_learning.mg",
		"policy/coder_campaign.mg",
		"policy/coder_observability.mg",
		"policy/coder_patterns.mg",
	}
	assertContainsAll(t, coderCfg.Policies.Files, expectedCoderPolicies, "Coder")

	// Test Tester
	testerCfg, err := factory.Generate(ctx, result, "/tester")
	if err != nil {
		t.Fatalf("Failed to generate tester config: %v", err)
	}
	assertContainsAll(t, testerCfg.Policies.Files, []string{"base.mg", "tester.mg"}, "Tester")

	// Test Reviewer
	reviewerCfg, err := factory.Generate(ctx, result, "/reviewer")
	if err != nil {
		t.Fatalf("Failed to generate reviewer config: %v", err)
	}
	assertContainsAll(t, reviewerCfg.Policies.Files, []string{"base.mg", "reviewer.mg"}, "Reviewer")
}

func TestConfigGeneration_HybridIntents(t *testing.T) {
	registry := NewSimpleRegistry()
	RegisterDefaultConfigAtoms(registry)
	factory := NewConfigFactory(registry)

	ctx := context.Background()
	result := &CompilationResult{Prompt: "Test Prompt"}

	// Hybrid: /fix (coder) + /test (tester)
	hybridCfg, err := factory.Generate(ctx, result, "/fix", "/test")
	if err != nil {
		t.Fatalf("Failed to generate hybrid config: %v", err)
	}

	// Should have both policies
	assertContainsAll(t, hybridCfg.Policies.Files, []string{
		"base.mg",
		"policy/coder_workflow.mg",
		"tester.mg",
	}, "Hybrid")

	// Should have union of tools
	if !contains(hybridCfg.Tools.AllowedTools, "write_file") || !contains(hybridCfg.Tools.AllowedTools, "run_shell_command") {
		t.Errorf("Hybrid config missing tools: %v", hybridCfg.Tools.AllowedTools)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func assertContainsAll(t *testing.T, got []string, expected []string, label string) {
	t.Helper()
	for _, item := range expected {
		if !contains(got, item) {
			t.Errorf("%s config missing %s", label, item)
		}
	}
}
