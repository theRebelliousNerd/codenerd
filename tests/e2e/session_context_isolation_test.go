//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
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
	return "sci-system:" + systemPrompt + "\nsci-input:" + userInput, nil
}
func (m *sciMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.mu.Unlock()
	return &types.LLMToolResponse{Text: "sci-system:" + systemPrompt + "\nsci-input:" + userInput}, nil
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
	return &prompt.CompilationResult{Prompt: "sci-context:" + sciIsolationMarker(cc)}, nil
}

// sciIsolationMarker exposes the session context a compilation actually used.
// The function is pure so concurrent compiles cannot contaminate each other
// through this mock.
func sciIsolationMarker(cc *prompt.CompilationContext) string {
	if cc == nil {
		return "missing-compilation-context"
	}
	sessionCtx, ok := cc.SessionContext.(*types.SessionContext)
	if !ok || sessionCtx == nil {
		return "missing-session-context"
	}
	marker := sessionCtx.ExtraContext["isolation_marker"]
	if marker == "" {
		return "missing-isolation-marker"
	}
	return marker
}

func sciSessionContext(marker string) *types.SessionContext {
	return &types.SessionContext{ExtraContext: map[string]string{"isolation_marker": marker}}
}

// sciRequireIsolatedOutput verifies that one execution consumed exactly its
// own task and session-context markers, and none of the foreign markers.
func sciRequireIsolatedOutput(t *testing.T, name, output, ownTask, ownContext string, foreign []string) {
	t.Helper()
	if !strings.Contains(output, ownTask) {
		t.Errorf("%s output lost its task marker %q: %q", name, ownTask, output)
	}
	if !strings.Contains(output, ownContext) {
		t.Errorf("%s output lost its session-context marker %q: %q", name, ownContext, output)
	}
	for _, marker := range foreign {
		if strings.Contains(output, marker) {
			t.Errorf("%s output leaked foreign marker %q: %q", name, marker, output)
		}
	}
}

// sciRequireHistoryOwnership verifies that a task-scoped executor history
// contains its own turn and no foreign task or context markers.
func sciRequireHistoryOwnership(t *testing.T, name string, history []perception.ConversationTurn, ownTask, ownContext string, foreign []string) {
	t.Helper()
	if len(history) != 2 {
		t.Fatalf("%s history has %d turns, want exactly one user turn and one assistant turn", name, len(history))
	}
	if history[0].Role != "user" || !strings.Contains(history[0].Content, ownTask) {
		t.Errorf("%s user turn does not own %q: %+v", name, ownTask, history[0])
	}
	if history[1].Role != "assistant" || !strings.Contains(history[1].Content, ownTask) || !strings.Contains(history[1].Content, ownContext) {
		t.Errorf("%s assistant turn does not own %q and %q: %+v", name, ownTask, ownContext, history[1])
	}
	for _, turn := range history {
		for _, marker := range foreign {
			if strings.Contains(turn.Content, marker) {
				t.Errorf("%s history leaked foreign marker %q: %+v", name, marker, turn)
			}
		}
	}
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
	taskMarkers := make([]string, goroutines)
	contextMarkers := make([]string, goroutines)
	for i := range taskMarkers {
		taskMarkers[i] = fmt.Sprintf("task-marker-%02d", i)
		contextMarkers[i] = fmt.Sprintf("context-marker-%02d", i)
	}

	var wg sync.WaitGroup
	results := make([]string, goroutines)
	errors := make([]error, goroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			taskCtx := types.WithSessionContext(ctx, sciSessionContext(contextMarkers[idx]))
			taskID, err := taskExec.ExecuteAsync(taskCtx, session.TaskRequest{IntentVerb: "/fix", Task: "repair " + taskMarkers[idx]})
			if err != nil {
				errors[idx] = err
				return
			}

			result, err := taskExec.WaitForResult(taskCtx, taskID)
			results[idx] = result
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}
	for i, result := range results {
		if errors[i] != nil {
			continue
		}
		foreign := make([]string, 0, 2*(goroutines-1))
		for j := range taskMarkers {
			if j == i {
				continue
			}
			foreign = append(foreign, taskMarkers[j], contextMarkers[j])
		}
		sciRequireIsolatedOutput(t, fmt.Sprintf("goroutine %d", i), result, taskMarkers[i], contextMarkers[i], foreign)
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

	parent := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	parent.SetConfig(session.DefaultExecutorConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Task-scoped work clones the session executor. Two sequential clones
	// must neither see each other's session context nor write their turns
	// back into the parent session.
	firstInput := "first task with sequential-task-A"
	first := parent.CloneForTask()
	first.SetSessionContext(sciSessionContext("sequential-context-A"))
	resultA, err := first.Process(ctx, firstInput)
	if err != nil {
		t.Fatalf("First Process failed: %v", err)
	}

	secondInput := "second task with sequential-task-B"
	second := parent.CloneForTask()
	second.SetSessionContext(sciSessionContext("sequential-context-B"))
	resultB, err := second.Process(ctx, secondInput)
	if err != nil {
		t.Fatalf("Second Process failed: %v", err)
	}

	sciRequireIsolatedOutput(t, "first", resultA.Response, "sequential-task-A", "sequential-context-A", []string{"sequential-task-B", "sequential-context-B"})
	sciRequireIsolatedOutput(t, "second", resultB.Response, "sequential-task-B", "sequential-context-B", []string{"sequential-task-A", "sequential-context-A"})
	sciRequireHistoryOwnership(t, "first", first.GetHistory(), "sequential-task-A", "sequential-context-A", []string{"sequential-task-B", "sequential-context-B"})
	sciRequireHistoryOwnership(t, "second", second.GetHistory(), "sequential-task-B", "sequential-context-B", []string{"sequential-task-A", "sequential-context-A"})

	if got := len(parent.GetHistory()); got != 0 {
		t.Errorf("parent session history has %d turns after two cloned task runs, want 0", got)
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *sciMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}
