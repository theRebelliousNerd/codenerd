//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// --- Mocks ---

// mockKernelQuerier provides stable facts for compilation.
type mockKernelQuerier struct {
	facts []prompt.Fact
}

func (m *mockKernelQuerier) Query(pred string) ([]prompt.Fact, error) {
	var res []prompt.Fact
	for _, f := range m.facts {
		if f.Predicate == pred {
			res = append(res, f)
		}
	}
	return res, nil
}
func (m *mockKernelQuerier) AssertBatch(facts []any) error { return nil }

// mockLLMClientPC simulates network conditions and streaming behavior.
type mockLLMClientPC struct {
	mu           sync.Mutex
	delay        time.Duration
	streamData   []string
	recordedReqs []string
	errToReturn  error
	leakCheck    int32
}

func (m *mockLLMClientPC) Complete(ctx context.Context, p string) (string, error) { return "", nil }

func (m *mockLLMClientPC) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.Lock()
	m.recordedReqs = append(m.recordedReqs, systemPrompt)
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(m.delay):
		}
	}
	return "mock response", m.errToReturn
}

func (m *mockLLMClientPC) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	atomic.AddInt32(&m.leakCheck, 1)

	ch := make(chan string, 10)
	errCh := make(chan error, 1)

	go func() {
		defer atomic.AddInt32(&m.leakCheck, -1)
		defer close(ch)
		defer close(errCh)

		for _, chunk := range m.streamData {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case <-time.After(m.delay):
				ch <- chunk
			}
		}
		if m.errToReturn != nil {
			errCh <- m.errToReturn
		}
	}()
	return ch, errCh
}

func (m *mockLLMClientPC) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

// --- Tests ---

func TestE2E_PromptCompiler_LLMClient_Smoke(t *testing.T) {
	t.Parallel()
	t.Log("Smoke test: Compiler creates a prompt, BudgetManager allocates, LLMClient executes.")

	kq := &mockKernelQuerier{facts: []prompt.Fact{{Predicate: "test_fact", Args: []any{"data"}}}}
	client := &mockLLMClientPC{}

	// Real Prompt Compiler
	compiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(kq))
	if err != nil {
		t.Fatalf("Failed to initialize compiler: %v", err)
	}

	compCtx := &prompt.CompilationContext{
		OperationalMode: "/active",
		UserIntent:      "test_intent",
		TokenBudget:     100000,
	}

	result, err := compiler.Compile(context.Background(), compCtx)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if result == nil {
		t.Fatalf("Compiler returned empty prompt")
	}

	// Pass output to LLMClient
	resp, err := client.CompleteWithSystem(context.Background(), result.Prompt, "User question")
	if err != nil {
		t.Fatalf("LLM execution failed: %v", err)
	}

	if resp == "" {
		t.Fatalf("Expected response from LLM client")
	}
}

func TestE2E_PromptCompiler_LLMClient_BudgetMiscalculation(t *testing.T) {
	t.Parallel()
	t.Log("Scenario 1: BudgetManager underestimates tokens, LLM rejects.")

	client := &mockLLMClientPC{errToReturn: errors.New("HTTP 400: Context Length Exceeded")}
	compiler, _ := prompt.NewJITPromptCompiler()

	compCtx := &prompt.CompilationContext{
		OperationalMode: "/active",
		TokenBudget:     100000,
	}

	result, err := compiler.Compile(context.Background(), compCtx)
	if err != nil {
		t.Errorf("Compile error: %v", err)
	}
	if result == nil {
		t.Errorf("Result is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.CompleteWithSystem(ctx, result.Prompt, "prompt")
	if err == nil || !strings.Contains(err.Error(), "Context Length Exceeded") {
		t.Fatalf("Expected 400 rejection from client, got: %v", err)
	}
	t.Log("Successfully propagated 400 error across boundary.")
}

func TestE2E_PromptCompiler_LLMClient_StreamGoroutineLeak(t *testing.T) {
	t.Parallel()
	t.Log("Scenario 3: Context cancellation cleanly exits streaming goroutine.")

	client := &mockLLMClientPC{
		delay:      50 * time.Millisecond,
		streamData: []string{"a", "b", "c", "d", "e"},
	}

	ctx, cancel := context.WithCancel(context.Background())

	compiler, _ := prompt.NewJITPromptCompiler()
	compCtx := &prompt.CompilationContext{OperationalMode: "/active", TokenBudget: 100000}
	result, err := compiler.Compile(context.Background(), compCtx)
	if err != nil {
		t.Errorf("Compile error: %v", err)
	}
	if result == nil {
		t.Errorf("Result is nil")
	}

	client.CompleteWithStreaming(ctx, result.Prompt, "usr", false)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&client.leakCheck) != 0 {
		t.Fatalf("Stream goroutine leaked after context cancellation")
	}
}

func TestE2E_PromptCompiler_LLMClient_ContextImmutability(t *testing.T) {
	t.Parallel()
	t.Log("Scenario 4: LLMClient must not mutate the request context array.")

	compiler, _ := prompt.NewJITPromptCompiler()
	compCtx := &prompt.CompilationContext{OperationalMode: "/active", TokenBudget: 100000}
	result, err := compiler.Compile(context.Background(), compCtx)
	if err != nil {
		t.Errorf("Compile error: %v", err)
	}
	if result == nil {
		t.Errorf("Result is nil")
	}

	originalMsg := result.Prompt

	client := &mockLLMClientPC{}
	_, _ = client.CompleteWithSystem(context.Background(), originalMsg, "usr")

	if len(client.recordedReqs) > 0 {
		recorded := client.recordedReqs[0]
		if recorded != originalMsg {
			t.Fatalf("Client mutated the message.")
		}
	}
}

func TestE2E_PromptCompiler_LLMClient_MassiveConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency exhaustion in short mode")
	}
	t.Parallel()
	t.Log("Scenario 6: Spawn 100 concurrent compilations to check for OOM/Deadlocks.")

	var wg sync.WaitGroup
	client := &mockLLMClientPC{delay: 10 * time.Millisecond}
	compiler, _ := prompt.NewJITPromptCompiler()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			compCtx := &prompt.CompilationContext{OperationalMode: "/active", TokenBudget: 100000}
			result, err := compiler.Compile(context.Background(), compCtx)
			if err != nil {
				t.Errorf("Compile error: %v", err)
			}
			if result == nil {
				t.Errorf("Result is nil")
			}
			_, _ = client.CompleteWithSystem(ctx, result.Prompt, "usr")
		}()
	}

	wg.Wait()
	t.Log("Survived 100 concurrent calls without deadlock.")
}
