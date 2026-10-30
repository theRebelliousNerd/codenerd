package core

import (
	"sort"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// conversation_acts.mg through the real kernel: the act Jev reads
// (jev_choice, settled by the jev_choice_top lattice in jev_evidence.mg)
// answers a turn in the conversation or asks first, and never starts work.
// Each case is one turn's routing EDB, as cmd/nerd/chat decideRoute asserts
// it, with and without the act, so the act's effect is the difference.

type actTurn struct {
	category, verb, target string
	shard                  string // delegation candidate; "/none" for none
	conf                   int64
	signals                []string
	held                   bool
	jev                    map[string]int64 // Noul judgments
	yes                    int64            // jev_yes_percent; 70 when zero
}

func (tc actTurn) facts(act map[string]int64) []Fact {
	yes := tc.yes
	if yes == 0 {
		yes = 70
	}
	out := []Fact{
		{Predicate: "config_param", Args: []any{MangleAtom("/jev_yes_percent"), yes}},
		{Predicate: "config_param", Args: []any{MangleAtom("/jev_no_percent"), int64(30)}},
		{Predicate: "config_param", Args: []any{MangleAtom("/routing_delegation_min_confidence"), int64(70)}},
		{Predicate: "user_intent", Args: []any{MangleAtom("/current_intent"), MangleAtom(tc.category), MangleAtom(tc.verb), tc.target, ""}},
		{Predicate: "delegation_candidate", Args: []any{MangleAtom("/current_intent"), MangleAtom(tc.shard), tc.conf}},
	}
	for _, s := range tc.signals {
		out = append(out, Fact{Predicate: "intent_signal", Args: []any{MangleAtom(s)}})
	}
	if tc.held {
		out = append(out, Fact{Predicate: "work_proposal_pending"})
	}
	for name, p := range tc.jev {
		out = append(out, Fact{Predicate: "jev_judgment", Args: []any{name, p}})
	}
	for option, p := range act {
		out = append(out, Fact{Predicate: "jev_choice", Args: []any{"conversation_act", option, p}})
	}
	return out
}

// routeOf boots a kernel, asserts the turn and returns its single
// route_decision ("" for none) and the rows of the extra predicates asked.
func routeOf(t *testing.T, facts []Fact, extra ...string) (string, map[string][]string) {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	rows, err := k.Query("route_decision")
	if err != nil {
		t.Fatalf("query route_decision: %v", err)
	}
	if len(rows) > 1 {
		t.Fatalf("%d lanes derived, the policy derives one at most: %v", len(rows), rows)
	}
	route := ""
	if len(rows) == 1 {
		route = types.ExtractString(rows[0].Args[0])
		if s := types.ExtractString(rows[0].Args[1]); s != "/none" {
			route += " " + s
		}
	}
	got := map[string][]string{}
	for _, p := range extra {
		ps, err := k.Query(p)
		if err != nil {
			t.Fatalf("query %s: %v", p, err)
		}
		for _, r := range ps {
			parts := make([]string, 0, len(r.Args))
			for _, a := range r.Args {
				parts = append(parts, types.ExtractString(a))
			}
			got[p] = append(got[p], strings.Join(parts, " "))
		}
		sort.Strings(got[p])
	}
	return route, got
}

// Bare replies and remarks perception put on a task verb.
var (
	saveReply    = actTurn{category: "/mutation", verb: "/write", target: "none", shard: "/coder", conf: 90}
	feedbackTurn = actTurn{category: "/query", verb: "/debug", target: "the reviewer shard", shard: "/coder", conf: 90}
)

func TestConversationAct_RoutesTheTurnsTaskVerbsMisread(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		turn        actTurn
		act         map[string]int64
		want, alone string // route with the act, and without any act
	}{
		// Positive: "save" in reply to a question, perception /write with no
		// target. Alone it asks "write what?"; the reply is answered in the
		// conversation it belongs to.
		{"bare reply answered in conversation", saveReply,
			map[string]int64{"reply_to_codenerd": 92, "request_for_work": 8}, "/respond_directly", "/deliberate"},
		// "the reviewer shard just refused... that's bad": a read-only coder
		// job alone; feedback is answered, not worked.
		{"feedback answered, not delegated", feedbackTurn,
			map[string]int64{"feedback_on_codenerd": 89, "request_for_work": 11}, "/respond_directly", "/delegate /reviewer"},
		// Read as a mutation, feedback asks first instead of changing code.
		{"feedback read as a mutation asks first",
			actTurn{category: "/mutation", verb: "/fix", target: "the reviewer shard", shard: "/coder", conf: 90},
			map[string]int64{"feedback_on_codenerd": 89, "request_for_work": 11}, "/deliberate", "/delegate /coder"},
		// "what work mode are you in?" filed as /self_diagnose with tools:
		// answered from the operating state, not a diagnosis job.
		{"question about codeNERD answered from its state",
			actTurn{category: "/query", verb: "/self_diagnose", target: "work mode", shard: "/reviewer", conf: 96, signals: []string{"/suggests_tools"}},
			map[string]int64{"question_about_codenerd": 100}, "/respond_directly", "/delegate /reviewer"},
		// Near miss: the top act below yes is unsettled, and acts on nothing.
		{"unsettled act changes nothing", saveReply,
			map[string]int64{"reply_to_codenerd": 65, "request_for_work": 35}, "/deliberate", "/deliberate"},
		// A request for work leaves perception's reading alone.
		{"request for work changes nothing",
			actTurn{category: "/mutation", verb: "/fix", target: "parser.go", shard: "/coder", conf: 90},
			map[string]int64{"request_for_work": 98}, "/delegate /coder", "/delegate /coder"},
		// A reply that carries a task ("1" picking "fix the flaky test"): the
		// task is perception's.
		{"reply naming a target keeps the task",
			actTurn{category: "/mutation", verb: "/fix", target: "the flaky test", shard: "/coder", conf: 90},
			map[string]int64{"reply_to_codenerd": 97}, "/delegate /coder", "/delegate /coder"},
		// A pasted path is still read: the evidence lane keeps the turn.
		{"shared path is still read",
			actTurn{category: "/query", verb: "/read", target: "README.md", shard: "/none", conf: 90, signals: []string{"/scope_is_file"}},
			map[string]int64{"shared_material": 90}, "/gather_evidence", "/gather_evidence"},
		// Negation: held work is the go-ahead judgments' to read.
		{"held work: the act is not read",
			actTurn{category: "/query", verb: "/debug", target: "the reviewer shard", shard: "/coder", conf: 90, held: true},
			map[string]int64{"feedback_on_codenerd": 89}, "/delegate /reviewer", "/delegate /reviewer"},
		// Negation: Jev also reads a request to investigate codeNERD's own
		// failures, which the self-diagnosis chain takes.
		{"about_codenerd yes: the act is not read",
			actTurn{category: "/query", verb: "/debug", target: "the reviewer shard", shard: "/coder", conf: 90, jev: map[string]int64{"about_codenerd": 85}},
			map[string]int64{"feedback_on_codenerd": 89}, "/delegate /reviewer", "/delegate /reviewer"},
		// Yes configured at 40: two acts tied at the top settle nothing.
		{"a tie settles nothing", actTurn{category: "/mutation", verb: "/write", target: "none", shard: "/coder", conf: 90, yes: 40},
			map[string]int64{"reply_to_codenerd": 45, "feedback_on_codenerd": 45, "small_talk": 10}, "/deliberate", "/deliberate"},
		// An option the kernel has no act for is never read.
		{"unmapped option is ignored", saveReply,
			map[string]int64{"brand_new_act": 95}, "/deliberate", "/deliberate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alone, _ := routeOf(t, c.turn.facts(nil))
			if alone != c.alone {
				t.Fatalf("without the act: route %q, want %q (the baseline moved)", alone, c.alone)
			}
			got, _ := routeOf(t, c.turn.facts(c.act))
			if got != c.want {
				t.Errorf("with %v: route %q, want %q", c.act, got, c.want)
			}
		})
	}
}

// The lattice keeps one top option per question, a tie as two, and the
// readings name what was settled and what was not.
func TestConversationAct_LatticeReading(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		act  map[string]int64
		yes  int64
		want map[string][]string
	}{
		{"clear top", map[string]int64{"reply_to_codenerd": 92, "request_for_work": 5, "small_talk": 3}, 70, map[string][]string{
			"jev_choice_top":       {"conversation_act /fn:pair(\"reply_to_codenerd\", 92)"},
			"jev_chosen":           {"conversation_act reply_to_codenerd"},
			"conversation_act":     {"/reply_to_codenerd"},
			"jev_choice_unsettled": nil,
		}},
		{"tie", map[string]int64{"reply_to_codenerd": 45, "feedback_on_codenerd": 45, "small_talk": 10}, 40, map[string][]string{
			"jev_choice_tied":      {"conversation_act"},
			"jev_chosen":           nil,
			"jev_choice_unsettled": {"conversation_act"},
		}},
		{"unmapped", map[string]int64{"brand_new_act": 95, "small_talk": 5}, 70, map[string][]string{
			"jev_chosen":                {"conversation_act brand_new_act"},
			"conversation_act":          nil,
			"conversation_act_unmapped": {"brand_new_act"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			turn := saveReply
			turn.yes = c.yes
			preds := make([]string, 0, len(c.want))
			for p := range c.want {
				preds = append(preds, p)
			}
			_, got := routeOf(t, turn.facts(c.act), preds...)
			for p, want := range c.want {
				if p == "jev_choice_top" {
					if len(got[p]) != 1 || !strings.Contains(got[p][0], "reply_to_codenerd") {
						t.Errorf("%s = %v, want one row holding the top option", p, got[p])
					}
					continue
				}
				if strings.Join(got[p], "|") != strings.Join(want, "|") {
					t.Errorf("%s = %v, want %v", p, got[p], want)
				}
			}
		})
	}
}
