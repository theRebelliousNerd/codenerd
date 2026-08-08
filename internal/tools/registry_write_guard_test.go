package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The defect these guard (F-SEC-4, from codeNERD's own security review of
// internal/tools/core/file_ops.go): nerd.md's forbidden-path protection lived
// in exactly two callers — session.Executor.executeToolCall and
// VirtualStore.executeAction — and the tools themselves enforced nothing.
//
// The registry is reachable process-globally via tools.Execute, so any code
// path calling it directly wrote protected paths unchecked. Not hypothetical:
// the codebase already suffered this once and documents it at
// virtual_store_routing.go:317 ("a shard could write .nerd/config.json"), fixed
// then by adding the SECOND caller-side gate rather than closing the class.

func guardTestTool(name string, ran *bool) *Tool {
	return &Tool{
		Name:        name,
		Description: "test tool",
		Category:    CategoryCode,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			*ran = true
			return "executed", nil
		},
		Schema: ToolSchema{Required: []string{"path"}},
	}
}

func TestWriteGuard_RefusalPreventsExecution(t *testing.T) {
	r := NewRegistry()
	ran := false
	if err := r.Register(guardTestTool("write_file", &ran)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	sentinel := errors.New("blocked by nerd.md: .nerd/config.json is write-protected")
	r.SetWriteGuard(func(_ context.Context, name string, args map[string]any) error {
		if name == "write_file" {
			return sentinel
		}
		return nil
	})

	_, err := r.Execute(context.Background(), "write_file", map[string]any{"path": ".nerd/config.json"})
	if err == nil {
		t.Fatal("guarded tool executed without error")
	}
	if !strings.Contains(err.Error(), "write-protected") {
		t.Errorf("error lost the guard's reason: %v", err)
	}
	// The decisive assertion: refusing must mean the tool body never ran.
	if ran {
		t.Error("the tool executed despite the guard refusing it")
	}
}

// The guard must not be a blanket veto — unrelated tools keep working.
func TestWriteGuard_AllowsUnguardedTools(t *testing.T) {
	r := NewRegistry()
	ran := false
	if err := r.Register(guardTestTool("read_file", &ran)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	r.SetWriteGuard(func(_ context.Context, name string, _ map[string]any) error {
		if name == "write_file" {
			return errors.New("blocked")
		}
		return nil
	})

	if _, err := r.Execute(context.Background(), "read_file", map[string]any{"path": "x"}); err != nil {
		t.Fatalf("an unguarded tool was refused: %v", err)
	}
	if !ran {
		t.Error("read_file did not execute")
	}
}

// This is the bypass itself: the process-global registry behind tools.Execute.
func TestWriteGuard_CoversTheGlobalRegistry(t *testing.T) {
	ran := false
	name := "guard_probe_write"
	if err := Register(guardTestTool(name, &ran)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { SetGlobalWriteGuard(nil) })

	SetGlobalWriteGuard(func(_ context.Context, toolName string, _ map[string]any) error {
		if toolName == name {
			return errors.New("blocked by nerd.md")
		}
		return nil
	})

	if _, err := Execute(context.Background(), name, map[string]any{"path": "p"}); err == nil {
		t.Fatal("tools.Execute bypassed the write guard")
	}
	if ran {
		t.Error("the tool executed despite the global guard refusing it")
	}
}

// No guard installed must behave exactly as before, or every existing caller
// changes behaviour as a side effect of this feature.
func TestWriteGuard_AbsentGuardIsANoOp(t *testing.T) {
	r := NewRegistry()
	ran := false
	if err := r.Register(guardTestTool("write_file", &ran)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := r.Execute(context.Background(), "write_file", map[string]any{"path": "x"}); err != nil {
		t.Fatalf("execution failed with no guard installed: %v", err)
	}
	if !ran {
		t.Error("tool did not execute with no guard installed")
	}
}
