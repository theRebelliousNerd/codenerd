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

	wantConsumerPredicates := []string{
		"mcp_prompt_registered", "mcp_remote_operation", "mcp_remote_prompt",
		"mcp_remote_request", "mcp_remote_resource", "mcp_remote_reviewed",
		"mcp_resource_registered", "mcp_server_registered", "mcp_server_status",
		"mcp_tool_name", "mcp_tool_registered", "mcp_tool_risk",
		"mcp_tool_risk_source", "mcp_tool_schema_hash",
		// The world shard's mandate admission reads the turn's expert cards
		// (shard_experts.mg); the cards stay home with the dealing rules.
		"turn_expert_card",
	}
	wantConsumer := map[string]string{"turn_expert_card": "world"}
	routes := PredicateConsumerRoutes()
	if len(routes) != len(wantConsumerPredicates) {
		t.Fatalf("consumer route count = %d, want exactly %d: %+v", len(routes), len(wantConsumerPredicates), routes)
	}
	wantOwners := map[string]string{
		"mcp_tool_registered":     "world",
		"mcp_tool_risk":           "world",
		"mcp_tool_risk_source":    "world",
		"mcp_server_status":       "world",
		"mcp_tool_name":           "cortex",
		"mcp_tool_schema_hash":    "cortex",
		"mcp_server_registered":   "cortex",
		"mcp_remote_request":      "cortex",
		"mcp_remote_reviewed":     "cortex",
		"mcp_remote_operation":    "cortex",
		"mcp_remote_resource":     "cortex",
		"mcp_resource_registered": "cortex",
		"mcp_remote_prompt":       "cortex",
		"mcp_prompt_registered":   "cortex",
		"turn_expert_card":        "cortex",
	}
	for i, route := range routes {
		consumer := wantConsumer[route.Predicate]
		if consumer == "" {
			consumer = "policy"
		}
		if route.Predicate != wantConsumerPredicates[i] || route.Consumer != consumer {
			t.Fatalf("consumer route[%d] = %+v, want %q -> %s", i, route, wantConsumerPredicates[i], consumer)
		}
		owner := owners[route.Predicate]
		if owner == "" {
			owner = "cortex"
		}
		if owner != wantOwners[route.Predicate] {
			t.Fatalf("canonical owner for %q changed to %q, want %q", route.Predicate, owner, wantOwners[route.Predicate])
		}
		if _, isShared := sharedSet[route.Predicate]; isShared {
			t.Fatalf("consumer-routed predicate %q must not become globally shared", route.Predicate)
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
