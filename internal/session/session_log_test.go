package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

func installTestSessionLog(t *testing.T, debug bool) (workspace string) {
	t.Helper()
	workspace = t.TempDir()
	var on atomic.Bool
	on.Store(debug)
	sessionlog.Install(sessionlog.NewStore(sessionlog.Options{Workspace: workspace, Session: "session-1", Enabled: on.Load}))
	t.Cleanup(func() { sessionlog.Install(nil) })
	return workspace
}

type transcriptFile struct {
	Agent  string `json:"agent"`
	Events []struct {
		Seq     int      `json:"seq"`
		Kind    string   `json:"kind"`
		Tool    string   `json:"tool"`
		CallID  string   `json:"call_id"`
		IsError bool     `json:"is_error"`
		Text    []string `json:"text"`
		Error   string   `json:"error"`
	} `json:"events"`
}

func readTranscript(t *testing.T, workspace, folder string) transcriptFile {
	t.Helper()
	path := filepath.Join(workspace, ".nerd", "sessions", "session-1", folder, sessionlog.TranscriptFile)
	data, err := os.ReadFile(path)
	if err != nil {
		entries, _ := os.ReadDir(filepath.Dir(path))
		t.Fatalf("no transcript at %s: %v (folder holds %v)", path, err, entries)
	}
	var doc transcriptFile
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("transcript is not valid JSON: %v\n%s", err, data)
	}
	return doc
}

// A subagent is given nothing about transcripts -- no prompt text, no tool --
// and its transcript exists anyway: the harness writes it from the run loop,
// the tool batch and the outcome, with debug mode OFF.
func TestSubAgentProducesATranscriptWithoutAnyTranscriptPromptOrTool(t *testing.T) {
	workspace := installTestSessionLog(t, false)
	const toolName = "transcript_probe"
	if !tools.Global().Has(toolName) {
		registerTestTool(t, &tools.Tool{
			Effect: tools.EffectRead, Name: toolName, Description: "probe", Category: tools.CategoryGeneral,
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				return "probe says\nhello", nil
			},
		})
	}
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if strings.Contains(strings.ToLower(sys+user), "transcript") {
				t.Errorf("the model must be told nothing about transcripts")
			}
			return &types.LLMToolResponse{
				Text:      "reading the probe",
				ToolCalls: []types.ToolCall{{ID: "call_1", Name: toolName, Input: map[string]any{}}},
			}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) { return "done", nil },
	}
	cfg := DefaultSubAgentConfig("scribe")
	cfg.ID = "scribe-42"
	agent := NewSubAgent(cfg, &MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{},
		&MockConfigFactory{GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}, nil
		}}, &MockTransducer{})
	agent.executor.config.EnableSafetyGate = false
	agent.executor.config.VerifyBuildAfterEdits = false
	agent.executor.config.VerifyTestsAfterEdits = false
	agent.executor.config.CriticReviewAfterEdits = false
	agent.executor.config.WorkspaceRoot = t.TempDir()
	agent.executor.EffectiveAgentRuntimeConfig = &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}

	ctx := usage.WithSessionID(context.Background(), "session-1")
	agent.Run(ctx, "inspect the probe\nand report")
	_, _ = agent.Wait()

	doc := readTranscript(t, workspace, "scribe-42")
	if doc.Agent != "scribe-42" {
		t.Errorf("agent = %q", doc.Agent)
	}
	var kinds []string
	var toolResult int
	for _, e := range doc.Events {
		kinds = append(kinds, e.Kind)
		if e.Kind == sessionlog.EventToolResult && e.Tool == toolName && e.CallID == "call_1" && strings.Join(e.Text, "\n") == "probe says\nhello" {
			toolResult++
		}
	}
	if len(kinds) < 3 || kinds[0] != sessionlog.EventInput || kinds[len(kinds)-1] != sessionlog.EventFinal {
		t.Fatalf("a transcript opens with the task and closes with the outcome, got %v", kinds)
	}
	if doc.Events[0].Text[0] != "inspect the probe" || doc.Events[0].Text[1] != "and report" {
		t.Errorf("the task must be stored as its lines: %v", doc.Events[0].Text)
	}
	if toolResult != 1 {
		t.Errorf("the tool call's result must be in the transcript exactly once, kinds %v", kinds)
	}
	// And there are no exchange files: debug is off.
	entries, _ := os.ReadDir(filepath.Join(workspace, ".nerd", "sessions", "session-1", "scribe-42"))
	if len(entries) != 1 {
		t.Errorf("debug off: only the transcript may exist, got %v", entries)
	}
}

func TestSubAgentTranscriptSurvivesACancelledRun(t *testing.T) {
	workspace := installTestSessionLog(t, false)
	started := make(chan struct{})
	var once sync.Once
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, ctx.Err()
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	cfg := DefaultSubAgentConfig("doomed")
	cfg.ID = "doomed-1"
	agent := NewSubAgent(cfg, &MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	agent.executor.config.WorkspaceRoot = t.TempDir()
	agent.Run(usage.WithSessionID(context.Background(), "session-1"), "never finishes")
	<-started
	_ = agent.Stop()
	_, _ = agent.Wait()

	doc := readTranscript(t, workspace, "doomed-1")
	if len(doc.Events) != 2 || doc.Events[0].Kind != sessionlog.EventInput || doc.Events[1].Kind != sessionlog.EventFinal || doc.Events[1].Error == "" {
		t.Fatalf("a cancelled agent still leaves its input and how it ended: %+v", doc.Events)
	}
}

func TestExecuteToolBatchRecordsEachResultInTheAgentsTranscript(t *testing.T) {
	workspace := installTestSessionLog(t, false)
	ctx := sessionlog.WithAgent(usage.WithSessionID(context.Background(), "session-1"), "coder", "coder-1")
	e := &Executor{}
	calls := []types.ToolCall{
		{ID: "a", Name: "bad_args", ArgsError: "unterminated string"},
		{ID: "b", Name: "also_bad", ArgsError: "bad json"},
	}
	results, _ := e.executeToolBatch(ctx, calls, &config.EffectiveAgentRuntimeConfig{}, &ExecutionResult{})
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	doc := readTranscript(t, workspace, "coder-1")
	if len(doc.Events) != 2 || doc.Events[0].Tool != "bad_args" || !doc.Events[0].IsError || doc.Events[1].CallID != "b" {
		t.Fatalf("tool results in transcript: %+v", doc.Events)
	}
	// A context cancelled before the batch records the skipped calls too.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, _ = e.executeToolBatch(cctx, []types.ToolCall{{ID: "c", Name: "never_ran"}}, &config.EffectiveAgentRuntimeConfig{}, &ExecutionResult{})
	doc = readTranscript(t, workspace, "coder-1")
	if len(doc.Events) != 3 || !strings.Contains(strings.Join(doc.Events[2].Text, "\n"), "not executed") {
		t.Fatalf("skipped call not recorded: %+v", doc.Events)
	}
}

func TestSessionHistoryLivesInTheSessionFolder(t *testing.T) {
	ws := t.TempDir()
	if err := SaveSessionHistory(ws, "session-9", []ChatMessage{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "sessions", "session-9", "session.json")); err != nil {
		t.Fatalf("history must be .nerd/sessions/<id>/session.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "sessions", "session-9.json")); !os.IsNotExist(err) {
		t.Fatal("the flat path is gone: a single path, no dual writing")
	}
	h, err := LoadSessionHistory(ws, "session-9")
	if err != nil || len(h.Messages) != 1 {
		t.Fatalf("load: %+v %v", h, err)
	}
}

func TestExecuteToolBatch_StreamsStartAndOutcomeWhileToolRuns(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed=%v", failed), func(t *testing.T) {
			events := make(chan sessionlog.LiveEvent, 16)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = sessionlog.WithAgent(sessionlog.WithLiveProgress(ctx, func(_ context.Context, ev sessionlog.LiveEvent) { events <- ev }), "coder", "probe")
			entered := make(chan struct{})
			release := make(chan struct{})
			const name = "live_progress_read_probe"
			registerTestTool(t, &tools.Tool{Name: name, Effect: tools.EffectRead, Category: tools.CategoryGeneral,
				Execute: func(ctx context.Context, args map[string]any) (string, error) {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return "", ctx.Err()
					}
					if failed {
						return "diagnostic", errors.New("probe failed")
					}
					return "private file content", nil
				}})
			e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
			e.config.EnableSafetyGate = false
			cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{name}}
			done := make(chan struct{})
			go func() {
				defer close(done)
				e.executeToolBatch(ctx, []types.ToolCall{{ID: "read-1", Name: name, Input: map[string]any{"path": "internal/loader.go"}}}, cfg, &ExecutionResult{})
			}()
			defer func() { cancel(); <-done }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("tool did not enter")
			}
			select {
			case ev := <-events:
				if ev.Kind != "tool_started" || ev.CallID != "read-1" || ev.Text != "internal/loader.go" {
					t.Fatalf("start = %+v", ev)
				}
			case <-time.After(time.Second):
				t.Fatal("tool start hidden until finish")
			}
			close(release)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("tool did not finish")
			}
			select {
			case ev := <-events:
				if ev.Kind != sessionlog.EventToolResult || ev.Failed != failed || ev.Text != "internal/loader.go" {
					t.Fatalf("outcome = %+v", ev)
				}
			default:
				t.Fatal("tool outcome missing")
			}
		})
	}
}

func TestLiveProgress_OnlyProcessReceiptsCanSayTestsPassed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		exits []int
		want  string
	}{
		{"no tests executed", nil, "DONE"},
		{"tests passed", []int{0}, "PASS"},
		{"earlier failure is not hidden", []int{1, 0}, "FAIL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []sessionlog.LiveEvent
			ctx := sessionlog.WithAgent(sessionlog.WithLiveProgress(context.Background(), func(_ context.Context, ev sessionlog.LiveEvent) { events = append(events, ev) }), "coder", "receipt")
			const name = "live_receipt_probe"
			registerTestTool(t, &tools.Tool{Name: name, Effect: tools.EffectRead, Category: tools.CategoryGeneral, Execute: func(ctx context.Context, _ map[string]any) (string, error) {
				for _, code := range tc.exits {
					tools.RecordTestRun(ctx, tools.TestRun{Argv: []string{"go", "test", "./pkg"}, ExitCode: code})
				}
				return "tool completed", nil
			}})
			e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
			e.config.EnableSafetyGate = false
			e.executeToolBatch(ctx, []types.ToolCall{{ID: "check", Name: name}}, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{name}}, &ExecutionResult{})
			if len(events) != 2 || events[1].Status != tc.want {
				t.Fatalf("outcome = %+v, want %s", events, tc.want)
			}
		})
	}
}
