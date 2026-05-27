package config

import (
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
				Policies:       []string{"base.mg", "coder.mg"},
			},
			wantErr: false,
		},
		{
			name: "Missing Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"base.mg"},
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "   \t\n  ",
				AllowedTools:   []string{"read_file"},
				Policies:       []string{"base.mg"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("EffectiveAgentRuntimeConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
