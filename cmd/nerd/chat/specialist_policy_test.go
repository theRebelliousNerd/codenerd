package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/shards"
	"codenerd/internal/types"
)

// The specialist routing rules in policy/shards.mg had no producer and, even
// with one, could not have fired: specialist_match carried the agent as a
// /string while specialist_classification keyed it as a /name. These tests
// run the real policy over the facts the Go matcher now asserts.

func specialistTestMatches() []shards.SpecialistMatch {
	return []shards.SpecialistMatch{
		{AgentName: "GoExpert", Score: 0.92},        // executor above 80 -> executes
		{AgentName: "mangleexpert", Score: 0.6},     // executor between 40 and 80 -> advises
		{AgentName: "securityauditor", Score: 0.95}, // advisor: never executes, whatever the score
	}
}

func derivedPairs(t *testing.T, k *core.RealKernel, predicate string) map[string]string {
	t.Helper()
	facts, err := k.Query(predicate)
	if err != nil {
		t.Fatalf("Query(%s): %v", predicate, err)
	}
	out := map[string]string{}
	for _, f := range facts {
		if len(f.Args) >= 2 {
			out[types.ExtractString(f.Args[0])] = types.ExtractString(f.Args[1])
		}
	}
	return out
}

func TestSpecialistPolicy_ShouldDeriveExecuteAdviseAndAdvisorFromMatcherFacts(t *testing.T) {
	m := newRoundtripModel(t)
	const task = "refactor the routing seam"

	m.assertSpecialistMatches(task, shards.TaskComplexity(task, 1), specialistTestMatches())

	execute := derivedPairs(t, m.kernel, "specialist_should_execute")
	if execute["/goexpert"] != task || len(execute) != 1 {
		t.Errorf("specialist_should_execute = %v, want only /goexpert for the task", execute)
	}
	advise := derivedPairs(t, m.kernel, "specialist_should_advise")
	if advise["/mangleexpert"] != task || len(advise) != 1 {
		t.Errorf("specialist_should_advise = %v, want only /mangleexpert", advise)
	}

	spec, ok := m.specialistThatShouldExecute(task, specialistTestMatches())
	if !ok || spec.AgentName != "GoExpert" {
		t.Errorf("specialistThatShouldExecute = %q, %v; want GoExpert", spec.AgentName, ok)
	}
	// "refactor" makes the task /high and a strategic advisor is classified.
	if !m.strategicAdvisorRequired(task, shards.TaskComplexity(task, 1), spec) {
		t.Error("strategic_advisor_required was not derived for a /high task")
	}

	// A second delegation replaces the per-task facts instead of stacking them.
	const other = "rename a variable"
	m.assertSpecialistMatches(other, shards.TaskComplexity(other, 1), specialistTestMatches()[1:])
	if execute := derivedPairs(t, m.kernel, "specialist_should_execute"); len(execute) != 0 {
		t.Errorf("stale specialist_should_execute survived a new delegation: %v", execute)
	}
	if m.strategicAdvisorRequired(other, shards.TaskComplexity(other, 1), spec) {
		t.Error("strategic_advisor_required derived for a /normal task")
	}
}

func TestSpecialistPolicy_NilKernelFallbackAgreesWithTheRules(t *testing.T) {
	m := NewTestModel() // kernel is nil
	matches := specialistTestMatches()
	for i := range matches {
		class, _ := shards.GetSpecialistClassification(matches[i].AgentName)
		matches[i].Classification = &class
		matches[i].ShouldExecute = shards.ShouldSpecialistExecuteTask(matches[i].AgentName, matches[i].Score)
	}
	spec, ok := m.specialistThatShouldExecute("t", matches)
	if !ok || spec.AgentName != "GoExpert" {
		t.Errorf("fallback chose %q, %v; want GoExpert", spec.AgentName, ok)
	}
	if !m.strategicAdvisorRequired("t", shards.TaskComplexityHigh, spec) {
		t.Error("fallback did not require a strategic advisor for a /high task")
	}
	if m.strategicAdvisorRequired("t", shards.TaskComplexityNormal, spec) {
		t.Error("fallback required a strategic advisor for a /normal task")
	}
}

func TestClassificationFacts_ShouldCoverEveryBuiltInSpecialist(t *testing.T) {
	facts := shards.ClassificationFacts()
	classified := map[string]bool{}
	for _, f := range facts {
		if f.Predicate == "specialist_classification" {
			classified[types.ExtractString(f.Args[0])] = true
		}
	}
	for name := range shards.DefaultSpecialistClassifications {
		if !classified[string(shards.SpecialistAtom(name))] {
			t.Errorf("no specialist_classification fact for %s", name)
		}
	}
}
