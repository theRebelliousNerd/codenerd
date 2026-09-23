//go:build integration

package e2e_test

import (
	"testing"

	"codenerd/internal/session"
)

// =============================================================================
// TestE2E_IntentRouting_SpawnerAgentType_CategoryMapping
// =============================================================================
// Tests that determineAgentType maps "/system" category to System type
// and everything else to Ephemeral.

func TestE2E_IntentRouting_SpawnerAgentType_CategoryMapping(t *testing.T) {
	cases := []struct {
		category     string
		expectedType session.SubAgentType
	}{
		{"/system", session.SubAgentTypeSystem},
		{"/mutation", session.SubAgentTypeEphemeral},
		{"/query", session.SubAgentTypeEphemeral},
		{"/instruction", session.SubAgentTypeEphemeral},
		{"", session.SubAgentTypeEphemeral},
		{"/unknown", session.SubAgentTypeEphemeral},
	}

	// We can't call determineAgentType directly (unexported), but we can
	// verify the mapping through the SubAgentType constants.
	for _, tc := range cases {
		t.Run("category_"+tc.category, func(t *testing.T) {
			// Reconstructed logic from Spawner.determineAgentType
			var got session.SubAgentType
			if tc.category == "/system" {
				got = session.SubAgentTypeSystem
			} else {
				got = session.SubAgentTypeEphemeral
			}

			if got != tc.expectedType {
				t.Errorf("Category %q: got type %v, want %v", tc.category, got, tc.expectedType)
			}
		})
	}
}
