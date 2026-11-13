package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A focal file's neighbourhood is selected file by file only where it is small
// enough to be a selection (context_neighbour_limit,
// policy/context_compilation.mg). Measured on this repository before the limit
// (TestRealWorldProfile, 2026-11-13): one modified file of a hub package
// selected 2,051 files, and a coder target in it 693.
func TestDomainCortex_AWideNeighbourhoodIsSelectedAsItsUnit(t *testing.T) {
	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	facts := []core.Fact{
		{Predicate: "modified", Args: []any{"app/main.go"}},
		{Predicate: "file_unit", Args: []any{"app/main.go", "m/app"}},
		{Predicate: "import_unit", Args: []any{"app/main.go", "m/small"}},
		{Predicate: "import_unit", Args: []any{"app/main.go", "m/big"}},
		{Predicate: "file_unit", Args: []any{"small/a.go", "m/small"}},
		{Predicate: "file_unit", Args: []any{"small/b.go", "m/small"}},
		// Two files import the focal file's unit: few.
		{Predicate: "import_unit", Args: []any{"cmd/x.go", "m/app"}},
		{Predicate: "import_unit", Args: []any{"cmd/y.go", "m/app"}},
		// A second focal file, in a unit a dozen files import: wide.
		{Predicate: "modified", Args: []any{"hub/h.go"}},
		{Predicate: "file_unit", Args: []any{"hub/h.go", "m/hub"}},
	}
	for i := range 12 {
		facts = append(facts,
			core.Fact{Predicate: "file_unit", Args: []any{fmt.Sprintf("big/f%02d.go", i), "m/big"}},
			core.Fact{Predicate: "import_unit", Args: []any{fmt.Sprintf("user/u%02d.go", i), "m/hub"}},
		)
	}
	if err := ck.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	rows, err := ck.Query("should_include_context")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	selected := map[string]bool{}
	for _, r := range rows {
		if len(r.Args) == 2 {
			selected[types.ExtractString(r.Args[0])] = true
		}
	}
	for _, want := range []string{
		"app/main.go", "hub/h.go", // the focal files
		"small/a.go", "small/b.go", // a small imported unit, file by file
		"cmd/x.go", "cmd/y.go", // few importers, file by file
		"m/big", // a wide imported unit, as itself
	} {
		if !selected[want] {
			t.Errorf("should_include_context lacks %q; selected: %v", want, selected)
		}
	}
	for i := range 12 {
		for _, absent := range []string{fmt.Sprintf("big/f%02d.go", i), fmt.Sprintf("user/u%02d.go", i)} {
			if selected[absent] {
				t.Errorf("should_include_context names %q: a wide neighbourhood was selected file by file", absent)
			}
		}
	}
	if selected["m/small"] {
		t.Error("a small unit was selected as itself as well as file by file")
	}
	// What the window says of the wide unit: its size and its importers.
	for query, want := range map[string]int64{
		`context_unit_size("m/big", N)`:               12,
		`intelligence_unit_importers("m/hub", N)`:     12,
		`intelligence_dependent_count("hub/h.go", N)`: 12,
	} {
		got, err := ck.Query(query)
		if err != nil || len(got) != 1 || got[0].Args[1] != want {
			t.Errorf("%s = %v (err=%v), want %d", query, got, err, want)
		}
	}
	// The impact relations are not selection and are not limited.
	impacted, err := ck.Query("impacted")
	if err != nil || len(impacted) != 14 {
		t.Errorf("impacted has %d rows (err=%v), want every importer of both modified files (14)", len(impacted), err)
	}
}
