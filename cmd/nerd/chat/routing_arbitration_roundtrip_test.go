package chat

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// kernelDerives reports whether the zero-arity predicate holds in the model's
// kernel.
func (m Model) kernelDerives(pred string) bool {
	rows, err := m.kernel.Query(pred)
	return err == nil && len(rows) > 0
}

// newRoundtripModel is a test model on a real kernel loaded with the embedded
// policy corpus.
func newRoundtripModel(t *testing.T) Model {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	m := NewTestModel()
	m.kernel = k
	return m
}

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
//  4. a kernel that cannot be asked decides nothing (RouteNone), and so does
//     an empty derivation: no Go gate answers in the kernel's place;
//  5. per-turn retracts prevent cross-turn signal contamination;
//  6. the delegation threshold is the routing section's, and one lane at
//     most derives.

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

// A kernel that cannot be asked delegates nothing. Until 2026-09-23 this
// returned RouteLegacy, and the caller delegated this confident mutation on a
// Go copy of the gate.
func TestDecideRoute_NilKernelDecidesNothing(t *testing.T) {
	m := NewTestModel() // kernel nil
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.93}
	route := m.decideRoute("fix the typo in README.md", intent, "coder")
	if route.Kind != RouteNone {
		t.Errorf("route = %s, want none with no kernel to ask", route.Kind)
	}
}

// The delegation threshold is routing.delegation_min_confidence, and the
// kernel's "no" is the answer (sweep finding F12: chat replaced it with
// confidence >= 0.5, so a 0.6 turn delegated under a threshold of 70).
func TestDecideRoute_DelegationThresholdIsTheConfigs(t *testing.T) {
	m := newRoundtripModel(t)
	m.Config = &config.UserConfig{Routing: &config.RoutingConfig{DelegationMinConfidence: 70}}

	review := perception.Intent{Category: "/query", Verb: "/review", Target: "internal/core/kernel.go", Confidence: 0.6}
	assertRouteIntent(t, m, review)
	if route := m.decideRoute("review internal/core/kernel.go", review, "reviewer"); route.Kind != RouteNone {
		t.Errorf("a 0.6 review under a threshold of 70: route = %s/%q, want none", route.Kind, route.Shard)
	}

	if err := m.kernel.Retract("user_intent"); err != nil {
		t.Fatal(err)
	}
	fix := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.6}
	assertRouteIntent(t, m, fix)
	if route := m.decideRoute("fix the typo in README.md", fix, "coder"); route.Kind != RouteClarify {
		t.Errorf("a 0.6 mutation under a threshold of 70: route = %s, want clarify", route.Kind)
	}

	fix.Confidence = 0.75
	if route := m.decideRoute("fix the typo in README.md", fix, "coder"); route.Kind != RouteDelegate || route.Shard != "coder" {
		t.Errorf("a 0.75 mutation under a threshold of 70: route = %s/%q, want delegate/coder", route.Kind, route.Shard)
	}

	// The kernel outlives a config: a changed threshold replaces the row.
	m.Config.Routing.DelegationMinConfidence = 50
	fix.Confidence = 0.6
	if route := m.decideRoute("fix the typo in README.md", fix, "coder"); route.Kind != RouteDelegate {
		t.Errorf("a 0.6 mutation after the threshold became 50: route = %s, want delegate", route.Kind)
	}
}

// One lane at most derives, and decomposition is the one when a confident
// mutation also decomposes. Before the precedence moved into the policy both
// rows derived and a Go switch picked.
func TestDecideRoute_OneLaneAtMost(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/create", Target: "auth middleware", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("create the auth middleware and write tests for it", intent, "coder"); route.Kind != RouteMultiStep {
		t.Fatalf("route = %s, want multi_step", route.Kind)
	}
	rows, err := m.kernel.Query("route_decision")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("route_decision holds %d rows, want 1: %v", len(rows), rows)
	}
}

// The derived /clarify lane asks once per input. Sent back unchanged after a
// clarification, the request is not asked about again: no lane derives, and
// articulation answers. Until 2026-09-23 the lane asked forever.
func TestDecideRoute_ClarifiesAnInputOnce(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "none", Confidence: 0.4}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("fix it", intent, "coder"); route.Kind != RouteClarify {
		t.Fatalf("first ask: route = %s, want clarify", route.Kind)
	}
	m.lastClarifyInput = "fix it"
	if route := m.decideRoute("Fix it ", intent, "coder"); route.Kind != RouteNone {
		t.Errorf("the same input again: route = %s, want none", route.Kind)
	}
	if route := m.decideRoute("fix the parser", intent, "coder"); route.Kind != RouteClarify {
		t.Errorf("a different input: route = %s, want clarify", route.Kind)
	}
}

// A compound the decomposition corpus knows ("review X and fix any issues")
// decomposes into the plan the kernel judged. Until 2026-09-23 the corpus was
// read only after the route was decided, the kernel never heard of it, and
// the request went whole to one shard: the fix never ran.
func TestDecideRoute_ACorpusCompoundDecomposes(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "internal/session/executor.go", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	route := m.decideRoute("review internal/session/executor.go and fix any issues", intent, "coder")
	if route.Kind != RouteMultiStep {
		t.Fatalf("route = %s, want multi_step", route.Kind)
	}
	var shards []string
	for _, step := range route.Steps {
		shards = append(shards, normalizeShardType(step.ShardType))
	}
	if len(shards) != 2 || shards[0] != "reviewer" || shards[1] != "coder" {
		t.Errorf("plan shards = %v, want the judged plan [reviewer coder]", shards)
	}
}

// A request the decomposition cannot split is delegated whole. Until
// 2026-09-23 the lane derived on the signals alone; the caller then found a
// one-step plan, fell through, and the turn was neither decomposed nor
// delegated.
func TestDecideRoute_AOneStepPlanIsDelegatedWhole(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "oauth", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	input := "research how to implement OAuth and then add it"
	if steps := decomposeTask(input, intent, m.workspace); len(steps) != 1 {
		t.Fatalf("the fixture needs a one-step plan; decomposeTask gave %d", len(steps))
	}
	if route := m.decideRoute(input, intent, "coder"); route.Kind != RouteDelegate || route.Shard != "coder" {
		t.Errorf("route = %s/%q, want delegate/coder", route.Kind, route.Shard)
	}
	if !m.kernelDerives("is_multi_step") {
		t.Error("the fixture needs is_multi_step to hold: the plan, not the signals, kept it whole")
	}
}

// A plan with a step that names no shard is not a plan: the executor skips
// such a step, so the request would run in part. The decomposer's clause
// parsing makes "/the" and "/greet" (the synonym "hi" inside "this") out of a
// single request; the request is delegated whole.
func TestDecideRoute_APlanWithAnUnroutedStepIsDelegatedWhole(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "the diff", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	input := "review the diff and tell me what you think"
	unrouted := false
	for _, step := range decomposeTask(input, intent, m.workspace) {
		if step.ShardType == "" {
			unrouted = true
		}
	}
	if !unrouted {
		t.Fatal("the fixture needs a plan with a step that names no shard")
	}
	if route := m.decideRoute(input, intent, "coder"); route.Kind != RouteDelegate {
		t.Errorf("route = %s, want delegate", route.Kind)
	}
	if !m.kernelDerives("is_multi_step") {
		t.Error("the fixture needs is_multi_step to hold: the plan, not the signals, kept it whole")
	}
}

// Turn 1 decomposes (multi_step_signal rows asserted); turn 2 is a one-step
// fix. If turn 1's signals lingered, turn 2 would decompose too. Only
// decideRoute's in-method retract protects turn 2.
func TestDecideRoute_MultiStepSignalsDoNotCarryOver(t *testing.T) {
	m := newRoundtripModel(t)
	create := perception.Intent{Category: "/mutation", Verb: "/create", Target: "auth middleware", Confidence: 0.95}
	assertRouteIntent(t, m, create)
	if route := m.decideRoute("create the auth middleware and write tests for it", create, "coder"); route.Kind != RouteMultiStep {
		t.Fatalf("turn 1: route = %s, want multi_step", route.Kind)
	}

	if err := m.kernel.Retract("user_intent"); err != nil {
		t.Fatal(err)
	}
	fix := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.93}
	assertRouteIntent(t, m, fix)
	if route := m.decideRoute("rename the variable", fix, "coder"); route.Kind != RouteDelegate {
		t.Errorf("turn 2 CONTAMINATED: route = %s, want delegate (stale multi_step_signal)", route.Kind)
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

// Whether a delegation runs the verification loop is the kernel's
// (route_verifies): a delegated mutation does, and read-only work does not pay
// the judged attempts. It was shouldVerifyDelegation, a Go test of the intent
// category, until 2026-09-24.
func TestDecideRoute_ADelegatedMutationVerifies(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		intent perception.Intent
		shard  string
		want   bool
	}{
		{"a fix verifies", "fix the typo in README.md",
			perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.93}, "coder", true},
		{"a review does not", "can you review internal/core/kernel.go?",
			perception.Intent{Category: "/query", Verb: "/review", Target: "internal/core/kernel.go", Confidence: 0.88, IsQuestion: true}, "reviewer", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newRoundtripModel(t)
			assertRouteIntent(t, m, tc.intent)
			route := m.decideRoute(tc.input, tc.intent, tc.shard)
			if route.Kind != RouteDelegate {
				t.Fatalf("route = %s, want delegate", route.Kind)
			}
			if route.Verify != tc.want {
				t.Errorf("verify = %v, want %v", route.Verify, tc.want)
			}
		})
	}
}

// A clause pattern of the decomposition corpus is a signal; a keyword-only
// corpus match ("first", "next", "each") is not.
func TestMultiStepSignals_TheCorpusClausePatternIsASignal(t *testing.T) {
	intent := perception.Intent{Verb: "/fix"}
	has := func(input string) bool {
		for _, sig := range multiStepSignals(input, intent) {
			if sig == "/corpus_pattern" {
				return true
			}
		}
		return false
	}
	if !has("review internal/session/executor.go and fix any issues") {
		t.Error("a review-then-fix clause produced no /corpus_pattern")
	}
	if _, _, byClause := MatchMultiStepPattern("what does the first function in main.go do?"); byClause {
		t.Fatal("the fixture needs a keyword-only corpus match")
	}
	if has("what does the first function in main.go do?") {
		t.Error("a keyword-only corpus match produced /corpus_pattern")
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
