package types

import (
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func TestAtom_WhenGivenAnyIdentifier_ShouldProduceANameConstant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want MangleAtom
	}{
		{"plain identifier", "hands_free", "/hands_free"},
		{"already prefixed", "/hands_free", "/hands_free"},
		{"upper case folds", "ExecutionError", "/executionerror"},
		{"spaces become underscores", "pattern not found", "/pattern_not_found"},
		{"punctuation runs collapse", "not found: pattern", "/not_found_pattern"},
		{"dots are not extensions", "no.confirmation", "/no_confirmation"},
		{"hyphen survives", "read-file", "/read-file"},
		{"deep path flattens", "/a/b/c", "/a_b_c"},
		{"leading and trailing junk trimmed", "  __weird__  ", "/weird"},
		{"empty is /unknown", "", "/unknown"},
		{"punctuation only is /unknown", "///", "/unknown"},
		{"unicode is replaced", "日本", "/unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Atom(tt.in)
			if got != tt.want {
				t.Fatalf("Atom(%q) = %q, want %q", tt.in, got, tt.want)
			}
			// The whole point is that the result reaches the kernel as a name,
			// so assert on the produced constant, not just the string.
			atom, err := (Fact{Predicate: "p", Args: []any{got}}).ToAtom()
			if err != nil {
				t.Fatalf("Atom(%q) produced %q which ToAtom rejects: %v", tt.in, got, err)
			}
			c, ok := atom.Args[0].(ast.Constant)
			if !ok || c.Type != ast.NameType {
				t.Fatalf("Atom(%q) reached the kernel as %#v, want a NameType constant", tt.in, atom.Args[0])
			}
		})
	}
}

// The values that actually appear at the mismatched assert sites found by the
// audit must all survive the conversion, so the fix really is a one-liner.
func TestAtom_WhenGivenTheAuditedMismatchValues_ShouldRoundTripThroughIsValidName(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"hands_free", "execution_error", "internal_error",
		"branch", "modified_files", "recent_commits", "unstaged_count",
		"pattern_not_found", "no_confirmation",
	} {
		got := string(Atom(v))
		if got != "/"+v {
			t.Errorf("Atom(%q) = %q, want %q", v, got, "/"+v)
		}
		if !isValidMangleNameConstant(got) {
			t.Errorf("Atom(%q) produced %q, which is not a valid name constant", v, got)
		}
	}
}
