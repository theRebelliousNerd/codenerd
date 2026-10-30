package core

import "testing"

// The shard-join analysis used to see three defects: a split join, a negated
// base fact owned elsewhere, and (item 55) an unshared per-turn fact. These
// pin the classes it missed. Presence says where a fact CAN exist; a predicate
// several shards each derive a part of is present in all of them and complete
// in none, so a negation over it under-blocks and an aggregate over it counts
// a part.

const completenessPolicy = `
# Policy Module: completeness.mg
Decl alpha_fact(X).
Decl beta_fact(X).
Decl shared_sig(X).
Decl q(X).
Decl t(X).
Decl v(X).
Decl r(X).
Decl s(X).
Decl u(X).
Decl w(X).
Decl n(N).
Decl n_alpha(N).
Decl m(Y).
Decl h(X).
Decl cfg(X).
Decl x(X).

# cfg has program facts in every shard and runtime rows in alpha only.
cfg(/default).
# x fires everywhere (shared) and negates cfg: the catch-all misses alpha's rows.
x(X) :- shared_sig(X), !cfg(X).

# q: one rule per shard, so q is present in alpha and beta and complete in neither.
q(X) :- alpha_fact(X).
q(X) :- beta_fact(X).
# t: every rule in alpha.
t(X) :- alpha_fact(X).

# Incomplete: alpha fires r, and alpha never sees beta's q rows.
r(X) :- alpha_fact(X), !q(X).
# Near miss: t is complete in alpha.
s(X) :- alpha_fact(X), !t(X).

# v is present in every shard (one rule reads only a shared fact) and complete in beta only.
v(X) :- shared_sig(X).
v(X) :- beta_fact(X).
# u fires everywhere and is read in the catch-all, which is not beta.
u(X) :- shared_sig(X), !v(X).
# Near miss: a negated base fact is BlindNegations' finding, not a second one.
w(X) :- alpha_fact(X), !beta_fact(X).

# Aggregates.
n(N) :- q(X) |> do fn:group_by(), let N = fn:count().
n_alpha(N) :- t(X) |> do fn:group_by(), let N = fn:count().
# A let-only transform is per-row, not an aggregate.
m(Y) :- q(X) |> let Y = fn:plus(1, 1).
`

func completenessMap(t *testing.T) *DerivationMap {
	t.Helper()
	owners := map[string]string{"alpha_fact": "alpha", "beta_fact": "beta", "cfg": "alpha"}
	shared := map[string]struct{}{"shared_sig": {}}
	m, err := BuildDerivationMapWithConsumerRoutes(completenessPolicy, nil, owners, shared, nil, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func findingsFor(list []RuleFinding, head string) []RuleFinding {
	var out []RuleFinding
	for _, f := range list {
		if f.Head == head {
			out = append(out, f)
		}
	}
	return out
}

func TestDerivationMap_CompleteIsWhereNoFactIsMissing(t *testing.T) {
	m := completenessMap(t)
	if !m.Presence["q"].Equals(unionPresence("alpha", "beta")) {
		t.Fatalf("control: q is present in alpha and beta, got %+v", m.Presence["q"])
	}
	if !m.Complete["q"].IsEmpty() {
		t.Errorf("q is derived by one alpha rule and one beta rule: complete nowhere, got %+v", m.Complete["q"])
	}
	if !m.Complete["t"].Equals(SingleShardPresence("alpha")) {
		t.Errorf("t is complete in alpha, got %+v", m.Complete["t"])
	}
	if !m.Presence["v"].All || !m.Complete["v"].Equals(SingleShardPresence("beta")) {
		t.Errorf("v is present everywhere and complete in beta only, got presence %+v complete %+v", m.Presence["v"], m.Complete["v"])
	}
}

func TestDerivationMap_IncompleteNegationOverADerivedPredicate(t *testing.T) {
	m := completenessMap(t)
	// The existing check sees neither: q is present where r fires, and v is
	// present everywhere.
	for _, f := range m.BlindNegations {
		if f.Head == "r" || f.Head == "u" {
			t.Fatalf("control: %s must not be a blind negation (that class is for owned base facts): %+v", f.Head, f)
		}
	}
	if got := findingsFor(m.IncompleteNegations, "r"); len(got) != 1 || got[0].Negated != "q" {
		t.Errorf("r negates q, which alpha holds only part of: %+v", m.IncompleteNegations)
	}
	if got := findingsFor(m.IncompleteNegations, "u"); len(got) != 1 || got[0].Negated != "v" {
		t.Errorf("u fires everywhere over v, complete only in beta; the catch-all copy is read: %+v", m.IncompleteNegations)
	}
	if got := findingsFor(m.IncompleteNegations, "s"); len(got) != 0 {
		t.Errorf("s negates t, complete in alpha where s fires: %+v", got)
	}
	if got := findingsFor(m.IncompleteNegations, "w"); len(got) != 0 {
		t.Errorf("w negates a base fact; BlindNegations owns that class: %+v", got)
	}
	if got := findingsFor(m.BlindNegations, "w"); len(got) != 1 {
		t.Errorf("control: w is a blind negation: %+v", m.BlindNegations)
	}
}

// A base predicate with program facts and an owner is present everywhere and
// complete only where its runtime rows land: the program facts hide the owner
// from the presence lattice.
func TestDerivationMap_ProgramFactsDoNotHideAnOwnersRows(t *testing.T) {
	m := completenessMap(t)
	if got := findingsFor(m.IncompleteNegations, "x"); len(got) != 1 || got[0].Negated != "cfg" {
		t.Errorf("x negates cfg, whose runtime rows live in alpha only: %+v", m.IncompleteNegations)
	}
	if !m.Presence["cfg"].All {
		t.Errorf("control: cfg is present everywhere through its program fact, got %+v", m.Presence["cfg"])
	}
}

func TestDerivationMap_PartialAggregate(t *testing.T) {
	m := completenessMap(t)
	got := findingsFor(m.PartialAggregates, "n")
	if len(got) != 1 || got[0].Incomplete != "q" {
		t.Errorf("n counts q, which every shard it fires in holds only part of: %+v", m.PartialAggregates)
	}
	if got := findingsFor(m.PartialAggregates, "n_alpha"); len(got) != 0 {
		t.Errorf("n_alpha counts t, complete in alpha where it fires: %+v", got)
	}
	if got := findingsFor(m.PartialAggregates, "m"); len(got) != 0 {
		t.Errorf("a let-only transform is per-row, not an aggregate: %+v", got)
	}
}

// A premise under a temporal operator is still a premise: dropping it hid it
// from every shard check, so a join across shards through one never showed.
func TestDerivationMap_TemporalPremiseJoinsItsPredicate(t *testing.T) {
	policy := `
# Policy Module: temporal.mg
Decl alpha_fact(X).
Decl beta_fact(X) temporal.
Decl h(X).
h(X) :- alpha_fact(X), <-[0m, 10m] beta_fact(X)@[now].
`
	owners := map[string]string{"alpha_fact": "alpha", "beta_fact": "beta"}
	m, err := BuildDerivationMapWithConsumerRoutes(policy, nil, owners, nil, nil, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if got := findingsFor(m.SplitJoins, "h"); len(got) != 1 {
		t.Errorf("h joins alpha_fact with a temporal beta_fact: a split join, got %+v", m.SplitJoins)
	}
}

// A predicate nothing produces has no rows in any shard. A rule that needs one
// never fires, so where its other premises live decides nothing; and a
// predicate whose every rule is dead is starved in its turn.
func TestBuildAuditDerivationMap_StarvedPredicatesAndDeadRules(t *testing.T) {
	policy := `
# Policy Module: starved.mg
Decl alpha_fact(X).
Decl beta_fact(X).
Decl never(X).
Decl via_dead(X).
Decl downstream(X).
Decl live(X).
Decl split_on_a_starved_input(X).
Decl split_for_real(X).

split_on_a_starved_input(X) :- alpha_fact(X), beta_fact(X), never(X).
split_for_real(X) :- alpha_fact(X), beta_fact(X).
via_dead(X) :- never(X).
downstream(X) :- alpha_fact(X), via_dead(X).
live(X) :- alpha_fact(X).
`
	owners := map[string]string{"alpha_fact": "alpha", "beta_fact": "beta"}
	plain, err := BuildDerivationMapWithConsumerRoutes(policy, nil, owners, nil, nil, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if len(findingsFor(plain.SplitJoins, "split_on_a_starved_input")) != 1 {
		t.Fatalf("control: without the starved set the rule is a split join: %+v", plain.SplitJoins)
	}
	audit, err := BuildAuditDerivationMap(policy, map[string]struct{}{"never": {}}, owners, nil, nil, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if got := findingsFor(audit.SplitJoins, "split_on_a_starved_input"); len(got) != 0 {
		t.Errorf("a rule with a starved premise never fires and is not reported: %+v", got)
	}
	if got := findingsFor(audit.SplitJoins, "split_for_real"); len(got) != 1 {
		t.Errorf("a genuine split join is still reported: %+v", audit.SplitJoins)
	}
	if !audit.Presence["via_dead"].All {
		t.Errorf("via_dead has only dead rules, so it is starved and present everywhere: %+v", audit.Presence["via_dead"])
	}
	for _, r := range audit.Rules {
		if r.Head == "downstream" || r.Head == "via_dead" || r.Head == "split_on_a_starved_input" {
			t.Errorf("dead rule %s survived: %+v", r.Head, r)
		}
	}
	if !audit.Presence["live"].Equals(SingleShardPresence("alpha")) {
		t.Errorf("control: live is unaffected, got %+v", audit.Presence["live"])
	}
}

func unionPresence(a, b string) Presence {
	return SingleShardPresence(a).Join(SingleShardPresence(b))
}
