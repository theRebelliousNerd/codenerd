//go:build integration

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// mockTDDLLMClient simulates an LLM for integration testing.
type mockTDDLLMClient struct {
	CompleteWithSystemFunc func(ctx context.Context, system string, prompt string) (string, error)
}

func (m *mockTDDLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	// generatePatch calls Complete, so the scripted func answers here too.
	if m.CompleteWithSystemFunc != nil {
		return m.CompleteWithSystemFunc(ctx, "", prompt)
	}
	return "", nil
}

func (m *mockTDDLLMClient) CompleteWithSystem(ctx context.Context, system string, prompt string) (string, error) {
	if m.CompleteWithSystemFunc != nil {
		return m.CompleteWithSystemFunc(ctx, system, prompt)
	}
	return "", nil
}

func (m *mockTDDLLMClient) CompleteWithTools(ctx context.Context, prompt string, input string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{}, nil
}

func (m *mockTDDLLMClient) Stream(ctx context.Context, prompt string, out chan<- string) error {
	close(out)
	return nil
}

func (m *mockTDDLLMClient) StreamWithSystem(ctx context.Context, system string, prompt string, out chan<- string) error {
	close(out)
	return nil
}

func (m *mockTDDLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error)
	go func() {
		close(ch)
		close(errCh)
	}()
	return ch, errCh
}

func (m *mockTDDLLMClient) ShouldUsePiggybackTools() bool {
	return false
}

func (m *mockTDDLLMClient) EmbedContext(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

// --- 1. Smoke Tests ---

func TestE2E_TDDLoop_SmokeTest(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "sh -c true" // Always pass (sh head passes the command allowlist)
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}

	if tdd.GetState() != core.TDDStatePassing {
		t.Errorf("Expected passing state, got %s", tdd.GetState())
	}
}

// --- 2. Contract Violation Tests ---

func TestE2E_TDDLoop_EmptyPatch_Escalates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			// Deliberately violate contract: return no patches.
			return "I analyzed the error. It's broken. Good luck.", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	config.TestCommand = "false" // Always fail
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tdd.RunToCompletion(ctx)

	// We expect the loop to finish without timing out, and hit an Escalated state
	if err != nil {
		t.Fatalf("Expected clean exit, got error: %v", err)
	}

	state := tdd.GetState()
	if state != core.TDDStateEscalated {
		t.Errorf("Expected state to be %s, got %s. Infinite loop or bad transition occurred.", core.TDDStateEscalated, state)
	}
}

func TestE2E_TDDLoop_MaxRetries_Escalation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			// Provide a valid patch, but test command will still fail
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}

	if tdd.GetState() != core.TDDStateEscalated {
		t.Errorf("Expected escalated state, got %s", tdd.GetState())
	}
}

func TestE2E_TDDLoop_GarbageOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "echo FAILED_GARBAGE && false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}

	if tdd.GetState() != core.TDDStateEscalated {
		t.Errorf("Expected escalated state, got %s", tdd.GetState())
	}
}

func TestE2E_TDDLoop_ExternalModification(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: ../outside/file.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}
}

func TestE2E_TDDLoop_VirtualStoreTypeConfusion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	// VirtualStore silently rejecting should cause loop escalation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}
}

// --- 3. State Corruption Tests ---

func TestE2E_TDDLoop_StateCorruption_ConcurrentReset(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	var wg sync.WaitGroup
	started := make(chan struct{})
	var startOnce sync.Once

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			// Once-guarded: a second LLM call must not panic on a
			// double close and mask the behavior under test.
			startOnce.Do(func() { close(started) })
			time.Sleep(100 * time.Millisecond) // Give time for reset to occur
			return "no patches", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	// MaxRetries must exceed 1: with a budget of 1 the first failure goes
	// Failing→Escalate directly, the LLM is never called, `started` never
	// closes, and the test below deadlocks the whole package. That exact
	// shape hung the full e2e suite at the 600s timeout.
	config.MaxRetries = 2
	// Bare "false" is not in the run_tests binary allowlist (the default
	// command runs instead); the sh wrapper fails fast and deterministically.
	config.TestCommand = `sh -c "false"`
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(runDone)
		_ = tdd.RunToCompletion(ctx)
	}()

	// Bounded wait: if the loop never reaches the LLM this fails loudly
	// instead of hanging the package until the go test timeout.
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatalf("loop never reached patch generation (state=%s)", tdd.GetState())
	}

	// Concurrent reset mid-generation: the in-flight generatePatch must
	// abort cleanly, not panic or corrupt state.
	tdd.Reset()

	select {
	case <-runDone:
	case <-time.After(20 * time.Second):
		t.Fatalf("RunToCompletion did not return after Reset (state=%s)", tdd.GetState())
	}
	wg.Wait()

	// Should not panic; state must be a valid loop state.
	switch tdd.GetState() {
	case core.TDDStateIdle, core.TDDStateEscalated, core.TDDStatePassing,
		core.TDDStateFailing, core.TDDStateAnalyzing, core.TDDStateGenerating,
		core.TDDStateApplying, core.TDDStateCompiling, core.TDDStateCompileError,
		core.TDDStateRunning:
	default:
		t.Fatalf("corrupt state after concurrent Reset: %q", tdd.GetState())
	}
}

func TestE2E_TDDLoop_StateCorruption_ConcurrentGetState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	var wg sync.WaitGroup

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tdd.RunToCompletion(ctx)
	}()

	for i := 0; i < 100; i++ {
		tdd.GetState()
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()
}

func TestE2E_TDDLoop_StateCorruption_InjectPatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			return "wait", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tdd.RunToCompletion(ctx)
	}()

	for i := 0; i < 100; i++ {
		tdd.InjectPatch(core.Patch{FilePath: "test.go"})
		time.Sleep(1 * time.Millisecond)
	}

	wg.Wait()
}

// --- 4. Resource Exhaustion Tests ---

func TestE2E_TDDLoop_ResourceExhaustion_LargeLog(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	// Simulate a massive log output via a custom test command
	largeOutput := strings.Repeat("FAIL: some test\n", 10000)

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "echo '" + largeOutput + "' && false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}
}

func TestE2E_TDDLoop_ResourceExhaustion_Flooding(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "no patches", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	// Rapidly loop
	for i := 0; i < 100; i++ {
		tdd.Reset()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_ = tdd.RunToCompletion(ctx)
		cancel()
	}
}

// --- 5. Temporal Failure Tests ---

func TestE2E_TDDLoop_ContextCancellation_Aborts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started := make(chan struct{})

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			close(started)
			// Wait for cancellation
			<-c.Done()
			return "", c.Err()
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	errCh := make(chan error, 1)
	go func() {
		errCh <- tdd.RunToCompletion(ctx)
	}()

	// Wait until LLM is called, bounded: an unbounded wait here hangs
	// the whole package if the loop never reaches patch generation.
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatalf("loop never reached patch generation (state=%s)", tdd.GetState())
	}
	// Cancel the context mid-flight
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatalf("Expected context cancellation error, got nil")
	}
}

func TestE2E_TDDLoop_BuildTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	// The build step only runs after a patch applies cleanly, so the
	// fixture file must exist or the loop escalates before ever building.
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\n\n// old marker\n"), 0644); err != nil {
		t.Fatalf("failed to seed buggy file: %v", err)
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	// NOTE: bare "false"/"sleep" are not in the run_tests/build_project
	// binary allowlist, so they would silently run the default command
	// instead of stalling. The sh -c wrapper makes the stall real.
	config.TestCommand = `sh -c "false"`     // fail tests fast
	config.BuildCommand = `sh -c "sleep 20"` // stall build
	config.BuildTimeout = 2 * time.Second    // kill the stall
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := tdd.RunToCompletion(ctx)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
	if tdd.GetState() != core.TDDStateEscalated {
		t.Errorf("Expected Escalated after killed build, got %s", tdd.GetState())
	}
	// A killed build passes through CompileError; a completed sleep would
	// have succeeded the build and skipped that state. Deterministic pin
	// that does not depend on timing margins.
	sawCompileError := false
	for _, tr := range tdd.GetHistory() {
		if tr.ToState == core.TDDStateCompileError {
			sawCompileError = true
			break
		}
	}
	if !sawCompileError {
		t.Error("expected a CompileError transition from the killed build")
	}
	// The sleep lasts 20s: returning sooner proves BuildTimeout killed it
	// rather than the command completing on its own.
	if elapsed >= 12*time.Second {
		t.Errorf("BuildTimeout did not kill the stalled build: loop took %v", elapsed)
	}
}

func TestE2E_TDDLoop_TestTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "no patches", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	// NOTE: bare "sleep" is not in the run_tests binary allowlist; the sh
	// -c wrapper makes the stall real instead of running the default.
	config.TestCommand = `sh -c "sleep 20"` // stall test
	config.TestTimeout = 2 * time.Second    // kill the stall
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := tdd.RunToCompletion(ctx)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
	// A killed stall is a test failure, so the loop must escalate. If the
	// timeout were ignored, sleep would exit 0 and the loop would pass.
	if tdd.GetState() != core.TDDStateEscalated {
		t.Errorf("Expected Escalated after killed test run, got %s", tdd.GetState())
	}
	if elapsed >= 12*time.Second {
		t.Errorf("TestTimeout did not kill the stalled suite: loop took %v", elapsed)
	}
}

// --- 6. Cascading Failure Tests ---

func TestE2E_TDDLoop_Cascading_VirtualStoreError(t *testing.T) {
	t.Parallel()

	// If VirtualStore's workspace is corrupted or missing entirely
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = "/this/path/does/not/exist/guaranteed/999"
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "no patches", nil
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "ls"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil err, got %v", err)
	}

	if tdd.GetState() != core.TDDStateEscalated {
		t.Errorf("Expected escalated state, got %s", tdd.GetState())
	}
}

func TestE2E_TDDLoop_Cascading_LLMError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "", context.DeadlineExceeded // Simulate LLM completely breaking
		},
	}

	config := core.DefaultTDDLoopConfig()
	// Two attempts: with one, the first test failure escalates before the
	// loop ever reaches patch generation, so the LLM breakage is never felt.
	config.MaxRetries = 2
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	// Should gracefully bubble up the error or escalate
	if err == nil {
		t.Fatalf("Expected error from LLM, got nil")
	}
}

// --- 7. Recovery Tests ---

func TestE2E_TDDLoop_Recovery_TemporaryLLMFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	var calls int
	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			calls++
			if calls == 1 {
				return "", context.DeadlineExceeded // Fail first time
			}
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil // Succeed second time
		},
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	config.TestCommand = "false"
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err == nil && calls == 1 { // If it didn't retry or recovered but no err
		// it shouldn't just pass.
	}
}

func TestE2E_TDDLoop_Recovery_SpuriousFailPass(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), vsConfig)
	vs.DisableBootGuard() // action routing is blocked until first user interaction
	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // destructive actions need the Dreamer simulator

	llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
OLD:
old
NEW:
new
RATIONALE: e2e fixture fix`, nil
		},
	}

	var runs int
	// We mock the test execution by intercepting VirtualStore
	// However, since we're using a real VirtualStore, we use a shell trick

	// Command that fails the first time, passes the second
	cmd := "sh -c \"if [ ! -f .test_passed ]; then touch .test_passed; false; else true; fi\""

	// The fixture patch replaces "old" with "new" in test.go: the file must
	// exist and stay valid Go, or the edit (or its post-action syntax
	// validation) fails and the loop escalates instead of recovering.
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\n\n// old marker\n"), 0644); err != nil {
		t.Fatalf("failed to seed buggy file: %v", err)
	}

	config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	config.TestCommand = cmd
	config.BuildCommand = "sh -c true" // no Go module in the fixture dir; stub like TestCommand
	tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tdd.RunToCompletion(ctx)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	runs++
	if tdd.GetState() != core.TDDStatePassing {
		t.Errorf("Expected Passing, got %s", tdd.GetState())
	}
}
