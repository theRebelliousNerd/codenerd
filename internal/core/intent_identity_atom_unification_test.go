package core

import (
	"testing"

	"codenerd/internal/types"
)

// user_intent/5 is the most consumed predicate in the kernel and it is pure EDB:
// no .mg head asserts it, five Go call sites do. That combination is what let its
// Decl drift unnoticed — it was declared bound [/string, ...] while every one of
// those producers emits a NAME into slot 1 and all 106 rule bodies match the
// /current_intent literal. A head-only static scan cannot see a predicate with no
// head, and Mangle does not enforce a bound list, so nothing anywhere disagreed
// out loud.
//
// The same value flows into processed_intent/1, executive_processed_intent/1 and
// no_action_reason/1, which were declared /string for the same reason, while
// pending_intent/1, intent_ready_for_executive/1 and clarification_question/1 —
// the predicates that READ it — were already /name. This test pins the whole
// group from both sides at once: the Go producers' exact argument shapes, the
// rule bodies that consume them, and the negative controls that prove a string
// with the same characters is a different value.
func TestIntentIdentityAtomsUnify(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v (a Decl bound-type change broke analysis)", err)
	}

	// Canary: a healthy kernel derives safe_action at the baseline count. Zero
	// means analysis broke and every assertion below is measuring a corpse.
	sa, err := k.Query("safe_action(A)")
	if err != nil {
		t.Fatalf("Query(safe_action) error = %v", err)
	}
	baseline := safeActionCanaryBaseline(t)
	if len(sa) != baseline {
		t.Fatalf("safe_action = %d rows, want %d — constitution analysis is degraded", len(sa), baseline)
	}

	check := func(query string, want int) {
		t.Helper()
		rows, qErr := k.Query(query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v", query, qErr)
		}
		if len(rows) != want {
			t.Errorf("Query(%s) got %d rows, want %d: %v", query, len(rows), want, rows)
		}
	}

	// --- The two producer shapes -------------------------------------------
	// perception/transducer.go Intent.ToFact and session/executor.go both wrap
	// the id in types.MangleAtom. chat/process.go, chat/process_seed.go,
	// shards/system/perception.go and context/serializer.go pass the bare Go
	// string "/current_intent" and rely on Fact.ToAtom's promotion. Both must
	// land on the same constant or the corpus sees two different relations.
	facts := []Fact{
		{Predicate: "user_intent", Args: []any{
			MangleAtom("/current_intent"), MangleAtom("/instruction"), MangleAtom("/python"), "test", ""}},
		{Predicate: "processed_intent", Args: []any{"/current_intent"}},
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts error = %v", err)
	}

	check("user_intent(/current_intent, C, V, T, K)", 1)
	check("processed_intent(/current_intent)", 1)
	// Negative control: the pre-fix Decl promised a string here. It matches
	// nothing, which is exactly how a wrong bound list goes unnoticed.
	check(`user_intent("current_intent", C, V, T, K)`, 0)
	check(`processed_intent("current_intent")`, 0)

	// The readers that round 1 already moved to /name still join.
	check("pending_intent(/current_intent)", 1)
	check("intent_ready_for_executive(/current_intent)", 1)

	// --- Target stays /string ----------------------------------------------
	// policy/capabilities.mg matches the sub-command Target as a quoted literal
	// and the Go producers pass intent.Target through unquoted, so a bare Go
	// string has to reach a "test" literal. This is the join that would break if
	// Target had been retyped to /name to accommodate campaign_rules.mg's two
	// stray /status and /progress literals.
	check("next_action(/python_run_pytest)", 1)
	check("next_action(/swebench_run_tests)", 0) // different verb, must not fire

	// --- executive_processed_intent guards the loop ------------------------
	// shards/system/executive.go asserts the bare string "/current_intent" once
	// it has emitted the first action envelope. Every capabilities.mg rule
	// negates it, so the action must disappear on the next pass. If the two
	// sides held different constant kinds the negation would never bite and the
	// executive would re-derive the same action forever.
	if err := k.Assert(Fact{Predicate: "executive_processed_intent", Args: []any{"/current_intent"}}); err != nil {
		t.Fatalf("Assert(executive_processed_intent) error = %v", err)
	}
	check("executive_processed_intent(/current_intent)", 1)
	check(`executive_processed_intent("current_intent")`, 0)
	check("next_action(/python_run_pytest)", 0)

	// --- no_action_reason -> clarification_question ------------------------
	// shards/system/router.go and shards/system/executive_intent.go both assert
	// the intent id as a bare Go string. clarification.mg copies it into
	// clarification_question/1, which is declared /name — so no_action_reason
	// arg 1 has to be /name too or the two Decls describe the same value
	// differently and the first bound query written against the pair silently
	// returns nothing.
	if err := k.Assert(Fact{
		Predicate: "no_action_reason",
		Args:      []any{"/current_intent", MangleAtom("/no_route")},
	}); err != nil {
		t.Fatalf("Assert(no_action_reason) error = %v", err)
	}
	check("no_action_reason(/current_intent, /no_route)", 1)
	check("clarification_question(/current_intent, Q)", 1)
	check(`clarification_question("current_intent", Q)`, 0)
	check("next_action(/interrogative_mode)", 1)

	// The Go consumers read this back with types.ExtractString, which keeps the
	// leading slash on a name constant. executive_intent.go compares the result
	// against the "/current_intent" literal, so prove the round trip.
	rows, err := k.Query("clarification_question")
	if err != nil {
		t.Fatalf("Query(clarification_question) error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("clarification_question returned no rows for the unbound query")
	}
	if got := types.ExtractString(rows[0].Args[0]); got != "/current_intent" {
		t.Errorf("clarification_question arg 0 reads back as %q, want %q — "+
			"executive_intent.go compares this against the literal", got, "/current_intent")
	}
}

// SubAgent runs get /task_intent_N instead of /current_intent so a concurrent
// task cannot overwrite the interactive turn's routing facts
// (session/executor.go ProcessWithIntent). Those ids are name constants too, and
// they must stay out of every /current_intent-scoped rule.
func TestTaskIntentIDIsANameAndDoesNotAliasCurrentIntent(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	if err := k.Assert(Fact{Predicate: "user_intent", Args: []any{
		MangleAtom("/task_intent_7"), MangleAtom("/instruction"), MangleAtom("/python"), "test", ""}}); err != nil {
		t.Fatalf("Assert error = %v", err)
	}

	check := func(query string, want int) {
		t.Helper()
		rows, qErr := k.Query(query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v", query, qErr)
		}
		if len(rows) != want {
			t.Errorf("Query(%s) got %d rows, want %d: %v", query, len(rows), want, rows)
		}
	}

	check("user_intent(/task_intent_7, C, V, T, K)", 1)
	check("user_intent(/current_intent, C, V, T, K)", 0)
	// The /current_intent-scoped capability rules must not fire for a task run.
	check("next_action(/python_run_pytest)", 0)
}

// campaign_rules.mg's status/progress queries were the only two places in the
// corpus that matched user_intent's /string Target with a name constant, so they
// could never fire whatever the intent said. Retyping Target to accommodate them
// would have broken the fifteen capabilities.mg rules that match it as a quoted
// string, so the literals were quoted instead. Prove they now route.
func TestCampaignStatusQueryRoutesOnStringTarget(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	if err := k.LoadFacts([]Fact{
		{Predicate: "current_campaign", Args: []any{"camp1"}},
		{Predicate: "user_intent", Args: []any{
			MangleAtom("/current_intent"), MangleAtom("/query"), MangleAtom("/explain"), "status", ""}},
	}); err != nil {
		t.Fatalf("LoadFacts error = %v", err)
	}

	rows, err := k.Query("next_action(/show_campaign_status)")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("next_action(/show_campaign_status) got %d rows, want 1 — "+
			"Target is /string and the rule must match the quoted literal", len(rows))
	}
	// The name form the rule used to carry matches nothing a producer can emit.
	nrows, err := k.Query("user_intent(/current_intent, /query, V, /status, K)")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	if len(nrows) != 0 {
		t.Errorf("user_intent Target matched the name /status in %d rows — "+
			"no Go producer can emit that", len(nrows))
	}
}
