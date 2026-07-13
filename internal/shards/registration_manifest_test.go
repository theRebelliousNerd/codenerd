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
}
