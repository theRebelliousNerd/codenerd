package prompt

import (
	"context"
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
				if len(cfg.Tools.AllowedTools) != len(tt.wantTools) {
					t.Errorf("Generate() Tools count = %v, want %v", len(cfg.Tools.AllowedTools), len(tt.wantTools))
				}

				// Verify policies
				if len(cfg.Policies.Files) != len(tt.wantPolicies) {
					t.Errorf("Generate() Policies count = %v, want %v", len(cfg.Policies.Files), len(tt.wantPolicies))
				}
			}
		})
	}
}
