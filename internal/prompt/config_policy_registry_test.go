package prompt

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

func TestDefaultConfigFactoryPoliciesResolveAgainstCoreInventory(t *testing.T) {
	factory := NewDefaultConfigFactory()
	result := &CompilationResult{Prompt: "bounded identity"}
	intents := []string{
		"/fix", "/test", "/review", "/research", "/attack", "/generate-tool", "/general",
	}

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			cfg, err := factory.Generate(context.Background(), result, intent)
			if err != nil {
				t.Fatalf("Generate(%q) error = %v", intent, err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Generate(%q) returned invalid policy references %v: %v", intent, cfg.Policies, err)
			}
			for _, policy := range cfg.Policies {
				if !core.IsDefaultPolicyFile(policy) {
					t.Errorf("Generate(%q) policy %q is not in the live core inventory", intent, policy)
				}
			}
		})
	}
}

func TestDefaultConfigProvidersShareCanonicalPolicySets(t *testing.T) {
	registry := NewSimpleRegistry()
	RegisterDefaultConfigAtoms(registry)
	legacyFactory := NewConfigFactory(registry)
	defaultFactory := NewDefaultConfigFactory()
	result := &CompilationResult{Prompt: "bounded identity"}

	for _, intent := range []string{"/fix", "/test", "/review", "/research"} {
		legacy, err := legacyFactory.Generate(context.Background(), result, intent)
		if err != nil {
			t.Fatalf("legacy Generate(%q) error = %v", intent, err)
		}
		current, err := defaultFactory.Generate(context.Background(), result, intent)
		if err != nil {
			t.Fatalf("default Generate(%q) error = %v", intent, err)
		}
		if len(legacy.Policies) != len(current.Policies) {
			t.Fatalf("Generate(%q) policy counts differ: registry=%v provider=%v", intent, legacy.Policies, current.Policies)
		}
		for i := range legacy.Policies {
			if legacy.Policies[i] != current.Policies[i] {
				t.Fatalf("Generate(%q) policy sets differ: registry=%v provider=%v", intent, legacy.Policies, current.Policies)
			}
		}
	}
}
