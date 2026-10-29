package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
)

func newExecWithRealKernel(t *testing.T, workspace string) *Executor {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	if workspace != "" {
		e.SetConfig(ExecutorConfig{WorkspaceRoot: workspace})
	}
	return e
}

func cfg() *config.EffectiveAgentRuntimeConfig {
	return &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "edit_file", "read_file", "run_command"}}
}

func TestVerifyModularityGuard_NewFileBlocked(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	// Use many params to trigger violation (>5)
	content := manyParamsSource("BadFunc", 10)
	path := filepath.Join(tmp, "new.go")
	// ensure file does NOT exist
	os.Remove(path)
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": content}}
	_, err := e.executeToolCall(context.Background(), call, cfg())
	if err == nil {
		t.Fatalf("expected blocked but got allowed")
	}
	if !strings.Contains(err.Error(), "BadFunc") {
		t.Fatalf("error should name BadFunc, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "too_many_params") {
		t.Fatalf("error should name rule too_many_params, got %q", err.Error())
	}
}

func TestVerifyModularityGuard_PreExistingAllowed(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	content := manyParamsSource("BadFunc", 10)
	path := filepath.Join(tmp, "exist.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": content}}
	reason, blocked := e.modularityGuard(call)
	if blocked {
		t.Fatalf("pre-existing violation should be allowed, got blocked %q", reason)
	}
}

func TestVerifyModularityGuard_SecondViolationBlocked(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	// existing has BadFunc
	existContent := manyParamsSource("BadFunc", 10)
	path := filepath.Join(tmp, "multi.go")
	if err := os.WriteFile(path, []byte(existContent), 0644); err != nil {
		t.Fatal(err)
	}
	// proposed adds SecondBad — strip package header to keep source valid Go
	secondSrc := manyParamsSource("SecondBad", 10)
	secondFunc := strings.TrimPrefix(secondSrc, "package p\n")
	proposed := existContent + secondFunc
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": proposed}}
	reason, blocked := e.modularityGuard(call)
	if !blocked {
		t.Fatalf("expected blocked for second violation")
	}
	if !strings.Contains(reason, "SecondBad") {
		t.Fatalf("should name SecondBad, got %q", reason)
	}
	if strings.Contains(reason, "BadFunc") {
		t.Fatalf("should not name BadFunc, got %q", reason)
	}
}

func TestVerifyModularityGuard_CleanAllowed(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	content := cleanSource()
	path := filepath.Join(tmp, "clean.go")
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": content}}
	reason, blocked := e.modularityGuard(call)
	if blocked {
		t.Fatalf("clean should be allowed, got blocked %q", reason)
	}
}

func TestVerifyModularityGuard_NonGoAllowed(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	content := manyParamsSource("BadFunc", 10)
	path := filepath.Join(tmp, "notgo.txt")
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": content}}
	reason, blocked := e.modularityGuard(call)
	if blocked {
		t.Fatalf("non-go should be allowed, got blocked %q", reason)
	}
}

func TestVerifyModularityGuard_UnparseableAllowed(t *testing.T) {
	tmp := t.TempDir()
	k, _ := core.NewRealKernel()
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(ExecutorConfig{WorkspaceRoot: tmp})
	content := "this is not go {{{"
	path := filepath.Join(tmp, "bad.go")
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": path, "content": content}}
	reason, blocked := e.modularityGuard(call)
	if blocked {
		t.Fatalf("unparseable should be allowed, got blocked %q", reason)
	}
}

func TestVerifyModularityGuard_NoKernelAllowed(t *testing.T) {
	e := &Executor{kernel: nil}
	content := manyParamsSource("BadFunc", 10)
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": "any.go", "content": content}}
	reason, blocked := e.modularityGuard(call)
	if blocked {
		t.Fatalf("no kernel should allow, got blocked %q", reason)
	}
}
