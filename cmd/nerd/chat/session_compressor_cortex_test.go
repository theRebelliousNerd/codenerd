package chat

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdsystem "codenerd/internal/system"
)

// The session's context compressor sees the session's whole kernel.
//
// It is built on the domain Cortex's catch-all shard, which holds only the
// facts no other shard owns. Measured 2026-09-25: a block_commit the world
// shard derives never reached it, so getCoreFacts -- whose job is to keep the
// constitutional facts in every prompt -- could not retain it. The compressor
// now asks through the Cortex (newSessionCompressor, the constructor boot
// uses).
func TestSessionCompressor_RetainsFactsOtherShardsDerive(t *testing.T) {
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	primary := ck.GetPrimaryRealKernel()
	for _, f := range []core.Fact{
		{Predicate: "modified", Args: []any{"internal/x/y.go"}},
		{Predicate: "user_intent", Args: []any{"/current_intent", "/mutation", "/fix", "internal/x/y.go", "/none"}},
	} {
		if err := ck.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}

	// Precondition: the Cortex holds a retained fact the catch-all does not.
	everywhere, err := ck.Query("block_commit")
	if err != nil {
		t.Fatal(err)
	}
	local, err := primary.Query("block_commit")
	if err != nil {
		t.Fatal(err)
	}
	onPrimary := map[string]bool{}
	for _, f := range local {
		onPrimary[f.String()] = true
	}
	var elsewhere []core.Fact
	for _, f := range everywhere {
		if !onPrimary[f.String()] {
			elsewhere = append(elsewhere, f)
		}
	}
	if len(elsewhere) == 0 {
		t.Fatalf("precondition: every block_commit row (%v) is on the catch-all shard; this test proves nothing", everywhere)
	}

	compressor := newSessionCompressor(ck, primary, nil, nil, config.DefaultContextWindowConfig())
	built, err := compressor.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	for _, f := range elsewhere {
		arg, _ := f.Args[0].(string)
		if !strings.Contains(built.CoreFacts, arg) {
			t.Errorf("block_commit(%q), derived outside the catch-all shard, is not in the session's constitutional section:\n%s",
				arg, built.CoreFacts)
		}
	}
	if n := compressor.GetSelectionStats().RetentionFloorUsed; n != 0 {
		t.Errorf("the kernel made no retention decision on the production kernel (floor used %d times)", n)
	}
}
