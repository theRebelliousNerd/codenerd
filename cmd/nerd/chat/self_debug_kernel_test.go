package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

// debugSelfWorkspace binds the process logging to a fresh workspace with
// debug logging on, so problems are written to a real problems log.
func debugSelfWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"logging":{"debug_mode":true,"level":"debug"}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{DebugMode: false})
		logging.ClearInjectedConfig()
	})
	return ws
}

func selfProblemKernelRows(t *testing.T, k core.Kernel, pred string) []types.Fact {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// While debug-self is on, the chat keeps codeNERD's whole problems log in the
// kernel as self_problem facts (history included, counts growing as the log
// does), asserts debug_self_active, and the hot problems come out as
// injectable_context lines. The per-turn digest keeps working beside it.
func TestDebugSelf_ProjectsProblemsLogIntoKernel(t *testing.T) {
	t.Parallel()
	if inOwnProcess(t) {
		return
	}
	debugSelfWorkspace(t)
	klog := logging.Get(logging.CategorySession)
	klog.Error("Tool call run_tests failed: exit status %d", 1)
	klog.Warn("Slow handler took %dms", 410)

	path, _, _ := logging.ProblemsSnapshot()
	if path == "" {
		t.Fatal("no problems log was opened")
	}

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel()
	m.Config = config.DefaultUserConfig()
	// The threshold comes from config, as in production: a warning seen
	// twice is hot.
	m.Config.Logging = &config.LoggingConfig{DebugSelf: true, SelfProblemWarnRepeats: 2}
	m.kernel = k
	m.selfDebug = newSelfDebugWatcher()
	m.selfDebug.prime(path)

	// Activation projects the history that predates it.
	m.syncSelfProblemFacts()
	if got := selfProblemKernelRows(t, k, "self_problem"); len(got) != 2 {
		t.Fatalf("history projected as %d problems, want 2: %v", len(got), got)
	}
	if got := selfProblemKernelRows(t, k, "debug_self_active"); len(got) != 1 {
		t.Fatalf("debug_self_active rows = %d, want 1", len(got))
	}
	if got := pendingSelfProblemsForTurn(m); !got.Empty() {
		t.Fatalf("history must not be a new batch: %+v", got)
	}

	// New problems arrive: the turn reports them and the kernel is refreshed.
	klog.Error("Tool call run_tests failed: exit status %d", 2)
	klog.Warn("Slow handler took %dms", 530)
	got := pendingSelfProblemsForTurn(m)
	if got.TotalErrors != 1 || got.TotalWarnings != 1 {
		t.Fatalf("digest = %d errors, %d warnings, want 1 and 1", got.TotalErrors, got.TotalWarnings)
	}
	rows := selfProblemKernelRows(t, k, "self_problem")
	if len(rows) != 2 {
		t.Fatalf("%d problems after refresh, want 2 (replaced, not appended): %v", len(rows), rows)
	}
	for _, f := range rows {
		if n, _ := types.ExtractInt64(f.Args[3]); n != 2 {
			t.Fatalf("count %v, want 2 for the whole log (history + new): %v", f.Args[3], f)
		}
	}
	m.consumeSelfProblems()

	var injected []string
	for _, f := range selfProblemKernelRows(t, k, "injectable_context") {
		if s := types.ExtractString(f.Args[1]); strings.HasPrefix(s, "Own log ") {
			injected = append(injected, s)
		}
	}
	joined := strings.Join(injected, "\n")
	if len(injected) != 2 ||
		!strings.Contains(joined, "Own log ERROR [session] x2: Tool call run_tests failed: exit status #") ||
		!strings.Contains(joined, "Own log WARN [session] x2: Slow handler took #ms") {
		t.Fatalf("hot problems not injected:\n%s", joined)
	}

	// Nothing new: no re-read, facts unchanged.
	if again := pendingSelfProblemsForTurn(m); !again.Empty() {
		t.Fatalf("quiet turn reported %+v", again)
	}
	if rows := selfProblemKernelRows(t, k, "self_problem"); len(rows) != 2 {
		t.Fatalf("quiet turn changed the facts: %v", rows)
	}
}

// The toggle owns the gate: /debug-self on asserts debug_self_active and off
// retracts it together with the projected problems.
func TestDebugSelfCommand_AssertsAndRetractsKernelFacts(t *testing.T) {
	m := NewTestModel()
	m.workspace = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Config = config.DefaultUserConfig()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	m.kernel = k

	updated, _ := m.handleCmdDebugSelf("/debug-self on", []string{"/debug-self", "on"})
	on := updated.(Model)
	if got := selfProblemKernelRows(t, k, "debug_self_active"); len(got) != 1 {
		t.Fatalf("/debug-self on left %d debug_self_active facts, want 1", len(got))
	}

	if err := problemlog.Replace(k, &problemlog.Report{Problems: []problemlog.Problem{{
		Signature: "boom #", Level: problemlog.LevelError, Category: "kernel", Sample: "boom 1", Count: 1,
	}}}); err != nil {
		t.Fatal(err)
	}
	if got := selfProblemKernelRows(t, k, "self_problem"); len(got) != 1 {
		t.Fatalf("seeded %d problems, want 1", len(got))
	}

	updated, _ = on.handleCmdDebugSelf("/debug-self off", []string{"/debug-self", "off"})
	if updated.(Model).debugSelfEnabled() {
		t.Fatal("/debug-self off left debug-self on")
	}
	for _, pred := range []string{"debug_self_active", "self_problem", "self_problem_seen"} {
		if got := selfProblemKernelRows(t, k, pred); len(got) != 0 {
			t.Fatalf("/debug-self off left %s facts: %v", pred, got)
		}
	}
}
