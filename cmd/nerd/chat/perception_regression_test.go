package chat

import (
	"testing"

	"codenerd/internal/perception"
)

// routeFor seeds user_intent the way process.go does and asks the kernel for
// the turn's lane (policy/routing_arbitration.mg).
func routeFor(t *testing.T, input string, intent perception.Intent) RouteDecision {
	t.Helper()
	m := newRoundtripModel(t)
	assertRouteIntent(t, m, intent)
	return m.decideRoute(input, intent, resolveShardTypeForIntent(intent))
}

// A turn perception already answered, that needs nothing the workspace holds
// and has no shard to hand to, returns perception's reply
// (route_decision(/perception_answer)). Until 2026-09-23 this was a Go fast
// path over a verb table (isConversationalIntent). Its regression stands: "hi"
// once produced a clarification request because /converse was missing from
// that table.
func TestDecideRoute_ConversationIsAnsweredByPerception(t *testing.T) {
	for _, intent := range []perception.Intent{
		{Category: "/query", Verb: "/greet", Target: "", Confidence: 0.1},
		{Category: "/query", Verb: "/converse", Target: "", Confidence: 0.3},
		{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.3},
		{Category: "/query", Verb: "/help", Target: "none", Confidence: 0.2},
		{Category: "/query", Verb: "/knowledge", Target: ""},
		{Category: "/instruction", Verb: "/configure", Target: ""},
		{Category: "/query", Verb: "/shadow", Target: ""},
		{Category: "/query", Verb: "/dream", Target: "delete auth middleware"},
		{Category: "/query", Verb: "/read", Target: ""},
		{Category: "/query", Verb: "/read", Target: "none"},
		{Category: "/query", Verb: "/explain", Target: "capabilities"},
	} {
		t.Run(intent.Verb+"/"+intent.Target, func(t *testing.T) {
			intent.Response = "Hello!"
			if got := routeFor(t, "hi", intent); got.Kind != RoutePerceptionAnswer {
				t.Fatalf("route = %s, want perception_answer", got.Kind)
			}
		})
	}
}

// The same verbs without a reply from perception, and the verbs whose answer
// needs the workspace, do not take that lane.
func TestDecideRoute_NotEveryConversationIsAnsweredByPerception(t *testing.T) {
	cases := []struct {
		name   string
		intent perception.Intent
		want   RouteKind
	}{
		{"a greeting with no reply is articulated", perception.Intent{Category: "/query", Verb: "/greet"}, RouteRespondDirectly},
		{"a codebase explanation goes through articulation", perception.Intent{Category: "/query", Verb: "/explain", Target: "auth", Response: "It..."}, RouteRespondDirectly},
		{"a read of a file is not answered from perception", perception.Intent{Category: "/query", Verb: "/read", Target: "main.go", Response: "Sure."}, RouteNone},
		{"a hypothetical with no reply consults the shards", perception.Intent{Category: "/query", Verb: "/dream", Target: "delete auth middleware"}, RouteDream},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := routeFor(t, "input", tc.intent); got.Kind != tc.want {
				t.Fatalf("route = %s, want %s", got.Kind, tc.want)
			}
		})
	}
}

// A request whose verb acts on something and that names nothing to act on is
// asked about before any lane acts on a guess -- also over a confident shard
// candidate, which the Go clarifiers overrode the same way. A targeted,
// confident one delegates. Until 2026-09-23 these were shouldClarifyIntent and
// shouldAutoClarify (a keyword match over the input) in Go.
func TestDecideRoute_ARequestNamingNothingClarifies(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		intent perception.Intent
		want   RouteKind
	}{
		{"an untargeted, uncertain fix", "fix something", perception.Intent{Category: "/mutation", Verb: "/fix", Target: "none", Confidence: 0.3}, RouteClarify},
		{"an untargeted, confident fix", "fix it", perception.Intent{Category: "/mutation", Verb: "/fix", Target: "none", Confidence: 0.9}, RouteClarify},
		{"an untargeted plan", "plan a new auth system", perception.Intent{Category: "/instruction", Verb: "/generate", Target: "", Confidence: 0.84}, RouteClarify},
		{"a targeted, confident fix delegates", "fix the auth bug", perception.Intent{Category: "/mutation", Verb: "/fix", Target: "auth.go", Confidence: 0.9}, RouteDelegate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := routeFor(t, tc.input, tc.intent); got.Kind != tc.want {
				t.Fatalf("route = %s (shard %q), want %s", got.Kind, got.Shard, tc.want)
			}
		})
	}
}
