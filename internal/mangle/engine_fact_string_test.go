package mangle

import (
	"testing"

	"codenerd/internal/types"
)

// TestFactString_TypedSerialisation pins the text-serializer fix for
// Fact.String: MangleString must always render quoted (even when the
// value looks like a name), MangleAtom must render bare, and the
// existing plain-string inference (strings.HasPrefix "/" => name) is
// left intentionally looser than types.Fact's isValidMangleNameConstant
// so callers depending on it keep working.
func TestFactString_TypedSerialisation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fact Fact
		want string
	}{
		{
			name: "MangleString slash value is quoted",
			fact: Fact{Predicate: "p", Args: []any{types.MangleString("/explain:/query")}},
			want: `p("/explain:/query").`,
		},
		{
			name: "MangleAtom renders unquoted as name constant",
			fact: Fact{Predicate: "p", Args: []any{types.MangleAtom("/success")}},
			want: `p(/success).`,
		},
		{
			name: "plain string slash value still renders unquoted (inference pinned)",
			fact: Fact{Predicate: "p", Args: []any{"/success"}},
			want: `p(/success).`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.fact.String(); got != tt.want {
				t.Fatalf("Fact.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
