package world

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

// TestUnwiredFunction_WhenCallGraphIsReal_ShouldFlagOnlyUncalled proves the
// reviewer wiring rule against the producers that actually emit calls.
//
// code_calls is not keyed in the symbol_graph space (func:<name> /
// method:<recv>.<name>). The Go cartographer (goCallRef) emits a bare
// <pkg>.<Name> row for every call and a dual fn: row only for a same-package
// ident or a selector whose receiver type the AST names. A non-aliased
// cross-package call therefore names the callee only as <pkg>.<Name>, which
// is the code_element ref without the fn: prefix. The rule has to see that
// edge or it accuses Used.
func TestUnwiredFunction_WhenCallGraphIsReal_ShouldFlagOnlyUncalled(t *testing.T) {
	root := writeUnwiredFixture(t)
	seed := collectUnwiredFacts(t, root)
	requireUnwiredProducers(t, seed)

	k := seedRealKernel(t, seed)
	assertRows(t, "in_scope", queryRows(t, k, "in_scope"),
		row("in_scope", "p/app.go"),
		row("in_scope", "q/lib.go"),
	)
	assertUnwired(t, queryRows(t, k, "unwired_function"), queryRows(t, k, "is_called"), queryRows(t, k, "raw_finding"),
		"q/lib.go", "p/app.go")

	// An entry point is uncalled by construction. The guard is per file:
	// q/lib.go drops out, p/app.go does not.
	if err := k.Assert(core.Fact{Predicate: "entry_point", Args: []any{"q/lib.go"}}); err != nil {
		t.Fatalf("assert entry_point: %v", err)
	}
	assertRows(t, "unwired_function after entry_point", queryRows(t, k, "unwired_function"),
		row("unwired_function", "fn:p.Caller", "p/app.go"),
	)
	assertRows(t, "UNWIRED_SYMBOL after entry_point", unwiredFindings(queryRows(t, k, "raw_finding")),
		row("raw_finding", "p/app.go", "1", "/warning", "/architecture", "UNWIRED_SYMBOL",
			"Unwired public function detected: fn:p.Caller (no call edge names it)"),
	)
}

// TestUnwiredFunction_WhenSharded_ShouldMatchSingleKernel is the split-join
// guard. Every predicate the rule joins is world-owned together, including
// entry_point, so the negation is not vacuous on the sharded kernel.
func TestUnwiredFunction_WhenSharded_ShouldMatchSingleKernel(t *testing.T) {
	root := writeUnwiredFixture(t)
	seed := collectUnwiredFacts(t, root)
	requireUnwiredProducers(t, seed)
	assertUnwiredParity(t, seed)

	withEntry := append(append([]core.Fact{}, seed...), core.Fact{
		Predicate: "entry_point", Args: []any{"q/lib.go"},
	})
	assertUnwiredParity(t, withEntry)
}

func writeUnwiredFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/unwired\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "q/lib.go", `package q

func Orphan() int { return 1 }

func Used() int { return UsedByLocal() }

func UsedByLocal() int { return 2 }

func hidden() int { return 3 }
`)
	writeWorkspaceFile(t, root, "p/app.go", `package p

import "example.com/unwired/q"

func Caller() int { return q.Used() }
`)
	return root
}

// collectUnwiredFacts runs the scope (which follows p/app.go's import into
// q/lib.go), the cartographer on both files, and the fast scanner's
// file_topology rows. The edit is not simulated: unwired_function reads no
// edit fact.
func collectUnwiredFacts(t *testing.T, root string) []core.Fact {
	t.Helper()
	scope := NewFileScope(root)
	if err := scope.Open(filepath.Join(root, filepath.FromSlash("p/app.go"))); err != nil {
		t.Fatalf("open p/app.go: %v", err)
	}
	facts := scope.ScopeFacts()

	cart := NewCartographer()
	defer cart.Close()
	for _, rel := range []string{"p/app.go", "q/lib.go"} {
		deep, err := cart.MapFileAs(filepath.Join(root, filepath.FromSlash(rel)), rel)
		if err != nil {
			t.Fatalf("map %s: %v", rel, err)
		}
		facts = append(facts, deep...)
	}

	scanned, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, f := range scanned {
		switch f.Predicate {
		case "entry_point":
			t.Fatalf("fixture produced entry_point%v; neither file is an entry point", f.Args)
		case "file_topology":
			facts = append(facts, f)
		}
	}
	return facts
}

// requireUnwiredProducers fails before any derived claim if the producers
// did not emit the edges the negative cases depend on. A missing cross-package
// bare edge would make "Used is not unwired" vacuously true.
func requireUnwiredProducers(t *testing.T, facts []core.Fact) {
	t.Helper()
	for _, want := range []struct {
		pred string
		args []string
	}{
		{"code_element", []string{"fn:q.Orphan", "/function", "q/lib.go"}},
		{"code_element", []string{"fn:q.Used", "/function", "q/lib.go"}},
		{"code_element", []string{"fn:q.UsedByLocal", "/function", "q/lib.go"}},
		{"code_element", []string{"fn:q.hidden", "/function", "q/lib.go"}},
		{"code_element", []string{"fn:p.Caller", "/function", "p/app.go"}},
		{"element_visibility", []string{"fn:q.Orphan", "/public"}},
		{"element_visibility", []string{"fn:q.hidden", "/private"}},
		{"element_visibility", []string{"fn:p.Caller", "/public"}},
		{"file_in_scope", []string{"q/lib.go"}},
		{"file_in_scope", []string{"p/app.go"}},
		{"code_calls", []string{"q.Used", "q.UsedByLocal"}},
		{"code_calls", []string{"fn:q.Used", "fn:q.UsedByLocal"}},
		{"code_calls", []string{"p.Caller", "q.Used"}},
	} {
		if !hasFactPrefix(facts, want.pred, want.args...) {
			t.Fatalf("producer seed missing %s%v", want.pred, want.args)
		}
	}
	// The cross-package call must NOT have an fn: row. That is the edge the
	// bare-to-fn: bridge exists for; a dual row would make the bridge untested.
	if hasFactPrefix(facts, "code_calls", "fn:p.Caller", "fn:q.Used") {
		t.Fatal("cartographer emitted code_calls(fn:p.Caller, fn:q.Used); goCallRef does not spell a package-qualified call as fn:")
	}
	for _, file := range []string{"q/lib.go", "p/app.go"} {
		if !fileTopologyIs(facts, file, "/false") {
			t.Fatalf("file_topology(%s, ..., /false) missing from the scanner seed", file)
		}
	}
}

func hasFactPrefix(facts []core.Fact, pred string, args ...string) bool {
	for _, f := range facts {
		if f.Predicate != pred || len(f.Args) < len(args) {
			continue
		}
		ok := true
		for i, want := range args {
			if fmt.Sprint(f.Args[i]) != want {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func fileTopologyIs(facts []core.Fact, file, isTest string) bool {
	for _, f := range facts {
		if f.Predicate != "file_topology" || len(f.Args) < 5 {
			continue
		}
		if fmt.Sprint(f.Args[0]) == file && fmt.Sprint(f.Args[4]) == isTest {
			return true
		}
	}
	return false
}

func assertUnwired(t *testing.T, unwired, called, findings []core.Fact, orphanFile, callerFile string) {
	t.Helper()
	assertRows(t, "unwired_function", unwired,
		row("unwired_function", "fn:q.Orphan", orphanFile),
		row("unwired_function", "fn:p.Caller", callerFile),
	)
	assertRows(t, "is_called", called,
		row("is_called", "q.Used"),
		row("is_called", "fn:q.Used"),
		row("is_called", "q.UsedByLocal"),
		row("is_called", "fn:q.UsedByLocal"),
	)
	assertRows(t, "UNWIRED_SYMBOL", unwiredFindings(findings),
		row("raw_finding", orphanFile, "1", "/warning", "/architecture", "UNWIRED_SYMBOL",
			"Unwired public function detected: fn:q.Orphan (no call edge names it)"),
		row("raw_finding", callerFile, "1", "/warning", "/architecture", "UNWIRED_SYMBOL",
			"Unwired public function detected: fn:p.Caller (no call edge names it)"),
	)
}

func unwiredFindings(facts []core.Fact) []core.Fact {
	var out []core.Fact
	for _, f := range facts {
		if f.Predicate == "raw_finding" && len(f.Args) >= 5 && fmt.Sprint(f.Args[4]) == "UNWIRED_SYMBOL" {
			out = append(out, f)
		}
	}
	return out
}

// newUnwiredCortex mirrors the world shard's ownership for the predicates
// unwired_function joins. code_defines is left unowned on purpose: production
// does not own it, and the rule must not need it.
func newUnwiredCortex(t *testing.T) *core.CortexKernel {
	t.Helper()
	cortex := core.NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(nil); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	world, err := core.NewKernelShard(core.KernelShardConfig{
		Domain: "world",
		OwnedPredicates: []string{
			"code_element", "element_visibility", "code_calls",
			"file_topology", "entry_point", "active_file", "file_in_scope",
			"in_scope",
		},
	})
	if err != nil {
		t.Fatalf("world shard: %v", err)
	}
	catchAll, err := core.NewKernelShard(core.KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("cortex shard: %v", err)
	}
	for _, s := range []*core.KernelShard{world, catchAll} {
		if err := cortex.RegisterShard(s); err != nil {
			t.Fatalf("register %s: %v", s.Domain(), err)
		}
	}
	return cortex
}

func assertUnwiredParity(t *testing.T, seed []core.Fact) {
	t.Helper()
	single := seedRealKernel(t, seed)
	sharded := newUnwiredCortex(t)
	for _, f := range seed {
		if err := sharded.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	if err := sharded.Evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	for _, pred := range []string{"unwired_function", "is_called"} {
		want, err := single.Query(pred)
		if err != nil {
			t.Fatalf("single query %s: %v", pred, err)
		}
		got, err := sharded.Query(pred)
		if err != nil {
			t.Fatalf("sharded query %s: %v", pred, err)
		}
		wantSet, gotSet := rowSet(want), rowSet(got)
		if len(wantSet) != len(gotSet) {
			t.Errorf("%s: single derived %d rows %q, sharded derived %d rows %q",
				pred, len(wantSet), sortedRowKeys(wantSet), len(gotSet), sortedRowKeys(gotSet))
			continue
		}
		for k := range wantSet {
			if _, ok := gotSet[k]; !ok {
				t.Errorf("%s: sharded kernel missing row %q", pred, k)
			}
		}
	}
	wantFindings := unwiredFindings(queryRows(t, single, "raw_finding"))
	gotFindings, err := sharded.Query("raw_finding")
	if err != nil {
		t.Fatalf("sharded raw_finding: %v", err)
	}
	assertRows(t, "sharded UNWIRED_SYMBOL", unwiredFindings(gotFindings), findingKeys(wantFindings)...)
	if rows, _ := single.Query("unwired_function"); len(rows) == 0 {
		t.Fatal("single kernel derived no unwired_function rows; the parity check is vacuous")
	}
}

func findingKeys(facts []core.Fact) []string {
	keys := make([]string, 0, len(facts))
	for _, f := range facts {
		parts := make([]string, 0, len(f.Args)+1)
		parts = append(parts, f.Predicate)
		for _, a := range f.Args {
			parts = append(parts, fmt.Sprint(a))
		}
		keys = append(keys, joinRow(parts))
	}
	return keys
}

func joinRow(parts []string) string {
	return row(parts[0], parts[1:]...)
}
