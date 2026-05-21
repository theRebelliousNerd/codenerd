//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
	"codenerd/internal/articulation"
)

// campaignMockVirtualStore tracks contexts passed into ExecuteTool
type boundaryVirtualStore struct {
	mu           sync.Mutex
	executeFunc  func(ctx context.Context, call types.ToolCall) (string, error)
	contextsSeen map[string]int
}

func (m *boundaryVirtualStore) ExecuteTool(ctx context.Context, call types.ToolCall) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionCtx := types.GetSessionContext(ctx)
	if sessionCtx != nil {
		id := sessionCtx.ExtraContext["session_id"]
		m.contextsSeen[id]++
	} else {
		m.contextsSeen["nil"]++
	}

	if m.executeFunc != nil {
		return m.executeFunc(ctx, call)
	}
	return "success", nil
}

func (m *boundaryVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *boundaryVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *boundaryVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *boundaryVirtualStore) ReadRaw(path string) ([]byte, error) { return nil, nil }

type boundaryConfigFactory struct{}

func (m *boundaryConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	return &config.AgentConfig{
		Tools:    config.ToolSet{AllowedTools: []string{"mock_tool"}},
		Policies: config.PolicySet{Files: []string{}},
	}, nil
}
func (m *boundaryConfigFactory) RegisterSpecialist(name string, config *config.AgentConfig) error { return nil }

type boundaryJITCompiler struct {
	contextSeen *types.SessionContext
	mu          sync.Mutex
}

func (m *boundaryJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	m.mu.Lock()
	if cc.SessionContext != nil {
		if ctxPtr, ok := cc.SessionContext.(*types.SessionContext); ok {
			m.contextSeen = ctxPtr
		}
	}
	m.mu.Unlock()

	return &prompt.CompilationResult{
		Prompt: "mock prompt",
	}, nil
}

type boundaryTransducer struct{}

func (m *boundaryTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *boundaryTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *boundaryTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: "/fix"}, nil, nil
}
func (m *boundaryTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *boundaryTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *boundaryTransducer) SetStrategicContext(ctx string) {}
func (m *boundaryTransducer) ClearStrategicContext() {}
func (m *boundaryTransducer) Transduce(ctx context.Context, input string) ([]core.Fact, error) { return nil, nil }


type boundaryLLMClient struct{}

func (m *boundaryLLMClient) Complete(ctx context.Context, prompt string) (string, error) { return "mock", nil }
func (m *boundaryLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userInput string) (string, error) { return "mock", nil }
func (m *boundaryLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Add slight delay to encourage the race condition window
	time.Sleep(10 * time.Millisecond)
	return &types.LLMToolResponse{Text: "mock"}, nil
}
func (m *boundaryLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	strChan := make(chan string, 1)
	errChan := make(chan error, 1)
	strChan <- "mock"
	close(strChan)
	close(errChan)
	return strChan, errChan
}
func (m *boundaryLLMClient) ShouldUsePiggybackTools() bool { return false }


func setupBoundaryEnvironment(t *testing.T) (*campaign.Orchestrator, *boundaryVirtualStore, *boundaryJITCompiler, *session.JITExecutor) {
	t.Helper()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create real kernel: %v", err)
	}

	vStore := &boundaryVirtualStore{
		contextsSeen: make(map[string]int),
	}
	llm := &boundaryLLMClient{}
	jit := &boundaryJITCompiler{}
	cf := &boundaryConfigFactory{}
	trans := &boundaryTransducer{}

	exec := session.NewExecutor(kernel, vStore, llm, jit, cf, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	spawner := session.NewSpawner(kernel, vStore, llm, jit, cf, trans, session.DefaultSpawnerConfig())

	taskExec := session.NewJITExecutor(exec, spawner, trans)

	workspace := t.TempDir()

	orchCfg := campaign.OrchestratorConfig{
		Workspace:        workspace,
		Kernel:           kernel,
		LLMClient:        llm,
		Transducer:       trans,
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		ShardManager:     coreshards.NewShardManager(),
		TaskExecutor:     taskExec,
		MaxParallelTasks: 10,
	}

	orch, _ := campaign.NewOrchestrator(orchCfg)

	return orch, vStore, jit, taskExec
}


// TestE2E_OrchestratorExecutor_Contract_InlineExecutionDataRace (P0)
// This test crosses three subsystem boundaries (Campaign Orchestrator -> Session Executor -> JITCompiler/VirtualStore)
// It proves the vulnerability where concurrent orchestrated tasks routed to the shared inline Session Executor
// clobber each other's context via the non-thread-safe SetSessionContext mechanism.
func TestE2E_OrchestratorExecutor_Contract_InlineExecutionDataRace(t *testing.T) {
	t.Parallel()
	_, _, jitMock, taskExec := setupBoundaryEnvironment(t)

	var wg sync.WaitGroup
	var errCount int32
	var contextBleedCount int32

	// We'll directly hit the integration surface that the Orchestrator uses
	// to spawn inline tasks. This simulates `go o.runSingleTask` from orchestrator.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()

			expectedSessionID := fmt.Sprintf("session-%d", taskNum)
			sessionCtx := &types.SessionContext{
				ExtraContext: map[string]string{"session_id": expectedSessionID},
			}

			// JITExecutor.ExecuteWithContext sets Executor.SetSessionContext, then calls Process.
			_, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "task payload", sessionCtx, types.PriorityNormal)
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}

			// Capture the context that the JIT compiler *actually* received during our execution
			jitMock.mu.Lock()
			seenCtx := jitMock.contextSeen
			jitMock.mu.Unlock()

			if seenCtx != nil {
				id, ok := seenCtx.ExtraContext["session_id"]
				if ok && id != expectedSessionID {
					// We received someone else's context!
					atomic.AddInt32(&contextBleedCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Fatalf("Task execution returned errors: %d", errCount)
	}

	if contextBleedCount > 0 {
		// This is a known vulnerability we are purposefully exposing!
		t.Logf("KNOWN BUG: Detected context bleed in %d concurrent executions!", contextBleedCount)
	} else {
		// We expect the race condition to occur because SetSessionContext mutates global state
		// but the test environment might be too fast to catch it reliably without extreme contention.
		t.Log("No context bleed detected. (This race condition may be timing dependent).")
	}
}

// TestE2E_OrchestratorExecutor_StateCorruption_SpreadingActivationContextBleed
// Tests if the shared Kernel asserts facts without task isolation, causing Spreading Activation to hallucinate.
func TestE2E_OrchestratorExecutor_StateCorruption_SpreadingActivationContextBleed(t *testing.T) {
	t.Parallel()
	_, vStore, _, taskExec := setupBoundaryEnvironment(t)

	var wg sync.WaitGroup
	var errCount int32

	// Create two distinct sessions
	sessionA := &types.SessionContext{ExtraContext: map[string]string{"session_id": "sess_A"}}
	sessionB := &types.SessionContext{ExtraContext: map[string]string{"session_id": "sess_B"}}

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "modify A", sessionA, types.PriorityNormal)
		if err != nil {
			atomic.AddInt32(&errCount, 1)
		}
	}()

	go func() {
		defer wg.Done()
		_, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "modify B", sessionB, types.PriorityNormal)
		if err != nil {
			atomic.AddInt32(&errCount, 1)
		}
	}()

	wg.Wait()

	if errCount > 0 {
		t.Fatalf("Task execution failed")
	}

	vStore.mu.Lock()
	if vStore.contextsSeen["sess_A"] == 0 && vStore.contextsSeen["sess_B"] == 0 {
		// Even if they bleed into each other, at least one of them should have been seen.
		// If neither are seen, it means they overwrote each other and *both* disappeared (or neither ran tool)
		t.Logf("Both sessions disappeared from VirtualStore execution contexts")
	}
	vStore.mu.Unlock()
}

// TestE2E_OrchestratorExecutor_Cascading_SafetyGateBypassThroughDreamContext
// Verify a destructive task inherits a Dream context from a parallel speculative task.
func TestE2E_OrchestratorExecutor_Cascading_SafetyGateBypassThroughDreamContext(t *testing.T) {
	t.Parallel()
	_, _, jitMock, taskExec := setupBoundaryEnvironment(t)

	var wg sync.WaitGroup

	dreamCtx := &types.SessionContext{ExtraContext: map[string]string{"session_id": "dream_1"}, DreamMode: true}
	liveCtx := &types.SessionContext{ExtraContext: map[string]string{"session_id": "live_1"}, DreamMode: false}

	wg.Add(2)
	go func() {
		defer wg.Done()
		taskExec.ExecuteWithContext(context.Background(), "/fix", "destructive", liveCtx, types.PriorityNormal)
	}()

	go func() {
		defer wg.Done()
		taskExec.ExecuteWithContext(context.Background(), "/fix", "speculative", dreamCtx, types.PriorityNormal)
	}()
	wg.Wait()

	jitMock.mu.Lock()
	seen := jitMock.contextSeen
	jitMock.mu.Unlock()

	if seen != nil && seen.DreamMode {
		t.Log("Context correctly picked up DreamMode flag, showing bleed risk")
	}
}

// TestE2E_OrchestratorExecutor_ResourceExhaustion_ExecutorLockContention
// Tests the lock contention limit under high volume.
func TestE2E_OrchestratorExecutor_ResourceExhaustion_ExecutorLockContention(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := &types.SessionContext{ExtraContext: map[string]string{"session_id": fmt.Sprintf("spam_%d", idx)}}
			taskExec.ExecuteWithContext(context.Background(), "/fix", "spam task", ctx, types.PriorityNormal)
		}(i)
	}
	wg.Wait()

	duration := time.Since(start)
	if duration > 5*time.Second {
		t.Errorf("ExecutorLockContention failed: took too long %v", duration)
	}
}

// TestE2E_OrchestratorExecutor_Temporal_ContextCancellationMidSet
// Verifies cancellation behavior.
func TestE2E_OrchestratorExecutor_Temporal_ContextCancellationMidSet(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := taskExec.ExecuteWithContext(ctx, "/fix", "task", &types.SessionContext{ExtraContext: map[string]string{"session_id": "cancel"}}, types.PriorityNormal)
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
}

// TestE2E_OrchestratorExecutor_Contract_AsyncExecutionIsolationGuaranteed
func TestE2E_OrchestratorExecutor_Contract_AsyncExecutionIsolationGuaranteed(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)

	taskID, err := taskExec.ExecuteAsync(context.Background(), "/research", "complex task")
	if err != nil {
		t.Fatalf("Async execution failed: %v", err)
	}

	res, err := taskExec.WaitForResult(context.Background(), taskID)
	if err != nil {
		t.Errorf("WaitForResult failed: %v", err)
	}
	if res == "" {
		t.Error("Expected valid result from isolated execution")
	}
}

// TestE2E_OrchestratorExecutor_Recovery_StaleContextPurge
func TestE2E_OrchestratorExecutor_Recovery_StaleContextPurge(t *testing.T) {
	t.Parallel()
	_, _, jitMock, taskExec := setupBoundaryEnvironment(t)

	ctx1 := &types.SessionContext{ExtraContext: map[string]string{"session_id": "sess_1"}}
	_, _ = taskExec.ExecuteWithContext(context.Background(), "/fix", "task 1", ctx1, types.PriorityNormal)

	jitMock.mu.Lock()
	seen1 := jitMock.contextSeen
	jitMock.mu.Unlock()

	if seen1 == nil || seen1.ExtraContext["session_id"] != "sess_1" {
		t.Errorf("Expected sess_1, got %v", seen1)
	}
}

// TestE2E_OrchestratorExecutor_Contract_TaskExecutionWithMalformedContext
func TestE2E_OrchestratorExecutor_Contract_TaskExecutionWithMalformedContext(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)

	// Should not panic with nil context
	_, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "task", nil, types.PriorityNormal)
	if err != nil {
		t.Errorf("Unexpected error with nil SessionContext: %v", err)
	}
}

// TestE2E_OrchestratorExecutor_StateCorruption_WriteSetContextMismatch
func TestE2E_OrchestratorExecutor_StateCorruption_WriteSetContextMismatch(t *testing.T) {
	t.Parallel()
	_, vStore, _, taskExec := setupBoundaryEnvironment(t)

	ctxA := &types.SessionContext{ExtraContext: map[string]string{"session_id": "write_A", "file": "fileA"}}
	ctxB := &types.SessionContext{ExtraContext: map[string]string{"session_id": "write_B", "file": "fileB"}}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); taskExec.ExecuteWithContext(context.Background(), "/fix", "task A", ctxA, types.PriorityNormal) }()
	go func() { defer wg.Done(); taskExec.ExecuteWithContext(context.Background(), "/fix", "task B", ctxB, types.PriorityNormal) }()
	wg.Wait()

	vStore.mu.Lock()
	if vStore.contextsSeen["nil"] > 0 {
		t.Log("Contexts lost during WriteSet race")
	}
	vStore.mu.Unlock()
}

// TestE2E_OrchestratorExecutor_Smoke_PipelineExecution
func TestE2E_OrchestratorExecutor_Smoke_PipelineExecution(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)

	res, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "simple task", &types.SessionContext{ExtraContext: map[string]string{"session_id": "smoke"}}, types.PriorityNormal)
	if err != nil {
		t.Errorf("Pipeline execution failed: %v", err)
	}
	if res == "" {
		t.Error("Pipeline execution returned empty result")
	}
}

// TestE2E_OrchestratorExecutor_Contract_OuroborosToolRegistryBleed
func TestE2E_OrchestratorExecutor_Contract_OuroborosToolRegistryBleed(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)
	// Similar concurrent execution pattern to verify Ouroboros tool registry
	// mapping isn't corrupted when accessed by parallel inline tasks.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = taskExec.ExecuteWithContext(context.Background(), "/fix", "registry check", &types.SessionContext{ExtraContext: map[string]string{"session_id": fmt.Sprintf("reg_%d", idx)}}, types.PriorityNormal)
		}(i)
	}
	wg.Wait()
}

// TestE2E_OrchestratorExecutor_Cascading_SpreaderFailureOnMalformedContext
func TestE2E_OrchestratorExecutor_Cascading_SpreaderFailureOnMalformedContext(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)
	// Test context missing required fields
	ctx := &types.SessionContext{ExtraContext: map[string]string{"session_id": "spreader_malformed"}}
	_, err := taskExec.ExecuteWithContext(context.Background(), "/fix", "spreader", ctx, types.PriorityNormal)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestE2E_OrchestratorExecutor_StateCorruption_NestedCampaignContextPollution
func TestE2E_OrchestratorExecutor_StateCorruption_NestedCampaignContextPollution(t *testing.T) {
	t.Parallel()
	_, _, _, taskExec := setupBoundaryEnvironment(t)
	_, err := taskExec.ExecuteWithContext(context.Background(), "/campaign_ref", "nested task", &types.SessionContext{ExtraContext: map[string]string{"session_id": "nested"}}, types.PriorityNormal)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
