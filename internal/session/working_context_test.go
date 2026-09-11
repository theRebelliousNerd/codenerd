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

	// Three more rounds push the first read out of the transcript window, so
	// it has to come back through the selected section, which is where the
	// zeroed budget used to lose it.
	for round := 2; round <= 4; round++ {
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
	if got := lastToolResult(t, provider.history); got != "round-4-body" {
		t.Fatalf("the current result was not sent whole; the model saw:\n%s", got)
	}
	for _, m := range provider.history {
		for _, r := range m.ToolResults {
			if r.Content == body {
				t.Fatal("the first read is outside the transcript window and must not be in the transcript")
			}
		}
	}
	if !strings.Contains(provider.system, "[observation id=") || !strings.Contains(provider.system, "needle-line-437") {
		t.Fatalf("the first read was not selected into the working section under a %d-tool catalog; system prompt tail:\n%s", len(defs), provider.system[max(0, len(provider.system)-600):])
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
	if !strings.HasPrefix(got, archivedResultPrefix) || !strings.Contains(got, fmt.Sprintf("%d-character", len(body))) || !strings.Contains(got, "recall_context id=") {
		t.Fatalf("archived pointer must name the size and the record; got:\n%s", got)
	}
	if strings.Contains(provider.system, "needle-line-437") {
		t.Fatal("a result the transcript points at must not also be sent in the section")
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

// The request keeps the policy's span of native rounds so the model sees its
// own recent turns; older rounds move to the section, and no observation is
// sent both ways. Three runs on 2026-09-11 stalled with only the current
// pair kept: each round the model, seeing no earlier turn of its own,
// re-read the same region to "locate the insertion point" and never wrote.
func TestPrepareWorkingRequest_KeepsRecentRoundsInTheTranscript(t *testing.T) {
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
	for round := 1; round <= 5; round++ {
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
	var transcript strings.Builder
	for _, m := range provider.history {
		transcript.WriteString(m.Text)
		for _, r := range m.ToolResults {
			transcript.WriteString(r.Content)
		}
	}
	for round := 3; round <= 5; round++ {
		if !strings.Contains(transcript.String(), fmt.Sprintf("round-%d-body", round)) {
			t.Fatalf("round %d must stay in the transcript (policy keeps 3 rounds); transcript: %q", round, transcript.String())
		}
		if strings.Contains(provider.system, fmt.Sprintf("round-%d-body", round)) {
			t.Fatalf("round %d is in the transcript and must not also be in the section", round)
		}
	}
	for round := 1; round <= 2; round++ {
		if strings.Contains(transcript.String(), fmt.Sprintf("round-%d-body", round)) {
			t.Fatalf("round %d is outside the kept span and must leave the transcript", round)
		}
		if !strings.Contains(provider.system, fmt.Sprintf("round-%d-body", round)) {
			t.Fatalf("round %d left the transcript and must be in the section; section tail: %q", round, provider.system[max(0, len(provider.system)-400):])
		}
	}
}
