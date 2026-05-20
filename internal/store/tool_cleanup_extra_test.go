package store

import (
	"context"
	"path/filepath"
	"testing"
	
	"codenerd/internal/types"
)

// A mock LLM client for cleanup testing
type mockCleanupLLMClient struct {
	response string
	err      error
}

func (m *mockCleanupLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

func (m *mockCleanupLLMClient) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	return m.response, m.err
}

func (m *mockCleanupLLMClient) CompleteWithStreaming(ctx context.Context, system string, prompt string, requireJSON bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	
	if m.err != nil {
		errCh <- m.err
	} else {
		ch <- m.response
	}
	
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m *mockCleanupLLMClient) CompleteWithTools(ctx context.Context, system, prompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: m.response}, m.err
}

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

func TestToolStore_GetToolStatsSummary_And_Intelligent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tools.db")
	s, err := NewToolStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create tool store: %v", err)
	}
	defer s.Close()

	// Add execution
	exec := ToolExecution{
		CallID:           "c3",
		SessionID:        "s3",
		ToolName:         "test_tool",
		Success:          true,
		ResultSize:       1024,
		DurationMs:       50,
		SessionRuntimeMs: 11 * 3600000, // 11 hours, so it's NOT < 10 hours
	}
	if err := s.Store(exec); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	summaries, err := s.GetToolStatsSummary()
	if err != nil {
		t.Errorf("GetToolStatsSummary failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 summary, got %d", len(summaries))
	}

	// Test CleanupIntelligent
	mockLLM := &mockCleanupLLMClient{
		response: `{"delete_stale":{"before_runtime_hours":10,"min_references":0}}`,
	}
	
	// Add an old execution to be deleted
	oldExec := ToolExecution{
		CallID:           "old1",
		SessionID:        "old_s",
		SessionRuntimeMs: 5 * 3600000, // 5 hours, so < 10 hours -> gets deleted
		ReferenceCount:   0,
	}
	s.Store(oldExec)

	stats, err := s.CleanupIntelligent(context.Background(), mockLLM)
	if err != nil {
		t.Errorf("CleanupIntelligent failed: %v", err)
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
		CallID:           "c4",
		SessionID:        "s4",
		ToolName:         "mytool",
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
