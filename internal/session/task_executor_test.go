package session

import (
	"context"
	"strings"
	"sync"
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

	result, err := jitExec.Execute(context.Background(), TaskRequest{IntentVerb: "/fix", Task: "Fix the bug"})
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

	result, err := jitExec.ExecuteWithContext(context.Background(), TaskRequest{IntentVerb: "/review", Task: "internal/core/shards/agents.go"}, nil, types.PriorityNormal)
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

	result, err := jitExec.Execute(ctx, TaskRequest{IntentVerb: "/research", Task: "Research this topic"})
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
	taskID, err := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/test", Task: "Run tests"})
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

func TestJITExecutor_NullUndefinedEmpty(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "processed", nil
		},
	}
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{},
		&MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig(),
	)
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	// 1. Empty task and intent
	_, err := jitExec.Execute(context.Background(), TaskRequest{})
	if err != nil {
		// Might fail down the line, but shouldn't panic — and with the new
		// strict intent validation, empty intent IS expected to fail.
		t.Logf("Empty execute returned: %v", err)
	}

	// 2. sessionCtx is a non-nil pointer to empty struct
	emptyCtx := &types.SessionContext{}
	_, err = jitExec.ExecuteWithContext(context.Background(), TaskRequest{}, emptyCtx, types.PriorityNormal)
	if err != nil {
		t.Logf("Empty context execute returned: %v", err)
	}

	// 3. GetResult and WaitForResult with empty taskID
	_, ok, err := jitExec.GetResult("   ")
	if ok || err == nil {
		t.Error("Expected error for empty/whitespace taskID in GetResult")
	}
	// Note: WaitForResult blocks if task doesn't exist?
	// WaitForResult loops calling GetResult. GetResult returns error if not found.
	// Let's verify it returns error quickly
	_, err = jitExec.WaitForResult(context.Background(), "   ")
	if err == nil {
		t.Error("Expected error for empty/whitespace taskID in WaitForResult")
	}

	// 4. nil context handling
	_, err = jitExec.WaitForResult(nil, "some-id")
	if err == nil {
		t.Error("Expected error when passing nil context")
	}
}
func TestJITExecutor_TypeCoercion(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "processed", nil
		},
	}
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{},
		&MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig(),
	)
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	// 1. Malformed prefixes, multiple slashes, invalid Unicode in intent.
	// With the strict TaskRequest validation, intents that don't start with
	// "/" are rejected — that's the intended behavior; we only check that
	// the validation doesn't panic on garbage input.
	intents := []string{"///refactor", "\\x80\\x81\\x82", "/   /test", "invalid \xff"}
	for _, intent := range intents {
		_, _ = jitExec.Execute(context.Background(), TaskRequest{IntentVerb: intent, Task: "task"})
	}

	// 2. Massive whitespace, binary/malformed UTF-8 in task strings
	var massiveWhitespace strings.Builder
	for range 10000 {
		massiveWhitespace.WriteString(" \t\n")
	}
	massiveWhitespace.WriteString("real task")

	tasks := []string{
		massiveWhitespace.String(),
		"\x00\x01\x02\xff\xfe",
	}
	for _, task := range tasks {
		_, err := jitExec.Execute(context.Background(), TaskRequest{IntentVerb: "/fix", Task: task})
		if err != nil {
			t.Errorf("Execute failed on malformed task: %v", err)
		}
	}
}
func TestJITExecutor_UserRequestExtremes(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			time.Sleep(10 * time.Millisecond) // Slow down to ensure exhaustion is hit
			return "processed", nil
		},
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			time.Sleep(10 * time.Millisecond) // Slow down to ensure exhaustion is hit
			return &types.LLMToolResponse{Text: "processed"}, nil
		},
	}
	spawnerConfig := DefaultSpawnerConfig()
	spawnerConfig.MaxActiveSubagents = 50 // Keep limit small for tests

	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{},
		&MockConfigFactory{}, &MockTransducer{}, spawnerConfig,
	)
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	// 1. Massive task payload (e.g., 5MB for speed, testing memory behavior)
	var massiveTask strings.Builder
	for range 50000 {
		massiveTask.WriteString("This is a very long string used to simulate a massive task payload from the user. ")
	}
	_, err := jitExec.Execute(context.Background(), TaskRequest{IntentVerb: "/fix", Task: massiveTask.String()})
	if err != nil {
		t.Errorf("Failed with massive task: %v", err)
	}

	// 2. 1,000+ concurrent ExecuteAsync calls and Spawner exhaustion
	var wg sync.WaitGroup
	errCount := 0
	var errMu sync.Mutex

	for i := range 1000 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, e := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/research", Task: "test"})
			if e != nil {
				errMu.Lock()
				errCount++
				errMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	t.Logf("errCount after 1000 async calls: %d", errCount)
	if errCount == 0 {
		t.Error("Expected spawner to reject requests when MaxActiveSubagents is exhausted")
	}

	// 3. Extreme priority values
	_, err = jitExec.ExecuteWithContext(context.Background(), TaskRequest{IntentVerb: "/fix", Task: "task"}, &types.SessionContext{}, types.SpawnPriority(-2147483648))
	if err != nil {
		t.Errorf("Failed with min priority: %v", err)
	}
	_, err = jitExec.ExecuteWithContext(context.Background(), TaskRequest{IntentVerb: "/fix", Task: "task"}, &types.SessionContext{}, types.SpawnPriority(2147483647))
	if err != nil {
		t.Errorf("Failed with max priority: %v", err)
	}
}
func TestJITExecutor_StateConflicts(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			time.Sleep(5 * time.Millisecond) // Simulate work
			return "processed", nil
		},
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			time.Sleep(5 * time.Millisecond)
			return &types.LLMToolResponse{Text: "processed"}, nil
		},
	}

	spawnerConfig := DefaultSpawnerConfig()
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, spawnerConfig)
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	t.Run("ExecuteWithContext Data Races", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range 50 {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// /fix is an inline intent (needsSubagent is false)
				ctx := &types.SessionContext{DreamMode: false}
				_, _ = jitExec.ExecuteWithContext(context.Background(), TaskRequest{IntentVerb: "/fix", Task: "task"}, ctx, types.PriorityNormal)
			}(i)
		}
		wg.Wait()
	})

	t.Run("WaitForResult Context Cancellation", func(t *testing.T) {
		// Spawn a subagent that will take some time
		taskID, err := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/research", Task: "long task"})
		if err != nil {
			t.Fatalf("ExecuteAsync failed: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		var waitErr error
		var wg sync.WaitGroup
		wg.Go(func() {
			_, waitErr = jitExec.WaitForResult(ctx, taskID)
		})

		// Cancel immediately
		cancel()
		wg.Wait()

		if waitErr != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", waitErr)
		}
	})

	t.Run("Results Map Thread Safety", func(t *testing.T) {
		var wg sync.WaitGroup

		for range 50 {
			wg.Go(func() {
				taskID, _ := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/research", Task: "test task"})

				// Concurrently poll GetResult and WaitForResult
				var innerWg sync.WaitGroup
				innerWg.Add(2)
				go func() {
					defer innerWg.Done()
					_, _, _ = jitExec.GetResult(taskID)
				}()
				go func() {
					defer innerWg.Done()
					// Small timeout to not block tests
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					defer cancel()
					_, _ = jitExec.WaitForResult(ctx, taskID)
				}()
				innerWg.Wait()
			})
		}
		wg.Wait()
	})
}
