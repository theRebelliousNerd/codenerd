package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// captureProvider records exactly what one working request handed the model.
type captureProvider struct {
	*MockLLMClient
	system      string
	history     []types.Message
	definitions []types.ToolDefinition
}

func (p *captureProvider) CompleteWithToolResults(_ context.Context, system string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.system, p.history, p.definitions = system, history, definitions
	return &types.LLMToolResponse{Text: "done"}, nil
}

// aProductionSizedCatalog is the shape of a chat turn's tool catalog: 26 tools
// whose JSON runs past 16 KB. Measured 2026-09-11: the 11 core tools alone
// encode to 7849 bytes, so a 26-tool turn is roughly 18 KB.
func aProductionSizedCatalog() []types.ToolDefinition {
	defs := make([]types.ToolDefinition, 0, 26)
	for i := 0; i < 26; i++ {
		defs = append(defs, types.ToolDefinition{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: strings.Repeat(fmt.Sprintf("Tool %d does one thing well and says so at length. ", i), 12),
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Workspace-relative path"}}},
		})
	}
	return defs
}

// oneWorkingRound sets up an executor inside a working loop that has read one
// 14 KB file (the size of the read that broke the live probe) and returns the
// round's transcript, ready to be sent.
func oneWorkingRound(t *testing.T, window int) (*Executor, context.Context, []types.Message, string) {
	t.Helper()
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = window

	var sb strings.Builder
	for i := 1; i <= 437; i++ {
		fmt.Fprintf(&sb, "line %03d of the target file, padding padding\n", i)
	}
	sb.WriteString("needle-line-437\n")
	body := sb.String()
	if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, "target.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix target.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "target.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)

	call := types.ToolCall{ID: "call-1", Name: "read_file", Input: map[string]any{"path": "target.go"}}
	if err := e.recordWorkingResult(ctx, call, body, nil); err != nil {
		t.Fatalf("recordWorkingResult: %v", err)
	}
	history := []types.Message{
		{Role: "user", Text: "fix target.go"},
		{Role: "assistant", ToolCalls: []types.ToolCall{call}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: body}}},
	}
	return e, ctx, history, body
}

// requestText is everything a request carried, in order: each turn's text,
// its tool results, and the harness's text appended to it.
func requestText(messages []types.Message) string {
	var b strings.Builder
	for _, m := range messages {
		for _, block := range m.Content() {
			b.WriteString(block.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func lastToolResult(t *testing.T, history []types.Message) string {
	t.Helper()
	for i := len(history) - 1; i >= 0; i-- {
		if len(history[i].ToolResults) > 0 {
			return history[i].ToolResults[0].Content
		}
	}
	t.Fatal("no tool result in the request")
	return ""
}

// Observed live 2026-09-11 on a one-line edit brief: every round logged
// "Working context: candidates=N selected=0 omitted=N chars=0", the 14 KB read
// of the named file went to the model as "Observation archived: recall_context
// id=...", and the model read, recalled a 2000-character page and re-read the
// same file for 24 rounds until the read-only stall stopped the turn. Two
// causes: the tool catalog (about 18 KB for 26 tools) was subtracted from a
// fixed 16 KB observation budget, leaving zero; and any result over 8000
// characters was archived out of the current pair regardless of the window.
func TestPrepareWorkingRequest_CarriesTheCurrentResultWholeUnderAProductionCatalog(t *testing.T) {
	e, ctx, history, body := oneWorkingRound(t, 200000)
	defs := aProductionSizedCatalog()
	if encoded, _ := json.Marshal(defs); len(encoded) <= 16384 {
		t.Fatalf("catalog is %d bytes; the regression needs one larger than the old 16384-character section budget", len(encoded))
	}

	// Six more rounds. The first read and every later one stay in the ledger
	// whole: together they are under its ceiling, and nothing a request
	// carried is taken out of the next.
	for round := 2; round <= 7; round++ {
		call := types.ToolCall{ID: fmt.Sprintf("call-%d", round), Name: "read_file", Input: map[string]any{"path": "target.go", "start_line": round}}
		later := fmt.Sprintf("round-%d-body", round)
		if err := e.recordWorkingResult(ctx, call, later, nil); err != nil {
			t.Fatalf("recordWorkingResult: %v", err)
		}
		history = append(history,
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{call}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: later}}})
	}

	provider := &captureProvider{MockLLMClient: &MockLLMClient{}}
	if _, err := e.completeWithWorkingContext(ctx, provider, "system", history, defs); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	if got := lastToolResult(t, provider.history); got != "round-7-body" {
		t.Fatalf("the current result was not sent whole; the model saw:\n%s", got)
	}
	carried := false
	for _, m := range provider.history {
		for _, r := range m.ToolResults {
			carried = carried || r.Content == body
		}
	}
	if !carried {
		t.Fatalf("the first read must be carried whole under a %d-tool catalog", len(defs))
	}
	if n := strings.Count(requestText(provider.history), "needle-line-437"); n != 1 {
		t.Fatalf("the first read must be carried once, not also restated or selected; it appears %d times", n)
	}
	// The compiled prompt reaches the provider as it was compiled: a section
	// written into it moved the cacheable prefix on every round.
	if provider.system != "system" {
		t.Fatalf("the system prompt was altered on its way to the provider: %q", truncateForFailure(provider.system))
	}
}

// A result is archived out of the current pair only when the request cannot
// otherwise fit the configured input window, and the pointer then says how
// large the body is so the model pages it instead of asking for it again.
func TestPrepareWorkingRequest_ArchivesOnlyWhatTheWindowCannotCarry(t *testing.T) {
	e, ctx, history, body := oneWorkingRound(t, 3000)

	provider := &captureProvider{MockLLMClient: &MockLLMClient{}}
	if _, err := e.completeWithWorkingContext(ctx, provider, "system", history, nil); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	got := lastToolResult(t, provider.history)
	if got == body {
		t.Fatalf("a %d-character result was sent whole into a 3000-token window", len(body))
	}
	// The pointer names the size and the record, and says it in the pipeline's
	// own marker so an audit of the assembled messages can recognise this cut
	// alongside every other one.
	if !strings.HasPrefix(got, archivedResultPrefix) || !types.IsClamped(got) ||
		!strings.Contains(got, fmt.Sprintf("%d chars", len(body))) || !strings.Contains(got, "recall_context id=") {
		t.Fatalf("archived pointer must name the size and the record; got:\n%s", got)
	}
	if strings.Contains(provider.system, "needle-line-437") || strings.Count(requestText(provider.history), "needle-line-437") != 0 {
		t.Fatal("a result the request points at must not also be sent some other way")
	}
}

// An intent target is often a phrase, not a path. Filing the first round's
// observations under that phrase made them unreachable from the file the tools
// then touched.
func TestNormalizeWorkingEntity_PhraseTargetIsTheWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "real.go"), []byte("package real"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := normalizeWorkingEntity("blackboard shard history line truncateForContext(sr.Task, 50) -> flattenForTask(sr.Task)", root); got != "." {
		t.Fatalf("phrase target normalised to %q, want the workspace root", got)
	}
	if got := normalizeWorkingEntity("real.go", root); got != "real.go" {
		t.Fatalf("file target normalised to %q, want real.go", got)
	}
}

// The request carries every round of the loop whole until the ledger outgrows
// its ceiling, so the model sees all of its own turns. Three runs on
// 2026-09-11 stalled with only the current pair kept: each round the model,
// seeing no earlier turn of its own, re-read the same region to "locate the
// insertion point" and never wrote; a sliding window of three rounds after
// that still dropped a turn's early reads.
func TestPrepareWorkingRequest_CarriesEveryRoundUntilTheCeiling(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000
	if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, "target.go"), []byte("package target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix target.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "target.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)

	history := []types.Message{{Role: "user", Text: "fix target.go"}}
	for round := 1; round <= 7; round++ {
		call := types.ToolCall{ID: fmt.Sprintf("call-%d", round), Name: "read_file", Input: map[string]any{"path": "target.go", "start_line": round}}
		body := fmt.Sprintf("round-%d-body", round)
		if err := e.recordWorkingResult(ctx, call, body, nil); err != nil {
			t.Fatalf("recordWorkingResult: %v", err)
		}
		history = append(history,
			types.Message{Role: "assistant", Text: fmt.Sprintf("round %d", round), ToolCalls: []types.ToolCall{call}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: body}}})
	}

	provider := &captureProvider{MockLLMClient: &MockLLMClient{}}
	if _, err := e.completeWithWorkingContext(ctx, provider, "system", history, nil); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	text := requestText(provider.history)
	for round := 1; round <= 7; round++ {
		if n := strings.Count(text, fmt.Sprintf("round-%d-body", round)); n != 1 {
			t.Fatalf("round %d's result must be carried once, whole; it appears %d times in %q", round, n, text)
		}
		if !strings.Contains(text, fmt.Sprintf("round %d", round)) {
			t.Fatalf("round %d's own turn must be carried", round)
		}
	}
}

// A change whose evidence spans files needs them in view together. The focus
// follows the file touched last, and the observations of the files touched
// before it used to leave the window with the transcript rounds that carried
// them: selection only reached the focus's import neighbourhood, which a
// same-package test never belongs to. Observed 2026-09-19: a flake fix read
// the test four times, one source file four times and the other three times,
// and stopped at the read-only stall with nothing written. The ledger carries
// every file's reads whole until its ceiling, whatever the focus.
func TestPrepareWorkingRequest_KeepsEarlierFilesInViewAfterTheFocusMoves(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000
	for _, name := range []string{"loop_test.go", "loop.go", "context.go"} {
		if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), []byte("package loop // "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix the flaky test", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "loop_test.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)

	// Seven rounds: the test, then the two files it exercises, then four more
	// reads of the last one.
	reads := []struct{ path, body string }{
		{"loop_test.go", "body-of-the-test"},
		{"loop.go", "body-of-loop"},
		{"context.go", "body-of-context-head"},
		{"context.go", "body-of-context-middle"},
		{"context.go", "body-of-context-tail"},
		{"context.go", "body-of-context-again"},
		{"context.go", "body-of-context-once-more"},
	}
	history := []types.Message{{Role: "user", Text: "fix the flaky test"}}
	for i, read := range reads {
		call := types.ToolCall{ID: fmt.Sprintf("call-%d", i+1), Name: "read_file", Input: map[string]any{"path": read.path, "start_line": i + 1}}
		if err := e.recordWorkingResult(ctx, call, read.body, nil); err != nil {
			t.Fatalf("recordWorkingResult: %v", err)
		}
		history = append(history,
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{call}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: read.body}}})
	}

	provider := &captureProvider{MockLLMClient: &MockLLMClient{}}
	if _, err := e.completeWithWorkingContext(ctx, provider, "system", history, nil); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	text := requestText(provider.history)
	for _, body := range []string{"body-of-the-test", "body-of-loop"} {
		if n := strings.Count(text, body); n != 1 {
			t.Fatalf("%s must be carried once, whole; the focus moving to context.go does not end what the turn is working with (it appears %d times)", body, n)
		}
	}
}

// A recall brings an archived observation back; it is not a new observation.
// Saving its result minted a copy under the focus -- whatever file was touched
// last -- at that file's revision, so the recalled evidence went stale when the
// wrong file changed and stayed current when its own file did. Observed
// 2026-09-19: bodies of a test file and executor_tools.go, recalled while the
// focus was working_context.go, were shown as working_context.go observations.
func TestRecordWorkingResult_RecallRestoresTheOriginalObservation(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000
	for _, name := range []string{"loop_test.go", "loop.go"} {
		if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), []byte("package loop // "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix the flaky test", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "loop_test.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	loop := activeWorkingLoop(ctx)

	read := types.ToolCall{ID: "call-read", Name: "read_file", Input: map[string]any{"path": "loop_test.go"}}
	if err := e.recordWorkingResult(ctx, read, "body-of-the-test", nil); err != nil {
		t.Fatal(err)
	}
	original := loop.observations[read.ID]
	other := types.ToolCall{ID: "call-other", Name: "read_file", Input: map[string]any{"path": "loop.go"}}
	if err := e.recordWorkingResult(ctx, other, "body-of-loop", nil); err != nil {
		t.Fatal(err)
	}
	page, err := loop.set.Recall(ctx, original, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	recall := types.ToolCall{ID: "call-recall", Name: "recall_context", Input: map[string]any{"id": original}}
	if err := e.recordWorkingResult(ctx, recall, page, nil); err != nil {
		t.Fatal(err)
	}

	if got := loop.observations[recall.ID]; got != original {
		t.Fatalf("the recall call must map to the observation it recalled, %q; got %q", original, got)
	}
	hits, err := loop.set.Search(ctx, "recall_context/", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hits, `"id":`) {
		t.Fatalf("a recall must not be saved as a new observation; the archive holds %s", hits)
	}
	// The recalled record names its file, and the focus follows it there
	// (N21); what a recall must not do is save a copy under the old focus.
	if loop.focus != "loop_test.go" {
		t.Fatalf("the focus must follow the recall to the recalled observation's file, loop_test.go; focus = %q", loop.focus)
	}
}

// N21 (R1-9, 2026-09-19): under the commit regime a recall is the only way
// left to look at code, and the focus -- whose context every request renders --
// moved only on reads, so it froze on the last file read before reading
// closed: eleven recalls of build_verify.go and repair_loop.go while every
// request rendered working_meter.go, then a read-only stall. The request
// after a recall renders the recalled file.
func TestCompleteWithWorkingContext_RendersTheRecalledFileUnderTheCommitRegime(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000
	for _, name := range []string{"fix.go", "defs.go"} {
		if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), []byte("package p // "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := &stubFileContext{section: "outline"}
	e.SetFileContextProvider(files)
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix the duplicate", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "fix.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	loop := activeWorkingLoop(ctx)

	readFix := types.ToolCall{ID: "read-fix", Name: "read_file", Input: map[string]any{"path": "fix.go"}}
	if err := e.recordWorkingResult(ctx, readFix, "body-of-fix", nil); err != nil {
		t.Fatal(err)
	}
	readDefs := types.ToolCall{ID: "read-defs", Name: "read_file", Input: map[string]any{"path": "defs.go"}}
	if err := e.recordWorkingResult(ctx, readDefs, "body-of-defs", nil); err != nil {
		t.Fatal(err)
	}
	loop.regime = commitRegime
	id := loop.observations[readFix.ID]
	page, err := loop.set.Recall(ctx, id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	recall := types.ToolCall{ID: "recall-fix", Name: "recall_context", Input: map[string]any{"id": id}}
	if err := e.recordWorkingResult(ctx, recall, page, nil); err != nil {
		t.Fatal(err)
	}

	files.calls = nil
	provider := &captureProvider{MockLLMClient: &MockLLMClient{}}
	if _, err := e.completeWithWorkingContext(ctx, provider, "system", nil, nil); err != nil {
		t.Fatalf("completeWithWorkingContext: %v", err)
	}
	if len(files.calls) == 0 || files.calls[len(files.calls)-1] != "fix.go" {
		t.Fatalf("the request after recalling fix.go rendered the context of %v; want fix.go", files.calls)
	}
}
