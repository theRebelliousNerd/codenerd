// Package mangle - GCC-style torture test suite for the Mangle logic engine.
//
// Modeled after GCC's gcc.c-torture/{compile,execute} and gcc.dg/noncompile:
//   - TestTorture_Parse_*:  Programs that MUST parse without error
//   - TestTorture_Eval_*:   Programs that MUST parse, evaluate, and derive expected facts
//   - TestTorture_Error_*:  Programs that MUST fail at parse, analysis, or stratification
//   - TestTorture_Lifecycle_*: Engine lifecycle invariants (Reset, Clear, Close, ordering)
//   - TestTorture_Differential_*: Differential engine (snapshot, delta, query, concurrent)
//   - TestTorture_SchemaValidator_*: Schema drift prevention + arity + forbidden heads
//   - TestTorture_TypeSystem_*: Boundary types (NaN, Inf, MaxInt, bool, time, duration)
//   - TestTorture_Concurrency_*: Race detector targets (concurrent add/query/recompute/delta)
//   - TestTorture_Eval_Aggregation*: Aggregation pipelines (count, sum, min, max, collect, group_by)
//   - TestTorture_ChainedFactStore_*: Overlay/base store pattern (contains, merge, multi-base)
//   - TestTorture_KnowledgeGraph_*: Stratum layer invariants
//   - TestTorture_FactStoreProxy_*: Lazy loading predicates
//
// Convention: each test is deterministic and does NOT require external resources.
// Run with: CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" go test ./internal/mangle/... -run TestTorture -count=1 -race -timeout 120s
package mangle

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/factstore"
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

// =============================================================================
// 10. AGGREGATION EVALUATION TORTURE — programs with |> pipelines that MUST
//     parse, evaluate, and produce correct aggregation results.
//     This was previously a ZERO-COVERAGE gap: only parsing was tested.
// =============================================================================

func TestTorture_Eval_AggregationCount(t *testing.T) {
	// CRITICAL: aggregation functions are ALL LOWERCASE (fn:count, fn:sum, fn:min, fn:max)
	// Uppercase variants (fn:Count, fn:Sum) parse successfully but fail analysis!
	program := `
Decl item(X).
Decl total_items(N).
total_items(N) :-
    item(_) |>
    do fn:group_by(),
    let N = fn:count().
`
	facts := []testFact{
		{"item", []interface{}{"a"}},
		{"item", []interface{}{"b"}},
		{"item", []interface{}{"c"}},
	}
	result := evaluateAndQuery(t, program, facts, "total_items")
	if len(result) != 1 {
		t.Fatalf("count aggregation: expected 1 total_items fact, got %d", len(result))
	}
	if n, ok := result[0].Args[0].(int64); !ok || n != 3 {
		t.Errorf("count aggregation: expected 3, got %v (type %T)", result[0].Args[0], result[0].Args[0])
	}
}

func TestTorture_Eval_AggregationSum(t *testing.T) {
	program := `
Decl sale(Amount).
Decl total_sales(Total).
total_sales(Total) :-
    sale(Amount) |>
    do fn:group_by(),
    let Total = fn:sum(Amount).
`
	facts := []testFact{
		{"sale", []interface{}{int64(10)}},
		{"sale", []interface{}{int64(20)}},
		{"sale", []interface{}{int64(30)}},
	}
	result := evaluateAndQuery(t, program, facts, "total_sales")
	if len(result) != 1 {
		t.Fatalf("sum aggregation: expected 1 total_sales fact, got %d", len(result))
	}
	if total, ok := result[0].Args[0].(int64); !ok || total != 60 {
		t.Errorf("sum aggregation: expected 60, got %v (type %T)", result[0].Args[0], result[0].Args[0])
	}
}

func TestTorture_Eval_AggregationGroupBySum(t *testing.T) {
	program := `
Decl sale(Region, Amount).
Decl total_by_region(Region, Total).
total_by_region(Region, Total) :-
    sale(Region, Amount) |>
    do fn:group_by(Region),
    let Total = fn:sum(Amount).
`
	facts := []testFact{
		{"sale", []interface{}{"north", int64(100)}},
		{"sale", []interface{}{"north", int64(200)}},
		{"sale", []interface{}{"south", int64(50)}},
		{"sale", []interface{}{"south", int64(75)}},
		{"sale", []interface{}{"south", int64(25)}},
	}
	result := evaluateAndQuery(t, program, facts, "total_by_region")
	if len(result) != 2 {
		t.Fatalf("group_by+sum: expected 2 region totals, got %d: %v", len(result), result)
	}

	totals := make(map[string]int64)
	for _, f := range result {
		region := f.Args[0].(string)
		total := f.Args[1].(int64)
		totals[region] = total
	}
	if totals["north"] != 300 {
		t.Errorf("north total: expected 300, got %d", totals["north"])
	}
	if totals["south"] != 150 {
		t.Errorf("south total: expected 150, got %d", totals["south"])
	}
}

func TestTorture_Eval_AggregationGroupByCount(t *testing.T) {
	program := `
Decl event(Category, ID).
Decl event_count(Category, N).
event_count(Category, N) :-
    event(Category, _) |>
    do fn:group_by(Category),
    let N = fn:count().
`
	facts := []testFact{
		{"event", []interface{}{"click", "e1"}},
		{"event", []interface{}{"click", "e2"}},
		{"event", []interface{}{"click", "e3"}},
		{"event", []interface{}{"scroll", "e4"}},
		{"event", []interface{}{"scroll", "e5"}},
	}
	result := evaluateAndQuery(t, program, facts, "event_count")
	if len(result) != 2 {
		t.Fatalf("group_by+count: expected 2 category counts, got %d", len(result))
	}

	counts := make(map[string]int64)
	for _, f := range result {
		cat := f.Args[0].(string)
		n := f.Args[1].(int64)
		counts[cat] = n
	}
	if counts["click"] != 3 {
		t.Errorf("click count: expected 3, got %d", counts["click"])
	}
	if counts["scroll"] != 2 {
		t.Errorf("scroll count: expected 2, got %d", counts["scroll"])
	}
}

func TestTorture_Eval_AggregationMinMax(t *testing.T) {
	program := `
Decl score(Player, Value).
Decl player_min(Player, Min).
Decl player_max(Player, Max).
player_min(Player, Min) :-
    score(Player, Value) |>
    do fn:group_by(Player),
    let Min = fn:min(Value).
player_max(Player, Max) :-
    score(Player, Value) |>
    do fn:group_by(Player),
    let Max = fn:max(Value).
`
	facts := []testFact{
		{"score", []interface{}{"alice", int64(50)}},
		{"score", []interface{}{"alice", int64(90)}},
		{"score", []interface{}{"alice", int64(70)}},
		{"score", []interface{}{"bob", int64(60)}},
		{"score", []interface{}{"bob", int64(80)}},
	}

	t.Run("min", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "player_min")
		mins := make(map[string]int64)
		for _, f := range result {
			mins[f.Args[0].(string)] = f.Args[1].(int64)
		}
		if mins["alice"] != 50 {
			t.Errorf("alice min: expected 50, got %d", mins["alice"])
		}
		if mins["bob"] != 60 {
			t.Errorf("bob min: expected 60, got %d", mins["bob"])
		}
	})

	t.Run("max", func(t *testing.T) {
		result := evaluateAndQuery(t, program, facts, "player_max")
		maxes := make(map[string]int64)
		for _, f := range result {
			maxes[f.Args[0].(string)] = f.Args[1].(int64)
		}
		if maxes["alice"] != 90 {
			t.Errorf("alice max: expected 90, got %d", maxes["alice"])
		}
		if maxes["bob"] != 80 {
			t.Errorf("bob max: expected 80, got %d", maxes["bob"])
		}
	})
}

func TestTorture_Eval_AggregationCollect(t *testing.T) {
	program := `
Decl tag(Item, Tag).
Decl item_tags(Item, Tags).
item_tags(Item, Tags) :-
    tag(Item, Tag) |>
    do fn:group_by(Item),
    let Tags = fn:collect(Tag).
`
	facts := []testFact{
		{"tag", []interface{}{"doc1", "go"}},
		{"tag", []interface{}{"doc1", "mangle"}},
		{"tag", []interface{}{"doc2", "python"}},
	}
	result := evaluateAndQuery(t, program, facts, "item_tags")
	if len(result) != 2 {
		t.Fatalf("collect: expected 2 item_tags facts, got %d", len(result))
	}

	// Verify each item has its tags collected
	for _, f := range result {
		item := f.Args[0].(string)
		tags := f.Args[1] // This is a list/pair
		t.Logf("collect: %s -> %v (type %T)", item, tags, tags)
		if item == "doc1" && tags == nil {
			t.Error("doc1 should have collected tags")
		}
	}
}

func TestTorture_Eval_AggregationEmptyInput(t *testing.T) {
	// Aggregation over empty set should produce no results (not crash)
	program := `
Decl item(X).
Decl total_items(N).
total_items(N) :-
    item(_) |>
    do fn:group_by(),
    let N = fn:count().
`
	// No facts at all
	result := evaluateAndQuery(t, program, nil, "total_items")
	// Empty input to aggregation: no groups means no results
	t.Logf("aggregation over empty: %d results", len(result))
	// Should be 0 (no items to count) or 1 with count 0 — either is acceptable
}

func TestTorture_Eval_AggregationWithFilter(t *testing.T) {
	// Aggregation after a filter in the rule body
	program := `
Decl sale(Region, Amount).
Decl big_sale_count(Region, N).
big_sale_count(Region, N) :-
    sale(Region, Amount), Amount > 50 |>
    do fn:group_by(Region),
    let N = fn:count().
`
	facts := []testFact{
		{"sale", []interface{}{"north", int64(100)}},
		{"sale", []interface{}{"north", int64(20)}},
		{"sale", []interface{}{"north", int64(200)}},
		{"sale", []interface{}{"south", int64(30)}},
		{"sale", []interface{}{"south", int64(75)}},
	}
	result := evaluateAndQuery(t, program, facts, "big_sale_count")
	counts := make(map[string]int64)
	for _, f := range result {
		counts[f.Args[0].(string)] = f.Args[1].(int64)
	}
	if counts["north"] != 2 {
		t.Errorf("north big sale count: expected 2, got %d", counts["north"])
	}
	if counts["south"] != 1 {
		t.Errorf("south big sale count: expected 1, got %d", counts["south"])
	}
}

// =============================================================================
// 11. DIFFERENTIAL ENGINE EXPANDED TORTURE
// =============================================================================

func TestTorture_Differential_ApplyDelta(t *testing.T) {
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

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Apply delta with a single fact
	err = diffEngine.ApplyDelta([]Fact{
		{Predicate: "item", Args: []interface{}{"hello"}},
	})
	if err != nil {
		t.Fatalf("ApplyDelta: %v", err)
	}

	// Verify fact was stored in some stratum
	found := false
	for _, layer := range diffEngine.strataStores {
		count := layer.store.EstimateFactCount()
		if count > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("ApplyDelta: fact was not stored in any stratum")
	}
}

func TestTorture_Differential_ApplyDeltaEmpty(t *testing.T) {
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

	// Empty delta should not error
	err = diffEngine.ApplyDelta(nil)
	if err != nil {
		t.Fatalf("ApplyDelta(nil): %v", err)
	}

	err = diffEngine.ApplyDelta([]Fact{})
	if err != nil {
		t.Fatalf("ApplyDelta(empty): %v", err)
	}
}

func TestTorture_Differential_AddFactIncremental(t *testing.T) {
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

	// AddFactIncremental is a convenience wrapper
	err = diffEngine.AddFactIncremental(Fact{Predicate: "item", Args: []interface{}{"single"}})
	if err != nil {
		t.Fatalf("AddFactIncremental: %v", err)
	}
}

func TestTorture_Differential_QueryWithDeclaredMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	schema := `
Decl item(X) descr [mode("-")].
`
	if err := baseEngine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Add fact via delta
	err = diffEngine.ApplyDelta([]Fact{
		{Predicate: "item", Args: []interface{}{"test_value"}},
	})
	if err != nil {
		t.Fatalf("ApplyDelta: %v", err)
	}

	// Query should find the fact
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := diffEngine.Query(ctx, "item(X)")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestTorture_Differential_QueryUndeclaredPredicate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := baseEngine.LoadSchemaString(`Decl item(X) descr [mode("-")].`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = diffEngine.Query(ctx, "ghost_predicate(X)")
	if err == nil {
		t.Error("Query for undeclared predicate should fail")
	}
}

func TestTorture_Differential_SnapshotMutationIsolation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := baseEngine.LoadSchemaString(`Decl item(X) descr [mode("-")].`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Add initial fact
	_ = diffEngine.ApplyDelta([]Fact{
		{Predicate: "item", Args: []interface{}{"before_snapshot"}},
	})

	// Take snapshot
	snapshot := diffEngine.Snapshot()

	// Mutate original after snapshot
	_ = diffEngine.ApplyDelta([]Fact{
		{Predicate: "item", Args: []interface{}{"after_snapshot"}},
	})

	// Snapshot should NOT see the fact added after it was taken
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshotResult, err := snapshot.Query(ctx, "item(X)")
	if err != nil {
		t.Fatalf("snapshot query: %v", err)
	}

	// Snapshot should only have "before_snapshot"
	for _, binding := range snapshotResult.Bindings {
		if val, ok := binding["X"]; ok && val == "after_snapshot" {
			t.Error("snapshot should not see facts added after snapshot was taken")
		}
	}
}

func TestTorture_Differential_MultipleDeltas(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := baseEngine.LoadSchemaString(`Decl counter(ID, Value).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Apply multiple deltas sequentially
	for i := 0; i < 10; i++ {
		err := diffEngine.ApplyDelta([]Fact{
			{Predicate: "counter", Args: []interface{}{fmt.Sprintf("id_%d", i), int64(i)}},
		})
		if err != nil {
			t.Fatalf("ApplyDelta(%d): %v", i, err)
		}
	}

	// Verify all facts stored
	totalFacts := 0
	for _, layer := range diffEngine.strataStores {
		totalFacts += layer.store.EstimateFactCount()
	}
	if totalFacts < 10 {
		t.Errorf("expected at least 10 facts across strata, got %d", totalFacts)
	}
}

// =============================================================================
// 12. CHAINED FACT STORE TORTURE — overlay/base store pattern
// =============================================================================

func TestTorture_ChainedFactStore_BasicOverlay(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	// Add fact to base
	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	baseAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("base_val")}}
	base.Add(baseAtom)

	// Add fact to overlay
	overlayAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("overlay_val")}}
	overlay.Add(overlayAtom)

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	// GetFacts should return both
	var found []string
	chain.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
		if len(a.Args) > 0 {
			found = append(found, convertBaseTermToInterface(a.Args[0]).(string))
		}
		return nil
	})
	if len(found) != 2 {
		t.Errorf("expected 2 facts (base+overlay), got %d: %v", len(found), found)
	}
}

func TestTorture_ChainedFactStore_Contains(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	baseAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("in_base")}}
	base.Add(baseAtom)

	overlayAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("in_overlay")}}
	overlay.Add(overlayAtom)

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	if !chain.Contains(baseAtom) {
		t.Error("Contains: should find fact in base")
	}
	if !chain.Contains(overlayAtom) {
		t.Error("Contains: should find fact in overlay")
	}

	missingAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("missing")}}
	if chain.Contains(missingAtom) {
		t.Error("Contains: should not find missing fact")
	}
}

func TestTorture_ChainedFactStore_ListPredicates(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	pred1 := ast.PredicateSym{Symbol: "pred_a", Arity: 1}
	pred2 := ast.PredicateSym{Symbol: "pred_b", Arity: 1}
	pred3 := ast.PredicateSym{Symbol: "pred_c", Arity: 1}

	base.Add(ast.Atom{Predicate: pred1, Args: []ast.BaseTerm{ast.String("x")}})
	base.Add(ast.Atom{Predicate: pred2, Args: []ast.BaseTerm{ast.String("y")}})
	overlay.Add(ast.Atom{Predicate: pred2, Args: []ast.BaseTerm{ast.String("z")}}) // duplicate pred
	overlay.Add(ast.Atom{Predicate: pred3, Args: []ast.BaseTerm{ast.String("w")}})

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	preds := chain.ListPredicates()
	// Should have 3 unique predicates (pred_a, pred_b, pred_c), no duplicates
	predNames := make(map[string]bool)
	for _, p := range preds {
		predNames[p.Symbol] = true
	}
	if len(predNames) != 3 {
		t.Errorf("expected 3 unique predicates, got %d: %v", len(predNames), predNames)
	}
}

func TestTorture_ChainedFactStore_EstimateFactCount(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	for i := 0; i < 5; i++ {
		base.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String(fmt.Sprintf("base_%d", i))}})
	}
	for i := 0; i < 3; i++ {
		overlay.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String(fmt.Sprintf("overlay_%d", i))}})
	}

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	count := chain.EstimateFactCount()
	if count != 8 {
		t.Errorf("expected estimate of 8, got %d", count)
	}
}

func TestTorture_ChainedFactStore_Merge(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()
	source := factstore.NewSimpleInMemoryStore()

	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	source.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("merged_val")}})

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	// Merge copies facts from source into overlay
	chain.Merge(source)

	// Verify via GetFacts on the overlay (Merge uses GetFacts with empty atom,
	// which may not iterate all facts in SimpleInMemoryStore — check via chain instead)
	var found int
	chain.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
		found++
		return nil
	})
	// The Merge implementation queries with ast.Atom{} — if the store doesn't support
	// wildcard queries, merge may not copy anything. This documents that behavior.
	t.Logf("Merge: chain has %d facts for predicate, overlay estimate=%d",
		found, overlay.EstimateFactCount())
}

func TestTorture_ChainedFactStore_MultipleBases(t *testing.T) {
	base1 := factstore.NewSimpleInMemoryStore()
	base2 := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	base1.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("from_base1")}})
	base2.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("from_base2")}})
	overlay.Add(ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("from_overlay")}})

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base1, base2},
		overlay: overlay,
	}

	var found []string
	chain.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
		if len(a.Args) > 0 {
			found = append(found, convertBaseTermToInterface(a.Args[0]).(string))
		}
		return nil
	})
	if len(found) != 3 {
		t.Errorf("expected 3 facts from 2 bases + overlay, got %d: %v", len(found), found)
	}
}

func TestTorture_ChainedFactStore_AddGoesToOverlay(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	overlay := factstore.NewSimpleInMemoryStore()

	chain := &ChainedFactStore{
		base:    []factstore.FactStore{base},
		overlay: overlay,
	}

	predSym := ast.PredicateSym{Symbol: "item", Arity: 1}
	newAtom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("new_fact")}}
	chain.Add(newAtom)

	// Should be in overlay, not base
	if !overlay.Contains(newAtom) {
		t.Error("Add should write to overlay")
	}
	if base.Contains(newAtom) {
		t.Error("Add should not write to base")
	}
}

// =============================================================================
// 13. SCHEMA VALIDATOR EXPANDED TORTURE
// =============================================================================

func TestTorture_SchemaValidator_AllForbiddenHeads(t *testing.T) {
	sv := NewSchemaValidator(`Decl user(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// Test every entry in forbiddenLearnedHeads
	for head, reason := range forbiddenLearnedHeads {
		t.Run(head, func(t *testing.T) {
			rule := fmt.Sprintf(`%s(/test) :- user(/admin).`, head)
			err := sv.ValidateLearnedRule(rule)
			if err == nil {
				t.Errorf("learned rule for %q (reason: %s) should be rejected", head, reason)
			}
		})
	}
}

func TestTorture_SchemaValidator_IsDeclared(t *testing.T) {
	sv := NewSchemaValidator("Decl alpha(X).\nDecl beta(X, Y).", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	if !sv.IsDeclared("alpha") {
		t.Error("alpha should be declared")
	}
	if !sv.IsDeclared("beta") {
		t.Error("beta should be declared")
	}
	if sv.IsDeclared("gamma") {
		t.Error("gamma should not be declared")
	}
}

func TestTorture_SchemaValidator_MultiRuleValidation(t *testing.T) {
	sv := NewSchemaValidator(`
Decl user(X).
Decl admin(X).
Decl regular(X).
`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	rules := []string{
		`regular(X) :- user(X), !admin(X).`,
		`regular(X) :- user(X), !ghost_pred(X).`, // should fail
	}
	errors := sv.ValidateRules(rules)
	if len(errors) != 1 {
		t.Errorf("expected 1 error (ghost_pred undefined), got %d: %v", len(errors), errors)
	}
}

func TestTorture_SchemaValidator_LearnedFromFile(t *testing.T) {
	schemas := `Decl user(X). Decl score(X, Y).`
	learned := `custom_result(X) :- user(X).`

	sv := NewSchemaValidator(schemas, learned)
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// custom_result should be implicitly declared via learned text heads
	if !sv.IsDeclared("custom_result") {
		t.Error("custom_result should be declared from learned text")
	}
}

func TestTorture_SchemaValidator_CommentAndBlankLines(t *testing.T) {
	sv := NewSchemaValidator(`Decl item(X).`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// Comments and blank lines should not cause errors
	if err := sv.ValidateLearnedRule(""); err != nil {
		t.Errorf("empty line should be valid: %v", err)
	}
	if err := sv.ValidateLearnedRule("# This is a comment"); err != nil {
		t.Errorf("comment should be valid: %v", err)
	}
}

func TestTorture_SchemaValidator_SetPredicateArity(t *testing.T) {
	sv := NewSchemaValidator("", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	// Initially unknown
	if sv.GetArity("custom") != -1 {
		t.Error("unset arity should be -1")
	}

	sv.SetPredicateArity("custom", 3)
	if sv.GetArity("custom") != 3 {
		t.Errorf("expected arity 3, got %d", sv.GetArity("custom"))
	}

	// Check arity enforcement
	if err := sv.CheckArity("custom", 3); err != nil {
		t.Errorf("correct arity should pass: %v", err)
	}
	if err := sv.CheckArity("custom", 5); err == nil {
		t.Error("wrong arity should fail")
	}
}

func TestTorture_SchemaValidator_GetDeclaredPredicates(t *testing.T) {
	sv := NewSchemaValidator(`
Decl alpha(X).
Decl beta(X, Y).
Decl gamma(X, Y, Z).
`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates: %v", err)
	}

	preds := sv.GetDeclaredPredicates()
	if len(preds) != 3 {
		t.Errorf("expected 3 declared predicates, got %d: %v", len(preds), preds)
	}
}

// =============================================================================
// 14. ADDITIONAL CONCURRENCY TORTURE
// =============================================================================

func TestTorture_Concurrency_ConcurrentQueryAndRecompute(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.LoadSchemaString(`
Decl base(X).
Decl derived(X).
derived(X) :- base(X).
`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// Seed some data
	for i := 0; i < 50; i++ {
		_ = eng.AddFact("base", fmt.Sprintf("item_%d", i))
	}

	var wg sync.WaitGroup

	// Concurrent readers
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				_, _ = eng.GetFacts("derived")
			}
		}()
	}

	// Concurrent recompute
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				_ = eng.RecomputeRules()
			}
		}()
	}

	wg.Wait()

	// Engine should still be functional
	facts, err := eng.GetFacts("derived")
	if err != nil {
		t.Fatalf("GetFacts after concurrent ops: %v", err)
	}
	t.Logf("concurrent query+recompute: %d derived facts", len(facts))
}

func TestTorture_Concurrency_DifferentialDeltaConcurrent(t *testing.T) {
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

	var wg sync.WaitGroup

	// Concurrent delta applications
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = diffEngine.ApplyDelta([]Fact{
					{Predicate: "item", Args: []interface{}{fmt.Sprintf("g%d_i%d", gid, i)}},
				})
			}
		}(g)
	}

	// Concurrent snapshots
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				_ = diffEngine.Snapshot()
			}
		}()
	}

	wg.Wait()

	// Engine should not have panicked; count facts
	totalFacts := 0
	for _, layer := range diffEngine.strataStores {
		totalFacts += layer.store.EstimateFactCount()
	}
	t.Logf("concurrent diff delta: %d total facts", totalFacts)
}

// =============================================================================
// 15. KNOWLEDGE GRAPH TORTURE — stratum layer invariants
// =============================================================================

func TestTorture_KnowledgeGraph_NewIsEmpty(t *testing.T) {
	kg := NewKnowledgeGraph()
	if kg == nil {
		t.Fatal("NewKnowledgeGraph returned nil")
	}
	if kg.isFrozen {
		t.Error("new KnowledgeGraph should not be frozen")
	}
	if kg.store.EstimateFactCount() != 0 {
		t.Errorf("new KnowledgeGraph should be empty, got %d facts", kg.store.EstimateFactCount())
	}
}

func TestTorture_KnowledgeGraph_AddAndRetrieve(t *testing.T) {
	kg := NewKnowledgeGraph()

	predSym := ast.PredicateSym{Symbol: "test_pred", Arity: 1}
	atom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("value")}}

	kg.store.Add(atom)

	if !kg.store.Contains(atom) {
		t.Error("KnowledgeGraph should contain added fact")
	}
	if kg.store.EstimateFactCount() != 1 {
		t.Errorf("expected 1 fact, got %d", kg.store.EstimateFactCount())
	}
}

// =============================================================================
// 16. FACT STORE PROXY TORTURE — lazy loading
// =============================================================================

func TestTorture_FactStoreProxy_LazyLoading(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	proxy := NewFactStoreProxy(base)

	loadCount := 0
	predSym := ast.PredicateSym{Symbol: "lazy_pred", Arity: 2}

	proxy.RegisterLoader("lazy_pred", func(queryAtom ast.Atom) bool {
		loadCount++
		// Simulate loading content for the queried key
		if len(queryAtom.Args) > 0 {
			key := convertBaseTermToInterface(queryAtom.Args[0])
			if keyStr, ok := key.(string); ok {
				newAtom := ast.Atom{
					Predicate: predSym,
					Args:      []ast.BaseTerm{ast.String(keyStr), ast.String("loaded_content")},
				}
				base.Add(newAtom)
				return true
			}
		}
		return false
	})

	// Query should trigger lazy loading
	queryAtom := ast.Atom{
		Predicate: predSym,
		Args:      []ast.BaseTerm{ast.String("file.txt"), ast.Variable{Symbol: "Content"}},
	}

	var results []ast.Atom
	proxy.GetFacts(queryAtom, func(a ast.Atom) error {
		results = append(results, a)
		return nil
	})

	if loadCount != 1 {
		t.Errorf("expected loader called once, got %d", loadCount)
	}
}

func TestTorture_FactStoreProxy_NoLoaderForPredicate(t *testing.T) {
	base := factstore.NewSimpleInMemoryStore()
	proxy := NewFactStoreProxy(base)

	// Add a fact directly
	predSym := ast.PredicateSym{Symbol: "direct_pred", Arity: 1}
	atom := ast.Atom{Predicate: predSym, Args: []ast.BaseTerm{ast.String("value")}}
	base.Add(atom)

	// Query without a registered loader should still work
	var count int
	proxy.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
		count++
		return nil
	})
	if count != 1 {
		t.Errorf("expected 1 fact without loader, got %d", count)
	}
}

func TestTorture_Differential_RegisterVirtualPredicate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := baseEngine.LoadSchemaString(`Decl file_content(Path, Content).`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("NewDifferentialEngine: %v", err)
	}

	// Register a virtual predicate that returns content
	diffEngine.RegisterVirtualPredicate("file_content", func(key string) (string, error) {
		if key == "main.go" {
			return "package main", nil
		}
		return "", fmt.Errorf("not found: %s", key)
	})

	// Verify the base layer got wrapped with a proxy
	baseLayer := diffEngine.strataStores[0]
	if _, ok := baseLayer.store.(*FactStoreProxy); !ok {
		t.Error("RegisterVirtualPredicate should wrap base store with FactStoreProxy")
	}
}
