package system

// =============================================================================
// TEST IMPACT PROVIDER WIRING
// =============================================================================
// internal/tools/codedom registers two model-facing tools, run_impacted_tests
// and get_impacted_tests, that answer "which tests does this edit put at risk?"
// from the world model's test->source dependency graph. Both begin by reading a
// package-level provider, and until 2026-09-09 nothing set it: RegisterTestImpactProvider
// had no non-test caller anywhere in the repo.
//
// The consequence was worse than a missing feature. Both tools were registered
// in the VirtualStore registry and the global registry
// (internal/core/virtual_store_tools.go:91,95), and advertised to the model by
// internal/prompt/atoms/capability/codedom_tools.yaml. The model was told it
// could scope its test runs, called the tool, and got
// "test impact provider not initialized" — one turn and one tool budget slot
// spent to discover a capability that did not exist, every time.
//
// This file is the missing half. It lives in internal/system because that is
// the only package that imports both internal/world (which owns the only
// TestDependencyAnalyzer implementation) and internal/tools/codedom; a
// registration inside internal/core would close the import cycle
// core -> world -> codedom.

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// kernelFactQuerier adapts a *core.RealKernel to codedom.KernelQuerier.
//
// core.Fact and codedom.FactData are the same shape by construction — codedom
// declares its own so internal/tools does not import internal/core — so this is
// a field copy, not a conversion.
type kernelFactQuerier struct {
	kernel *core.RealKernel
}

// Query returns the kernel's facts for predicate in codedom's fact shape.
func (q *kernelFactQuerier) Query(predicate string) ([]codedom.FactData, error) {
	if q == nil || q.kernel == nil {
		return nil, nil
	}
	facts, err := q.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	out := make([]codedom.FactData, 0, len(facts))
	for _, f := range facts {
		out = append(out, codedom.FactData{Predicate: f.Predicate, Args: f.Args})
	}
	return out, nil
}

// testImpactProvider supplies the impacted-test tools with a kernel view and a
// dependency-graph builder scoped to one workspace.
type testImpactProvider struct {
	querier     *kernelFactQuerier
	projectRoot string
}

// GetKernel returns the read-only kernel view the tools query for plan_edit.
func (p *testImpactProvider) GetKernel() codedom.KernelQuerier {
	return p.querier
}

// GetProjectRoot returns the workspace root the tools run `go test` from.
func (p *testImpactProvider) GetProjectRoot() string {
	return p.projectRoot
}

// NewTestDependencyAnalyzer returns a fresh builder.
//
// Fresh per call, deliberately: the graph is derived from code_element and
// code_calls facts that change on every edit the session makes, and an
// impacted-test answer computed against a pre-edit graph is exactly the wrong
// answer. The build cost is paid only when the model actually asks.
func (p *testImpactProvider) NewTestDependencyAnalyzer() codedom.TestDependencyAnalyzer {
	return world.NewTestDependencyBuilder(p.querier, p.projectRoot)
}

// wireTestImpactProvider registers the impacted-test provider for this Cortex.
//
// Nil kernel or empty workspace clears the registration instead of installing a
// half-built provider: the tools then fail with their honest
// "not initialized" message rather than querying a nil kernel.
func wireTestImpactProvider(rk *core.RealKernel, workspace string) {
	if rk == nil || workspace == "" {
		codedom.RegisterTestImpactProvider(nil)
		logging.Get(logging.CategoryBoot).Debug("test impact provider not registered (kernel=%v workspace=%q)", rk != nil, workspace)
		return
	}
	codedom.RegisterTestImpactProvider(&testImpactProvider{
		querier:     &kernelFactQuerier{kernel: rk},
		projectRoot: workspace,
	})
	logging.Get(logging.CategoryBoot).Debug("test impact provider registered for %s", workspace)
}
