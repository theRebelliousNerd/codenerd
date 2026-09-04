package campaign_test

import (
	"testing"

	"codenerd/internal/shards"
)

// TestCampaignShardManifestContract pins the campaign shard's predicate
// ownership to the full campaign fact family. Rules evaluate per shard, so a
// campaign predicate left out of the manifest lands in the catch-all and its
// joins with owned predicates stop deriving (e.g. campaign_phase without
// phase_category makes build_topology.mg emit missing_category).
//
// The ToFacts side is pinned dynamically in package campaign by
// TestToFacts_GoldenFixture_ShouldExerciseEveryEmitBranch against
// goldenToFactsCampaign(); this contract mirrors that required list here
// because an internal (package campaign) test cannot import shards without an
// import cycle (shards/system/campaign_runner.go imports campaign, so a
// package-campaign test importing shards is rejected with "import cycle not
// allowed in test"). Transitivity (fixture <-> required list <->
// manifest) keeps the pin exact: the golden test fails if the fixture emits
// anything outside the mirrored list, and this test fails if the manifest is
// missing anything inside it.
func TestCampaignShardManifestContract(t *testing.T) {
	// Predicates emitted by goldenToFactsCampaign().ToFacts(). Mirrors the
	// required list in TestToFacts_GoldenFixture_ShouldExerciseEveryEmitBranch.
	toFacts := []string{
		"campaign", "campaign_metadata", "campaign_goal", "campaign_progress",
		"context_profile", "source_document",
		"campaign_phase", "phase_category", "phase_objective", "phase_dependency",
		"phase_estimate", "context_compression",
		"campaign_task", "task_priority", "task_order", "task_dependency",
		"task_soft_dependency", "requires_resource", "task_sub_campaign",
		"task_artifact", "task_inference", "task_attempt", "task_retry_at",
		"task_error", "task_write_target",
	}

	// Runtime campaign facts asserted by the campaign package outside ToFacts:
	// syncCampaignFacts (plan_revision), orchestrator config/heartbeat/
	// checkpoint/result/replan paths, decomposer doc seeding, pager atoms,
	// plus the named facts the orchestrator queries/retracts or the session
	// asserts via mangle_updates. Collected via grep for `Predicate: "` in
	// internal/campaign (production asserts, retracts, and kernel queries).
	runtimeAsserted := []string{
		"plan_revision",
		"campaign_config",
		"failed_campaign_task_count_computed",
		"campaign_heartbeat",
		"phase_checkpoint",
		"task_result",
		"replan_trigger",
		"goal_topic",
		"doc_metadata",
		"doc_layer",
		"doc_tag",
		"phase_context_atom",
		"requirement_coverage",
		"current_phase",
		"campaign_dependency",
		"checkpoint_verdict",
		"task_verification",
	}

	required := make(map[string]struct{}, len(toFacts)+len(runtimeAsserted))
	for _, pred := range toFacts {
		required[pred] = struct{}{}
	}
	for _, pred := range runtimeAsserted {
		required[pred] = struct{}{}
	}

	var owned map[string]struct{}
	for _, m := range shards.DefaultShardPredicateManifests() {
		if m.Domain == "campaign" {
			owned = make(map[string]struct{}, len(m.OwnedPredicates))
			for _, pred := range m.OwnedPredicates {
				owned[pred] = struct{}{}
			}
			break
		}
	}
	if owned == nil {
		t.Fatal(`no "campaign" manifest in shards.DefaultShardPredicateManifests()`)
	}

	for pred := range required {
		if _, ok := owned[pred]; !ok {
			t.Errorf("campaign manifest is missing predicate %q; routing will split it away from the rules that join it", pred)
		}
	}
}
