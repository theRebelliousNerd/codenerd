package store

import (
	"path/filepath"
	"testing"
)

func TestToolStore_AutoCleanup_And_CleanupBySizeLimit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tools.db")
	s, err := NewToolStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create tool store: %v", err)
	}
	defer s.Close()

	// 1. Test ShouldAutoCleanup
	cfgSize := CleanupConfig{
		MaxSizeBytes:         50, // lower than 60
		AutoCleanupThreshold: 0.5,
		CleanupMode:          "size",
	}

	// Add an execution with size 60 to trigger size limit (60 > 50)
	exec := ToolExecution{
		CallID:           "c1",
		SessionID:        "s1",
		ResultSize:       60,
		SessionRuntimeMs: 1000,
	}
	if err := s.Store(exec); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if !s.ShouldAutoCleanup(cfgSize) {
		t.Errorf("expected ShouldAutoCleanup to return true for size")
	}

	// Test AutoCleanup (size mode)
	stats, err := s.AutoCleanup(cfgSize)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if stats.ExecutionsDeleted != 1 {
		t.Errorf("expected 1 execution deleted, got %d", stats.ExecutionsDeleted)
	}

	// 2. Test ShouldAutoCleanup (runtime mode)
	// Make sure DB is empty or accounts for previous deletion
	cfgRuntime := CleanupConfig{
		MaxRuntimeHours:      1.0,
		AutoCleanupThreshold: 0.5,
		CleanupMode:          "runtime",
	}

	exec2 := ToolExecution{
		CallID:           "c2",
		SessionID:        "s2",
		SessionRuntimeMs: int64(2.0 * 3600000.0), // 2 hours
	}
	if err := s.Store(exec2); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if !s.ShouldAutoCleanup(cfgRuntime) {
		t.Errorf("expected ShouldAutoCleanup to return true for runtime")
	}

	// Test AutoCleanup (runtime mode)
	stats, err = s.AutoCleanup(cfgRuntime)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if stats.ExecutionsDeleted != 1 {
		t.Errorf("expected 1 execution deleted, got %d", stats.ExecutionsDeleted)
	}
}

func TestToolStore_ToolStoreMissingMethods(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tools.db")
	s, err := NewToolStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create tool store: %v", err)
	}
	defer s.Close()

	exec := ToolExecution{
		CallID:    "c4",
		SessionID: "s4",
		ToolName:  "mytool",
	}
	s.Store(exec)

	// GetBySession
	execs, err := s.GetBySession("s4")
	if err != nil {
		t.Errorf("GetBySession failed: %v", err)
	}
	if len(execs) != 1 {
		t.Errorf("expected 1 exec, got %d", len(execs))
	}

	// GetRecent
	recent, err := s.GetRecent(10)
	if err != nil {
		t.Errorf("GetRecent failed: %v", err)
	}
	if len(recent) != 1 {
		t.Errorf("expected 1 recent, got %d", len(recent))
	}

	// GetRecentByTool
	recentByTool, err := s.GetRecentByTool("mytool", 10)
	if err != nil {
		t.Errorf("GetRecentByTool failed: %v", err)
	}
	if len(recentByTool) != 1 {
		t.Errorf("expected 1 recent by tool, got %d", len(recentByTool))
	}

	// GetStats
	overallStats, err := s.GetStats()
	if err != nil {
		t.Errorf("GetStats failed: %v", err)
	}
	if overallStats.TotalExecutions != 1 {
		t.Errorf("expected 1 total exec, got %d", overallStats.TotalExecutions)
	}

	// Check DefaultCleanupConfig doesn't panic
	cfg := DefaultCleanupConfig()
	if cfg.MaxRuntimeHours == 0 {
		t.Errorf("expected default config to have non-zero runtime")
	}
}
