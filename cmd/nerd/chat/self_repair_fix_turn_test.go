package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/session"
	"codenerd/internal/types"
	"codenerd/internal/verification"
)

// fixTurnWorker is the coder of a self-repair fix turn: it writes what the
// script says, records each write in the turn's journal as the session
// executor does at the end of its turn (ProcessWithIntent), and fails the
// way the live fix turn did -- its repair loop gave up with its own new test
// red.
type fixTurnWorker struct {
	mu        sync.Mutex
	workspace string
	writes    map[string]string // workspace-relative path -> new content
	failures  []string
	after     func() // runs once the turn has written and recorded
	journaled bool
}

func (w *fixTurnWorker) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	return "", fmt.Errorf("unexpected string execution")
}
func (w *fixTurnWorker) ExecuteWithContext(context.Context, session.TaskRequest, *types.SessionContext, types.SpawnPriority) (string, error) {
	return "", fmt.Errorf("unexpected string execution")
}
func (w *fixTurnWorker) ExecuteObserved(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	return w.ExecuteObservedWithContext(ctx, req, nil, types.PriorityNormal)
}
func (w *fixTurnWorker) ExecuteObservedWithContext(ctx context.Context, _ session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pre := map[string]session.PreImage{}
	var written []string
	for rel, content := range w.writes {
		path := filepath.Join(w.workspace, filepath.FromSlash(rel))
		img := session.PreImage{}
		if data, err := os.ReadFile(path); err == nil {
			img = session.PreImage{Existed: true, Content: string(data)}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return observation.Return{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return observation.Return{}, err
		}
		sum := sha256.Sum256([]byte(content))
		img.LastWriteHash = hex.EncodeToString(sum[:])
		pre[rel] = img
		written = append(written, rel)
	}
	if j := session.WriteJournalFrom(ctx); j != nil {
		j.Record(w.workspace, written, pre)
		j.NoteTestFailures(w.failures)
		w.journaled = true
	}
	if w.after != nil {
		w.after()
	}
	return observation.Return{Output: "the repair loop did not converge"}, fmt.Errorf("execution failed: %w: edits broke the tests and the repair loop did not converge after 3 attempts", session.ErrVerificationFailed)
}
func (w *fixTurnWorker) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", fmt.Errorf("unexpected async execution")
}
func (w *fixTurnWorker) GetResult(string) (string, bool, error) {
	return "", false, fmt.Errorf("unexpected result lookup")
}
func (w *fixTurnWorker) WaitForResult(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected wait")
}

var _ session.ObservedTaskExecutor = (*fixTurnWorker)(nil)

// A self-repair fix turn that does not end installed leaves the tree as it
// found it: the files it changed get their bytes back and the test it created
// is removed, while a file another session changed after the turn wrote it,
// and a file the turn never touched, keep that session's edit. The undone
// attempt is saved with the tests that failed, the ledger names it, the
// failure's own log lines are the attempt's (not due), and the retry is told
// what was tried and what failed. Observed live 2026-11-12: the first
// autonomous fix turn left its edits and its red test in the tree.
func TestSelfRepair_AFailedFixTurnUndoesOnlyItsOwnWritesAndTheRetryIsToldWhy(t *testing.T) {
	t.Parallel()
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "internal/session/executor.go", Confidence: 0.9,
		Response: "Fixing the nil check the diagnosis found.",
		Brief:    &perception.Brief{Goal: "fix the root cause the self-diagnosis found", Ready: true},
	}
	r := newSelfRepairRig(t, "", "20261028_061954_problems.log", nilSessionError, fix)
	r.exec.response = "Root cause: internal/session/executor.go:120 dereferences a nil session. Fix: guard it."
	ws := r.m.workspace
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(rel string) (string, bool) {
		data, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
		return string(data), err == nil
	}
	const executorBefore = "package session\n\n// before the fix\n"
	const sharedBefore = "package session\n\n// shared before\n"
	const foreign = "package session\n\n// another session's edit\n"
	const usersOwn = "the user's own notes, written while the fix ran\n"
	write("internal/session/executor.go", executorBefore)
	write("internal/session/shared.go", sharedBefore)
	worker := &fixTurnWorker{
		workspace: ws,
		writes: map[string]string{
			"internal/session/executor.go":      "package session\n\n// the attempt's edit\n",
			"internal/session/executor_test.go": "package session\n\n// the attempt's new test\n",
			"internal/session/shared.go":        "package session\n\n// the attempt's shared edit\n",
		},
		failures: []string{"codenerd/internal/session.TestNilGuard: executor_test.go:9: failed to analyze program: missing arg atoms"},
		// Another session edits shared.go after the turn wrote it, and the
		// user writes a file the turn never touched.
		after: func() {
			write("internal/session/shared.go", foreign)
			write("NOTES.txt", usersOwn)
		},
	}
	r.m.verifier = verification.NewTaskVerifier(r.m.client, nil, nil, config.DefaultJITConfig())
	r.m.verifier.SetKernel(r.m.kernel)
	r.m.verifier.SetTaskExecutor(worker)

	m, cmd := landPlan(r.m.handleSelfRepairTick(selfRepairTickMsg{gen: 0}))
	jobs := m.background.list()
	if len(jobs) != 1 {
		t.Fatalf("no self-repair job started: %+v", jobs)
	}
	job := jobs[0].ID
	sig := r.ledger(t).Attempts()[0].Target
	done := runBatchFor[backgroundDoneMsg](t, cmd)
	next, followUp := m.Update(done)
	m = next.(Model)
	if followUp == nil || !m.isLoading {
		t.Fatal("the chain stopped before its fix turn")
	}
	if ws := r.ledger(t).FixWindows(); len(ws) != 1 || ws[0].Job != job || ws[0].EndUnix != 0 {
		t.Fatalf("fix windows while the fix runs = %+v, want one open for %s", ws, job)
	}

	// The fix turn runs and fails; the problem it logs lies inside its window.
	r.advance(10 * time.Second)
	inside := r.m.selfRepair.clock().Local().Format("2006/01/02 15:04:05.000")
	logPath := r.m.selfRepair.problemsLog()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(inside + " [ERROR] [api] [meta] CompleteWithSystem failed after 120s: context deadline exceeded\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	failed := awaitProcessStream(t, followUp())
	if !worker.journaled {
		t.Fatal("the fix turn ran without a write journal on its context")
	}
	r.advance(10 * time.Second)
	next, _ = m.Update(failed)
	m = next.(Model)

	if got, _ := read("internal/session/executor.go"); got != executorBefore {
		t.Fatalf("executor.go = %q, want it as the fix found it", got)
	}
	if _, ok := read("internal/session/executor_test.go"); ok {
		t.Fatal("the red test the fix created is still in the tree")
	}
	if got, _ := read("internal/session/shared.go"); got != foreign {
		t.Fatalf("shared.go = %q: another session's edit was reverted", got)
	}
	if got, _ := read("NOTES.txt"); got != usersOwn {
		t.Fatalf("NOTES.txt = %q: the user's own file was touched", got)
	}

	a, _ := r.ledger(t).Get(sig)
	wantDiff := ".nerd/self_repair/attempts/" + job + ".diff"
	if a.Status != problemlog.RepairFailed || a.AttemptDiff != wantDiff || !strings.Contains(a.Note, wantDiff) ||
		!strings.Contains(a.Note, "internal/session/shared.go (changed since") || len(a.FailedTests) != 1 {
		t.Fatalf("ledger after the failed fix = %+v", a)
	}
	saved, ok := read(wantDiff)
	if !ok || !strings.Contains(saved, "the attempt's new test") || !strings.Contains(saved, "TestNilGuard") {
		t.Fatalf("the saved attempt lacks the diff or the failing test:\n%s", saved)
	}
	if !strings.Contains(lastAssistant(m), "Its edits were undone") || !strings.Contains(lastAssistant(m), wantDiff) {
		t.Fatalf("the user is not told the edits were undone: %q", lastAssistant(m))
	}
	if ws := r.ledger(t).FixWindows(); len(ws) != 1 || ws[0].EndUnix == 0 {
		t.Fatalf("fix window after the turn = %+v, want it closed", ws)
	}

	// The retry, after the cooldown, is told what was tried and what failed.
	r.advance(time.Duration(config.DefaultSelfRepairCooldownSec+1) * time.Second)
	m, _ = stepSelfRepair(t, m)
	var retry *backgroundJob
	for _, j := range m.background.list() {
		if j.ID != job {
			j := j
			retry = &j
		}
	}
	if retry == nil {
		t.Fatalf("no retry started; ledger %+v", r.ledger(t).Attempts())
	}
	// The timeout the fix turn logged is the attempt's: not a target.
	for _, target := range retry.selfRepairTargets {
		if strings.Contains(target, "CompleteWithSystem") {
			t.Fatalf("the retry targets the failed fix turn's own log line: %v", retry.selfRepairTargets)
		}
	}
	if !strings.Contains(strings.Join(retry.selfRepairTargets, "|"), "nil session in executor dispatch") {
		t.Fatalf("the retry does not target the problem that recurs outside the fix: %v", retry.selfRepairTargets)
	}
	if !strings.Contains(retry.Task, "saved at "+wantDiff) || !strings.Contains(retry.Task, "TestNilGuard: executor_test.go:9: failed to analyze program") {
		t.Fatalf("the retry is not told what the attempt changed and broke:\n%s", retry.Task)
	}
}

// A turn whose edits failed verification ended with a verdict on the
// workspace, which the turn reports itself: it is logged at INFO, never in the
// problems log debug-self reads as codeNERD's own faults. Any other turn error
// still is. Live 2026-11-12: the failed fix turn's "TUI error: ... post-edit
// verification failed" became the next self-repair's target.
func TestHandleTurnError_AVerificationVerdictIsNotAProblem(t *testing.T) {
	t.Parallel()
	if inOwnProcess(t) {
		return
	}
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(`{"logging":{"debug_mode":true,"level":"debug"}}`), 0o644); err != nil {
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
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	verdict := failedVerifiedDelegation("verified execution failed", &verification.ExecutionError{Attempt: 1,
		Err: fmt.Errorf("execution failed: %w", fmt.Errorf("%w: edits broke the tests and the repair loop did not converge after 3 attempts", session.ErrVerificationFailed))})
	_, _ = m.handleTurnError(verdict)
	_, _ = m.handleTurnError(fmt.Errorf("shard execution failed: nil session in executor dispatch"))
	logging.CloseAll()

	read := func(glob string) string {
		matches, err := filepath.Glob(filepath.Join(ws, ".nerd", "logs", glob))
		if err != nil || len(matches) == 0 {
			t.Fatalf("no %s: %v", glob, err)
		}
		var b strings.Builder
		for _, p := range matches {
			data, _ := os.ReadFile(p)
			b.Write(data)
		}
		return b.String()
	}
	problems := read("*_problems.log")
	if strings.Contains(problems, "post-edit verification failed") {
		t.Fatalf("the verification verdict is in the problems log:\n%s", problems)
	}
	if !strings.Contains(problems, "[ERROR] [session] TUI error: shard execution failed: nil session") {
		t.Fatalf("a real turn error is missing from the problems log:\n%s", problems)
	}
	if s := read("*_session.log"); !strings.Contains(s, "post-edit verification failed") {
		t.Fatalf("the verdict is not in the session log at all:\n%s", s)
	}
}
