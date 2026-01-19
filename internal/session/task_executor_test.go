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
