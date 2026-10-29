package core

import (
	"slices"
	"testing"

	"codenerd/internal/types"
)

// External audit N02 (2026-09-19): the orchestrator scheduled from an in-memory
// dependency scan whenever eligible_task derived nothing, so a policy that
// scheduled nothing was never seen to. With the kernel the only scheduler, the
// ordering rule has to leave a runnable task: a higher-priority task that
// depends on a lower-priority one in the same phase must not hold that
// dependency back, or neither runs.
func TestCampaignPolicy_ABlockedHigherPriorityTaskDoesNotHoldBackItsDependency(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	for _, f := range []types.Fact{
		{Predicate: "campaign", Args: []any{"c1", types.MangleAtom("/feature"), "campaign", "src", types.MangleAtom("/active")}},
		{Predicate: "campaign_phase", Args: []any{"p1", "c1", "build", 1, types.MangleAtom("/in_progress"), "ctx"}},
		{Predicate: "campaign_task", Args: []any{"t_ship", "p1", "ship it", types.MangleAtom("/pending"), types.MangleAtom("/file_modify")}},
		{Predicate: "campaign_task", Args: []any{"t_base", "p1", "lay the base", types.MangleAtom("/pending"), types.MangleAtom("/file_create")}},
		{Predicate: "task_priority", Args: []any{"t_ship", types.MangleAtom("/high")}},
		{Predicate: "task_priority", Args: []any{"t_base", types.MangleAtom("/normal")}},
		{Predicate: "task_dependency", Args: []any{"t_ship", "t_base"}},
	} {
		if aerr := k.Assert(f); aerr != nil {
			t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
		}
	}

	eligible, err := k.Query("eligible_task")
	if err != nil {
		t.Fatalf("query eligible_task: %v", err)
	}
	var got []string
	for _, f := range eligible {
		got = append(got, types.ExtractString(f.Args[0]))
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"t_base"}) {
		t.Fatalf("eligible_task = %v, want [t_base]: t_ship waits on t_base, so its priority must not hold t_base back", got)
	}

	blocked, err := k.Query("campaign_blocked")
	if err != nil {
		t.Fatalf("query campaign_blocked: %v", err)
	}
	if len(blocked) != 0 {
		t.Fatalf("campaign_blocked = %v, want none: t_base can run", blocked)
	}
}
