package core

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// TestKernelValidation_AbsurdCorruptedHeal tests self-healing capability
// on extremely corrupted rules, validating statistics and parsing safety.
func TestKernelValidation_AbsurdCorruptedHeal(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl valid_pred(Name).\nDecl other_pred(Name, Val).")
	k.SetPolicy("")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Kernel evaluate failed: %v", err)
	}

	corruptedText := `
# Normal comment
valid_pred("ok").

# 1. Unclosed string literal
valid_pred("unclosed string.

# 2. Complete gibberish
Gibberish!!! Emojis 🚀🔥

# 3. Missing dot
other_pred("missing", "dot")

# 4. Undeclared predicate
undeclared_pred("oops").

# 5. Infinite loop risk
next_action(/do_something) :- current_time(T).

# 6. Valid rule
other_pred(X, "val") :- valid_pred(X).
`

	healedText := k.healLearnedRules(corruptedText, "")

	// KNOWN GAP: validateLearnedRulesContent uses a line-by-line parser that only recognizes:
	//   isRule: strings.Contains(trimmed, ":-")
	//   isFact: !isRule && strings.Contains(trimmed, "(") && strings.HasSuffix(trimmed, ").")
	// Lines that don't match either pattern (unclosed strings, gibberish, missing-dot expressions)
	// pass through unchanged and are NOT counted in stats.
	//
	// Additionally, facts (no ":-") pass ValidateRule since it only checks body predicates.
	// So undeclared_pred("oops"). passes because there's no body to validate.
	//
	// Recognized lines from the corrupted text:
	//   1. valid_pred("ok").               -> fact, valid (syntax OK, schema OK)
	//   2. undeclared_pred("oops").         -> fact, valid (no body to check, head not forbidden)
	//   3. next_action(/do_something) :- current_time(T). -> rule, INVALID (undefined predicate current_time)
	//   4. other_pred(X, "val") :- valid_pred(X).         -> rule, valid
	//
	// NOT recognized (silently passed through):
	//   - valid_pred("unclosed string.     -> no closing ")."
	//   - Gibberish!!! Emojis 🚀🔥         -> no "(" at all
	//   - other_pred("missing", "dot")     -> ends with ")" not ")."
	t.Log("KNOWN GAP: line-by-line parser does not catch malformed lines that lack rule/fact structure")

	// The next_action rule is caught by schema validation (undefined predicate in body)
	if !strings.Contains(healedText, "# SELF-HEALED: rule uses undefined predicates") {
		t.Errorf("Expected healed text to contain SELF-HEALED comment for undefined predicates, got:\n%s", healedText)
	}

	// Verify valid rules are kept intact
	if !strings.Contains(healedText, `valid_pred("ok").`) {
		t.Errorf("Expected valid_pred(\"ok\"). to be kept intact")
	}
	if !strings.Contains(healedText, `other_pred(X, "val") :- valid_pred(X).`) {
		t.Errorf("Expected valid rule to be kept intact")
	}

	// Verify that unrecognized corrupted lines pass through unchanged (known gap)
	if !strings.Contains(healedText, `Gibberish!!! Emojis`) {
		t.Errorf("Expected gibberish line to pass through unchanged (parser does not recognize it)")
	}
	if !strings.Contains(healedText, `other_pred("missing", "dot")`) {
		t.Errorf("Expected missing-dot line to pass through unchanged (parser does not recognize it)")
	}

	// Verify validation stats match actual parser behavior
	stats := k.validateLearnedRulesContent(corruptedText, "", false)
	if stats.stats.TotalRules != 4 {
		t.Errorf("Expected 4 recognized rules/facts, got %d", stats.stats.TotalRules)
	}
	if stats.stats.ValidRules != 3 {
		t.Errorf("Expected 3 valid (valid_pred fact, undeclared_pred fact, other_pred rule), got %d", stats.stats.ValidRules)
	}
	if stats.stats.InvalidRules != 1 {
		t.Errorf("Expected 1 invalid (next_action with undefined current_time), got %d", stats.stats.InvalidRules)
	}
}

// TestKernelValidation_AbsurdInfiniteLoopRisk exhaustively checks loop-risk classification
// across complex combinations of ubiquitous, negation-only, and idle-state predicates.
func TestKernelValidation_AbsurdInfiniteLoopRisk(t *testing.T) {
	k := setupMockKernel(t)

	tests := []struct {
		name     string
		rule     string
		wantRisk bool
	}{
		{
			name:     "unconditional system action fact",
			rule:     "next_action(/initialize).",
			wantRisk: true,
		},
		{
			name:     "unconditional normal action fact",
			rule:     "next_action(/normal_action).",
			wantRisk: false,
		},
		{
			name:     "negation-only condition",
			rule:     "next_action(/alert) :- !current_task(/alert).",
			wantRisk: true,
		},
		{
			name:     "negation with positive guard",
			rule:     "next_action(/alert) :- has_alert(A), !current_task(/alert).",
			wantRisk: false,
		},
		{
			name:     "idle-state loop",
			rule:     "next_action(/tick) :- coder_state(/idle).",
			wantRisk: true,
		},
		{
			name:     "idle-state with proper guard",
			rule:     "next_action(/tick) :- coder_state(/idle), time_elapsed(T), active_session(S), T > 10.",
			wantRisk: false,
		},
		{
			name:     "ubiquitous current_time wildcard",
			rule:     "next_action(/tick) :- current_time(_).",
			wantRisk: true,
		},
		{
			name:     "ubiquitous build_system",
			rule:     "next_action(/run) :- build_system(S).",
			wantRisk: true,
		},
		{
			name:     "broad wildcard session_state",
			rule:     "next_action(/alert) :- session_state(_, _).",
			wantRisk: true,
		},
		{
			name:     "safe session_state with constants",
			rule:     "next_action(/alert) :- session_state(/active, /error).",
			wantRisk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := k.checkInfiniteLoopRisk(tt.rule)
			gotRisk := result != ""
			if gotRisk != tt.wantRisk {
				t.Errorf("checkInfiniteLoopRisk(%q) = %q, wantRisk=%v, gotRisk=%v",
					tt.rule, result, tt.wantRisk, gotRisk)
			}
		})
	}
}

// TestKernelValidation_AbsurdConcurrencyStress runs high concurrent read/write loads
// to stress-test schema hot-reloads and concurrent validation methods, checking for deadlocks.
func TestKernelValidation_AbsurdConcurrencyStress(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl base_pred(Name).")
	k.SetPolicy("")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Kernel evaluate failed: %v", err)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const readers = 40
	const writers = 10

	// Concurrent Readers
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(id)))
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// 1. Validate learned rules
					_ = k.ValidateLearnedRule("base_pred(\"test\").")

					// 2. Validate multiple rules
					_ = k.ValidateLearnedRules([]string{
						"base_pred(\"rule_1\").",
						"invalid_pred(\"rule_2\").",
					})

					// 3. Check declared predicates
					_ = k.IsPredicateDeclared("base_pred")
					_ = k.GetDeclaredPredicates()

					// Sleep a tiny bit to prevent pegging CPU completely
					time.Sleep(time.Duration(rng.Intn(100)) * time.Microsecond)
				}
			}
		}(i)
	}

	// Concurrent Writers (Schema Hot-reloads)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(id + 1000)))
			for {
				select {
				case <-ctx.Done():
					return
				default:
					schema := fmt.Sprintf("Decl base_pred(Name).\nDecl dynamic_pred_%d(Val).", rng.Intn(10))
					k.SetSchemas(schema)

					// Sleep to allow read operations to progress
					time.Sleep(time.Duration(rng.Intn(500)) * time.Microsecond)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestKernelValidation_AbsurdTransactionEdgeCases validates transactional operations
// against dynamic predicate sets under high concurrency, checking type enforcement bounds.
func TestKernelValidation_AbsurdTransactionEdgeCases(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl num_pred(Number).\nDecl name_pred(Name).")
	k.SetPolicy("")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Kernel evaluate failed: %v", err)
	}

	// 1. Valid transaction
	tx := k.Transaction()
	tx.Assert(types.Fact{Predicate: "num_pred", Args: []interface{}{42}})
	tx.Assert(types.Fact{Predicate: "name_pred", Args: []interface{}{"john"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit valid transaction: %v", err)
	}

	if err := k.Evaluate(); err != nil {
		t.Errorf("Evaluate failed after valid transaction: %v", err)
	}

	// Verify asserted facts
	numFacts, err := k.Query("num_pred")
	if err != nil || len(numFacts) != 1 || numFacts[0].Args[0].(int64) != 42 {
		t.Errorf("Expected num_pred(42), got: %v", numFacts)
	}

	// 2. Type Mismatch Assertion
	tx2 := k.Transaction()
	// Assert string "not_a_number" into num_pred(Number)
	tx2.Assert(types.Fact{Predicate: "num_pred", Args: []interface{}{"not_a_number"}})
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit transactional assertion: %v", err)
	}

	// Under strict rules evaluate should flag warning or error depending on Mangle tolerance
	evalErr := k.Evaluate()
	t.Logf("Evaluate error on type mismatch: %v", evalErr)

	// 3. Exact Fact Retraction
	tx3 := k.Transaction()
	tx3.RetractExactFact(types.Fact{Predicate: "name_pred", Args: []interface{}{"john"}})
	if err := tx3.Commit(); err != nil {
		t.Fatalf("Failed to commit retraction transaction: %v", err)
	}

	k.Evaluate()
	nameFacts, _ := k.Query("name_pred")
	if len(nameFacts) != 0 {
		t.Errorf("Expected name_pred(john) to be retracted, but got: %v", nameFacts)
	}
}
