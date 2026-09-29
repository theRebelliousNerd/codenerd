package core

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

// hydrateCatalogSearcher makes RegisterGroundedWebSearch install its tool.
// The tool is not executed.
type hydrateCatalogSearcher struct{}

func (hydrateCatalogSearcher) SupportsGroundedWebSearch() bool { return true }

func (hydrateCatalogSearcher) GroundedWebSearch(context.Context, string) (*types.GroundedWebSearchResult, error) {
	return nil, fmt.Errorf("hydrate catalog probe does not search")
}

// TestHydrateModularTools_MatchesCatalogNames is the check catalog.Names
// cannot make for itself. Names is built by walking catalog.Families and
// RegisterGroundedWebSearch; this runs the production installer on a fresh
// VirtualStore and requires the same set, including the conditional tool
// a supporting searcher installs.
func TestHydrateModularTools_MatchesCatalogNames(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	if err := vs.HydrateModularTools(hydrateCatalogSearcher{}); err != nil {
		t.Fatalf("HydrateModularTools: %v", err)
	}
	local := vs.GetModularTools()
	if local == nil {
		t.Fatal("GetModularTools() = nil")
	}

	want, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names: %v", err)
	}
	got := local.Names()
	if diff := hydrateNameDiff(got, want); diff != "" {
		t.Fatalf("HydrateModularTools names != catalog.Names: %s", diff)
	}

	probe := tools.NewRegistry()
	ok, err := catalog.RegisterGroundedWebSearch(probe, hydrateCatalogSearcher{})
	if err != nil {
		t.Fatalf("RegisterGroundedWebSearch: %v", err)
	}
	if !ok || len(probe.Names()) != 1 {
		t.Fatalf("conditional registrar installed %v (ok=%v), want one tool", probe.Names(), ok)
	}
	if !local.Has(probe.Names()[0]) {
		t.Fatalf("supporting searcher did not install %q", probe.Names()[0])
	}
}

func hydrateNameDiff(got, want []string) string {
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
