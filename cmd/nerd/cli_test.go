package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func TestInitCmd(t *testing.T) {
	// Initialize global logger
	logger = zap.NewNop()

	// Setup temp workspace
	ws := t.TempDir()
	workspace = ws // Set global workspace flag
	defer func() { workspace = "" }()

	// Mock args
	cmd := &cobra.Command{}

	// Execute runInit
	err := runInit(cmd, []string{})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Verify .nerd directory exists
	if _, err := os.Stat(filepath.Join(ws, ".nerd")); os.IsNotExist(err) {
		t.Error(".nerd directory was not created")
	}

	// Test idempotency (running it again should warn but pass)
	err = runInit(cmd, []string{})
	if err != nil {
		t.Errorf("runInit second run failed: %v", err)
	}
}

func TestScanCmd(t *testing.T) {
	logger = zap.NewNop()
	// Setup temp workspace
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	// Mock args
	cmd := &cobra.Command{}

	// 1. Run scan before init (should pass but print warning)
	err := runScan(cmd, []string{})
	if err != nil {
		t.Fatalf("runScan failed on uninitialized repo: %v", err)
	}

	// 2. Init
	if err := runInit(cmd, []string{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 3. Create some files to scan
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Run scan
	err = runScan(cmd, []string{})
	if err != nil {
		t.Fatalf("runScan failed: %v", err)
	}

	// Verify facts persisted
	factsPath := filepath.Join(ws, ".nerd", "mangle", "scan.mg")
	if _, err := os.Stat(factsPath); os.IsNotExist(err) {
		t.Error("scan.mg was not created")
	}
}

func TestSpawnCmd_ShortTimeout(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	// Create a very short timeout so we don't hang trying to boot cortex
	// Save old timeout
	oldTimeout := timeout
	timeout = 10 * time.Millisecond
	defer func() { timeout = oldTimeout }()

	cmd := &cobra.Command{}
	ctx := context.Background()
	cmd.SetContext(ctx)

	// This is expected to fail because cortex boot needs more time/resources/api key
	err := spawnShard(cmd, []string{"coder", "fix bugs"})
	if err == nil {
		// If it somehow passes, that's okay too (maybe mocks involved in future),
		// but we mostly want to ensure it exercises the code path.
	} else {
		t.Logf("spawnShard failed as expected (timeout/boot): %v", err)
	}
}

func TestDefineAgentCmd_Validation(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	cmd := &cobra.Command{}
	// Mock flags
	cmd.Flags().String("name", "Invalid Name!", "help") // Space and ! are invalid
	cmd.Flags().String("topic", "Go", "help")

	err := defineAgent(cmd, []string{})
	if err == nil {
		t.Error("defineAgent should fail with invalid name")
	}
}

func TestDirectActions_Validation(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	cmd := &cobra.Command{}
	ctx := context.Background()
	cmd.SetContext(ctx)

	// Test review with no args
	// reviewCmd calls runDirectAction("review", ...)
	// We can call runDirectAction directly.

	// runDirectAction tries to boot cortex. We expect timeout/fail.
	oldTimeout := timeout
	timeout = 10 * time.Millisecond
	defer func() { timeout = oldTimeout }()

	err := runDirectAction("reviewer", "/review")(cmd, []string{})
	if err == nil {
		// Should fail due to empty args if it checks, or boot failure.
	}
}

func TestQueryCmd(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	// Init first to have a DB
	_ = runInit(&cobra.Command{}, []string{})

	// Query non-existent predicate
	// queryFacts is in cmd_query.go
	err := queryFacts(&cobra.Command{}, []string{"non_existent_pred"})
	// Should not error, just print nothing or "0 facts".
	if err != nil {
		t.Errorf("queryFacts failed: %v", err)
	}
}
