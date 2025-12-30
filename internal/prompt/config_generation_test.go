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
	if !contains(coderCfg.Policies.Files, "coder.mg") {
		t.Errorf("Coder config missing coder.mg")
	}

	// Test Tester
	testerCfg, err := factory.Generate(ctx, result, "/tester")
	if err != nil {
		t.Fatalf("Failed to generate tester config: %v", err)
	}
	if !contains(testerCfg.Policies.Files, "tester.mg") {
		t.Errorf("Tester config missing tester.mg")
	}

	// Test Reviewer
	reviewerCfg, err := factory.Generate(ctx, result, "/reviewer")
	if err != nil {
		t.Fatalf("Failed to generate reviewer config: %v", err)
	}
	if !contains(reviewerCfg.Policies.Files, "reviewer.mg") {
		t.Errorf("Reviewer config missing reviewer.mg")
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
	if !contains(hybridCfg.Policies.Files, "coder.mg") || !contains(hybridCfg.Policies.Files, "tester.mg") {
		t.Errorf("Hybrid config missing policies: %v", hybridCfg.Policies.Files)
	}

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
