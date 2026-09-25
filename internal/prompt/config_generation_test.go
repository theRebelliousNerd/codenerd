package prompt

import (
	"context"
	"slices"
	"testing"
)

// The production provider (NewDefaultConfigAtomProvider, the one system boot
// hands to NewConfigFactory) is the single authority for intent -> tools and
// policies. These assertions used to run against a second, hand-maintained
// SimpleRegistry catalog that no production path consulted and that had
// drifted (it still granted run_shell_command); they now pin the live one.
func TestConfigGeneration_StandardIntents(t *testing.T) {
	factory := NewConfigFactory(NewDefaultConfigAtomProvider())

	ctx := context.Background()
	result := &CompilationResult{Prompt: "Test Prompt"}

	coderCfg, err := factory.Generate(ctx, result, "/fix")
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

	testerCfg, err := factory.Generate(ctx, result, "/test")
	if err != nil {
		t.Fatalf("Failed to generate tester config: %v", err)
	}
	assertContainsAll(t, testerCfg.Policies, []string{"policy/constitution.mg", "policy/validation.mg", "tester.mg"}, "Tester")

	reviewerCfg, err := factory.Generate(ctx, result, "/review")
	if err != nil {
		t.Fatalf("Failed to generate reviewer config: %v", err)
	}
	assertContainsAll(t, reviewerCfg.Policies, []string{"policy/constitution.mg", "policy/validation.mg", "reviewer.mg"}, "Reviewer")

	researchCfg, err := factory.Generate(ctx, result, "/research")
	if err != nil {
		t.Fatalf("Failed to generate researcher config: %v", err)
	}
	assertContainsAll(t, researchCfg.AllowedTools, []string{"browser_observe", "browser_act", "browser_mangle", "browser_wait", "browser_reason", "browser_evidence", "browser_specs", "browser_test"}, "Researcher")
}

func TestDefaultConfigAtomProvider_ProgressiveBrowserToolsReachResearchAndVerify(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	for _, intent := range []string{"/research", "/explore", "/verify", "/validate"} {
		atom, ok := provider.GetAtom(intent)
		if !ok {
			t.Fatalf("missing config atom for %s", intent)
		}
		assertContainsAll(t, atom.Tools, []string{"browser_observe", "browser_act", "browser_mangle", "browser_wait", "browser_reason", "browser_evidence", "browser_specs", "browser_test"}, intent)
	}
}

func TestConfigGeneration_HybridIntents(t *testing.T) {
	factory := NewConfigFactory(NewDefaultConfigAtomProvider())

	ctx := context.Background()
	result := &CompilationResult{Prompt: "Test Prompt"}

	fixCfg, err := factory.Generate(ctx, result, "/fix")
	if err != nil {
		t.Fatalf("Failed to generate /fix config: %v", err)
	}
	testCfg, err := factory.Generate(ctx, result, "/test")
	if err != nil {
		t.Fatalf("Failed to generate /test config: %v", err)
	}

	// Hybrid: /fix (coder) + /test (tester)
	hybridCfg, err := factory.Generate(ctx, result, "/fix", "/test")
	if err != nil {
		t.Fatalf("Failed to generate hybrid config: %v", err)
	}

	// Should have both policy sets
	assertContainsAll(t, hybridCfg.Policies, []string{
		"policy/constitution.mg",
		"policy/validation.mg",
		"policy/coder_workflow.mg",
		"tester.mg",
	}, "Hybrid")

	// Should have the union of both intents' tools
	assertContainsAll(t, hybridCfg.AllowedTools, fixCfg.AllowedTools, "Hybrid/fix")
	assertContainsAll(t, hybridCfg.AllowedTools, testCfg.AllowedTools, "Hybrid/test")
	// The production catalog grants no free-form shell; a hybrid must not
	// manufacture one either.
	if contains(hybridCfg.AllowedTools, "run_shell_command") {
		t.Errorf("Hybrid config grants free-form shell: %v", hybridCfg.AllowedTools)
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
