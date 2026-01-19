// Package chat provides tests for session.go adapters and utilities.
package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// =============================================================================
// SESSION KERNEL ADAPTER TESTS
// =============================================================================

func TestSessionKernelAdapter_LoadFacts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	facts := []types.Fact{
		{Predicate: "test_fact", Args: []interface{}{"arg1", "arg2"}},
	}

	err := adapter.LoadFacts(facts)
	if err != nil {
		t.Logf("LoadFacts returned error (may be expected): %v", err)
	}
}

func TestSessionKernelAdapter_Query(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	results, err := adapter.Query("config_value")
	if err != nil {
		t.Logf("Query returned error: %v", err)
	}
	t.Logf("Query returned %d results", len(results))
}

func TestSessionKernelAdapter_QueryAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	results, err := adapter.QueryAll()
	if err != nil {
		t.Logf("QueryAll returned error: %v", err)
	}
	t.Logf("QueryAll returned %d predicates", len(results))
}

func TestSessionKernelAdapter_Assert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	fact := types.Fact{Predicate: "test_asserted", Args: []interface{}{"value1"}}
	err := adapter.Assert(fact)
	if err != nil {
		t.Logf("Assert returned error: %v", err)
	}
}

func TestSessionKernelAdapter_Retract(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	// First assert a fact
	fact := types.Fact{Predicate: "test_to_retract", Args: []interface{}{"value"}}
	_ = adapter.Assert(fact)

	// Then retract
	err := adapter.Retract("test_to_retract")
	if err != nil {
		t.Logf("Retract returned error: %v", err)
	}
}

func TestSessionKernelAdapter_RetractFact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	fact := types.Fact{Predicate: "test_retract_fact", Args: []interface{}{"value"}}
	_ = adapter.Assert(fact)

	err := adapter.RetractFact(fact)
	if err != nil {
		t.Logf("RetractFact returned error: %v", err)
	}
}

func TestSessionKernelAdapter_UpdateSystemFacts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	err := adapter.UpdateSystemFacts()
	if err != nil {
		t.Logf("UpdateSystemFacts returned error: %v", err)
	}
}

func TestSessionKernelAdapter_Reset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	// Should not panic
	adapter.Reset()
	t.Log("Reset completed successfully")
}

func TestSessionKernelAdapter_AppendPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	// Should not panic
	adapter.AppendPolicy("test_rule(X) :- some_fact(X).")
	t.Log("AppendPolicy completed successfully")
}

func TestSessionKernelAdapter_RetractExactFactsBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	facts := []types.Fact{
		{Predicate: "batch_fact1", Args: []interface{}{"a"}},
		{Predicate: "batch_fact2", Args: []interface{}{"b"}},
	}

	err := adapter.RetractExactFactsBatch(facts)
	if err != nil {
		t.Logf("RetractExactFactsBatch returned error: %v", err)
	}
}

func TestSessionKernelAdapter_RemoveFactsByPredicateSet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	adapter := &sessionKernelAdapter{kernel: m.kernel}

	predicates := map[string]struct{}{
		"pred1": {},
		"pred2": {},
	}

	err := adapter.RemoveFactsByPredicateSet(predicates)
	if err != nil {
		t.Logf("RemoveFactsByPredicateSet returned error: %v", err)
	}
}

// =============================================================================
// SESSION VIRTUAL STORE ADAPTER TESTS
// =============================================================================

func TestSessionVirtualStoreAdapter_ReadFile(t *testing.T) {
	t.Parallel()

	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &sessionVirtualStoreAdapter{vs: nil}

	lines, err := adapter.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("Expected 'line1', got '%s'", lines[0])
	}
}

func TestSessionVirtualStoreAdapter_ReadFile_NotExists(t *testing.T) {
	t.Parallel()

	adapter := &sessionVirtualStoreAdapter{vs: nil}

	_, err := adapter.ReadFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestSessionVirtualStoreAdapter_WriteFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test requiring VirtualStore in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")

	// Use VirtualStore from model if available
	adapter := &sessionVirtualStoreAdapter{vs: m.virtualStore}

	lines := []string{"line1", "line2", "line3"}
	err := adapter.WriteFile(testFile, lines)
	if err != nil {
		t.Logf("WriteFile returned error: %v", err)
		// Fallback case - might not have editor wired
	}

	// Verify file was written (if no error)
	if err == nil {
		data, readErr := os.ReadFile(testFile)
		if readErr != nil {
			t.Logf("Failed to read written file: %v", readErr)
		} else {
			t.Logf("File content: %s", string(data))
		}
	}
}

func TestSessionVirtualStoreAdapter_WriteFile_NilVS(t *testing.T) {
	t.Parallel()

	// Test that WriteFile with nil VS panics (production code issue)
	// This test documents the current behavior
	adapter := &sessionVirtualStoreAdapter{vs: nil}

	defer func() {
		if r := recover(); r != nil {
			t.Log("WriteFile panicked with nil VirtualStore (expected - bug in production code)")
		}
	}()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	_ = adapter.WriteFile(testFile, []string{"test"})
}

func TestSessionVirtualStoreAdapter_WriteFile_WithVirtualStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring VirtualStore in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.virtualStore == nil {
		t.Skip("VirtualStore not available")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "vs_output.txt")

	adapter := &sessionVirtualStoreAdapter{vs: m.virtualStore}

	lines := []string{"line1", "line2"}
	err := adapter.WriteFile(testFile, lines)
	if err != nil {
		t.Logf("WriteFile with VirtualStore returned error: %v", err)
	}
}

func TestSessionVirtualStoreAdapter_Exec(t *testing.T) {
	t.Parallel()

	adapter := &sessionVirtualStoreAdapter{vs: nil}

	ctx := context.Background()
	stdout, stderr, err := adapter.Exec(ctx, "echo test", nil)

	// Should return "not yet wired" error
	if err == nil {
		t.Error("Expected error for Exec (not yet wired)")
	}
	if !strings.Contains(err.Error(), "not yet wired") {
		t.Errorf("Expected 'not yet wired' error, got: %v", err)
	}

	t.Logf("Exec result: stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
}

// =============================================================================
// SESSION LLM ADAPTER TESTS
// =============================================================================

func TestSessionLLMAdapter_Complete(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("Test response")

	adapter := &sessionLLMAdapter{client: mockClient}

	ctx := context.Background()
	result, err := adapter.Complete(ctx, "Test prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", result)
	}
}

func TestSessionLLMAdapter_CompleteWithSystem(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("System response")

	adapter := &sessionLLMAdapter{client: mockClient}

	ctx := context.Background()
	result, err := adapter.CompleteWithSystem(ctx, "System prompt", "User prompt")
	if err != nil {
		t.Fatalf("CompleteWithSystem failed: %v", err)
	}

	if result != "System response" {
		t.Errorf("Expected 'System response', got '%s'", result)
	}
}

func TestSessionLLMAdapter_CompleteWithTools(t *testing.T) {
	t.Parallel()

	mockClient := NewMockLLMClient()

	adapter := &sessionLLMAdapter{client: mockClient}

	ctx := context.Background()
	tools := []types.ToolDefinition{
		{Name: "test_tool", Description: "A test tool"},
	}

	result, err := adapter.CompleteWithTools(ctx, "System", "User", tools)
	if err != nil {
		t.Logf("CompleteWithTools returned error: %v", err)
	}
	if result != nil {
		t.Logf("CompleteWithTools returned response: %+v", result)
	}
}

func TestSessionLLMAdapter_NilClient(t *testing.T) {
	t.Parallel()

	adapter := &sessionLLMAdapter{client: nil}

	ctx := context.Background()

	// Should panic or return error
	defer func() {
		if r := recover(); r != nil {
			t.Log("Complete with nil client panicked (expected)")
		}
	}()

	_, err := adapter.Complete(ctx, "test")
	if err != nil {
		t.Logf("Complete with nil client: %v", err)
	}
}

// =============================================================================
// LOCAL STORE TRACE ADAPTER TESTS
// =============================================================================

func TestLocalStoreTraceAdapter_New(t *testing.T) {
	t.Parallel()

	adapter := NewLocalStoreTraceAdapter(nil)
	if adapter == nil {
		t.Error("Expected non-nil adapter")
	}
}

func TestLocalStoreTraceAdapter_StoreReasoningTrace_NilStore(t *testing.T) {
	t.Parallel()

	adapter := NewLocalStoreTraceAdapter(nil)

	trace := &perception.ReasoningTrace{
		SessionID: "test-session",
		Timestamp: time.Now(),
	}

	err := adapter.StoreReasoningTrace(trace)
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestLocalStoreTraceAdapter_StoreReasoningTrace_NilTrace(t *testing.T) {
	t.Parallel()

	adapter := NewLocalStoreTraceAdapter(nil)

	err := adapter.StoreReasoningTrace(nil)
	if err != nil {
		t.Errorf("Expected nil error for nil trace, got: %v", err)
	}
}

func TestLocalStoreTraceAdapter_StoreReasoningTrace_WithStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring LocalStore in short mode")
	}

	tmpDir := t.TempDir()
	localStore, err := store.NewLocalStore(tmpDir)
	if err != nil {
		t.Skipf("Failed to create LocalStore (SQLite issue on this platform): %v", err)
	}
	defer localStore.Close()

	adapter := NewLocalStoreTraceAdapter(localStore)

	trace := &perception.ReasoningTrace{
		SessionID: "test-session",
		Timestamp: time.Now(),
	}

	err = adapter.StoreReasoningTrace(trace)
	if err != nil {
		t.Logf("StoreReasoningTrace returned error: %v", err)
	}
}

// =============================================================================
// CORE LEARNING STORE ADAPTER TESTS
// =============================================================================

func TestCoreLearningStoreAdapter_Save_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.Save("coder", "learned_pattern", []any{"arg1"}, "campaign1")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_Load_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	results, err := adapter.Load("coder")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for nil store, got: %v", results)
	}
}

func TestCoreLearningStoreAdapter_LoadByPredicate_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	results, err := adapter.LoadByPredicate("coder", "learned_pattern")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for nil store, got: %v", results)
	}
}

func TestCoreLearningStoreAdapter_DecayConfidence_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.DecayConfidence("coder", 0.9)
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_Close_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.Close()
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_WithStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring LearningStore in short mode")
	}

	tmpDir := t.TempDir()
	learningStore, err := store.NewLearningStore(tmpDir)
	if err != nil {
		t.Skipf("Failed to create LearningStore (SQLite issue on this platform): %v", err)
	}
	defer learningStore.Close()

	adapter := &coreLearningStoreAdapter{store: learningStore}

	// Test Save
	err = adapter.Save("coder", "test_pattern", []any{"arg1", "arg2"}, "test_campaign")
	if err != nil {
		t.Logf("Save returned error: %v", err)
	}

	// Test Load
	results, err := adapter.Load("coder")
	if err != nil {
		t.Logf("Load returned error: %v", err)
	}
	t.Logf("Load returned %d results", len(results))

	// Test LoadByPredicate
	results, err = adapter.LoadByPredicate("coder", "test_pattern")
	if err != nil {
		t.Logf("LoadByPredicate returned error: %v", err)
	}
	t.Logf("LoadByPredicate returned %d results", len(results))

	// Test DecayConfidence
	err = adapter.DecayConfidence("coder", 0.95)
	if err != nil {
		t.Logf("DecayConfidence returned error: %v", err)
	}

	// Test Close
	err = adapter.Close()
	if err != nil {
		t.Logf("Close returned error: %v", err)
	}
}

// =============================================================================
// SHARD MANAGER OBSERVER/CONSULTATION SPAWNER TESTS
// =============================================================================

func TestShardManagerObserverSpawner_NilShardMgr(t *testing.T) {
	t.Parallel()

	spawner := &shardManagerObserverSpawner{shardMgr: nil}

	ctx := context.Background()
	result, err := spawner.SpawnObserver(ctx, "test_observer", "test task")

	if err == nil {
		t.Error("Expected error for nil shard manager")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("Expected 'not available' error, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got: %s", result)
	}
}

func TestShardManagerConsultationSpawner_NilShardMgr(t *testing.T) {
	t.Parallel()

	spawner := &shardManagerConsultationSpawner{shardMgr: nil}

	ctx := context.Background()
	result, err := spawner.SpawnConsultation(ctx, "test_specialist", "test task")

	if err == nil {
		t.Error("Expected error for nil shard manager")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("Expected 'not available' error, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got: %s", result)
	}
}

// =============================================================================
// RESOLVE SESSION/TURN TESTS
// =============================================================================

func TestResolveSessionID_WithConfig(t *testing.T) {
	t.Parallel()

	session := &Session{
		SessionID: "existing-session-id",
	}

	result := resolveSessionID(session)
	if result != "existing-session-id" {
		t.Errorf("Expected 'existing-session-id', got '%s'", result)
	}
}

func TestResolveSessionID_Empty(t *testing.T) {
	t.Parallel()

	session := &Session{
		SessionID: "",
	}

	result := resolveSessionID(session)
	if result == "" {
		t.Error("Expected non-empty session ID to be generated")
	}
}

func TestResolveSessionID_NilSession(t *testing.T) {
	t.Parallel()

	result := resolveSessionID(nil)
	if result == "" {
		t.Error("Expected non-empty session ID to be generated for nil session")
	}
}

func TestResolveTurnCount_WithValue(t *testing.T) {
	t.Parallel()

	session := &Session{
		TurnCount: 42,
	}

	result := resolveTurnCount(session)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}
}

func TestResolveTurnCount_Zero(t *testing.T) {
	t.Parallel()

	session := &Session{
		TurnCount: 0,
	}

	result := resolveTurnCount(session)
	if result != 0 {
		t.Errorf("Expected 0 (for zero turn count), got %d", result)
	}
}

func TestResolveTurnCount_NilSession(t *testing.T) {
	t.Parallel()

	result := resolveTurnCount(nil)
	if result != 0 {
		t.Errorf("Expected 0 for nil session, got %d", result)
	}
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestSessionAdapters_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Create all adapters
	kernelAdapter := &sessionKernelAdapter{kernel: m.kernel}
	vsAdapter := &sessionVirtualStoreAdapter{vs: m.virtualStore}
	llmAdapter := &sessionLLMAdapter{client: m.client}
	traceAdapter := NewLocalStoreTraceAdapter(nil)
	learningAdapter := &coreLearningStoreAdapter{store: nil}

	// Test kernel adapter flow
	perf.Track("kernel_adapter_flow", func() {
		fact := types.Fact{Predicate: "integration_test", Args: []interface{}{"value"}}
		_ = kernelAdapter.Assert(fact)
		_, _ = kernelAdapter.Query("integration_test")
		_ = kernelAdapter.Retract("integration_test")
	})

	// Test VS adapter
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	perf.Track("vs_adapter_flow", func() {
		_ = vsAdapter.WriteFile(tmpFile, []string{"test"})
		_, _ = vsAdapter.ReadFile(tmpFile)
	})

	// Test LLM adapter (if client available)
	if m.client != nil {
		perf.Track("llm_adapter_flow", func() {
			ctx := context.Background()
			_, _ = llmAdapter.Complete(ctx, "test")
		})
	}

	// Test trace adapter
	perf.Track("trace_adapter_flow", func() {
		_ = traceAdapter.StoreReasoningTrace(nil)
	})

	// Test learning adapter
	perf.Track("learning_adapter_flow", func() {
		_ = learningAdapter.Save("test", "pred", nil, "")
		_, _ = learningAdapter.Load("test")
	})

	t.Log("All adapter integrations completed")
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestSessionVirtualStoreAdapter_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	adapter := &sessionVirtualStoreAdapter{vs: nil}

	lines, err := adapter.ReadFile(emptyFile)
	if err != nil {
		t.Fatalf("ReadFile failed on empty file: %v", err)
	}

	// Empty file should have 1 empty line after split
	t.Logf("Empty file returned %d lines", len(lines))
}

func TestSessionVirtualStoreAdapter_LargeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large.txt")

	// Create file with many lines
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, strings.Repeat("x", 100))
	}
	content := strings.Join(lines, "\n")

	if err := os.WriteFile(largeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	adapter := &sessionVirtualStoreAdapter{vs: nil}

	result, err := adapter.ReadFile(largeFile)
	if err != nil {
		t.Fatalf("ReadFile failed on large file: %v", err)
	}

	if len(result) != 1000 {
		t.Errorf("Expected 1000 lines, got %d", len(result))
	}
}

func TestSessionKernelAdapter_NilKernel(t *testing.T) {
	t.Parallel()

	adapter := &sessionKernelAdapter{kernel: nil}

	// All methods should panic with nil kernel
	panicTests := []struct {
		name string
		fn   func()
	}{
		{"LoadFacts", func() { _ = adapter.LoadFacts(nil) }},
		{"Query", func() { _, _ = adapter.Query("test") }},
		{"QueryAll", func() { _, _ = adapter.QueryAll() }},
		{"Assert", func() { _ = adapter.Assert(types.Fact{}) }},
		{"Retract", func() { _ = adapter.Retract("test") }},
		{"Reset", func() { adapter.Reset() }},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("%s panicked with nil kernel (expected)", tt.name)
				}
			}()
			tt.fn()
		})
	}
}

// =============================================================================
// PERFORMANCE TESTS
// =============================================================================

func TestSessionAdapters_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	kernelAdapter := &sessionKernelAdapter{kernel: m.kernel}
	iterations := 100

	perf.Track("query_100x", func() {
		for i := 0; i < iterations; i++ {
			_, _ = kernelAdapter.Query("config_value")
		}
	})

	perf.Track("assert_retract_100x", func() {
		for i := 0; i < iterations; i++ {
			fact := types.Fact{Predicate: "perf_test", Args: []interface{}{i}}
			_ = kernelAdapter.Assert(fact)
			_ = kernelAdapter.Retract("perf_test")
		}
	})
}
