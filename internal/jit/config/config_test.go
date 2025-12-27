package config

import (
	"testing"
)

func TestAgentConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: AgentConfig{
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
			config: AgentConfig{
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
			name: "Empty Policies",
			config: AgentConfig{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("AgentConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
