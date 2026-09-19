package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// External audit F4: a write guard on the context is asked about every write
// call's targets, absolute, before the tool runs. A refused write never runs,
// leaves no preimage and no written path, and its reason is the tool result
// the model reads.
func TestWriteGuard_ARefusedWriteNeverRuns(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	var asked []string
	ctx := WithWriteGuard(context.Background(), func(_ context.Context, paths []string) error {
		asked = append(asked, paths...)
		if strings.HasSuffix(filepath.ToSlash(paths[0]), "held.go") {
			return errors.New("held by campaign task /task_other")
		}
		return nil
	})
	e := NewExecutor(&MockKernel{}, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = ws
	e.config.EnableSafetyGate = false
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}}
	result := &ExecutionResult{}

	results, _ := e.executeToolBatch(ctx, []types.ToolCall{
		{ID: "held", Name: "write_file", Input: map[string]any{"path": "held.go", "content": "package x\n"}},
		{ID: "free", Name: "write_file", Input: map[string]any{"path": "free.go", "content": "package x\n"}},
	}, cfg, result)

	if len(results) != 2 || !results[0].IsError || !strings.Contains(results[0].Content, "held by campaign task /task_other") {
		t.Fatalf("results = %+v, want the first refused with the guard's reason", results)
	}
	if _, err := os.Stat(filepath.Join(ws, "held.go")); !os.IsNotExist(err) {
		t.Fatalf("held.go was written after the guard refused it (stat: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "free.go")); err != nil {
		t.Fatalf("free.go was not written: %v", err)
	}
	if result.SuccessfulWriteTools != 1 || len(result.WrittenPaths) != 1 || result.WrittenPaths[0] != "free.go" {
		t.Fatalf("SuccessfulWriteTools=%d WrittenPaths=%v, want only free.go", result.SuccessfulWriteTools, result.WrittenPaths)
	}
	if _, snapped := result.PreWriteContents["held.go"]; snapped {
		t.Fatal("a refused write left a preimage")
	}
	for _, p := range asked {
		if !filepath.IsAbs(p) {
			t.Errorf("the guard was asked about %q, want absolute paths", p)
		}
	}
}
