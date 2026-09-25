package session

import (
	"context"
	"testing"

	working "codenerd/internal/context"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Every delegated task runs on a fresh executor clone with its own random
// working scope, and each scope used to leave a .nerd/context archive behind
// forever -- 320 of them on the dogfood workspace (program of work item 15).
// A clone's scope dies with its task, so its archive goes when the task
// returns; the session's own archive stays while its process runs.
func TestJITExecutor_AnInlineTaskLeavesNoWorkingArchive(t *testing.T) {
	ws := t.TempDir()
	// The multi-round client: the tool loop, and so a working set, runs only
	// for a client that can take tool results back.
	var archivesDuring int
	mockLLM := &MockToolResultsLLM{MockLLMClient: &MockLLMClient{}}
	respond := func() (*types.LLMToolResponse, error) {
		if r, err := working.SurveyWorkingArchives(ws); err == nil {
			archivesDuring = max(archivesDuring, r.Total)
		}
		return &types.LLMToolResponse{Text: "review complete"}, nil
	}
	mockLLM.CompleteWithToolsFunc = func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
		return respond()
	}
	mockLLM.CompleteWithToolResultsFunc = func(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error) {
		return respond()
	}
	mockLLM.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) { return "review complete", nil }
	transducer := &MockTransducer{
		ParseIntentWithContextFunc: func(context.Context, string, []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/review", Category: "/query"}, nil
		},
	}
	executor := NewExecutor(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, transducer)
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	cfg.EnableSafetyGate = false
	cfg.VerifyBuildAfterEdits = false
	cfg.VerifyTestsAfterEdits = false
	cfg.CriticReviewAfterEdits = false
	executor.SetConfig(cfg)
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, transducer, DefaultSpawnerConfig())
	jit := NewJITExecutor(executor, spawner, transducer)

	if _, err := jit.Execute(context.Background(), TaskRequest{IntentVerb: "/review", Task: "Review the change"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if archivesDuring != 1 {
		t.Fatalf("the task's tool loop saw %d archive(s), want its own one: the probe did not exercise a working loop", archivesDuring)
	}
	after, err := working.SurveyWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if after.Total != 0 {
		t.Fatalf("after the task: %s, want its archive gone with its executor", after)
	}
}

// The session's executor keeps its scope across turns (a handle from an
// earlier turn is still redeemable); only the executor that is done retires.
func TestRetireWorkingScopes_TheSessionKeepsItsArchiveAClonesGoes(t *testing.T) {
	root := newWorkingLoopExecutor(t, &MockLLMClient{})
	ws := root.config.WorkspaceRoot
	_, closeRoot, err := root.beginWorkingLoop(context.Background(), "fix a", &prompt.CompilationContext{ShardID: "root"})
	if err != nil {
		t.Fatal(err)
	}
	closeRoot()
	clone := root.CloneForTask()
	_, closeClone, err := clone.beginWorkingLoop(context.Background(), "fix b", &prompt.CompilationContext{ShardID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	closeClone()
	if r, _ := working.SurveyWorkingArchives(ws); r.Total != 2 || r.Redeemable != 2 {
		t.Fatalf("before retiring: %s, want both archives live", r)
	}

	clone.RetireWorkingScopes()

	r, err := working.SurveyWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 1 || r.Redeemable != 1 {
		t.Fatalf("after the clone retired: %s, want the session's archive alone, still redeemable", r)
	}
	// A second turn of the session reopens the same scope's archive.
	_, closeAgain, err := root.beginWorkingLoop(context.Background(), "fix c", &prompt.CompilationContext{ShardID: "root"})
	if err != nil {
		t.Fatal(err)
	}
	closeAgain()
	if r, _ := working.SurveyWorkingArchives(ws); r.Total != 1 {
		t.Fatalf("the session's second turn minted a new archive: %s", r)
	}
}
