package types

import (
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// TestMangleString_FactStringAndToAtom table-drives the six cases described
// in the task over both Fact.String (printed form) and Fact.ToAtom
// (concrete term type). The MangleString type must always produce a string
// constant, never a name, whatever the shape of the value looks like.
func TestMangleString_FactStringAndToAtom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		arg        any
		wantString string // full Fact.String() with predicate "p"
		wantSymbol string
		isString   bool // true => String constant, false => Name constant
	}{
		{
			// Exact production value that motivated MangleString: /explain:/query was
			// being stored as a name in a column declared /string when passed as a
			// plain Go string because Fact.String/ToAtom infer term type from shape.
			// MangleString must force a quoted string constant.
			name:       "MangleString /explain:/query is string constant (production value)",
			arg:        MangleString("/explain:/query"),
			wantString: `p("/explain:/query").`,
			wantSymbol: "/explain:/query",
			isString:   true,
		},
		{
			name:       "MangleString plain is string constant",
			arg:        MangleString("plain"),
			wantString: `p("plain").`,
			wantSymbol: "plain",
			isString:   true,
		},
		{
			name:       "MangleString empty is string constant and does not error",
			arg:        MangleString(""),
			wantString: `p("").`,
			wantSymbol: "",
			isString:   true,
		},
		{
			// Unchanged behaviour: MangleAtom must still produce a name constant.
			name:       "MangleAtom /success still produces name constant (unchanged behaviour pinned)",
			arg:        MangleAtom("/success"),
			wantString: `p(/success).`,
			wantSymbol: "/success",
			isString:   false,
		},
		{
			// Pins existing shape inference as deliberately unchanged. Plain string
			// "/success" is a valid Mangle name per isValidMangleNameConstant, so
			// it stays a name. The fix is additive via MangleString; a future edit
			// that "cleans up" inference should fail here consciously.
			name:       "plain string /success still produces name constant (shape inference pinned additive fix)",
			arg:        "/success",
			wantString: `p(/success).`,
			wantSymbol: "/success",
			isString:   false,
		},
		{
			name:       "plain string not an atom still produces string constant",
			arg:        "not an atom",
			wantString: `p("not an atom").`,
			wantSymbol: "not an atom",
			isString:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fact := Fact{Predicate: "p", Args: []any{tt.arg}}

			// Fact.String: printed form already distinguishes quoted vs unquoted.
			gotStr := fact.String()
			if gotStr != tt.wantString {
				t.Fatalf("Fact.String() = %q, want %q", gotStr, tt.wantString)
			}

			// Fact.ToAtom: compare produced term against constructed expected term.
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() unexpected error: %v", err)
			}
			if len(atom.Args) != 1 {
				t.Fatalf("ToAtom() args len = %d, want 1", len(atom.Args))
			}

			term := atom.Args[0]
			c, ok := term.(ast.Constant)
			if !ok {
				t.Fatalf("ToAtom() arg is %T, want ast.Constant", term)
			}

			if c.Symbol != tt.wantSymbol {
				t.Fatalf("ToAtom() Symbol = %q, want %q", c.Symbol, tt.wantSymbol)
			}

			var wantTerm ast.Constant
			if tt.isString {
				wantTerm = ast.String(tt.wantSymbol)
			} else {
				var err error
				wantTerm, err = ast.Name(tt.wantSymbol)
				if err != nil {
					t.Fatalf("ast.Name(%q) error: %v", tt.wantSymbol, err)
				}
			}
			if c != wantTerm {
				t.Fatalf("ToAtom() term = %#v, want %#v (symbol %q)", c, wantTerm, tt.wantSymbol)
			}
		})
	}
}

// Separated Fact.String table to make failure messages focused on the
// rendering side, while still covering the same six cases.
func TestMangleString_FactString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arg        any
		wantString string
	}{
		{"MangleString /explain:/query is string constant (production value)", MangleString("/explain:/query"), `p("/explain:/query").`},
		{"MangleString plain is string constant", MangleString("plain"), `p("plain").`},
		{"MangleString empty is string constant and does not error", MangleString(""), `p("").`},
		{"MangleAtom /success still produces name constant (unchanged behaviour pinned)", MangleAtom("/success"), `p(/success).`},
		{"plain string /success still produces name constant (shape inference pinned additive fix)", "/success", `p(/success).`},
		{"plain string not an atom still produces string constant", "not an atom", `p("not an atom").`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "p", Args: []any{tt.arg}}
			if got := fact.String(); got != tt.wantString {
				t.Fatalf("Fact.String() = %q, want %q", got, tt.wantString)
			}
		})
	}
}

// Separated ToAtom table to make failure messages focused on the concrete
// term, comparing the produced term against a constructed expected term so
// a name-vs-string regression fails on the term itself.
func TestMangleString_ToAtom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arg        any
		wantSymbol string
		isString   bool
	}{
		{"MangleString /explain:/query is string constant (production value)", MangleString("/explain:/query"), "/explain:/query", true},
		{"MangleString plain is string constant", MangleString("plain"), "plain", true},
		{"MangleString empty is string constant and does not error", MangleString(""), "", true},
		{"MangleAtom /success still produces name constant (unchanged behaviour pinned)", MangleAtom("/success"), "/success", false},
		{"plain string /success still produces name constant (shape inference pinned additive fix)", "/success", "/success", false},
		{"plain string not an atom still produces string constant", "not an atom", "not an atom", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "p", Args: []any{tt.arg}}
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() unexpected error: %v", err)
			}
			if len(atom.Args) != 1 {
				t.Fatalf("ToAtom() args len = %d, want 1", len(atom.Args))
			}
			var c ast.Constant
			switch v := atom.Args[0].(type) {
			case ast.Constant:
				c = v
			default:
				t.Fatalf("ToAtom() arg is %T, want ast.Constant", v)
			}
			if c.Symbol != tt.wantSymbol {
				t.Fatalf("ToAtom() Symbol = %q, want %q", c.Symbol, tt.wantSymbol)
			}
			var wantTerm ast.Constant
			if tt.isString {
				wantTerm = ast.String(tt.wantSymbol)
			} else {
				var err error
				wantTerm, err = ast.Name(tt.wantSymbol)
				if err != nil {
					t.Fatalf("ast.Name(%q) error: %v", tt.wantSymbol, err)
				}
			}
			if c != wantTerm {
				t.Fatalf("ToAtom() term = %#v, want %#v", c, wantTerm)
			}
		})
	}
}
