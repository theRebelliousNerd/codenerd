package ux

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

func TestIsFirstRun(t *testing.T) {
	workspace := t.TempDir()
	if !IsFirstRun(workspace) {
		t.Fatalf("expected first run when .nerd does not exist")
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("failed to create .nerd: %v", err)
	}
	if IsFirstRun(workspace) {
		t.Fatalf("expected not first run when .nerd exists")
	}
}

func TestMigratePreferencesNewUser(t *testing.T) {
	workspace := t.TempDir()
	result, err := MigratePreferences(workspace)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !result.WasMigrated {
		t.Fatalf("expected migration for new user")
	}

	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if pm.GetJourneyState() != StateNew {
		t.Fatalf("expected state new")
	}
}

func TestMigratePreferencesExistingUser(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("failed to create .nerd: %v", err)
	}
	result, err := MigratePreferences(workspace)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !result.WasMigrated {
		t.Fatalf("expected migration for existing user")
	}

	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if pm.GetJourneyState() != StateProductive {
		t.Fatalf("expected productive state for existing user")
	}
	if !pm.IsOnboardingComplete() {
		t.Fatalf("expected onboarding complete for existing user")
	}
	if pm.Get().Metrics.SessionsCount != 20 {
		t.Fatalf("expected sessions count default for existing user")
	}
}

func TestShouldShowOnboarding(t *testing.T) {
	workspace := t.TempDir()
	if !ShouldShowOnboarding(workspace) {
		t.Fatalf("expected onboarding on first run")
	}

	t.Setenv("NERD_SKIP_ONBOARDING", "1")
	if ShouldShowOnboarding(workspace) {
		t.Fatalf("expected onboarding to be skipped by env")
	}
	t.Setenv("NERD_SKIP_ONBOARDING", "")

	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("failed to create .nerd: %v", err)
	}
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if err := pm.MarkOnboardingComplete(); err != nil {
		t.Fatalf("mark onboarding complete failed: %v", err)
	}
	if err := pm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if ShouldShowOnboarding(workspace) {
		t.Fatalf("expected onboarding to be hidden for completed user")
	}
}

func TestRecordSessionStart(t *testing.T) {
	workspace := t.TempDir()
	recorder := NewPreferencesManager(workspace)
	if err := recorder.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if err := recorder.RecordSessionStart(); err != nil {
		t.Fatalf("record session start failed: %v", err)
	}
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if pm.Get().Metrics.SessionsCount != 1 {
		t.Fatalf("expected sessions count increment")
	}
	if pm.Get().Metrics.LastSession == "" {
		t.Fatalf("expected last session timestamp")
	}
}

func TestCheckJourneyTransition(t *testing.T) {
	workspace := t.TempDir()
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pm.preferences.UserJourney.State = StateLearning
	pm.preferences.Metrics.SessionsCount = 15
	pm.preferences.Metrics.SuccessfulTasks = 20
	pm.preferences.Metrics.ClarificationsNeeded = 2
	if err := pm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	newState, transitioned, err := pm.CheckJourneyTransition()
	if err != nil {
		t.Fatalf("check transition failed: %v", err)
	}
	if !transitioned || newState != StateProductive {
		t.Fatalf("expected transition to productive")
	}
}

func TestGetExperienceLevelFromPreferences(t *testing.T) {
	workspace := t.TempDir()
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pm.preferences.UserJourney.State = StatePower
	if err := pm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	level := GetExperienceLevelFromPreferences(workspace)
	if level != config.ExperienceExpert {
		t.Fatalf("expected expert experience level, got %s", level)
	}
}

func TestExperienceLevelForState_AllStates(t *testing.T) {
	cases := []struct {
		state UserJourneyState
		want  config.ExperienceLevel
	}{
		{StatePower, config.ExperienceExpert},
		{StateProductive, config.ExperienceAdvanced},
		{StateLearning, config.ExperienceIntermediate},
		{StateNew, config.ExperienceBeginner},
		{StateOnboarding, config.ExperienceBeginner},
		{UserJourneyState("archived"), config.ExperienceBeginner},
		{UserJourneyState(""), config.ExperienceBeginner},
	}
	for _, tc := range cases {
		if got := ExperienceLevelForState(tc.state); got != tc.want {
			t.Errorf("ExperienceLevelForState(%q) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestMigratePreferences_WhenCurrentVersion_ShouldNotMigrate(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"version": "2.0", "user_journey": {"state": "learning"}}`)
	result, err := MigratePreferences(ws)
	if err != nil {
		t.Fatalf("MigratePreferences: %v", err)
	}
	if result.WasMigrated {
		t.Fatalf("a current-version file should not migrate: %+v", result)
	}
	if result.FromVersion != PreferencesVersion || result.ToVersion != PreferencesVersion {
		t.Fatalf("version round-trip wrong: %+v", result)
	}
}

func TestMigratePreferences_WhenVersionMissing_ShouldMigrate(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"user_journey": {"state": "learning"}}`)
	result, err := MigratePreferences(ws)
	if err != nil {
		t.Fatalf("MigratePreferences: %v", err)
	}
	if !result.WasMigrated {
		t.Fatalf("a versionless file should migrate: %+v", result)
	}
	if got := GetUserJourneyState(ws); got != StateProductive {
		t.Fatalf("migrated journey state = %q, want productive", got)
	}
}

func TestMigratePreferences_WhenAgentSelectionHasNonStrings_ShouldSkipThem(t *testing.T) {
	ws := t.TempDir()
	oldPrefs := map[string]any{
		"version": "1.0",
		"agent_selection": map[string]any{
			"accepted_agents":         []any{"coder", 42, true},
			"rejected_agents":         []any{"nemesis", 7},
			"last_interactive":        "2026-01-01",
			"auto_accept_recommended": true,
		},
	}
	result, err := migrateFromOldVersion(ws, oldPrefs)
	if err != nil {
		t.Fatalf("migrateFromOldVersion: %v", err)
	}
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := pm.Get().AgentSelection
	if len(got.AcceptedAgents) != 1 || got.AcceptedAgents[0] != "coder" {
		t.Fatalf("non-string accepted agents should be skipped, got %v (result %+v)", got.AcceptedAgents, result)
	}
	if len(got.RejectedAgents) != 1 || got.RejectedAgents[0] != "nemesis" {
		t.Fatalf("non-string rejected agents should be skipped, got %v", got.RejectedAgents)
	}
}

func TestMigratePreferences_WhenMetricsMalformed_ShouldNotCarryMetrics(t *testing.T) {
	ws := t.TempDir()
	oldPrefs := map[string]any{
		"version": "1.0",
		"metrics": "not-an-object",
	}
	result, err := migrateFromOldVersion(ws, oldPrefs)
	if err != nil {
		t.Fatalf("migrateFromOldVersion: %v", err)
	}
	for _, key := range result.PreservedData {
		if key == "metrics" {
			t.Fatalf("malformed metrics should not be reported as preserved: %v", result.PreservedData)
		}
	}
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pm.Get().Metrics; got.SessionsCount != 0 || got.CommandsExecuted != 0 {
		t.Fatalf("malformed metrics should migrate as zero, got %+v", got)
	}
}

func TestShouldShowOnboarding_WhenIncomplete_ShouldShow(t *testing.T) {
	ws := t.TempDir()
	if err := NewPreferencesManager(ws).Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := pm.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// A saved-but-incomplete workspace is not on first run and has not
	// finished onboarding, so the wizard should still show.
	if !ShouldShowOnboarding(ws) {
		t.Fatalf("onboarding should show for an incomplete saved workspace")
	}
}

func TestShouldShowOnboarding_WhenFileIsCorrupt_ShouldHide(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"truncated": `)
	// A file that does not parse fails closed: no onboarding prompt.
	if ShouldShowOnboarding(ws) {
		t.Fatalf("onboarding should hide when preferences fail to load")
	}
}

func TestGetUserJourneyState_WhenFileIsCorrupt_ShouldReturnNew(t *testing.T) {
	ws := t.TempDir()
	writePrefsFile(t, ws, `{"truncated": `)
	if got := GetUserJourneyState(ws); got != StateNew {
		t.Fatalf("corrupt preferences journey state = %q, want new", got)
	}
}
