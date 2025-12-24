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
	if err := RecordSessionStart(workspace); err != nil {
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

	newState, transitioned, err := CheckJourneyTransition(workspace)
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
