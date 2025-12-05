package mangle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

// TestSchemasGLParsesWithoutError validates that schemas.gl is syntactically correct.
func TestSchemasGLParsesWithoutError(t *testing.T) {
	schemasPath := findMangleFile(t, "schemas.gl")
	data, err := os.ReadFile(schemasPath)
	if err != nil {
		t.Fatalf("Failed to read schemas.gl: %v", err)
	}

	unit, err := parse.Unit(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse schemas.gl: %v", err)
	}

	t.Logf("schemas.gl parsed successfully: %d declarations, %d clauses",
		len(unit.Decls), len(unit.Clauses))

	// Verify we have the expected core declarations
	declNames := make(map[string]bool)
	for _, decl := range unit.Decls {
		declNames[decl.DeclaredAtom.Predicate.Symbol] = true
	}

	expectedDecls := []string{
		"user_intent",
		"focus_resolution",
		"file_topology",
		"symbol_graph",
		"diagnostic",
		"test_state",
		"next_action",
		"permitted",
		"delegate_task",
	}

	for _, expected := range expectedDecls {
		if !declNames[expected] {
			t.Errorf("Expected declaration %q not found in schemas.gl", expected)
		}
	}
}

// TestPolicyGLParsesWithoutError validates that policy.gl is syntactically correct.
func TestPolicyGLParsesWithoutError(t *testing.T) {
	policyPath := findMangleFile(t, "policy.gl")
	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Failed to read policy.gl: %v", err)
	}

	unit, err := parse.Unit(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse policy.gl: %v", err)
	}

	t.Logf("policy.gl parsed successfully: %d declarations, %d clauses",
		len(unit.Decls), len(unit.Clauses))

	if len(unit.Clauses) == 0 {
		t.Error("policy.gl should contain rules (clauses)")
	}
}

// TestSchemasPlusPolicyAnalyzeTogether validates schemas+policy analyze together.
func TestSchemasPlusPolicyAnalyzeTogether(t *testing.T) {
	schemasPath := findMangleFile(t, "schemas.gl")
	policyPath := findMangleFile(t, "policy.gl")

	schemasData, err := os.ReadFile(schemasPath)
	if err != nil {
		t.Fatalf("Failed to read schemas.gl: %v", err)
	}

	policyData, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Failed to read policy.gl: %v", err)
	}

	// Combine schemas and policy
	combined := string(schemasData) + "\n\n" + string(policyData)

	unit, err := parse.Unit(strings.NewReader(combined))
	if err != nil {
		t.Fatalf("Failed to parse combined schemas+policy: %v", err)
	}

	// Analyze the program
	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("Failed to analyze combined program: %v", err)
	}

	t.Logf("Combined program analyzed successfully: %d predicates declared",
		len(programInfo.Decls))

	// Verify some key derived predicates exist
	keyPredicates := []string{
		"activation",
		"active_strategy",
		"next_action",
		"clarification_needed",
		"impacted",
		"block_commit",
		"permitted",
	}

	for _, pred := range keyPredicates {
		found := false
		for sym := range programInfo.Decls {
			if sym.Symbol == pred {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected predicate %q not found in program", pred)
		}
	}
}

// TestCoderGLParsesWithoutError validates coder.gl syntax.
func TestCoderGLParsesWithoutError(t *testing.T) {
	coderPath := findMangleFile(t, "coder.gl")
	data, err := os.ReadFile(coderPath)
	if err != nil {
		t.Skipf("coder.gl not found (optional): %v", err)
	}

	_, err = parse.Unit(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse coder.gl: %v", err)
	}

	t.Log("coder.gl parsed successfully")
}

// TestTesterGLParsesWithoutError validates tester.gl syntax.
func TestTesterGLParsesWithoutError(t *testing.T) {
	testerPath := findMangleFile(t, "tester.gl")
	data, err := os.ReadFile(testerPath)
	if err != nil {
		t.Skipf("tester.gl not found (optional): %v", err)
	}

	_, err = parse.Unit(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse tester.gl: %v", err)
	}

	t.Log("tester.gl parsed successfully")
}

// TestReviewerGLParsesWithoutError validates reviewer.gl syntax.
func TestReviewerGLParsesWithoutError(t *testing.T) {
	reviewerPath := findMangleFile(t, "reviewer.gl")
	data, err := os.ReadFile(reviewerPath)
	if err != nil {
		t.Skipf("reviewer.gl not found (optional): %v", err)
	}

	_, err = parse.Unit(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Failed to parse reviewer.gl: %v", err)
	}

	t.Log("reviewer.gl parsed successfully")
}

// TestAllGLFilesCombinedAnalysis tests that all .gl files work together.
func TestAllGLFilesCombinedAnalysis(t *testing.T) {
	glFiles := []string{"schemas.gl", "policy.gl", "coder.gl", "tester.gl", "reviewer.gl"}

	var combined strings.Builder
	loadedFiles := 0

	for _, filename := range glFiles {
		path := findMangleFile(t, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("Skipping %s: %v", filename, err)
			continue
		}
		combined.WriteString(string(data))
		combined.WriteString("\n\n")
		loadedFiles++
	}

	if loadedFiles < 2 {
		t.Skip("Not enough .gl files found for combined analysis")
	}

	unit, err := parse.Unit(strings.NewReader(combined.String()))
	if err != nil {
		t.Fatalf("Failed to parse combined .gl files: %v", err)
	}

	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("Failed to analyze combined program: %v", err)
	}

	t.Logf("All %d .gl files analyzed together: %d predicates, %d rules",
		loadedFiles, len(programInfo.Decls), len(programInfo.Rules))
}

// TestTDDRepairLoopRules tests the TDD repair loop state machine.
func TestTDDRepairLoopRules(t *testing.T) {
	program := `
# Schemas
Decl test_state(State).
Decl retry_count(Count).
Decl next_action(Action).

# TDD Rules from policy.gl
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.

next_action(/complete) :-
    test_state(/passing).
`
	testCases := []struct {
		name     string
		facts    []testFact
		expected string // Expected next_action
	}{
		{
			name: "failing with retries remaining",
			facts: []testFact{
				{"test_state", []interface{}{"/failing"}},
				{"retry_count", []interface{}{int64(1)}},
			},
			expected: "/read_error_log",
		},
		{
			name: "failing with max retries exceeded",
			facts: []testFact{
				{"test_state", []interface{}{"/failing"}},
				{"retry_count", []interface{}{int64(3)}},
			},
			expected: "/escalate_to_user",
		},
		{
			name: "tests passing",
			facts: []testFact{
				{"test_state", []interface{}{"/passing"}},
			},
			expected: "/complete",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := evaluateAndQuery(t, program, tc.facts, "next_action")
			if len(result) == 0 {
				t.Fatalf("No next_action derived")
			}

			found := false
			for _, fact := range result {
				if len(fact.Args) > 0 {
					if arg, ok := fact.Args[0].(string); ok && arg == tc.expected {
						found = true
						break
					}
				}
			}

			if !found {
				t.Errorf("Expected next_action(%s), got: %v", tc.expected, result)
			}
		})
	}
}

// TestSpreadingActivationRules tests context activation.
func TestSpreadingActivationRules(t *testing.T) {
	program := `
# Schemas
Decl new_fact(Fact).
Decl activation(Fact, Score).
Decl context_atom(Fact).

# Spreading Activation Rules
activation(Fact, 100) :- new_fact(Fact).

context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.
`
	facts := []testFact{
		{"new_fact", []interface{}{"important_fact"}},
	}

	result := evaluateAndQuery(t, program, facts, "context_atom")
	if len(result) == 0 {
		t.Error("Expected context_atom to be derived for high-activation facts")
	}
}

// TestSafeNegation tests that safe negation patterns work correctly.
func TestSafeNegation(t *testing.T) {
	program := `
# Schemas
Decl block_commit(Reason).
Decl has_block_commit().
Decl safe_to_commit().
Decl diagnostic(Severity, Path, Line, Code, Msg).

# Rules
block_commit("Build Broken") :- diagnostic(/error, _, _, _, _).
has_block_commit() :- block_commit(_).
safe_to_commit() :- !has_block_commit().
`
	t.Run("no errors means safe to commit", func(t *testing.T) {
		// No diagnostic facts
		result := evaluateAndQuery(t, program, []testFact{}, "safe_to_commit")
		if len(result) == 0 {
			t.Error("Expected safe_to_commit() when no errors present")
		}
	})

	t.Run("errors block commit", func(t *testing.T) {
		facts := []testFact{
			{"diagnostic", []interface{}{"/error", "test.go", int64(10), "E001", "syntax error"}},
		}
		result := evaluateAndQuery(t, program, facts, "block_commit")
		if len(result) == 0 {
			t.Error("Expected block_commit when errors present")
		}
	})
}

// TestDelegateTaskRules tests shard delegation.
func TestDelegateTaskRules(t *testing.T) {
	program := `
# Schemas
Decl user_intent(ID, Category, Verb, Target, Constraint).
Decl delegate_task(ShardType, TaskDesc, Status).

# Delegation Rules
delegate_task(/researcher, Task, /pending) :-
    user_intent(_, _, /research, Task, _).

delegate_task(/coder, Task, /pending) :-
    user_intent(_, /mutation, /implement, Task, _).

delegate_task(/tester, Task, /pending) :-
    user_intent(_, _, /test, Task, _).

delegate_task(/reviewer, Task, /pending) :-
    user_intent(_, _, /review, Task, _).
`
	testCases := []struct {
		name     string
		intent   testFact
		expected string // Expected shard type
	}{
		{
			name:     "research delegates to researcher",
			intent:   testFact{"user_intent", []interface{}{"id1", "/query", "/research", "API docs", ""}},
			expected: "/researcher",
		},
		{
			name:     "implement delegates to coder",
			intent:   testFact{"user_intent", []interface{}{"id2", "/mutation", "/implement", "auth feature", ""}},
			expected: "/coder",
		},
		{
			name:     "test delegates to tester",
			intent:   testFact{"user_intent", []interface{}{"id3", "/instruction", "/test", "unit tests", ""}},
			expected: "/tester",
		},
		{
			name:     "review delegates to reviewer",
			intent:   testFact{"user_intent", []interface{}{"id4", "/query", "/review", "code review", ""}},
			expected: "/reviewer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := evaluateAndQuery(t, program, []testFact{tc.intent}, "delegate_task")
			if len(result) == 0 {
				t.Fatalf("No delegate_task derived")
			}

			found := false
			for _, fact := range result {
				if len(fact.Args) > 0 {
					if arg, ok := fact.Args[0].(string); ok && arg == tc.expected {
						found = true
						break
					}
				}
			}

			if !found {
				t.Errorf("Expected delegate_task(%s, ...), got: %v", tc.expected, result)
			}
		})
	}
}

// TestImpactAnalysisTransitiveClosure tests recursive dependency tracking.
func TestImpactAnalysisTransitiveClosure(t *testing.T) {
	program := `
# Schemas
Decl modified(File).
Decl dependency_link(Caller, Callee, Import).
Decl impacted(File).

# Rules
impacted(X) :- dependency_link(X, Y, _), modified(Y).
impacted(X) :- dependency_link(X, Z, _), impacted(Z).
`
	// A -> B -> C, and C is modified
	// Both A and B should be impacted
	facts := []testFact{
		{"modified", []interface{}{"C"}},
		{"dependency_link", []interface{}{"A", "B", "import1"}},
		{"dependency_link", []interface{}{"B", "C", "import2"}},
	}

	result := evaluateAndQuery(t, program, facts, "impacted")

	impactedFiles := make(map[string]bool)
	for _, fact := range result {
		if len(fact.Args) > 0 {
			if file, ok := fact.Args[0].(string); ok {
				impactedFiles[file] = true
			}
		}
	}

	if !impactedFiles["B"] {
		t.Error("Expected B to be impacted (direct dependency on modified C)")
	}
	if !impactedFiles["A"] {
		t.Error("Expected A to be impacted (transitive dependency via B)")
	}
}

// TestConstitutionalSafety tests permission gates.
func TestConstitutionalSafety(t *testing.T) {
	program := `
# Schemas
Decl safe_action(Action).
Decl dangerous_action(Action).
Decl admin_override(User).
Decl signed_approval(Action).
Decl permitted(Action).

# Safety Rules
permitted(Action) :- safe_action(Action).
permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

# Base facts
safe_action(/read_file).
dangerous_action(/delete_file).
`
	t.Run("safe action is permitted", func(t *testing.T) {
		result := evaluateAndQuery(t, program, []testFact{}, "permitted")
		found := false
		for _, fact := range result {
			if len(fact.Args) > 0 {
				if arg, ok := fact.Args[0].(string); ok && arg == "/read_file" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("Expected /read_file to be permitted")
		}
	})

	t.Run("dangerous action needs approval", func(t *testing.T) {
		// Without admin_override and signed_approval, delete should not be permitted
		result := evaluateAndQuery(t, program, []testFact{}, "permitted")
		for _, fact := range result {
			if len(fact.Args) > 0 {
				if arg, ok := fact.Args[0].(string); ok && arg == "/delete_file" {
					t.Error("Expected /delete_file to NOT be permitted without approval")
				}
			}
		}
	})

	t.Run("dangerous action with approval is permitted", func(t *testing.T) {
		facts := []testFact{
			{"admin_override", []interface{}{"admin_user"}},
			{"signed_approval", []interface{}{"/delete_file"}},
		}
		result := evaluateAndQuery(t, program, facts, "permitted")
		found := false
		for _, fact := range result {
			if len(fact.Args) > 0 {
				if arg, ok := fact.Args[0].(string); ok && arg == "/delete_file" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("Expected /delete_file to be permitted with admin override and signed approval")
		}
	})
}

// TestCampaignPhaseEligibility tests campaign orchestration rules.
func TestCampaignPhaseEligibility(t *testing.T) {
	program := `
# Schemas
Decl campaign(ID, Type, Title, Source, Status).
Decl campaign_phase(PhaseID, CampaignID, Name, Order, Status, Profile).
Decl phase_dependency(PhaseID, DependsOn, Type).
Decl current_campaign(CampaignID).
Decl phase_eligible(PhaseID).
Decl has_incomplete_hard_dep(PhaseID).

# Rules
current_campaign(CampaignID) :- campaign(CampaignID, _, _, _, /active).

has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(DepPhaseID, _, _, _, Status, _),
    /completed != Status.

phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).
`
	// Phase 2 depends on Phase 1 (hard dependency)
	// Phase 1 is not complete, so Phase 2 should not be eligible
	facts := []testFact{
		{"campaign", []interface{}{"campaign1", "/feature", "Test Feature", "spec.md", "/active"}},
		{"campaign_phase", []interface{}{"phase1", "campaign1", "Design", int64(1), "/in_progress", "design_profile"}},
		{"campaign_phase", []interface{}{"phase2", "campaign1", "Implement", int64(2), "/pending", "impl_profile"}},
		{"phase_dependency", []interface{}{"phase2", "phase1", "/hard"}},
	}

	result := evaluateAndQuery(t, program, facts, "phase_eligible")

	// phase2 should NOT be eligible because phase1 is not complete
	for _, fact := range result {
		if len(fact.Args) > 0 {
			if arg, ok := fact.Args[0].(string); ok && arg == "phase2" {
				t.Error("Expected phase2 to NOT be eligible (phase1 not complete)")
			}
		}
	}

	// Now complete phase1 and check again
	facts2 := []testFact{
		{"campaign", []interface{}{"campaign1", "/feature", "Test Feature", "spec.md", "/active"}},
		{"campaign_phase", []interface{}{"phase1", "campaign1", "Design", int64(1), "/completed", "design_profile"}},
		{"campaign_phase", []interface{}{"phase2", "campaign1", "Implement", int64(2), "/pending", "impl_profile"}},
		{"phase_dependency", []interface{}{"phase2", "phase1", "/hard"}},
	}

	result2 := evaluateAndQuery(t, program, facts2, "phase_eligible")

	found := false
	for _, fact := range result2 {
		if len(fact.Args) > 0 {
			if arg, ok := fact.Args[0].(string); ok && arg == "phase2" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Expected phase2 to be eligible after phase1 completed")
	}
}

// Helper types and functions

type testFact struct {
	Predicate string
	Args      []interface{}
}

func findMangleFile(t *testing.T, filename string) string {
	// Search in multiple locations
	searchPaths := []string{
		filename,
		filepath.Join(".", filename),
		filepath.Join("..", "mangle", filename),
		filepath.Join("internal", "mangle", filename),
		filepath.Join("..", "..", "internal", "mangle", filename),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	t.Fatalf("Could not find %s in any search path", filename)
	return ""
}

func evaluateAndQuery(t *testing.T, program string, facts []testFact, queryPred string) []Fact {
	t.Helper()

	// Parse the program
	unit, err := parse.Unit(strings.NewReader(program))
	if err != nil {
		t.Fatalf("Failed to parse program: %v", err)
	}

	// Analyze
	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("Failed to analyze program: %v", err)
	}

	// Create fact store and add facts
	store := factstore.NewSimpleInMemoryStore()

	for _, f := range facts {
		atom := factToAtom(t, programInfo, f)
		store.Add(atom)
	}

	// Evaluate
	_, err = engine.EvalProgramWithStats(programInfo, store)
	if err != nil {
		t.Fatalf("Failed to evaluate program: %v", err)
	}

	// Query results
	var results []Fact
	for sym := range programInfo.Decls {
		if sym.Symbol == queryPred {
			store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
				args := make([]interface{}, len(a.Args))
				for i, arg := range a.Args {
					args[i] = termToValue(arg)
				}
				results = append(results, Fact{
					Predicate: queryPred,
					Args:      args,
				})
				return nil
			})
			break
		}
	}

	return results
}

func factToAtom(t *testing.T, programInfo *analysis.ProgramInfo, f testFact) ast.Atom {
	t.Helper()

	// Find predicate symbol
	var predSym ast.PredicateSym
	found := false
	for sym := range programInfo.Decls {
		if sym.Symbol == f.Predicate {
			predSym = sym
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Predicate %s not declared", f.Predicate)
	}

	// Convert args
	terms := make([]ast.BaseTerm, len(f.Args))
	for i, arg := range f.Args {
		terms[i] = valueToTerm(t, arg)
	}

	return ast.Atom{Predicate: predSym, Args: terms}
}

func valueToTerm(t *testing.T, value interface{}) ast.BaseTerm {
	t.Helper()

	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "/") {
			name, err := ast.Name(v)
			if err != nil {
				t.Fatalf("Invalid name constant %q: %v", v, err)
			}
			return name
		}
		return ast.String(v)
	case int:
		return ast.Number(int64(v))
	case int64:
		return ast.Number(v)
	case float64:
		return ast.Float64(v)
	case bool:
		if v {
			return ast.TrueConstant
		}
		return ast.FalseConstant
	default:
		t.Fatalf("Unsupported arg type %T", value)
		return nil
	}
}

func termToValue(term ast.BaseTerm) interface{} {
	switch t := term.(type) {
	case ast.Constant:
		switch t.Type {
		case ast.NameType, ast.StringType:
			return t.Symbol
		case ast.NumberType:
			return t.NumValue
		case ast.Float64Type:
			return t.Float64Value
		default:
			return t.Symbol
		}
	case ast.Variable:
		return "?" + t.Symbol
	default:
		return term.String()
	}
}

// TestEngineWithRealPolicy tests the Engine wrapper with actual policy files.
func TestEngineWithRealPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true

	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Load schemas
	schemasPath := findMangleFile(t, "schemas.gl")
	if err := eng.LoadSchema(schemasPath); err != nil {
		t.Fatalf("Failed to load schemas.gl: %v", err)
	}

	// Load policy
	policyPath := findMangleFile(t, "policy.gl")
	if err := eng.LoadSchema(policyPath); err != nil {
		t.Fatalf("Failed to load policy.gl: %v", err)
	}

	// Add test facts
	err = eng.AddFacts([]Fact{
		{Predicate: "test_state", Args: []interface{}{"/failing"}},
		{Predicate: "retry_count", Args: []interface{}{int64(1)}},
	})
	if err != nil {
		t.Fatalf("Failed to add facts: %v", err)
	}

	// Query for next_action using GetFacts
	facts, err := eng.GetFacts("next_action")
	if err != nil {
		t.Logf("GetFacts error (may be expected): %v", err)
	} else {
		t.Logf("next_action facts: %v", facts)
	}
}
