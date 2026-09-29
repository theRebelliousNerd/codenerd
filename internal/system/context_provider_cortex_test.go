package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// CortexKernel is the querier both providers are built on.
var _ world.FactQuerier = (*core.CortexKernel)(nil)

// TestHolographicAndImpactProviders_ReadThroughCortex is the production shape
// of the context providers: a CortexKernel with the real shard manifest.
//
// World-owned facts asserted the way the scanner and the edit handlers do
// (code_calls, dependency_link, code_element, element_modified, plan_edit)
// land in the world shard. The catch-all, which is what GetPrimaryRealKernel
// returns and what these providers used to be built on, answers none of them.
// context_priority_file is derived in that same shard from code_calls plus
// the code_defines bridge off symbol_graph, so the catch-all misses the
// impact ranking too.
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

	// symbol_graph ids are the strings code_calls uses. The world shard
	// derives code_defines from symbol_graph (knowledge.mg) and impact.mg
	// joins that to code_calls; the two only meet when the identifier is the
	// same. The scanner's own ids are func:Name, which do not match the
	// cartographer's pkg.Name, so this fixture states the joined form.
	facts := []core.Fact{
		{Predicate: "symbol_graph", Args: []any{"lib.Target", "/function", "/public", target, "func Target() int"}},
		{Predicate: "symbol_graph", Args: []any{"lib.DirectCaller", "/function", "/public", callerFile, "func DirectCaller() int"}},
		{Predicate: "code_calls", Args: []any{"lib.DirectCaller", "lib.Target"}},
		{Predicate: "modified_function", Args: []any{"lib.Target", target}},
		{Predicate: "dependency_link", Args: []any{importer, "pkg:example.com/m/internal/lib", "example.com/m/internal/lib"}},
		{Predicate: "file_topology", Args: []any{target, "h1", "/go", int64(1), "/false"}},
		{Predicate: "file_topology", Args: []any{testFile, "h2", "/go", int64(1), "/true"}},
		{Predicate: "code_element", Args: []any{"fn:lib.Target", "/function", target, int64(3), int64(3)}},
		{Predicate: "code_element", Args: []any{"fn:lib.TestTarget", "/function", testFile, int64(3), int64(5)}},
		{Predicate: "code_calls", Args: []any{"fn:lib.TestTarget", "fn:lib.Target"}},
		{Predicate: "element_modified", Args: []any{"fn:lib.Target", "sess-1", int64(1)}},
		{Predicate: "plan_edit", Args: []any{"fn:lib.Target"}},
		// Cartographer EDB. Unowned, so it is stored in the catch-all. The
		// world shard's derived code_defines (from symbol_graph, lines 0,0)
		// is a different row.
		{Predicate: "code_defines", Args: []any{target, "lib.Target", "/function", int64(3), int64(3)}},
	}
	for _, f := range facts {
		if err := ck.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	for _, pred := range []string{"code_calls", "dependency_link", "element_modified", "plan_edit", "context_priority_file"} {
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
	if !strings.Contains(blindImpact, "no plan_edit facts found") {
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
		if e.Caller == "lib.DirectCaller" && e.Callee == "lib.Target" {
			sawEdge = true
		}
	}
	if !sawEdge {
		t.Fatalf("call graph = %+v, want lib.DirectCaller -> lib.Target", hc.CallGraph)
	}
	if len(hc.PrioritizedCallers) == 0 || hc.PrioritizedCallers[0].Name != "lib.DirectCaller" {
		t.Fatalf("prioritized callers = %+v, want lib.DirectCaller first", hc.PrioritizedCallers)
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
	if strings.Contains(out, "no plan_edit facts found") || strings.Contains(out, "not initialized") {
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
	if strings.Contains(runOut, "no plan_edit facts found") || !strings.Contains(runOut, "fn:lib.TestTarget") {
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
