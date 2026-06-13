package ux

import (
	"slices"
	"testing"
)

func TestMigrateFromOldVersionPreservesData(t *testing.T) {
	ws := t.TempDir()
	oldPrefs := map[string]any{
		"agent_selection": map[string]any{
			"accepted_agents":         []any{"coder", "tester"},
			"rejected_agents":         []any{"nemesis"},
			"last_interactive":        "2026-01-01",
			"auto_accept_recommended": true,
		},
	}

	result, err := migrateFromOldVersion(ws, oldPrefs)
	if err != nil {
		t.Fatalf("migrateFromOldVersion: %v", err)
	}
	if !result.WasMigrated || result.ToVersion != PreferencesVersion {
		t.Errorf("unexpected migration result: %+v", result)
	}
	for _, key := range []string{"accepted_agents", "rejected_agents", "last_interactive", "auto_accept_recommended"} {
		if !slices.Contains(result.PreservedData, key) {
			t.Errorf("expected %q to be preserved, got %v", key, result.PreservedData)
		}
	}
	if !slices.Contains(result.DefaultsApplied, "productive_state") {
		t.Errorf("expected productive_state default, got %v", result.DefaultsApplied)
	}

	// Migration persisted prefs, so the journey state is now productive.
	if got := GetUserJourneyState(ws); got != StateProductive {
		t.Errorf("GetUserJourneyState after migration=%v, want %v", got, StateProductive)
	}
}

func TestGetUserJourneyStateDefaultsToNew(t *testing.T) {
	// An empty workspace has no saved preferences -> brand-new journey state.
	if got := GetUserJourneyState(t.TempDir()); got != StateNew {
		t.Errorf("GetUserJourneyState on empty workspace=%v, want %v", got, StateNew)
	}
}
