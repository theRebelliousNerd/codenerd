// Package perception implements LLM interaction and tracing for codeNERD.
// This file contains comprehensive tests for TracingLLMClient, ShardTraceStore, and SystemLLMContext.
package perception

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// MOCK IMPLEMENTATIONS
// =============================================================================

// mockTraceStore implements TraceStore for testing.
type mockTraceStore struct {
	mu     sync.Mutex
	traces []*ReasoningTrace
}

func (m *mockTraceStore) StoreReasoningTrace(trace *ReasoningTrace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = append(m.traces, trace)
	return nil
}

func (m *mockTraceStore) getTraces() []*ReasoningTrace {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.traces
}

// mockShardTraceReader implements ShardTraceReader for testing.
type mockShardTraceReader struct {
	traces      []ReasoningTrace
	failedOnly  []ReasoningTrace
	highQuality []ReasoningTrace
}

func (m *mockShardTraceReader) GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	if limit > len(m.traces) {
		limit = len(m.traces)
	}
	return m.traces[:limit], nil
}

func (m *mockShardTraceReader) GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error) {
	if limit > len(m.failedOnly) {
		limit = len(m.failedOnly)
	}
	return m.failedOnly[:limit], nil
}

func (m *mockShardTraceReader) GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error) {
	if limit > len(m.traces) {
		limit = len(m.traces)
	}
	return m.traces[:limit], nil
}

func (m *mockShardTraceReader) GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error) {
	if limit > len(m.highQuality) {
		limit = len(m.highQuality)
	}
	return m.highQuality[:limit], nil
}

// mockTracingLLMClient is a simple LLM client for testing.
type mockTracingLLMClient struct {
	response string
	err      error
}

func (m *mockTracingLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

func (m *mockTracingLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, m.err
}

func (m *mockTracingLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: m.response}, m.err
}

// =============================================================================
// REASONING TRACE STRUCT TESTS
// =============================================================================

func TestReasoningTrace_Struct(t *testing.T) {
	now := time.Now()
	trace := ReasoningTrace{
		ID:            "trace_123",
		ShardID:       "shard_1",
		ShardType:     "coder",
		ShardCategory: "ephemeral",
		SessionID:     "session_abc",
		TaskContext:   "implement feature X",
		SystemPrompt:  "You are a coding assistant.",
		UserPrompt:    "Write a function to...",
		Response:      "Here is the function...",
		Model:         "gemini-2.0-flash",
		TokensUsed:    1500,
		DurationMs:    250,
		Success:       true,
		ErrorMessage:  "",
		QualityScore:  0.85,
		LearningNotes: []string{"Good response structure"},
		Timestamp:     now,
	}

	if trace.ID != "trace_123" {
		t.Errorf("ID = %q, want %q", trace.ID, "trace_123")
	}
	if trace.ShardID != "shard_1" {
		t.Errorf("ShardID = %q, want %q", trace.ShardID, "shard_1")
	}
	if trace.ShardType != "coder" {
		t.Errorf("ShardType = %q, want %q", trace.ShardType, "coder")
	}
	if trace.ShardCategory != "ephemeral" {
		t.Errorf("ShardCategory = %q, want %q", trace.ShardCategory, "ephemeral")
	}
	if trace.SessionID != "session_abc" {
		t.Errorf("SessionID = %q, want %q", trace.SessionID, "session_abc")
	}
	if trace.TaskContext != "implement feature X" {
		t.Errorf("TaskContext = %q, want %q", trace.TaskContext, "implement feature X")
	}
	if trace.SystemPrompt != "You are a coding assistant." {
		t.Errorf("SystemPrompt = %q, want %q", trace.SystemPrompt, "You are a coding assistant.")
	}
	if trace.UserPrompt != "Write a function to..." {
		t.Errorf("UserPrompt = %q, want %q", trace.UserPrompt, "Write a function to...")
	}
	if trace.Response != "Here is the function..." {
		t.Errorf("Response = %q, want %q", trace.Response, "Here is the function...")
	}
	if trace.Model != "gemini-2.0-flash" {
		t.Errorf("Model = %q, want %q", trace.Model, "gemini-2.0-flash")
	}
	if trace.TokensUsed != 1500 {
		t.Errorf("TokensUsed = %d, want 1500", trace.TokensUsed)
	}
	if trace.DurationMs != 250 {
		t.Errorf("DurationMs = %d, want 250", trace.DurationMs)
	}
	if !trace.Success {
		t.Error("Expected Success=true")
	}
	if trace.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty", trace.ErrorMessage)
	}
	if trace.QualityScore != 0.85 {
		t.Errorf("QualityScore = %f, want 0.85", trace.QualityScore)
	}
	if len(trace.LearningNotes) != 1 || trace.LearningNotes[0] != "Good response structure" {
		t.Errorf("LearningNotes = %v, want [Good response structure]", trace.LearningNotes)
	}
	if trace.Timestamp != now {
		t.Errorf("Timestamp mismatch")
	}
}

// =============================================================================
// TRACING LLM CLIENT TESTS
// =============================================================================

func TestNewTracingLLMClient(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	store := &mockTraceStore{}

	client := NewTracingLLMClient(underlying, store)

	if client == nil {
		t.Fatal("NewTracingLLMClient returned nil")
	}
	if client.underlying != underlying {
		t.Error("underlying client not set correctly")
	}
	if client.store != store {
		t.Error("store not set correctly")
	}
}

func TestTracingLLMClient_SetShardContext(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	client := NewTracingLLMClient(underlying, nil)

	client.SetShardContext("shard_1", "coder", "ephemeral", "session_abc", "task context")

	shardID, shardType, shardCategory, sessionID, taskContext := client.GetCurrentContext()

	if shardID != "shard_1" {
		t.Errorf("shardID = %q, want %q", shardID, "shard_1")
	}
	if shardType != "coder" {
		t.Errorf("shardType = %q, want %q", shardType, "coder")
	}
	if shardCategory != "ephemeral" {
		t.Errorf("shardCategory = %q, want %q", shardCategory, "ephemeral")
	}
	if sessionID != "session_abc" {
		t.Errorf("sessionID = %q, want %q", sessionID, "session_abc")
	}
	if taskContext != "task context" {
		t.Errorf("taskContext = %q, want %q", taskContext, "task context")
	}
}

func TestTracingLLMClient_ClearShardContext(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	client := NewTracingLLMClient(underlying, nil)

	client.SetShardContext("shard_1", "coder", "ephemeral", "session_abc", "task")
	client.ClearShardContext()

	shardID, shardType, shardCategory, _, taskContext := client.GetCurrentContext()

	if shardID != "" {
		t.Errorf("shardID = %q, want empty", shardID)
	}
	if shardType != "" {
		t.Errorf("shardType = %q, want empty", shardType)
	}
	if shardCategory != "" {
		t.Errorf("shardCategory = %q, want empty", shardCategory)
	}
	if taskContext != "" {
		t.Errorf("taskContext = %q, want empty", taskContext)
	}
}

func TestTracingLLMClient_Complete(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test response"}
	store := &mockTraceStore{}
	client := NewTracingLLMClient(underlying, store)

	response, err := client.Complete(context.Background(), "test prompt")

	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if response != "test response" {
		t.Errorf("response = %q, want %q", response, "test response")
	}

	// Wait for async trace storage
	time.Sleep(50 * time.Millisecond)

	traces := store.getTraces()
	if len(traces) != 1 {
		t.Errorf("Expected 1 trace, got %d", len(traces))
	}
}

func TestTracingLLMClient_CompleteWithSystem(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "system response"}
	store := &mockTraceStore{}
	client := NewTracingLLMClient(underlying, store)

	client.SetShardContext("shard_test", "tester", "ephemeral", "session_1", "test task")

	response, err := client.CompleteWithSystem(context.Background(), "system prompt", "user prompt")

	if err != nil {
		t.Fatalf("CompleteWithSystem error: %v", err)
	}
	if response != "system response" {
		t.Errorf("response = %q, want %q", response, "system response")
	}

	// Wait for async trace storage
	time.Sleep(50 * time.Millisecond)

	traces := store.getTraces()
	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace, got %d", len(traces))
	}

	trace := traces[0]
	if trace.ShardID != "shard_test" {
		t.Errorf("ShardID = %q, want %q", trace.ShardID, "shard_test")
	}
	if trace.ShardType != "tester" {
		t.Errorf("ShardType = %q, want %q", trace.ShardType, "tester")
	}
	if trace.SystemPrompt != "system prompt" {
		t.Errorf("SystemPrompt = %q, want %q", trace.SystemPrompt, "system prompt")
	}
	if trace.UserPrompt != "user prompt" {
		t.Errorf("UserPrompt = %q, want %q", trace.UserPrompt, "user prompt")
	}
	if !trace.Success {
		t.Error("Expected Success=true")
	}
}

func TestTracingLLMClient_CompleteWithSystem_Error(t *testing.T) {
	testErr := errors.New("LLM error")
	underlying := &mockTracingLLMClient{err: testErr}
	store := &mockTraceStore{}
	client := NewTracingLLMClient(underlying, store)

	_, err := client.CompleteWithSystem(context.Background(), "system", "user")

	if !errors.Is(err, testErr) {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	// Wait for async trace storage
	time.Sleep(50 * time.Millisecond)

	traces := store.getTraces()
	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace, got %d", len(traces))
	}

	if traces[0].Success {
		t.Error("Expected Success=false for error case")
	}
	if traces[0].ErrorMessage == "" {
		t.Error("Expected ErrorMessage to be set")
	}
}

func TestTracingLLMClient_CompleteWithTools(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "tool response"}
	store := &mockTraceStore{}
	client := NewTracingLLMClient(underlying, store)

	client.SetShardContext("shard_tools", "tool_user", "specialist", "session_2", "use tools")

	tools := []ToolDefinition{
		{Name: "test_tool", Description: "A test tool"},
	}

	response, err := client.CompleteWithTools(context.Background(), "system", "user", tools)

	if err != nil {
		t.Fatalf("CompleteWithTools error: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.Text != "tool response" {
		t.Errorf("response.Text = %q, want %q", response.Text, "tool response")
	}

	// Wait for async trace storage
	time.Sleep(50 * time.Millisecond)

	traces := store.getTraces()
	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace, got %d", len(traces))
	}
}

func TestTracingLLMClient_GetUnderlying(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	client := NewTracingLLMClient(underlying, nil)

	result := client.GetUnderlying()

	if result != underlying {
		t.Error("GetUnderlying did not return underlying client")
	}
}

func TestTracingLLMClient_ConcurrentContext(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	client := NewTracingLLMClient(underlying, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			shardID := "shard_" + string(rune('A'+n))
			client.SetShardContext(shardID, "type", "cat", "sess", "task")
			// Small sleep to interleave
			time.Sleep(time.Millisecond)
			_, _, _, _, _ = client.GetCurrentContext()
			client.ClearShardContext()
		}(i)
	}
	wg.Wait()

	// Just verify no race/deadlock occurred
}

// =============================================================================
// SHARD TRACE STORE TESTS
// =============================================================================

func TestNewShardTraceStore(t *testing.T) {
	reader := &mockShardTraceReader{}
	store := NewShardTraceStore(reader, "coder")

	if store == nil {
		t.Fatal("NewShardTraceStore returned nil")
	}
	if store.shardType != "coder" {
		t.Errorf("shardType = %q, want %q", store.shardType, "coder")
	}
}

func TestShardTraceStore_GetMyTraces(t *testing.T) {
	reader := &mockShardTraceReader{
		traces: []ReasoningTrace{
			{ID: "trace_1", ShardType: "coder"},
			{ID: "trace_2", ShardType: "coder"},
			{ID: "trace_3", ShardType: "coder"},
		},
	}
	store := NewShardTraceStore(reader, "coder")

	traces, err := store.GetMyTraces(2)

	if err != nil {
		t.Fatalf("GetMyTraces error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("Expected 2 traces, got %d", len(traces))
	}
}

func TestShardTraceStore_GetMyTraces_NilStore(t *testing.T) {
	store := NewShardTraceStore(nil, "coder")

	traces, err := store.GetMyTraces(10)

	if err != nil {
		t.Fatalf("GetMyTraces error: %v", err)
	}
	if traces != nil {
		t.Error("Expected nil traces for nil store")
	}
}

func TestShardTraceStore_GetMyFailedTraces(t *testing.T) {
	reader := &mockShardTraceReader{
		failedOnly: []ReasoningTrace{
			{ID: "fail_1", Success: false},
			{ID: "fail_2", Success: false},
		},
	}
	store := NewShardTraceStore(reader, "coder")

	traces, err := store.GetMyFailedTraces(10)

	if err != nil {
		t.Fatalf("GetMyFailedTraces error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("Expected 2 failed traces, got %d", len(traces))
	}
}

func TestShardTraceStore_GetSimilarTasks(t *testing.T) {
	reader := &mockShardTraceReader{
		traces: []ReasoningTrace{
			{ID: "task_1", TaskContext: "implement foo"},
			{ID: "task_2", TaskContext: "implement bar"},
		},
	}
	store := NewShardTraceStore(reader, "coder")

	traces, err := store.GetSimilarTasks("implement", 10)

	if err != nil {
		t.Fatalf("GetSimilarTasks error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("Expected 2 traces, got %d", len(traces))
	}
}

func TestShardTraceStore_GetSuccessfulPatterns(t *testing.T) {
	reader := &mockShardTraceReader{
		highQuality: []ReasoningTrace{
			{ID: "hq_1", QualityScore: 0.9, Success: true},
			{ID: "hq_2", QualityScore: 0.85, Success: true},
		},
	}
	store := NewShardTraceStore(reader, "coder")

	traces, err := store.GetSuccessfulPatterns(10)

	if err != nil {
		t.Fatalf("GetSuccessfulPatterns error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("Expected 2 high-quality traces, got %d", len(traces))
	}
}

// =============================================================================
// EXTRACT LEARNINGS TESTS
// =============================================================================

func TestExtractLearnings_NoTraces(t *testing.T) {
	learnings := ExtractLearnings(nil)

	if len(learnings) != 0 {
		t.Errorf("Expected 0 learnings for nil traces, got %d", len(learnings))
	}

	learnings = ExtractLearnings([]ReasoningTrace{})

	if len(learnings) != 0 {
		t.Errorf("Expected 0 learnings for empty traces, got %d", len(learnings))
	}
}

func TestExtractLearnings_SuccessfulTraces(t *testing.T) {
	traces := []ReasoningTrace{
		{
			ID:           "trace_1",
			Success:      true,
			QualityScore: 0.9,
			DurationMs:   100,
			TaskContext:  "implement feature",
			Response:     "High quality response here",
		},
		{
			ID:           "trace_2",
			Success:      true,
			QualityScore: 0.85,
			DurationMs:   150,
			TaskContext:  "write tests",
			Response:     "Another good response",
		},
	}

	learnings := ExtractLearnings(traces)

	if len(learnings) == 0 {
		t.Error("Expected learnings from successful traces")
	}

	// Should include stats
	foundStats := false
	for _, l := range learnings {
		if len(l) > 5 && l[:5] == "STATS" {
			foundStats = true
		}
	}
	if !foundStats {
		t.Error("Expected STATS learning")
	}
}

func TestExtractLearnings_FailedTraces(t *testing.T) {
	traces := []ReasoningTrace{
		{
			ID:           "trace_1",
			Success:      false,
			DurationMs:   100,
			TaskContext:  "complex task",
			ErrorMessage: "context deadline exceeded",
		},
	}

	learnings := ExtractLearnings(traces)

	if len(learnings) == 0 {
		t.Error("Expected learnings from failed traces")
	}

	// Should include AVOID learning
	foundAvoid := false
	for _, l := range learnings {
		if len(l) > 5 && l[:5] == "AVOID" {
			foundAvoid = true
		}
	}
	if !foundAvoid {
		t.Error("Expected AVOID learning from failure")
	}
}

func TestExtractLearnings_MixedTraces(t *testing.T) {
	traces := []ReasoningTrace{
		{ID: "1", Success: true, QualityScore: 0.9, DurationMs: 100, Response: "Good"},
		{ID: "2", Success: false, DurationMs: 200, ErrorMessage: "timeout", TaskContext: "task2"},
		{ID: "3", Success: true, QualityScore: 0.5, DurationMs: 150},
		{ID: "4", Success: true, QualityScore: 0.85, DurationMs: 120, Response: "Another good one"},
	}

	learnings := ExtractLearnings(traces)

	if len(learnings) < 2 {
		t.Errorf("Expected at least 2 learnings, got %d", len(learnings))
	}
}

// =============================================================================
// CATEGORY FROM SHARD TYPE TESTS
// =============================================================================

func TestCategoryFromShardType_System(t *testing.T) {
	systemTypes := []string{
		"perception_firewall",
		"constitution_gate",
		"executive_policy",
		"cost_guard",
	}

	for _, typeName := range systemTypes {
		t.Run(typeName, func(t *testing.T) {
			category := CategoryFromShardType(typeName)
			if category != "system" {
				t.Errorf("CategoryFromShardType(%q) = %q, want %q", typeName, category, "system")
			}
		})
	}
}

func TestCategoryFromShardType_Ephemeral(t *testing.T) {
	ephemeralTypes := []string{
		"coder",
		"tester",
		"reviewer",
		"researcher",
	}

	for _, typeName := range ephemeralTypes {
		t.Run(typeName, func(t *testing.T) {
			category := CategoryFromShardType(typeName)
			if category != "ephemeral" {
				t.Errorf("CategoryFromShardType(%q) = %q, want %q", typeName, category, "ephemeral")
			}
		})
	}
}

func TestCategoryFromShardType_Specialist(t *testing.T) {
	specialistTypes := []string{
		"custom_analyzer",
		"domain_expert",
		"user_created_agent",
		"unknown_type",
	}

	for _, typeName := range specialistTypes {
		t.Run(typeName, func(t *testing.T) {
			category := CategoryFromShardType(typeName)
			if category != "specialist" {
				t.Errorf("CategoryFromShardType(%q) = %q, want %q", typeName, category, "specialist")
			}
		})
	}
}

// =============================================================================
// SYSTEM LLM CONTEXT TESTS
// =============================================================================

func TestNewSystemLLMContext(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}

	ctx := NewSystemLLMContext(underlying, "compressor", "context-compression")

	if ctx == nil {
		t.Fatal("NewSystemLLMContext returned nil")
	}
	if ctx.componentID != "compressor" {
		t.Errorf("componentID = %q, want %q", ctx.componentID, "compressor")
	}
	if ctx.taskContext != "context-compression" {
		t.Errorf("taskContext = %q, want %q", ctx.taskContext, "context-compression")
	}
	if ctx.sessionID == "" {
		t.Error("Expected sessionID to be set")
	}
}

func TestSystemLLMContext_Complete(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "completed"}

	sysCtx := NewSystemLLMContext(underlying, "test", "task")
	defer sysCtx.Clear()

	response, err := sysCtx.Complete(context.Background(), "prompt")

	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if response != "completed" {
		t.Errorf("response = %q, want %q", response, "completed")
	}
}

func TestSystemLLMContext_CompleteWithSystem(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "with system"}

	sysCtx := NewSystemLLMContext(underlying, "transducer", "intent-extraction")
	defer sysCtx.Clear()

	response, err := sysCtx.CompleteWithSystem(context.Background(), "system", "user")

	if err != nil {
		t.Fatalf("CompleteWithSystem error: %v", err)
	}
	if response != "with system" {
		t.Errorf("response = %q, want %q", response, "with system")
	}
}

func TestSystemLLMContext_CompleteWithTools(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "with tools"}

	sysCtx := NewSystemLLMContext(underlying, "orchestrator", "tool-selection")
	defer sysCtx.Clear()

	tools := []ToolDefinition{{Name: "tool1"}}
	response, err := sysCtx.CompleteWithTools(context.Background(), "system", "user", tools)

	if err != nil {
		t.Fatalf("CompleteWithTools error: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.Text != "with tools" {
		t.Errorf("response.Text = %q, want %q", response.Text, "with tools")
	}
}

func TestSystemLLMContext_WithTracingClient(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "traced"}
	store := &mockTraceStore{}
	tracingClient := NewTracingLLMClient(underlying, store)

	sysCtx := NewSystemLLMContext(tracingClient, "feedback_loop", "learning-update")
	defer sysCtx.Clear()

	// The context should have set shard context on the tracing client
	shardID, shardType, shardCategory, _, _ := tracingClient.GetCurrentContext()

	if shardType != "feedback_loop" {
		t.Errorf("shardType = %q, want %q", shardType, "feedback_loop")
	}
	if shardCategory != "system" {
		t.Errorf("shardCategory = %q, want %q", shardCategory, "system")
	}
	if shardID == "" {
		t.Error("Expected shardID to be set")
	}
}

func TestSystemLLMContext_ClearWithTracingClient(t *testing.T) {
	underlying := &mockTracingLLMClient{response: "test"}
	tracingClient := NewTracingLLMClient(underlying, nil)

	sysCtx := NewSystemLLMContext(tracingClient, "test", "task")

	// Verify context was set
	_, shardType, _, _, _ := tracingClient.GetCurrentContext()
	if shardType != "test" {
		t.Errorf("shardType before clear = %q, want %q", shardType, "test")
	}

	sysCtx.Clear()

	// Verify context was cleared
	_, shardType, _, _, _ = tracingClient.GetCurrentContext()
	if shardType != "" {
		t.Errorf("shardType after clear = %q, want empty", shardType)
	}
}

// =============================================================================
// SUMMARIZE APPROACH TESTS
// =============================================================================

func TestSummarizeApproach(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantLen  int
	}{
		{
			name:     "short response",
			response: "Short response",
			wantLen:  14,
		},
		{
			name:     "long response truncated",
			response: string(make([]byte, 300)),
			wantLen:  203, // 200 + "..."
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := ReasoningTrace{Response: tt.response}
			result := summarizeApproach(trace)

			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}
