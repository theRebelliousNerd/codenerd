package core

import "testing"

// An aggregate over more than one premise is evaluated through a scratch
// relation the engine writes into the store (mangle-go's rewrite: the body
// into "<head><n>__tmp", the do-transform over that). The cone evaluator
// cleared the aggregate's own rows and kept the scratch rows, so a retracted
// input still fed the group: the count below stayed 2 after one item died.
const coneTransformPolicy = `
	Decl ct_item(Group, Item).
	Decl ct_live(Item).
	Decl ct_live_count(Group, N).

	ct_live_count(G, N) :- ct_item(G, I), ct_live(I) |> do fn:group_by(G), let N = fn:count().
`

func TestConeEvaluationReDerivesAMultiPremiseAggregate(t *testing.T) {
	k := newPolicyKernel(t, setupMockKernel(t), coneTransformPolicy)
	for _, f := range []Fact{
		{Predicate: "ct_item", Args: []any{"/g", "/a"}},
		{Predicate: "ct_item", Args: []any{"/g", "/b"}},
		{Predicate: "ct_live", Args: []any{"/a"}},
		{Predicate: "ct_live", Args: []any{"/b"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	count := func() []Fact {
		t.Helper()
		rows, err := k.Query("ct_live_count")
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if rows := count(); len(rows) != 1 || rows[0].Args[1] != int64(2) {
		t.Fatalf("ct_live_count = %v, want 2", rows)
	}
	if err := k.Retract("ct_live"); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "ct_live", Args: []any{"/a"}}); err != nil {
		t.Fatal(err)
	}
	if rows := count(); len(rows) != 1 || rows[0].Args[1] != int64(1) {
		t.Fatalf("after /b died ct_live_count = %v, want 1", rows)
	}
	if err := k.Retract("ct_live"); err != nil {
		t.Fatal(err)
	}
	if rows := count(); len(rows) != 0 {
		t.Fatalf("with nothing live ct_live_count = %v, want no group", rows)
	}
}
