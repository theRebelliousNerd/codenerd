package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// "Debug yourself" names no target: its object is codeNERD. Live (TUI,
// 2026-11-12) such a turn came back from perception with an empty target,
// derived needs_clarification from the empty target, and became a multi-round
// deliberation instead of the self-diagnosis chain. Every shape perception
// gives a self-diagnosis must reach the chain.
func TestSelfDiagnosis_AnUntargetedTurnReachesTheChain(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"", "none", "codenerd"} {
		t.Run("target="+target, func(t *testing.T) {
			m := admissionModel(t, config.WorkModeYolo, nil)
			intent := perception.Intent{Category: "/query", Verb: "/self_diagnose", Target: target, Confidence: 0.9}
			assertRouteIntent(t, m, intent)
			route := m.decideRoute("Debug yourself: diagnose the hot problems in your own problems log and fix their root causes", intent, "reviewer")
			if route.Kind != RouteDelegate || !route.Background || !route.SelfDiagnosis {
				t.Fatalf("route = %+v, want the background self-diagnosis chain", route)
			}
		})
	}
}

// A verb that does need a target still asks for one: the implied target is
// the self-diagnosis verb's alone.
func TestSelfDiagnosis_ImpliedTargetDoesNotSpreadToOtherVerbs(t *testing.T) {
	t.Parallel()
	m := admissionModel(t, config.WorkModeYolo, nil)
	intent := perception.Intent{Category: "/query", Verb: "/review", Target: "none", Confidence: 0.9}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("review it", intent, "reviewer"); route.Kind != RouteDeliberate {
		t.Fatalf("an untargeted review routed %+v, want the deliberation that asks what to review", route)
	}
}

// The model is offered its action vocabulary by the understanding atom; the
// kernel audits it against valid_action_type. Every action the atom offers
// must be one the kernel knows, or each such turn is an
// understanding_vocab_miss whose routing silently falls back to the model's
// suggestion (live: action_type=diagnose_self, 2026-11-12).
func TestUnderstandingVocabulary_EveryOfferedActionIsKnownToTheKernel(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "prompt", "atoms", "perception", "understanding.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "### 2. Action Type")
	end := strings.Index(text, "### 3. Domain")
	if start < 0 || end < start {
		t.Fatal("the understanding atom's action table moved; update this test to find it")
	}
	row := regexp.MustCompile("(?m)^\\s*\\| `([a-z_]+)` \\|")
	var offered []string
	for _, mm := range row.FindAllStringSubmatch(text[start:end], -1) {
		offered = append(offered, mm[1])
	}
	if len(offered) < 20 {
		t.Fatalf("read %d actions from the atom's table, want the whole table: %v", len(offered), offered)
	}

	m := newRoundtripModel(t)
	rows, err := m.kernel.Query("valid_action_type")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, f := range rows {
		known[strings.TrimPrefix(types.ExtractString(f.Args[0]), "/")] = true
	}
	for _, action := range offered {
		if !known[action] {
			t.Errorf("the understanding atom offers action %q, but valid_action_type has no /%s", action, action)
		}
	}

	// The audit itself: a self-diagnosis understanding is no miss.
	if err := m.kernel.Assert(core.Fact{Predicate: "current_understanding", Args: []any{
		types.MangleAtom("/causation"), types.MangleAtom("/diagnose_self"), types.MangleAtom("/error_handling"), types.MangleAtom("/codebase"),
	}}); err != nil {
		t.Fatal(err)
	}
	misses, err := m.kernel.Query("understanding_vocab_miss")
	if err != nil {
		t.Fatal(err)
	}
	if len(misses) != 0 {
		t.Fatalf("a self-diagnosis understanding is a vocabulary miss: %v", misses)
	}
}
