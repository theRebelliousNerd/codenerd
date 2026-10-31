package mangle

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

const readinessProgram = `
Decl pending(X) bound [/string].
Decl excluded(X) bound [/string].
Decl tag(X) bound [/string].
Decl eligible(X) bound [/string].
eligible(X) :- pending(X), !excluded(X).
`

func readinessEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.LoadSchemaString(readinessProgram); err != nil {
		t.Fatal(err)
	}
	if err := e.AddFact("pending", "work.go"); err != nil {
		t.Fatal(err)
	}
	if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 {
		t.Fatalf("baseline is not materialized: %v / %v", rows, err)
	}
	return e
}

func assertDerivedUnavailable(t *testing.T, e *Engine, predicate string, cause error) {
	t.Helper()
	if rows, err := e.GetFacts(predicate); err == nil || !errors.Is(err, cause) || len(rows) != 0 {
		t.Errorf("GetFacts served unready derived state: %v / %v (cause %v)", rows, err, cause)
	}
	if rows, err := e.AtomStrings(predicate); err == nil || !errors.Is(err, cause) || len(rows) != 0 {
		t.Errorf("AtomStrings served unready derived state: %v / %v", rows, err)
	}
	if rows, err := e.Query(t.Context(), predicate+"(X)"); err == nil || !errors.Is(err, cause) || rows != nil {
		t.Errorf("Query served unready derived state: %v / %v", rows, err)
	}
	if rows := e.QueryFacts(predicate); len(rows) != 0 {
		t.Errorf("QueryFacts served unready derived state: %v", rows)
	}
	for row := range e.GetFactsSeq(predicate) {
		t.Errorf("GetFactsSeq served unready derived state: %v", row)
	}
	for row := range e.EvaluateRule(predicate) {
		t.Errorf("EvaluateRule served unready derived state: %v", row)
	}
}

// Cancel at the next check after 256 accepted inserts, without wall-clock
// timing or an inconsistent Context: cancellation closes the real Done channel.
type readinessCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *readinessCancelContext) Err() error {
	c.checks++
	if c.checks == 3 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestEngineFailedBatch_DerivedReadsRequireRecovery(t *testing.T) {
	for _, mode := range []string{"invalid_prefix", "fact_limit", "cancel_prefix", "replace_file", "replace_control", "control_predicate", "manual"} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AutoEval = true
			if mode == "fact_limit" {
				cfg.FactLimit = 2
			}
			e := readinessEngine(t, cfg)
			bad := Fact{Predicate: "undeclared", Args: []any{"bad"}}
			prefix := Fact{Predicate: "excluded", Args: []any{"work.go"}}
			var err error
			switch mode {
			case "replace_file":
				err = e.ReplaceFactsForFile("work.go", []Fact{prefix, bad})
			case "replace_control":
				err = e.ReplaceControlFacts([]Fact{prefix, bad}, "pending")
			case "control_predicate":
				err = e.ReplaceControlFacts(nil, "pending", "undeclared")
			case "fact_limit":
				err = e.AddFacts([]Fact{prefix, {Predicate: "tag", Args: []any{"over_limit"}}})
			case "cancel_prefix":
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				checkCtx := &readinessCancelContext{Context: ctx, cancel: cancel}
				batch := []Fact{prefix}
				for i := 0; i < 256; i++ {
					batch = append(batch, Fact{Predicate: "tag", Args: []any{fmt.Sprint(i)}})
				}
				err = e.AddFactsContext(checkCtx, batch)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("missing cancellation: %v", err)
				}
				if tags, tagErr := e.GetFacts("tag"); tagErr != nil || len(tags) != 255 {
					t.Fatalf("wrong accepted prefix: %d tags / %v", len(tags), tagErr)
				}
			default:
				if mode == "manual" {
					e.ToggleAutoEval(false)
				}
				err = e.AddFacts([]Fact{prefix, bad, {Predicate: "tag", Args: []any{"suffix"}}})
			}
			if err == nil {
				t.Fatal("fixture did not fail")
			}
			if mode != "control_predicate" {
				if base, baseErr := e.GetFacts("excluded"); baseErr != nil || len(base) != 1 {
					t.Fatalf("accepted base prefix lost: %v / %v", base, baseErr)
				}
			}
			assertDerivedUnavailable(t, e, "eligible", err)
			if noop := e.AddFacts(nil); noop != nil {
				t.Fatal(noop)
			}
			e.ToggleAutoEval(true)
			assertDerivedUnavailable(t, e, "eligible", err)
			if err := e.RecomputeRules(); err != nil {
				t.Fatal(err)
			}
			if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 0 {
				t.Fatalf("recovery kept the stale conclusion: %v / %v", rows, err)
			}
		})
	}
}

func TestEngineFailedEvaluation_DerivedReadsRequireRecovery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval, cfg.DerivedFactsLimit = true, 2
	e := readinessEngine(t, cfg)
	if err := e.LoadSchemaString("Decl fan_input(N) bound [/number].\nDecl fan(N) bound [/number].\nfan(N) :- fan_input(N).\n"); err != nil {
		t.Fatal(err)
	}
	if err := e.Evaluate(); err != nil {
		t.Fatal(err)
	}
	var batch []Fact
	for i := range int64(6) {
		batch = append(batch, Fact{Predicate: "fan_input", Args: []any{i}})
	}
	err := e.AddFacts(batch)
	if err == nil {
		t.Fatal("gas-limit fixture did not fail")
	}
	if base, err := e.GetFacts("fan_input"); err != nil || len(base) != 6 {
		t.Fatalf("accepted EDB did not survive evaluation failure: %v / %v", base, err)
	}
	assertDerivedUnavailable(t, e, "eligible", err)
	assertDerivedUnavailable(t, e, "fan", err)
	if err := e.ReplaceControlFacts(nil, "fan_input"); err != nil {
		t.Fatalf("recovery under the original gas limit: %v", err)
	}
	if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 {
		t.Fatalf("unrelated conclusion not recovered: %v / %v", rows, err)
	}
	if rows, err := e.GetFacts("fan"); err != nil || len(rows) != 0 {
		t.Fatalf("partial evaluation leaked through recovery: %v / %v", rows, err)
	}
}

func TestEngineRejectedBeforeMutation_KeepsDerivedReadsReady(t *testing.T) {
	for _, mode := range []string{"invalid_first", "duplicate_then_invalid", "precancelled", "unknown_control"} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AutoEval = true
			e := readinessEngine(t, cfg)
			bad := Fact{Predicate: "undeclared", Args: []any{"bad"}}
			var err error
			switch mode {
			case "duplicate_then_invalid":
				err = e.AddFacts([]Fact{{Predicate: "pending", Args: []any{"work.go"}}, bad})
			case "precancelled":
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				err = e.AddFactsContext(ctx, []Fact{bad})
			case "unknown_control":
				err = e.ReplaceControlFacts(nil, "undeclared")
			default:
				err = e.AddFacts([]Fact{bad})
			}
			if err == nil {
				t.Fatal("rejection fixture succeeded")
			}
			if rows, err := e.GetFacts("eligible"); err != nil || len(rows) != 1 {
				t.Fatalf("rejection poisoned unchanged materialization: %v / %v", rows, err)
			}
		})
	}
}
