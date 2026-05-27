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
				Tools: ToolSet{
					AllowedTools: []string{"read_file", "write_file"},
				},
				Policies: PolicySet{
					Files: []string{"base.mg", "coder.mg"},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "",
				Tools: ToolSet{
					AllowedTools: []string{"read_file"},
				},
				Policies: PolicySet{
					Files: []string{"base.mg"},
				},
			},
			wantErr: true,
		},
		{
			name: "Whitespace-only Identity",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "   \t\n  ",
				Tools: ToolSet{
					AllowedTools: []string{"read_file"},
				},
				Policies: PolicySet{
					Files: []string{"base.mg"},
				},
			},
			wantErr: true,
		},
		{
			name: "Empty Policies",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Tools: ToolSet{
					AllowedTools: []string{"read_file"},
				},
				Policies: PolicySet{
					Files: []string{},
				},
			},
			wantErr: true,
		},
		{
			name: "Nil Policies Files",
			config: EffectiveAgentRuntimeConfig{
				IdentityPrompt: "Identity",
				Tools: ToolSet{
					AllowedTools: []string{"read_file"},
				},
				Policies: PolicySet{
					Files: nil,
				},
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
