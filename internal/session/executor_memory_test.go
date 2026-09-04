package session

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// memoryHydrationCall records one HydrateSessionContext invocation.
type memoryHydrationCall struct {
	sessionID string
	query     string
}

// memoryHydrationFakeStore is a VirtualStore that also implements
// MemoryHydrator, recording every hydration call.
type memoryHydrationFakeStore struct {
	MockVirtualStore
	mu             sync.Mutex
	learningsCalls int
	sessionCalls   []memoryHydrationCall
}

func (f *memoryHydrationFakeStore) HydrateLearnings(ctx context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.learningsCalls++
	return 0, nil
}

func (f *memoryHydrationFakeStore) HydrateSessionContext(ctx context.Context, sessionID, query string, shardTypes []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionCalls = append(f.sessionCalls, memoryHydrationCall{sessionID: sessionID, query: query})
	return 0, nil
}

// newMemoryTestExecutor builds an executor whose LLM answers in prose and
// whose transducer returns /greet, so turns complete without tool calls.
func newMemoryTestExecutor(store types.VirtualStore, jit JITCompiler) (*Executor, *MockKernel) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "memory test response"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "memory test response", nil
		},
	}
	transducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	}
	return NewExecutor(kernel, store, llm, jit, &MockConfigFactory{}, transducer), kernel
}

// newMemoryTestStore creates a real LocalStore, which already implements
// SessionPersister, so the atomsJSON assertions exercise the genuine
// session_turns write path instead of a mock.
func newMemoryTestStore(t *testing.T, name string) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("create local store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// awaitSessionRow polls GetSessionHistory until the turn row appears, hiding
// the asynchronous persistTurn goroutine without racy sleeps.
func awaitSessionRow(t *testing.T, db *store.LocalStore, sessionID string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		history, err := db.GetSessionHistory(sessionID, 10)
		if err != nil {
			t.Fatalf("GetSessionHistory: %v", err)
		}
		if len(history) > 0 {
			return history[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the persisted turn")
	return nil
}

// decodeAtomsIDs unmarshals an atoms JSON payload into its atom ID list.
func decodeAtomsIDs(t *testing.T, atomsJSON string) []string {
	t.Helper()
	var ids []string
	if err := json.Unmarshal([]byte(atomsJSON), &ids); err != nil {
		t.Fatalf("atoms %q is not a JSON string array: %v", atomsJSON, err)
	}
	return ids
}

// requireSingleTurnCost returns the one turn_cost fact a turn must assert.
func requireSingleTurnCost(t *testing.T, kernel *MockKernel) types.Fact {
	t.Helper()
	facts, err := kernel.Query("turn_cost")
	if err != nil {
		t.Fatalf("query turn_cost: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("turn_cost facts = %d, want exactly 1 per turn", len(facts))
	}
	if len(facts[0].Args) != 6 {
		t.Fatalf("turn_cost args = %d, want 6 (SessionID, TurnNum, Prompt, Completion, Tools, Outcome)", len(facts[0].Args))
	}
	return facts[0]
}

// requireTurnCostInt checks one numeric turn_cost slot.
func requireTurnCostInt(t *testing.T, fact types.Fact, index int, want int64, name string) {
	t.Helper()
	n, ok := types.ExtractInt64(fact.Args[index])
	if !ok || n != want {
		t.Errorf("turn_cost %s = %v, want %d", name, fact.Args[index], want)
	}
}

// requireTurnCostOutcome checks the outcome slot against the allowed set.
func requireTurnCostOutcome(t *testing.T, fact types.Fact, allowed ...string) {
	t.Helper()
	got := types.ExtractString(fact.Args[5])
	for _, want := range allowed {
		if got == want {
			return
		}
	}
	t.Errorf("turn_cost outcome = %q, want one of %v", got, allowed)
}

func TestExecutorHydration_LearningsOnceContextPerTurn(t *testing.T) {
	store := &memoryHydrationFakeStore{}
	executor, _ := newMemoryTestExecutor(store, &MockJITCompiler{})
	executor.SetSessionID("hydration-sess")
	inputs := []string{"first question", "second question", "third question"}
	for _, in := range inputs {
		if _, err := executor.Process(context.Background(), in); err != nil {
			t.Fatalf("Process(%q) failed: %v", in, err)
		}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.learningsCalls != 1 {
		t.Errorf("HydrateLearnings calls = %d, want exactly 1 across %d turns", store.learningsCalls, len(inputs))
	}
	if len(store.sessionCalls) != len(inputs) {
		t.Fatalf("HydrateSessionContext calls = %d, want %d (one per turn)", len(store.sessionCalls), len(inputs))
	}
	for i, call := range store.sessionCalls {
		if call.sessionID != "hydration-sess" {
			t.Errorf("turn %d sessionID = %q, want %q", i, call.sessionID, "hydration-sess")
		}
		if call.query != inputs[i] {
			t.Errorf("turn %d query = %q, want input %q", i, call.query, inputs[i])
		}
	}
}

func TestExecutorHydration_SkippedWithoutInterface(t *testing.T) {
	executor, _ := newMemoryTestExecutor(&MockVirtualStore{}, &MockJITCompiler{})
	executor.SetSessionID("plain-sess")
	if _, err := executor.Process(context.Background(), "hello"); err != nil {
		t.Fatalf("Process with a store lacking the hydration interface failed: %v", err)
	}
}

func TestExecutorPersistTurn_AtomsJSONRecordsSelectedAtoms(t *testing.T) {
	db := newMemoryTestStore(t, "atoms.db")
	jit := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{
				Prompt:        "memory test prompt",
				IncludedAtoms: []*prompt.PromptAtom{{ID: "atom-one"}, {ID: "atom-two"}},
			}, nil
		},
	}
	executor, _ := newMemoryTestExecutor(&MockVirtualStore{}, jit)
	executor.SetSessionPersister(db)
	executor.SetSessionID("atoms-sess")
	if _, err := executor.Process(context.Background(), "hello"); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	row := awaitSessionRow(t, db, "atoms-sess")
	atoms, ok := row["atoms"].(string)
	if !ok {
		t.Fatalf("persisted atoms = %v (%T), want a JSON string", row["atoms"], row["atoms"])
	}
	ids := decodeAtomsIDs(t, atoms)
	if len(ids) != 2 || ids[0] != "atom-one" || ids[1] != "atom-two" {
		t.Errorf("persisted atoms ids = %v, want [atom-one atom-two]", ids)
	}
}

func TestExecutorPersistTurn_AtomsJSONEmptyWhenCompilationSkipped(t *testing.T) {
	db := newMemoryTestStore(t, "atoms-empty.db")
	executor, _ := newMemoryTestExecutor(&MockVirtualStore{}, nil)
	executor.SetSessionPersister(db)
	executor.SetSessionID("atoms-empty-sess")
	if _, err := executor.Process(context.Background(), "hello"); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	row := awaitSessionRow(t, db, "atoms-empty-sess")
	if atoms, ok := row["atoms"].(string); !ok || atoms != "[]" {
		t.Errorf("persisted atoms = %v, want %q (empty array when compilation was skipped)", row["atoms"], "[]")
	}
}

func TestExecutorTurnCost_AssertedPerTurn(t *testing.T) {
	executor, kernel := newMemoryTestExecutor(&MockVirtualStore{}, &MockJITCompiler{})
	executor.SetSessionID("cost-sess")
	if _, err := executor.Process(context.Background(), "hello"); err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	fact := requireSingleTurnCost(t, kernel)
	if got := types.ExtractString(fact.Args[0]); got != "cost-sess" {
		t.Errorf("turn_cost session = %q, want %q", got, "cost-sess")
	}
	requireTurnCostInt(t, fact, 1, 1, "turn")
	requireTurnCostInt(t, fact, 2, 0, "prompt tokens")
	requireTurnCostInt(t, fact, 3, 0, "completion tokens")
	requireTurnCostInt(t, fact, 4, 0, "tool calls")
	requireTurnCostOutcome(t, fact, "/done", "/hollow", "/failed", "/unverified")
}

// A read-only turn earns turn_done like any other (it used to return before
// asserting evidence, so every /explain landed as /unverified), and is never
// failed for hollowness even when a verb-agnostic hollow rule fires.
func TestExecutorTurnCost_ReadOnlyTurnIsVerifiedNotFailed(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.SetSessionID("cost-readonly-sess")

	clean := &ExecutionResult{Response: "the file asserts one fact per turn", ToolCallsExecuted: 1, SuccessfulToolCalls: 1}
	clean.Intent.Verb = "/explain"
	clean.Intent.Category = "/query"
	if err := executor.checkHollowSuccess(clean); err != nil {
		t.Fatalf("read-only turn must never fail hollow checks: %v", err)
	}
	if clean.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("clean read-only TurnOutcome = %q, want /done", clean.TurnOutcome)
	}

	// Claimed test-runner output with no test tool: the verb-agnostic rule
	// fires, the outcome records it, the turn still does not fail.
	claimed := &ExecutionResult{Response: "--- PASS: TestX (0.00s)\nok  \tcodenerd/internal/x\t0.1s", ToolCallsExecuted: 1, SuccessfulToolCalls: 1}
	claimed.Intent.Verb = "/explain"
	claimed.Intent.Category = "/query"
	if err := executor.checkHollowSuccess(claimed); err != nil {
		t.Fatalf("read-only turn must never fail hollow checks even with a claimed test output: %v", err)
	}
	if claimed.TurnOutcome == types.MangleAtom("/unverified") || claimed.TurnOutcome == "" {
		t.Fatalf("claimed-output read-only TurnOutcome = %q, want a kernel verdict (/done or /hollow)", claimed.TurnOutcome)
	}
}

func TestExecutorTurnCost_DoneOutcomeOnVerifiedTurn(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.SetSessionID("cost-done-sess")
	result := &ExecutionResult{Response: "done", ToolCallsExecuted: 1, SuccessfulToolCalls: 1}
	result.Intent.Verb = "/run"
	result.Intent.Category = "/action"
	if err := executor.checkHollowSuccess(result); err != nil {
		t.Fatalf("checkHollowSuccess for a clean /run turn with a successful tool call: %v", err)
	}
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %q, want /done (kernel derived turn_done)", result.TurnOutcome)
	}
	executor.persistTurn(context.Background(), "run the thing", result.Intent, result, turnTelemetry{})
	facts, err := kernel.Query("turn_cost")
	if err != nil {
		t.Fatalf("query turn_cost: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("turn_cost facts = %d, want 1", len(facts))
	}
	if got := types.ExtractString(facts[0].Args[0]); got != "cost-done-sess" {
		t.Errorf("turn_cost session = %q, want %q", got, "cost-done-sess")
	}
	if got := types.ExtractString(facts[0].Args[5]); got != "/done" {
		t.Errorf("turn_cost outcome = %q, want /done", got)
	}
}
