package autopoiesis

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestToolOperationCoordination_IsPerToolAndCancellationAware(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.EnableThunderdome = false
	cfg.WorkspaceRoot = ""
	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	loop.compiler.config.WorkspaceRoot = ""

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	blocking := runtimeFixtureRequest(t, map[string]any{
		"mode": "block", "admission": listener.Addr().String(),
	}, types.GeneratedStdinV1)
	blocking.Tool.Name = "blocked_a"
	blocking.Tool.Workspace = workspace
	blocking.Action = types.MangleAtom("/blocked_a")
	blocking.Target = "blocked_a"
	blocking.ScopeID = "operation-lock"
	blocking.AuthorizationID = "exec-" + blocking.CallID
	if _, err := loop.registry.Register(&GeneratedTool{Name: blocking.Tool.Name}, &CompileResult{
		Success: true, OutputPath: blocking.Tool.BinaryPath, Hash: blocking.Tool.BinaryHash,
	}); err != nil {
		t.Fatalf("register blocking tool: %v", err)
	}
	blocking.Tool, err = loop.registry.GeneratedToolIdentity(blocking.Tool.Name)
	if err != nil {
		t.Fatalf("load blocking tool identity: %v", err)
	}

	executionCtx, cancelExecution := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelExecution()
	blockedDone := make(chan struct {
		output string
		err    error
	}, 1)
	go func() {
		defer close(blockedDone)
		output, err := loop.ExecuteTool(executionCtx, blocking.Tool.Name, blocking.CanonicalArgs)
		blockedDone <- struct {
			output string
			err    error
		}{output: output, err: err}
	}()

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- connection
	}()
	var connection net.Conn
	select {
	case connection = <-accepted:
	case err := <-acceptErr:
		t.Fatalf("blocking tool failed to connect: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("blocking tool did not start its process")
	}
	defer func() {
		cancelExecution()
		if connection != nil {
			_ = connection.Close()
		}
		select {
		case <-blockedDone:
		case <-time.After(5 * time.Second):
			t.Error("blocked generated execution did not stop during cleanup")
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var admitted [1]byte
	if _, err := io.ReadFull(connection, admitted[:]); err != nil || admitted[0] != 1 {
		t.Fatalf("blocking tool admission = %v, %v", admitted, err)
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}

	// A real compile, generated test, publication, and execution for B must
	// finish while A remains inside its blocked generated process.
	bTool := commitFixtureTool(cfg.ToolsDir, "unrelated_b", "new", passingCommitFixtureTest())
	bCommitCtx, cancelBCommit := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelBCommit()
	bCommitDone := make(chan error, 1)
	go func() { bCommitDone <- loop.commitTool(bCommitCtx, bTool, &LoopResult{}) }()
	select {
	case err := <-bCommitDone:
		if err != nil {
			t.Fatalf("unrelated B commit failed while A was blocked: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("unrelated B commit waited for blocked A")
	}
	if output, err := loop.ExecuteTool(context.Background(), "unrelated_b", `{"value":"ready"}`); err != nil || output != "new:ready" {
		t.Fatalf("unrelated B execution = %q, %v; want new:ready", output, err)
	}

	// A same-tool execution waiter must honor cancellation without disturbing
	// the active process or releasing its per-tool publication barrier.
	waitCtx, cancelWait := context.WithCancel(context.Background())
	defer cancelWait()
	waitDone := make(chan error, 1)
	go func() {
		_, err := loop.ExecuteTool(waitCtx, blocking.Tool.Name, blocking.CanonicalArgs)
		waitDone <- err
	}()
	waitForToolOperationWaiters(t, &loop.toolOperations, blocking.Tool.Name, 1)
	cancelWait()
	select {
	case err := <-waitDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled execution waiter error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled execution waiter did not return promptly")
	}

	// Publication uses the same keyed lock and must also return promptly when
	// its caller cancels while waiting for that tool's active execution.
	commitCtx, cancelCommit := context.WithCancel(context.Background())
	defer cancelCommit()
	commitDone := make(chan error, 1)
	aReplacement := commitFixtureTool(cfg.ToolsDir, blocking.Tool.Name, "new", passingCommitFixtureTest())
	go func() { commitDone <- loop.commitTool(commitCtx, aReplacement, &LoopResult{}) }()
	waitForToolOperationWaiters(t, &loop.toolOperations, blocking.Tool.Name, 1)
	cancelCommit()
	select {
	case err := <-commitDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled publication waiter error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled publication waiter did not return promptly")
	}
	if _, err := os.Stat(filepath.Join(cfg.ToolsDir, blocking.Tool.Name+".go")); !os.IsNotExist(err) {
		t.Fatalf("canceled publication wrote source before acquiring its lock (stat error %v)", err)
	}

	if _, err := connection.Write([]byte{1}); err != nil {
		t.Fatalf("release blocked tool: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close blocked tool connection: %v", err)
	}
	connection = nil
	select {
	case result := <-blockedDone:
		if result.err != nil || result.output != "literal" {
			t.Fatalf("blocked A result = %q, %v; want literal", result.output, result.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("blocked A did not finish after release")
	}
}

func TestExecuteGeneratedBackendWaitHonorsToolOperationCancellation(t *testing.T) {
	workspace := t.TempDir()
	config := Config{
		ToolsDir:      filepath.Join(workspace, ".nerd", "tools"),
		AgentsDir:     filepath.Join(workspace, ".nerd", "agents"),
		WorkspaceRoot: workspace,
		TargetOS:      runtime.GOOS,
		TargetArch:    runtime.GOARCH,
	}
	orchestrator := NewOrchestrator(&MockLLMClient{}, config)
	loop, ok := orchestrator.ouroboros.(*OuroborosLoop)
	if !ok {
		t.Fatal("orchestrator did not install the native Ouroboros loop")
	}
	request := runtimeFixtureRequest(t, map[string]any{"value": "must-not-run"}, types.GeneratedStdinV1)
	request.Tool.Name = "backend_tool"
	request.Tool.Workspace = workspace
	request.Action = types.MangleAtom("/backend_tool")
	request.Target = "backend_tool"
	request.ScopeID = "backend-lock"
	request.AuthorizationID = "exec-" + request.CallID
	if _, err := loop.registry.Register(&GeneratedTool{Name: request.Tool.Name}, &CompileResult{
		Success: true, OutputPath: request.Tool.BinaryPath, Hash: request.Tool.BinaryHash,
	}); err != nil {
		t.Fatalf("register generated backend fixture: %v", err)
	}
	identity, err := loop.registry.GeneratedToolIdentity(request.Tool.Name)
	if err != nil {
		t.Fatalf("resolve generated backend identity: %v", err)
	}
	request.Tool = identity

	release, err := loop.toolOperations.acquire(context.Background(), request.Tool.Name)
	if err != nil {
		t.Fatalf("hold tool operation lock: %v", err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan types.GeneratedToolReceipt, 1)
	go func() { done <- orchestrator.ExecuteGeneratedBackend(ctx, request) }()
	waitForToolOperationWaiters(t, &loop.toolOperations, request.Tool.Name, 1)
	cancel()
	select {
	case receipt := <-done:
		if !errors.Is(receipt.BackendError, context.Canceled) || receipt.ProcessStarted {
			t.Fatalf("canceled backend waiter receipt = %+v, want canceled before process start", receipt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("generated backend waiter did not return promptly after cancellation")
	}
}

func waitForToolOperationWaiters(t *testing.T, locks *toolOperationLocks, name string, want int) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		locks.mu.Lock()
		state := locks.locks[name]
		var waiters int
		var changed <-chan struct{}
		if state != nil {
			waiters = state.waiters
			changed = state.stateChanged
		}
		locks.mu.Unlock()
		if waiters >= want {
			return
		}
		if changed == nil {
			t.Fatalf("tool operation lock %q disappeared before waiter registration", name)
		}
		select {
		case <-changed:
		case <-deadline.C:
			t.Fatalf("tool %q did not reach %d waiting operations (got %d)", name, want, waiters)
		}
	}
}
