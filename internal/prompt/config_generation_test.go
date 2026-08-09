package prompt

import (
	"context"
	"slices"
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
	if len(coderCfg.AllowedTools) == 0 {
		t.Errorf("Coder config has no tools")
	}
	expectedCoderPolicies := []string{
		"policy/constitution.mg",
		"policy/validation.mg",
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
	assertContainsAll(t, coderCfg.Policies, expectedCoderPolicies, "Coder")

	// Test Tester
	testerCfg, err := factory.Generate(ctx, result, "/tester")
	if err != nil {
		t.Fatalf("Failed to generate tester config: %v", err)
	}
	assertContainsAll(t, testerCfg.Policies, []string{"policy/constitution.mg", "policy/validation.mg", "tester.mg"}, "Tester")

	// Test Reviewer
	reviewerCfg, err := factory.Generate(ctx, result, "/reviewer")
	if err != nil {
		t.Fatalf("Failed to generate reviewer config: %v", err)
	}
	assertContainsAll(t, reviewerCfg.Policies, []string{"policy/constitution.mg", "policy/validation.mg", "reviewer.mg"}, "Reviewer")

	// Researcher progressive browser loop.
	researchCfg, err := factory.Generate(ctx, result, "/researcher")
	if err != nil {
		t.Fatalf("Failed to generate researcher config: %v", err)
	}
	assertContainsAll(t, researchCfg.AllowedTools, []string{"browser_observe", "browser_act", "browser_mangle", "browser_wait", "browser_reason"}, "Researcher")
}

func TestDefaultConfigAtomProvider_ProgressiveBrowserToolsReachResearchAndVerify(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	for _, intent := range []string{"/research", "/explore", "/verify", "/validate"} {
		atom, ok := provider.GetAtom(intent)
		if !ok {
			t.Fatalf("missing config atom for %s", intent)
		}
		assertContainsAll(t, atom.Tools, []string{"browser_observe", "browser_act", "browser_mangle", "browser_wait", "browser_reason"}, intent)
	}
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
	assertContainsAll(t, hybridCfg.Policies, []string{
		"policy/constitution.mg",
		"policy/validation.mg",
		"policy/coder_workflow.mg",
		"tester.mg",
	}, "Hybrid")

	// Should have union of tools
	if !contains(hybridCfg.AllowedTools, "write_file") || !contains(hybridCfg.AllowedTools, "run_shell_command") {
		t.Errorf("Hybrid config missing tools: %v", hybridCfg.AllowedTools)
	}
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func assertContainsAll(t *testing.T, got []string, expected []string, label string) {
	t.Helper()
	for _, item := range expected {
		if !contains(got, item) {
			t.Errorf("%s config missing %s", label, item)
		}
	}
}
