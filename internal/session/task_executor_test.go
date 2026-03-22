package session

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func TestJITExecutor_Execute_InlineExecution(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "Task complete"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "Task complete", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/fix", Category: "/coding"}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
	)

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
		DefaultSpawnerConfig(),
	)

	jitExec := NewJITExecutor(executor, spawner, mockTransducer)

	// Execute
	// "/fix" is NOT in complexIntents map in executor.go (checked previously: /research, /implement, /refactor, /campaign)
	// Wait, /fix maps to "coder".
	// Let's check needsSubagent in task_executor.go
	// complexIntents: /research, /implement, /refactor, /campaign

	result, err := jitExec.Execute(context.Background(), "/fix", "Fix the bug")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "Task complete" {
		t.Errorf("Expected 'Task complete', got '%s'", result)
	}
}

func TestJITExecutor_ExecuteWithContext_PreservesInlineIntent(t *testing.T) {
	var observedInput string

	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "review complete"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "review complete", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			observedInput = input
			return perception.Intent{Verb: "/review", Category: "/query"}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
	)
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
		DefaultSpawnerConfig(),
	)

	jitExec := NewJITExecutor(executor, spawner, mockTransducer)

	result, err := jitExec.ExecuteWithContext(context.Background(), "/review", "internal/core/shards/agents.go", nil, types.PriorityNormal)
	if err != nil {
		t.Fatalf("ExecuteWithContext failed: %v", err)
	}
	if result != "review complete" {
		t.Fatalf("expected review result, got %q", result)
	}
	if observedInput != "review internal/core/shards/agents.go" {
		t.Fatalf("expected inline input to preserve intent, got %q", observedInput)
	}
}

func TestJITExecutor_Execute_SubagentExecution(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "Research complete"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "Research complete", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/research", Category: "/knowledge"}, nil
		},
	}

	executor := createTestExecutor(t)

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		mockTransducer,
		DefaultSpawnerConfig(),
	)

	jitExec := NewJITExecutor(executor, spawner, mockTransducer)

	// Execute /research (triggers subagent)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := jitExec.Execute(ctx, "/research", "Research this topic")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "Research complete" {
		t.Errorf("Expected 'Research complete', got '%s'", result)
	}
}

func TestJITExecutor_ExecuteAsync(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return &types.LLMToolResponse{Text: "Async result"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return "Async result", nil
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	// Execute Async
	taskID, err := jitExec.ExecuteAsync(context.Background(), "/test", "Run tests")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	if taskID == "" {
		t.Error("Expected taskID")
	}

	// Wait for result
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := jitExec.WaitForResult(ctx, taskID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}

	if result != "Async result" {
		t.Errorf("Expected 'Async result', got '%s'", result)
	}

	// Check GetResult after completion
	res, done, err := jitExec.GetResult(taskID)
	if !done {
		t.Error("Expected done=true")
	}
	if res != "Async result" {
		t.Errorf("Expected 'Async result', got '%s'", res)
	}
}

// TODO: TEST_GAP: Null/Undefined/Empty Inputs
// 1. Empty `intent` and `task` in `Execute` / `ExecuteWithContext`.
// 2. Empty `taskID` passed to `GetResult`.
// 3. Passing a `nil` `context.Context` to `Execute` or `WaitForResult`.
// 4. Passing a `nil` `sessionCtx` in `executeAsyncInternal`.
func TestJITExecutor_Execute_NullEmptyInputs(t *testing.T) {
	// Add test coverage for empty and nil inputs here.
}

// TODO: TEST_GAP: Type Coercion & Unexpected Formats
// 1. `intent` without a leading slash, multiple slashes, or missing characters.
// 2. Massive whitespace payloads for `task`.
// 3. Binary or malformed UTF-8 in `task` strings.
func TestJITExecutor_Execute_TypeCoercion(t *testing.T) {
	// Add test coverage for format manipulation here.
}

// TODO: TEST_GAP: User Request Extremes and Load
// 1. Extreme context sizes directly passed to `task`.
// 2. 10,000+ concurrent rapid-fire `ExecuteAsync` calls (Load/DDoS).
// 3. Canceled context on `ExecuteAsync` spawn request.
func TestJITExecutor_Execute_UserRequestExtremes(t *testing.T) {
	// Add test coverage for execution load and context extreme behaviors here.
}

// TODO: TEST_GAP: State Conflicts & Race Conditions
// 1. Concurrent modification of `j.executor.SetSessionContext(sessionCtx)` bleed.
// 2. Infinite block in `WaitForResult` missing a context timeout.
// 3. Unbounded map growth memory leak in `j.results` over numerous calls.
// 4. TOCTOU condition in `GetResult` checking agent state vs result state.
// 5. Very fast synchronous completion beating the `j.mu.Lock()` map assignment in `ExecuteAsync`.
func TestJITExecutor_Execute_StateConflicts(t *testing.T) {
	// Add test coverage for concurrency map races and cache leakages here.
}
