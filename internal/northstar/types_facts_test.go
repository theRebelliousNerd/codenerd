package northstar

import "testing"

func TestParsePriority(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"critical":     100,
		"must_have":    100,
		"high":         80,
		"should_have":  80,
		"medium":       50,
		"low":          20,
		"nice_to_have": 20,
		"":             50, // default
		"unknown":      50, // default
	}
	for in, want := range cases {
		if got := parsePriority(in); got != want {
			t.Errorf("parsePriority(%q)=%d, want %d", in, got, want)
		}
	}
}

func TestParseRiskImpact(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"high":    100,
		"medium":  50,
		"low":     20,
		"":        50, // default
		"extreme": 50, // default
	}
	for in, want := range cases {
		if got := parseRiskImpact(in); got != want {
			t.Errorf("parseRiskImpact(%q)=%d, want %d", in, got, want)
		}
	}
}

// factIndex returns the args of the first fact with the given predicate, or nil.
func factArgs(facts []factLike, predicate string) []any {
	for _, f := range facts {
		if f.pred == predicate {
			return f.args
		}
	}
	return nil
}

type factLike struct {
	pred string
	args []any
}

func TestVisionToFacts(t *testing.T) {
	t.Parallel()
	v := &Vision{
		Mission:    "make logic first",
		Problem:    "agents hallucinate",
		VisionStmt: "deterministic agents",
		Personas: []Persona{
			{Name: "dev", PainPoints: []string{"flaky"}, Needs: []string{"trust"}},
		},
		Capabilities: []Capability{
			{ID: "cap1", Description: "kernel", Timeline: "now", Priority: "critical"},
		},
		Risks: []Risk{
			{ID: "risk1", Description: "drift", Likelihood: "high", Impact: "high", Mitigation: "tests"},
		},
		Requirements: []Requirement{
			{ID: "req1", Type: "functional", Description: "merge", Priority: "must_have"},
		},
		Constraints: []string{"go only"},
	}

	raw := v.ToFacts()
	facts := make([]factLike, len(raw))
	for i, f := range raw {
		facts[i] = factLike{pred: f.Predicate, args: f.Args}
	}

	// Singletons carry the global subject + their text.
	for _, pred := range []string{"northstar_mission", "northstar_problem", "northstar_vision"} {
		args := factArgs(facts, pred)
		if args == nil || len(args) != 2 || args[0] != "global" {
			t.Errorf("%s missing or malformed: %v", pred, args)
		}
	}

	// Persona fans out into pain point + need facts keyed by persona id.
	if a := factArgs(facts, "northstar_persona"); a == nil || a[1] != "dev" {
		t.Errorf("northstar_persona malformed: %v", a)
	}
	if a := factArgs(facts, "northstar_pain_point"); a == nil || a[1] != "flaky" {
		t.Errorf("northstar_pain_point malformed: %v", a)
	}

	// Capability priority is resolved to its numeric weight.
	if a := factArgs(facts, "northstar_capability"); a == nil || a[3] != 100 {
		t.Errorf("northstar_capability priority not resolved: %v", a)
	}
	// Risk impact likewise.
	if a := factArgs(facts, "northstar_risk"); a == nil || a[3] != 100 {
		t.Errorf("northstar_risk impact not resolved: %v", a)
	}
	if factArgs(facts, "northstar_constraint") == nil {
		t.Error("expected a northstar_constraint fact")
	}
	// The sentinel fact is always emitted last.
	if factArgs(facts, "northstar_defined") == nil {
		t.Error("expected the northstar_defined sentinel fact")
	}
}
