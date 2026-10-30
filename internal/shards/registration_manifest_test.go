package shards

import (
	"codenerd/internal/core"
	"testing"
)

func TestDefaultShardPredicateManifestsAreUnambiguous(t *testing.T) {
	manifests := DefaultShardPredicateManifests()
	if len(manifests) == 0 {
		t.Fatal("default predicate manifests are empty")
	}

	domains := make(map[string]struct{}, len(manifests))
	owners := make(map[string]string)
	for _, manifest := range manifests {
		if manifest.Domain == "" {
			t.Fatal("manifest has empty domain")
		}
		if _, exists := domains[manifest.Domain]; exists {
			t.Fatalf("duplicate manifest domain %q", manifest.Domain)
		}
		domains[manifest.Domain] = struct{}{}
		for _, predicate := range manifest.OwnedPredicates {
			if predicate == "" {
				t.Fatalf("domain %q owns an empty predicate", manifest.Domain)
			}
			if owner, exists := owners[predicate]; exists {
				t.Fatalf("predicate %q owned by both %q and %q", predicate, owner, manifest.Domain)
			}
			owners[predicate] = manifest.Domain
		}
	}

	for _, predicate := range []string{"pending_action", "permitted_action", "permission_check_result", "permitted"} {
		if owner := owners[predicate]; owner != "policy" {
			t.Fatalf("authorization predicate %q owner = %q, want policy", predicate, owner)
		}
	}
	if _, ok := domains["cortex"]; !ok {
		t.Fatal("catch-all cortex manifest is missing")
	}

	// Shared predicates are replicated to every shard; one cannot also be
	// authoritative in a single shard. user_intent must be shared, not
	// owned: rules in every domain join it (item 55).
	shared := SharedPredicates()
	if len(shared) == 0 {
		t.Fatal("shared predicate list is empty")
	}
	sharedSet := make(map[string]struct{}, len(shared))
	for _, p := range shared {
		if owner, owned := owners[p]; owned {
			t.Fatalf("predicate %q is both shared and owned by %q", p, owner)
		}
		sharedSet[p] = struct{}{}
	}
	if _, ok := sharedSet["user_intent"]; !ok {
		t.Fatal("user_intent must be a shared predicate")
	}

	// Routes are sorted by predicate. The MCP control plane is copied to the
	// policy shard; the plan-intelligence inputs (policy/intelligence.mg) are
	// copied to the world shard, whose model they are joined against.
	type route struct{ consumer, owner string }
	wantRoutes := map[string]route{
		"mcp_prompt_registered":       {"policy", "cortex"},
		"mcp_remote_operation":        {"policy", "cortex"},
		"mcp_remote_prompt":           {"policy", "cortex"},
		"mcp_remote_request":          {"policy", "cortex"},
		"mcp_remote_resource":         {"policy", "cortex"},
		"mcp_remote_reviewed":         {"policy", "cortex"},
		"mcp_resource_registered":     {"policy", "cortex"},
		"mcp_server_registered":       {"policy", "cortex"},
		"mcp_server_status":           {"policy", "world"},
		"mcp_tool_name":               {"policy", "cortex"},
		"mcp_tool_registered":         {"policy", "world"},
		"mcp_tool_risk":               {"policy", "world"},
		"mcp_tool_risk_source":        {"policy", "world"},
		"mcp_tool_schema_hash":        {"policy", "cortex"},
		"task_artifact":               {"world", "campaign"},
		"intelligence_churn_hotspot":  {"world", "campaign"},
		"intelligence_code_pattern":   {"world", "campaign"},
		"intelligence_safety_warning": {"world", "campaign"},
		"intelligence_test_coverage":  {"world", "campaign"},
		// The world shard's mandate admission reads the turn's expert cards
		// (shard_experts.mg); the cards stay home with the dealing rules.
		"turn_expert_card": {"world", "cortex"},
	}
	routes := PredicateConsumerRoutes()
	if len(routes) != len(wantRoutes) {
		t.Fatalf("consumer route count = %d, want exactly %d: %+v", len(routes), len(wantRoutes), routes)
	}
	for _, r := range routes {
		want, ok := wantRoutes[r.Predicate]
		if !ok || r.Consumer != want.consumer {
			t.Fatalf("consumer route %+v is not expected (want %+v)", r, want)
		}
		owner := owners[r.Predicate]
		if owner == "" {
			owner = "cortex"
		}
		if owner != want.owner {
			t.Fatalf("canonical owner for %q changed to %q, want %q", r.Predicate, owner, want.owner)
		}
		if _, isShared := sharedSet[r.Predicate]; isShared {
			t.Fatalf("consumer-routed predicate %q must not become globally shared", r.Predicate)
		}
	}
	if owner, owned := owners["next_action"]; owned {
		t.Fatalf("next_action must be unowned so queries fan out to every shard that derives it, got owner %q", owner)
	}
}

// Cleanup's completion witness must reach the same kernel shard as campaign,
// for both the positive archive join and the cleanup rule's negation.
func TestCampaignCleanup_CompletionChangesTheShardedAction(t *testing.T) {
	cortex := bootGateCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := gatePair{t: t, cortex: cortex, single: single}
	both.assert(gf("campaign", ms("finished-campaign"), ma("/feature"), ms("Finished"), ms("brief"), ma("/completed")))
	both.has("next_action", "/campaign_cleanup", "")
	both.lacks("next_action", "/archive_campaign", "")
	both.assert(gf("campaign_cleaned_up", ms("finished-campaign")))
	both.lacks("next_action", "/campaign_cleanup", "")
	both.has("next_action", "/archive_campaign", "")
}
