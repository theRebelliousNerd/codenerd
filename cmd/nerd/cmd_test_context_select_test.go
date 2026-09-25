package main

import (
	"bytes"
	"strings"
	"testing"

	"codenerd/internal/testing/context_harness"
)

// The help lists every registered scenario. It was hand-written and named
// four of the eight mock scenarios and six of the seven integration ones.
func TestTestContextHelp_ShouldListEveryRegisteredScenario(t *testing.T) {
	for _, sc := range context_harness.AllScenarios() {
		if !strings.Contains(testContextCmd.Long, sc.ScenarioID) {
			t.Errorf("nerd test-context --help does not list %s", sc.ScenarioID)
		}
	}
}

// --category narrows the run and, without --scenario, runs the category. The
// flag was parsed and never read.
func TestSelectTestContextRun_ShouldApplyTheCategory(t *testing.T) {
	h := context_harness.NewHarness(nil, context_harness.SimulatorConfig{}, &bytes.Buffer{}, "json")
	runAll, err := selectTestContextRun(h, "", false, "integration")
	if err != nil || !runAll {
		t.Fatalf("--category=integration: runAll=%v err=%v, want a run of the category", runAll, err)
	}
	for _, id := range h.ListScenarios() {
		if !strings.Contains(strings.Join(scenarioIDs(context_harness.ScenariosByCategory(context_harness.CategoryIntegration)), " "), id) {
			t.Errorf("--category=integration selected %s", id)
		}
	}
	if len(h.ListScenarios()) != 7 {
		t.Errorf("--category=integration selected %d scenarios, want 7", len(h.ListScenarios()))
	}

	h = context_harness.NewHarness(nil, context_harness.SimulatorConfig{}, &bytes.Buffer{}, "json")
	if runAll, err := selectTestContextRun(h, "", false, ""); err != nil || runAll {
		t.Errorf("no flags: runAll=%v err=%v, want the listing", runAll, err)
	}
	if _, err := selectTestContextRun(h, "", false, "nightly"); err == nil {
		t.Error("an unknown category was accepted")
	}
}

func scenarioIDs(scs []*context_harness.Scenario) []string {
	ids := make([]string, len(scs))
	for i, sc := range scs {
		ids[i] = sc.ScenarioID
	}
	return ids
}
