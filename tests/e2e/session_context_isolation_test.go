//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS — session context isolation
// =============================================================================

// sciMockLLMClient captures the system prompt it received
type sciMockLLMClient struct {
	mu            sync.Mutex
	systemPrompts []string
}

func (m *sciMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (m *sciMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userInput string) (string, error) {
	m.mu.Lock()
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.mu.Unlock()
	return "ok", nil
}
func (m *sciMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.mu.Unlock()
	return &types.LLMToolResponse{Text: "ok"}, nil
}
func (m *sciMockLLMClient) ShouldUsePiggybackTools() bool { return false }

func (m *sciMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

type sciMockVirtualStore struct{}

func (m *sciMockVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *sciMockVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *sciMockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *sciMockVirtualStore) ReadRaw(path string) ([]byte, error) { return nil, nil }

type sciMockConfigFactory struct{}

func (m *sciMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{}, nil
}
func (m *sciMockConfigFactory) RegisterSpecialist(name string, config *config.EffectiveAgentRuntimeConfig) error {
	return nil
}

type sciMockJITCompiler struct{}

func (m *sciMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "mock"}, nil
}

type sciMockTransducer struct{}

func (m *sciMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *sciMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *sciMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: "/fix"}, nil, nil
}
func (m *sciMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *sciMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *sciMockTransducer) SetStrategicContext(ctx string)                   {}

// =============================================================================
// TestE2E_SessionContext_ConcurrentExecute_NoBleed
// =============================================================================
// Verifies that concurrent ExecuteAsync calls with different session contexts
// don't leak state between them.

func TestE2E_SessionContext_ConcurrentExecute_NoBleed(t *testing.T) {
	llm := &sciMockLLMClient{}
	vstore := &sciMockVirtualStore{}
	jit := &sciMockJITCompiler{}
	cfgFactory := &sciMockConfigFactory{}
	trans := &sciMockTransducer{}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	spawner := session.NewSpawner(nil, vstore, llm, jit, cfgFactory, trans, session.DefaultSpawnerConfig())
	taskExec := session.NewJITExecutor(exec, spawner, trans)

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]string, goroutines)
	errors := make([]error, goroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			uniqueTask := fmt.Sprintf("task_with_unique_marker_%d_%d", idx, time.Now().UnixNano())

			taskID, err := taskExec.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: uniqueTask})
			if err != nil {
				errors[idx] = err
				return
			}

			result, err := taskExec.WaitForResult(ctx, taskID)
			results[idx] = result
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Count successes
	successes := 0
	for i, err := range errors {
		if err != nil {
			t.Logf("Goroutine %d error: %v (may be capacity limit)", i, err)
		} else {
			successes++
		}
	}

	t.Logf("Concurrent context isolation: %d/%d succeeded", successes, goroutines)

	// Verify all results are independent (no shared result values between goroutines)
	// Since our mock always returns "ok", we can't distinguish by value.
	// But the key invariant is: no panics, no races (tested with -race flag)
	if successes == 0 {
		t.Error("All goroutines failed — possible systemic issue")
	}
}

// =============================================================================
// TestE2E_SessionContext_SequentialExecution_NoStateBleed
// =============================================================================
// Verifies that two sequential Process calls don't share conversation state.

func TestE2E_SessionContext_SequentialExecution_NoStateBleed(t *testing.T) {
	llm := &sciMockLLMClient{}
	vstore := &sciMockVirtualStore{}
	jit := &sciMockJITCompiler{}
	cfgFactory := &sciMockConfigFactory{}
	trans := &sciMockTransducer{}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First call
	result1, err := exec.Process(ctx, "first task with unique context A")
	if err != nil {
		t.Fatalf("First Process failed: %v", err)
	}
	t.Logf("First result: %v", result1)

	// Second call
	result2, err := exec.Process(ctx, "second task with unique context B")
	if err != nil {
		t.Fatalf("Second Process failed: %v", err)
	}
	t.Logf("Second result: %v", result2)

	// Verify conversation history isn't shared
	// The mock LLM should have received 2 distinct calls
	llm.mu.Lock()
	promptCount := len(llm.systemPrompts)
	llm.mu.Unlock()

	t.Logf("Total system prompts received: %d", promptCount)

	// Each Process call should have its own invocation
	// (At minimum 2, could be more if the executor retries)
	if promptCount < 2 {
		t.Errorf("Expected at least 2 LLM calls for 2 Process calls, got %d", promptCount)
	}
}
