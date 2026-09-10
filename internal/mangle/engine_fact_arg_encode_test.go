package mangle

import (
	"math"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// A fact argument that cannot be encoded must stop the conversion, never
// become a constant.
//
// The failure this pins is specific to a logic kernel and worth stating
// plainly: json.Marshal returns nil on error, string(nil) is "", and
// ast.String("") is a perfectly valid Mangle constant. So a map holding a NaN
// used to be asserted as an empty string, indistinguishable from a genuinely
// empty argument, and every rule that fired on it derived confidently from a
// value that was not the value. An error here costs one fact; the alternative
// costs the soundness of everything derived from it.
func TestConvertValueToTypedTermRefusesUnencodableArguments(t *testing.T) {
	unencodable := map[string]any{
		"map with a NaN":     map[string]any{"score": math.NaN()},
		"map with an Inf":    map[string]any{"score": math.Inf(1)},
		"map with a channel": map[string]any{"ch": make(chan int)},
		"bare channel":       make(chan int),
		"func value":         func() {},
	}

	for name, v := range unencodable {
		t.Run(name, func(t *testing.T) {
			term, err := convertValueToTypedTerm(v, ast.StringType)
			if err == nil {
				t.Fatalf("converted an unencodable argument to %v with no error", term)
			}
			if term != nil {
				t.Errorf("returned term %v alongside its error", term)
			}
			if !strings.Contains(err.Error(), "unsupported fact argument type") {
				t.Errorf("error does not name the problem: %v", err)
			}
		})
	}
}

// The encodable maps must still convert, and must not be confusable with the
// empty string the broken path produced.
func TestConvertValueToTypedTermEncodesMaps(t *testing.T) {
	cases := map[string]any{
		"string map": map[string]string{"k": "v"},
		"any map":    map[string]any{"k": "v"},
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			term, err := convertValueToTypedTerm(v, ast.StringType)
			if err != nil {
				t.Fatalf("convertValueToTypedTerm: %v", err)
			}
			got, ok := term.(ast.Constant)
			if !ok {
				t.Fatalf("expected a constant, got %T", term)
			}
			s, err := got.StringValue()
			if err != nil {
				t.Fatalf("StringValue: %v", err)
			}
			if s != `{"k":"v"}` {
				t.Errorf("encoded to %q, want %q", s, `{"k":"v"}`)
			}
		})
	}
}
