package system

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tools/codedom"
)

// seedImpactFacts loads the world-model facts the test dependency graph is
// built from: which files exist and are tests, which elements they define, and
// which functions call which.
func seedImpactFacts(t *testing.T, k *core.RealKernel) {
	t.Helper()
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []any{"calc.go", "h1", "/go", int64(1), "/false"}},
		{Predicate: "file_topology", Args: []any{"calc_test.go", "h2", "/go", int64(1), "/true"}},
		{Predicate: "code_element", Args: []any{"fn:calc.Add", "/function", "calc.go", int64(1), int64(5)}},
		{Predicate: "code_element", Args: []any{"fn:calc.TestAdd", "/function", "calc_test.go", int64(1), int64(9)}},
		{Predicate: "code_calls", Args: []any{"fn:calc.TestAdd", "fn:calc.Add"}},
		// element_modified is what the CodeDOM edit handlers actually emit, and
		// is the shape the dependency graph is keyed by. plan_edit used to be
		// read here alone, while its only producer wrote file paths into it —
		// so the tools' no-argument path matched nothing on every real call.
		{Predicate: "element_modified", Args: []any{"fn:calc.Add", "sess-1", int64(1)}},
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
}

// TestImpactedTestTools_WiredEndToEnd is the regression test for the defect this
// wiring fixes: run_impacted_tests and get_impacted_tests were registered and
// advertised to the model while RegisterTestImpactProvider had no production
// caller, so every invocation returned "test impact provider not initialized".
//
// It drives the real tool Execute functions, not the provider in isolation, so
// a future refactor that drops the wireTestImpactProvider call fails here
// rather than in a user's session.
func TestImpactedTestTools_WiredEndToEnd(t *testing.T) {
	t.Cleanup(func() { codedom.RegisterTestImpactProvider(nil) })

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	seedImpactFacts(t, kernel)

	wireTestImpactProvider(kernel, t.TempDir())

	ctx := context.Background()
	tool := codedom.GetImpactedTestsTool()
	out, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("get_impacted_tests returned an error: %v", err)
	}
	if strings.Contains(out, "not initialized") {
		t.Fatalf("provider still unregistered after wiring: %s", out)
	}
	// The edited ref comes from the plan_edit fact, and TestAdd calls Add, so
	// the graph must reach it. Anything less means the adapter dropped facts.
	if !strings.Contains(out, "fn:calc.TestAdd") {
		t.Fatalf("impacted test not found in output; got: %s", out)
	}
	if !strings.Contains(out, "fn:calc.Add") {
		t.Fatalf("edited ref not echoed in output; got: %s", out)
	}

	// dry_run keeps this test hermetic: it exercises selection and reporting
	// without shelling out to `go test` in a temp dir with no module.
	runOut, err := codedom.RunImpactedTestsTool().Execute(ctx, map[string]any{"dry_run": true})
	if err != nil {
		t.Fatalf("run_impacted_tests returned an error: %v", err)
	}
	if strings.Contains(runOut, "not initialized") {
		t.Fatalf("run_impacted_tests still unregistered: %s", runOut)
	}
	if !strings.Contains(runOut, "fn:calc.TestAdd") {
		t.Fatalf("run_impacted_tests did not select the impacted test; got: %s", runOut)
	}
}

// TestWireTestImpactProvider_ClearsOnNilKernel pins the fail-honest path: a boot
// that could not build a kernel must leave the tools reporting "not
// initialized" rather than installing a provider that dereferences nil.
func TestWireTestImpactProvider_ClearsOnNilKernel(t *testing.T) {
	t.Cleanup(func() { codedom.RegisterTestImpactProvider(nil) })

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	wireTestImpactProvider(kernel, t.TempDir())
	wireTestImpactProvider(nil, t.TempDir())

	out, err := codedom.GetImpactedTestsTool().Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatalf("expected an error after clearing the provider, got output: %s", out)
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected a not-initialized error, got: %v", err)
	}
}

// TestKernelFactQuerier_ConvertsArgs pins the adapter: codedom.FactData and
// core.Fact are the same shape by construction, and a conversion that dropped
// Args would silently produce an empty dependency graph instead of an error.
func TestKernelFactQuerier_ConvertsArgs(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	seedImpactFacts(t, kernel)

	q := &kernelFactQuerier{kernel: kernel}
	facts, err := q.Query("code_calls")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 code_calls fact, got %d", len(facts))
	}
	if len(facts[0].Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(facts[0].Args), facts[0].Args)
	}
	if facts[0].Args[0] != "fn:calc.TestAdd" || facts[0].Args[1] != "fn:calc.Add" {
		t.Fatalf("args not preserved: %v", facts[0].Args)
	}

	var nilQ *kernelFactQuerier
	if got, err := nilQ.Query("code_calls"); err != nil || got != nil {
		t.Fatalf("nil querier must degrade, got %v, %v", got, err)
	}
}
