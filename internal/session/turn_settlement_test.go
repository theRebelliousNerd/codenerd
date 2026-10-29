package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// A provider failure after tool work is the witness the settlement exists for:
// an authorized call wrote a file, the next provider call failed. The writes,
// their preimages and the checks are measured and live on the result the loop
// was handed; the hard error used to return (nil, err) and lose all of it.

var errProviderDown = errors.New("provider transport: connection reset")

// failAfterWrite scripts one authorized write and then a provider failure on
// the follow-up. followUps counts how often the follow-up provider is called,
// so a test can see that closure made no further model call.
func failAfterWrite(h *repairHarness, followUp func(ctx context.Context) error) *atomic.Int32 {
	var followUps atomic.Int32
	call := h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo)
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{call}}, nil
			},
		},
		CompleteWithToolResultsFunc: func(ctx context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// The working loop sends the opening generation down this channel
			// too; only a request that carries tool results is the follow-up.
			for _, m := range history {
				if len(m.ToolResults) > 0 {
					followUps.Add(1)
					return nil, followUp(ctx)
				}
			}
			return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{call}}, nil
		},
	}
	return &followUps
}

func queryRows(t *testing.T, e *Executor, predicate string) []types.Fact {
	t.Helper()
	rows, err := e.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return rows
}

func TestSettleTurn_HardLoopErrorKeepsTheMeasuredWork(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	followUps := failAfterWrite(h, func(context.Context) error { return errProviderDown })

	result, err := h.drive(t, "fix write main.go")

	if err == nil || !errors.Is(err, errProviderDown) {
		t.Fatalf("err = %v, want the provider's error unchanged under the wrap", err)
	}
	if result == nil {
		t.Fatal("the hard error discarded the result that holds the writes")
	}
	if !errors.Is(result.Error, errProviderDown) {
		t.Errorf("result.Error = %v, want the primary failure", result.Error)
	}
	if got, want := result.WriteToolEffects, 1; got != want {
		t.Errorf("WriteToolEffects = %d, want %d", got, want)
	}
	if len(result.WrittenPaths) != 1 || !strings.HasSuffix(result.WrittenPaths[0], "main.go") {
		t.Errorf("WrittenPaths = %v, want main.go", result.WrittenPaths)
	}
	if len(result.PreWriteContents) != 1 {
		t.Errorf("PreWriteContents = %v, want the write's preimage", result.PreWriteContents)
	}
	if len(result.ToolReceipts) != 1 {
		t.Errorf("ToolReceipts = %d, want the write's receipt", len(result.ToolReceipts))
	}
	if disk, rerr := os.ReadFile(filepath.Join(h.ws, "main.go")); rerr != nil || string(disk) != repairMainGo {
		t.Errorf("the effect is not on disk as written: %q %v", disk, rerr)
	}
	// The settlement is accounting, not more work: the one write happened once
	// and the provider was asked once.
	if got := followUps.Load(); got != 1 {
		t.Errorf("follow-up provider calls = %d, want 1: closure must not call a model or replay a batch", got)
	}
	if turnSucceeded(result, err) {
		t.Error("the failed turn settles as a success")
	}
}

func TestSettleTurn_EveryConsumerReadsTheSameFailure(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	failAfterWrite(h, func(context.Context) error { return errProviderDown })

	result, err := h.drive(t, "fix write main.go")
	if result == nil || err == nil {
		t.Fatalf("result=%v err=%v", result, err)
	}

	// The kernel's verdict and the turn record agree with the caller.
	if result.TurnOutcome != types.MangleAtom("/failed") {
		t.Errorf("TurnOutcome = %q, want /failed", result.TurnOutcome)
	}
	costs := queryRows(t, h.executor, "turn_cost")
	if len(costs) != 1 {
		t.Fatalf("turn_cost rows = %d, want the failed turn counted once", len(costs))
	}
	if got := types.ExtractString(costs[0].Args[len(costs[0].Args)-1]); got != "/failed" {
		t.Errorf("turn_cost outcome = %q, want /failed", got)
	}
	// The parent sees the change and the failure together.
	observed := withTurnWrites(observedReturn("coder", "task", result), h.executor.workspaceForVerification(), result)
	if len(observed.Changed) != 1 || observed.Failure == "" || observed.Outcome != "/failed" {
		t.Errorf("observed return lost the partial state: changed=%v failure=%q outcome=%q", observed.Changed, observed.Failure, observed.Outcome)
	}
	if len(observed.Writes) != 1 || !observed.Writes[0].After.Exists {
		t.Errorf("the write record a rollback needs is missing: %+v", observed.Writes)
	}
	if !strings.Contains(observed.PartialState(), "main.go") {
		t.Errorf("the partial state does not name the changed file:\n%s", observed.PartialState())
	}
	// Checks the turn never reached stay unestablished: nothing claims success.
	if observed.Done() {
		t.Error("a failed turn is reported done")
	}
}

func TestSettleTurn_RetiresExactlyTheTurnsFacts(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	failAfterWrite(h, func(context.Context) error { return errProviderDown })

	// A sibling executor on the same kernel holds a live turn fact. The failed
	// turn must retire its own and leave that one alone.
	sibling := h.executor.CloneForTask()
	siblingFact := types.Fact{Predicate: "turn_verb", Args: []any{types.MangleAtom("/turn_sibling"), types.MangleAtom("/fix")}}
	if !sibling.assertTurnFact(siblingFact) {
		t.Fatal("could not assert the sibling's fact")
	}

	result, err := h.drive(t, "fix write main.go")
	if result == nil || err == nil {
		t.Fatalf("result=%v err=%v", result, err)
	}

	h.executor.mu.Lock()
	leaked := len(h.executor.turnFacts) + len(h.executor.turnCreatedSources) + len(h.executor.turnCreatedTests)
	h.executor.mu.Unlock()
	if leaked != 0 {
		t.Errorf("%d per-turn record(s) survived the failed turn", leaked)
	}
	var own, sib int
	for _, row := range queryRows(t, h.executor, "turn_verb") {
		switch types.ExtractString(row.Args[0]) {
		case string(result.turnAtom()):
			own++
		case "/turn_sibling":
			sib++
		}
	}
	if own != 0 {
		t.Errorf("the failed turn's own turn_verb survived (%d)", own)
	}
	if sib != 1 {
		t.Errorf("the sibling's turn_verb rows = %d, want 1: cleanup reached past its own turn", sib)
	}
	sibling.mu.Lock()
	siblingFacts := len(sibling.turnFacts)
	sibling.mu.Unlock()
	if siblingFacts != 1 {
		t.Errorf("the sibling executor lost its record (%d)", siblingFacts)
	}
}

func TestSettleTurn_ACanceledTurnIsStillAccountedFor(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failAfterWrite(h, func(c context.Context) error {
		cancel()
		return c.Err()
	})

	result, err := h.executor.ProcessWithIntent(ctx, "fix write main.go", presetIntentForTask("/fix", "fix write main.go", ""))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation identifiable", err)
	}
	if result == nil || result.WriteToolEffects != 1 {
		t.Fatalf("the write completed before the cancel is not retained: %+v", result)
	}
	if result.TurnOutcome != types.MangleAtom("/failed") {
		t.Errorf("TurnOutcome = %q, want the canceled turn settled /failed", result.TurnOutcome)
	}
	if len(queryRows(t, h.executor, "turn_cost")) != 1 {
		t.Error("the canceled turn's cost record was never written: closure waited on a canceled context")
	}
	if result.Acceptance != nil && result.Acceptance.Status == "verified" {
		t.Error("a canceled turn reports verified acceptance")
	}
}

func TestSettleTurn_FirstProviderCallFailingIsAFailedTurnWithNoEffects(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return nil, errProviderDown
			},
		},
		CompleteWithToolResultsFunc: func(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return nil, errProviderDown
		},
	}

	result, err := h.drive(t, "fix write main.go")

	if !errors.Is(err, errProviderDown) {
		t.Fatalf("err = %v", err)
	}
	if result == nil || result.WriteToolEffects != 0 || len(result.WrittenPaths) != 0 {
		t.Fatalf("a turn that never reached a tool reports effects: %+v", result)
	}
	if result.TurnOutcome != types.MangleAtom("/failed") {
		t.Errorf("TurnOutcome = %q", result.TurnOutcome)
	}
	if got := observedReturn("coder", "task", result).PartialState(); got != "" {
		t.Errorf("invented a partial state for a turn that did nothing:\n%s", got)
	}
}

func TestSettleTurn_ATurnRefusedBeforeAdmissionHasNoResult(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	result, err := h.executor.ProcessWithIntent(context.Background(), "   ", nil)
	if err == nil || result != nil {
		t.Fatalf("an empty request was admitted: result=%v err=%v", result, err)
	}
}

func TestTurnSucceeded_ReadsWhatWasReturned(t *testing.T) {
	failed := &ExecutionResult{Error: errProviderDown}
	for name, tc := range map[string]struct {
		result *ExecutionResult
		err    error
		want   bool
	}{
		"clean":                                {&ExecutionResult{}, nil, true},
		"early return, no error on the result": {&ExecutionResult{}, errProviderDown, false},
		"soft failure":                         {failed, nil, false},
		"hard failure":                         {failed, errProviderDown, false},
		"no result":                            {nil, nil, false},
	} {
		if got := turnSucceeded(tc.result, tc.err); got != tc.want {
			t.Errorf("%s: turnSucceeded = %v, want %v", name, got, tc.want)
		}
	}
}

// The inline task topology returns the failure and the worker's observation
// together.
func TestSettleTurn_InlineTaskKeepsPartialStateOnError(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	failAfterWrite(h, func(context.Context) error { return errProviderDown })
	exec := NewJITExecutor(h.executor, nil, &MockTransducer{})

	observed, err := exec.ExecuteObserved(context.Background(), TaskRequest{IntentVerb: "/fix", Task: "write main.go"})

	if !errors.Is(err, errProviderDown) {
		t.Fatalf("err = %v, want the provider failure identifiable", err)
	}
	if len(observed.Changed) != 1 || len(observed.Writes) != 1 {
		t.Errorf("the observation lost the write: changed=%v writes=%d", observed.Changed, len(observed.Writes))
	}
	if observed.Failure == "" || observed.Done() {
		t.Errorf("the observation does not read as a failure: %+v", observed)
	}
}

// The subagent topology publishes the same partial observation.
func TestSettleTurn_SubagentKeepsPartialStateOnError(t *testing.T) {
	h := newRepairHarness(t, nil)
	failAfterWrite(h, func(context.Context) error { return errProviderDown })
	cfg := DefaultSubAgentConfig("coder")
	cfg.IntentVerb = "/fix"
	agent := NewSubAgent(cfg, &MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	agent.executor = h.executor

	_, err := agent.execute(context.Background(), "write main.go")

	if !errors.Is(err, errProviderDown) {
		t.Fatalf("err = %v", err)
	}
	observed := agent.ObservedReturn()
	if len(observed.Changed) != 1 || observed.Failure == "" || observed.Outcome != "/failed" {
		t.Errorf("the published observation is impoverished: %+v", observed)
	}
}

// A routed constraint reaches the isolated worker's preset intent, as it
// reaches the inline one's: the spawn request carries it beside the task text.
func TestSubagent_PresetIntentCarriesTheRoutedConstraint(t *testing.T) {
	kernel := &MockKernel{}
	cfg := DefaultSubAgentConfig("coder")
	cfg.IntentVerb = "/fix"
	cfg.IntentTarget = "a.go"
	cfg.IntentConstraint = "  keep the exported API  "
	llm := &MockLLMClient{
		CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return nil, errProviderDown
		},
	}
	agent := NewSubAgent(cfg, kernel, &MockVirtualStore{}, llm, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	_, _ = agent.execute(context.Background(), "fix a.go")

	kernel.mu.RLock()
	defer kernel.mu.RUnlock()
	for _, f := range kernel.asserts {
		if f.Predicate == "user_intent" {
			if got := types.ExtractString(f.Args[4]); got != "keep the exported API" {
				t.Fatalf("the worker's intent constraint = %q", got)
			}
			return
		}
	}
	t.Fatal("the worker never asserted its intent")
}

// A turn that ends before the model is ever called still retires what it
// asserted. Its catalog facts are recorded before the prompt compiles; a
// baseline that cannot be assembled used to return without retiring them.
func TestSettleTurn_AnEarlyExitRetiresTheCatalogFacts(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var heldAtCompile int
	h.executor.jitCompiler = &MockJITCompiler{CompileFunc: func(context.Context, *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		h.executor.mu.Lock()
		heldAtCompile = len(h.executor.turnFacts)
		h.executor.mu.Unlock()
		cancel()
		return nil, errors.New("compiler stopped")
	}}

	result, err := h.executor.ProcessWithIntent(ctx, "fix write main.go", presetIntentForTask("/fix", "fix write main.go", ""))

	if err == nil || result == nil {
		t.Fatalf("result=%v err=%v, want a failed admitted turn", result, err)
	}
	if heldAtCompile == 0 {
		t.Fatal("precondition: the turn had recorded no facts before the compile, so this proves nothing")
	}
	h.executor.mu.Lock()
	left := len(h.executor.turnFacts)
	h.executor.mu.Unlock()
	if left != 0 {
		t.Errorf("%d turn fact(s) survived an exit before the model was called", left)
	}
}
