package shards

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"codenerd/internal/core"
)

// acceptedSeams is the documented residue: rules whose body facts live in
// different shards on purpose or for the architect to decide. Everything
// else must join within one shard. Each exception pins the entire parsed clause, never just its head.
func acceptedSeam(f core.RuleFinding) bool {
	for _, pin := range seamPins {
		if pin.File == f.File && normalizedClause(pin.Clause) == normalizedClause(f.Clause) {
			return true
		}
	}
	return false
}

type seamPin struct {
	File   string
	Head   string
	Clause string
	Reason string
}

//go:embed testdata/accepted_seams.json
var seamPinJSON []byte
var seamPins = func() []seamPin {
	var pins []seamPin
	if err := json.Unmarshal(seamPinJSON, &pins); err != nil {
		panic(err)
	}
	return pins
}()

// Ignore formatting outside quoted literals, preserving string identity.
func normalizedClause(s string) string {
	var b strings.Builder
	quoted, escaped := false, false
	for _, r := range s {
		if !quoted && unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
		if escaped {
			escaped = false
			continue
		}
		if quoted && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
		}
	}
	return b.String()
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
	dm, err := core.BuildAuditDerivationMap(
		schemas+"\n"+policy, core.IntentCorpusPredicates(), starvedPredicates(t), owners, sharedSet, PredicateConsumerRoutes(), "cortex")
	if err != nil {
		t.Fatal(err)
	}
	return dm
}

// starvedPredicates are the predicates nothing produces: no rule head, no fact,
// no Go that asserts one. internal/core/defaults TestStarvedPredicateBudget keeps
// that inventory exact in both directions, so it is the measurement. A predicate
// with no rows has no rows in any shard: it is present and complete everywhere,
// and a rule that joins, negates or counts it is not split by where it would
// have been homed (BuildAuditDerivationMap). Homing it in the manifest would
// instead make the starved gate believe something produces it.
func starvedPredicates(t *testing.T) map[string]struct{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "core", "defaults", "testdata", "starved_predicates.txt"))
	if err != nil {
		t.Fatalf("read the starved-predicate inventory: %v", err)
	}
	out := map[string]struct{}{}
	for _, line := range strings.Split(string(data), "\n") {
		name, _, _ := strings.Cut(line, "#")
		if name = strings.TrimSpace(name); name != "" {
			out[name] = struct{}{}
		}
	}
	if len(out) < 20 {
		t.Fatalf("read %d starved predicates; the inventory moved", len(out))
	}
	return out
}

func shardList(p core.Presence) []string {
	if p.All {
		return []string{"ALL"}
	}
	var s []string
	for sh := range p.Shards {
		s = append(s, sh)
	}
	sort.Strings(s)
	return s
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
	if f.Incomplete != "" {
		neg = " over " + f.Incomplete
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
	assertPolicyRuleFires := func(head string, containsPositive string) {
		t.Helper()
		for _, rule := range dm.Rules {
			if rule.File != "policy_mcp.mg" || rule.Head != head {
				continue
			}
			if containsPositive != "" && !containsString(rule.Pos, containsPositive) {
				continue
			}
			if !rule.Fires.Equals(core.SingleShardPresence("policy")) {
				t.Errorf("MCP authority rule %s with %v fires in %v, want only the policy consumer", head, rule.Pos, shardList(rule.Fires))
			}
			return
		}
		t.Errorf("MCP authority rule %s with positive %q is missing from derivation map", head, containsPositive)
	}
	assertPolicyRuleFires("mcp_remote_confirmation_required", "mcp_tool_risk_source")
	assertPolicyRuleFires("mcp_remote_subject_current", "mcp_tool_risk_source")
	assertPolicyRuleFires("mcp_remote_permitted", "pending_action")
	var bad []string
	for _, f := range dm.SplitJoins {
		if acceptedSeam(f) {
			continue
		}
		bad = append(bad, "split join   "+describe(f))
	}
	for _, f := range dm.BlindNegations {
		if acceptedSeam(f) {
			continue
		}
		bad = append(bad, "blind negate "+describe(f))
	}
	// A negation over a derived predicate, and an aggregate over a derived
	// relation, are only as good as the facts the shard that evaluates them
	// holds: a predicate several shards each derive a part of is present
	// everywhere and complete nowhere.
	for _, f := range dm.IncompleteNegations {
		if acceptedSeam(f) {
			continue
		}
		bad = append(bad, "incomplete negate "+describe(f)+" complete in "+strings.Join(shardList(dm.Complete[f.Negated]), "/"))
	}
	for _, f := range dm.PartialAggregates {
		if acceptedSeam(f) {
			continue
		}
		bad = append(bad, "partial aggregate "+describe(f)+" complete in "+strings.Join(shardList(dm.Complete[f.Incomplete]), "/"))
	}
	t.Logf("corpus: %d predicates, %d split joins, %d blind negations, %d incomplete negations and %d partial aggregates (accepted residue included)",
		len(dm.Presence), len(dm.SplitJoins), len(dm.BlindNegations), len(dm.IncompleteNegations), len(dm.PartialAggregates))
	all := []string{"routing", "world", "tools", "policy", "campaign", "prompts", "cortex"}
	for _, p := range []string{"delegate_task", "next_action", "hollow_success", "turn_done", "injectable_context", "relevant_tool", "write_oriented_intent", "safe_action"} {
		t.Logf("query targets %-24s %v", p, dm.ShardsFor(p, all))
	}
	for _, s := range all {
		t.Logf("shared consumed by %-9s %d of %d", s, len(dm.Consumes[s]), len(SharedPredicates()))
	}
	for _, r := range dm.Rules {
		if r.Head == "delegate_task" && !r.Fires.All {
			t.Logf("delegate_task rule in %s fires in %v (pos %v)", r.File, shardList(r.Fires), r.Pos)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		for _, b := range bad {
			t.Error(b)
		}
		t.Fatalf("%d rules cannot fire on the sharded kernel; share the per-turn fact, re-home the family, or restructure the rule", len(bad))
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
		if !acceptedSeam(f) {
			extra++
		}
	}
	if extra < 20 {
		t.Fatalf("unsharing user_intent must split many rules; the analysis found only %d", extra)
	}
}

func TestShardJoin_ExceptionDoesNotExemptAnotherClauseWithSameHead(t *testing.T) {
	pin := seamPins[0]
	if !acceptedSeam(core.RuleFinding{File: pin.File, Head: pin.Head, Clause: pin.Clause}) {
		t.Fatal("pin failed positive control")
	}
	if acceptedSeam(core.RuleFinding{File: pin.File, Head: pin.Head, Clause: pin.Clause + " defective(X)."}) {
		t.Fatal("new clause inherited exemption")
	}
}

// Every shard kernel loads the intent corpus's facts at init, so a rule that
// joins one with an owned fact fires in the owner: the analysis must take those
// predicates as present everywhere, or it reports a split join that does not
// exist and routes a read away from the shard where the rule fires.
func TestShardJoin_IntentCorpusFactsArePresentInEveryShard(t *testing.T) {
	dm := buildProductionDerivationMap(t, SharedPredicates())
	for p := range core.IntentCorpusPredicates() {
		if pr, ok := dm.Presence[p]; ok && !pr.All {
			t.Errorf("intent corpus predicate %s is loaded into every shard but the analysis places it in %v", p, shardList(pr))
		}
	}
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
	shared := map[string]struct{}{}
	for _, p := range SharedPredicates() {
		shared[p] = struct{}{}
	}
	without, err := core.BuildAuditDerivationMap(schemas+"\n"+policy, nil, starvedPredicates(t), owners, shared, PredicateConsumerRoutes(), "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if pr := without.Presence["interrogative_type"]; pr.All {
		t.Fatalf("control: unmodelled, interrogative_type is a catch-all runtime fact, got %v", shardList(pr))
	}
}

// Every pinned seam must still be a finding: a pin whose rule was fixed, moved
// or deleted would otherwise stay behind and exempt the next clause that
// happens to read the same.
func TestShardJoin_EveryAcceptedSeamIsStillAFinding(t *testing.T) {
	dm := buildProductionDerivationMap(t, SharedPredicates())
	var findings []core.RuleFinding
	findings = append(findings, dm.SplitJoins...)
	findings = append(findings, dm.BlindNegations...)
	findings = append(findings, dm.IncompleteNegations...)
	findings = append(findings, dm.PartialAggregates...)
	for _, pin := range seamPins {
		found := false
		for _, f := range findings {
			if f.File == pin.File && normalizedClause(pin.Clause) == normalizedClause(f.Clause) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("accepted seam is no longer a finding; remove it from accepted_seams.json: %s: %s", pin.File, pin.Head)
		}
	}
}
