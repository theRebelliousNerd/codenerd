//go:build integration

package e2e_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"codenerd/internal/core"
)

// These legacy scenarios query private synthetic state, not a production
// pipeline. Append only that case's zero-arity Decl; keep the full production
// schema/policy and normal undeclared-predicate visibility contract intact.
func newScenarioStateKernel(scenario int) (*core.RealKernel, error) {
	if scenario < 1 || scenario > 40 {
		return nil, fmt.Errorf("unsupported synthetic state scenario %d", scenario)
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		return nil, err
	}
	kernel.AppendSchema(fmt.Sprintf("Decl scenario_%d_baseline().", scenario))
	// Materialize the appended schema before the scenario's producer asserts.
	if _, err := kernel.Query(fmt.Sprintf("scenario_%d_baseline", scenario)); err != nil {
		return nil, fmt.Errorf("prepare synthetic state schema: %w", err)
	}
	return kernel, nil
}

func scenarioStateRows(t *testing.T, kernel *core.RealKernel, predicate string) []core.Fact {
	t.Helper()
	rows, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return rows
}

func TestE2E_SyntheticStateFixture_DeclarationMakesStoredFactVisible(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	fact := core.Fact{Predicate: "scenario_1_baseline"}
	if err := kernel.Assert(fact); err != nil {
		t.Fatal(err)
	}
	if rows := scenarioStateRows(t, kernel, fact.Predicate); len(rows) != 0 {
		t.Fatalf("undeclared state leaked into normal queries: %+v", rows)
	}
	// Do not assert again: the same stored fact becomes queryable when its
	// schema is known. This distinguishes visibility from lost state.
	kernel.AppendSchema("Decl scenario_1_baseline().")
	rows := scenarioStateRows(t, kernel, fact.Predicate)
	if len(rows) != 1 || rows[0].Predicate != fact.Predicate || len(rows[0].Args) != 0 {
		t.Fatalf("declaration did not reveal the retained fact: %+v", rows)
	}
}

func TestE2E_SyntheticStateFixture_ArityAndKernelIsolation(t *testing.T) {
	declared, err := newScenarioStateKernel(1)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for _, kernel := range []*core.RealKernel{declared, fresh} {
		if err := kernel.Assert(core.Fact{Predicate: "scenario_1_baseline"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(scenarioStateRows(t, declared, "scenario_1_baseline")); got != 1 {
		t.Fatalf("declared fixture state: %d rows, want 1", got)
	}
	if got := len(scenarioStateRows(t, fresh, "scenario_1_baseline")); got != 0 {
		t.Fatalf("fixture declaration escaped into a fresh kernel: %d rows", got)
	}
	if err := declared.Assert(core.Fact{Predicate: "scenario_1_baseline", Args: []any{"wrong arity"}}); err != nil {
		t.Fatal(err)
	}
	if got := len(scenarioStateRows(t, declared, "scenario_1_baseline(X)")); got != 0 {
		t.Fatalf("undeclared arity became visible: %d rows", got)
	}
	if err := declared.Assert(core.Fact{Predicate: "scenario_2_baseline"}); err != nil {
		t.Fatal(err)
	}
	if got := len(scenarioStateRows(t, declared, "scenario_2_baseline")); got != 0 {
		t.Fatalf("one fixture declared another scenario: %d rows", got)
	}
	if got := len(scenarioStateRows(t, declared, "scenario_1_baseline")); got != 1 {
		t.Fatalf("near-miss facts corrupted the valid state: %d rows", got)
	}
}

func TestE2E_SyntheticStateFixture_ActualRuleConsumesState(t *testing.T) {
	kernel, err := newScenarioStateKernel(1)
	if err != nil {
		t.Fatal(err)
	}
	kernel.AppendSchema("Decl fixture_scenario_seen().")
	kernel.AppendPolicy("fixture_scenario_seen() :- scenario_1_baseline().")
	if got := len(scenarioStateRows(t, kernel, "fixture_scenario_seen")); got != 0 {
		t.Fatalf("derived state without its input: %d rows", got)
	}
	if err := kernel.Assert(core.Fact{Predicate: "scenario_1_baseline"}); err != nil {
		t.Fatal(err)
	}
	if got := len(scenarioStateRows(t, kernel, "fixture_scenario_seen")); got != 1 {
		t.Fatalf("declared input did not reach the real evaluator: %d rows", got)
	}
}

func TestE2E_SyntheticStateFixture_ExistingAuthorityUnchanged(t *testing.T) {
	baseline, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := newScenarioStateKernel(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.Assert(core.Fact{Predicate: "scenario_1_baseline"}); err != nil {
		t.Fatal(err)
	}
	keys := func(kernel *core.RealKernel, predicate string) []string {
		rows := scenarioStateRows(t, kernel, predicate)
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.String())
		}
		sort.Strings(out)
		return out
	}
	for _, predicate := range []string{"turn_tool_allowed", "permitted"} {
		before, after := keys(baseline, predicate), keys(fixture, predicate)
		if predicate == "turn_tool_allowed" && len(before) == 0 {
			t.Fatal("authority control observed no production catalog")
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("fixture declaration changed %s: before=%v after=%v", predicate, before, after)
		}
	}
}

func TestE2E_SyntheticStateFixture_ConstructorIsBounded(t *testing.T) {
	for _, scenario := range []int{-1, 0, 41} {
		if kernel, err := newScenarioStateKernel(scenario); err == nil || kernel != nil {
			t.Fatalf("out-of-scope scenario %d accepted: %v %v", scenario, kernel, err)
		}
	}
}
