package core

import "testing"

// A predicate Go asserts into its owner and that rules also derive is
// complete in the owner AND in every bounded consumer its asserted rows are
// copied to: a rule in the consumer that negates a conclusion over it is not
// an incomplete negation. (self_repair.mg derives project_forbidden_path,
// which the world owns and the policy shard holds a copy of; the
// constitution negates coder_edit_barred over it in the policy shard.)
func TestDerivationMap_AnOwnedDerivedPredicateIsCompleteInItsConsumers(t *testing.T) {
	const policy = `
Decl forbidden(P).
Decl marker().
Decl protected(P).
Decl action(P).
Decl barred(P).
Decl allowed(P).
protected("gate.mg").
forbidden(P) :- marker(), protected(P).
barred(P) :- action(P), forbidden(P).
allowed(P) :- action(P), !barred(P).
`
	owners := map[string]string{"forbidden": "world", "action": "policy"}
	shared := map[string]struct{}{"marker": {}}
	routes := []PredicateConsumerRoute{{Predicate: "forbidden", Consumer: "policy"}}
	m, err := BuildDerivationMapWithConsumerRoutes(policy, nil, owners, shared, routes, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	if c := m.Complete["forbidden"]; !c.Contains("world") || !c.Contains("policy") {
		t.Fatalf("forbidden complete in %+v, want its owner and its consumer", c)
	}
	for _, f := range m.IncompleteNegations {
		if f.Head == "allowed" {
			t.Fatalf("a negation in the consumer was called incomplete: %+v", f)
		}
	}
}
