package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSpecialistConfig(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Change to temp dir to mock .nerd location (auto-restored via t.Chdir)
	t.Chdir(tmpDir)

	// Create .nerd/agents/test-agent/config.yaml
	configDir := filepath.Join(".nerd", "agents", "test-agent")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	// Matches the current EffectiveAgentRuntimeConfig yaml shape: flat
	// allowed_tools/policies, no nested Tools or Policies.Files wrappers,
	// no Mode field (Mode was removed in the cleanup pass).
	configContent := `
identity_prompt: "You are a test agent."
allowed_tools:
  - "read_file"
policies:
  - "policy/validation.mg"
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Spawner (with nil configFactory since we expect file load to succeed)
	spawner := &Spawner{}

	// Test Case 1: Load existing config
	ctx := context.Background()
	cfg, err := spawner.loadSpecialistConfig(ctx, "test-agent")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IdentityPrompt != "You are a test agent." {
		t.Errorf("Expected IdentityPrompt 'You are a test agent.', got '%s'", cfg.IdentityPrompt)
	}
	if len(cfg.AllowedTools) != 1 || cfg.AllowedTools[0] != "read_file" {
		t.Errorf("Unexpected tools: %v", cfg.AllowedTools)
	}
	// (Mode field removed from EffectiveAgentRuntimeConfig; no longer asserted.)

	// Test Case 2: Load missing config (Fallback)
	// With nil configFactory, it should return empty config
	cfgFallback, err := spawner.loadSpecialistConfig(ctx, "missing-agent")
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	// Check if it returned an empty config (since configFactory is nil)
	// We expect empty EffectiveAgentRuntimeConfig, checking IdentityPrompt is empty
	if cfgFallback.IdentityPrompt != "" {
		t.Errorf("Expected empty config for missing agent, got: %+v", cfgFallback)
	}

	// Double check that we received a valid pointer
	if cfgFallback == nil {
		t.Error("Expected non-nil config for fallback")
	}
}

func TestLoadSpecialistConfigRejectsInvalidRuntimeConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	tests := []struct {
		name       string
		configYAML string
		wantField  string
	}{
		{
			name: "blank-identity",
			configYAML: `identity_prompt: "   "
policies:
  - "policy/constitution.mg"
`,
			wantField: "identity_prompt",
		},
		{
			name: "missing-policies",
			configYAML: `identity_prompt: "You are a bounded specialist."
allowed_tools:
  - "read_file"
`,
			wantField: "policy file",
		},
	}

	spawner := &Spawner{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := filepath.Join(".nerd", "agents", tt.name)
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatalf("create specialist config directory: %v", err)
			}
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0o644); err != nil {
				t.Fatalf("write specialist config: %v", err)
			}

			cfg, err := spawner.loadSpecialistConfig(context.Background(), tt.name)
			if err == nil {
				t.Fatalf("loadSpecialistConfig() cfg = %+v, want validation error", cfg)
			}
			if !strings.Contains(err.Error(), configPath) {
				t.Errorf("error %q does not identify config path %q", err, configPath)
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Errorf("error %q does not identify invalid field %q", err, tt.wantField)
			}
		})
	}
}

func TestLoadSpecialistConfigPreservesBoundaryGates(t *testing.T) {
	t.Chdir(t.TempDir())
	spawner := &Spawner{}

	t.Run("path-containment", func(t *testing.T) {
		cfg, err := spawner.loadSpecialistConfig(context.Background(), "../escape")
		if err == nil {
			t.Fatalf("loadSpecialistConfig() cfg = %+v, want containment error", cfg)
		}
		if !strings.Contains(err.Error(), "path traversal") {
			t.Fatalf("loadSpecialistConfig() error = %q, want path traversal context", err)
		}
	})

	t.Run("one-megabyte-limit", func(t *testing.T) {
		const name = "oversized-specialist"
		configDir := filepath.Join(".nerd", "agents", name)
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("create specialist config directory: %v", err)
		}
		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(strings.Repeat("x", maxSpecialistConfigSize+1)), 0o644); err != nil {
			t.Fatalf("write oversized specialist config: %v", err)
		}

		cfg, err := spawner.loadSpecialistConfig(context.Background(), name)
		if err == nil {
			t.Fatalf("loadSpecialistConfig() cfg = %+v, want size error", cfg)
		}
		if !strings.Contains(err.Error(), "exceeds maximum size") {
			t.Fatalf("loadSpecialistConfig() error = %q, want size context", err)
		}
	})
}
