package prompt

import (
	"context"
	"strings"
	"testing"
)

// TestGenerate_AllRegisteredVerbsPassValidate proves Generate populates a
// non-empty Policies slice for every verb the atom provider knows. Validate
// requires it, so any verb that generates an empty list is a silent
// disarm: the turn would run with no policy anchor.
func TestGenerate_AllRegisteredVerbsPassValidate(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	result := &CompilationResult{Prompt: "You are codeNERD."}

	intents := provider.RegisteredIntents()
	if len(intents) == 0 {
		t.Fatal("RegisteredIntents returned no verbs")
	}
	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			cfg, err := factory.Generate(ctx, result, intent)
			if err != nil {
				t.Fatalf("Generate(%q) error = %v", intent, err)
			}
			if cfg == nil {
				t.Fatalf("Generate(%q) returned nil config", intent)
			}
			if len(cfg.Policies) == 0 {
				t.Fatalf("Generate(%q) produced empty Policies", intent)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Generate(%q) produced config that fails Validate: %v (policies=%v)",
					intent, err, cfg.Policies)
			}
		})
	}
}

// TestGenerate_RejectsEmptyPolicies proves a config with empty Policies is
// rejected at Generate time instead of passing silently downstream.
func TestGenerate_RejectsEmptyPolicies(t *testing.T) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/empty": {Tools: []string{"read_file"}, Policies: nil},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()

	_, err := factory.Generate(ctx, &CompilationResult{Prompt: "identity"}, "/empty")
	if err == nil {
		t.Fatal("Generate with empty Policies succeeded, want validation error")
	}
	if !strings.Contains(err.Error(), "policy") {
		t.Fatalf("Generate error %q does not mention policy", err)
	}
}
