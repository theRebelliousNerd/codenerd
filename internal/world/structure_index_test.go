package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeStructFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return path
}

func newStructFixture(t *testing.T) (string, *StructureIndex) {
	t.Helper()
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "lib/lib.go", `package lib

// Engine runs things.
type Engine struct{}

// Run runs the engine.
func (e *Engine) Run() error { return helper() }

func helper() error { return nil }

// Orphan is declared and never named again.
func Orphan() {}

// Build makes an engine.
func Build() *Engine { return &Engine{} }
`)
	writeStructFile(t, root, "app/app.go", `package app

import "fixture/lib"

func Start() error {
	e := lib.Build()
	return e.Run()
}
`)
	return root, NewStructureIndex(root)
}

func TestStructureIndexFindSymbolForms(t *testing.T) {
	_, idx := newStructFixture(t)
	ctx := context.Background()
	for _, query := range []string{"Run", "Engine.Run", "lib.Engine.Run"} {
		got, _, err := idx.FindSymbol(ctx, query, "")
		if err != nil {
			t.Fatalf("FindSymbol(%q): %v", query, err)
		}
		if len(got) != 1 || got[0].ID != "lib.Engine.Run" || got[0].File != "lib/lib.go" || got[0].StartLine != 7 {
			t.Fatalf("FindSymbol(%q) = %+v, want lib.Engine.Run at lib/lib.go:7", query, got)
		}
		if got[0].Kind != "method" || got[0].Signature != "func (*Engine) Run() error" {
			t.Fatalf("FindSymbol(%q) kind/signature = %q / %q", query, got[0].Kind, got[0].Signature)
		}
	}
	structs, _, err := idx.FindSymbol(ctx, "Engine", "struct")
	if err != nil || len(structs) != 1 || structs[0].Doc != "Engine runs things." {
		t.Fatalf("FindSymbol(Engine, struct) = %+v, err=%v", structs, err)
	}
}

func TestStructureIndexOutlineIsPerDirectory(t *testing.T) {
	_, idx := newStructFixture(t)
	got, _, err := idx.Outline(context.Background(), "lib")
	if err != nil {
		t.Fatalf("Outline: %v", err)
	}
	var ids []string
	for _, sym := range got {
		ids = append(ids, sym.ID)
	}
	want := []string{"lib.Engine", "lib.Engine.Run", "lib.helper", "lib.Orphan", "lib.Build"}
	if len(ids) != len(want) {
		t.Fatalf("Outline(lib) = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("Outline(lib) = %v, want %v in line order", ids, want)
		}
	}
}

func TestStructureIndexCallersAcrossPackages(t *testing.T) {
	_, idx := newStructFixture(t)
	ctx := context.Background()

	_, callers, _, err := idx.Callers(ctx, "lib.Build")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(callers) != 1 || callers[0].Caller != "app.Start" || callers[0].File != "app/app.go" || callers[0].Line != 6 || callers[0].Match != "exact" {
		t.Fatalf("Callers(lib.Build) = %+v, want one exact site at app/app.go:6 in app.Start", callers)
	}

	// A method called through a variable cannot be tied to its receiver type
	// without type checking; the index says so instead of guessing.
	_, viaVar, _, err := idx.Callers(ctx, "Engine.Run")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(viaVar) != 1 || viaVar[0].Line != 7 || viaVar[0].Match != "by-name" {
		t.Fatalf("Callers(Engine.Run) = %+v, want one by-name site at line 7", viaVar)
	}

	_, local, _, err := idx.Callers(ctx, "helper")
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(local) != 1 || local[0].Caller != "lib.Engine.Run" || local[0].Match != "exact" {
		t.Fatalf("Callers(helper) = %+v, want the same-package bare call from lib.Engine.Run", local)
	}
}

func TestStructureIndexCalleesResolveCandidates(t *testing.T) {
	_, idx := newStructFixture(t)
	_, callees, _, err := idx.Callees(context.Background(), "app.Start")
	if err != nil {
		t.Fatalf("Callees: %v", err)
	}
	if len(callees) != 2 {
		t.Fatalf("Callees(app.Start) = %+v, want two calls", callees)
	}
	if callees[0].Call != "lib.Build" || len(callees[0].Candidates) != 1 || callees[0].Candidates[0] != "lib.Build @ lib/lib.go:15" {
		t.Fatalf("first callee = %+v, want lib.Build resolved to lib/lib.go:15", callees[0])
	}
}

func TestStructureIndexUnreferencedIsConservative(t *testing.T) {
	_, idx := newStructFixture(t)
	got, _, err := idx.Unreferenced(context.Background(), "lib")
	if err != nil {
		t.Fatalf("Unreferenced: %v", err)
	}
	if len(got) != 1 || got[0].ID != "lib.Orphan" {
		t.Fatalf("Unreferenced(lib) = %+v, want exactly lib.Orphan", got)
	}
}

// TestStructureIndexFollowsEdits pins freshness: an answer after an edit
// describes the file as it is, and only the edited file is parsed again.
func TestStructureIndexFollowsEdits(t *testing.T) {
	root, idx := newStructFixture(t)
	ctx := context.Background()
	if _, _, err := idx.FindSymbol(ctx, "Orphan", ""); err != nil {
		t.Fatalf("warm: %v", err)
	}

	path := writeStructFile(t, root, "lib/extra.go", "package lib\n\nfunc Later() { Orphan() }\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	found, stats, err := idx.FindSymbol(ctx, "Later", "")
	if err != nil {
		t.Fatalf("FindSymbol: %v", err)
	}
	if len(found) != 1 || found[0].File != "lib/extra.go" {
		t.Fatalf("FindSymbol(Later) = %+v, want the new file", found)
	}
	if stats.Reparsed != 1 {
		t.Fatalf("reparsed %d files after adding one, want 1", stats.Reparsed)
	}
	unref, _, err := idx.Unreferenced(ctx, "lib")
	if err != nil {
		t.Fatalf("Unreferenced: %v", err)
	}
	for _, sym := range unref {
		if sym.ID == "lib.Orphan" {
			t.Fatalf("lib.Orphan is still reported unreferenced after lib.Later started calling it")
		}
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	gone, _, err := idx.FindSymbol(ctx, "Later", "")
	if err != nil || len(gone) != 0 {
		t.Fatalf("FindSymbol(Later) after delete = %+v, err=%v; want nothing", gone, err)
	}
}

// TestStructureIndexRepositoryScale measures the real tree: the cold build and
// the warm refresh every query pays. It reports; the only assertions are the
// ones that would mean the index is unusable.
func TestStructureIndexRepositoryScale(t *testing.T) {
	if testing.Short() {
		t.Skip("repository-scale measurement")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	idx := NewStructureIndex(root)
	ctx := context.Background()
	cold, err := idx.Refresh(ctx)
	if err != nil {
		t.Fatalf("cold refresh: %v", err)
	}
	warm, err := idx.Refresh(ctx)
	if err != nil {
		t.Fatalf("warm refresh: %v", err)
	}
	t.Logf("cold: files=%d symbols=%d calls=%d in %v; warm: reparsed=%d in %v",
		cold.Files, cold.Symbols, cold.Calls, cold.RefreshTook, warm.Reparsed, warm.RefreshTook)
	if warm.Reparsed != 0 {
		t.Fatalf("warm refresh reparsed %d files with nothing changed", warm.Reparsed)
	}
	found, _, err := idx.FindSymbol(ctx, "StructureIndex.FindSymbol", "")
	if err != nil || len(found) != 1 || found[0].File != "internal/world/structure_index.go" {
		t.Fatalf("the index cannot find its own method: %+v err=%v", found, err)
	}
}
