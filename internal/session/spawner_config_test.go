package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpecialistConfig(t *testing.T) {
	// Save current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Change to temp dir to mock .nerd location
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Create .nerd/agents/test-agent/config.yaml
	configDir := filepath.Join(".nerd", "agents", "test-agent")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	configContent := `
identity_prompt: "You are a test agent."
tools:
  allowed_tools:
    - "read_file"
policies:
  files:
    - "policy/test.mg"
mode: "SingleTurn"
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
	if len(cfg.Tools.AllowedTools) != 1 || cfg.Tools.AllowedTools[0] != "read_file" {
		t.Errorf("Unexpected tools: %v", cfg.Tools.AllowedTools)
	}
	if cfg.Mode != "SingleTurn" {
		t.Errorf("Expected Mode 'SingleTurn', got '%s'", cfg.Mode)
	}

	// Test Case 2: Load missing config (Fallback)
	// With nil configFactory, it should return empty config
	cfgFallback, err := spawner.loadSpecialistConfig(ctx, "missing-agent")
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	// Check if it returned an empty config (since configFactory is nil)
	// We expect empty AgentConfig, checking IdentityPrompt is empty
	if cfgFallback.IdentityPrompt != "" {
		t.Errorf("Expected empty config for missing agent, got: %+v", cfgFallback)
	}

	// Double check that we received a valid pointer
	if cfgFallback == nil {
		t.Error("Expected non-nil config for fallback")
	}
}
