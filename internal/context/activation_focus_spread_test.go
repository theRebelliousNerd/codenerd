package context

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// focusSpreadWalk is the walk computeDependencyScore used to make for every
// fact that named a focused path: the reference focusSpreadFor is held to.
func focusSpreadWalk(graph map[string][]string, focusPath string) float64 {
	spread := 0.0
	for symbol, callees := range graph {
		if strings.Contains(symbol, focusPath) {
			spread += float64(len(callees)) * 2.0
		}
		for _, callee := range callees {
			if strings.Contains(callee, focusPath) {
				spread += 3.0
			}
		}
	}
	return spread
}

// The symbol-graph share of a fact depends on the focused path and the graph,
// so it is computed once per path per graph build -- and a rebuilt graph must
// not answer with the previous one's figure.
func TestFocusSpread_IsTheGraphWalkAndFollowsTheGraph(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	first := []core.Fact{
		// The import graph in factors: handler.go imports the auth unit,
		// whose member is pkg/auth/auth.go.
		{Predicate: "import_unit", Args: []any{"handler.go", "m/pkg/auth"}},
		{Predicate: "file_unit", Args: []any{"pkg/auth/auth.go", "m/pkg/auth"}},
		{Predicate: "import_unit", Args: []any{"main.go", "m/http"}},
		{Predicate: "file_unit", Args: []any{"handler.go", "m/http"}},
		{Predicate: "dependency_link", Args: []any{"handler.go", "pkg:fmt", "fmt"}},
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.buildSymbolGraphLocked(first)

	if got := engine.symbolGraph["m/pkg/auth"]; len(got) != 1 || got[0] != "pkg/auth/auth.go" {
		t.Fatalf("the unit does not reach its member: %v", got)
	}
	if got := engine.symbolGraph["handler.go"]; len(got) != 2 {
		t.Fatalf("handler.go has %d outgoing edges, want its unit import and its raw import: %v", len(got), got)
	}
	for _, path := range []string{"handler.go", "pkg/auth/auth.go", "absent.go"} {
		want := focusSpreadWalk(engine.symbolGraph, path)
		if got := engine.focusSpreadFor(path); got != want {
			t.Errorf("focusSpreadFor(%q) = %v, the walk gives %v", path, got, want)
		}
		if got := engine.focusSpreadFor(path); got != want {
			t.Errorf("second focusSpreadFor(%q) = %v, want %v", path, got, want)
		}
	}
	if engine.focusSpreadFor("handler.go") == 0 {
		t.Fatal("a focused importer scored nothing; the test would pass on an empty graph")
	}

	engine.buildSymbolGraphLocked(first[:2])
	if got, want := engine.focusSpreadFor("handler.go"), focusSpreadWalk(engine.symbolGraph, "handler.go"); got != want {
		t.Errorf("after a rebuild focusSpreadFor = %v, the walk gives %v: a stale figure survived", got, want)
	}
}
