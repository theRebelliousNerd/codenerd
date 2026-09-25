package prompt

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

func TestDefaultConfigFactoryPoliciesResolveAgainstCoreInventory(t *testing.T) {
	factory := NewConfigFactory(NewDefaultConfigAtomProvider())
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
