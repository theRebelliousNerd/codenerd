package core

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A changed family member obliges a review of every peer per shared
// requirement; editing the peer or a recorded no-change decision settles it,
// and an open review withholds turn_verified.
func TestCodeFamilies_PolicyDerivesAndSettlesReviews(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	turn := types.MangleAtom("/turn_family_probe")
	ms := func(v string) types.MangleString { return types.MangleString(v) }
	base := []Fact{
		{Predicate: "code_family", Args: []any{ms("retry")}},
		{Predicate: "code_family_member", Args: []any{ms("retry"), ms("fn:a.Retry")}},
		{Predicate: "code_family_member", Args: []any{ms("retry"), ms("fn:b.Retry")}},
		{Predicate: "code_family_requirement", Args: []any{ms("retry"), ms("honor cancellation")}},
		{Predicate: "turn_changed_element", Args: []any{turn, ms("fn:a.Retry")}},
	}
	if err := k.LoadFacts(base); err != nil {
		t.Fatal(err)
	}
	open := func() []Fact {
		rows, err := k.Query("turn_family_open")
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows := open()
	if len(rows) != 1 || types.ExtractString(rows[0].Args[2]) != "fn:b.Retry" {
		t.Fatalf("turn_family_open = %v; want the unchanged peer fn:b.Retry", rows)
	}
	if got, _ := k.Query("turn_has_open_family_obligation"); len(got) != 1 {
		t.Fatalf("turn_has_open_family_obligation = %v", got)
	}
	ctx, _ := k.Query("injectable_context")
	found := false
	for _, f := range ctx {
		if s := types.ExtractString(f.Args[1]); len(s) > 0 && strings.Contains(s, "retry") && strings.Contains(s, "fn:b.Retry") && strings.Contains(s, "honor cancellation") {
			found = true
		}
	}
	if !found {
		t.Fatal("the open review is not named to the model")
	}
	if err := k.Assert(Fact{Predicate: "turn_family_no_change", Args: []any{turn, ms("retry"), ms("fn:b.Retry"), ms("b already returns on ctx.Done")}}); err != nil {
		t.Fatal(err)
	}
	if rows := open(); len(rows) != 0 {
		t.Fatalf("a recorded no-change decision did not settle the review: %v", rows)
	}
}

// A declared member that resolves to nothing is named to the model as a broken
// contract, but only in a turn that changed a member of that family.
func TestCodeFamilies_UnresolvedMemberIsNamedWhenTheFamilyIsTouched(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	turn := types.MangleAtom("/turn_family_unresolved")
	ms := func(v string) types.MangleString { return types.MangleString(v) }
	base := []Fact{
		{Predicate: "code_family", Args: []any{ms("retry")}},
		{Predicate: "code_family_member", Args: []any{ms("retry"), ms("fn:a.Retry")}},
		{Predicate: "code_family_member", Args: []any{ms("retry"), ms("fn:b.Gone")}},
		{Predicate: "code_family_requirement", Args: []any{ms("retry"), ms("honor cancellation")}},
		{Predicate: "code_family_member_unresolved", Args: []any{ms("retry"), ms("fn:b.Gone"), ms("names no Go function or method in the workspace")}},
	}
	if err := k.LoadFacts(base); err != nil {
		t.Fatal(err)
	}
	named := func() bool {
		ctx, _ := k.Query("injectable_context")
		for _, f := range ctx {
			if s := types.ExtractString(f.Args[1]); strings.Contains(s, "fn:b.Gone") && strings.Contains(s, "needs repair") {
				return true
			}
		}
		return false
	}
	if named() {
		t.Fatal("a broken contract was announced in a turn that touched no member of the family")
	}
	if err := k.Assert(Fact{Predicate: "turn_changed_element", Args: []any{turn, ms("fn:a.Retry")}}); err != nil {
		t.Fatal(err)
	}
	if !named() {
		t.Fatal("changing a member must name the unresolved peer as a contract to repair")
	}
}
