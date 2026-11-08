package config

import (
	"strings"
	"testing"
)

func TestAgentConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  EffectiveAgentRuntimeConfig
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "You are a helpful agent.",
				AllowedTools:   []string{"read_file", "write_file"},
				Policies:       []string{"policy/constitution.mg", "policy/coder_safety.mg"},
			},
			wantErr: false,
		},
		{
			name: "Missing Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"policy/constitution.mg"},
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "   \t\n  ",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"policy/constitution.mg"},
			},
			wantErr: true,
		},
		{
			name: "Empty Policies",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{},
			},
			wantErr: true,
		},
		{
			name: "Nil Policies Files",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				AllowedTools:   []string{"read_file"},
				Policies:       nil,
			},
			wantErr: true,
		},
		{
			name: "Missing Policy Alias",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"base.mg"},
			},
			wantErr: true,
		},
		{
			name: "Traversal Policy",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"../policy/constitution.mg"},
			},
			wantErr: true,
		},
		{
			name: "Duplicate Policy",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"policy/constitution.mg", "policy/constitution.mg"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("EffectiveAgentRuntimeConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentConfigValidation_UncheckedFieldsStayValid(t *testing.T) {
	tests := []struct {
		name   string
		config EffectiveAgentRuntimeConfig
	}{
		{
			name: "Empty allowlist fails open at validation",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				AllowedTools:   []string{},
				Policies:       []string{"policy/constitution.mg"},
			},
		},
		{
			name: "Nil allowlist fails open at validation",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				AllowedTools:   nil,
				Policies:       []string{"policy/constitution.mg"},
			},
		},
		{
			name: "Empty persona and verb are descriptive only",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				IntentVerb:     "",
				Persona:        "",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"policy/constitution.mg"},
			},
		},
		{
			name: "Padded identity is trimmed not rejected",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "  grounded identity  ",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"policy/constitution.mg"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); err != nil {
				t.Errorf("EffectiveAgentRuntimeConfig.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestAgentConfigValidation_RejectsPolicyVariants(t *testing.T) {
	tests := []struct {
		name   string
		config EffectiveAgentRuntimeConfig
	}{
		{
			name: "Second entry non-canonical",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"policy/constitution.mg", "base.mg"},
			},
		},
		{
			name: "Non-adjacent duplicate",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"policy/constitution.mg", "policy/coder_safety.mg", "policy/constitution.mg"},
			},
		},
		{
			name: "Whitespace-padded policy",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{" policy/constitution.mg"},
			},
		},
		{
			name: "Backslash policy",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{`policy\constitution.mg`},
			},
		},
		{
			name: "Empty string policy entry",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{""},
			},
		},
		{
			name: "Absolute policy path",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Policies:       []string{"/policy/constitution.mg"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); err == nil {
				t.Errorf("EffectiveAgentRuntimeConfig.Validate() error = nil, want error")
			}
		})
	}
}

func TestAgentConfigValidation_ErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		config  EffectiveAgentRuntimeConfig
		wantSub string
	}{
		{
			name:    "Missing identity names field",
			config:  EffectiveAgentRuntimeConfig{Policies: []string{"policy/constitution.mg"}},
			wantSub: "identity_prompt",
		},
		{
			name:    "Empty policies names requirement",
			config:  EffectiveAgentRuntimeConfig{IdentityPrompt: "Identity"},
			wantSub: "at least one policy",
		},
		{
			name:    "Non-canonical names canonical",
			config:  EffectiveAgentRuntimeConfig{IdentityPrompt: "Identity", Policies: []string{"base.mg"}},
			wantSub: "not a canonical",
		},
		{
			name:    "Duplicate names duplicate",
			config:  EffectiveAgentRuntimeConfig{IdentityPrompt: "Identity", Policies: []string{"policy/constitution.mg", "policy/constitution.mg"}},
			wantSub: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Fatalf("EffectiveAgentRuntimeConfig.Validate() error = nil, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("EffectiveAgentRuntimeConfig.Validate() error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}
