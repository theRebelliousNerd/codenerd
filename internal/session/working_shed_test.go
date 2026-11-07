package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// perFileView renders a view of the file it is asked for, sized so a snapshot
// costs what a real outline does.
type perFileView struct{ chars int }

func (p perFileView) render(path string) string {
	return "OUTLINE-OF-" + path + "\n" + strings.Repeat("v", p.chars)
}

func (p perFileView) PromptSection(_ context.Context, path string) string { return p.render(path) }

func (p perFileView) PromptSectionWithBudget(_ context.Context, path string, _ int, _ func(ctx context.Context, dimension, target string, total, avgBytes, budgetBytes int) (int, error)) string {
	return p.render(path)
}

const (
	shedReasoningTokens = 2000 // per assistant round, as the provider counted it
	shedResultChars     = 4000
	shedViewChars       = 4000
)

// shedLoop is a working loop shaped like the 2026-11-07 incident: many
// assistant rounds, each replaying provider-encrypted reasoning, tool results
// whole, and a focus-view snapshot appended each time the focus moved, two
// files seen twice.
type shedLoop struct {
	t       *testing.T
	e       *Executor
	ctx     context.Context
	loop    *workingLoop
	history []types.Message
	defs    []types.ToolDefinition
}

func newShedLoop(t *testing.T, foci ...string) *shedLoop {
	t.Helper()
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 1 << 22
	e.config.Working.LedgerCeilingBytes = 1 << 30
	e.config.Working.LedgerKeepRounds = 1000
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), []byte("package target // "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.SetFileContextProvider(perFileView{chars: shedViewChars})
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), nil, "fix a.go", &prompt.CompilationContext{ShardID: "shed", IntentTarget: "a.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	l := &shedLoop{
		t: t, e: e, ctx: ctx, loop: activeWorkingLoop(ctx),
		history: []types.Message{{Role: "user", Text: "fix a.go"}},
		defs:    aProductionSizedCatalog(),
	}
	for _, focus := range foci {
		l.round(focus)
	}
	return l
}

// round runs one assistant round on file: the assistant turn carries a
// redacted reasoning block counted at shedReasoningTokens, prose and a call;
// the user turn carries the result. The request is built against the window
// then in force, as the executor does each round.
func (l *shedLoop) round(file string) []types.Message {
	l.t.Helper()
	n := len(l.history)/2 + 1
	call := types.ToolCall{ID: fmt.Sprintf("call-%d", n), Name: "read_file", Input: map[string]any{"path": file, "start_line": n, "end_line": n}}
	body := fmt.Sprintf("result-%d\n%s", n, strings.Repeat("r", shedResultChars))
	if err := l.e.recordWorkingResult(l.ctx, call, body, nil); err != nil {
		l.t.Fatalf("recordWorkingResult: %v", err)
	}
	l.loop.focus = file
	reasoning := types.RedactedThinkingBlock(fmt.Sprintf("encrypted-%d", n))
	reasoning.ID = fmt.Sprintf("rs_%d", n)
	reasoning.Tokens = shedReasoningTokens
	l.history = append(l.history,
		types.NewAssistantMessage(reasoning, types.TextBlock(fmt.Sprintf("round %d text", n)), types.ToolUseBlock(call.ID, call.Name, call.Input)),
		types.NewUserMessage(types.ToolResultBlock(call.ID, body, false)))
	messages, err := l.e.prepareWorkingRequest(l.ctx, "SYSTEM", l.history, l.defs)
	if err != nil {
		l.t.Fatalf("round %d: prepareWorkingRequest: %v", n, err)
	}
	return messages
}

func (l *shedLoop) tokens(messages []types.Message) int {
	l.t.Helper()
	n, err := prompt.EstimateRequestTokens("SYSTEM", messages, l.defs)
	if err != nil {
		l.t.Fatal(err)
	}
	return n
}

// tighten sets a window that the request as it stands overshoots by what the
// tool results, the superseded snapshots and shedTurns rounds of reasoning
// hold, plus the reserve and a margin under one more round's reasoning.
func (l *shedLoop) tighten(shedTurns int) {
	l.t.Helper()
	messages, _, err := l.loop.assemble(l.ctx, l.history, ledgerStart(l.history, l.loop.task))
	if err != nil {
		l.t.Fatal(err)
	}
	rounds := len(l.history) / 2
	results := rounds * (shedResultChars / 4)
	snapshots := 2 * (shedViewChars / 4)
	l.e.config.TokenBudget = l.tokens(messages) - results - snapshots - shedTurns*shedReasoningTokens + workingInputReserve + shedReasoningTokens/2
}

func hasReasoning(m types.Message) bool {
	for _, b := range m.Content() {
		if b.Kind == types.BlockThinking {
			return true
		}
	}
	return false
}

// assistantTurns are the request's assistant messages, oldest first.
func assistantTurns(messages []types.Message) []types.Message {
	var out []types.Message
	for _, m := range messages {
		if m.Role == "assistant" {
			out = append(out, m)
		}
	}
	return out
}

// The incident: once the tool results were archived the request was still over
// its budget, and the guard refused three times. Everything that remained was
// replayed reasoning (47% of the input) and view snapshots, two of them of the
// same file. The guard now sheds them, oldest reasoning first, and keeps what
// the next request needs.
func TestFitWorkingRequest_ShedsSnapshotsAndReasoningBeforeRefusing(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go", "a.go", "b.go", "b.go", "a.go", "a.go", "b.go", "b.go")
	before := l.round("b.go") // the ninth round, with room to spare
	if got := len(assistantTurns(before)); got != 9 {
		t.Fatalf("fixture has %d assistant turns, want 9", got)
	}
	for i, m := range assistantTurns(before) {
		if !hasReasoning(m) {
			t.Fatalf("assistant turn %d lost its reasoning with room to spare", i+1)
		}
	}
	if strings.Contains(requestText(before), "superseded") {
		t.Fatal("a request that fits must carry every view snapshot as it was appended")
	}
	l.tighten(3)

	messages, err := l.e.prepareWorkingRequest(l.ctx, "SYSTEM", l.history, l.defs)
	if err != nil {
		t.Fatalf("a request that sheds its way to fitting must not be refused: %v", err)
	}
	if used := l.tokens(messages); l.e.config.TokenBudget-used < workingInputReserve {
		t.Fatalf("the request needs %d tokens in a %d budget with %d held back", used, l.e.config.TokenBudget, workingInputReserve)
	}

	// 1. Every tool result went first.
	for _, m := range messages {
		for _, r := range m.ToolResults {
			if !strings.HasPrefix(r.Content, archivedResultPrefix) {
				t.Fatalf("a tool result was left whole while reasoning was shed: %.60s", r.Content)
			}
		}
	}
	text := requestText(messages)
	// 2. The older snapshot of each file that has a later one is a pointer;
	// the latest per file survives whole.
	for _, file := range []string{"a.go", "b.go"} {
		if n := strings.Count(text, fmt.Sprintf(workingViewHeader, file)); n != 1 {
			t.Errorf("%s: %d whole view snapshots in the request, want exactly the latest", file, n)
		}
		if n := strings.Count(text, fmt.Sprintf("the view snapshot of %s that stood here is superseded by a later snapshot of %s", file, file)); n < 1 {
			t.Errorf("%s: no superseded-snapshot pointer", file)
		}
	}
	if !strings.Contains(text, "chars of this view snapshot") || !types.IsClamped(text) {
		t.Error("a collapsed snapshot must carry the pipeline's drop marker")
	}
	// 3. Reasoning went oldest first and never touched the latest turn's.
	turns := assistantTurns(messages)
	dropped := 0
	for dropped < len(turns) && !hasReasoning(turns[dropped]) {
		dropped++
	}
	if dropped < 1 || dropped >= len(turns) {
		t.Fatalf("reasoning dropped from %d of %d turns, want some but not all", dropped, len(turns))
	}
	for i := dropped; i < len(turns); i++ {
		if !hasReasoning(turns[i]) {
			t.Fatalf("turn %d lost its reasoning but an older turn %d kept its own: not oldest first", i+1, dropped)
		}
	}
	latest := turns[len(turns)-1].Content()
	if latest[0].Kind != types.BlockThinking || latest[0].ID != "rs_9" || latest[0].Signature != "encrypted-9" {
		t.Fatalf("the latest round's reasoning was touched: %+v", latest[0])
	}
	// Every turn keeps its prose and its call.
	for i, m := range turns {
		if m.Text != fmt.Sprintf("round %d text", i+1) || len(m.ToolCalls) != 1 || m.ToolCalls[0].ID != fmt.Sprintf("call-%d", i+1) {
			t.Fatalf("turn %d lost its text or call: text=%q calls=%+v", i+1, m.Text, m.ToolCalls)
		}
	}
	// The marker says what is gone and that it cannot come back.
	if !strings.Contains(text, "cannot be recalled") || !strings.Contains(text, fmt.Sprintf("the replayed reasoning of the %d earlier assistant turn(s)", dropped)) {
		t.Errorf("no marker for the %d dropped reasoning turns", dropped)
	}

	// The ledger agrees: what was shed stays shed. Asked again, the request
	// keeps the collapsed snapshots and the dropped reasoning (the results are
	// archived only on the rounds that need it, as ever, so they may now fit
	// whole), and the next round does not resurrect either.
	again, err := l.e.prepareWorkingRequest(l.ctx, "SYSTEM", l.history, l.defs)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"a.go", "b.go"} {
		if n := strings.Count(requestText(again), fmt.Sprintf(workingViewHeader, file)); n != 1 {
			t.Errorf("asked again, %d whole snapshots of %s are in the request, want the latest alone", n, file)
		}
	}
	for i, m := range assistantTurns(again)[:dropped] {
		if hasReasoning(m) {
			t.Fatalf("asked again, turn %d's dropped reasoning came back", i+1)
		}
	}
	l.e.config.TokenBudget += 4 * shedReasoningTokens // room for one more round after shedding
	next := l.round("a.go")
	nextText := requestText(next)
	// The round has room, so the snapshot it supersedes is not collapsed too
	// (only pressure collapses); the one collapsed before stays collapsed.
	if n := strings.Count(nextText, fmt.Sprintf(workingViewHeader, "a.go")); n != 2 {
		t.Errorf("after another round %d whole snapshots of a.go are in the request, want the previous latest and the new one", n)
	}
	if n := strings.Count(nextText, "the view snapshot of a.go that stood here is superseded"); n != 1 {
		t.Errorf("the snapshot collapsed earlier shows %d pointers after another round, want 1", n)
	}
	nextTurns := assistantTurns(next)
	for i := 0; i < dropped; i++ {
		if hasReasoning(nextTurns[i]) {
			t.Fatalf("turn %d's dropped reasoning came back on the next round", i+1)
		}
	}
	if !hasReasoning(nextTurns[len(nextTurns)-1]) {
		t.Fatal("the new latest round lost its reasoning")
	}
}

// The ruling on when a superseded snapshot collapses: only under budget
// pressure. Rewriting a turn mid-epoch breaks the provider's cached prefix at
// that turn (measured 2026-10-29), so a request that fits sends every snapshot
// as it was appended, and one that does not breaks the prefix once, at the shed.
func TestFitWorkingRequest_CollapsesSnapshotsOnlyUnderPressure(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go", "b.go", "a.go")
	messages := l.round("a.go")
	if n := strings.Count(requestText(messages), fmt.Sprintf(workingViewHeader, "a.go")); n != 2 {
		t.Fatalf("with room to spare the request carries %d snapshots of a.go, want both", n)
	}
	if len(l.loop.reasoningShed) != 0 {
		t.Fatal("nothing was over budget, yet reasoning was shed")
	}
	for _, s := range l.loop.snapshots {
		if s.collapsed {
			t.Fatalf("snapshot of %s was collapsed with room to spare", s.path)
		}
	}
}

// A refusal says what the request is made of once everything the guard may shed
// is shed, so what to cut (or which window to raise) can be read off it.
func TestFitWorkingRequest_RefusalNamesTheRemainingParts(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go", "a.go", "b.go", "b.go", "a.go")
	l.e.config.TokenBudget = 6000
	_, err := l.e.prepareWorkingRequest(l.ctx, "SYSTEM", l.history, l.defs)
	if err == nil || !errors.Is(err, ErrInputBudgetExceeded) {
		t.Fatalf("a window the latest round alone overflows must be refused with ErrInputBudgetExceeded, got %v", err)
	}
	msg := err.Error()
	snapshotTokens := 0
	for _, s := range l.loop.snapshots {
		if !s.collapsed {
			snapshotTokens += prompt.EstimateTokens(s.text)
		}
	}
	for _, want := range []string{
		"system prompt ", "task and anchor ", "tool catalog ", "prior turns ",
		fmt.Sprintf("reasoning kept %d", shedReasoningTokens),
		fmt.Sprintf("view snapshots kept %d", snapshotTokens),
		"tool calls and assistant text ", "tool results and harness notes ",
		"every superseded view snapshot collapsed", "all replayed reasoning but the latest assistant turn's dropped",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, msg)
		}
	}
	if snapshotTokens == 0 {
		t.Fatal("fixture error: no snapshot was kept")
	}
	// The latest round's reasoning survived the refusal's shedding too.
	if len(l.loop.reasoningShed) != 4 {
		t.Errorf("the guard shed %d turns of reasoning, want every turn but the latest (4)", len(l.loop.reasoningShed))
	}
	if _, kept := l.loop.reasoningShed[ledgerKey(len(l.history)-2)]; kept {
		t.Error("the latest assistant turn's reasoning was shed")
	}
}

// The Responses path end to end: the guard's request goes through the real
// Responses encoder of the production client, and the input array it sends is
// valid -- the latest round's reasoning item is in front of its call, the older
// reasoning items are gone, and every call has its output.
func TestFitWorkingRequest_ResponsesInputIsValidAfterShedding(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go", "a.go", "b.go", "b.go", "a.go", "a.go", "b.go", "b.go")
	l.round("b.go")
	l.tighten(3)

	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp","status":"completed","model":"test-responses-contributor","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`))
	}))
	defer srv.Close()
	cfg := perception.DefaultOpenAICompatConfig(perception.ProviderMeta, "test-key")
	cfg.Model = "test-responses-contributor"
	cfg.BaseURL = srv.URL
	cfg.Timeout = 5 * time.Second
	client, err := perception.NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.e.completeWithWorkingContext(l.ctx, client, "SYSTEM", l.history, l.defs); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	input, _ := body["input"].([]any)
	if len(input) == 0 {
		t.Fatalf("the server saw no input array: %v", body)
	}
	var kinds []string
	calls, outputs := map[string]int{}, map[string]int{}
	reasoning := 0
	for i, raw := range input {
		item, _ := raw.(map[string]any)
		kind, _ := item["type"].(string)
		if kind == "" {
			kind = "message:" + fmt.Sprint(item["role"])
		}
		kinds = append(kinds, kind)
		switch kind {
		case "reasoning":
			reasoning++
			if i == len(input)-1 {
				t.Fatalf("a reasoning item ends the input: %v", kinds)
			}
			next, _ := input[i+1].(map[string]any)
			if next["type"] == nil && next["role"] != "assistant" {
				t.Fatalf("reasoning item %v is not followed by the assistant item it produced: %v", item["id"], kinds)
			}
			if next["type"] == "function_call_output" {
				t.Fatalf("reasoning item %v is followed by a result, not by its call: %v", item["id"], kinds)
			}
		case "function_call":
			calls[fmt.Sprint(item["call_id"])]++
		case "function_call_output":
			outputs[fmt.Sprint(item["call_id"])]++
		}
	}
	if len(calls) != 9 {
		t.Fatalf("%d distinct function calls on the wire, want all 9", len(calls))
	}
	for id, n := range calls {
		if n != 1 || outputs[id] != 1 {
			t.Errorf("call %s: %d calls and %d outputs on the wire, want one of each", id, n, outputs[id])
		}
	}
	if reasoning < 1 || reasoning >= 9 {
		t.Fatalf("%d reasoning items on the wire, want some but fewer than the 9 replayed unshed", reasoning)
	}
	last := -1
	for i, raw := range input {
		if item, _ := raw.(map[string]any); item["type"] == "reasoning" {
			last = i
		}
	}
	latest, _ := input[last].(map[string]any)
	if latest["id"] != "rs_9" || latest["encrypted_content"] != "encrypted-9" {
		t.Fatalf("the latest round's reasoning item is not on the wire: %v", latest)
	}
	// The latest turn is reasoning, prose, call: its reasoning item precedes the
	// prose item it produced, and its call follows.
	if text, _ := input[last+1].(map[string]any); text["role"] != "assistant" {
		t.Fatalf("the latest reasoning item must precede its own assistant item: %v", text)
	}
	if call, _ := input[last+2].(map[string]any); call["type"] != "function_call" || call["call_id"] != "call-9" {
		t.Fatalf("the latest turn's call must follow its reasoning: %v", call)
	}
}
