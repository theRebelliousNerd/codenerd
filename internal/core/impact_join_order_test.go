package core

import (
	"slices"
	"sort"
	"testing"
)

func queryFirstArgs(t *testing.T, k *RealKernel, pred string) []string {
	t.Helper()
	facts, err := k.Query(pred)
	if err != nil {
		t.Fatalf("Query %s: %v", pred, err)
	}
	var out []string
	for _, f := range facts {
		out = append(out, f.Args[0].(string))
	}
	sort.Strings(out)
	return out
}

// The impact rules lead with the modified file and look its importers up in
// dependency_link (one row per file import edge), never the other way round.
// Body order changes cost, not answers: these are the answers.
func TestImpactRulesFollowImportersOfModifiedFiles(t *testing.T) {
	k := setupMockKernel(t)
	for _, f := range []Fact{
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "p/b"}},
		{Predicate: "dependency_link", Args: []any{"b.go", "c.go", "p/c"}},
		{Predicate: "dependency_link", Args: []any{"z.go", "a.go", "p/a"}},
		// Near miss: an edge into a file nothing modified reaches.
		{Predicate: "dependency_link", Args: []any{"x.go", "y.go", "p/y"}},
		{Predicate: "modified", Args: []any{"c.go"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	for pred, want := range map[string][]string{
		"impacted":         {"a.go", "b.go", "z.go"},
		"coder_impacted_1": {"b.go"},
		"coder_impacted_2": {"a.go"},
		"coder_impacted_3": {"z.go"},
	} {
		if got := queryFirstArgs(t, k, pred); !slices.Equal(got, want) {
			t.Fatalf("%s = %v, want %v", pred, got, want)
		}
	}
}
