package world

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

// A call edge belongs to a file when its caller or its callee IS one of the
// file's symbols. The walk used strings.Contains, so core.Kernel.Query's file
// was also credited with every edge of core.Kernel.QueryContext and
// core.Kernel.QueryAll: the counts the prompt's remainder is computed from
// were inflated for any symbol that prefixes another.
func TestHolographicCallGraph_CountsOnlyEdgesOfTheFilesOwnSymbols(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "query.go")
	if err := os.WriteFile(target, []byte("package core\n\nfunc Query() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.go")
	q := &stubQuerier{facts: map[string][]core.Fact{
		"code_defines": {
			{Predicate: "code_defines", Args: []any{target, "core.Kernel.Query", "/function", int64(3), int64(3)}},
			{Predicate: "code_defines", Args: []any{other, "core.Kernel.QueryContext", "/function", int64(3), int64(9)}},
		},
		"code_calls": {
			{Predicate: "code_calls", Args: []any{"core.Kernel.Query", "k.QueryContext"}},                 // its own call
			{Predicate: "code_calls", Args: []any{"fn:core.Kernel.Query", "fn:core.Kernel.QueryContext"}}, // the dual row
			{Predicate: "code_calls", Args: []any{"chat.Model.ask", "core.Kernel.Query"}},                 // a caller of it
			{Predicate: "code_calls", Args: []any{"core.Kernel.QueryContext", "core.parse"}},              // another function's
			{Predicate: "code_calls", Args: []any{"core.Kernel.QueryAll", "core.Kernel.QueryContext"}},    // nothing of this file's
		},
	}}
	hc, err := NewHolographicProvider(q, dir).GetContext(target)
	if err != nil {
		t.Fatal(err)
	}
	if hc.CallGraphEdges != 2 || hc.CallerCount != 2 {
		t.Fatalf("edges = %d, callers = %d; want the file's own 2 and 2. graph: %+v", hc.CallGraphEdges, hc.CallerCount, hc.CallGraph)
	}
	for _, e := range hc.CallGraph {
		if e.Caller != "core.Kernel.Query" && e.Callee != "core.Kernel.Query" {
			t.Fatalf("an edge of another function was credited to this file: %+v", e)
		}
	}
}

// The importers of a file's package are the rows of dependency_link that name
// it, whatever else the relation holds.
func TestHolographicDirectImporters_AsksOnlyForThePackagesRows(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "lib", "lib.go")
	if err := os.WriteFile(target, []byte("package lib\n\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	asked := &recordingQuerier{stubQuerier: stubQuerier{facts: map[string][]core.Fact{
		"dependency_link": {
			{Predicate: "dependency_link", Args: []any{"cmd/a/main.go", "pkg:example.test/m/lib", "example.test/m/lib"}},
			{Predicate: "dependency_link", Args: []any{"cmd/b/main.go", "pkg:example.test/m/lib", "example.test/m/lib"}},
			{Predicate: "dependency_link", Args: []any{"lib/other.go", "pkg:example.test/m/lib", "example.test/m/lib"}},
			{Predicate: "dependency_link", Args: []any{"cmd/a/main.go", "pkg:fmt", "fmt"}},
		},
	}}}
	hc, err := NewHolographicProvider(asked, dir).GetContext(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(hc.DirectImporters) != 2 || hc.DirectImporters[0] != "cmd/a/main.go" || hc.DirectImporters[1] != "cmd/b/main.go" {
		t.Fatalf("importers = %v, want the two files outside the package", hc.DirectImporters)
	}
	for _, q := range asked.queries {
		if q == "dependency_link" || q == "code_calls" || q == "code_defines" {
			t.Fatalf("the provider copied a whole relation: asked %q", q)
		}
	}
}

// recordingQuerier has an argument index, as the kernels do, and records what
// it was asked for whole.
type recordingQuerier struct {
	stubQuerier
	queries []string
}

func (r *recordingQuerier) Query(pred string) ([]core.Fact, error) {
	r.queries = append(r.queries, pred)
	return r.stubQuerier.Query(pred)
}

func (r *recordingQuerier) QueryBound(pred string, arity int, bound map[int]any) ([]core.Fact, error) {
	return core.BoundRows(&r.stubQuerier, pred, arity, bound)
}
