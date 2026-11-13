package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/core"
)

// edgeCaseWorkspace writes a module with two packages that each hold a
// types.go, so base-name matching cannot tell their files apart.
func edgeCaseWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/m\n\ngo 1.24\n")
	write("a/types.go", "package a\n")
	write("b/types.go", "package b\n")
	write("c/user.go", "package c\n")
	write("d/other.go", "package d\n")
	return root
}

func loadedKernel(t *testing.T, facts []core.Fact) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	return k
}

func decisionFor(t *testing.T, decisions []FileDecision, path string) FileDecision {
	t.Helper()
	for _, d := range decisions {
		if d.Path == path {
			return d
		}
	}
	t.Fatalf("no decision for %s in %+v", path, decisions)
	return FileDecision{}
}

// A Go import names a package, so the scanner's raw row for it is
// dependency_link(Importer, "pkg:<import path>", _). A Go file whose package
// is imported has dependents; the importer's own package directory does not.
func TestEdgeCaseDetector_GoFileDependentsComeFromPackageImports(t *testing.T) {
	root := edgeCaseWorkspace(t)
	k := loadedKernel(t, []core.Fact{
		{Predicate: "dependency_link", Args: []any{"c/user.go", "pkg:example.com/m/a", "example.com/m/a"}},
		// An external test package in a/ imports a/: its own package, not a dependent.
		{Predicate: "dependency_link", Args: []any{"a/types_test.go", "pkg:example.com/m/a", "example.com/m/a"}},
		{Predicate: "dependency_link", Args: []any{"a/types.go", "pkg:fmt", "fmt"}},
	})
	detector := NewEdgeCaseDetector(k, root)

	target := filepath.Join(root, "a", "types.go")
	decisions, err := detector.AnalyzeFiles(context.Background(), []string{target}, nil)
	if err != nil {
		t.Fatalf("AnalyzeFiles: %v", err)
	}
	got := decisionFor(t, decisions, target)
	if !slices.Equal(got.Dependents, []string{"c/user.go"}) {
		t.Fatalf("dependents = %v, want [c/user.go]", got.Dependents)
	}
	if got.ImpactScore != 1 {
		t.Fatalf("impact = %d, want 1", got.ImpactScore)
	}
	if !slices.Equal(got.Dependencies, []string{"fmt"}) {
		t.Fatalf("dependencies = %v, want [fmt]", got.Dependencies)
	}
}

// Two files named types.go in different packages are different files: each
// has its own dependents and its own complexity. The units are the files'
// packages, as the scanner's file_unit/import_unit rows record them.
func TestEdgeCaseDetector_SameBaseNameFilesDoNotShareDependents(t *testing.T) {
	root := edgeCaseWorkspace(t)
	k := &boundOnlyKernel{}
	k.Facts = []core.Fact{
		{Predicate: "file_unit", Args: []any{"a/types.go", "example.com/m/a"}},
		{Predicate: "file_unit", Args: []any{"b/types.go", "example.com/m/b"}},
		{Predicate: "import_unit", Args: []any{"c/user.go", "example.com/m/a"}},
		{Predicate: "import_unit", Args: []any{"d/other.go", "example.com/m/b"}},
		{Predicate: "cyclomatic_complexity", Args: []any{"a/types.go", "Heavy", int64(14)}},
		{Predicate: "cyclomatic_complexity", Args: []any{"b/types.go", "Light", int64(2)}},
	}
	detector := NewEdgeCaseDetector(k, root)

	absA := filepath.Join(root, "a", "types.go")
	decisions, err := detector.AnalyzeFiles(context.Background(), []string{absA, "b/types.go"}, nil)
	if err != nil {
		t.Fatalf("AnalyzeFiles: %v", err)
	}
	a := decisionFor(t, decisions, absA)
	b := decisionFor(t, decisions, "b/types.go")
	if !slices.Equal(a.Dependents, []string{"c/user.go"}) {
		t.Errorf("a/types.go dependents = %v, want [c/user.go]", a.Dependents)
	}
	if !slices.Equal(b.Dependents, []string{"d/other.go"}) {
		t.Errorf("b/types.go dependents = %v, want [d/other.go]", b.Dependents)
	}
	if a.Complexity != 14 || b.Complexity != 2 {
		t.Errorf("complexity a=%v b=%v, want 14 and 2", a.Complexity, b.Complexity)
	}
}

// boundOnlyKernel answers bound row requests and refuses to hand over a whole
// world-sized relation, which is what every analysis used to copy.
type boundOnlyKernel struct {
	MockKernel
	bound []string
}

func (k *boundOnlyKernel) Query(predicate string) ([]core.Fact, error) {
	switch predicate {
	case "dependency_link", "cyclomatic_complexity", "file_unit", "import_unit":
		return nil, fmt.Errorf("whole relation %s requested", predicate)
	}
	return k.MockKernel.Query(predicate)
}

func (k *boundOnlyKernel) QueryBound(pred string, arity int, bound map[int]any) ([]core.Fact, error) {
	k.bound = append(k.bound, core.BoundQuery(pred, arity, bound))
	// The embedded fake has no index, so BoundRows filters its rows exactly.
	return core.BoundRows(&k.MockKernel, pred, arity, bound)
}

func TestEdgeCaseDetector_AsksOnlyForTheFilesRows(t *testing.T) {
	root := edgeCaseWorkspace(t)
	k := &boundOnlyKernel{}
	k.Facts = []core.Fact{
		{Predicate: "dependency_link", Args: []any{"c/user.go", "pkg:example.com/m/a", "example.com/m/a"}},
		{Predicate: "file_unit", Args: []any{"a/types.go", "example.com/m/a"}},
		{Predicate: "import_unit", Args: []any{"c/user.go", "example.com/m/a"}},
	}
	decisions, err := NewEdgeCaseDetector(k, root).AnalyzeFiles(context.Background(), []string{"a/types.go"}, nil)
	if err != nil {
		t.Fatalf("AnalyzeFiles: %v", err)
	}
	if got := decisionFor(t, decisions, "a/types.go"); got.ImpactScore != 1 {
		t.Fatalf("impact = %d (dependents %v), want 1", got.ImpactScore, got.Dependents)
	}
	want := []string{
		`dependency_link("a/types.go", V1, V2)`,
		`file_unit("a/types.go", V1)`,
		`import_unit(V0, "example.com/m/a")`,
		`dependency_link(V0, "pkg:example.com/m/a", V2)`,
		`cyclomatic_complexity("a/types.go", V1, V2)`,
	}
	if !slices.Equal(k.bound, want) {
		t.Fatalf("bound queries = %q, want %q", k.bound, want)
	}
}
