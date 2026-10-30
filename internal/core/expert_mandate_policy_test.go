package core

import (
	"testing"

	"codenerd/internal/types"
)

// An expert's REQUIRE line is advice unless the user's registry grants the
// agent mandate authority; an admitted mandate is discharged only by its test
// passing in the turn's own run, and while it is unmet the turn is not
// verified and the verdict names why.
func TestExpertMandates_AdmittedOnlyByAuthorityAndDischargedOnlyByAPass(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	turn := types.MangleAtom("/turn_mandate_probe")
	agent := types.MangleAtom("/schemaexpert")
	if err := k.LoadFacts([]Fact{
		{Predicate: "turn_evidence", Args: []any{turn, types.MangleAtom("/fix"), int64(1), int64(1), int64(1), types.MangleAtom("/false"), types.MangleAtom("/false")}},
		{Predicate: "turn_expert_card", Args: []any{turn, int64(0), agent, types.MangleAtom("/advise"), types.MangleAtom("/answered")}},
		{Predicate: "turn_card_mandate", Args: []any{turn, int64(0), int64(0), "TestDecoderKeepsOldColumn"}},
	}); err != nil {
		t.Fatal(err)
	}
	has := func(pred string, want ...any) bool {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			match := len(r.Args) >= len(want)
			for i, w := range want {
				if match && types.ExtractString(r.Args[i]) != types.ExtractString(w) {
					match = false
				}
			}
			if match {
				return true
			}
		}
		return false
	}
	if has("turn_mandate_admitted", turn) {
		t.Fatal("a REQUIRE from an agent with no mandate authority was admitted")
	}
	if err := k.Assert(Fact{Predicate: "shard_agent_authority", Args: []any{agent, types.MangleAtom("/mandate")}}); err != nil {
		t.Fatal(err)
	}
	if !has("turn_mandate_unmet", turn, "TestDecoderKeepsOldColumn") {
		t.Fatal("an authorized REQUIRE with no passing run is not unmet")
	}
	if has("turn_verified", turn) {
		t.Fatal("a turn with an unmet mandate was verified")
	}
	if !has("turn_missing_evidence", turn, types.MangleAtom("/mandate_unmet")) {
		t.Fatal("the verdict does not name the unmet mandate")
	}
	if err := k.Assert(Fact{Predicate: "turn_test_case", Args: []any{turn, "example.com/p", "TestDecoderKeepsOldColumn", types.MangleAtom("/pass"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	if has("turn_mandate_unmet", turn) || has("turn_missing_evidence", turn, types.MangleAtom("/mandate_unmet")) {
		t.Fatal("the mandate's test passed in the turn's run and it is still unmet")
	}
}
