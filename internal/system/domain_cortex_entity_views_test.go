package system

import (
	"context"
	"strings"
	"testing"

	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/store"
)

// What the window shows of a selected file is the kernel's decision
// (context_view, context_focal; policy/context_compilation.mg), carried out
// against the session's whole kernel.
//
// The compressor is built on the catch-all shard's kernel, as a chat session
// builds it, and reads its decisions through the Cortex. Before entity views a
// selected file was resolved to the facts that mention it in the catch-all,
// which holds none of the world model: the block for a modified file carried
// no symbol and no import of it.
func TestDomainCortex_ASelectedFileIsShownThroughItsViews(t *testing.T) {
	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	atom := func(s string) core.MangleAtom { return core.MangleAtom(s) }
	facts := []core.Fact{
		// The file being worked on, and what the world model knows of it.
		{Predicate: "modified", Args: []any{"pkg/a.go"}},
		{Predicate: "symbol_graph", Args: []any{"func:pkg.Run", atom("/function"), atom("/public"), "pkg/a.go", "func Run() error"}},
		{Predicate: "file_unit", Args: []any{"pkg/a.go", "m/pkg"}},
		{Predicate: "import_unit", Args: []any{"pkg/a.go", "m/lib"}},
		// Its neighbour: selected because the modified file imports it.
		{Predicate: "file_unit", Args: []any{"lib/l.go", "m/lib"}},
		{Predicate: "symbol_graph", Args: []any{"func:lib.Help", atom("/function"), atom("/public"), "lib/l.go", "func Help()"}},
		{Predicate: "import_unit", Args: []any{"lib/l.go", "m/deep"}},
		// A file nothing selected.
		{Predicate: "symbol_graph", Args: []any{"func:far.Away", atom("/function"), atom("/public"), "far/f.go", "func Away()"}},
		{Predicate: "file_unit", Args: []any{"far/f.go", "m/far"}},
	}
	if err := ck.AssertBatch(facts); err != nil {
		t.Fatalf("assert world facts: %v", err)
	}
	if rows, err := ck.GetPrimaryRealKernel().Query("symbol_graph"); err != nil || len(rows) != 0 {
		t.Fatalf("the catch-all holds symbol_graph rows (%v, err=%v); this test no longer shows a cross-shard read", rows, err)
	}

	db, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer db.Close()
	comp := ctxcompress.NewCompressor(ck.GetPrimaryRealKernel(), db, &MockLLMClient{})
	comp.SetKernelReader(ck)

	built, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	atoms := built.ContextAtoms
	for _, want := range []string{
		`symbol_graph("func:pkg.Run"`, // the focal file's surface
		`import_unit("pkg/a.go", "m/lib")`,
		`file_unit("pkg/a.go", "m/pkg")`,
		`file_unit("lib/l.go", "m/lib")`, // the neighbour's compact view
	} {
		if !strings.Contains(atoms, want) {
			t.Errorf("active context lacks %s\n%s", want, atoms)
		}
	}
	for _, absent := range []string{
		`func:lib.Help`,          // a neighbour is not shown through a focal view
		`import_unit("lib/l.go"`, // nor what it reaches
		`far/f.go`,               // an unselected file is not shown at all
	} {
		if strings.Contains(atoms, absent) {
			t.Errorf("active context carries %s, which no view of a selected entity shows\n%s", absent, atoms)
		}
	}
	// Compact views of every entity come before the bulky view of any.
	if neighbour, surface := strings.Index(atoms, `file_unit("lib/l.go"`), strings.Index(atoms, `symbol_graph("func:pkg.Run"`); neighbour < 0 || surface < 0 || neighbour > surface {
		t.Errorf("the neighbour's rank-2 view (at %d) does not precede the focal file's rank-4 surface (at %d)\n%s", neighbour, surface, atoms)
	}

	// A view is policy: adding one changes the block with no Go change.
	ck.AppendPolicy("context_view(/is_test_file, 1, 0, 2, /any).\n")
	if err := ck.Assert(core.Fact{Predicate: "is_test_file", Args: []any{"lib/l.go"}}); err != nil {
		t.Fatalf("assert is_test_file: %v", err)
	}
	built, err = comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext after the policy grew: %v", err)
	}
	if !strings.Contains(built.ContextAtoms, `is_test_file("lib/l.go")`) {
		t.Errorf("a view added in policy did not reach the block\n%s", built.ContextAtoms)
	}
}
