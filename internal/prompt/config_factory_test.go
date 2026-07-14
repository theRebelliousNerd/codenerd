package prompt

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
)

// MockConfigAtomProvider simulates the retrieval of config atoms.
type MockConfigAtomProvider struct {
	atoms map[string]ConfigAtom
}

func (m *MockConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
	atom, ok := m.atoms[intent]
	return atom, ok
}



// TODO: [Null/Undefined/Empty] Missing test for NewConfigFactory(nil) which would panic when Generate() is called.
// TODO: [Null/Undefined/Empty] Missing test for ConfigAtom.Merge behavior when Tools or Policies are explicitly nil versus empty slices, ensuring it doesn't panic and returns initialized slices if expected.
// TODO: [Type Coercion] Missing test for intent strings containing null bytes (\x00) or non-UTF8 sequences. Ensure GetAtom safely handles these without panicking or returning unexpected fallback atoms.
// TODO: [User Request Extremes] Missing test for GenerateFallback with a massive fallbackIdentity string (e.g., 50MB) to ensure it doesn't cause OOM when constructing the EffectiveAgentRuntimeConfig.
// TODO: [User Request Extremes] Missing test for uniqueStrings performance/OOM when handling a ConfigAtom with millions of duplicated tool strings.
// TODO: [State Conflicts] Missing test verifying that mutating a slice (e.g., Tools) of a ConfigAtom *after* passing it to RegisterAtom does not cause race conditions when Generate() concurrently reads from it.

func TestConfigFactory_Generate(t *testing.T) {

	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/coder": {
				Tools:    []string{"write_file", "read_file"},
				Policies: []string{"coder.mg"},
			},
			"/tester": {
				Tools:    []string{"run_test", "read_file"},
				Policies: []string{"tester.mg"},
			},
		},
	}

	factory := NewConfigFactory(provider)

	tests := []struct {
		name           string
		intent         string
		identityPrompt string
		wantTools      []string
		wantPolicies   []string
		wantErr        bool
	}{
		{
			name:           "Coder Intent",
			intent:         "/coder",
			identityPrompt: "You are a coder.",
			wantTools:      []string{"write_file", "read_file"},
			wantPolicies:   []string{"coder.mg"},
			wantErr:        false,
		},
		{
			name:           "Tester Intent",
			intent:         "/tester",
			identityPrompt: "You are a tester.",
			wantTools:      []string{"run_test", "read_file"},
			wantPolicies:   []string{"tester.mg"},
			wantErr:        false,
		},
		{
			name:           "Unknown Intent",
			intent:         "/unknown",
			identityPrompt: "Who am I?",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			compilationResult := &CompilationResult{
				Prompt: tt.identityPrompt,
			}

			// We need a way to pass intent. For now, let's assume it's passed directly or derived.
			// In the real implementation, we might extract it from CompilationContext.
			// Here we just test the factory logic.

			cfg, err := factory.Generate(ctx, compilationResult, tt.intent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigFactory.Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.IdentityPrompt != tt.identityPrompt {
					t.Errorf("Generate() IdentityPrompt = %v, want %v", cfg.IdentityPrompt, tt.identityPrompt)
				}

				// Verify tools
				if len(cfg.AllowedTools) != len(tt.wantTools) {
					t.Errorf("Generate() Tools count = %v, want %v", len(cfg.AllowedTools), len(tt.wantTools))
				}

				// Verify policies
				if len(cfg.Policies) != len(tt.wantPolicies) {
					t.Errorf("Generate() Policies count = %v, want %v", len(cfg.Policies), len(tt.wantPolicies))
				}
			}
		})
	}
}

func TestConfigFactory_NullUndefinedEmpty(t *testing.T) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"": {
				Tools:    []string{"fallback_tool"},
				Policies: []string{"fallback.mg"},
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()

	// Test 1: compilationResult is nil
	_, err := factory.Generate(ctx, nil, "/coder")
	if err == nil || !strings.Contains(err.Error(), "compilation result cannot be nil") {
		t.Errorf("Expected error about nil compilation result, got %v", err)
	}
}

func TestConfigFactory_NullUndefinedEmpty_2(t *testing.T) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"": {
				Tools:    []string{"fallback_tool"},
				Policies: []string{"fallback.mg"},
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	compilationResult := &CompilationResult{Prompt: "test"}

	cfg, err := factory.Generate(ctx, compilationResult)
	if err == nil {
		t.Errorf("Expected error for empty intents slice, got cfg: %v", cfg)
	}

	var intents []string
	cfg, err = factory.Generate(ctx, compilationResult, intents...)
	if err == nil {
		t.Errorf("Expected error for nil intents slice, got cfg: %v", cfg)
	}

	cfg, err = factory.Generate(ctx, compilationResult, "")
	if err != nil {
		t.Errorf("Unexpected error for empty string intent: %v", err)
	} else {
		if len(cfg.AllowedTools) != 1 || cfg.AllowedTools[0] != "fallback_tool" {
			t.Errorf("Expected fallback_tool for empty string intent, got %v", cfg.AllowedTools)
		}
	}
}

func TestConfigFactory_TypeCoercion(t *testing.T) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/mixed_case": {
				Tools:    []string{"ToolA", "toolA "},
				Policies: []string{" Policy.mg", "policy.mg"},
				Priority: 10,
			},
			"/priority_min": {
				Tools:    []string{"tool"},
				Policies: []string{"policy.mg"},
				Priority: math.MinInt,
			},
			"/priority_max": {
				Tools:    []string{"tool2"},
				Policies: []string{"policy2.mg"},
				Priority: math.MaxInt,
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	compilationResult := &CompilationResult{Prompt: "test"}

	cfg, err := factory.Generate(ctx, compilationResult, "/mixed_case")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(cfg.AllowedTools) != 2 {
		t.Errorf("Expected 2 tools due to mixed casing and trailing spaces, got %v", len(cfg.AllowedTools))
	}
	if len(cfg.Policies) != 2 {
		t.Errorf("Expected 2 policies due to trailing spaces, got %v", len(cfg.Policies))
	}

	atomMin, _ := provider.GetAtom("/priority_min")
	atomMax, _ := provider.GetAtom("/priority_max")
	merged := atomMin.Merge(atomMax)
	if merged.Priority != math.MaxInt {
		t.Errorf("Merge priority math.MaxInt failed, got %v", merged.Priority)
	}
	merged2 := atomMax.Merge(atomMin)
	if merged2.Priority != math.MaxInt {
		t.Errorf("Merge priority math.MinInt over math.MaxInt failed, got %v", merged2.Priority)
	}
}

func TestConfigFactory_UserExtremes(t *testing.T) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/base": {
				Tools:    []string{"t1"},
				Policies: []string{"p1.mg"},
			},
		},
	}
	for range 10000 {
		provider.atoms["/base"] = ConfigAtom{
			Tools:    []string{"t1", "t2"},
			Policies: []string{"p1.mg", "p2.mg"},
		}
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	compilationResult := &CompilationResult{Prompt: "test"}

	intents := make([]string, 10000)
	for i := range 10000 {
		intents[i] = "/base"
	}

	cfg, err := factory.Generate(ctx, compilationResult, intents...)
	if err != nil {
		t.Fatalf("Generate failed for large intents array: %v", err)
	}

	if len(cfg.AllowedTools) != 2 {
		t.Errorf("Deduplication failed or returned wrong number of tools: %v", len(cfg.AllowedTools))
	}
	if len(cfg.Policies) != 2 {
		t.Errorf("Deduplication failed or returned wrong number of policies: %v", len(cfg.Policies))
	}
}

// TestDefaultConfigFactory_OutputPassesValidate asserts that every config
// produced by the default factory satisfies EffectiveAgentRuntimeConfig.Validate.
// This is the regression guard for bug #13: the default provider previously
// produced configs with empty Policies, which would fail validation once
// Validate started enforcing the policy requirement.
func TestDefaultConfigFactory_OutputPassesValidate(t *testing.T) {
	factory := NewDefaultConfigFactory()
	ctx := context.Background()
	compileResult := &CompilationResult{Prompt: "You are codeNERD."}

	intents := []string{
		// Coder family
		"/fix", "/implement", "/refactor", "/create", "/modify", "/add", "/update",
		// Tester family
		"/test", "/cover", "/verify", "/validate",
		// Reviewer family
		"/review", "/audit", "/check", "/analyze", "/inspect",
		// Researcher family
		"/research", "/learn", "/document", "/understand", "/explore", "/find",
		// Nemesis family
		"/attack", "/break", "/exploit", "/fuzz", "/pentest", "/nemesis",
		// Tool generator family
		"/generate", "/generate-tool", "/tool_generator", "/create_tool",
		// Fallback
		"/general",
	}

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			cfg, err := factory.Generate(ctx, compileResult, intent)
			if err != nil {
				t.Fatalf("Generate(%q) failed: %v", intent, err)
			}
			if cfg == nil {
				t.Fatalf("Generate(%q) returned nil config", intent)
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("Generate(%q) produced config that fails Validate: %v (policies=%v)",
					intent, err, cfg.Policies)
			}
		})
	}
}

// TestDefaultConfigFactory_FallbackPassesValidate asserts that GenerateFallback
// also produces a config that satisfies Validate when given a known intent.
func TestDefaultConfigFactory_FallbackPassesValidate(t *testing.T) {
	factory := NewDefaultConfigFactory()
	ctx := context.Background()

	cfg := factory.GenerateFallback(ctx, "/fix", "Fallback identity prompt.")
	if cfg == nil {
		t.Fatal("GenerateFallback returned nil")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("GenerateFallback produced config that fails Validate: %v (policies=%v)",
			err, cfg.Policies)
	}

	// Unknown intent should fall back to /general, which also has policies.
	cfgUnknown := factory.GenerateFallback(ctx, "/no-such-intent", "Fallback identity.")
	if cfgUnknown == nil {
		t.Fatal("GenerateFallback(unknown) returned nil")
	}
	if err := cfgUnknown.Validate(); err != nil {
		t.Errorf("GenerateFallback(unknown) produced config that fails Validate: %v (policies=%v)",
			err, cfgUnknown.Policies)
	}
}

func TestConfigFactory_StateConflicts(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	compilationResult := &CompilationResult{Prompt: "test"}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 1000 {
			_, _ = factory.Generate(ctx, compilationResult, "/coder")
		}
	}()

	go func() {
		defer wg.Done()
		for range 1000 {
			provider.RegisterAtom("/new_intent", ConfigAtom{Tools: []string{"tool"}, Policies: []string{"policy.mg"}})
		}
	}()

	wg.Wait()
}
