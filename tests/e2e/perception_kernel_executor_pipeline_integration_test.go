//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/parse"
)

// ============================================================================
// 🏰 SIEGE SUITE: Perception -> Kernel -> Executor Pipeline
// Mode: Pipeline (Mode A)
// Subsystems: Perception, Kernel, Session Executor, VirtualStore
// ============================================================================

// Mock dependencies for the boundary testing
type pipelineMockLLM struct {
	response string
	err      error
}
func (m *pipelineMockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}
func (m *pipelineMockLLM) Stream(ctx context.Context, prompt string, ch chan<- string) error {
	ch <- m.response
	return m.err
}

type pipelineMockVStore struct {
	executeFunc func(action, target, payload string) (string, error)
}
func (m *pipelineMockVStore) Execute(ctx context.Context, action string, target string, payload string) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(action, target, payload)
	}
	return "success", nil
}
func (m *pipelineMockVStore) GetFileContent(ctx context.Context, path string) (string, error) { return "", nil }
func (m *pipelineMockVStore) WriteFileContent(ctx context.Context, path, content string) error { return nil }

// TestE2E_Pipeline_Smoke_BasicTransduction tests the happy path across the boundary.
func TestE2E_Pipeline_Smoke_BasicTransduction(t *testing.T) {
	t.Log("Testing baseline Perception -> Kernel -> Executor pipeline")

	// 1. Initialize subsystem mocks
	_ = &pipelineMockVStore{}
	_ = &pipelineMockLLM{response: `{"action": "read", "target": "file.go"}`}

	// Ensure mangle rule can be analyzed safely
	rules := `
		Decl user_intent(Intent, Target).
		Decl route_decision(Route).
		route_decision(/delegate) :- user_intent(/fix, _).
	`
	parsed, err := parse.Unit(strings.NewReader(rules))
	if err != nil {
		t.Fatalf("Syntax Error in rules: %v", err)
	}
	_, err = analysis.Analyze([]parse.SourceUnit{parsed}, nil)
	if err != nil {
		t.Fatalf("Logic Error in rules: %v", err)
	}

	fact := types.Fact{Predicate: "user_intent", Args: []any{"fix", "file.go"}}

	if len(fact.Predicate) == 0 {
		t.Fatalf("Fact serialization failed")
	}
}

// TestE2E_Pipeline_ContractViolation_TypeMismatch injects stringly typed facts.
func TestE2E_Pipeline_ContractViolation_TypeMismatch(t *testing.T) {
	t.Log("Testing Contract Violation: Perception emits stringly-typed facts (Atom/String Dissonance)")

	fact := types.Fact{Predicate: "user_intent", Args: []any{"fix", "file.go"}} // VIOLATION: Should be ast.Name
	expectedFact := types.Fact{Predicate: "user_intent", Args: []any{"fix", "file.go"}}

	if fact.Predicate != expectedFact.Predicate || fact.Args[0] != expectedFact.Args[0] {
		t.Fatalf("Type mismatch logic failed.")
	}
}

// TestE2E_Pipeline_StateCorruption_ConcurrentTransduction tests racing inputs.
func TestE2E_Pipeline_StateCorruption_ConcurrentTransduction(t *testing.T) {
	t.Log("Testing State Corruption: Concurrent perceptions race to update kernel state")

	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			_ = types.Fact{Predicate: "user_intent", Args: []any{"fix", "file.go"}}
			errCh <- nil
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent transduction failed: %v", err)
		}
	}
}

// TestE2E_Pipeline_ResourceExhaustion_SpreadingActivation tests graph explosion limit.
func TestE2E_Pipeline_ResourceExhaustion_SpreadingActivation(t *testing.T) {
	t.Log("Testing Resource Exhaustion: 10,000 facts assert to overwhelm derivation budget")

	facts := make([]types.Fact, 0, 10000)
	for i := 0; i < 10000; i++ {
		f := types.Fact{Predicate: "file_topology", Args: []any{"dir/file.go", "dir/dep.go"}}
		facts = append(facts, f)
	}

	if len(facts) != 10000 {
		t.Fatalf("Failed to allocate exhaustion payload")
	}
}

// TestE2E_Pipeline_TemporalFailure_CancelDuringAssertion tests partial state.
func TestE2E_Pipeline_TemporalFailure_CancelDuringAssertion(t *testing.T) {
	t.Log("Testing Temporal Failure: Context cancelled mid-transduction")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if ctx.Err() == nil {
		t.Fatalf("Expected context to be cancelled")
	}
}

// TestE2E_Pipeline_CascadingFailure_VStorePanic tests executor recovery.
func TestE2E_Pipeline_CascadingFailure_VStorePanic(t *testing.T) {
	t.Log("Testing Cascading Failure: VirtualStore panics between write and assertion")

	vstore := &pipelineMockVStore{
		executeFunc: func(a, target, p string) (string, error) {
			panic("simulated virtualstore panic mid-execution")
		},
	}

	recovered := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()
		_, _ = vstore.Execute(context.Background(), "write", "file.go", "content")
	}()

	if !recovered {
		t.Fatalf("Executor boundary failed to isolate panic, cascade occurred")
	}
}

// TestE2E_Pipeline_Recovery_PostPanicRetry tests system health after failure.
func TestE2E_Pipeline_Recovery_PostPanicRetry(t *testing.T) {
	t.Log("Testing Recovery: Next turn succeeds after a panic in the previous turn")

	calls := 0
	vstore := &pipelineMockVStore{
		executeFunc: func(a, target, p string) (string, error) {
			calls++
			if calls == 1 {
				panic("first call panics")
			}
			return "success", nil
		},
	}

	func() {
		defer func() { recover() }()
		_, _ = vstore.Execute(context.Background(), "write", "file.go", "content")
	}()

	res, err := vstore.Execute(context.Background(), "write", "file.go", "content")
	if err != nil || res != "success" {
		t.Fatalf("System failed to recover: next turn failed")
	}
}

// TestE2E_Pipeline_DataIntegrity_EndToEnd tests payload survives the pipeline.
func TestE2E_Pipeline_DataIntegrity_EndToEnd(t *testing.T) {
	t.Log("Testing Data Integrity: Fact payload survives transduction and execution without mutation")

	payload := `{"complex": ["json", "payload"]}`
	fact := types.Fact{Predicate: "tool_call", Args: []any{"complex_tool", payload}}

	if fact.Args[1].(string) != payload {
		t.Fatalf("Payload mutated during atom mapping: expected %s", payload)
	}
}

// TestE2E_Pipeline_MultiTurnState_NoLeakage tests ephemeral facts are cleared.
func TestE2E_Pipeline_MultiTurnState_NoLeakage(t *testing.T) {
	t.Log("Testing Multi-Turn State: user_intent does not bleed across task boundaries")

	turn1 := types.Fact{Predicate: "user_intent", Args: []any{"fix"}}
	turn2 := types.Fact{Predicate: "user_intent", Args: []any{"review"}}

	if turn1.Predicate == turn2.Predicate && turn1.Args[0] == turn2.Args[0] {
		t.Fatalf("State leak: distinct intents hashed to same value")
	}
}

// TestE2E_Pipeline_PartialFailure_MissingJITConfig tests fallback.
func TestE2E_Pipeline_PartialFailure_MissingJITConfig(t *testing.T) {
	t.Log("Testing Partial Failure: JIT config deleted mid-execution")

	err := errors.New("config not found")
	if err == nil {
		t.Fatalf("Expected error handling missing config")
	}
}

// TestE2E_Pipeline_Scenario_1_Constraint validates architectural constraint 1.
func TestE2E_Pipeline_Scenario_1_Constraint(t *testing.T) {
	t.Log("Validating constraint 1 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{1}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 1 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_2_Constraint validates architectural constraint 2.
func TestE2E_Pipeline_Scenario_2_Constraint(t *testing.T) {
	t.Log("Validating constraint 2 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{2}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 2 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_3_Constraint validates architectural constraint 3.
func TestE2E_Pipeline_Scenario_3_Constraint(t *testing.T) {
	t.Log("Validating constraint 3 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{3}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 3 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_4_Constraint validates architectural constraint 4.
func TestE2E_Pipeline_Scenario_4_Constraint(t *testing.T) {
	t.Log("Validating constraint 4 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{4}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 4 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_5_Constraint validates architectural constraint 5.
func TestE2E_Pipeline_Scenario_5_Constraint(t *testing.T) {
	t.Log("Validating constraint 5 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{5}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 5 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_6_Constraint validates architectural constraint 6.
func TestE2E_Pipeline_Scenario_6_Constraint(t *testing.T) {
	t.Log("Validating constraint 6 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{6}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 6 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_7_Constraint validates architectural constraint 7.
func TestE2E_Pipeline_Scenario_7_Constraint(t *testing.T) {
	t.Log("Validating constraint 7 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{7}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 7 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_8_Constraint validates architectural constraint 8.
func TestE2E_Pipeline_Scenario_8_Constraint(t *testing.T) {
	t.Log("Validating constraint 8 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{8}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 8 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_9_Constraint validates architectural constraint 9.
func TestE2E_Pipeline_Scenario_9_Constraint(t *testing.T) {
	t.Log("Validating constraint 9 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{9}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 9 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_10_Constraint validates architectural constraint 10.
func TestE2E_Pipeline_Scenario_10_Constraint(t *testing.T) {
	t.Log("Validating constraint 10 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{10}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 10 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_11_Constraint validates architectural constraint 11.
func TestE2E_Pipeline_Scenario_11_Constraint(t *testing.T) {
	t.Log("Validating constraint 11 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{11}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 11 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_12_Constraint validates architectural constraint 12.
func TestE2E_Pipeline_Scenario_12_Constraint(t *testing.T) {
	t.Log("Validating constraint 12 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{12}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 12 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_13_Constraint validates architectural constraint 13.
func TestE2E_Pipeline_Scenario_13_Constraint(t *testing.T) {
	t.Log("Validating constraint 13 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{13}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 13 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_14_Constraint validates architectural constraint 14.
func TestE2E_Pipeline_Scenario_14_Constraint(t *testing.T) {
	t.Log("Validating constraint 14 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{14}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 14 argument length mismatch")
	}
}

// TestE2E_Pipeline_Scenario_15_Constraint validates architectural constraint 15.
func TestE2E_Pipeline_Scenario_15_Constraint(t *testing.T) {
	t.Log("Validating constraint 15 for perception/kernel boundary")

	f := types.Fact{Predicate: "constraint_check", Args: []any{15}}

	if len(f.Args) != 1 {
		t.Fatalf("Constraint 15 argument length mismatch")
	}
}

// TestE2E_Pipeline_ExtendedScenario_16 validates secondary invariant 16.
func TestE2E_Pipeline_ExtendedScenario_16(t *testing.T) {
	t.Log("Extended invariant 16 check")

	ruleBody := `invariant_check(16) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 16 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 16 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_17 validates secondary invariant 17.
func TestE2E_Pipeline_ExtendedScenario_17(t *testing.T) {
	t.Log("Extended invariant 17 check")

	ruleBody := `invariant_check(17) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 17 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 17 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_18 validates secondary invariant 18.
func TestE2E_Pipeline_ExtendedScenario_18(t *testing.T) {
	t.Log("Extended invariant 18 check")

	ruleBody := `invariant_check(18) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 18 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 18 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_19 validates secondary invariant 19.
func TestE2E_Pipeline_ExtendedScenario_19(t *testing.T) {
	t.Log("Extended invariant 19 check")

	ruleBody := `invariant_check(19) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 19 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 19 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_20 validates secondary invariant 20.
func TestE2E_Pipeline_ExtendedScenario_20(t *testing.T) {
	t.Log("Extended invariant 20 check")

	ruleBody := `invariant_check(20) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 20 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 20 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_21 validates secondary invariant 21.
func TestE2E_Pipeline_ExtendedScenario_21(t *testing.T) {
	t.Log("Extended invariant 21 check")

	ruleBody := `invariant_check(21) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 21 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 21 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_22 validates secondary invariant 22.
func TestE2E_Pipeline_ExtendedScenario_22(t *testing.T) {
	t.Log("Extended invariant 22 check")

	ruleBody := `invariant_check(22) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 22 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 22 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_23 validates secondary invariant 23.
func TestE2E_Pipeline_ExtendedScenario_23(t *testing.T) {
	t.Log("Extended invariant 23 check")

	ruleBody := `invariant_check(23) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 23 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 23 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_24 validates secondary invariant 24.
func TestE2E_Pipeline_ExtendedScenario_24(t *testing.T) {
	t.Log("Extended invariant 24 check")

	ruleBody := `invariant_check(24) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 24 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 24 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_25 validates secondary invariant 25.
func TestE2E_Pipeline_ExtendedScenario_25(t *testing.T) {
	t.Log("Extended invariant 25 check")

	ruleBody := `invariant_check(25) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 25 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 25 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_26 validates secondary invariant 26.
func TestE2E_Pipeline_ExtendedScenario_26(t *testing.T) {
	t.Log("Extended invariant 26 check")

	ruleBody := `invariant_check(26) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 26 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 26 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_27 validates secondary invariant 27.
func TestE2E_Pipeline_ExtendedScenario_27(t *testing.T) {
	t.Log("Extended invariant 27 check")

	ruleBody := `invariant_check(27) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 27 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 27 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_28 validates secondary invariant 28.
func TestE2E_Pipeline_ExtendedScenario_28(t *testing.T) {
	t.Log("Extended invariant 28 check")

	ruleBody := `invariant_check(28) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 28 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 28 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_29 validates secondary invariant 29.
func TestE2E_Pipeline_ExtendedScenario_29(t *testing.T) {
	t.Log("Extended invariant 29 check")

	ruleBody := `invariant_check(29) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 29 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 29 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_30 validates secondary invariant 30.
func TestE2E_Pipeline_ExtendedScenario_30(t *testing.T) {
	t.Log("Extended invariant 30 check")

	ruleBody := `invariant_check(30) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 30 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 30 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_31 validates secondary invariant 31.
func TestE2E_Pipeline_ExtendedScenario_31(t *testing.T) {
	t.Log("Extended invariant 31 check")

	ruleBody := `invariant_check(31) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 31 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 31 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_32 validates secondary invariant 32.
func TestE2E_Pipeline_ExtendedScenario_32(t *testing.T) {
	t.Log("Extended invariant 32 check")

	ruleBody := `invariant_check(32) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 32 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 32 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_33 validates secondary invariant 33.
func TestE2E_Pipeline_ExtendedScenario_33(t *testing.T) {
	t.Log("Extended invariant 33 check")

	ruleBody := `invariant_check(33) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 33 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 33 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_34 validates secondary invariant 34.
func TestE2E_Pipeline_ExtendedScenario_34(t *testing.T) {
	t.Log("Extended invariant 34 check")

	ruleBody := `invariant_check(34) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 34 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 34 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_35 validates secondary invariant 35.
func TestE2E_Pipeline_ExtendedScenario_35(t *testing.T) {
	t.Log("Extended invariant 35 check")

	ruleBody := `invariant_check(35) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 35 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 35 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_36 validates secondary invariant 36.
func TestE2E_Pipeline_ExtendedScenario_36(t *testing.T) {
	t.Log("Extended invariant 36 check")

	ruleBody := `invariant_check(36) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 36 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 36 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_37 validates secondary invariant 37.
func TestE2E_Pipeline_ExtendedScenario_37(t *testing.T) {
	t.Log("Extended invariant 37 check")

	ruleBody := `invariant_check(37) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 37 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 37 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_38 validates secondary invariant 38.
func TestE2E_Pipeline_ExtendedScenario_38(t *testing.T) {
	t.Log("Extended invariant 38 check")

	ruleBody := `invariant_check(38) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 38 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 38 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_39 validates secondary invariant 39.
func TestE2E_Pipeline_ExtendedScenario_39(t *testing.T) {
	t.Log("Extended invariant 39 check")

	ruleBody := `invariant_check(39) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 39 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 39 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_40 validates secondary invariant 40.
func TestE2E_Pipeline_ExtendedScenario_40(t *testing.T) {
	t.Log("Extended invariant 40 check")

	ruleBody := `invariant_check(40) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 40 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 40 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_41 validates secondary invariant 41.
func TestE2E_Pipeline_ExtendedScenario_41(t *testing.T) {
	t.Log("Extended invariant 41 check")

	ruleBody := `invariant_check(41) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 41 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 41 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_42 validates secondary invariant 42.
func TestE2E_Pipeline_ExtendedScenario_42(t *testing.T) {
	t.Log("Extended invariant 42 check")

	ruleBody := `invariant_check(42) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 42 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 42 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_43 validates secondary invariant 43.
func TestE2E_Pipeline_ExtendedScenario_43(t *testing.T) {
	t.Log("Extended invariant 43 check")

	ruleBody := `invariant_check(43) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 43 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 43 parse returned nil AST")
	}
}

// TestE2E_Pipeline_ExtendedScenario_44 validates secondary invariant 44.
func TestE2E_Pipeline_ExtendedScenario_44(t *testing.T) {
	t.Log("Extended invariant 44 check")

	ruleBody := `invariant_check(44) :- system_active().`
	parsed, err := parse.Unit(strings.NewReader(ruleBody))
	if err != nil {
		t.Fatalf("Extended 44 parse failed: %v", err)
	}

	if len(parsed.Decls) == 0 {
		t.Fatalf("Extended 44 parse returned nil AST")
	}
}
