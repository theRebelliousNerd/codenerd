package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// CortexKernel is the querier both providers are built on.
var _ world.FactQuerier = (*core.CortexKernel)(nil)

// TestHolographicAndImpactProviders_ReadThroughCortex is the production shape
// of the context providers: a CortexKernel with the real shard manifest.
//
// World-owned facts asserted the way the scanner, the cartographer and the
// edit handlers do (code_calls, code_defines, dependency_link, code_element,
// element_modified) land in the world shard. The catch-all, which is what
// GetPrimaryRealKernel returns and what these providers used to be built on,
// answers none of them. context_priority_file is derived in that same shard
// from code_calls plus the cartographer's code_defines, so the catch-all
// misses the impact ranking too.
func TestHolographicAndImpactProviders_ReadThroughCortex(t *testing.T) {
	t.Cleanup(func() { codedom.RegisterTestImpactProvider(nil) })

	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	libDir := filepath.Join(ws, "internal", "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(libDir, "lib.go")
	callerFile := filepath.Join(libDir, "caller.go")
	importer := filepath.Join(ws, "cmd", "app", "main.go")
	testFile := filepath.Join(libDir, "lib_test.go")
	for name, src := range map[string]string{
		filepath.Join(ws, "go.mod"): "module example.com/m\n\ngo 1.26\n",
		target:                      "package lib\n\nfunc Target() int { return 1 }\n",
		callerFile:                  "package lib\n\nfunc DirectCaller() int { return Target() }\n",
		importer:                    "package main\n\nimport \"example.com/m/internal/lib\"\n\nfunc main() { lib.Target() }\n",
		testFile:                    "package lib\n\nfunc TestTarget(t *testing.T) { Target() }\n",
	} {
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ck, err := NewDomainCortex(ws)
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	primary := ck.GetPrimaryRealKernel()
	if primary == nil {
		t.Fatal("production cortex has no catch-all kernel")
	}

	// Call-graph facts come from the cartographer, and modified_function
	// carries that same symbol: symbolIDFromRef is specified to emit the
	// cartographer's <pkg>.<Name> unchanged (codedom_modified_symbols.go).
	// The scanner's symbol_graph id is func:<Name> and is not a substitute.
	mapped := cartographerSymbolFacts(t, target, callerFile)
	targetDef, ok := definedFunction(mapped, target, "Target")
	if !ok {
		t.Fatalf("cartographer emitted no code_defines for Target in %s", target)
	}
	callerDef, ok := definedFunction(mapped, callerFile, "DirectCaller")
	if !ok {
		t.Fatalf("cartographer emitted no code_defines for DirectCaller in %s", callerFile)
	}
	targetSym := types.ExtractString(targetDef.Args[1])
	callerSym := types.ExtractString(callerDef.Args[1])
	facts := append([]core.Fact{}, mapped...)
	facts = append(facts,
		core.Fact{Predicate: "modified_function", Args: []any{targetSym, target}},
		core.Fact{Predicate: "dependency_link", Args: []any{importer, "pkg:example.com/m/internal/lib", "example.com/m/internal/lib"}},
		core.Fact{Predicate: "file_topology", Args: []any{target, "h1", "/go", int64(1), "/false"}},
		core.Fact{Predicate: "file_topology", Args: []any{testFile, "h2", "/go", int64(1), "/true"}},
		core.Fact{Predicate: "code_element", Args: []any{"fn:" + targetSym, "/function", target, int64(3), int64(3)}},
		// TestTarget is the code_element ref (fn:<pkg>.<Name>), the spelling
		// test_impact.mg joins. The cartographer was not asked to map the
		// test file; this row is that other producer.
		core.Fact{Predicate: "code_element", Args: []any{"fn:lib.TestTarget", "/function", testFile, int64(3), int64(5)}},
		core.Fact{Predicate: "code_calls", Args: []any{"fn:lib.TestTarget", "fn:" + targetSym}},
		core.Fact{Predicate: "element_modified", Args: []any{"fn:" + targetSym, "sess-1", int64(1)}},
	)
	for _, f := range facts {
		if err := ck.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	for _, pred := range []string{"code_calls", "dependency_link", "element_modified", "context_priority_file"} {
		n, err := countPred(primary, pred)
		if err != nil {
			t.Fatalf("catch-all %s: %v", pred, err)
		}
		if n != 0 {
			t.Fatalf("catch-all holds %d %s rows; the fixture no longer reproduces the production split", n, pred)
		}
		n, err = countPred(ck, pred)
		if err != nil {
			t.Fatalf("cortex %s: %v", pred, err)
		}
		if n == 0 {
			t.Fatalf("cortex query returned no %s rows", pred)
		}
	}

	// The wiring these providers used to get.
	blind := world.NewHolographicProvider(primary, ws)
	blindCtx, err := blind.GetContext(target)
	if err != nil {
		t.Fatalf("catch-all GetContext: %v", err)
	}
	if len(blindCtx.CallGraph) != 0 || len(blindCtx.DirectImporters) != 0 || len(blindCtx.PrioritizedCallers) != 0 {
		t.Fatalf("catch-all provider saw world facts: calls=%d importers=%d prioritized=%d",
			len(blindCtx.CallGraph), len(blindCtx.DirectImporters), len(blindCtx.PrioritizedCallers))
	}
	wireTestImpactProvider(primary, ws)
	// run_impacted_tests is the tool that says this. get_impacted_tests
	// returns an empty JSON body instead, which hides the miss.
	blindImpact, err := codedom.RunImpactedTestsTool().Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("catch-all run_impacted_tests: %v", err)
	}
	if !strings.Contains(blindImpact, "no element_modified facts found") {
		t.Fatalf("catch-all impact provider returned edits it cannot see: %s", blindImpact)
	}

	// The wiring initFinalExecutors installs.
	provider := installContextProviders(ck, ws)
	h, ok := provider.(*world.HolographicProvider)
	if !ok || h == nil {
		t.Fatalf("installContextProviders = %T, want *world.HolographicProvider", provider)
	}
	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("cortex GetContext: %v", err)
	}
	if len(hc.DirectImporters) != 1 || !strings.Contains(hc.DirectImporters[0], "main.go") {
		t.Fatalf("DirectImporters = %v, want the cmd/app importer", hc.DirectImporters)
	}
	var sawEdge bool
	for _, e := range hc.CallGraph {
		if e.Caller == callerSym && e.Callee == targetSym {
			sawEdge = true
		}
	}
	if !sawEdge {
		t.Fatalf("call graph = %+v, want %s -> %s", hc.CallGraph, callerSym, targetSym)
	}
	if len(hc.PrioritizedCallers) == 0 || hc.PrioritizedCallers[0].Name != callerSym {
		t.Fatalf("prioritized callers = %+v, want %s first", hc.PrioritizedCallers, callerSym)
	}
	if hc.PrioritizedCallers[0].Priority != 100 || hc.PrioritizedCallers[0].Depth != 1 {
		t.Fatalf("direct caller priority/depth = %d/%d, want 100/1",
			hc.PrioritizedCallers[0].Priority, hc.PrioritizedCallers[0].Depth)
	}
	section := h.PromptSection(context.Background(), target)
	if !strings.Contains(section, "### Imported by") || !strings.Contains(section, "Callers (impact-prioritized)") {
		t.Fatalf("prompt fell through to the unordered branch:\n%s", section)
	}

	out, err := codedom.GetImpactedTestsTool().Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("get_impacted_tests: %v", err)
	}
	if strings.Contains(out, "no element_modified facts found") || strings.Contains(out, "not initialized") {
		t.Fatalf("impact provider still blind on the cortex: %s", out)
	}
	if !strings.Contains(out, "fn:lib.TestTarget") {
		t.Fatalf("impacted test not found: %s", out)
	}
	// dry_run keeps the run tool off the shell. The selection is the same
	// kernel read get_impacted_tests just made.
	runOut, err := codedom.RunImpactedTestsTool().Execute(context.Background(), map[string]any{"dry_run": true})
	if err != nil {
		t.Fatalf("run_impacted_tests: %v", err)
	}
	if strings.Contains(runOut, "no element_modified facts found") || !strings.Contains(runOut, "fn:lib.TestTarget") {
		t.Fatalf("run_impacted_tests did not select the impacted test: %s", runOut)
	}
}

func countPred(q world.FactQuerier, predicate string) (int, error) {
	facts, err := q.Query(predicate)
	if err != nil {
		return 0, err
	}
	return len(facts), nil
}

// TestCartographerCodeDefines_VisibleThroughCortexAndDrivesImpact is the
// production read path for the holographic call graph. The cartographer's
// code_defines rows (real line spans, <pkg>.<Name> ids) must come back from
// CortexKernel.Query, and impact.mg must derive context_priority_file from
// those rows plus the cartographer's code_calls and a modified_function
// that uses the same symbol. A symbol_graph row (func:<Name>, the scanner's
// id) and a file-level dependency_link must not invent a second code_defines
// or code_calls row: those two bridges hid the cartographer and mistyped a
// file edge as a call.
func TestCartographerCodeDefines_VisibleThroughCortexAndDrivesImpact(t *testing.T) {
	ws := t.TempDir()
	libDir := filepath.Join(ws, "internal", "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(libDir, "lib.go")
	callerFile := filepath.Join(libDir, "caller.go")
	importer := filepath.Join(ws, "cmd", "app", "main.go")
	for name, src := range map[string]string{
		target:     "package lib\n\nfunc Target() int { return 1 }\n",
		callerFile: "package lib\n\nfunc DirectCaller() int { return Target() }\n",
	} {
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mapped := cartographerSymbolFacts(t, target, callerFile)
	targetDef, ok := definedFunction(mapped, target, "Target")
	if !ok {
		t.Fatal("cartographer emitted no code_defines for Target")
	}
	callerDef, ok := definedFunction(mapped, callerFile, "DirectCaller")
	if !ok {
		t.Fatal("cartographer emitted no code_defines for DirectCaller")
	}
	targetSym := types.ExtractString(targetDef.Args[1])
	callerSym := types.ExtractString(callerDef.Args[1])
	start, startOK := types.ExtractInt64(targetDef.Args[3])
	end, endOK := types.ExtractInt64(targetDef.Args[4])
	if !startOK || !endOK || start == 0 || end == 0 || end < start {
		t.Fatalf("cartographer line span for %s = %d,%d (ok %v,%v); impact joins real spans, not 0,0", targetSym, start, end, startOK, endOK)
	}

	ck, err := NewDomainCortex(ws)
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	facts := append([]core.Fact{}, mapped...)
	facts = append(facts,
		// The edit handler's spelling of the symbol the cartographer just named.
		core.Fact{Predicate: "modified_function", Args: []any{targetSym, target}},
		// Scanner spelling. Must not be copied into code_defines.
		core.Fact{Predicate: "symbol_graph", Args: []any{"func:Target", "/function", "/public", target, "func Target() int"}},
		core.Fact{Predicate: "symbol_graph", Args: []any{"func:DirectCaller", "/function", "/public", callerFile, "func DirectCaller() int"}},
		// File-level import edge. Must not be copied into code_calls.
		core.Fact{Predicate: "dependency_link", Args: []any{importer, "pkg:example.com/m/internal/lib", "example.com/m/internal/lib"}},
	)
	for _, f := range facts {
		if err := ck.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	defines, err := ck.Query("code_defines")
	if err != nil {
		t.Fatalf("query code_defines: %v", err)
	}
	var sawTarget bool
	for _, f := range defines {
		if len(f.Args) < 5 {
			continue
		}
		sym := types.ExtractString(f.Args[1])
		if sym == "func:Target" || sym == "func:DirectCaller" {
			t.Fatalf("code_defines carries the scanner id %q; that row is the symbol_graph bridge, not the cartographer", sym)
		}
		line, lineOK := types.ExtractInt64(f.Args[3])
		if sym == targetSym && types.ExtractString(f.Args[0]) == target {
			if !lineOK || line != start {
				t.Fatalf("code_defines %s start line = %d (ok %v), cartographer emitted %d", sym, line, lineOK, start)
			}
			if types.ExtractString(f.Args[2]) != "/function" {
				t.Fatalf("code_defines %s type = %q, want /function", sym, types.ExtractString(f.Args[2]))
			}
			sawTarget = true
		}
		if lineOK && line == 0 && sym != "" {
			t.Fatalf("code_defines %s has start line 0; the symbol_graph bridge fills both line slots with 0", sym)
		}
	}
	if !sawTarget {
		t.Fatalf("CortexKernel.Query(code_defines) did not return the cartographer row %s in %s: %+v", targetSym, target, defines)
	}

	calls, err := ck.Query("code_calls")
	if err != nil {
		t.Fatalf("query code_calls: %v", err)
	}
	var sawCall bool
	for _, f := range calls {
		if len(f.Args) < 2 {
			continue
		}
		caller := types.ExtractString(f.Args[0])
		callee := types.ExtractString(f.Args[1])
		if caller == importer || callee == importer || caller == "pkg:example.com/m/internal/lib" || callee == "pkg:example.com/m/internal/lib" {
			t.Fatalf("code_calls contains a dependency_link endpoint (%s -> %s); that bridge types a file edge as a call", caller, callee)
		}
		if caller == callerSym && callee == targetSym {
			sawCall = true
		}
	}
	if !sawCall {
		t.Fatalf("code_calls has no %s -> %s from the cartographer: %+v", callerSym, targetSym, calls)
	}

	priorities, err := ck.Query("context_priority_file")
	if err != nil {
		t.Fatalf("query context_priority_file: %v", err)
	}
	var sawPriority bool
	for _, f := range priorities {
		if len(f.Args) < 3 {
			continue
		}
		if types.ExtractString(f.Args[0]) != callerFile || types.ExtractString(f.Args[1]) != callerSym {
			continue
		}
		prio, prioOK := types.ExtractInt64(f.Args[2])
		if !prioOK || prio != 3 {
			t.Fatalf("context_priority_file(%s, %s) priority = %d (ok %v), want 3 (direct caller)", callerFile, callerSym, prio, prioOK)
		}
		sawPriority = true
	}
	if !sawPriority {
		t.Fatalf("impact.mg derived no context_priority_file for %s in %s from cartographer facts: %+v", callerSym, callerFile, priorities)
	}
}

// cartographerSymbolFacts maps Go files and keeps the code_defines and
// code_calls rows. Those are the facts impact.mg joins; data-flow rows from
// the same walk are a different family.
func cartographerSymbolFacts(t *testing.T, paths ...string) []core.Fact {
	t.Helper()
	c := world.NewCartographer()
	t.Cleanup(c.Close)
	var out []core.Fact
	for _, p := range paths {
		facts, err := c.MapFile(p)
		if err != nil {
			t.Fatalf("cartographer %s: %v", p, err)
		}
		for _, f := range facts {
			if f.Predicate == "code_defines" || f.Predicate == "code_calls" {
				out = append(out, f)
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("cartographer emitted no code_defines or code_calls")
	}
	return out
}

// definedFunction returns the cartographer code_defines row for a function
// declared in file whose id's last component is funcName. A method id is
// <pkg>.<Recv>.<Name>; the function id <pkg>.<Name> is the one modified_function
// and the bare code_calls row use.
func definedFunction(facts []core.Fact, file, funcName string) (core.Fact, bool) {
	var match core.Fact
	found := false
	for _, f := range facts {
		if f.Predicate != "code_defines" || len(f.Args) < 5 {
			continue
		}
		if types.ExtractString(f.Args[0]) != file || types.ExtractString(f.Args[2]) != "/function" {
			continue
		}
		sym := types.ExtractString(f.Args[1])
		base := sym
		if i := strings.LastIndex(sym, "."); i >= 0 {
			base = sym[i+1:]
		}
		if base != funcName {
			continue
		}
		if found && strings.Count(sym, ".") >= strings.Count(types.ExtractString(match.Args[1]), ".") {
			continue
		}
		match = f
		found = true
	}
	return match, found
}
