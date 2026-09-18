package core

import (
	"strings"
	"testing"
)

func TestKernelEval_Evaluate(t *testing.T) {
	k := setupMockKernel(t)

	// 1. Assert some base facts
	k.Assert(Fact{Predicate: "foo", Args: []any{"bar"}})
	k.Assert(Fact{Predicate: "num", Args: []any{42}})

	// 2. Define a rule in policy
	// Explicitly declare predicates for strict mode
	policy := `
	Decl foo(Name).
	Decl num(Number).
	Decl baz(Name).
	Decl big(Number).

	baz(X) :- foo(X).
	big(X) :- num(N), N > 10, X = N.
	`
	k.AppendPolicy(policy)

	// 3. Evaluate
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// 4. Verify results
	results, err := k.Query("baz")
	if err != nil {
		t.Fatalf("Query baz failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for baz, got %d", len(results))
	} else if len(results) > 0 && results[0].Args[0] != "bar" {
		t.Errorf("Expected baz('bar'), got %v", results[0])
	}

	results, _ = k.Query("big")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for big, got %d", len(results))
	}
}

// A kernel built with no explicit configuration must still bound inference
// at the package default; an unbounded fixpoint over learned rules is the
// fact-explosion failure the gas limit exists to prevent.
func TestKernelEval_ZeroConfigDerivedFactLimit(t *testing.T) {
	k := &RealKernel{}

	k.mu.Lock()
	limit := k.effectiveDerivedFactLimitLocked()
	k.mu.Unlock()

	if limit != defaultDerivedFactLimit {
		t.Fatalf("zero-config derived-fact limit = %d, want kernel default %d", limit, defaultDerivedFactLimit)
	}
}

func TestKernelEval_Stratification(t *testing.T) {
	k := setupMockKernel(t)

	// Negation cycle with Mangle `!` syntax: must parse, then fail stratification.
	badPolicy := `
	Decl strat_bad_base(Name).
	Decl strat_bad_p(Name).
	Decl strat_bad_q(Name).
	strat_bad_p(X) :- strat_bad_base(X), !strat_bad_q(X).
	strat_bad_q(X) :- strat_bad_base(X), !strat_bad_p(X).
	`
	k.AppendPolicy(badPolicy)

	err := k.Evaluate()
	if err == nil {
		t.Fatalf("expected stratification error for negation cycle, got nil; stratification is checked in rebuildProgram (internal/core/kernel_eval.go)")
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "parse") || strings.Contains(msg, "syntax") {
		t.Fatalf("expected stratification error, got parse/syntax error: %v", err)
	}
	if !strings.Contains(msg, "strat") && !strings.Contains(msg, "cycl") && !strings.Contains(msg, "negat") {
		t.Fatalf("expected stratification/cycle error, got: %v", err)
	}

	// Same shape but well-stratified (q defined without negating p) must load cleanly.
	good := setupMockKernel(t)
	goodPolicy := `
	Decl strat_good_base(Name).
	Decl strat_good_p(Name).
	Decl strat_good_q(Name).
	strat_good_p(X) :- strat_good_base(X), !strat_good_q(X).
	strat_good_q(X) :- strat_good_base(X).
	`
	good.AppendPolicy(goodPolicy)
	if err := good.Evaluate(); err != nil {
		t.Fatalf("well-stratified program with same shape failed to load: %v", err)
	}
}
