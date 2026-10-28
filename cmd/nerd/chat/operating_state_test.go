package chat

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

// The main agent is told its own operating state, so "what mode are you in,
// is debug-self on?" is answered from facts. Dogfooding, it said "I'm in
// perception mode, not in debug-self mode" while both were otherwise.
func TestOperatingState_ReachesTheMainAgentsContext(t *testing.T) {
	m := admissionModel(t, config.WorkModePropose, nil)
	if err := m.kernel.Assert(core.Fact{Predicate: problemlog.DebugSelfActivePredicate}); err != nil {
		t.Fatal(err)
	}
	lines := injectedLines(t, m)
	for _, want := range []string{"Work mode: propose", "Debug-self is on"} {
		if !strings.Contains(lines, want) {
			t.Errorf("the main agent's context lacks %q:\n%s", want, lines)
		}
	}
	if err := m.kernel.Retract(problemlog.DebugSelfActivePredicate); err != nil {
		t.Fatal(err)
	}
	if lines := injectedLines(t, m); !strings.Contains(lines, "Debug-self is off") {
		t.Errorf("debug-self off is not stated:\n%s", lines)
	}
}

// A question perception filed as /self_diagnose is answered directly when Jev
// reads it as not asking codeNERD to investigate its failures, and still
// starts the diagnosis when Jev says it is.
func TestSelfDiagnoseQuestion_JevDecidesBetweenAnswerAndDiagnosis(t *testing.T) {
	question := perception.Intent{Category: "/query", Verb: "/self_diagnose", Target: "debug-self mode", Confidence: 0.9, IsQuestion: true}
	for _, tc := range []struct {
		name     string
		percent  int64
		diagnose bool
	}{
		{"a status question answers directly", 27, false},
		{"a request to investigate itself diagnoses", 91, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := admissionModel(t, config.WorkModePropose, nil)
			if err := config.EnsureParams(m.kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
				t.Fatal(err)
			}
			assertRouteIntent(t, m, question)
			if err := m.kernel.Assert(core.Fact{Predicate: "jev_judgment", Args: []any{types.MangleString("about_codenerd"), tc.percent}}); err != nil {
				t.Fatal(err)
			}
			route := m.decideRoute("is debug-self on?", question, "reviewer")
			if route.SelfDiagnosis != tc.diagnose || (!tc.diagnose && route.Kind != RouteRespondDirectly) {
				t.Fatalf("route = %+v, want diagnosis=%v", route, tc.diagnose)
			}
		})
	}
}
