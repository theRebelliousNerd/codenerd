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
// reads it as not asking codeNERD to investigate its failures, or as a
// question about codeNERD's own state (the conversation act,
// conversation_acts.mg), and still starts the diagnosis when Jev says it is
// about its failures.
func TestSelfDiagnoseQuestion_JevDecidesBetweenAnswerAndDiagnosis(t *testing.T) {
	question := perception.Intent{Category: "/query", Verb: "/self_diagnose", Target: "debug-self mode", Confidence: 0.9, IsQuestion: true}
	for _, tc := range []struct {
		name     string
		about    int64
		state    int64 // the question_about_codenerd act
		diagnose bool
	}{
		{"a status question Jev reads as not about failures answers directly", 27, 5, false},
		{"a status question with about_codenerd unsettled answers directly", 31, 98, false},
		{"a request to investigate itself diagnoses", 91, 3, true},
		{"a request to investigate itself diagnoses, whatever the act", 91, 95, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := admissionModel(t, config.WorkModePropose, nil)
			if err := config.EnsureParams(m.kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
				t.Fatal(err)
			}
			assertRouteIntent(t, m, question)
			if err := m.kernel.AssertBatch([]core.Fact{
				{Predicate: "jev_judgment", Args: []any{types.MangleString("about_codenerd"), tc.about}},
				{Predicate: "jev_choice", Args: []any{types.MangleString("conversation_act"), types.MangleString("question_about_codenerd"), tc.state}},
				{Predicate: "jev_choice", Args: []any{types.MangleString("conversation_act"), types.MangleString("request_for_work"), 100 - tc.state}},
			}); err != nil {
				t.Fatal(err)
			}
			route := m.decideRoute("is debug-self on?", question, "reviewer")
			if route.SelfDiagnosis != tc.diagnose || (!tc.diagnose && route.Kind != RouteRespondDirectly) {
				t.Fatalf("route = %+v, want diagnosis=%v", route, tc.diagnose)
			}
		})
	}
}
