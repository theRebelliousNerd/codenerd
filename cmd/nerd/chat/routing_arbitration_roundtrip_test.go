package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// intentToKernelFact mirrors the exact user_intent fact shape process.go
// asserts during kernel seeding (string args; the kernel coerces "/..." to
// name constants).
func intentToKernelFact(intent perception.Intent) core.Fact {
	return core.Fact{
		Predicate: "user_intent",
		Args: []any{
			"/current_intent",
			intent.Category,
			intent.Verb,
			intent.Target,
			intent.Constraint,
		},
	}
}

// routing_arbitration_roundtrip_test.go exercises Model.decideRoute — the
// single per-turn DECIDE point — against a REAL kernel loaded with the
// embedded policy corpus (policy/routing_arbitration.mg). It proves the Go
// assert→query→map cycle end to end:
//
//  1. questions terminate in RouteRespondDirectly even when a high-confidence
//     shard candidate exists (the "what is the JIT system?" 20-minute bug);
//  2. workhorse verbs delegate even when phrased as questions;
//  3. mutations split into delegate / clarify / multi_step lanes;
//  4. the nil-kernel fail-safe returns RouteLegacy;
//  5. per-turn retracts prevent cross-turn signal contamination.

// assertRouteIntent mirrors production: user_intent must be in the kernel
// before decideRoute runs (process.go seeds it before arbitration).
func assertRouteIntent(t *testing.T, m Model, intent perception.Intent) {
	t.Helper()
	if err := m.kernel.Assert(intentToKernelFact(intent)); err != nil {
		t.Fatalf("assert user_intent failed: %v", err)
	}
}

func TestDecideRoute_QuestionAnswersDirectly(t *testing.T) {
	// The headline regression: a /query question classified as /explain,
	// /analyze, or /research must route to a direct answer — never to the
	// reviewer/researcher delegation that turned questions into 20-minute
	// shard pipelines.
	cases := []struct {
		verb  string
		shard string
	}{
		{"/explain", ""},
		{"/analyze", "reviewer"},
		{"/research", "researcher"},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			m := newRoundtripModel(t)
			intent := perception.Intent{
				Category:   "/query",
				Verb:       tc.verb,
				Target:     "jit system",
				Confidence: 0.88,
				IsQuestion: true,
			}
			assertRouteIntent(t, m, intent)

			route := m.decideRoute("what is the jit system", intent, tc.shard)
			if route.Kind != RouteRespondDirectly {
				t.Errorf("route = %s, want respond_directly (question must not become shard work)", route.Kind)
			}
		})
	}
}

func TestDecideRoute_ConversationalVerbAlwaysDirect(t *testing.T) {
	// /greet is conversational even without the is_question signal.
	m := newRoundtripModel(t)
	intent := perception.Intent{
		Category:   "/query",
		Verb:       "/greet",
		Target:     "none",
		Confidence: 0.99,
		IsQuestion: false,
	}
	assertRouteIntent(t, m, intent)

	route := m.decideRoute("hello there", intent, "")
	if route.Kind != RouteRespondDirectly {
		t.Errorf("route = %s, want respond_directly for /greet", route.Kind)
	}
}

func TestDecideRoute_WorkhorseQuestionDelegates(t *testing.T) {
	// "Can you review my code?" — question phrasing, workhorse verb. The
	// question mark is politeness; the review must still run.
	m := newRoundtripModel(t)
	intent := perception.Intent{
		Category:   "/query",
		Verb:       "/review",
		Target:     "internal/core/kernel.go",
		Confidence: 0.88,
		IsQuestion: true,
	}
	assertRouteIntent(t, m, intent)

	route := m.decideRoute("can you review internal/core/kernel.go?", intent, "reviewer")
	if route.Kind != RouteDelegate {
		t.Fatalf("route = %s, want delegate for workhorse /review question", route.Kind)
	}
	if route.Shard != "reviewer" {
		t.Errorf("route shard = %q, want %q", route.Shard, "reviewer")
	}
}

func TestDecideRoute_MutationLanes(t *testing.T) {
	t.Run("confident_mutation_delegates", func(t *testing.T) {
		m := newRoundtripModel(t)
		intent := perception.Intent{
			Category:   "/mutation",
			Verb:       "/fix",
			Target:     "README.md",
			Confidence: 0.93,
		}
		assertRouteIntent(t, m, intent)

		route := m.decideRoute("fix the typo in README.md", intent, "coder")
		if route.Kind != RouteDelegate || route.Shard != "coder" {
			t.Errorf("route = %s/%q, want delegate/coder", route.Kind, route.Shard)
		}
	})

	t.Run("uncertain_mutation_clarifies", func(t *testing.T) {
		m := newRoundtripModel(t)
		intent := perception.Intent{
			Category:   "/mutation",
			Verb:       "/fix",
			Target:     "none",
			Confidence: 0.4,
		}
		assertRouteIntent(t, m, intent)

		route := m.decideRoute("fix it", intent, "coder")
		if route.Kind != RouteClarify {
			t.Errorf("route = %s, want clarify for low-confidence mutation", route.Kind)
		}
	})

	t.Run("compound_mutation_decomposes", func(t *testing.T) {
		m := newRoundtripModel(t)
		intent := perception.Intent{
			Category:   "/mutation",
			Verb:       "/create",
			Target:     "auth middleware",
			Confidence: 0.95,
		}
		assertRouteIntent(t, m, intent)

		// "create ... tests" trips the compound pattern (strong signal).
		route := m.decideRoute("create the auth middleware and write tests for it", intent, "coder")
		if route.Kind != RouteMultiStep {
			t.Errorf("route = %s, want multi_step for compound mutation", route.Kind)
		}
	})
}

func TestDecideRoute_NilKernelFallsBackToLegacy(t *testing.T) {
	m := NewTestModel() // kernel nil
	intent := perception.Intent{Category: "/query", Verb: "/explain", IsQuestion: true}
	route := m.decideRoute("what is this?", intent, "")
	if route.Kind != RouteLegacy {
		t.Errorf("route = %s, want legacy with nil kernel", route.Kind)
	}
}

// TestDecideRoute_NoContaminationAcrossTurns: turn 1 is a question (asserts
// intent_signal(/is_question)); turn 2 is a confident mutation with NO
// question signal. If turn 1's intent_signal lingered, wants_direct_answer
// could suppress turn 2's delegation. Only decideRoute's in-method retract
// protects turn 2 — no manual cleanup between calls.
func TestDecideRoute_NoContaminationAcrossTurns(t *testing.T) {
	m := newRoundtripModel(t)

	q := perception.Intent{
		Category:   "/query",
		Verb:       "/explain",
		Target:     "jit system",
		Confidence: 0.9,
		IsQuestion: true,
	}
	assertRouteIntent(t, m, q)
	if route := m.decideRoute("what is the jit system", q, ""); route.Kind != RouteRespondDirectly {
		t.Fatalf("turn 1: route = %s, want respond_directly", route.Kind)
	}

	// Turn 2: replace the intent (stable /current_intent ID, mirror production
	// retract-then-assert) and route a mutation.
	if err := m.kernel.Retract("user_intent"); err != nil {
		t.Fatalf("retract user_intent failed: %v", err)
	}
	mut := perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "README.md",
		Confidence: 0.93,
	}
	assertRouteIntent(t, m, mut)
	route := m.decideRoute("fix the typo in README.md", mut, "coder")
	if route.Kind != RouteDelegate {
		t.Errorf("turn 2 CONTAMINATED: route = %s, want delegate "+
			"(stale turn-1 intent_signal not cleared by in-method Retract)", route.Kind)
	}
}

// -----------------------------------------------------------------------------
// Verification scoping + signal extraction
// -----------------------------------------------------------------------------

func TestShouldVerifyDelegation_MutationsOnly(t *testing.T) {
	if !shouldVerifyDelegation(perception.Intent{Category: "/mutation", Verb: "/fix"}) {
		t.Error("mutations must be verified")
	}
	if shouldVerifyDelegation(perception.Intent{Category: "/query", Verb: "/review"}) {
		t.Error("read-only query work must not pay the verification retry loop")
	}
	if shouldVerifyDelegation(perception.Intent{Category: "/instruction", Verb: "/configure"}) {
		t.Error("instructions must not pay the verification retry loop")
	}
}

// TestMultiStepSignals_WeakKeywordsRemoved pins the keyword tightening:
// conversational filler must not produce /keyword_match.
func TestMultiStepSignals_WeakKeywordsRemoved(t *testing.T) {
	intent := perception.Intent{Verb: "/fix"}

	weak := []string{
		"also update the docs",
		"then we can talk",
		"1. is this right?",
		"the first thing I noticed",
		"furthermore it crashes",
	}
	for _, input := range weak {
		for _, sig := range multiStepSignals(input, intent) {
			if sig == "/keyword_match" {
				t.Errorf("weak filler %q produced /keyword_match", input)
			}
		}
	}

	strong := []string{
		"fix the parser and then run the suite",
		"step 1: scaffold it, step 2: wire it up",
		"first, add the field, second, migrate the data",
	}
	for _, input := range strong {
		found := false
		for _, sig := range multiStepSignals(input, intent) {
			if sig == "/keyword_match" {
				found = true
			}
		}
		if !found {
			t.Errorf("explicit sequencing %q did not produce /keyword_match", input)
		}
	}
}

func TestLegacyMultiStepDecision_MirrorsPolicy(t *testing.T) {
	cases := []struct {
		name    string
		signals []string
		want    bool
	}{
		{"empty", nil, false},
		{"campaign_alone", []string{"/campaign_verb"}, true},
		{"compound_alone", []string{"/compound_pattern"}, true},
		{"keyword_alone", []string{"/keyword_match"}, false},
		{"verb_count_alone", []string{"/verb_count_high"}, false},
		{"keyword_plus_verb_count", []string{"/keyword_match", "/verb_count_high"}, true},
	}
	for _, tc := range cases {
		if got := legacyMultiStepDecision(tc.signals); got != tc.want {
			t.Errorf("%s: legacyMultiStepDecision(%v) = %v, want %v", tc.name, tc.signals, got, tc.want)
		}
	}
}
