package world

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The test-to-code chain (test_impact.mg) derives test_depends_on,
// impacted_test, coverage_gap and test_priority from the facts the world
// scanners produce. Until W1 it derived nothing: is_test_function had no
// producer, and code_calls used a bare spelling no code_element join could
// meet. W1 proved the call rules from real producer output with the file
// facts (is_test_file, file_imports, file_package) simulated;
// W3 gave those their producers in the fast scanner, and these tests now
// take every input except plan_edit from real producer output — not
// hand-spelled facts that are self-consistent and wrong (see
// TestImpactChain_EndToEndThroughVirtualStore for why that distinction
// matters).

const (
	impactSrcFile  = "p/app.go"
	impactTestFile = "p/app_test.go"
)

// writeTestImpactFixture lays out a two-file Go package. app.go holds a
// struct with a method plus two public functions; app_test.go holds two
// tests (one calling Target, one calling the method) and a non-Test helper
// that must never be marked.
func writeTestImpactFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/impact\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, impactSrcFile,
		"package p\n\ntype S struct{ N int }\n\nfunc (s *S) M() int { return s.N }\n\nfunc Target() int { return 1 }\n\nfunc Helper() int { return 2 }\n")
	writeWorkspaceFile(t, root, impactTestFile,
		"package p\n\nimport \"testing\"\n\nfunc TestTarget(t *testing.T) {\n\tif Target() != 1 {\n\t\tt.Fail()\n\t}\n}\n\nfunc TestMethod(t *testing.T) {\n\ts := &S{}\n\tif s.M() != 0 {\n\t\tt.Fail()\n\t}\n}\n\nfunc helperInTest(t *testing.T) { t.Helper() }\n")
	return root
}

// collectProducerFacts runs the scope and deep producers over the fixture:
// FileScope for the CodeDOM layer (code_element + element_* +
// is_test_function + language facts) and the Cartographer for the deep
// layer (code_defines + code_calls). Both label facts with the canonical
// file identity, so the joins below prove the production spelling.
func collectProducerFacts(t *testing.T, root string) []core.Fact {
	t.Helper()
	scope := NewFileScope(root)
	if err := scope.Open(filepath.Join(root, impactTestFile)); err != nil {
		t.Fatalf("open test file: %v", err)
	}
	facts := scope.ScopeFacts()
	cart := NewCartographer()
	defer cart.Close()
	for _, rel := range []string{impactSrcFile, impactTestFile} {
		deep, err := cart.MapFileAs(filepath.Join(root, filepath.FromSlash(rel)), rel)
		if err != nil {
			t.Fatalf("map %s: %v", rel, err)
		}
		facts = append(facts, deep...)
	}
	return facts
}

// collectScannerFileFacts runs the production fast scanner over the fixture
// and keeps the test-impact file facts: file_package, is_test_file,
// file_imports and the file_dir companion the same-directory joins group
// on. The filter keeps each scenario's seed focused; every kept row is real
// producer output, spelled exactly as the .mg joins read it.
func collectScannerFileFacts(t *testing.T, root string) []core.Fact {
	t.Helper()
	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	var out []core.Fact
	for _, f := range facts {
		switch f.Predicate {
		case "file_package", "is_test_file", "file_imports", "file_dir":
			out = append(out, f)
		}
	}
	return out
}

// collectTestImpactFacts is the whole production seed: scope and deep facts
// plus the fast scanner's file facts.
func collectTestImpactFacts(t *testing.T, root string) []core.Fact {
	t.Helper()
	return append(collectProducerFacts(t, root), collectScannerFileFacts(t, root)...)
}

func seedRealKernel(t *testing.T, facts []core.Fact) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	return k
}

func queryRows(t *testing.T, k *core.RealKernel, pred string) []core.Fact {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	return rows
}

// row encodes one expected derived row for set comparison. fmt.Sprint
// renders a queried argument however the kernel typed it (string or atom),
// so the expectation is about the value, not the Go type.
func row(pred string, args ...string) string {
	return pred + "\x00" + strings.Join(args, "\x00")
}

func rowSet(facts []core.Fact) map[string]struct{} {
	out := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		parts := make([]string, 0, len(f.Args)+1)
		parts = append(parts, f.Predicate)
		for _, a := range f.Args {
			parts = append(parts, fmt.Sprint(a))
		}
		out[strings.Join(parts, "\x00")] = struct{}{}
	}
	return out
}

func sortedRowKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// assertRows compares a derived relation against its exact expected rows.
// Exact, not subset: a chain that derives extra rows (a junk callee joining
// something it should not) is as broken as one that derives too few.
func assertRows(t *testing.T, what string, got []core.Fact, want ...string) {
	t.Helper()
	set := rowSet(got)
	if len(set) != len(want) {
		t.Fatalf("%s: got %d rows %q, want %d rows %q", what, len(set), sortedRowKeys(set), len(want), want)
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			t.Errorf("%s: missing row %q (got %q)", what, w, sortedRowKeys(set))
		}
	}
}

// planEdit is the edit trigger the chain reads. plan_edit has no producer
// in the repo (see the test_impact.mg note): the transaction manager emits
// modified_file instead. The chain tests assert the edit the way a producer
// would spell it — a code_element ref — so the test proves the rules fire
// given the fact, and the missing producer stays visible as a simulation.
func planEdit(ref string) core.Fact {
	return core.Fact{Predicate: "plan_edit", Args: []any{ref}}
}

// TestCodeElement_WhenTestFunction_ShouldEmitIsTestFunction pins the
// producer: which elements get the marker, and that the marker carries the
// element's own ref — the exact spelling test_impact.mg joins on.
func TestCodeElement_WhenTestFunction_ShouldEmitIsTestFunction(t *testing.T) {
	for _, tc := range []struct {
		name string
		elem CodeElement
		want bool
	}{
		{"go test", CodeElement{Ref: "fn:p.TestTarget", Type: ElementFunction, File: "p/app_test.go", Name: "TestTarget"}, true},
		{"go benchmark", CodeElement{Ref: "fn:p.BenchmarkParse", Type: ElementFunction, File: "p/app_test.go", Name: "BenchmarkParse"}, true},
		{"go fuzz", CodeElement{Ref: "fn:p.FuzzParse", Type: ElementFunction, File: "p/app_test.go", Name: "FuzzParse"}, true},
		{"go example", CodeElement{Ref: "fn:p.ExampleParse", Type: ElementFunction, File: "p/app_test.go", Name: "ExampleParse"}, true},
		{"go helper in test file", CodeElement{Ref: "fn:p.helperInTest", Type: ElementFunction, File: "p/app_test.go", Name: "helperInTest"}, false},
		{"go test name needs boundary", CodeElement{Ref: "fn:p.Testify", Type: ElementFunction, File: "p/app_test.go", Name: "Testify"}, false},
		{"go test name in source file", CodeElement{Ref: "fn:p.TestTarget", Type: ElementFunction, File: "p/app.go", Name: "TestTarget"}, false},
		{"go method excluded", CodeElement{Ref: "fn:p.S.M", Type: ElementMethod, File: "p/app_test.go", Name: "M"}, false},
		{"go struct excluded", CodeElement{Ref: "struct:p.S", Type: ElementStruct, File: "p/app_test.go", Name: "S"}, false},
		{"python test", CodeElement{Ref: "py:test_a.py:test_parse", Type: ElementFunction, File: "test_a.py", Name: "test_parse"}, true},
		{"python non-test", CodeElement{Ref: "py:test_a.py:parse", Type: ElementFunction, File: "test_a.py", Name: "parse"}, false},
		{"typescript test", CodeElement{Ref: "ts:x.test.ts:test renders", Type: ElementFunction, File: "x.test.ts", Name: "test renders"}, true},
		{"rust test", CodeElement{Ref: "rs:tests/a.rs:test_parse", Type: ElementFunction, File: "tests/a.rs", Name: "test_parse"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.elem.IsTestFunction(); got != tc.want {
				t.Errorf("IsTestFunction() = %v, want %v", got, tc.want)
			}
			var marked []string
			for _, f := range tc.elem.ToFacts() {
				if f.Predicate == "is_test_function" && len(f.Args) == 1 {
					marked = append(marked, fmt.Sprint(f.Args[0]))
				}
			}
			if tc.want && len(marked) != 1 || !tc.want && len(marked) != 0 {
				t.Errorf("ToFacts() emitted %d is_test_function facts, want %v", len(marked), tc.want)
			}
			if tc.want && len(marked) == 1 && marked[0] != tc.elem.Ref {
				t.Errorf("is_test_function(%q) does not match the element ref %q", marked[0], tc.elem.Ref)
			}
		})
	}
}

// callsOnlySeed is the single-package scenario: every production producer
// plus the edit trigger. Only plan_edit is simulated (it still has no
// producer); is_test_file, file_dir and file_package come from the fast
// scanner. The fixture has no in-repo imports, so no file_imports row exists
// and every dependency below must come through the code_calls (R2) and
// same-package (R3) rules.
func callsOnlySeed(t *testing.T, root string) []core.Fact {
	t.Helper()
	facts := collectTestImpactFacts(t, root)
	return append(facts, planEdit("fn:p.Target"))
}

// TestTestImpactChain_WhenCallsOnly_ShouldDeriveDirectImpact proves the call
// and same-directory rules live on real inputs: TestTarget calls Target (dual
// fn: row from the Cartographer), the scanner marks the test file, keys both
// files to their shared directory and reports both packages, and the chain
// derives the dependency, the impact, both priorities, the package selection
// and the coverage gap for Helper — with the scanner's is_test_file mark
// keeping the gap from accusing the tests themselves.
func TestTestImpactChain_WhenCallsOnly_ShouldDeriveDirectImpact(t *testing.T) {
	root := writeTestImpactFixture(t)
	k := seedRealKernel(t, callsOnlySeed(t, root))

	assertRows(t, "is_test_function", queryRows(t, k, "is_test_function"),
		row("is_test_function", "fn:p.TestTarget"),
		row("is_test_function", "fn:p.TestMethod"),
	)
	// No in-repo imports in this fixture, so the file-import rule (R1) has
	// nothing to join; this pins that the scenario exercises R2 and R3 only.
	if rows := queryRows(t, k, "file_imports"); len(rows) != 0 {
		t.Fatalf("single-package fixture produced file_imports rows %v; the R2/R3 isolation is broken", rows)
	}
	// s := &S{} then s.M() resolves to the method's code_element fn:p.S.M.
	// t.Fail() does not: *testing.T is not a type of this package, so the
	// cartographer keeps the bare row and emits no fn: row for it. R2 joins
	// the fn: strings without requiring the callee to be a code_element, so
	// a spurious fn: row would show up here. R3 (same directory + referenced
	// symbol, on the scanner's real file_dir rows) re-derives those call
	// edges. The transitive row to struct:p.S is method_of from element_parent.
	assertRows(t, "test_depends_on", queryRows(t, k, "test_depends_on"),
		row("test_depends_on", "fn:p.TestTarget", "fn:p.Target"),
		row("test_depends_on", "fn:p.TestMethod", "fn:p.S.M"),
	)
	assertRows(t, "test_depends_on_transitive", queryRows(t, k, "test_depends_on_transitive"),
		row("test_depends_on_transitive", "fn:p.TestTarget", "fn:p.Target"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "fn:p.S.M"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "struct:p.S"),
	)
	assertRows(t, "impacted_test", queryRows(t, k, "impacted_test"),
		row("impacted_test", "fn:p.TestTarget"),
	)
	assertRows(t, "impacted_test_file", queryRows(t, k, "impacted_test_file"),
		row("impacted_test_file", impactTestFile),
	)
	// TestMethod is not impacted, but the scanner's real file_dir rows in
	// the edited file's directory still earn it /low.
	assertRows(t, "test_priority", queryRows(t, k, "test_priority"),
		row("test_priority", "fn:p.TestTarget", "/high"),
		row("test_priority", "fn:p.TestMethod", "/low"),
	)
	assertRows(t, "has_test_coverage", queryRows(t, k, "has_test_coverage"),
		row("has_test_coverage", "fn:p.Target"),
	)
	assertRows(t, "coverage_gap", queryRows(t, k, "coverage_gap"),
		row("coverage_gap", "fn:p.Helper", "/no_direct_tests"),
	)
	// Package-level selection from the scanner's real file_package rows.
	assertRows(t, "test_func_package", queryRows(t, k, "test_func_package"),
		row("test_func_package", "fn:p.TestTarget", "p"),
		row("test_func_package", "fn:p.TestMethod", "p"),
	)
	assertRows(t, "impacted_test_package", queryRows(t, k, "impacted_test_package"),
		row("impacted_test_package", "p"),
	)
}

const (
	impactImportSrcFile  = "q/lib.go"
	impactImportTestFile = "p/app_test.go"
)

// writeTestImpactImportFixture lays out a two-package workspace: q/lib.go
// holds the struct, method and functions, and p/app_test.go imports the q
// package and tests it. Unlike the single-package fixture, this one has a
// genuine in-repo import for the scanner to resolve into file_imports.
func writeTestImpactImportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/impact\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, impactImportSrcFile,
		"package q\n\ntype S struct{ N int }\n\nfunc (s *S) M() int { return s.N }\n\nfunc Target() int { return 1 }\n\nfunc Helper() int { return 2 }\n")
	writeWorkspaceFile(t, root, impactImportTestFile,
		"package p\n\nimport (\n\t\"testing\"\n\n\t\"example.com/impact/q\"\n)\n\nfunc TestTarget(t *testing.T) {\n\tif q.Target() != 1 {\n\t\tt.Fail()\n\t}\n}\n\nfunc TestMethod(t *testing.T) {\n\ts := &q.S{}\n\tif s.M() != 0 {\n\t\tt.Fail()\n\t}\n}\n\nfunc helperInTest(t *testing.T) { t.Helper() }\n")
	return root
}

// collectImportSeed runs every production producer over the import fixture:
// the scope (which follows the test file's import into q/lib.go, so both
// files' code_elements are present), the Cartographer over both files, and
// the fast scanner's file facts — including the real file_imports edge.
func collectImportSeed(t *testing.T, root string) []core.Fact {
	t.Helper()
	scope := NewFileScope(root)
	if err := scope.Open(filepath.Join(root, impactImportTestFile)); err != nil {
		t.Fatalf("open test file: %v", err)
	}
	facts := scope.ScopeFacts()
	cart := NewCartographer()
	defer cart.Close()
	for _, rel := range []string{impactImportSrcFile, impactImportTestFile} {
		deep, err := cart.MapFileAs(filepath.Join(root, filepath.FromSlash(rel)), rel)
		if err != nil {
			t.Fatalf("map %s: %v", rel, err)
		}
		facts = append(facts, deep...)
	}
	scannerFacts := collectScannerFileFacts(t, root)
	// The edge this scenario exists for must be in the scanner's output,
	// not hand-made: fail here rather than asserting a vacuous chain.
	found := false
	for _, f := range scannerFacts {
		if f.Predicate == "file_imports" && len(f.Args) == 2 &&
			fmt.Sprint(f.Args[0]) == impactImportTestFile && fmt.Sprint(f.Args[1]) == impactImportSrcFile {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("scanner produced no file_imports(%s, %s); seed=%v", impactImportTestFile, impactImportSrcFile, scannerFacts)
	}
	return append(append(facts, scannerFacts...), planEdit("fn:q.Target"))
}

// TestTestImpactChain_WhenFileImports_ShouldDeriveCrossProduct proves the
// file-import rule (R1) lives on a real import: the scanner resolves the
// test file's import of q into file_imports, R1 derives every
// test-to-element pair across the two files, so TestMethod is impacted too
// (by the same edit), both tests are high priority, and Helper is covered,
// leaving no gap.
func TestTestImpactChain_WhenFileImports_ShouldDeriveCrossProduct(t *testing.T) {
	root := writeTestImpactImportFixture(t)
	k := seedRealKernel(t, collectImportSeed(t, root))

	assertRows(t, "test_depends_on", queryRows(t, k, "test_depends_on"),
		row("test_depends_on", "fn:p.TestTarget", "struct:q.S"),
		row("test_depends_on", "fn:p.TestTarget", "fn:q.S.M"),
		row("test_depends_on", "fn:p.TestTarget", "fn:q.Target"),
		row("test_depends_on", "fn:p.TestTarget", "fn:q.Helper"),
		row("test_depends_on", "fn:p.TestMethod", "struct:q.S"),
		row("test_depends_on", "fn:p.TestMethod", "fn:q.S.M"),
		row("test_depends_on", "fn:p.TestMethod", "fn:q.Target"),
		row("test_depends_on", "fn:p.TestMethod", "fn:q.Helper"),
	)
	// No second-hop rows: Target and Helper call nothing, and q.Target /
	// s.M() / t.Fail() do not emit an fn: row (other package, or a
	// receiver type this file does not name). Transitive equals direct.
	// method_of's struct is already in the R1 cross-product.
	assertRows(t, "test_depends_on_transitive", queryRows(t, k, "test_depends_on_transitive"),
		row("test_depends_on_transitive", "fn:p.TestTarget", "struct:q.S"),
		row("test_depends_on_transitive", "fn:p.TestTarget", "fn:q.S.M"),
		row("test_depends_on_transitive", "fn:p.TestTarget", "fn:q.Target"),
		row("test_depends_on_transitive", "fn:p.TestTarget", "fn:q.Helper"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "struct:q.S"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "fn:q.S.M"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "fn:q.Target"),
		row("test_depends_on_transitive", "fn:p.TestMethod", "fn:q.Helper"),
	)
	assertRows(t, "impacted_test", queryRows(t, k, "impacted_test"),
		row("impacted_test", "fn:p.TestTarget"),
		row("impacted_test", "fn:p.TestMethod"),
	)
	assertRows(t, "test_priority", queryRows(t, k, "test_priority"),
		row("test_priority", "fn:p.TestTarget", "/high"),
		row("test_priority", "fn:p.TestMethod", "/high"),
	)
	assertRows(t, "test_func_package", queryRows(t, k, "test_func_package"),
		row("test_func_package", "fn:p.TestTarget", "p"),
		row("test_func_package", "fn:p.TestMethod", "p"),
	)
	assertRows(t, "impacted_test_package", queryRows(t, k, "impacted_test_package"),
		row("impacted_test_package", "p"),
	)
	if rows := queryRows(t, k, "coverage_gap"); len(rows) != 0 {
		t.Errorf("coverage_gap should be empty once R1 covers Helper, got %v", rows)
	}
}

// TestTestImpactChain_WhenNoTestFileMark_ShouldAccuseTheTests is the negative
// control for the scanner's is_test_file mark: seeded from scope and deep
// facts alone, without a scan, the gap rule's negation is vacuous and the
// tests themselves are reported as coverage gaps. The scenarios above prove
// the mark excludes them; this one proves the mark is what excludes them.
func TestTestImpactChain_WhenNoTestFileMark_ShouldAccuseTheTests(t *testing.T) {
	root := writeTestImpactFixture(t)
	facts := collectProducerFacts(t, root)
	facts = append(facts, planEdit("fn:p.Target"))
	k := seedRealKernel(t, facts)

	assertRows(t, "coverage_gap", queryRows(t, k, "coverage_gap"),
		row("coverage_gap", "fn:p.Helper", "/no_direct_tests"),
		row("coverage_gap", "fn:p.TestTarget", "/no_direct_tests"),
		row("coverage_gap", "fn:p.TestMethod", "/no_direct_tests"),
	)
}

// newChainCortex builds a sharded kernel shaped like production for the
// chain: the world shard owns every predicate the fixture asserts that
// production owns in world. The owned list mirrors
// DefaultShardPredicateManifests in internal/shards/registration.go (which
// this package cannot import: shards/system imports world), the same way
// core's cortex_split_join_test mirrors the entries it needs. method_of,
// go_tag and code_defines EDB are unowned in production and route to the
// catch-all here too; derived method_of (codedom_core.mg, from world-owned
// element_parent) is what the chain rules read.
func newChainCortex(t *testing.T) *core.CortexKernel {
	t.Helper()
	cortex := core.NewCortexKernel("cortex")
	// The chain joins no per-turn shared predicates.
	if err := cortex.SetSharedPredicates(nil); err != nil {
		t.Fatalf("SetSharedPredicates: %v", err)
	}
	world, err := core.NewKernelShard(core.KernelShardConfig{
		Domain: "world",
		OwnedPredicates: []string{
			"code_element", "element_signature", "element_visibility",
			"element_parent", "code_interactable", "is_test_function",
			"active_file", "file_in_scope",
			"code_calls", "file_imports", "file_dir",
			"plan_edit", "modified_file", "file_package", "is_test_file",
			"type_embeds", "go_struct",
			"assigns", "uses", "guards_block", "guards_return",
			"error_checked_block", "error_checked_return", "same_scope",
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

// TestTestImpactChain_WhenSharded_ShouldMatchSingleKernel is the split-join
// guard for the chain: every predicate the test_impact rules join is owned
// by the world shard together with is_test_function, so the sharded kernel
// must derive exactly what the single-store kernel derives. If a future
// manifest move strands any of them on another shard, the sharded side
// goes quiet here while the single side keeps deriving.
func TestTestImpactChain_WhenSharded_ShouldMatchSingleKernel(t *testing.T) {
	root := writeTestImpactFixture(t)
	seed := callsOnlySeed(t, root)
	single := seedRealKernel(t, seed)
	sharded := newChainCortex(t)
	for _, f := range seed {
		if err := sharded.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	if err := sharded.Evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	for _, pred := range []string{
		"is_test_function", "test_depends_on", "test_depends_on_transitive",
		"impacted_test", "test_priority", "coverage_gap",
		"impacted_test_package",
	} {
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
	// The parity above is only meaningful if the chain derived something.
	if rows, _ := single.Query("impacted_test"); len(rows) == 0 {
		t.Fatal("single kernel derived no impacted_test rows; the parity check is vacuous")
	}
}

// TestCodeCalls_WhenAssertedAsCartographerDid_ShouldStoreAsString is the
// evidence for the producer-side type fix. The Cartographer used to assert
// code_calls args as core.MangleAtom without a leading slash against a
// Decl that bounds both slots /string. That stored fine — ToAtom falls
// back to a string constant — and this pins that it queries back.
func TestCodeCalls_WhenAssertedAsCartographerDid_ShouldStoreAsString(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	legacy := core.Fact{Predicate: "code_calls", Args: []any{core.MangleAtom("p.A"), core.MangleAtom("p.B")}}
	if err := k.Assert(legacy); err != nil {
		t.Fatalf("assert legacy code_calls: %v", err)
	}
	rows, err := k.Query("code_calls")
	if err != nil {
		t.Fatalf("query code_calls: %v", err)
	}
	for _, r := range rows {
		if len(r.Args) == 2 && fmt.Sprint(r.Args[0]) == "p.A" && fmt.Sprint(r.Args[1]) == "p.B" {
			return
		}
	}
	t.Fatalf("legacy MangleAtom code_calls(p.A, p.B) did not query back as a string row: %v", rows)
}

// TestCodeCalls_WhenDualSpelled_ShouldJoinCodeElement proves the spelling
// boundary the chain depends on: a bare row stores and queries (above) but
// joins no code_element ref, while the dual ref-spelled row for the same
// edge joins and derives test_depends_on.
func TestCodeCalls_WhenDualSpelled_ShouldJoinCodeElement(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	seed := []core.Fact{
		{Predicate: "code_element", Args: []any{"fn:p.TestA", "/function", "p/a_test.go", int64(1), int64(9)}},
		{Predicate: "code_element", Args: []any{"fn:p.B", "/function", "p/a.go", int64(1), int64(9)}},
		{Predicate: "is_test_function", Args: []any{"fn:p.TestA"}},
		{Predicate: "code_calls", Args: []any{"p.TestA", "p.B"}},
		{Predicate: "code_calls", Args: []any{"fn:p.TestA", "fn:p.B"}},
	}
	for _, f := range seed {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	assertRows(t, "test_depends_on", queryRows(t, k, "test_depends_on"),
		row("test_depends_on", "fn:p.TestA", "fn:p.B"),
	)
}

// TestScope_WhenTestFileOpened_ShouldEmitIsTestFunction proves the emission
// path end to end: opening the test file puts the marker into ScopeFacts
// with the code_element ref's exact spelling, and opening a source file
// afterwards yields a scope without it (emission-side replacement; the
// kernel-side retraction is pinned by
// TestCodeDOMScopePredicates_CoverEveryEmittedPredicate).
func TestScope_WhenTestFileOpened_ShouldEmitIsTestFunction(t *testing.T) {
	root := writeTestImpactFixture(t)
	scope := NewFileScope(root)
	if err := scope.Open(filepath.Join(root, impactTestFile)); err != nil {
		t.Fatalf("open test file: %v", err)
	}
	elemRefs := map[string]struct{}{}
	marked := map[string]struct{}{}
	for _, f := range scope.ScopeFacts() {
		if f.Predicate == "code_element" && len(f.Args) > 0 {
			elemRefs[fmt.Sprint(f.Args[0])] = struct{}{}
		}
		if f.Predicate == "is_test_function" && len(f.Args) == 1 {
			marked[fmt.Sprint(f.Args[0])] = struct{}{}
		}
	}
	for _, want := range []string{"fn:p.TestTarget", "fn:p.TestMethod"} {
		if _, ok := elemRefs[want]; !ok {
			t.Fatalf("scope has no code_element for %s; the spelling check is vacuous", want)
		}
		if _, ok := marked[want]; !ok {
			t.Errorf("scope marks no is_test_function for %s (marked %v)", want, sortedRowKeys(marked))
		}
	}
	if _, ok := marked["fn:p.helperInTest"]; ok {
		t.Errorf("scope marks the non-Test helper fn:p.helperInTest")
	}
	if len(marked) != 2 {
		t.Errorf("scope marked %d test functions, want 2 (%v)", len(marked), sortedRowKeys(marked))
	}

	if err := scope.Open(filepath.Join(root, impactSrcFile)); err != nil {
		t.Fatalf("open source file: %v", err)
	}
	for _, f := range scope.ScopeFacts() {
		if f.Predicate == "is_test_function" {
			t.Errorf("source-file scope still emits %v; emission must follow the scope", f.Args)
		}
	}
}
