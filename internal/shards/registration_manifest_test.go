package shards

import "testing"

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
	if owner, owned := owners["next_action"]; owned {
		t.Fatalf("next_action must be unowned so queries fan out to every shard that derives it, got owner %q", owner)
	}
}
