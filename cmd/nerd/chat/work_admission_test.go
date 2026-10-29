package chat

import (
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// work_admission_test.go: the work mode and the brief decide, in the kernel,
// whether a lane that starts work runs now or holds (routing_arbitration.mg,
// brief.mg). Real kernel, embedded policy.

func fixReadme() perception.Intent {
	return perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.93}
}

func admissionModel(t *testing.T, mode config.WorkMode, brief *perception.Brief) Model {
	t.Helper()
	m := newRoundtripModel(t)
	m.Config.WorkMode = mode
	m.syncWorkModeFacts()
	m.recordBrief(brief)
	return m
}

func TestWorkAdmission_ModesAndBrief(t *testing.T) {
	t.Parallel()
	ready := &perception.Brief{Goal: "fix the README typo", Ready: true}
	open := &perception.Brief{Goal: "fix the README typo", OpenQuestions: []string{"which typo?"}, Ready: true}
	notReady := &perception.Brief{Goal: "fix the README typo"}
	cases := []struct {
		name  string
		mode  config.WorkMode
		brief *perception.Brief
		want  RouteKind
	}{
		{"yolo_no_brief_runs", config.WorkModeYolo, nil, RouteDelegate},
		{"yolo_ready_brief_runs", config.WorkModeYolo, ready, RouteDelegate},
		{"yolo_open_question_holds", config.WorkModeYolo, open, RouteHold},
		{"yolo_unready_brief_holds", config.WorkModeYolo, notReady, RouteHold},
		{"propose_holds_even_when_ready", config.WorkModePropose, ready, RouteHold},
		{"explicit_holds", config.WorkModeExplicit, ready, RouteHold},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := admissionModel(t, tc.mode, tc.brief)
			intent := fixReadme()
			assertRouteIntent(t, m, intent)
			route := m.decideRoute("fix the typo in README.md", intent, "coder")
			if route.Kind != tc.want {
				t.Fatalf("route = %s, want %s", route.Kind, tc.want)
			}
			if route.Kind == RouteHold && route.Shard != "coder" {
				t.Errorf("held work must name the shard it would run: %q", route.Shard)
			}
		})
	}
}

func TestWorkAdmission_ReadOnlyWorkIsNeverHeld(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModeExplicit, nil)
	intent := perception.Intent{Category: "/query", Verb: "/review", Target: "internal/auth", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("review internal/auth", intent, "reviewer"); route.Kind == RouteHold {
		t.Fatal("a review was held: read-only work must ground the conversation, not wait on it")
	}
}

func TestWorkAdmission_GoAheadRunsHeldWork(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModePropose, nil)
	intent := fixReadme()
	m.goAheadWork = &pendingWork{Input: "fix the typo in README.md", Intent: intent}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("fix the typo in README.md", intent, "coder"); route.Kind != RouteDelegate {
		t.Fatalf("route = %s, want delegate: the user said go", route.Kind)
	}
}

func TestWorkAdmission_ApproveVerbTakesThePendingLane(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModePropose, nil)
	m.pendingWork = &pendingWork{Input: "fix the typo in README.md", Intent: fixReadme()}
	approve := perception.Intent{Category: "/query", Verb: "/approve", Target: "none", Confidence: 0.9}
	assertRouteIntent(t, m, approve)
	if route := m.decideRoute("go", approve, ""); route.Kind != RouteApprovePending {
		t.Fatalf("route = %s, want approve_pending", route.Kind)
	}

	// With nothing held, "go" is conversation.
	m2 := admissionModel(t, config.WorkModePropose, nil)
	assertRouteIntent(t, m2, approve)
	if route := m2.decideRoute("go", approve, ""); route.Kind == RouteApprovePending {
		t.Fatal("approve_pending with nothing held")
	}
}

func TestBrief_KernelRoundTripAndGrounding(t *testing.T) {
	t.Parallel()
	brief := &perception.Brief{
		Goal:          "add rate limiting to the API",
		Constraints:   []string{"keep the public API"},
		Decisions:     []string{"token bucket per client"},
		OpenQuestions: []string{"what limit per minute?"},
	}
	m := admissionModel(t, config.WorkModeYolo, brief)
	if got := m.currentBrief(); !reflect.DeepEqual(got, brief) {
		t.Fatalf("brief round trip:\n got %+v\nwant %+v", got, brief)
	}

	// Nil from perception keeps the brief.
	m.recordBrief(nil)
	if m.currentBrief() == nil {
		t.Fatal("a turn with no brief erased the kept one")
	}

	rows, err := m.kernel.Query("injectable_context")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == "*" {
			lines = append(lines, types.ExtractString(r.Args[1]))
		}
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Brief goal: add rate limiting", "Brief constraint: keep the public API", "Brief decision: token bucket", "Brief open question: what limit"} {
		if !strings.Contains(joined, want) {
			t.Errorf("shards are not grounded in %q; injected:\n%s", want, joined)
		}
	}

	// A new brief replaces the old one whole.
	m.recordBrief(&perception.Brief{Goal: "something else", Ready: true})
	if got := m.currentBrief(); got.Goal != "something else" || len(got.Constraints) != 0 || !got.Ready {
		t.Fatalf("brief not replaced: %+v", got)
	}
}

func TestHoldSurface_SaysWhatIsHeldAndHowToStart(t *testing.T) {
	route := RouteDecision{Kind: RouteHold, Shard: "coder"}
	intent := fixReadme()
	intent.Response = "I can fix that typo."
	proposal := holdSurface(intent, route, config.WorkModePropose, &perception.Brief{Goal: "fix", Ready: true})
	if !strings.Contains(proposal, "**Proposed:**") || !strings.Contains(proposal, "coder") || !strings.Contains(proposal, "go") {
		t.Errorf("propose surface: %s", proposal)
	}
	waiting := holdSurface(intent, route, config.WorkModeYolo, &perception.Brief{Goal: "fix", OpenQuestions: []string{"which typo?"}})
	if !strings.Contains(waiting, "which typo?") {
		t.Errorf("held surface must name the open question: %s", waiting)
	}
}

// Saying "go" is a turn of its own: the firewall read it, found no mapping for
// "approve", and asserted intent_unmapped. The approved run reuses the held
// intent without the firewall, so the "go" turn's perception facts must go
// with it; they routed the approved fix to clarify ("I recognize the action
// but don't have a mapping for it"), while /go, which runs no turn, worked.
func TestWorkAdmission_ApprovedRunIsNotClarifiedByTheGoTurn(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModePropose, nil)
	approve := perception.Intent{Category: "/instruction", Verb: "/approve", Target: "the held fix", Confidence: 0.4}
	assertRouteIntent(t, m, approve)
	if err := m.kernel.Assert(types.Fact{Predicate: "intent_unmapped", Args: []any{types.MangleAtom("/approve"), types.MangleAtom("/no_action_mapping")}}); err != nil {
		t.Fatalf("assert the go turn's intent_unmapped: %v", err)
	}

	intent := fixReadme()
	m.goAheadWork = &pendingWork{Input: "fix the typo in README.md", Intent: intent}
	if warnings := m.assertDirectIntent(intent); len(warnings) != 0 {
		t.Fatalf("assertDirectIntent warnings: %v", warnings)
	}
	if rows, _ := m.kernel.Query("intent_unmapped"); len(rows) != 0 {
		t.Fatalf("the go turn's intent_unmapped survived into the approved run: %v", rows)
	}
	if route := m.decideRoute("fix the typo in README.md", intent, "coder"); route.Kind != RouteDelegate {
		t.Fatalf("route = %s, want delegate: the user said go", route.Kind)
	}
}
