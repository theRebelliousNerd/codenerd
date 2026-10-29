package synth

// Behavioral uplift tests for the synth package: the compiler must never
// silently drop meaning, stamp lying arity, or bless names the builder
// rejects — and its output must run on a real engine.

import (
	"strings"
	"testing"

	manglepkg "codenerd/internal/mangle"
)

func upliftSingleClauseOptions() Options {
	return Options{
		RequireSingleClause: true,
		AllowDecls:          false,
		AllowPackage:        false,
		AllowUse:            false,
		SkipAnalysis:        true,
	}
}

// A transform on a bodyless clause used to vanish without error: the compiled
// fact carried no trace of the caller's aggregation. The AST renderer cannot
// even print that shape, so the only honest behavior is a loud error.
func TestUpliftTransformRequiresBody(t *testing.T) {
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{{
				Head: AtomSpec{Pred: "total", Args: []ExprSpec{{Kind: "var", Value: "N"}}},
				Transform: &TransformSpec{Statements: []TransformStmtSpec{{
					Kind: "let",
					Var:  "N",
					Fn:   ExprSpec{Kind: "apply", Function: "fn:count"},
				}}},
			}},
		},
	}
	_, err := Compile(spec, upliftSingleClauseOptions())
	if err == nil || !strings.Contains(err.Error(), "transform requires a non-empty body") {
		t.Fatalf("Compile(transform, no body) = %v, want transform-body error", err)
	}
}

// Arity -1 means "any": the builder must resolve it to the real arg count,
// not stamp -1 on the function symbol.
func TestUpliftArityMinusOneResolves(t *testing.T) {
	any := -1
	apply, err := buildApplyFn(ExprSpec{
		Kind:     "apply",
		Function: "fn:count",
		Args:     []ExprSpec{{Kind: "var", Value: "X"}, {Kind: "var", Value: "Y"}},
		Arity:    &any,
	})
	if err != nil {
		t.Fatalf("buildApplyFn() error = %v", err)
	}
	if apply.Function.Arity != 2 {
		t.Fatalf("resolved arity = %d, want 2 (the arg count)", apply.Function.Arity)
	}
}

// ValidateSpec alone must reject names the builder would refuse: a bare "/"
// prefix check is not validation.
func TestUpliftValidateRejectsMalformedName(t *testing.T) {
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{{
				Head: AtomSpec{Pred: "out", Args: []ExprSpec{{Kind: "name", Value: "/a//b"}}},
			}},
		},
	}
	if err := ValidateSpec(spec, upliftSingleClauseOptions()); err == nil {
		t.Fatal("ValidateSpec(malformed name) = nil, want error")
	}
	good := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{{
				Head: AtomSpec{Pred: "out", Args: []ExprSpec{{Kind: "name", Value: "/ok_atom"}}},
			}},
		},
	}
	if err := ValidateSpec(good, upliftSingleClauseOptions()); err != nil {
		t.Fatalf("ValidateSpec(good name) = %v, want nil", err)
	}
}

// End to end: JSON in, derived facts out. The compiled source must load into
// a real engine and derive through the generated rule.
func TestUpliftCompiledSourceDerivesOnRealEngine(t *testing.T) {
	raw := `{
		"format": "mangle_synth_v1",
		"program": {
			"decls": [
				{"atom": {"pred": "item", "args": [{"kind": "var", "value": "X"}]}},
				{"atom": {"pred": "chosen", "args": [{"kind": "var", "value": "X"}]}},
				{"atom": {"pred": "blocked", "args": [{"kind": "var", "value": "X"}]}}
			],
			"clauses": [{
				"head": {"pred": "chosen", "args": [{"kind": "var", "value": "X"}]},
				"body": [
					{"kind": "atom", "atom": {"pred": "item", "args": [{"kind": "var", "value": "X"}]}},
					{"kind": "not", "atom": {"pred": "blocked", "args": [{"kind": "var", "value": "X"}]}}
				]
			}]
		}
	}`
	result, err := FromResponse(raw, Options{AllowDecls: true})
	if err != nil {
		t.Fatalf("FromResponse() error = %v", err)
	}

	engine, err := manglepkg.NewEngine(manglepkg.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if err := engine.LoadSchemaString(result.Source); err != nil {
		t.Fatalf("LoadSchemaString(compiled) error = %v\n%s", err, result.Source)
	}
	// blocked(/b) goes in FIRST: engine derivation is monotone, so a
	// chosen(/b) derived before the block arrives would never retract.
	if err := engine.AddFact("blocked", "/b"); err != nil {
		t.Fatalf("AddFact(blocked) error = %v", err)
	}
	if err := engine.AddFact("item", "/a"); err != nil {
		t.Fatalf("AddFact(item) error = %v", err)
	}
	if err := engine.AddFact("item", "/b"); err != nil {
		t.Fatalf("AddFact(item) error = %v", err)
	}
	facts, err := engine.GetFacts("chosen")
	if err != nil {
		t.Fatalf("GetFacts(chosen) error = %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("chosen facts = %v, want exactly [/a]", facts)
	}
	if got := facts[0].Args[0]; got != "/a" {
		t.Fatalf("chosen fact = %v, want /a", got)
	}
}
