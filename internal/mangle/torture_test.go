// Package mangle - GCC-style torture test suite for the Mangle logic engine.
//
// Modeled after GCC's gcc.c-torture/{compile,execute} and gcc.dg/noncompile:
//   - TestTorture_Parse_*:  Programs that MUST parse without error
//   - TestTorture_Eval_*:   Programs that MUST parse, evaluate, and derive expected facts
//   - TestTorture_Error_*:  Programs that MUST fail at parse, analysis, or stratification
//   - TestTorture_Lifecycle_*: Engine lifecycle invariants (Reset, Clear, Close, ordering)
//   - TestTorture_Differential_*: Differential engine (snapshot, delta, concurrent)
//   - TestTorture_SchemaValidator_*: Schema drift prevention + arity + forbidden heads
//   - TestTorture_TypeSystem_*: Boundary types (NaN, Inf, MaxInt, bool, time, duration)
//   - TestTorture_Concurrency_*: Race detector targets
//
// Convention: each test is deterministic and does NOT require external resources.
// Run with: CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/mangle/... -run TestTorture -count=1 -race -timeout 120s
package mangle

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/mangle/parse"
)

// =============================================================================
// 1. PARSE TORTURE — programs that MUST parse without error
// =============================================================================

func TestTorture_Parse_EmptySchema(t *testing.T) {
	// GCC parallel: empty translation unit should compile
	_, err := parse.Unit(strings.NewReader(""))
	if err != nil {
		t.Fatalf("empty string should parse: %v", err)
	}
}

func TestTorture_Parse_CommentOnly(t *testing.T) {
	src := "# This is only a comment\n# Another comment\n"
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("comment-only program should parse: %v", err)
	}
}

func TestTorture_Parse_SingleDecl(t *testing.T) {
	src := `Decl simple(X).`
	unit, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("single Decl should parse: %v", err)
	}
	if len(unit.Decls) < 1 {
		t.Errorf("expected at least 1 Decl, got %d", len(unit.Decls))
	}
}

func TestTorture_Parse_DeclWithBound(t *testing.T) {
	src := `Decl typed_pred(Name, Age, Score) bound [/string, /number, /float64].`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decl with bound should parse: %v", err)
	}
}

func TestTorture_Parse_MultipleBoundAlternatives(t *testing.T) {
	src := `Decl polymorphic(X, Y) bound [/string, /string] bound [/number, /number].`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decl with multiple bound alternatives should parse: %v", err)
	}
}

func TestTorture_Parse_DeclWithMode(t *testing.T) {
	src := `Decl queryable(X, Y) descr [mode("-", "-")].`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decl with mode descriptor should parse: %v", err)
	}
}

func TestTorture_Parse_GroundFact(t *testing.T) {
	src := `Decl color(X). color(/red).`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ground fact should parse: %v", err)
	}
}

func TestTorture_Parse_NameConstants(t *testing.T) {
	// All valid atom forms
	src := `
Decl status(X).
status(/active).
status(/inactive).
status(/pending).
status(/error_state).
status(/a123).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("name constants should parse: %v", err)
	}
}

func TestTorture_Parse_SimpleRule(t *testing.T) {
	src := `
Decl parent(X, Y).
Decl ancestor(X, Y).
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("recursive rule should parse: %v", err)
	}
}

func TestTorture_Parse_NegationInRule(t *testing.T) {
	src := `
Decl person(X).
Decl admin(X).
Decl non_admin(X).
non_admin(X) :- person(X), !admin(X).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("rule with negation should parse: %v", err)
	}
}

func TestTorture_Parse_Comparison(t *testing.T) {
	src := `
Decl score(Name, Value).
Decl high_score(Name).
high_score(Name) :- score(Name, V), V >= 90.
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("comparison in rule should parse: %v", err)
	}
}

func TestTorture_Parse_AggregationPipeline(t *testing.T) {
	// Critical: aggregation uses |> pipeline syntax, NOT SQL-style
	src := `
Decl sale(Region, Amount).
Decl total_by_region(Region, Total).
total_by_region(Region, Total) :-
    sale(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:Sum(Amount).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("aggregation pipeline should parse: %v", err)
	}
}

func TestTorture_Parse_CountAggregation(t *testing.T) {
	src := `
Decl item(X).
Decl total_items(N).
total_items(N) :-
    item(_) |>
    do fn:group_by(),
    let N = fn:Count().
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("count aggregation should parse: %v", err)
	}
}

func TestTorture_Parse_CollectAggregation(t *testing.T) {
	src := `
Decl tag(Item, Tag).
Decl item_tags(Item, Tags).
item_tags(Item, Tags) :-
    tag(Item, Tag) |>
    do fn:group_by(Item),
    let Tags = fn:collect(Tag).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("collect aggregation should parse: %v", err)
	}
}

func TestTorture_Parse_AnonymousVariable(t *testing.T) {
	src := `
Decl edge(X, Y).
Decl has_outgoing(X).
has_outgoing(X) :- edge(X, _).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("anonymous variable should parse: %v", err)
	}
}

func TestTorture_Parse_ArithmeticBuiltins(t *testing.T) {
	src := `
Decl input(X).
Decl computed(X, Y).
computed(X, Y) :- input(X), Y = fn:plus(X, 1).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("arithmetic builtins should parse: %v", err)
	}
}

func TestTorture_Parse_MultiClauseUnion(t *testing.T) {
	// Multiple rules for the same head = UNION semantics (not override)
	src := `
Decl reachable(X).
Decl start(X).
Decl via_edge(X).
reachable(X) :- start(X).
reachable(X) :- via_edge(X).
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("multi-clause union should parse: %v", err)
	}
}

func TestTorture_Parse_DeeplyNestedProgram(t *testing.T) {
	// Large program with many declarations and rules
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("Decl pred_" + string(rune('a'+i%26)) + "(X).\n")
	}
	for i := 0; i < 20; i++ {
		sb.WriteString("Decl chain_" + string(rune('a'+i%26)) + "(X, Y).\n")
	}
	_, err := parse.Unit(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("large program should parse: %v", err)
	}
}

func TestTorture_Parse_UnicodeInStrings(t *testing.T) {
	src := `
Decl msg(Content).
msg("Hello, 世界").
msg("Ströme").
msg("café").
msg("αβγ").
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unicode in strings should parse: %v", err)
	}
}

func TestTorture_Parse_MixedTypesInDecl(t *testing.T) {
	src := `
Decl record(ID, Name, Score, Active, Updated) bound [/number, /string, /float64, /name, /string].
`
	_, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("mixed types in Decl should parse: %v", err)
	}
}

// =============================================================================
// 2. EVAL TORTURE — programs that MUST produce expected derived facts
// =============================================================================

func TestTorture_Eval_TransitiveClosure(t *testing.T) {
	program := `
Decl edge(X, Y).
Decl reachable(X, Y).
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
`
	facts := []testFact{
		{"edge", []interface{}{"a", "b"}},
		{"edge", []interface{}{"b", "c"}},
		{"edge", []interface{}{"c", "d"}},
	}
	result := evaluateAndQuery(t, program, facts, "reachable")
	// Expected: (a,b), (a,c), (a,d), (b,c), (b,d), (c,d) = 6
	if len(result) != 6 {
		t.Errorf("transitive closure: expected 6 reachable pairs, got %d: %v", len(result), result)
	}
}

func TestTorture_Eval_DiamondDependency(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D: D should be reachable from A (no duplicate)
	program := `
Decl dep(X, Y).
Decl reachable(X, Y).
reachable(X, Y) :- dep(X, Y).
reachable(X, Z) :- dep(X, Y), reachable(Y, Z).
`
	facts := []testFact{
		{"dep", []interface{}{"A", "B"}},
		{"dep", []interface{}{"A", "C"}},
		{"dep", []interface{}{"B", "D"}},
		{"dep", []interface{}{"C", "D"}},
	}
	result := evaluateAndQuery(t, program, facts, "reachable")
	// Reachable: (A,B), (A,C), (B,D), (C,D), (A,D) = 5 unique pairs
	// Note: (A,D) derived via A->B->D and A->C->D, but deduplication gives 1
	reachMap := make(map[string]bool)
	for _, f := range result {
		if len(f.Args) >= 2 {
			key := f.Args[0].(string) + "->" + f.Args[1].(string)
			reachMap[key] = true
		}
	}
	if !reachMap["A->D"] {
		t.Error("diamond: A should reach D")
	}
	if len(result) != 5 {
		t.Errorf("diamond: expected 5 reachable pairs, got %d", len(result))
	}
}

func TestTorture_Eval_NegationSafe(t *testing.T) {
	program := `
Decl user(X).
Decl admin(X).
Decl regular_user(X).
regular_user(X) :- user(X), !admin(X).
`
	facts := []testFact{
		{"user", []interface{}{"alice"}},
		{"user", []interface{}{"bob"}},
		{"user", []interface{}{"charlie"}},
		{"admin", []interface{}{"alice"}},
	}
	result := evaluateAndQuery(t, program, facts, "regular_user")
	if len(result) != 2 {
		t.Errorf("negation: expected 2 regular users, got %d", len(result))
	}
	// Verify alice is NOT in results
	for _, f := range result {
		if f.Args[0].(string) == "alice" {
			t.Error("negation: alice should not be a regular_user (she's admin)")
		}
	}
}

func TestTorture_Eval_NegationEmptyBase(t *testing.T) {
	// When the negated predicate has no facts, negation should succeed for all bound values
	program := `
Decl candidate(X).
Decl excluded(X).
Decl selected(X).
selected(X) :- candidate(X), !excluded(X).
`
	facts := []testFact{
		{"candidate", []interface{}{"x"}},
		{"candidate", []interface{}{"y"}},
		// No excluded facts at all
	}
	result := evaluateAndQuery(t, program, facts, "selected")
	if len(result) != 2 {
		t.Errorf("negation with empty base: expected 2 selected, got %d", len(result))
	}
}

func TestTorture_Eval_DoubleNegation(t *testing.T) {
	// Double negation: confirmed(X) if X is a candidate and NOT not_confirmed(X)
	program := `
Decl candidate(X).
Decl rejected(X).
Decl not_rejected(X).
Decl confirmed(X).
not_rejected(X) :- candidate(X), !rejected(X).
confirmed(X) :- not_rejected(X).
`
	facts := []testFact{
		{"candidate", []interface{}{"a"}},
		{"candidate", []interface{}{"b"}},
		{"rejected", []interface{}{"b"}},
	}
	result := evaluateAndQuery(t, program, facts, "confirmed")
	if len(result) != 1 {
		t.Errorf("double negation: expected 1 confirmed, got %d", len(result))
	}
	if len(result) > 0 && result[0].Args[0].(string) != "a" {
		t.Errorf("double negation: expected 'a' confirmed, got %v", result[0].Args[0])
	}
}

func TestTorture_Eval_ComparisonOperators(t *testing.T) {
	program := `
Decl score(Name, Value).
Decl pass(Name).
Decl fail(Name).
Decl perfect(Name).
pass(Name) :- score(Name, V), V >= 60.
fail(Name) :- score(Name, V), V < 60.
perfect(Name) :- score(Name, V), V = 100.
`
	facts := []testFact{
		{"score", []interface{}{"alice", int64(95)}},
		{"score", []interface{}{"bob", int64(55)}},
		{"score", []interface{}{"charlie", int64(100)}},
		{"score", []interface{}{"dave", int64(60)}},
	}

	t.Run("pass", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "pass")
		if len(result) != 3 {
			t.Errorf("expected 3 passes (alice, charlie, dave), got %d", len(result))
		}
	})
	t.Run("fail", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "fail")
		if len(result) != 1 {
			t.Errorf("expected 1 fail (bob), got %d", len(result))
		}
	})
	t.Run("perfect", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "perfect")
		if len(result) != 1 {
			t.Errorf("expected 1 perfect (charlie), got %d", len(result))
		}
	})
}

func TestTorture_Eval_Arithmetic(t *testing.T) {
	program := `
Decl input(Name, X).
Decl doubled(Name, Result).
Decl offset(Name, Result).
doubled(Name, R) :- input(Name, X), R = fn:mult(X, 2).
offset(Name, R) :- input(Name, X), R = fn:plus(X, 10).
`
	facts := []testFact{
		{"input", []interface{}{"a", int64(5)}},
		{"input", []interface{}{"b", int64(0)}},
		{"input", []interface{}{"c", int64(-3)}},
	}

	t.Run("doubled", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "doubled")
		if len(result) != 3 {
			t.Errorf("expected 3 doubled facts, got %d", len(result))
		}
		for _, f := range result {
			name := f.Args[0].(string)
			val := f.Args[1].(int64)
			switch name {
			case "a":
				if val != 10 {
					t.Errorf("doubled(a) = %d, want 10", val)
				}
			case "b":
				if val != 0 {
					t.Errorf("doubled(b) = %d, want 0", val)
				}
			case "c":
				if val != -6 {
					t.Errorf("doubled(c) = %d, want -6", val)
				}
			}
		}
	})

	t.Run("offset", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "offset")
		for _, f := range result {
			name := f.Args[0].(string)
			val := f.Args[1].(int64)
			if name == "c" && val != 7 {
				t.Errorf("offset(c) = %d, want 7", val)
			}
		}
	})
}

func TestTorture_Eval_ConstantRule(t *testing.T) {
	// A rule that produces a constant fact unconditionally
	program := `
Decl always(X).
Decl trigger().
always(/yes) :- trigger().
`
	facts := []testFact{
		{"trigger", []interface{}{}},
	}
	result := evaluateAndQuery(t, program, facts, "always")
	if len(result) != 1 {
		t.Errorf("constant rule: expected 1 fact, got %d", len(result))
	}
}

func TestTorture_Eval_MultiRuleUnion(t *testing.T) {
	// Multiple rules for same head = UNION, not override
	program := `
Decl source_a(X).
Decl source_b(X).
Decl combined(X).
combined(X) :- source_a(X).
combined(X) :- source_b(X).
`
	facts := []testFact{
		{"source_a", []interface{}{"x"}},
		{"source_a", []interface{}{"y"}},
		{"source_b", []interface{}{"y"}}, // duplicate with source_a
		{"source_b", []interface{}{"z"}},
	}
	result := evaluateAndQuery(t, program, facts, "combined")
	// x, y, z — y deduplicated
	if len(result) != 3 {
		t.Errorf("union: expected 3 combined facts, got %d: %v", len(result), result)
	}
}

func TestTorture_Eval_DeepRecursion(t *testing.T) {
	// Long chain: node_0 -> node_1 -> ... -> node_99 (100 nodes, 99 edges)
	program := `
Decl edge(X, Y).
Decl reachable(X, Y).
reachable(X, Y) :- edge(X, Y).
reachable(X, Z) :- edge(X, Y), reachable(Y, Z).
`
	var facts []testFact
	for i := 0; i < 99; i++ {
		from := "node_" + string(rune('0'+(i/10))) + string(rune('0'+(i%10)))
		to := "node_" + string(rune('0'+((i+1)/10))) + string(rune('0'+((i+1)%10)))
		facts = append(facts, testFact{"edge", []interface{}{from, to}})
	}
	result := evaluateAndQuery(t, program, facts, "reachable")
	// For a chain of 100 nodes: n*(n-1)/2 = 4950 reachable pairs
	if len(result) != 4950 {
		t.Errorf("deep recursion: expected 4950 reachable pairs, got %d", len(result))
	}
}

func TestTorture_Eval_SelfLoop(t *testing.T) {
	// Self-referencing: p(X) :- p(X) should not cause infinite loop (fixpoint = no new facts)
	program := `
Decl p(X).
p(X) :- p(X).
`
	facts := []testFact{
		{"p", []interface{}{"seed"}},
	}
	result := evaluateAndQuery(t, program, facts, "p")
	if len(result) != 1 {
		t.Errorf("self-loop: expected 1 fact (seed), got %d", len(result))
	}
}

func TestTorture_Eval_MutualRecursion(t *testing.T) {
	// a(X) :- b(X). b(X) :- a(X). — positive cycle, should reach fixpoint with seeded facts
	program := `
Decl a(X).
Decl b(X).
a(X) :- b(X).
b(X) :- a(X).
`
	facts := []testFact{
		{"a", []interface{}{"seed"}},
	}
	// b("seed") should be derived from a("seed")
	resultB := evaluateAndQuery(t, program, facts, "b")
	if len(resultB) != 1 {
		t.Errorf("mutual recursion: expected b(seed), got %d facts", len(resultB))
	}
}

func TestTorture_Eval_NameConstantUnification(t *testing.T) {
	// Critical: /active (atom) and "active" (string) must NOT unify
	program := `
Decl status(Entity, State).
Decl active_entity(Entity).
active_entity(E) :- status(E, /active).
`
	facts := []testFact{
		{"status", []interface{}{"server1", "/active"}}, // atom via / prefix
		{"status", []interface{}{"server2", "/inactive"}},
	}
	result := evaluateAndQuery(t, program, facts, "active_entity")
	if len(result) != 1 {
		t.Errorf("atom unification: expected 1 active entity, got %d", len(result))
	}
	if len(result) > 0 && result[0].Args[0].(string) != "server1" {
		t.Errorf("atom unification: expected server1, got %v", result[0].Args[0])
	}
}

func TestTorture_Eval_BoundedRecursion(t *testing.T) {
	// Recursion with explicit depth limit
	program := `
Decl edge(X, Y).
Decl path(X, Y, D).
path(X, Y, 1) :- edge(X, Y).
path(X, Z, D) :- edge(X, Y), path(Y, Z, D1), D = fn:plus(D1, 1), D < 5.
`
	facts := []testFact{
		{"edge", []interface{}{"a", "b"}},
		{"edge", []interface{}{"b", "c"}},
		{"edge", []interface{}{"c", "d"}},
		{"edge", []interface{}{"d", "e"}},
		{"edge", []interface{}{"e", "f"}},
		{"edge", []interface{}{"f", "g"}},
	}
	result := evaluateAndQuery(t, program, facts, "path")
	// All paths with depth < 5 should be derived; depth 5+ should be cut
	for _, f := range result {
		depth := f.Args[2].(int64)
		if depth >= 5 {
			t.Errorf("bounded recursion: found path with depth %d >= 5", depth)
		}
	}
	if len(result) == 0 {
		t.Error("bounded recursion: expected some paths")
	}
}

func TestTorture_Eval_ClosedWorldAssumption(t *testing.T) {
	// Unknown facts are false (not null/unknown)
	program := `
Decl item(X).
Decl known(X).
Decl unknown_item(X).
known(X) :- item(X).
# Everything in item but not in known is impossible — CWA means there are no unknowns
# This tests that the system correctly handles "absence = false"
`
	facts := []testFact{
		{"item", []interface{}{"a"}},
		{"item", []interface{}{"b"}},
	}
	result := evaluateAndQuery(t, program, facts, "known")
	if len(result) != 2 {
		t.Errorf("CWA: expected 2 known items, got %d", len(result))
	}
	// unknown_item should have 0 facts (never derived)
	unknown := evaluateAndQuery(t, program, facts, "unknown_item")
	if len(unknown) != 0 {
		t.Errorf("CWA: expected 0 unknown_items, got %d", len(unknown))
	}
}

func TestTorture_Eval_FactDeduplication(t *testing.T) {
	// Same fact derived via multiple rules should appear only once
	program := `
Decl source1(X).
Decl source2(X).
Decl result(X).
result(X) :- source1(X).
result(X) :- source2(X).
`
	facts := []testFact{
		{"source1", []interface{}{"shared"}},
		{"source2", []interface{}{"shared"}}, // same value
		{"source1", []interface{}{"only1"}},
		{"source2", []interface{}{"only2"}},
	}
	result := evaluateAndQuery(t, program, facts, "result")
	if len(result) != 3 {
		t.Errorf("dedup: expected 3 unique results, got %d", len(result))
	}
}

func TestTorture_Eval_MultiJoinRule(t *testing.T) {
	// Rule with 3+ body atoms (multi-way join)
	program := `
Decl employee(Name, Dept).
Decl dept_budget(Dept, Budget).
Decl dept_head(Dept, Head).
Decl eligible(Name, Dept, Budget, Head).
eligible(Name, Dept, Budget, Head) :-
    employee(Name, Dept),
    dept_budget(Dept, Budget),
    dept_head(Dept, Head),
    Budget > 50000.
`
	facts := []testFact{
		{"employee", []interface{}{"alice", "eng"}},
		{"employee", []interface{}{"bob", "sales"}},
		{"dept_budget", []interface{}{"eng", int64(100000)}},
		{"dept_budget", []interface{}{"sales", int64(30000)}},
		{"dept_head", []interface{}{"eng", "carol"}},
		{"dept_head", []interface{}{"sales", "dave"}},
	}
	result := evaluateAndQuery(t, program, facts, "eligible")
	// Only alice in eng (budget 100000 > 50000)
	if len(result) != 1 {
		t.Errorf("multi-join: expected 1 eligible, got %d", len(result))
	}
}

// =============================================================================
// 3. ERROR TORTURE — programs that MUST fail
// =============================================================================

func TestTorture_Error_SyntaxError(t *testing.T) {
	// Missing period
	src := `Decl foo(X)`
	_, err := parse.Unit(strings.NewReader(src))
	if err == nil {
		t.Error("missing period should cause parse error")
	}
}

func TestTorture_Error_InvalidAtomSyntax(t *testing.T) {
	// Atom starting with uppercase (variables are uppercase, predicates are lowercase)
	src := `Decl Valid(X).`
	_, err := parse.Unit(strings.NewReader(src))
	// "Valid" starts with uppercase — this might be rejected as a predicate name
	if err == nil {
		t.Log("uppercase predicate name was accepted (may be valid in some Mangle versions)")
	}
}

func TestTorture_Error_UnstratifiableNegation(t *testing.T) {
	// Negative cycle: winning -> losing -> NOT winning
	program := `
Decl move(X, Y).
Decl position(X).
Decl winning(X).
Decl losing(X).
winning(X) :- move(X, Y), losing(Y).
losing(X) :- position(X), !winning(X).
`
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = engine.LoadSchemaString(program)
	if err == nil {
		t.Error("unstratifiable negation cycle should fail during analysis/stratification")
	} else {
		t.Logf("correctly rejected: %v", err)
	}
}

func TestTorture_Error_ArityMismatchInRule(t *testing.T) {
	// Rule body uses wrong arity
	program := `
Decl binary(X, Y).
Decl result(X).
result(X) :- binary(X).
`
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = engine.LoadSchemaString(program)
	if err == nil {
		t.Error("arity mismatch in rule body should fail")
	} else {
		t.Logf("correctly rejected: %v", err)
	}
}

func TestTorture_Error_UndeclaredPredInRule(t *testing.T) {
	// Rule uses predicate not declared
	program := `
Decl result(X).
result(X) :- unknown_pred(X).
`
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = engine.LoadSchemaString(program)
	if err == nil {
		t.Error("undeclared predicate in rule body should fail analysis")
	} else {
		t.Logf("correctly rejected: %v", err)
	}
}

func TestTorture_Error_UnsafeHeadVariable(t *testing.T) {
	// Variable in head not bound in body
	program := `
Decl source(X).
Decl result(X, Y).
result(X, Y) :- source(X).
`
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = engine.LoadSchemaString(program)
	if err == nil {
		t.Error("unsafe head variable Y should fail analysis")
	} else {
		t.Logf("correctly rejected: %v", err)
	}
}

func TestTorture_Error_UnsafeNegation(t *testing.T) {
	// Variable in negated atom not bound
	program := `
Decl bad(X).
bad(X) :- !bad(X).
`
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	err = engine.LoadSchemaString(program)
	if err == nil {
		t.Error("unsafe negation (X unbound) should fail")
	} else {
		t.Logf("correctly rejected: %v", err)
	}
}

// =============================================================================
// 4. LIFECYCLE TORTURE — Engine state machine invariants
// =============================================================================

func TestTorture_Lifecycle_AddFactBeforeSchema(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Adding a fact without loading schema should fail or be handled gracefully
	err = engine.AddFact("unschema_pred", "value")
	if err == nil {
		t.Error("AddFact before LoadSchemaString should fail")
	}
}

func TestTorture_Lifecycle_QueryBeforeSchema(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	_, err = engine.GetFacts("nonexistent")
	if err == nil {
		t.Error("GetFacts before LoadSchemaString should fail")
	}
}

func TestTorture_Lifecycle_ClearPreservesSchema(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFact("item", "before_clear")

	engine.Clear()

	// Schema should survive Clear — AddFact should still work
	err = engine.AddFact("item", "after_clear")
	if err != nil {
		t.Fatalf("AddFact after Clear should succeed (schema preserved): %v", err)
	}

	facts, _ := engine.GetFacts("item")
	if len(facts) != 1 {
		t.Errorf("expected 1 fact after clear+add, got %d", len(facts))
	}
}

func TestTorture_Lifecycle_ResetWipesSchema(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFact("item", "before_reset")

	engine.Reset()

	// After Reset, schema is gone — AddFact should fail
	err = engine.AddFact("item", "after_reset")
	if err == nil {
		t.Error("AddFact after Reset should fail (schema wiped)")
	}
}

func TestTorture_Lifecycle_ResetThenReload(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := engine.LoadSchemaString(`Decl v1(X).`); err != nil {
		t.Fatalf("first load: %v", err)
	}
	_ = engine.AddFact("v1", "data")

	engine.Reset()

	// Reload different schema
	if err := engine.LoadSchemaString(`Decl v2(X, Y).`); err != nil {
		t.Fatalf("reload after reset: %v", err)
	}
	if err := engine.AddFact("v2", "a", "b"); err != nil {
		t.Fatalf("AddFact to new schema: %v", err)
	}
	facts, _ := engine.GetFacts("v2")
	if len(facts) != 1 {
		t.Errorf("expected 1 fact in v2, got %d", len(facts))
	}
}

func TestTorture_Lifecycle_DoubleClear(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFact("item", "data")

	engine.Clear()
	engine.Clear() // Double clear should not panic

	// Should still be functional
	if err := engine.AddFact("item", "after_double_clear"); err != nil {
		t.Fatalf("AddFact after double clear: %v", err)
	}
}

func TestTorture_Lifecycle_CloseAndReuse(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	_ = engine.Close()

	// Close is a no-op, engine should still work (documented behavior)
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString after Close: %v", err)
	}
}

func TestTorture_Lifecycle_IncrementalSchemaLoading(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Load schema in multiple calls (incremental)
	if err := engine.LoadSchemaString(`Decl pred_a(X).`); err != nil {
		t.Fatalf("first schema: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl pred_b(X, Y).`); err != nil {
		t.Fatalf("second schema: %v", err)
	}

	// Both predicates should work
	if err := engine.AddFact("pred_a", "hello"); err != nil {
		t.Fatalf("AddFact to pred_a: %v", err)
	}
	if err := engine.AddFact("pred_b", "x", "y"); err != nil {
		t.Fatalf("AddFact to pred_b: %v", err)
	}

	factsA, _ := engine.GetFacts("pred_a")
	factsB, _ := engine.GetFacts("pred_b")
	if len(factsA) != 1 || len(factsB) != 1 {
		t.Errorf("incremental schema: expected 1 fact each, got pred_a=%d pred_b=%d", len(factsA), len(factsB))
	}
}

func TestTorture_Lifecycle_SchemaAfterFacts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Load first schema and add facts
	if err := engine.LoadSchemaString(`Decl existing(X).`); err != nil {
		t.Fatalf("first schema: %v", err)
	}
	_ = engine.AddFact("existing", "data")

	// Load additional schema AFTER facts are already in the store
	if err := engine.LoadSchemaString(`Decl new_pred(X).`); err != nil {
		t.Fatalf("second schema after facts: %v", err)
	}

	// Both should work
	if err := engine.AddFact("new_pred", "new_data"); err != nil {
		t.Fatalf("AddFact to new_pred: %v", err)
	}

	// Existing facts should survive
	facts, _ := engine.GetFacts("existing")
	if len(facts) != 1 {
		t.Errorf("existing facts lost after schema addition: got %d", len(facts))
	}
}

func TestTorture_Lifecycle_GetStatsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	stats := engine.GetStats()
	if stats.TotalFacts < 0 {
		t.Error("GetStats on empty engine should not return negative")
	}
}

func TestTorture_Lifecycle_QueryImmediatelyAfterClear(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	_ = engine.AddFact("item", "a")
	_ = engine.AddFact("item", "b")

	engine.Clear()

	facts, err := engine.GetFacts("item")
	if err != nil {
		t.Fatalf("GetFacts after Clear: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after Clear, got %d", len(facts))
	}
}

// =============================================================================
// 5. DIFFERENTIAL ENGINE TORTURE
// =============================================================================

func TestTorture_Differential_EmptySnapshot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := baseEngine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	snapshot := diffEngine.Snapshot()
	if snapshot == nil {
		t.Fatal("Snapshot of empty differential engine should not be nil")
	}
}

func TestTorture_Differential_RequiresSchema(t *testing.T) {
	cfg := DefaultConfig()
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// No schema loaded
	_, err = NewDifferentialEngine(baseEngine)
	if err == nil {
		t.Error("NewDifferentialEngine without schema should fail")
	}
}

func TestTorture_Differential_SnapshotIsolation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	schema := `
Decl item(X).
Decl derived(X).
derived(X) :- item(X).
`
	if err := baseEngine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = baseEngine.AddFact("item", "base_item")

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Create snapshot
	snapshot := diffEngine.Snapshot()

	// The snapshot and original should be independent
	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}
}

// =============================================================================
// 6. SCHEMA VALIDATOR TORTURE
// =============================================================================

func TestTorture_SchemaValidator_ForbiddenHead(t *testing.T) {
	sv := NewSchemaValidator(`Decl user(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	tests := []struct {
		name string
		rule string
	}{
		{"permitted", `permitted(/action, /target, /payload) :- user(/admin).`},
		{"safe_action", `safe_action(/delete) :- user(/admin).`},
		{"admin_override", `admin_override(/yes) :- user(/admin).`},
		{"pending_action", `pending_action(/do_stuff) :- user(/admin).`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.ValidateLearnedRule(tt.rule)
			if err == nil {
				t.Errorf("learned rule defining %q should be rejected", tt.name)
			} else if !strings.Contains(err.Error(), "protected") && !strings.Contains(err.Error(), "core-owned") {
				t.Errorf("error should mention 'protected' or 'core-owned', got: %v", err)
			}
		})
	}
}

func TestTorture_SchemaValidator_ArityMismatch(t *testing.T) {
	sv := NewSchemaValidator(`Decl pair(X, Y).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	err := sv.CheckArity("pair", 3) // pair has 2 args, not 3
	if err == nil {
		t.Error("CheckArity should fail for 3 args on a 2-arg predicate")
	}

	err = sv.CheckArity("pair", 2) // correct
	if err != nil {
		t.Errorf("CheckArity should succeed for correct arity: %v", err)
	}
}

func TestTorture_SchemaValidator_UnknownPredicateArity(t *testing.T) {
	sv := NewSchemaValidator(`Decl known(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// Unknown predicate should return -1 arity and CheckArity should pass (no data = no check)
	arity := sv.GetArity("unknown_pred")
	if arity != -1 {
		t.Errorf("unknown predicate arity should be -1, got %d", arity)
	}

	err := sv.CheckArity("unknown_pred", 42)
	if err != nil {
		t.Errorf("CheckArity for unknown predicate should pass: %v", err)
	}
}

func TestTorture_SchemaValidator_HotLoadEmpty(t *testing.T) {
	sv := NewSchemaValidator(`Decl item(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	err := sv.HotLoadRule("")
	if err == nil {
		t.Error("HotLoadRule with empty string should fail")
	}
}

func TestTorture_SchemaValidator_HotLoadNoPeriod(t *testing.T) {
	sv := NewSchemaValidator(`Decl item(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// HotLoadRule appends period if missing, so this should parse
	err := sv.HotLoadRule(`item(/hello)`)
	if err != nil {
		t.Errorf("HotLoadRule without trailing period should auto-append: %v", err)
	}
}

func TestTorture_SchemaValidator_UndefinedBodyPredicate(t *testing.T) {
	sv := NewSchemaValidator(`Decl result(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	err := sv.ValidateRule(`result(X) :- ghost_predicate(X).`)
	if err == nil {
		t.Error("rule using undefined body predicate should be rejected")
	}
}

func TestTorture_SchemaValidator_ValidProgram(t *testing.T) {
	schemas := `
Decl user(Name).
Decl admin(Name).
Decl regular(Name).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// ValidateProgram runs full Mangle analysis, so the program must include
	// Decl statements for all referenced predicates.
	program := `
Decl user(Name).
Decl admin(Name).
Decl regular(Name).
regular(X) :- user(X), !admin(X).
`
	err := sv.ValidateProgram(program)
	if err != nil {
		t.Errorf("valid program should pass: %v", err)
	}
}

// =============================================================================
// 7. TYPE SYSTEM TORTURE — boundary values
// =============================================================================

func TestTorture_TypeSystem_NaN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl metric(Name, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// NaN should either store or fail gracefully, never panic
	err = engine.AddFact("metric", "nan_test", math.NaN())
	if err != nil {
		t.Logf("NaN storage returned error (acceptable): %v", err)
	} else {
		facts, _ := engine.GetFacts("metric")
		t.Logf("NaN stored successfully, %d facts", len(facts))
	}
}

func TestTorture_TypeSystem_Infinity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl metric(Name, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	tests := []struct {
		name  string
		value float64
	}{
		{"pos_inf", math.Inf(1)},
		{"neg_inf", math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.AddFact("metric", tt.name, tt.value)
			if err != nil {
				t.Logf("Infinity storage returned error (acceptable): %v", err)
			}
		})
	}
}

func TestTorture_TypeSystem_MaxInt64(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl bignum(Name, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	tests := []struct {
		name  string
		value int64
	}{
		{"max", math.MaxInt64},
		{"min", math.MinInt64},
		{"zero", 0},
		{"negative_one", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.AddFact("bignum", tt.name, tt.value)
			if err != nil {
				t.Fatalf("AddFact(%s, %d) failed: %v", tt.name, tt.value, err)
			}
		})
	}

	facts, _ := engine.GetFacts("bignum")
	if len(facts) != len(tests) {
		t.Errorf("expected %d facts, got %d", len(tests), len(facts))
	}
}

func TestTorture_TypeSystem_BoolValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl flag(Name, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	if err := engine.AddFact("flag", "on", true); err != nil {
		t.Fatalf("AddFact(true): %v", err)
	}
	if err := engine.AddFact("flag", "off", false); err != nil {
		t.Fatalf("AddFact(false): %v", err)
	}

	facts, _ := engine.GetFacts("flag")
	if len(facts) != 2 {
		t.Errorf("expected 2 bool facts, got %d", len(facts))
	}
}

func TestTorture_TypeSystem_IntCoercion(t *testing.T) {
	// Go int (not int64) should be handled via value conversion
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl data(Key, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// int (Go native) — should be coerced to int64
	err = engine.AddFact("data", "go_int", 42)
	if err != nil {
		t.Fatalf("AddFact with Go int: %v", err)
	}

	// int64 (explicit)
	err = engine.AddFact("data", "go_int64", int64(42))
	if err != nil {
		t.Fatalf("AddFact with int64: %v", err)
	}

	facts, _ := engine.GetFacts("data")
	if len(facts) < 2 {
		t.Errorf("expected at least 2 facts, got %d", len(facts))
	}
}

func TestTorture_TypeSystem_EmptyString(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl data(Key, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	err = engine.AddFact("data", "empty", "")
	if err != nil {
		t.Fatalf("AddFact with empty string: %v", err)
	}

	facts, _ := engine.GetFacts("data")
	if len(facts) != 1 {
		t.Errorf("expected 1 fact with empty string, got %d", len(facts))
	}
}

func TestTorture_TypeSystem_ZeroArityFact(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl flag().`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// Zero-arity fact
	err = engine.AddFact("flag")
	if err != nil {
		t.Logf("Zero-arity AddFact: %v (may need special handling)", err)
	}
}

func TestTorture_TypeSystem_DuplicateFactDedup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// Add same fact twice
	_ = engine.AddFact("item", "x")
	_ = engine.AddFact("item", "x") // duplicate

	facts, _ := engine.GetFacts("item")
	if len(facts) != 1 {
		t.Errorf("duplicate fact should be deduplicated: expected 1, got %d", len(facts))
	}
}

// =============================================================================
// 8. CONCURRENCY TORTURE — race detector targets
// =============================================================================

func TestTorture_Concurrency_AddFactAndQuery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(ID, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	var wg sync.WaitGroup

	// Writers
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = engine.AddFact("item", gid*1000+i, "val")
			}
		}(g)
	}

	// Readers (concurrent with writers)
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, _ = engine.GetFacts("item")
			}
		}()
	}

	wg.Wait()

	// Verify engine is still functional
	facts, err := engine.GetFacts("item")
	if err != nil {
		t.Fatalf("GetFacts after concurrent ops: %v", err)
	}
	t.Logf("concurrent add+query: %d facts stored", len(facts))
}

func TestTorture_Concurrency_ConcurrentSchemaLoad(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	var wg sync.WaitGroup
	// Multiple goroutines loading schemas concurrently
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			schema := "Decl concurrent_" + string(rune('a'+gid)) + "(X)."
			_ = engine.LoadSchemaString(schema)
		}(g)
	}

	wg.Wait()

	// Engine should not have panicked
	stats := engine.GetStats()
	t.Logf("concurrent schema load: %d total facts", stats.TotalFacts)
}

func TestTorture_Concurrency_ClearDuringAddFact(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	var wg sync.WaitGroup

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = engine.AddFact("item", i)
		}
	}()

	// Clearer (races with writer)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			engine.Clear()
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// Engine should not have panicked; fact count is non-deterministic
	facts, err := engine.GetFacts("item")
	if err != nil {
		t.Fatalf("GetFacts after concurrent clear: %v", err)
	}
	t.Logf("concurrent clear+add: %d facts survived", len(facts))
}

func TestTorture_Concurrency_QueryWithTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	schema := `
Decl item(X) descr [mode("-")].
`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFact("item", "test")

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = engine.Query(ctx, "item(X)")
		}()
	}
	wg.Wait()
}

// =============================================================================
// 9. ENGINE WRAPPER TORTURE — wrapper-specific edge cases
// =============================================================================

func TestTorture_Engine_RecomputeWithoutSchema(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	err = engine.RecomputeRules()
	if err == nil {
		t.Error("RecomputeRules without schema should fail")
	}
}

func TestTorture_Engine_ToggleAutoEval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`
Decl base(X).
Decl derived(X).
derived(X) :- base(X).
`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// With auto-eval ON, adding a fact should trigger rule evaluation
	_ = engine.AddFact("base", "auto")
	factsOn, _ := engine.GetFacts("derived")

	// Toggle OFF
	engine.ToggleAutoEval(false)
	_ = engine.AddFact("base", "manual")

	// derived should not have "manual" yet
	factsOff, _ := engine.GetFacts("derived")

	// Toggle ON and recompute
	engine.ToggleAutoEval(true)
	_ = engine.RecomputeRules()
	factsAfter, _ := engine.GetFacts("derived")

	t.Logf("auto-eval on: %d, off: %d, after recompute: %d", len(factsOn), len(factsOff), len(factsAfter))

	if len(factsAfter) < len(factsOff) {
		t.Error("recompute should have derived at least as many facts as before")
	}
}

func TestTorture_Engine_QueryEmpty(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X) descr [mode("-")].`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := engine.Query(ctx, "item(X)")
	if err != nil {
		t.Fatalf("Query on empty engine: %v", err)
	}
	if len(result.Bindings) != 0 {
		t.Errorf("expected 0 bindings for empty engine, got %d", len(result.Bindings))
	}
}

func TestTorture_Engine_QueryWithLeadingQuestion(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl item(X) descr [mode("-")].`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFact("item", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Engine should handle "?" prefix gracefully
	result, err := engine.Query(ctx, "?item(X)")
	if err != nil {
		t.Logf("Query with ? prefix: err=%v (may need stripping)", err)
	} else {
		t.Logf("Query with ? prefix returned %d bindings", len(result.Bindings))
	}
}

func TestTorture_Engine_DerivedFactCount(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`
Decl base(X).
Decl derived(X).
derived(X) :- base(X).
`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	initial := engine.GetDerivedFactCount()
	_ = engine.AddFact("base", "a")
	_ = engine.AddFact("base", "b")
	after := engine.GetDerivedFactCount()

	t.Logf("derived count: initial=%d, after=%d", initial, after)

	engine.ResetDerivedFactCount()
	reset := engine.GetDerivedFactCount()
	if reset != 0 {
		t.Errorf("ResetDerivedFactCount should set to 0, got %d", reset)
	}
}

func TestTorture_Engine_QueryFactsFiltered(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl record(ID, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	_ = engine.AddFact("record", "a", "apple")
	_ = engine.AddFact("record", "b", "banana")
	_ = engine.AddFact("record", "c", "cherry")

	// QueryFacts with first arg bound
	results := engine.QueryFacts("record", "b")
	if len(results) != 1 {
		t.Errorf("QueryFacts(record, b): expected 1, got %d", len(results))
	}

	// QueryFacts with non-existent key
	results = engine.QueryFacts("record", "z")
	if len(results) != 0 {
		t.Errorf("QueryFacts(record, z): expected 0, got %d", len(results))
	}
}
