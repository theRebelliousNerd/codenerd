package core

import (
	"testing"
	"time"
)

// A three-shard toy program: alpha owns alpha_fact, beta owns beta_fact,
// shared_sig is shared, everything else lands in the catch-all.
const toyPolicy = `
# Policy Module: toy.mg
Decl alpha_fact(X).
Decl beta_fact(X).
Decl loose_fact(X).
Decl shared_sig(X).
Decl table(X).
Decl local_head(X).
Decl split_head(X).
Decl blind_head(X).
Decl shared_head(X).
Decl mixed_head(X).

table(/a).
table(/b).

local_head(X) :- alpha_fact(X), table(X).
split_head(X) :- alpha_fact(X), beta_fact(X).
blind_head(X) :- alpha_fact(X), !beta_fact(X).
shared_head(X) :- shared_sig(X), loose_fact(X).
mixed_head(X) :- shared_sig(X), table(X).
mixed_head(X) :- alpha_fact(X), shared_sig(X).
Decl via_head(X).
via_head(X) :- beta_fact(X), mixed_head(X).
`

func TestDerivationMap_ToyProgram(t *testing.T) {
	owners := map[string]string{"alpha_fact": "alpha", "beta_fact": "beta"}
	shared := map[string]struct{}{"shared_sig": {}}
	m, err := BuildDerivationMap(toyPolicy, nil, owners, shared, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if !m.Presence["table"].All {
		t.Errorf("program fact table must be present everywhere: %+v", m.Presence["table"])
	}
	if !m.Presence["local_head"].Equals(SingleShardPresence("alpha")) {
		t.Errorf("local_head must derive only in alpha: %+v", m.Presence["local_head"])
	}
	if !m.Presence["split_head"].IsEmpty() {
		t.Errorf("split_head must derive nowhere: %+v", m.Presence["split_head"])
	}
	if !m.Presence["shared_head"].Equals(SingleShardPresence("cortex")) {
		t.Errorf("shared_head must derive in the catch-all where loose_fact lives: %+v", m.Presence["shared_head"])
	}
	if len(m.SplitJoins) != 1 || m.SplitJoins[0].Head != "split_head" {
		t.Errorf("want exactly one split join on split_head, got %+v", m.SplitJoins)
	}
	if len(m.BlindNegations) != 1 || m.BlindNegations[0].Head != "blind_head" || m.BlindNegations[0].Negated != "beta_fact" {
		t.Errorf("want exactly one blind negation on blind_head/!beta_fact, got %+v", m.BlindNegations)
	}
	if _, ok := m.Consumes["cortex"]["shared_sig"]; !ok {
		t.Errorf("the catch-all consumes shared_sig through shared_head: %+v", m.Consumes)
	}
	if _, ok := m.Consumes["alpha"]["shared_sig"]; !ok {
		t.Errorf("alpha reads shared_sig through mixed_head's second rule: %+v", m.Consumes["alpha"])
	}
	// beta fires via_head, whose body needs mixed_head, whose rules need
	// shared_sig: the replica must reach beta transitively.
	if _, ok := m.Consumes["beta"]["shared_sig"]; !ok {
		t.Errorf("beta needs shared_sig through via_head -> mixed_head: %+v", m.Consumes["beta"])
	}
	if got := m.ShardsFor("via_head", []string{"alpha", "beta", "cortex"}); len(got) != 1 || got[0] != "beta" {
		t.Errorf("ShardsFor(via_head) = %v, want [beta]", got)
	}
	got := m.ShardsFor("local_head", []string{"alpha", "beta", "cortex"})
	if len(got) != 1 || got[0] != "alpha" {
		t.Errorf("ShardsFor(local_head) = %v, want [alpha]", got)
	}
	if got := m.ShardsFor("table", []string{"alpha", "beta", "cortex"}); len(got) != 3 {
		t.Errorf("ShardsFor(table) must be every shard, got %v", got)
	}
	// mixed_head can exist everywhere (its first rule needs only shared and
	// program facts) but a query need only visit the catch-all for that rule
	// and alpha for the second: never beta.
	if !m.Presence["mixed_head"].All {
		t.Errorf("mixed_head presence must be All: %+v", m.Presence["mixed_head"])
	}
	got = m.ShardsFor("mixed_head", []string{"alpha", "beta", "cortex"})
	if len(got) != 2 || got[0] != "alpha" || got[1] != "cortex" {
		t.Errorf("ShardsFor(mixed_head) = %v, want [alpha cortex]", got)
	}
}

// TestDerivationMap_DefaultCorpus builds the map over the embedded corpus
// with the routing/world/policy ownership the manifests declare (copied here;
// internal/core cannot import internal/shards).
func TestDerivationMap_DefaultCorpus(t *testing.T) {
	schemas, policy, err := DefaultCorpusText()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{
		"derived_mode":  "routing",
		"file_topology": "world", "symbol_graph": "world", "diagnostic": "world",
		"dependency_link": "world", "code_element": "world", "file_dir": "world",
		"in_scope": "world", "active_file": "world",
		"pending_action": "policy", "permitted": "policy", "routing_result": "policy",
	}
	shared := map[string]struct{}{
		"user_intent": {}, "active_shard": {}, "current_time": {}, "executive_processed_intent": {},
	}
	start := time.Now()
	m, err := BuildDerivationMap(schemas+"\n"+policy, nil, owners, shared, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("building the map over the corpus took %v, want < 2s", d)
	}
	if !m.Presence["task_stage"].All {
		t.Errorf("task_stage joins shared user_intent with a program table, must be everywhere: %+v", m.Presence["task_stage"])
	}
	if _, ok := m.Consumes["world"]["user_intent"]; !ok {
		t.Error("world rules (codedom_edit.mg next_action(/open_file)) consume user_intent")
	}
	// Whole-corpus correctness against the real manifests is pinned by
	// internal/shards/shard_join_audit_test.go; this fixture only proves the
	// parser and the fixpoint handle the full corpus.
	t.Logf("corpus: %d predicates, %d split joins, %d blind negations", len(m.Presence), len(m.SplitJoins), len(m.BlindNegations))
}
