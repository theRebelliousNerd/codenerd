package core

import (
	"fmt"
	"slices"
	"testing"
)

// Query patterns carry Mangle literal types. A quoted string must not match a
// name atom with the same symbol, and vice versa. The public Fact readback
// intentionally uses plain Go strings for both, so filtering must preserve the
// parsed pattern's AST type instead of comparing only the decoded values.
func TestKernelQueryPatternPreservesLiteralType(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy(`
Decl typed_query_probe(Value, Row) bound [/any, /string].
typed_query_probe(/alice, "name-row").
typed_query_probe("/alice", "string-row").
`)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("boot typed query fixture: %v", err)
	}

	cases := []struct {
		query      string
		wantLabels []string
	}{
		{query: `typed_query_probe(/alice, Row)`, wantLabels: []string{"name-row"}},
		{query: `typed_query_probe("/alice", Row)`, wantLabels: []string{"string-row"}},
		{query: `typed_query_probe(Value, Row)`, wantLabels: []string{"name-row", "string-row"}},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			rows, err := k.Query(tc.query)
			if err != nil {
				t.Fatalf("Query(%q): %v", tc.query, err)
			}
			queryLabels := make([]string, 0, len(rows))
			for _, row := range rows {
				if len(row.Args) != 2 {
					t.Fatalf("Query(%q) returned malformed row: %#v", tc.query, row)
				}
				label, ok := row.Args[1].(string)
				if !ok {
					t.Fatalf("Query(%q) row label has type %T, want string", tc.query, row.Args[1])
				}
				queryLabels = append(queryLabels, label)
			}
			slices.Sort(queryLabels)
			if !slices.Equal(queryLabels, tc.wantLabels) {
				t.Fatalf("Query(%q) returned labels %v, want %v", tc.query, queryLabels, tc.wantLabels)
			}

			callbackLabels := make([]string, 0, len(rows))
			if err := k.QueryCallback(tc.query, func(row Fact) error {
				if len(row.Args) != 2 {
					return fmt.Errorf("malformed callback row: %#v", row)
				}
				label, ok := row.Args[1].(string)
				if !ok {
					return fmt.Errorf("callback row label has type %T, want string", row.Args[1])
				}
				callbackLabels = append(callbackLabels, label)
				return nil
			}); err != nil {
				t.Fatalf("QueryCallback(%q): %v", tc.query, err)
			}
			slices.Sort(callbackLabels)
			if !slices.Equal(callbackLabels, tc.wantLabels) {
				t.Fatalf("QueryCallback(%q) returned labels %v, want %v", tc.query, callbackLabels, tc.wantLabels)
			}
		})
	}
}
