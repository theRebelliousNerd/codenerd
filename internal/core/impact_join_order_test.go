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
// the import graph (file_unit of the modified file, then import_unit), never
// the other way round.
// Body order changes cost, not answers: these are the answers.
func TestImpactRulesFollowImportersOfModifiedFiles(t *testing.T) {
	k := setupMockKernel(t)
	imports := func(from, to string) []Fact {
		return []Fact{
			{Predicate: "import_unit", Args: []any{from, "unit:" + to}},
			{Predicate: "file_unit", Args: []any{to, "unit:" + to}},
		}
	}
	var facts []Fact
	for _, e := range [][2]string{
		{"a.go", "b.go"}, {"b.go", "c.go"}, {"z.go", "a.go"},
		// Near miss: an edge into a file nothing modified reaches.
		{"x.go", "y.go"},
	} {
		facts = append(facts, imports(e[0], e[1])...)
	}
	facts = append(facts, Fact{Predicate: "modified", Args: []any{"c.go"}})
	for _, f := range facts {
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
