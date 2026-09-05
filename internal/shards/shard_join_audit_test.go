package shards

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// acceptedSeams is the documented residue: rules whose body facts live in
// different shards on purpose or for the architect to decide. Everything
// else must join within one shard. Keep this list in step with ACCEPTED in
// .claude/skills/codenerd-dogfood/scripts/shard_join_audit.py.
var acceptedSeams = map[[2]string]string{
	// The constitution's override paths: signed_approval / admin_override /
	// has_active_override / candidate_action land in the catch-all while
	// pending_action lives in policy. They have never fired in production;
	// homing them would make a dormant permission path live (architect's
	// decision). The deny-side rules only over-deny.
	{"constitution.mg", "permitted"}:                    "override path, architect's call",
	{"constitution.mg", "final_action"}:                 "override path, architect's call",
	{"constitution.mg", "permission_denied"}:            "over-denies only",
	{"constitution.mg", "action_denied"}:                "over-denies only",
	{"constitution.mg", "blocked_learned_action_count"}: "reporting only",
	// Campaign quality check joins campaign_task with file_topology and
	// negates test_coverage; needs a world-side helper, dormant Path B.
	{"campaign_rules.mg", "quality_violation_detected"}: "needs restructure",
	// coder_quality_mode(/normal) :- !in_campaign_context() has no positive
	// anchor, so it fires everywhere; no Go reader today.
	{"coder_campaign.mg", "coder_quality_mode"}: "no positive anchor",
	{"coder_workflow.mg", "coder_quality_mode"}: "no positive anchor",
	// selection_policy.mg admits campaign documents into include_in_context;
	// the exclusion facts are world file facts. Documents are never
	// generated/vendor/binary, so the vacuous negation is harmless.
	{"coder_context.mg", "final_context_include"}:  "documents never excluded",
	{"coder_workflow.mg", "final_context_include"}: "documents never excluded",
}

func buildProductionDerivationMap(t *testing.T, shared []string) *core.DerivationMap {
	t.Helper()
	schemas, policy, err := core.DefaultCorpusText()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, m := range DefaultShardPredicateManifests() {
		for _, p := range m.OwnedPredicates {
			owners[p] = m.Domain
		}
	}
	sharedSet := make(map[string]struct{}, len(shared))
	for _, p := range shared {
		sharedSet[p] = struct{}{}
	}
	// Rules live in schema files too (file_exists in schemas_world.mg).
	dm, err := core.BuildDerivationMap(schemas+"\n"+policy, nil, owners, sharedSet, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	return dm
}

func describe(f core.RuleFinding) string {
	var homes []string
	for p, pr := range f.Homes {
		if pr.All {
			continue
		}
		var s []string
		for sh := range pr.Shards {
			s = append(s, sh)
		}
		sort.Strings(s)
		homes = append(homes, fmt.Sprintf("%s@%s", p, strings.Join(s, "/")))
	}
	sort.Strings(homes)
	neg := ""
	if f.Negated != "" {
		neg = " !" + f.Negated
	}
	return fmt.Sprintf("%s: %s%s [%s]", f.File, f.Head, neg, strings.Join(homes, ", "))
}

// TestShardJoin_EveryRuleCanFireOnTheShardedKernel is the standing form of
// item 55: the Cortex kernel evaluates each shard over its own facts only, so
// a rule whose body joins facts owned by different shards never fires in
// production, and a negation over a fact owned elsewhere is vacuous — while
// every single-store unit test passes. Any new split join or blind negation
// outside acceptedSeams fails here.
func TestShardJoin_EveryRuleCanFireOnTheShardedKernel(t *testing.T) {
	dm := buildProductionDerivationMap(t, SharedPredicates())
	var bad []string
	for _, f := range dm.SplitJoins {
		if _, ok := acceptedSeams[[2]string{f.File, f.Head}]; ok {
			continue
		}
		bad = append(bad, "split join   "+describe(f))
	}
	for _, f := range dm.BlindNegations {
		if _, ok := acceptedSeams[[2]string{f.File, f.Head}]; ok {
			continue
		}
		bad = append(bad, "blind negate "+describe(f))
	}
	t.Logf("corpus: %d predicates, %d split joins and %d blind negations (accepted residue included)",
		len(dm.Presence), len(dm.SplitJoins), len(dm.BlindNegations))
	if len(bad) > 0 {
		sort.Strings(bad)
		for _, b := range bad {
			t.Error(b)
		}
		t.Fatalf("%d rules cannot fire on the sharded kernel; share the per-turn fact, re-home the family, or restructure the rule (see shard_join_audit.py)", len(bad))
	}
}

// TestShardJoin_DetectsTheOriginalDefect proves the analysis has teeth: with
// user_intent unshared (the pre-item-55 shape) dozens of rules split.
func TestShardJoin_DetectsTheOriginalDefect(t *testing.T) {
	var withoutIntent []string
	for _, p := range SharedPredicates() {
		if p != "user_intent" {
			withoutIntent = append(withoutIntent, p)
		}
	}
	dm := buildProductionDerivationMap(t, withoutIntent)
	extra := 0
	for _, f := range dm.SplitJoins {
		if _, ok := acceptedSeams[[2]string{f.File, f.Head}]; !ok {
			extra++
		}
	}
	if extra < 20 {
		t.Fatalf("unsharing user_intent must split many rules; the analysis found only %d", extra)
	}
}
