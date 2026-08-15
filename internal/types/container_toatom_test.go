package types

import (
	"encoding/json"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// CONTAINER ARGUMENTS
// =============================================================================
//
// Mangle has no compound constant, so ToAtom encodes maps and slices as JSON
// string constants. These tables pin the exact encoding, because the encoding
// IS the wire format: pending_action payloads, virtual_store action metadata and
// intent metadata are written by one package and re-decoded by another, and a
// change from JSON to %v (or to a different key order) breaks the reader with no
// compile error anywhere.

func TestToAtom_WhenMapArg_ShouldProduceJSONStringConstant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{"empty map", map[string]any{}, `{}`},
		{"string values", map[string]any{"b": "two", "a": "one"}, `{"a":"one","b":"two"}`},
		{"mixed scalar values", map[string]any{"n": float64(2), "s": "x", "t": true}, `{"n":2,"s":"x","t":true}`},
		{"nested container", map[string]any{"outer": map[string]any{"inner": []any{1.0, 2.0}}}, `{"outer":{"inner":[1,2]}}`},
		{"nil-valued key", map[string]any{"k": nil}, `{"k":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			atom, err := Fact{Predicate: "p", Args: []any{tt.arg}}.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertStringConstant(t, atom.Args[0], tt.want)
		})
	}
}

func TestToAtom_WhenSliceArg_ShouldProduceJSONStringConstant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{"empty []any", []any{}, `[]`},
		{"[]any of scalars", []any{"a", float64(1), true}, `["a",1,true]`},
		{"[]string", []string{"a", "b"}, `["a","b"]`},
		{"[]string empty", []string{}, `[]`},
		{"[]int", []int{1, 2, 3}, `[1,2,3]`},
		{"[]int64", []int64{9007199254740993}, `[9007199254740993]`},
		{"[]float64 whole", []float64{2, 3}, `[2,3]`},
		{"[]float64 fractional", []float64{0.5, 2.25}, `[0.5,2.25]`},
		{"[]any nested", []any{[]any{"x"}, map[string]any{"k": "v"}}, `[["x"],{"k":"v"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			atom, err := Fact{Predicate: "p", Args: []any{tt.arg}}.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertStringConstant(t, atom.Args[0], tt.want)
		})
	}
}

// Nil containers are typed nils, not the untyped nil that ToAtom rejects: a
// caller that built an empty slice and a caller that built none should not
// diverge into "fact written" vs "assert failed".
func TestToAtom_WhenNilTypedContainer_ShouldEncodeAsJSONNull(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{"nil map[string]any", map[string]any(nil), `null`},
		{"nil []any", []any(nil), `null`},
		{"nil []string", []string(nil), `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			atom, err := Fact{Predicate: "p", Args: []any{tt.arg}}.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertStringConstant(t, atom.Args[0], tt.want)
		})
	}
}

// Containers whose contents cannot be JSON-encoded must fail loudly and name the
// predicate and index. Silently storing "0x..." or "<nil>" is the failure this
// branch exists to prevent.
func TestToAtom_WhenContainerHoldsUnencodableValue_ShouldReturnNamedError(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "unencodable_pred", Args: []any{"ok", map[string]any{"fn": func() {}}}}
	_, err := fact.ToAtom()
	if err == nil {
		t.Fatal("expected error for a container holding a func value")
	}
	if !strings.Contains(err.Error(), "unencodable_pred") {
		t.Errorf("error must name the predicate, got: %v", err)
	}
	if !strings.Contains(err.Error(), "index 1") {
		t.Errorf("error must name the argument index, got: %v", err)
	}
}

// A container round-trips: what a reader json.Unmarshals must equal what the
// writer passed. Pinning this makes the JSON encoding a contract rather than an
// implementation detail of ToAtom's default branch.
func TestToAtom_WhenContainerArg_ShouldRoundTripThroughJSON(t *testing.T) {
	t.Parallel()
	original := map[string]any{"path": "/tmp/x.go", "lines": []any{float64(1), float64(2)}, "ok": true}
	atom, err := Fact{Predicate: "p", Args: []any{original}}.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	c, ok := atom.Args[0].(ast.Constant)
	if !ok || c.Type != ast.StringType {
		t.Fatalf("expected string constant, got %#v", atom.Args[0])
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(c.Symbol), &back); err != nil {
		t.Fatalf("stored container is not valid JSON: %v (%q)", err, c.Symbol)
	}
	if back["path"] != "/tmp/x.go" || back["ok"] != true {
		t.Fatalf("round trip lost data: %#v", back)
	}
	if lines, ok := back["lines"].([]any); !ok || len(lines) != 2 {
		t.Fatalf("round trip lost the slice: %#v", back["lines"])
	}
}
