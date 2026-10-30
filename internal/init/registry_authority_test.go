package init

import (
	"slices"
	"testing"
)

// Init recreates an agent it created before and replaces its registry entry;
// the user's authority grant on that entry is not init's to drop.
func TestMergeAgentRegistry_KeepsTheUsersAuthorityGrant(t *testing.T) {
	existing := map[string]CreatedAgent{"GoExpert": {Name: "GoExpert", Authority: []string{"mandate"}}}
	merged := mergeAgentRegistry([]CreatedAgent{{Name: "goexpert", Domains: []string{"go"}}}, existing, nil)
	if len(merged) != 1 || !slices.Contains(merged[0].Authority, "mandate") || !slices.Contains(merged[0].Domains, "go") {
		t.Fatalf("merged = %+v, want the recreated entry with the grant kept", merged)
	}
}
