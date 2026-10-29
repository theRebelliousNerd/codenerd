package system

import (
	"context"
	"fmt"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func TestEvaluatePolicy_EmitsEveryDerivedAction(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	exec := NewExecutivePolicyShard()
	exec.SetParentKernel(kernel)
	exec.DisableBootGuard()

	const n = 8
	for i := 0; i < n; i++ {
		if err := kernel.Assert(core.Fact{
			Predicate: "delegate_task",
			Args:      []any{"/coder", fmt.Sprintf("internal/file_%d.go", i), "/pending"},
		}); err != nil {
			t.Fatalf("assert delegate_task %d: %v", i, err)
		}
	}

	if err := exec.evaluatePolicy(context.Background()); err != nil {
		t.Fatalf("evaluatePolicy: %v", err)
	}
	facts, err := kernel.Query("pending_action")
	if err != nil {
		t.Fatalf("Query(pending_action) error = %v", err)
	}
	got := 0
	for _, f := range facts {
		if len(f.Args) > 1 && types.ExtractString(f.Args[1]) == "/delegate_coder" {
			got++
		}
	}
	if got < n {
		t.Fatalf("pending_action /delegate_coder = %d, want at least %d (a tick used to keep 5)", got, n)
	}
}

func TestExecutiveQueryNextActions_TranslatesDelegateTask(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	exec := NewExecutivePolicyShard()
	exec.SetParentKernel(kernel)
	exec.DisableBootGuard()

	if err := kernel.Assert(core.Fact{
		Predicate: "delegate_task",
		Args:      []any{"/reviewer", "internal/core/shards/agents.go", "/pending"},
	}); err != nil {
		t.Fatalf("assert delegate_task: %v", err)
	}

	actions, err := exec.queryNextActions()
	if err != nil {
		t.Fatalf("queryNextActions() error = %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected delegated action")
	}

	found := false
	for _, action := range actions {
		if action.Action != "/delegate_reviewer" {
			continue
		}
		found = true
		if action.Target != "internal/core/shards/agents.go" {
			t.Fatalf("unexpected delegated target: %q", action.Target)
		}
	}
	if !found {
		t.Fatal("expected /delegate_reviewer action from delegate_task")
	}
}
