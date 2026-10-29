package system

import (
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The planner runs whenever a campaign is current, and what its model writes
// through mangle_updates goes through the one allowlist every envelope surface
// uses. Its own list of prefixes let the model schedule a task the policy held
// back (eligible_task), force a replan (replan_trigger), and write the
// campaign rows phase and campaign completion are derived from.
func TestSessionPlanner_ModelUpdatesCannotWriteCampaignState(t *testing.T) {
	planner := NewSessionPlannerShard()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	planner.Kernel = kernel

	planner.routeControlPacketToKernel(&articulation.ControlPacket{MangleUpdates: []string{
		`eligible_task("/task_held_back")`,
		`replan_trigger("/campaign_x", /checkpoint_failed, 1)`,
		`campaign_task("/task_x", "/phase_x", "done", /completed, /file_modify)`,
		`phase_checkpoint("/phase_x", /tests_pass, /true, "passed", 1)`,
		`observation("planner_note", "the build is slow")`,
	}})

	for _, pred := range []string{"eligible_task", "replan_trigger", "campaign_task", "phase_checkpoint"} {
		facts, err := kernel.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		for _, f := range facts {
			if len(f.Args) > 0 && types.ExtractString(f.Args[0]) != "" {
				t.Errorf("the planner's model wrote %s%v", pred, f.Args)
			}
		}
	}
	notes, err := kernel.Query("observation")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range notes {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == "planner_note" {
			found = true
		}
	}
	if !found {
		t.Fatal("an observation the allowlist admits was not asserted")
	}
}
