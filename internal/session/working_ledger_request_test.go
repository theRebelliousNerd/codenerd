package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// ledgerLoop is a working loop over target.go, with a stub focus view, whose
// ledger compacts past ceiling bytes and keeps keep rounds whole.
type ledgerLoop struct {
	t       *testing.T
	e       *Executor
	ctx     context.Context
	files   *stubFileContext
	history []types.Message
	sent    [][]types.Message
}

func newLedgerLoop(t *testing.T, ceiling, keep int) *ledgerLoop {
	t.Helper()
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000
	e.config.Working.LedgerCeilingBytes = ceiling
	e.config.Working.LedgerKeepRounds = keep
	if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, "target.go"), []byte("package target // v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := &stubFileContext{section: "VIEW-OF-TARGET"}
	e.SetFileContextProvider(files)
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix target.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "target.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	return &ledgerLoop{t: t, e: e, ctx: ctx, files: files, history: []types.Message{{Role: "user", Text: "fix target.go"}}}
}

// round records one tool call on target.go with body as its result, appends
// the round to the history and sends the next request.
func (l *ledgerLoop) round(name, body string) []types.Message {
	l.t.Helper()
	return l.roundCall(name, map[string]any{"path": "target.go", "start_line": len(l.sent) + 1}, body)
}

// roundCall is round with the call's input given.
func (l *ledgerLoop) roundCall(name string, input map[string]any, body string) []types.Message {
	l.t.Helper()
	call := types.ToolCall{ID: fmt.Sprintf("call-%d", len(l.sent)+1), Name: name, Input: input}
	if err := l.e.recordWorkingResult(l.ctx, call, body, nil); err != nil {
		l.t.Fatalf("recordWorkingResult: %v", err)
	}
	l.history = append(l.history,
		types.Message{Role: "assistant", ToolCalls: []types.ToolCall{call}},
		types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: body}}})
	messages, err := l.e.prepareWorkingRequest(l.ctx, "SYSTEM", l.history, nil)
	if err != nil {
		l.t.Fatalf("prepareWorkingRequest: %v", err)
	}
	l.sent = append(l.sent, messages)
	return messages
}

// requireAppendOnly fails unless every turn of request before is the same turn,
// byte for byte, in request after.
func requireAppendOnly(t *testing.T, before, after []types.Message, what string) {
	t.Helper()
	if len(before) > len(after) {
		t.Fatalf("%s: the later request is shorter", what)
	}
	for i := range before {
		if a, b := fmt.Sprintf("%+v", before[i]), fmt.Sprintf("%+v", after[i]); a != b {
			t.Fatalf("%s: turn %d changed\nbefore: %s\nafter:  %s", what, i, truncateForFailure(a), truncateForFailure(b))
		}
	}
}

// Between compactions the ledger only grows at its end, and the focus view is
// appended once -- not rendered and resent at the tail of every request, which
// is what kept the working section out of every provider cache.
func TestWorkingLedger_RequestsAreAppendOnlyAndTheViewIsSentOnce(t *testing.T) {
	l := newLedgerLoop(t, 1<<20, 2)
	for n := 1; n <= 5; n++ {
		l.round("read_file", fmt.Sprintf("round-%d-body", n))
		if n > 1 {
			requireAppendOnly(t, l.sent[n-2], l.sent[n-1], fmt.Sprintf("request %d after request %d", n, n-1))
		}
	}
	if n := strings.Count(requestText(l.sent[4]), "VIEW-OF-TARGET"); n != 1 {
		t.Fatalf("the focus view appears %d times in the fifth request; it is appended once per focus and revision", n)
	}
	if len(l.files.calls) != 1 {
		t.Fatalf("the focus view was rendered %d times for one focus at one revision", len(l.files.calls))
	}
}

// Past the ceiling one compaction moves every result older than the kept
// rounds out behind its recall handle; the handle is sent unchanged from then
// on, and it recovers the result whole. Eviction is not deletion.
func TestWorkingLedger_ACompactionMovesOldRoundsBehindTheirHandles(t *testing.T) {
	l := newLedgerLoop(t, 4096, 2)
	first := strings.Repeat("first-round ", 125) // 1500 bytes
	l.round("read_file", first)
	l.round("outline", strings.Repeat("second-round ", 116))
	third := l.round("read_symbol", strings.Repeat("third-round ", 125))

	handle := lastResultOf(t, third, "call-1")
	if !strings.HasPrefix(handle, archivedResultPrefix) || !types.IsClamped(handle) || !strings.Contains(handle, "recall_context id=") ||
		!strings.Contains(handle, fmt.Sprintf("%d chars", len(first))) {
		t.Fatalf("past the ceiling the first round's result must be its recall handle, naming its size; got %q", handle)
	}
	for _, call := range []string{"call-2", "call-3"} {
		if got := lastResultOf(t, third, call); strings.HasPrefix(got, archivedResultPrefix) {
			t.Fatalf("%s is within the kept rounds and must stay whole; got %q", call, got)
		}
	}
	if !strings.Contains(requestText(third), "VIEW-OF-TARGET") {
		t.Fatal("a compaction drops the harness text of earlier rounds and must append the focus view again")
	}

	fourth := l.round("read_file", "small")
	if got := lastResultOf(t, fourth, "call-1"); got != handle {
		t.Fatalf("the handle must be sent unchanged after the compaction; got %q, was %q", got, handle)
	}
	loop := activeWorkingLoop(l.ctx)
	page, err := loop.set.Recall(l.ctx, loop.observations["call-1"], 0, 0)
	if err != nil || !strings.Contains(page, first) {
		t.Fatalf("the handle must recover the result whole; recall = %q, %v", truncateForFailure(page), err)
	}
}

// An edit makes a carried read stale. It is not rewritten -- that would change
// every request from its round on -- and not left standing as current: the
// round that sees the change carries a notice naming it, once, and the focus
// view is appended again at the new revision.
func TestWorkingLedger_AnEditRestatesTheStaleReadOnce(t *testing.T) {
	l := newLedgerLoop(t, 1<<20, 2)
	l.round("read_file", "v1-body")
	id := activeWorkingLoop(l.ctx).observations["call-1"]

	if err := os.WriteFile(filepath.Join(l.e.config.WorkspaceRoot, "target.go"), []byte("package target // v2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := l.round("edit_lines", "edited")
	if n := strings.Count(requestText(second), "observation "+id+" (read_file of target.go) predates"); n != 1 {
		t.Fatalf("the round that sees the edit must restate the stale read once; the notice appears %d times in:\n%s", n, requestText(second))
	}
	if !strings.Contains(requestText(second), "v1-body") {
		t.Fatal("the stale read stays in the ledger: rewriting it would change every request from its round on")
	}

	third := l.round("read_file", "v2-body")
	requireAppendOnly(t, second, third, "the request after the restatement")
	text := requestText(third)
	if n := strings.Count(text, "predates the current content"); n != 1 {
		t.Fatalf("the notice must not be repeated at the same revision; it appears %d times", n)
	}
	if n := strings.Count(text, "VIEW-OF-TARGET"); n != 2 {
		t.Fatalf("the focus view is appended again at the new revision and only then; it appears %d times", n)
	}
}

// R8-3: what a call read of one element is dated by that element's bytes.
// Editing function A restates the read of A and leaves the read of B -- in the
// same file -- standing, where a whole-file revision staled both and the model
// re-read what had not changed (one file 34 times in one campaign).
func TestWorkingLedger_AnEditToOneFunctionRestatesOnlyWhatWasReadOfIt(t *testing.T) {
	l := newLedgerLoop(t, 1<<20, 2)
	path := filepath.Join(l.e.config.WorkspaceRoot, "target.go")
	write := func(aBody string) {
		t.Helper()
		src := "package target\n\n// A does a.\nfunc A() int { " + aBody + " }\n\n// B does b.\nfunc B() int { return 2 }\n"
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("return 1")
	l.roundCall("get_element", map[string]any{"path": "target.go", "ref": "A"}, "func A() int { return 1 }")
	l.roundCall("get_element", map[string]any{"path": "target.go", "ref": "B"}, "func B() int { return 2 }")

	loop := activeWorkingLoop(l.ctx)
	for call, want := range map[string]string{"call-1": "target.go::A", "call-2": "target.go::B"} {
		if got, err := loop.set.Entity(l.ctx, loop.observations[call]); err != nil || got != want {
			t.Fatalf("%s is filed under %q (%v), want the element %q", call, got, err, want)
		}
	}
	if loop.focus != "target.go" {
		t.Fatalf("the focus is a file, not an element: %q", loop.focus)
	}

	write("return 3")
	third := l.roundCall("edit_element", map[string]any{"path": "target.go", "ref": "A"}, "func A() int { return 3 }")
	text := requestText(third)
	if n := strings.Count(text, "predates the current content of target.go::A"); n != 1 {
		t.Fatalf("the read of A must be restated once after A changed; the notice appears %d times in:\n%s", n, text)
	}
	if strings.Contains(text, "predates the current content of target.go::B") {
		t.Fatal("B did not change: what was read of it stands")
	}
	if got, err := loop.set.Entity(l.ctx, loop.observations["call-3"]); err != nil || got != "target.go::A" {
		t.Fatalf("the edit's observation is the element as the edit left it; filed under %q (%v)", got, err)
	}

	// A recall of an element's observation moves the focus to its file: the
	// focus names whose view is rendered, and an element is not a file.
	loop.focus = "."
	recall := types.ToolCall{ID: "recall-b", Name: "recall_context", Input: map[string]any{"id": loop.observations["call-2"]}}
	if err := l.e.recordWorkingResult(l.ctx, recall, "page", nil); err != nil {
		t.Fatal(err)
	}
	if loop.focus != "target.go" {
		t.Fatalf("after recalling B's observation the focus is %q, want its file target.go", loop.focus)
	}
}

// A client with no message channel gets one call: the focus view rides the
// user input, never the system prompt, and a request that does not fit the
// window is refused whole -- it has no tool result to archive. The ledger
// commit dropped that refusal (e2e LargePayload_RefusedWhole and two
// orchestrator payload tests went green on a request they must refuse).
func TestSingleShotRequest_CarriesTheViewAndRefusesWhatCannotFit(t *testing.T) {
	l := newLedgerLoop(t, 1<<20, 2)
	input, err := l.e.singleShotRequest(l.ctx, "SYSTEM", "fix target.go", nil)
	if err != nil {
		t.Fatalf("singleShotRequest: %v", err)
	}
	if !strings.HasPrefix(input, "fix target.go") || !strings.Contains(input, fmt.Sprintf(workingViewHeader, "target.go")) || !strings.Contains(input, "VIEW-OF-TARGET") {
		t.Fatalf("the focus view must ride the user input; got %q", input)
	}
	if got := l.e.workingFocusView(l.ctx, nil); got != "" {
		t.Fatalf("no loop, no view; got %q", got)
	}

	l.e.config.TokenBudget = 3000
	if _, err := l.e.singleShotRequest(l.ctx, "SYSTEM", strings.Repeat("a massive task ", 5000), nil); !errors.Is(err, ErrInputBudgetExceeded) {
		t.Fatalf("a task larger than the window: err = %v, want ErrInputBudgetExceeded", err)
	}
}

// lastResultOf is the content a request carried for the named call.
func lastResultOf(t *testing.T, messages []types.Message, call string) string {
	t.Helper()
	for i := len(messages) - 1; i >= 0; i-- {
		for _, r := range messages[i].ToolResults {
			if r.ToolUseID == call {
				return r.Content
			}
		}
	}
	t.Fatalf("the request carries no result for %s", call)
	return ""
}
