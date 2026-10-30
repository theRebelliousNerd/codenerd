package chat

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

// journey_conversation_act_test.go: Jev's conversation act (turnjudge
// conversation_act, policy/conversation_acts.mg) through the chat's own
// judgment and routing calls, with a scripted perception and the real kernel.

// "save" in reply to codeNERD's question, which perception put on /write with
// no target: alone it asks "write what?"; read as a reply, it is answered in
// the conversation, and nothing runs.
func TestJevJourney_BareReplyIsAnsweredNotAskedAbout(t *testing.T) {
	t.Parallel()
	save := perception.Intent{Category: "/mutation", Verb: "/write", Target: "none", Confidence: 0.9}
	for _, tc := range []struct {
		name  string
		reply float64
		want  RouteKind
	}{
		{"a settled reply is answered in the conversation", 0.92, RouteRespondDirectly},
		{"an unsettled reply still asks first", 0.6, RouteDeliberate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, exec := journeyModel(t, config.WorkModeYolo, save)
			m = withJev(t, m, &fakeJevServer{choices: map[string]map[string]float64{
				"conversation_act": {"reply_to_codenerd": tc.reply, "request_for_work": 1 - tc.reply},
			}})
			assertRouteIntent(t, m, save)
			m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "save", nil))
			if route := m.decideRoute("save", save, "coder"); route.Kind != tc.want {
				t.Fatalf("route = %s, want %s", route.Kind, tc.want)
			}
			if exec.callCount() != 0 {
				t.Fatal("a reply ran work")
			}
		})
	}
}

// "the reviewer shard just refused... that's bad", which perception read as a
// read-only /debug for a shard: feedback is answered, not delegated; read as a
// mutation, it asks first.
func TestJevJourney_FeedbackIsAnsweredNotWorked(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		intent perception.Intent
		want   RouteKind
	}{
		{"read-only", perception.Intent{Category: "/query", Verb: "/debug", Target: "the reviewer shard", Confidence: 0.9}, RouteRespondDirectly},
		{"mutation", perception.Intent{Category: "/mutation", Verb: "/fix", Target: "the reviewer shard", Confidence: 0.9}, RouteDeliberate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, exec := journeyModel(t, config.WorkModeYolo, tc.intent)
			m = withJev(t, m, &fakeJevServer{choices: map[string]map[string]float64{
				"conversation_act": {"feedback_on_codenerd": 0.89, "request_for_work": 0.11},
			}})
			assertRouteIntent(t, m, tc.intent)
			m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "the reviewer shard just refused, that's bad", nil))
			if route := m.decideRoute("the reviewer shard just refused, that's bad", tc.intent, "coder"); route.Kind != tc.want {
				t.Fatalf("route = %s, want %s", route.Kind, tc.want)
			}
			if exec.callCount() != 0 {
				t.Fatal("feedback ran work")
			}
		})
	}
}
