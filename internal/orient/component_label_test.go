package orient

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// read_component_ord is a merge lattice: each path holds exactly one label,
// the smallest ordinal in its read component, even when that minimum sits
// behind a larger intermediate. A separate component keeps its own minimum.
func TestPolicy_ComponentLabelIsTheComponentMinimum(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultOrientConfig()
	facts := []types.Fact{spanFact(0, 1000, 20, "/no")}
	ords := map[string]int{"a.md": 3, "b.md": 5, "c.md": 1, "d.md": 4, "e.md": 2, "f.md": 6}
	for p, o := range ords {
		facts = append(facts, docFact(p, ""), histFact(p, 500, 500, 6, 1), tieFact(p, o))
	}
	// a - b - c - d in one component (c holds its minimum, behind b), e - f
	// in another.
	for _, pair := range [][2]string{{"a.md", "b.md"}, {"b.md", "c.md"}, {"c.md", "d.md"}, {"e.md", "f.md"}} {
		facts = append(facts, F("doc_similar", pair[0], pair[1], int64(975)))
	}
	e := evalFacts(t, &cfg, facts)
	rows, err := e.Query("read_component_ord")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"a.md": 1, "b.md": 1, "c.md": 1, "d.md": 1, "e.md": 2, "f.md": 2}
	if len(rows) != len(want) {
		t.Fatalf("read_component_ord = %v, want one row per linked path", rows)
	}
	for _, r := range rows {
		path, _ := r.Args[0].(string)
		if got, _ := r.Args[1].(int64); got != want[path] {
			t.Fatalf("read_component_ord(%s) = %d, want %d (rows %v)", path, got, want[path], rows)
		}
	}
	if predCount(t, e, "read_component") != len(want) {
		t.Fatalf("read_component = %d rows, want %d", predCount(t, e, "read_component"), len(want))
	}
}
