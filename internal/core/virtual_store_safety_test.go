package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tactile"
)

type countingRouteExecutor struct {
	calls int
	err   error
}

func (e *countingRouteExecutor) Execute(context.Context, tactile.Command) (*tactile.ExecutionResult, error) {
	e.calls++
	if e.err != nil {
		return nil, e.err
	}
	return &tactile.ExecutionResult{ExitCode: 0, Stdout: "executed", Success: true}, nil
}

func (*countingRouteExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (*countingRouteExecutor) Validate(tactile.Command) error { return nil }

type failingRouteValidator struct{}

func (*failingRouteValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionReadFile
}

func (*failingRouteValidator) Validate(context.Context, ActionRequest, ActionResult) ValidationResult {
	return ValidationResult{
		Verified:   false,
		Confidence: 1,
		Method:     ValidationMethodContentCheck,
		Error:      "forced mismatch",
	}
}

func (*failingRouteValidator) Name() string  { return "failing_route_validator" }
func (*failingRouteValidator) Priority() int { return 1 }

func TestRouteActionFailsClosedWithoutKernel(t *testing.T) {
	executor := &countingRouteExecutor{}
	vs := NewVirtualStoreWithConfig(executor, DefaultVirtualStoreConfig())
	vs.DisableModernExecutor()
	vs.DisableBootGuard()

	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args: []any{
			"nil-kernel-action",
			MangleAtom("/exec_cmd"),
			"echo must-not-run",
			map[string]any{"binary": "echo"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "dreamer unavailable") {
		t.Fatalf("RouteAction error = %v, want fail-closed Dreamer error", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}
}

func TestRouteActionFailsClosedWhenCortexHasNoDreamerKernel(t *testing.T) {
	executor := &countingRouteExecutor{}
	vs := NewVirtualStoreWithConfig(executor, DefaultVirtualStoreConfig())
	vs.DisableModernExecutor()
	vs.SetKernel(NewCortexKernel("cortex"))
	vs.DisableBootGuard()

	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"empty-cortex-action", MangleAtom("/exec_cmd"), "echo must-not-run"},
	})
	if err == nil || !strings.Contains(err.Error(), "dreamer unavailable") {
		t.Fatalf("RouteAction error = %v, want fail-closed Dreamer error", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}
}

func TestVirtualStoreDreamerUsesCortexPrimaryKernel(t *testing.T) {
	cortex := NewCortexKernel("cortex")
	shard, err := NewKernelShard(KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("NewKernelShard: %v", err)
	}
	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("RegisterShard: %v", err)
	}

	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(cortex)
	dreamer := vs.GetDreamer()
	if dreamer == nil {
		t.Fatal("expected Cortex-backed Dreamer")
	}
	if dreamer.kernel != shard.kernel {
		t.Fatal("Dreamer is not attached to Cortex primary RealKernel")
	}
}

func TestPreflightDestructiveToolCallFailsClosedWithoutDreamer(t *testing.T) {
	kernel := &stubKernel{}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)

	err := vs.PreflightDestructiveToolCall(
		context.Background(),
		"interactive-write",
		"write_file",
		map[string]any{"path": "blocked.go", "content": "package blocked"},
	)
	if err == nil || !strings.Contains(err.Error(), "dreamer unavailable") {
		t.Fatalf("PreflightDestructiveToolCall error = %v, want fail-closed Dreamer error", err)
	}
	fact, ok := findAssertedFact(kernel.asserted, "security_violation")
	if !ok || len(fact.Args) != 3 {
		t.Fatalf("security_violation = %#v, want declared /3 denial fact", fact)
	}

	if err := vs.PreflightDestructiveToolCall(
		context.Background(),
		"interactive-read",
		"read_file",
		map[string]any{"path": "safe.go"},
	); err != nil {
		t.Fatalf("non-destructive preflight error = %v", err)
	}
}

func TestRouteActionFailureFactsMatchDeclaredContracts(t *testing.T) {
	t.Run("security_violation_3", func(t *testing.T) {
		kernel := &stubKernel{}
		vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
		vs.SetKernel(kernel)
		vs.DisableBootGuard()

		_, err := vs.RouteAction(context.Background(), Fact{
			Predicate: "next_action",
			Args:      []any{"denied-action", MangleAtom("/read_file"), "denied.go"},
		})
		if err == nil {
			t.Fatal("expected permission denial")
		}

		fact, ok := findAssertedFact(kernel.asserted, "security_violation")
		if !ok {
			t.Fatal("security_violation fact not asserted")
		}
		if len(fact.Args) != 3 {
			t.Fatalf("security_violation arity = %d, want 3", len(fact.Args))
		}
		if got, ok := fact.Args[0].(MangleAtom); !ok || got != MangleAtom("/read_file") {
			t.Fatalf("security_violation action = %#v, want /read_file name", fact.Args[0])
		}
		if _, ok := fact.Args[1].(string); !ok {
			t.Fatalf("security_violation reason type = %T, want string", fact.Args[1])
		}
		if _, ok := fact.Args[2].(int64); !ok {
			t.Fatalf("security_violation timestamp type = %T, want int64", fact.Args[2])
		}
		if _, err := fact.ToAtom(); err != nil {
			t.Fatalf("security_violation ToAtom: %v", err)
		}
	})

	t.Run("execution_error_2", func(t *testing.T) {
		kernel := &stubKernel{permitted: []Fact{{
			Predicate: "permitted",
			Args:      []any{MangleAtom("/invented_action"), "target", "{}"},
		}}}
		vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
		vs.SetKernel(kernel)
		vs.DisableBootGuard()

		_, err := vs.RouteAction(context.Background(), Fact{
			Predicate: "next_action",
			Args:      []any{"execution-error-action", MangleAtom("/invented_action"), "target"},
		})
		if err == nil {
			t.Fatal("expected unknown action error")
		}

		fact, ok := findAssertedFact(kernel.asserted, "execution_error")
		if !ok {
			t.Fatal("execution_error fact not asserted")
		}
		if len(fact.Args) != 2 {
			t.Fatalf("execution_error arity = %d, want 2", len(fact.Args))
		}
		if fact.Args[0] != "execution-error-action" {
			t.Fatalf("execution_error request ID = %#v", fact.Args[0])
		}
		if _, ok := fact.Args[1].(string); !ok {
			t.Fatalf("execution_error message type = %T, want string", fact.Args[1])
		}
		if _, err := fact.ToAtom(); err != nil {
			t.Fatalf("execution_error ToAtom: %v", err)
		}
	})
}

func TestMaybePruneActionLogsUsesExecutionResultTimestamp(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	now := time.Now()
	oldFact := Fact{
		Predicate: "execution_result",
		Args:      []any{"old-result", MangleAtom("/read_file"), "old.go", true, "not-a-timestamp", now.Add(-16 * time.Minute).Unix()},
	}
	recentFact := Fact{
		Predicate: "execution_result",
		Args:      []any{"recent-result", MangleAtom("/read_file"), "new.go", true, "1", now.Unix()},
	}
	if err := kernel.AssertBatch([]Fact{oldFact, recentFact}); err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}

	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)
	vs.maybePruneActionLogs(now)

	results, err := kernel.Query("execution_result")
	if err != nil {
		t.Fatalf("Query(execution_result): %v", err)
	}
	if _, ok := findFactByFirstArg(results, "old-result"); ok {
		t.Fatal("stale execution_result was not pruned by timestamp slot 5")
	}
	if _, ok := findFactByFirstArg(results, "recent-result"); !ok {
		t.Fatal("recent execution_result was pruned using its output slot")
	}
}

func TestRouteActionReturnsPostValidationFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "verified.txt")
	if err := os.WriteFile(target, []byte("content"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	kernel := &stubKernel{permitted: []Fact{{
		Predicate: "permitted",
		Args:      []any{MangleAtom("/read_file"), target, "{}"},
	}}}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)
	vs.DisableBootGuard()
	vs.validators = NewValidatorRegistry()
	vs.validators.Register(&failingRouteValidator{})

	output, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"validation-failure-action", MangleAtom("/read_file"), target},
	})
	if err == nil || !strings.Contains(err.Error(), "post-action validation failed") {
		t.Fatalf("RouteAction error = %v, want post-validation failure", err)
	}
	if output != "content" {
		t.Fatalf("RouteAction output = %q, want handler output retained", output)
	}
}

func findAssertedFact(facts []Fact, predicate string) (Fact, bool) {
	for _, fact := range facts {
		if fact.Predicate == predicate {
			return fact, true
		}
	}
	return Fact{}, false
}

func findFactByFirstArg(facts []Fact, value string) (Fact, bool) {
	for _, fact := range facts {
		if len(fact.Args) > 0 && fact.Args[0] == value {
			return fact, true
		}
	}
	return Fact{}, false
}
