package types_test

// These tests live in the external test package because they parse Mangle, and
// the only sanctioned parser entry point is mangle.ParseUnit — the ANTLR
// prediction cache under codeberg.org/TauCeti/mangle-go/parse is process-global
// and mutated during parsing, so internal/mangle serializes every call
// (internal/mangle/parse_lock.go, enforced by TestCodeUsesSerializedMangleParser).
// internal/mangle imports internal/types, so an in-package test could not import
// it; types_test can.

import (
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// Fact.String must render a container as the same quoted JSON blob ToAtom
// stores. It used to render it with %v as a bare `map[a:b]`, which is not valid
// Mangle at all — and Fact.String output is not display-only:
// northstar.RenderVisionMangle writes it into a .mg file the kernel loads at
// boot, so a single container-valued fact made the whole generated file
// unparseable.
func TestFactString_WhenContainerArg_ShouldRenderQuotedJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{"map", map[string]any{"a": "b"}, `p("{\"a\":\"b\"}").`},
		{"slice of strings", []string{"x", "y"}, `p("[\"x\",\"y\"]").`},
		{"nil map encodes as null like ToAtom", map[string]any(nil), `p("null").`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := types.Fact{Predicate: "p", Args: []any{tt.arg}}
			got := fact.String()
			if got != tt.want {
				t.Fatalf("Fact.String() = %s, want %s", got, tt.want)
			}
			// The point of the change: the rendered fact must parse.
			unit, err := mangle.ParseUnit(strings.NewReader(got))
			if err != nil {
				t.Fatalf("rendered fact does not parse as Mangle: %v (%s)", err, got)
			}
			c, ok := unit.Clauses[0].Head.Args[0].(ast.Constant)
			if !ok || c.Type != ast.StringType {
				t.Fatalf("container arg re-parsed as %#v, want a string constant", unit.Clauses[0].Head.Args[0])
			}
		})
	}
}

// TestFactString_WhenWholeFloat_ShouldRoundTripAsFloat64 pins the %f verb in
// Fact.String against the obvious "simplification" to %v or to
// ast.Constant.String().
//
// mangle-go renders Float64(2.0) as the text "2", and re-parsing "2" yields a
// NumberType (int64) constant — a whole-valued float silently becomes an int on
// any string round trip. Facts DO make that trip: northstar.RenderVisionMangle
// writes Fact.String() into a .mg file that the kernel loads at boot. %f keeps
// the decimal point ("2.000000"), so the value comes back as a float.
func TestFactString_WhenWholeFloat_ShouldRoundTripAsFloat64(t *testing.T) {
	t.Parallel()
	for _, v := range []float64{2, 0, -3, 2.5} {
		fact := types.Fact{Predicate: "p", Args: []any{v}}
		unit, err := mangle.ParseUnit(strings.NewReader(fact.String()))
		if err != nil {
			t.Fatalf("Fact.String() %q does not parse: %v", fact.String(), err)
		}
		arg := unit.Clauses[0].Head.Args[0]
		c, ok := arg.(ast.Constant)
		if !ok {
			t.Fatalf("parsed arg is %T, want ast.Constant", arg)
		}
		if c.Type != ast.Float64Type {
			t.Fatalf("Fact.String()=%q re-parsed as type %v, want Float64Type "+
				"(whole floats must not decay to int64 on a string round trip)", fact.String(), c.Type)
		}
		got, err := c.Float64Value()
		if err != nil || got != v {
			t.Fatalf("round trip changed the value: got %v (err %v), want %v", got, err, v)
		}
	}
}

// The counterpart: the AST renderer really does drop the decimal point. This
// pins the upstream behaviour the test above defends against, so if a future
// mangle-go bump fixes it, this test fails and the workaround can be revisited
// rather than cargo-culted forever.
func TestMangleAST_WhenWholeFloatRendered_ShouldLoseItsFloatness(t *testing.T) {
	t.Parallel()
	rendered := ast.Float64(2).String()
	if rendered != "2" {
		t.Skipf("mangle-go now renders Float64(2) as %q; re-evaluate Fact.String's %%f verb", rendered)
	}
	unit, err := mangle.ParseUnit(strings.NewReader("p(" + rendered + ")."))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	c := unit.Clauses[0].Head.Args[0].(ast.Constant)
	if c.Type != ast.NumberType {
		t.Fatalf("expected the rendered whole float to re-parse as NumberType, got %v", c.Type)
	}
}

// TestFactString_WhenAnySupportedArgType_ShouldParseBackAsMangle is the
// invariant behind the container, float and time branches of Fact.String:
// whatever a fact holds, its printed form has to be a loadable Mangle clause.
//
// Before this pass a map-valued argument printed as a bare `map[a:b]` and a
// pointer printed as `0xc000…`, either of which turns the whole generated file
// into a parse error — which surfaces as "the kernel derived nothing" rather
// than as anything naming the fact that caused it.
func TestFactString_WhenAnySupportedArgType_ShouldParseBackAsMangle(t *testing.T) {
	t.Parallel()

	type opaque struct{ hidden int }

	tests := []struct {
		name string
		arg  any
	}{
		{"MangleAtom", types.MangleAtom("/active")},
		{"MangleString name-shaped", types.MangleString("/explain:/query")},
		{"plain string", "hello world"},
		{"name-shaped string", "/coder"},
		{"string with quotes", `she said "hi"`},
		{"int", 7},
		{"int64", int64(-9)},
		{"float64 whole", float64(2)},
		{"float64 fractional", 0.25},
		{"float32", float32(1.5)},
		{"bool true", true},
		{"bool false", false},
		{"time", time.Unix(1700000000, 123).UTC()},
		{"duration", 1500 * time.Millisecond},
		{"map", map[string]any{"k": "v"}},
		{"slice", []string{"a", "b"}},
		{"nil slice", []any(nil)},
		{"pointer to struct with only unexported fields", &opaque{hidden: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := types.Fact{Predicate: "round_trip", Args: []any{tt.arg}}
			printed := fact.String()
			unit, err := mangle.ParseUnit(strings.NewReader(printed))
			if err != nil {
				t.Fatalf("Fact.String() = %s — does not parse: %v", printed, err)
			}
			if len(unit.Clauses) != 1 || len(unit.Clauses[0].Head.Args) != 1 {
				t.Fatalf("Fact.String() = %s — parsed to an unexpected shape", printed)
			}
			if _, ok := unit.Clauses[0].Head.Args[0].(ast.Constant); !ok {
				t.Fatalf("Fact.String() = %s — argument did not parse as a constant", printed)
			}
			if strings.Contains(printed, "0x") {
				t.Fatalf("a pointer address leaked into a fact: %s", printed)
			}
		})
	}
}
