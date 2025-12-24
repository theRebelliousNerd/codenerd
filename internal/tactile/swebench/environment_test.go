package swebench

import (
	"testing"

	"codenerd/internal/tactile/python"
)

func TestDefaultEnvironmentConfig(t *testing.T) {
	cfg := DefaultEnvironmentConfig()
	if cfg.WorkspaceDir != "/testbed" {
		t.Fatalf("unexpected workspace dir: %s", cfg.WorkspaceDir)
	}
	if cfg.TestTimeout <= 0 || cfg.SetupTimeout <= 0 {
		t.Fatalf("expected positive timeouts")
	}
	if !cfg.NetworkEnabled {
		t.Fatalf("expected network enabled by default")
	}
}

func TestNewHarnessWorkspaceOverride(t *testing.T) {
	instance := &Instance{Repo: "org/repo"}
	cfg := python.EnvironmentConfig{WorkspaceDir: "/workspace"}
	harness := NewHarness(instance, cfg, nil)

	expected := "/testbed/repo"
	if harness.Environment().RepoPath() != expected {
		t.Fatalf("expected repo path %s, got %s", expected, harness.Environment().RepoPath())
	}
}
