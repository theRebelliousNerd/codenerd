package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

// Dogfood run 5: a repair loop edited another agent's files. The repair
// guard confines an episode to the turn's write set: an existing file
// outside it is refused with the set named, the set itself stays writable,
// and files the episode creates are allowed (the coverage and pinning
// rounds write their tests inside the episode).
func TestRepairWriteGuard_ConfinesAnEpisodeToTheWriteSet(t *testing.T) {
	ws := t.TempDir()
	own := filepath.Join(ws, "a.go")
	foreign := filepath.Join(ws, "other.go")
	for _, p := range []string{own, foreign} {
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	guard := RepairWriteGuard(ws, []string{"a.go"}, nil)
	ctx := context.Background()

	if err := guard(ctx, []string{own}); err != nil {
		t.Errorf("in-set write refused: %v", err)
	}
	if err := guard(ctx, []string{filepath.Join(ws, "new_test.go")}); err != nil {
		t.Errorf("created file refused: %v", err)
	}
	err := guard(ctx, []string{foreign})
	if err == nil {
		t.Fatal("existing file outside the write set allowed")
	}
	if !strings.Contains(err.Error(), "a.go") || !strings.Contains(err.Error(), "outside that set") {
		t.Errorf("refusal %q neither names the set nor the boundary", err.Error())
	}
}

// The repair guard chains the guard already on the context (the campaign
// turn's leases): the outer refusal wins, even inside the write set.
func TestRepairWriteGuard_ParentRefusalWins(t *testing.T) {
	ws := t.TempDir()
	own := filepath.Join(ws, "a.go")
	if err := os.WriteFile(own, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := func(context.Context, []string) error { return errors.New("held by campaign task /task_other") }
	if err := RepairWriteGuard(ws, []string{"a.go"}, parent)(context.Background(), []string{own}); err == nil ||
		!strings.Contains(err.Error(), "held by campaign task") {
		t.Fatalf("err = %v, want the parent's refusal", err)
	}
}

// Through the tool batch, as a repair round runs it: the outside write is
// refused before it runs, leaves no preimage and no written path, and the
// foreign file on disk is untouched.
func TestRepairWriteGuard_RefusedRepairWriteNeverRuns(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	foreign := filepath.Join(ws, "other.go")
	before := "package other\n\nfunc Other() {}\n"
	if err := os.WriteFile(foreign, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := WithWriteGuard(context.Background(), RepairWriteGuard(ws, []string{"a.go"}, parentWriteGuard(context.Background())))
	e := NewExecutor(&MockKernel{}, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = ws
	e.config.EnableSafetyGate = false
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"edit_file"}}
	result := &ExecutionResult{WrittenPaths: []string{"a.go"}}

	results, _ := e.executeToolBatch(ctx, []types.ToolCall{
		{ID: "r1", Name: "edit_file", Input: map[string]any{"path": "other.go", "old_text": "func Other() {}", "new_text": "func Other() int { return 1 }"}},
	}, cfg, result)

	if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "outside that set") {
		t.Fatalf("results = %+v, want the refusal naming the boundary", results)
	}
	if data, _ := os.ReadFile(foreign); string(data) != before {
		t.Fatalf("the refused repair edit landed: %q", data)
	}
	if _, snapped := result.PreWriteContents["other.go"]; snapped {
		t.Fatal("a refused repair write left a preimage")
	}
	if len(result.WrittenPaths) != 1 || result.WrittenPaths[0] != "a.go" {
		t.Fatalf("WrittenPaths = %v, want the write set unchanged", result.WrittenPaths)
	}
}

// A file the turn writes after the pre-gate snapshot is still the turn's.
// The refusal keeps naming the frozen set, not the files created since.
func TestRepairWriteGuard_AllowsWhatTheTurnWroteAfterTheSnapshot(t *testing.T) {
	ws := t.TempDir()
	later := filepath.Join(ws, "made.go")
	foreign := filepath.Join(ws, "other.go")
	for _, p := range []string{later, foreign} {
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	live := []string{"a.go"}
	guard := repairWriteGuard(ws, []string{"a.go"}, func() []string { return live }, nil)
	if err := guard(context.Background(), []string{later}); err == nil {
		t.Fatal("a file outside the frozen set and the live set was allowed")
	}
	live = append(live, "made.go")
	if err := guard(context.Background(), []string{later}); err != nil {
		t.Fatalf("a file the turn wrote after the snapshot was refused: %v", err)
	}
	err := guard(context.Background(), []string{foreign})
	if err == nil || !strings.Contains(err.Error(), "a.go") || !strings.Contains(err.Error(), "outside that set") {
		t.Fatalf("err = %v, want the frozen set named", err)
	}
	if strings.Contains(err.Error(), "made.go") {
		t.Fatalf("refusal %q names a file the turn wrote later as if it were the boundary", err.Error())
	}
}

func TestNamedWriteSet_StaysAtThePreGateSnapshot(t *testing.T) {
	result := &ExecutionResult{WrittenPaths: []string{"a.go"}}
	ctx := WithTurnWriteSet(context.Background(), result.WrittenPaths)
	result.WrittenPaths = append(result.WrittenPaths, "b.go")
	if got := namedWriteSet(ctx, result); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("named = %v, want the pre-gate snapshot [a.go]", got)
	}
	owned := turnOwnedPaths(ctx, result)
	if len(owned) != 2 || owned[0] != "a.go" || owned[1] != "b.go" {
		t.Fatalf("owned = %v, want a.go then b.go", owned)
	}
	if got := namedWriteSet(context.Background(), result); len(got) != 2 {
		t.Fatalf("fallback = %v, want current WrittenPaths", got)
	}
}

// The session write path refuses an edit that hides a Go file from the
// build, through the same guard every write mutation passes. No path guard
// is installed: the refusal is the constraint, not the write set. GOFLAGS
// and CGO_ENABLED are pinned so the verdicts hold whatever the ambient
// toolchain environment carries.
func TestGuardWrite_RefusesAddedBuildExclusion(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	e := NewExecutor(&MockKernel{}, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = ws
	e.config.EnableSafetyGate = false
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "edit_file", "insert_lines", "create_file"}}
	ctx := context.Background()
	otherOS := "linux"
	if runtime.GOOS == "linux" {
		otherOS = "windows"
	}

	t.Run("edit ignore", func(t *testing.T) {
		path := filepath.Join(ws, "a.go")
		before := "package p\n\nfunc A() int { return 1 }\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "e1", Name: "edit_file",
			Input: map[string]any{"path": "a.go", "old_text": "package p\n", "new_text": "//go:build ignore\n\npackage p\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "excludes") {
			t.Fatalf("results = %+v, want the exclusion refusal", results)
		}
		if data, _ := os.ReadFile(path); string(data) != before {
			t.Fatalf("the refused edit landed: %q", data)
		}
	})

	t.Run("write new ignore", func(t *testing.T) {
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "w1", Name: "write_file",
			Input: map[string]any{"path": "hidden.go", "content": "//go:build ignore\n\npackage hidden\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "ignore") {
			t.Fatalf("results = %+v, want the exclusion refusal", results)
		}
		if _, err := os.Stat(filepath.Join(ws, "hidden.go")); !os.IsNotExist(err) {
			t.Fatalf("the refused write created hidden.go (stat %v)", err)
		}
	})

	t.Run("create ignore", func(t *testing.T) {
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "c1", Name: "create_file",
			Input: map[string]any{"path": "created.go", "source": "//go:build ignore\n\npackage p\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "excludes") {
			t.Fatalf("results = %+v, want the exclusion refusal", results)
		}
		if _, err := os.Stat(filepath.Join(ws, "created.go")); !os.IsNotExist(err) {
			t.Fatalf("the refused create landed (stat %v)", err)
		}
	})

	t.Run("already ignored", func(t *testing.T) {
		path := filepath.Join(ws, "kept.go")
		before := "//go:build ignore\n\npackage p\n\nfunc A() int { return 1 }\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "k1", Name: "edit_file",
			Input: map[string]any{"path": "kept.go", "old_text": "return 1", "new_text": "return 2"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || results[0].IsError {
			t.Fatalf("results = %+v, want the edit of an already-excluded file", results)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "go:build ignore") || !strings.Contains(string(data), "return 2") {
			t.Fatalf("already-excluded file = %q", data)
		}
	})

	t.Run("plus build ignore", func(t *testing.T) {
		path := filepath.Join(ws, "plus.go")
		before := "package p\n\nfunc A() {}\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "p1", Name: "edit_file",
			Input: map[string]any{"path": "plus.go", "old_text": "package p\n", "new_text": "// +build ignore\n\npackage p\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "excludes") {
			t.Fatalf("results = %+v, want the +build refusal", results)
		}
		if data, _ := os.ReadFile(path); string(data) != before {
			t.Fatalf("the refused edit landed: %q", data)
		}
	})

	t.Run("other goos", func(t *testing.T) {
		path := filepath.Join(ws, "os.go")
		before := "package p\n\nfunc A() {}\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "o1", Name: "edit_file",
			Input: map[string]any{"path": "os.go", "old_text": "package p\n", "new_text": "//go:build " + otherOS + "\n\npackage p\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "excludes") {
			t.Fatalf("results = %+v, want the %s constraint refused", results, otherOS)
		}
		if data, _ := os.ReadFile(path); string(data) != before {
			t.Fatalf("the refused edit landed: %q", data)
		}
	})

	t.Run("included platform", func(t *testing.T) {
		path := filepath.Join(ws, "here.go")
		before := "package p\n\nfunc A() int { return 1 }\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "h1", Name: "edit_file",
			Input: map[string]any{"path": "here.go", "old_text": "package p\n", "new_text": "//go:build " + runtime.GOOS + "\n\npackage p\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || results[0].IsError {
			t.Fatalf("results = %+v, want a constraint the current build satisfies", results)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "go:build "+runtime.GOOS) {
			t.Fatalf("included constraint did not land: %q", data)
		}
	})

	t.Run("insert header", func(t *testing.T) {
		path := filepath.Join(ws, "ins.go")
		before := "package p\n\nfunc A() {}\n"
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{{
			ID: "i1", Name: "insert_lines",
			Input: map[string]any{"path": "ins.go", "after_line": 0, "content": "//go:build ignore\n"},
		}}, cfg, &ExecutionResult{})
		if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "excludes") {
			t.Fatalf("results = %+v, want the inserted constraint refused", results)
		}
		if data, _ := os.ReadFile(path); string(data) != before {
			t.Fatalf("the refused insert landed: %q", data)
		}
	})
}
