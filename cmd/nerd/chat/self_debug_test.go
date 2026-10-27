package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

func writeProblems(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendProblems(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func TestSelfDebugWatcher_DiffDedupeAndOffsetAdvance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_problems.log")
	writeProblems(t, path,
		"2026/10/27 10:00:00.000001 [ERROR] [kernel] kernel blew up 42\n"+
			"2026/10/27 10:00:01.000001 [WARN] [session] hydrate failed for /home/u/a.db after 12ms\n"+
			"2026/10/27 10:00:02.000001 [WARN] [session] hydrate failed for /home/u/b.db after 99ms\n"+
			"2026/10/27 10:00:03.000001 [INFO] [session] not a problem\n"+
			"garbage line\n"+
			"2026/10/27 10:00:04.000001 [WARN] [session] user-visible warning: debug-self: 1 new error(s), 1 warning kind(s) since last turn — in context\n")

	w := newSelfDebugWatcher()
	got := w.collect(path)
	if got.TotalErrors != 1 || got.TotalWarnings != 2 {
		t.Fatalf("totals = %d errors, %d warnings; want 1, 2 (self-generated note excluded)", got.TotalErrors, got.TotalWarnings)
	}
	if len(got.WarnKinds) != 1 || got.WarnKinds[0].Count != 2 {
		t.Fatalf("warn digest = %+v, want one kind x2 (paths/numbers normalized)", got.WarnKinds)
	}
	if !strings.Contains(got.WarnKinds[0].Signature, "<path>") {
		t.Fatalf("signature must strip volatile paths: %q", got.WarnKinds[0].Signature)
	}
	if got.SummaryLine() != "debug-self: 1 new error(s), 1 warning kind(s) since last turn — in context" {
		t.Fatalf("summary = %q", got.SummaryLine())
	}

	// Re-collect without consume: offset advanced (no duplicates), batch kept.
	again := w.collect(path)
	if again.TotalErrors != 1 || again.TotalWarnings != 2 {
		t.Fatalf("re-collect duplicated lines: %+v", again)
	}
	// After consume, only lines appended later are new.
	w.consume()
	appendProblems(t, path, "2026/10/27 10:01:00.000001 [ERROR] [api] second failure\n")
	next := w.collect(path)
	if next.TotalErrors != 1 || next.TotalWarnings != 0 || !strings.Contains(next.Errors[0].Message, "second failure") {
		t.Fatalf("post-consume batch = %+v, want only the appended error", next)
	}
	w.consume()
	if empty := w.collect(path); !empty.Empty() {
		t.Fatalf("no new lines must yield an empty batch: %+v", empty)
	}
}

func TestSelfDebugWatcher_PrimeSkipsHistoryAndPartialLineWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_problems.log")
	writeProblems(t, path, "2026/10/27 10:00:00.000001 [ERROR] [kernel] before activation\n")
	w := newSelfDebugWatcher()
	w.prime(path)
	if got := w.collect(path); !got.Empty() {
		t.Fatalf("prime must skip pre-activation lines: %+v", got)
	}
	appendProblems(t, path, "2026/10/27 10:02:00.000001 [ERROR] [kernel] partial")
	if got := w.collect(path); !got.Empty() {
		t.Fatalf("partial line must wait for its newline: %+v", got)
	}
	appendProblems(t, path, " line done\n")
	if got := w.collect(path); got.TotalErrors != 1 {
		t.Fatalf("completed line must surface: %+v", got)
	}
	// Truncation/rotation in place: smaller file restarts at 0.
	w.consume()
	writeProblems(t, path, "2026/10/27 10:03:00.000001 [WARN] [kernel] fresh file\n")
	if got := w.collect(path); got.TotalWarnings != 1 {
		t.Fatalf("truncated file must be re-read from 0: %+v", got)
	}
}

func TestSelfDebugWatcher_RenderIsCappedDedupedDigestWithRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_problems.log")
	var b strings.Builder
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&b, "2026/10/27 10:00:00.000001 [ERROR] [kernel] failure number %d in /some/long/path/file.go\n", i)
	}
	for i := 0; i < 23; i++ {
		fmt.Fprintf(&b, "2026/10/27 10:00:00.000001 [WARN] [kernel] unique-kind-%c problem\n", 'a'+i)
	}
	writeProblems(t, path, b.String())
	w := newSelfDebugWatcher()
	got := w.collect(path)
	// Raw lines stay available for stage 2, with the batch's byte range.
	if len(got.Errors) != 25 || got.StartOffset != 0 || got.EndOffset != int64(len(b.String())) {
		t.Fatalf("batch = %d raw errors, bytes %d..%d; want 25 and 0..%d", len(got.Errors), got.StartOffset, got.EndOffset, len(b.String()))
	}
	rendered := got.Render()
	if len(rendered) > maxSelfRenderChars {
		t.Fatalf("main-context digest exceeds hard cap: %d > %d chars\n%s", len(rendered), maxSelfRenderChars, rendered)
	}
	if !strings.Contains(rendered, "## Debug-Self: New codeNERD Log Problems") ||
		!strings.Contains(rendered, "ref: `"+path+"` bytes 0..") ||
		!strings.Contains(rendered, "failure number # in <path> x25") {
		t.Fatalf("digest must carry deduped signatures + byte-range ref:\n%s", rendered)
	}
	// No full raw lines in the main-context digest.
	if strings.Contains(rendered, "failure number 17") || strings.Contains(rendered, "/some/long/path/file.go") {
		t.Fatalf("raw log lines must not enter main context:\n%s", rendered)
	}
	if (PendingProblems{}).Render() != "" {
		t.Fatal("empty batch must render nothing")
	}

	// A signature reported last turn collapses to the tally line next turn.
	w.consume()
	appendProblems(t, path, "2026/10/27 10:05:00.000001 [ERROR] [kernel] failure number 777 in /other/file.go\n")
	next := w.collect(path)
	if len(next.ErrorKinds) != 1 || !next.ErrorKinds[0].Repeat {
		t.Fatalf("recurring signature must be marked repeat: %+v", next.ErrorKinds)
	}
	if next.StartOffset != int64(len(b.String())) {
		t.Fatalf("next batch StartOffset=%d, want %d", next.StartOffset, len(b.String()))
	}
	r2 := next.Render()
	if !strings.Contains(r2, "repeats of already-reported: 1 signature(s), 1 line(s)") ||
		strings.Contains(r2, "failure number #") {
		t.Fatalf("repeat signature must collapse to the tally only:\n%s", r2)
	}
}

func TestPendingSelfProblemsForTurn_DisabledCollectsNothing(t *testing.T) {
	m := NewTestModel()
	m.selfDebug = newSelfDebugWatcher()
	if got := pendingSelfProblemsForTurn(m); !got.Empty() {
		t.Fatalf("debug-self off must return no problems: %+v", got)
	}
	m.Config.Logging = &config.LoggingConfig{DebugSelf: true}
	// Enabled but no problems path in this process: still empty, no panic.
	if got := pendingSelfProblemsForTurn(m); !got.Empty() && got.Path == "" {
		t.Fatalf("unexpected batch without a problems log: %+v", got)
	}
}

func TestDebugSelfInjection_RendersOnlyWhenNonEmpty(t *testing.T) {
	intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "x"}
	batch := PendingProblems{
		Path: "/tmp/run_problems.log", StartOffset: 10, EndOffset: 99,
		Errors: []SelfProblem{{Level: "ERROR", Category: "kernel", Message: "raw-sentinel-line-999"}},
		ErrorKinds: []SelfProblemKind{{
			Signature: "sentinel-boom #", Category: "kernel", Example: "raw-sentinel-line-999",
			Count: 1, FirstSeen: "2026/10/27 10:00:00", LastSeen: "2026/10/27 10:00:00",
		}},
		TotalErrors: 1,
	}

	client := NewMockLLMClient()
	_, _ = articulateWithConversation(context.Background(), client, intent,
		payloadForArticulation(intent, nil), nil, "sys",
		&ConversationContext{DebugSelfReport: batch.Render()}, nil, nil)
	client.mu.Lock()
	withReport := client.lastPrompt
	client.mu.Unlock()
	if !strings.Contains(withReport, "## Debug-Self: New codeNERD Log Problems") ||
		!strings.Contains(withReport, "sentinel-boom #") ||
		!strings.Contains(withReport, "bytes 10..99") ||
		!strings.Contains(withReport, "codeNERD's own new log problems since your last turn") {
		t.Fatalf("debug-self digest missing from articulation prompt:\n%s", withReport)
	}
	if strings.Contains(withReport, "raw-sentinel-line-999") {
		t.Fatalf("raw log lines must not enter the main prompt:\n%s", withReport)
	}

	client2 := NewMockLLMClient()
	_, _ = articulateWithConversation(context.Background(), client2, intent,
		payloadForArticulation(intent, nil), nil, "sys",
		&ConversationContext{}, nil, nil)
	client2.mu.Lock()
	without := client2.lastPrompt
	client2.mu.Unlock()
	if strings.Contains(without, "Debug-Self:") {
		t.Fatalf("empty report must inject nothing:\n%s", without)
	}
}

func TestDebugSelfCommand_TogglesPersistsAndImpliesSessionYolo(t *testing.T) {
	m := NewTestModel()
	m.workspace = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Config = config.DefaultUserConfig()

	updated, _ := m.handleCmdDebugSelf("/debug-self", []string{"/debug-self"})
	on := updated.(Model)
	if !on.debugSelfEnabled() {
		t.Fatal("bare /debug-self must toggle on from off")
	}
	if !on.yoloEnabled() || !on.yoloViaDebugSelf {
		t.Fatal("debug-self on must imply session yolo via debug_self")
	}
	raw, err := os.ReadFile(filepath.Join(m.workspace, ".nerd", "config.json"))
	if err != nil {
		t.Fatalf("debug-self must persist: %v", err)
	}
	if !strings.Contains(string(raw), `"debug_self": true`) {
		t.Fatalf("persisted config missing debug_self:true: %s", raw[:300])
	}
	if strings.Contains(string(raw), `"yolo": true`) {
		t.Fatal("debug-self must not persist yolo:true")
	}

	updated, _ = on.handleCmdDebugSelf("/debug-self status", []string{"/debug-self", "status"})
	statusModel := updated.(Model)
	last := statusModel.history[len(statusModel.history)-1].Content
	if !strings.Contains(last, "Debug-self mode is ON") ||
		!strings.Contains(last, "session-level via debug_self") ||
		!strings.Contains(last, "Log capture:") {
		t.Fatalf("status must name debug_self yolo source and capture state: %q", last)
	}

	updated, _ = on.handleCmdDebugSelf("/debug-self off", []string{"/debug-self", "off"})
	off := updated.(Model)
	if off.debugSelfEnabled() {
		t.Fatal("/debug-self off must toggle off")
	}
	if off.yoloEnabled() {
		t.Fatal("turning debug-self off must end the yolo it solely granted")
	}
}

func TestDebugSelfCommand_OffKeepsPersistedYoloAndStatusUsage(t *testing.T) {
	m := NewTestModel()
	m.workspace = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Config = config.DefaultUserConfig()
	m.Config.Yolo = true

	updated, _ := m.handleCmdDebugSelf("/debug-self on", []string{"/debug-self", "on"})
	on := updated.(Model)
	updated, _ = on.handleCmdDebugSelf("/debug-self status", []string{"/debug-self", "status"})
	last := updated.(Model).history[len(updated.(Model).history)-1].Content
	if !strings.Contains(last, "persisted yolo:true") {
		t.Fatalf("status must distinguish persisted yolo from debug_self yolo: %q", last)
	}
	updated, _ = on.handleCmdDebugSelf("/debug-self off", []string{"/debug-self", "off"})
	if !updated.(Model).yoloEnabled() {
		t.Fatal("/debug-self off must not end persisted yolo")
	}
	updated, _ = m.handleCmdDebugSelf("/debug-self frobnicate", []string{"/debug-self", "frobnicate"})
	last = updated.(Model).history[len(updated.(Model).history)-1].Content
	if !strings.Contains(last, "Usage") {
		t.Fatalf("bad arg must show usage: %q", last)
	}

	nilCfg := NewTestModel()
	nilCfg.Config = nil
	updated, _ = nilCfg.handleCmdDebugSelf("/debug-self", []string{"/debug-self"})
	last = updated.(Model).history[len(updated.(Model).history)-1].Content
	if !strings.Contains(last, "no config loaded") {
		t.Fatalf("nil config must refuse cleanly, got %q", last)
	}
}
