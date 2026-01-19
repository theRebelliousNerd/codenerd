package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

type MockLLMClient struct {
	// Satisfy interface
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "Mock response", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "Mock response", nil
}

func (m *MockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{
		Text: "Mock tool response",
	}, nil
}

func TestNewInitializer(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultInitConfig(tmpDir)
	cfg.LLMClient = &MockLLMClient{}

	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	if init == nil {
		t.Fatal("Expected non-nil initializer")
	}
	defer init.Close()
}

func TestInitializer_Initialize_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping init integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a dummy file to scan
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)

	cfg := DefaultInitConfig(tmpDir)
	cfg.LLMClient = &MockLLMClient{}
	cfg.SkipResearch = true
	cfg.SkipAgentCreate = true
	cfg.Interactive = false // Important to avoid prompts

	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	defer init.Close()

	// Run initialize
	res, err := init.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !res.Success {
		t.Error("Expected success")
	}

	// Verify .nerd directory
	if _, err := os.Stat(filepath.Join(tmpDir, ".nerd")); os.IsNotExist(err) {
		t.Error(".nerd directory not created")
	}
	// Verify knowledge.db
	if _, err := os.Stat(filepath.Join(tmpDir, ".nerd", "knowledge.db")); os.IsNotExist(err) {
		t.Error("knowledge.db not created")
	}
}
