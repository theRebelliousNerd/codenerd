package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A granted mandate holds the turn unverified until its test passes in the
// turn's own run -- through the real sharded kernel, where the facts it joins
// live in different domain shards.
func TestBootMandateBlocksVerificationUntilItsTestPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	k := cortex.Kernel
	turn := types.MangleAtom("/turn_mandate_boot")
	agent := types.MangleAtom("/schemaexpert")
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "turn_evidence", Args: []any{turn, types.MangleAtom("/fix"), int64(1), int64(1), int64(1), types.MangleAtom("/false"), types.MangleAtom("/false")}},
		{Predicate: "shard_agent_authority", Args: []any{agent, types.MangleAtom("/mandate")}},
		{Predicate: "turn_expert_card", Args: []any{turn, int64(0), agent, types.MangleAtom("/advise"), types.MangleAtom("/answered")}},
		{Predicate: "turn_card_mandate", Args: []any{turn, int64(0), int64(0), "TestDecoderKeepsOldColumn"}},
	}); err != nil {
		t.Fatalf("assert: %v", err)
	}
	has := func(pred string, want ...any) bool {
		t.Helper()
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
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
	if !has("turn_mandate_unmet", turn, "TestDecoderKeepsOldColumn") {
		t.Fatal("the granted mandate is not unmet on the sharded kernel")
	}
	if has("turn_verified", turn) {
		t.Fatal("a turn with an unmet mandate was verified")
	}
	if !has("turn_missing_evidence", turn, types.MangleAtom("/mandate_unmet")) {
		t.Fatal("the verdict does not name the unmet mandate")
	}
	if err := k.Assert(types.Fact{Predicate: "turn_test_case", Args: []any{turn, "example.com/p", "TestDecoderKeepsOldColumn", types.MangleAtom("/pass"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	if has("turn_mandate_unmet", turn) || has("turn_missing_evidence", turn, types.MangleAtom("/mandate_unmet")) {
		t.Fatal("the mandate's test passed in the turn's run and the turn is still held for it")
	}
}
