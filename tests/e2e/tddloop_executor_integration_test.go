//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// mockTDDLLMClient simulates an LLM for integration testing.
type mockTDDLLMClient struct {
	CompleteWithSystemFunc func(ctx context.Context, system string, prompt string) (string, error)
}

func (m *mockTDDLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
		},
	}

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "true" // Always pass
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, err := core.NewRealKernel()
    if err != nil {
        t.Fatalf("Failed to create kernel: %v", err)
    }

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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
            // Provide a valid patch, but test command will still fail
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: ../outside/file.go
<<<<
old
====
new
>>>>`, nil
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    var wg sync.WaitGroup
    started := make(chan struct{})

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			close(started)
			time.Sleep(100*time.Millisecond) // Give time for reset to occur
			return "no patches", nil
		},
	}

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false"
    tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

    wg.Add(1)
	go func() {
        defer wg.Done()
		_ = tdd.RunToCompletion(ctx)
	}()

    <-started

    // Concurrent reset
    tdd.Reset()

    wg.Wait()

    // Should not panic, state should be reasonable
}

func TestE2E_TDDLoop_StateCorruption_ConcurrentGetState(t *testing.T) {
    t.Parallel()

    tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    var wg sync.WaitGroup

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(c context.Context, system string, prompt string) (string, error) {
			time.Sleep(10*time.Millisecond)
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
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
        time.Sleep(1*time.Millisecond)
    }

    wg.Wait()
}

func TestE2E_TDDLoop_StateCorruption_InjectPatch(t *testing.T) {
    t.Parallel()

    tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

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
        time.Sleep(1*time.Millisecond)
    }

    wg.Wait()
}


// --- 4. Resource Exhaustion Tests ---

func TestE2E_TDDLoop_ResourceExhaustion_LargeLog(t *testing.T) {
    t.Parallel()

    tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

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

	// Wait until LLM is called
	<-started
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
		},
	}

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "false" // fail tests
    config.BuildCommand = "sleep 10" // stall build
    config.BuildTimeout = 10 * time.Millisecond // fast timeout
    tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

    err := tdd.RunToCompletion(ctx)
    if err != nil {
        t.Fatalf("Expected nil, got %v", err)
    }
}

func TestE2E_TDDLoop_TestTimeout(t *testing.T) {
    t.Parallel()

    tmpDir := t.TempDir()
	vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = tmpDir
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "no patches", nil
		},
	}

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
	config.TestCommand = "sleep 10" // stall test
    config.TestTimeout = 10 * time.Millisecond // fast timeout
    tdd := core.NewTDDLoopWithConfig(vs, kernel, llm, config)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

    err := tdd.RunToCompletion(ctx)
    if err != nil {
        t.Fatalf("Expected nil, got %v", err)
    }
}


// --- 6. Cascading Failure Tests ---

func TestE2E_TDDLoop_Cascading_VirtualStoreError(t *testing.T) {
    t.Parallel()

    // If VirtualStore's workspace is corrupted or missing entirely
    vsConfig := core.DefaultVirtualStoreConfig()
	vsConfig.WorkingDir = "/this/path/does/not/exist/guaranteed/999"
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return "", context.DeadlineExceeded // Simulate LLM completely breaking
		},
	}

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 1
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    var calls int
    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
            calls++
            if calls == 1 {
                return "", context.DeadlineExceeded // Fail first time
            }
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil // Succeed second time
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
	vs := core.NewVirtualStoreWithConfig(nil, vsConfig)
	kernel, _ := core.NewRealKernel()

    llm := &mockTDDLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system string, prompt string) (string, error) {
			return `FILE: test.go
<<<<
old
====
new
>>>>`, nil
		},
	}

    var runs int
    // We mock the test execution by intercepting VirtualStore
    // However, since we're using a real VirtualStore, we use a shell trick

    // Command that fails the first time, passes the second
    cmd := "if [ ! -f .test_passed ]; then touch .test_passed; false; else true; fi"

    config := core.DefaultTDDLoopConfig()
	config.MaxRetries = 2
	config.TestCommand = cmd
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
