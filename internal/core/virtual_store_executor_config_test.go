package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tactile"
)

// Every production command runs through VirtualStore's audited composite, not
// through the executor boot hands in. The composite used to be built from
// DefaultExecutorConfig, so the configured execution.default_timeout and the
// project build environment (BaseEnvironment: CGO_CFLAGS, GOFLAGS, GOCACHE)
// that boot put on the DirectExecutor never reached a single command.
func TestVirtualStoreModernExecutorInheritsCallerExecutorConfig(t *testing.T) {
	cfg := tactile.DefaultExecutorConfig()
	cfg.DefaultTimeout = 47 * time.Second
	cfg.BaseEnvironment = []string{"GOFLAGS=-mod=mod"}

	vsCfg := DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = t.TempDir()
	vs := NewVirtualStoreWithConfig(tactile.NewDirectExecutorWithConfig(cfg), vsCfg)

	vs.mu.RLock()
	modern := vs.modernExecutor
	vs.mu.RUnlock()
	if modern == nil {
		t.Fatal("modern executor not initialized")
	}

	if got := modern.Capabilities().DefaultTimeout; got != 47*time.Second {
		t.Fatalf("modern executor DefaultTimeout = %s, want the caller's 47s", got)
	}

	// The build environment must reach the child process.
	result, err := modern.Execute(context.Background(), tactile.Command{
		Binary:    "go",
		Arguments: []string{"env", "GOFLAGS"},
	})
	if err != nil {
		t.Fatalf("execute go env: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("go env GOFLAGS exited %d: %s", result.ExitCode, result.Combined)
	}
	if got := strings.TrimSpace(result.Stdout); got != "-mod=mod" {
		t.Fatalf("child GOFLAGS = %q, want the caller's base environment -mod=mod", got)
	}
}

// Cortex.Executor (what chat campaigns and `nerd campaign` run commands on)
// is VirtualStore.AuditedExecutor. It used to be the bare injected
// DirectExecutor, whose commands never reached the kernel.
func TestVirtualStoreAuditedExecutorEmitsExecutionFacts(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	direct := tactile.NewDirectExecutorWithConfig(tactile.DefaultExecutorConfig())
	vsCfg := DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = t.TempDir()
	vs := NewVirtualStoreWithConfig(direct, vsCfg)
	vs.SetKernel(kernel)

	audited := vs.AuditedExecutor()
	if audited == tactile.Executor(direct) {
		t.Fatal("AuditedExecutor returned the bare injected executor")
	}
	if _, err := audited.Execute(context.Background(), tactile.Command{Binary: "go", Arguments: []string{"version"}}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	facts, err := kernel.Query("execution_completed")
	if err != nil {
		t.Fatalf("query execution_completed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("a command on VirtualStore.AuditedExecutor left no execution_completed fact")
	}
}

// An executed command's action result carries its execution receipt,
// correlated by action ID; a denied action never reaches an executor and has
// no receipt (tactile-effect-receipt-v1).
func TestExecActionCarriesItsReceiptAndDeniedActionHasNone(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	vsCfg := DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = t.TempDir()
	vs := NewVirtualStoreWithConfig(tactile.NewDirectExecutorWithConfig(tactile.DefaultExecutorConfig()), vsCfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	res, err := vs.handleExecCmd(context.Background(), ActionRequest{ActionID: "act-receipt", Type: ActionExecCmd, Target: "go version", SessionID: "sess-r"})
	if err != nil {
		t.Fatalf("handleExecCmd: %v", err)
	}
	receipt, ok := res.Metadata["execution_receipt"].(tactile.ExecutionReceipt)
	if !ok {
		t.Fatalf("exec result carries no execution receipt: %+v", res.Metadata)
	}
	if receipt.RequestID != "act-receipt" || receipt.SessionID != "sess-r" || receipt.Outcome != tactile.ReceiptOutcomeSuccess {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.FactsEmitted == 0 || receipt.FactsRejected != 0 {
		t.Fatalf("the kernel should have accepted the execution facts: %+v", receipt)
	}

	if _, err := vs.RouteActionResult(context.Background(), Fact{Predicate: "next_action", Args: []any{"act-denied", "/exec_cmd", "go version"}}); err == nil {
		t.Fatal("an exec action with no permission was routed")
	}
	if _, ok := vs.auditLogger.Receipt("act-denied"); ok {
		t.Fatal("a denied action has an execution receipt")
	}
}

// Boot rollback builds a Cortex from a context whose store may not exist;
// AuditedExecutor on a nil store is nil, not a panic.
func TestVirtualStoreAuditedExecutorOnNilStore(t *testing.T) {
	var vs *VirtualStore
	if exec := vs.AuditedExecutor(); exec != nil {
		t.Fatalf("nil store returned executor %T", exec)
	}
}

// Config hands out a copy: mutating it cannot reach the executor.
func TestDirectExecutorConfigIsACopy(t *testing.T) {
	cfg := tactile.DefaultExecutorConfig()
	cfg.BaseEnvironment = []string{"A=1"}
	exec := tactile.NewDirectExecutorWithConfig(cfg)

	got := exec.Config()
	got.BaseEnvironment[0] = "A=2"
	got.DefaultLimits.TimeoutMs = 1

	again := exec.Config()
	if again.BaseEnvironment[0] != "A=1" || again.DefaultLimits.TimeoutMs == 1 {
		t.Fatalf("Config() leaked the executor's own configuration: %+v", again)
	}
}
