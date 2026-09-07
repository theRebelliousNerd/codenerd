package shards

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode"

	"codenerd/internal/core"
)

// acceptedSeams is the documented residue: rules whose body facts live in
// different shards on purpose or for the architect to decide. Everything
// else must join within one shard. Keep this list in step with ACCEPTED in
// .claude/skills/codenerd-dogfood/scripts/shard_join_audit.py.
// Each exception pins the entire parsed clause, never just its head.
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
	dm, err := core.BuildDerivationMap(schemas+"\n"+policy, nil, owners, sharedSet, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	return dm
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
	t.Logf("corpus: %d predicates, %d split joins and %d blind negations (accepted residue included)",
		len(dm.Presence), len(dm.SplitJoins), len(dm.BlindNegations))
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
