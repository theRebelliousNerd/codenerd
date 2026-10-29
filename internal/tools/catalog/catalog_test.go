package catalog_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/tools/catalog"
	"codenerd/internal/types"
)

// conditionalProbeSearcher reports support so RegisterGroundedWebSearch
// installs its tool. Execute is never reached.
type conditionalProbeSearcher struct{}

func (conditionalProbeSearcher) SupportsGroundedWebSearch() bool { return true }

func (conditionalProbeSearcher) GroundedWebSearch(context.Context, string) (*types.GroundedWebSearchResult, error) {
	return nil, fmt.Errorf("catalog probe does not search")
}

func TestNames_StableSortedUnique(t *testing.T) {
	got, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names: %v", err)
	}
	again, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names again: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("catalog.Names returned no tools")
	}
	if diff := setDiff(got, again); diff != "" {
		t.Fatalf("catalog.Names is not stable: %s", diff)
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	for i := range got {
		if got[i] != sorted[i] {
			t.Fatalf("catalog.Names is not sorted at %d: got %q, sorted %q", i, got[i], sorted[i])
		}
		if i > 0 && got[i] == got[i-1] {
			t.Fatalf("duplicate name %q", got[i])
		}
	}
}

// TestConditionalTool_StaysOutOfFamilies pins the split Names cannot pin
// by unioning the two: the tool RegisterGroundedWebSearch installs is in
// Names, and no Family's RegisterAll installs it. Equality with a real
// HydrateModularTools is internal/core/virtual_store_tools_test.go, because
// this package must not import internal/core.
func TestConditionalTool_StaysOutOfFamilies(t *testing.T) {
	probe := tools.NewRegistry()
	ok, err := catalog.RegisterGroundedWebSearch(probe, conditionalProbeSearcher{})
	if err != nil {
		t.Fatalf("RegisterGroundedWebSearch: %v", err)
	}
	if !ok {
		t.Fatal("RegisterGroundedWebSearch skipped a supporting searcher")
	}
	installed := probe.Names()
	if len(installed) != 1 || installed[0] == "" {
		t.Fatalf("conditional registrar installed %v, want one named tool", installed)
	}
	name := installed[0]

	skipped := tools.NewRegistry()
	ok, err = catalog.RegisterGroundedWebSearch(skipped, nil)
	if err != nil {
		t.Fatalf("RegisterGroundedWebSearch(nil): %v", err)
	}
	if ok || skipped.Has(name) {
		t.Fatalf("nil searcher installed %q", name)
	}

	families := catalog.Families()
	if len(families) == 0 {
		t.Fatal("catalog.Families is empty")
	}
	seen := make(map[string]struct{}, len(families))
	for _, family := range families {
		if family.Name == "" || family.Register == nil {
			t.Fatalf("family %+v is missing a name or registrar", family)
		}
		if _, dup := seen[family.Name]; dup {
			t.Fatalf("duplicate family name %q", family.Name)
		}
		seen[family.Name] = struct{}{}
		reg := tools.NewRegistry()
		if err := family.Register(reg); err != nil {
			t.Fatalf("%s.RegisterAll: %v", family.Name, err)
		}
		if reg.Count() == 0 {
			t.Errorf("%s.RegisterAll registered no tools", family.Name)
		}
		if reg.Has(name) {
			t.Errorf("family %s registers conditional tool %q; HydrateModularTools would install it with no searcher", family.Name, name)
		}
	}

	names, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names: %v", err)
	}
	if !contains(names, name) {
		t.Fatalf("catalog.Names omits conditional tool %q", name)
	}
}

func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func setDiff(got, want []string) string {
	gm := make(map[string]struct{}, len(got))
	for _, n := range got {
		if _, dup := gm[n]; dup {
			return "duplicate name " + n
		}
		gm[n] = struct{}{}
	}
	wm := make(map[string]struct{}, len(want))
	for _, n := range want {
		wm[n] = struct{}{}
	}
	var missing, extra []string
	for _, n := range want {
		if _, ok := gm[n]; !ok {
			missing = append(missing, n)
		}
	}
	for _, n := range got {
		if _, ok := wm[n]; !ok {
			extra = append(extra, n)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}
	return "missing [" + strings.Join(missing, ", ") + "]; extra [" + strings.Join(extra, ", ") + "]"
}
