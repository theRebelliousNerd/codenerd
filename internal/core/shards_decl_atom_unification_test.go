package core

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The shard/session predicates in defaults/schemas_shards.mg carried bound
// lists that disagreed with their own heads: activate_shard, shard_startup,
// spawn_subagent, specialist_*, pending_intent, intent_ready_for_executive and
// propose_new_rule were declared /string while every head emitted a name
// constant, and priority_higher was declared [/number, /number] while its six
// ordering facts are /critical, /high, /normal, /low.
//
// escalation_needed/3 was worse than a careless Decl: the .mg rules emitted
// name constants into Target while planner.go and constitution.go asserted bare
// Go strings, so one relation held two kinds of value and any bound query saw
// half of it.
//
// The static guard (mg_decl_literal_conformance_test.go) can only compare a
// head literal to its own Decl. What it cannot see is the Go side and the
// joins, which is where this bug class actually costs something — so those are
// what this test pins.
func TestShardsDeclAtomsUnify(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v (a Decl bound-type change broke analysis)", err)
	}

	// Canary: a healthy kernel derives safe_action at the baseline count. Zero
	// means analysis broke and every downstream query is dead regardless of
	// what follows.
	sa, err := k.Query("safe_action(A)")
	if err != nil {
		t.Fatalf("Query(safe_action) error = %v", err)
	}
	baseline := safeActionCanaryBaseline(t)
	if len(sa) != baseline {
		t.Errorf("safe_action = %d rows, want %d — constitution analysis is degraded", len(sa), baseline)
	}

	facts := []Fact{
		// campaign.Task.ToFacts() shape: the Go TaskStatus/TaskType/TaskPriority
		// constants all carry a leading slash ("/pending", "/critical"), and
		// Fact.ToAtom promotes such strings to NAME constants. That is why
		// task_priority joins the priority_higher table at all.
		// Name is the phase's free-form display label, exactly as Phase.ToFacts
		// passes it through — "Implementation", not an atom. The normalized
		// build layer arrives separately as phase_category/2, which is what
		// activate_specialist_for_phase can actually join. Seeding the label as
		// an atom here made the fixture agree with a rule that could never have
		// matched real campaign data.
		{Predicate: "campaign_phase", Args: []any{"ph1", "camp1", "Implementation", int64(1), MangleAtom("/in_progress"), "profile"}},
		{Predicate: "phase_category", Args: []any{"ph1", MangleAtom("/service")}},
		// A second phase on the other branch, because both rules were rewritten
		// and one passing check would not have told me the other fires.
		// "Discovery" normalizes to /research, which is the plan_reviewer's.
		{Predicate: "campaign_phase", Args: []any{"ph2", "camp1", "Discovery", int64(2), MangleAtom("/pending"), "profile"}},
		{Predicate: "phase_category", Args: []any{"ph2", MangleAtom("/research")}},
		{Predicate: "current_phase", Args: []any{"ph1"}},
		{Predicate: "campaign_task", Args: []any{"t-low", "ph1", "low task", "/pending", "/file_create"}},
		{Predicate: "campaign_task", Args: []any{"t-crit", "ph1", "crit task", "/pending", "/file_create"}},
		{Predicate: "task_priority", Args: []any{"t-low", "/low"}},
		{Predicate: "task_priority", Args: []any{"t-crit", "/critical"}},

		// perception.go emits intentID as the Go string "/current_intent".
		{Predicate: "user_intent", Args: []any{"/current_intent", MangleAtom("/research"), MangleAtom("/research"), "the parser", ""}},
		{Predicate: "target_word_count", Args: []any{"the parser", int64(60)}},

		// Drives activate_shard(/world_model_ingestor).
		{Predicate: "modified", Args: []any{"internal/core/kernel.go"}},

		// Two struggling shard types (3+ failed traces each) drive
		// escalation_needed(/system_health, "shard_performance", ...).
		{Predicate: "reasoning_trace", Args: []any{"tr1", MangleAtom("/coder"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},
		{Predicate: "reasoning_trace", Args: []any{"tr2", MangleAtom("/coder"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},
		{Predicate: "reasoning_trace", Args: []any{"tr3", MangleAtom("/coder"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},
		{Predicate: "reasoning_trace", Args: []any{"tr4", MangleAtom("/tester"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},
		{Predicate: "reasoning_trace", Args: []any{"tr5", MangleAtom("/tester"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},
		{Predicate: "reasoning_trace", Args: []any{"tr6", MangleAtom("/tester"), MangleAtom("/ephemeral"), "s1", MangleAtom("/false"), int64(10)}},

		// escalation_needed: what the FIXED Go producers now emit ...
		{Predicate: "escalation_needed", Args: []any{MangleAtom("/session_planner"), "task_blocked:item-1", "retries exhausted"}},
		{Predicate: "escalation_needed", Args: []any{MangleAtom("/constitution_gate"), "write_file:/etc/hosts", "domain not in allowlist"}},
		// ... and what they emitted BEFORE, kept as a negative control.
		{Predicate: "escalation_needed", Args: []any{"session_planner", "task_blocked:item-0", "retries exhausted"}},
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts error = %v", err)
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

	// --- priority_higher/2 ------------------------------------------------
	check("priority_higher", 6)
	check("priority_higher(/critical, P)", 3)
	check("priority_higher(/high, /low)", 1)
	// Negative control: the /number form the old Decl promised matches nothing.
	check("priority_higher(100, 80)", 0)

	// The live consumer chain: has_earlier_task/2 joins priority_higher against
	// task_priority. If either side held the other constant kind this is 0 and
	// every pending task looks equally eligible.
	check(`has_earlier_task("t-low", "ph1")`, 1)
	check(`has_earlier_task("t-crit", "ph1")`, 0)
	check(`eligible_task("t-crit")`, 1)
	check(`eligible_task("t-low")`, 0)

	// --- escalation_needed/3 ----------------------------------------------
	// Target unified on /name across the .mg rules and both Go producers.
	check("escalation_needed(/session_planner, S, R)", 1)
	check("escalation_needed(/constitution_gate, S, R)", 1)
	// Negative control: the pre-fix bare-string Target is a different value,
	// invisible to the bound query the policy corpus would use. Two rows that
	// mean the same thing, in one relation, that never match each other.
	check(`escalation_needed("session_planner", S, R)`, 1)

	// Subject stays /string — the same slot carries ItemIDs, PhaseIDs and
	// "action:target" composites from escalationSubject().
	check(`escalation_needed(/system_health, "shard_performance", R)`, 1)
	check("escalation_needed(/system_health, /shard_performance, R)", 0)

	// --- pending_intent/1, intent_ready_for_executive/1 --------------------
	check("pending_intent(/current_intent)", 1)
	check("intent_ready_for_executive(/current_intent)", 1)
	check(`pending_intent("current_intent")`, 0)

	// --- shard_startup/2, activate_shard/1 --------------------------------
	check("shard_startup(S, /auto)", 3)
	check("shard_startup(/session_planner, /on_demand)", 1)
	check("activate_shard(/world_model_ingestor)", 1)

	// The Go consumer (shards.ShardManager.StartSystemShards) reads this with
	// ExtractString + TrimLeft("/") to get its profile key. Prove that still
	// yields the registered profile name now that the slot is /name.
	rows, err := k.Query("activate_shard")
	if err != nil {
		t.Fatalf("Query(activate_shard) error = %v", err)
	}
	sawIngestor := false
	for _, f := range rows {
		if len(f.Args) == 0 {
			continue
		}
		if strings.TrimLeft(types.ExtractString(f.Args[0]), "/") == "world_model_ingestor" {
			sawIngestor = true
		}
	}
	if !sawIngestor {
		t.Errorf(`activate_shard consumer normalization did not yield "world_model_ingestor" from %v`, rows)
	}

	// --- spawn_subagent/1 --------------------------------------------------
	check("spawn_subagent(/researcher)", 1)
	check(`spawn_subagent("researcher")`, 0)

	// --- specialist_* family ----------------------------------------------
	// All three tables key on the same closed specialist vocabulary and join
	// each other on it.
	check("specialist_classification(/goexpert, /executor, /technical)", 1)
	check("specialist_can_execute(S)", 5)
	check("specialist_context_source(/goexpert, DB)", 1)
	check("specialist_campaign_role(/northstar, /alignment_guardian)", 1)
	// 5 phase_executors on the /service phase, 2 plan_reviewers on the /research
	// one. Asserting each branch separately is what distinguishes "the rule
	// fires" from "one of the two rules fires".
	check(`activate_specialist_for_phase(S, "ph1")`, 5)
	check(`activate_specialist_for_phase(S, "ph2")`, 2)
	check(`activate_specialist_for_phase(/securityauditor, "ph2")`, 1)
	check(`activate_specialist_for_phase(/securityauditor, "ph1")`, 0)
}

// style_rule/3 declared Threshold as /number while STY003 carried the regex
// "TODO|FIXME" there. Nothing consumed style_rule, so it sat inert — but a
// string in a numeric comparison does not return zero rows, it aborts the whole
// program. With the pre-fix fact in place every query below fails with
// `value "TODO|FIXME" (1) is not a number`, including the unbound
// Query("style_rule"), which is the blast radius that makes this worth a test.
func TestStyleRuleThresholdIsNumericallyComparable(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	k.AppendPolicy(`
Decl style_probe_over(RuleID) bound [/string].
style_probe_over(R) :- style_rule(R, _, T), T > 3.

Decl style_probe_zero(RuleID) bound [/string].
style_probe_zero(R) :- style_rule(R, _, 0).
`)

	// AppendPolicy only flips policyDirty; the rebuild is gated on factsDirty,
	// so a mutation has to land before the appended rules are analyzed.
	if aErr := k.Assert(Fact{Predicate: "modified", Args: []any{"internal/core/x.go"}}); aErr != nil {
		t.Fatalf("Assert(modified) error = %v", aErr)
	}

	check := func(query string, want int) {
		t.Helper()
		rows, qErr := k.Query(query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v — a string in the /number Threshold slot aborts whole-program evaluation", query, qErr)
		}
		if len(rows) != want {
			t.Errorf("Query(%s) got %d rows, want %d: %v", query, len(rows), want, rows)
		}
	}

	check("style_rule", 4)
	check("style_probe_over", 2) // STY001 (120) and STY005 (5)
	check("style_probe_zero", 2) // STY002 and STY003, both zero-tolerance

	// The regex is preserved, just not in a threshold slot.
	rows, err := k.Query(`style_rule_pattern("STY003", P)`)
	if err != nil {
		t.Fatalf("Query(style_rule_pattern) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("style_rule_pattern(\"STY003\", P) got %d rows, want 1", len(rows))
	}
	if got := types.ExtractString(rows[0].Args[1]); got != "TODO|FIXME" {
		t.Errorf("style_rule_pattern STY003 pattern = %q, want %q", got, "TODO|FIXME")
	}
}
