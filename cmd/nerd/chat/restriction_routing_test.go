package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

func restrictionOf(u perception.Understanding) perception.Restriction {
	return perception.RestrictionFrom(&u)
}

// "What would happen if we modified retryLoop?" is a /fix by action and a
// what-if by meaning. Before the recognised hypothetical reached routing, a
// confident coder candidate delegated it as work; the dream lane required a
// /dream verb that the conversion never produced.
func TestDecideRoute_RecognisedHypotheticalTakesTheDreamLane(t *testing.T) {
	t.Parallel()
	hypothetical := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "retryLoop", Confidence: 0.95,
		Restriction: restrictionOf(perception.Understanding{Signals: perception.Signals{IsHypothetical: true}}),
	}

	m := newRoundtripModel(t)
	assertRouteIntent(t, m, hypothetical)
	if route := m.decideRoute("what would happen if we modified retryLoop?", hypothetical, "coder"); route.Kind != RouteDream {
		t.Fatalf("hypothetical modify routed %s, want dream", route.Kind)
	}

	// Control: the same request without the recognised limit is work.
	m = newRoundtripModel(t)
	plain := hypothetical
	plain.Restriction = perception.Restriction{}
	assertRouteIntent(t, m, plain)
	if route := m.decideRoute("modify retryLoop", plain, "coder"); route.Kind != RouteDelegate {
		t.Fatalf("the unrestricted modify routed %s, want delegate", route.Kind)
	}

	// A signal from last turn must not flip the next one.
	m = newRoundtripModel(t)
	assertRouteIntent(t, m, hypothetical)
	_ = m.decideRoute("what if", hypothetical, "coder")
	_ = m.kernel.RetractFact(core.Fact{Predicate: "user_intent", Args: []any{"/current_intent"}})
	assertRouteIntent(t, m, plain)
	if route := m.decideRoute("modify retryLoop", plain, "coder"); route.Kind != RouteDelegate {
		t.Fatalf("the next, unrestricted turn routed %s, want delegate (stale /hypothetical?)", route.Kind)
	}
}

// Confirm-first and no-changes requests are not what-ifs: they keep their
// lane and are held to their limit at the action boundary instead.
func TestDecideRoute_OtherRestrictionsKeepTheirLane(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	intent := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "retryLoop", Confidence: 0.95,
		Restriction: restrictionOf(perception.Understanding{UserConstraints: []string{"confirm_first"}}),
	}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("fix retryLoop but check with me first", intent, "coder"); route.Kind != RouteDelegate {
		t.Fatalf("confirm_first routed %s, want delegate (the consent is owed at the write)", route.Kind)
	}
}

// Perception publishes this turn's routing facts while it runs. The stale-fact
// clear that follows it must not be the one that removes them; the previous
// turn's must still go.
func TestRoutingFacts_PerceptionPublicationSurvivesTheTurnsClears(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	assertAll(t, m, core.Fact{Predicate: "derived_mode", Args: []any{"/debug"}}) // last turn's

	m.clearStaleRoutingFacts() // before perception
	if rows, _ := m.kernel.Query("derived_mode"); len(rows) != 0 {
		t.Fatalf("last turn's derived_mode survived the pre-perception clear: %v", rows)
	}

	// Perception publishes this turn's facts (LLMTransducer.assertRoutingFacts).
	assertAll(t, m,
		core.Fact{Predicate: "current_understanding", Args: []any{"/hypothetical", "/modify", "/general", "/function"}},
		core.Fact{Predicate: "derived_mode", Args: []any{"/dream"}},
	)
	m.clearStaleKernelFacts() // after perception, as process.go does
	for _, pred := range []string{"current_understanding", "derived_mode"} {
		if rows, _ := m.kernel.Query(pred); len(rows) == 0 {
			t.Errorf("%s published by perception was retracted by the post-perception clear", pred)
		}
	}
}
