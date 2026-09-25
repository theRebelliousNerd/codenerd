package ux

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/features"
)

// MigrationResult contains information about a preferences migration.
type MigrationResult struct {
	WasMigrated     bool
	FromVersion     string
	ToVersion       string
	PreservedData   []string // List of data that was preserved
	DefaultsApplied []string // List of defaults that were applied
}

// MigratePreferences checks and migrates preferences to the latest schema.
// For existing users without a preferences.json, creates one with "productive" state
// to skip onboarding (they're clearly not new users).
func MigratePreferences(workspace string) (*MigrationResult, error) {
	result := &MigrationResult{
		ToVersion: PreferencesVersion,
	}

	prefsPath := filepath.Join(workspace, ".nerd", "preferences.json")
	nerdDir := filepath.Join(workspace, ".nerd")

	// Check if .nerd directory exists (indicates existing user)
	nerdDirExists := false
	if _, err := os.Stat(nerdDir); err == nil {
		nerdDirExists = true
	}

	// Check if preferences.json exists
	data, err := os.ReadFile(prefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No preferences file
			if nerdDirExists {
				// Existing user without preferences - create with productive state
				return createExistingUserPreferences(workspace)
			}
			// New installation - create default preferences
			return createNewUserPreferences(workspace)
		}
		return nil, fmt.Errorf("failed to read preferences: %w", err)
	}

	// Parse existing preferences
	var rawPrefs map[string]any
	if err := json.Unmarshal(data, &rawPrefs); err != nil {
		// Invalid JSON - recreate with existing user settings
		return createExistingUserPreferences(workspace)
	}

	// Check version
	version, _ := rawPrefs["version"].(string)
	result.FromVersion = version

	if version == PreferencesVersion {
		// Already current version
		result.WasMigrated = false
		return result, nil
	}

	// Migrate from old version
	return migrateFromOldVersion(workspace, rawPrefs)
}

// createNewUserPreferences creates default preferences for a new user.
func createNewUserPreferences(workspace string) (*MigrationResult, error) {
	prefs := DefaultUserPreferences()

	pm := NewPreferencesManager(workspace)
	pm.preferences = prefs

	if err := pm.Save(); err != nil {
		return nil, fmt.Errorf("failed to save new preferences: %w", err)
	}

	return &MigrationResult{
		WasMigrated:     true,
		ToVersion:       PreferencesVersion,
		DefaultsApplied: []string{"new_user_defaults"},
	}, nil
}

// createExistingUserPreferences creates preferences for an existing user.
// They start as "productive" and skip onboarding.
func createExistingUserPreferences(workspace string) (*MigrationResult, error) {
	prefs := DefaultUserPreferences()

	// Existing users start as productive and skip onboarding
	prefs.UserJourney.State = StateProductive
	prefs.UserJourney.OnboardingCompleted = true
	prefs.UserJourney.TransitionTimestamp = time.Now().Format(time.RFC3339)
	prefs.UserJourney.CompletedSteps = []string{
		"config_complete",
		"first_command",
		"first_shard",
		"existing_user_migration",
	}

	// Set reasonable metrics for existing user
	prefs.Metrics.SessionsCount = 20 // Assume they've used it
	prefs.Metrics.CommandsExecuted = 50

	pm := NewPreferencesManager(workspace)
	pm.preferences = prefs

	if err := pm.Save(); err != nil {
		return nil, fmt.Errorf("failed to save migrated preferences: %w", err)
	}

	return &MigrationResult{
		WasMigrated:     true,
		FromVersion:     "",
		ToVersion:       PreferencesVersion,
		DefaultsApplied: []string{"existing_user_productive_state"},
	}, nil
}

// migrateFromOldVersion migrates from an older preferences schema.
func migrateFromOldVersion(workspace string, oldPrefs map[string]any) (*MigrationResult, error) {
	result := &MigrationResult{
		WasMigrated: true,
		ToVersion:   PreferencesVersion,
	}

	prefs := DefaultUserPreferences()

	// Preserve agent_selection if it exists
	if agentSel, ok := oldPrefs["agent_selection"].(map[string]any); ok {
		if accepted, ok := agentSel["accepted_agents"].([]any); ok {
			for _, a := range accepted {
				if s, ok := a.(string); ok {
					prefs.AgentSelection.AcceptedAgents = append(prefs.AgentSelection.AcceptedAgents, s)
				}
			}
			result.PreservedData = append(result.PreservedData, "accepted_agents")
		}
		if rejected, ok := agentSel["rejected_agents"].([]any); ok {
			for _, r := range rejected {
				if s, ok := r.(string); ok {
					prefs.AgentSelection.RejectedAgents = append(prefs.AgentSelection.RejectedAgents, s)
				}
			}
			result.PreservedData = append(result.PreservedData, "rejected_agents")
		}
		if lastInteractive, ok := agentSel["last_interactive"].(string); ok {
			prefs.AgentSelection.LastInteractive = lastInteractive
			result.PreservedData = append(result.PreservedData, "last_interactive")
		}
		if autoAccept, ok := agentSel["auto_accept_recommended"].(bool); ok {
			prefs.AgentSelection.AutoAcceptRecommended = autoAccept
			result.PreservedData = append(result.PreservedData, "auto_accept_recommended")
		}
	}

	// Learned patterns and metrics are the user's history; a schema bump must
	// not reset them to zero. Only well-formed sections are carried over.
	if raw, ok := oldPrefs["learned_patterns"]; ok {
		if data, err := json.Marshal(raw); err == nil {
			var lp LearnedPatterns
			if json.Unmarshal(data, &lp) == nil {
				if lp.CommandPreferences == nil {
					lp.CommandPreferences = make(map[string]CommandPrefs)
				}
				prefs.LearnedPatterns = lp
				result.PreservedData = append(result.PreservedData, "learned_patterns")
			}
		}
	}
	if raw, ok := oldPrefs["metrics"]; ok {
		if data, err := json.Marshal(raw); err == nil {
			var m UserMetrics
			if json.Unmarshal(data, &m) == nil {
				prefs.Metrics = m
				result.PreservedData = append(result.PreservedData, "metrics")
			}
		}
	}

	// Existing users with old prefs are definitely not new
	prefs.UserJourney.State = StateProductive
	prefs.UserJourney.OnboardingCompleted = true
	prefs.UserJourney.TransitionTimestamp = time.Now().Format(time.RFC3339)
	result.DefaultsApplied = append(result.DefaultsApplied, "productive_state")

	pm := NewPreferencesManager(workspace)
	pm.preferences = prefs

	if err := pm.Save(); err != nil {
		return nil, fmt.Errorf("failed to save migrated preferences: %w", err)
	}

	return result, nil
}

// IsFirstRun checks if this is a first-time run (no .nerd directory).
func IsFirstRun(workspace string) bool {
	nerdDir := filepath.Join(workspace, ".nerd")
	_, err := os.Stat(nerdDir)
	return os.IsNotExist(err)
}

// ShouldShowOnboarding determines if onboarding should be shown.
func ShouldShowOnboarding(workspace string) bool {
	// Check feature flag override (env var > .nerd/config.json > default).
	if features.IsOnboardingSkipped() {
		return false
	}

	// Check if first run
	if !IsFirstRun(workspace) {
		// Not first run - check preferences
		pm := NewPreferencesManager(workspace)
		if err := pm.Load(); err != nil {
			return false // Error loading, skip onboarding
		}
		return !pm.IsOnboardingComplete()
	}

	// First run - show onboarding
	return true
}

// GetUserJourneyState returns the current journey state for a workspace.
func GetUserJourneyState(workspace string) UserJourneyState {
	pm := NewPreferencesManager(workspace)
	if err := pm.Load(); err != nil {
		return StateNew
	}
	return pm.GetJourneyState()
}

// GetExperienceLevelFromPreferences returns the experience level the
// workspace's journey state implies.
func GetExperienceLevelFromPreferences(workspace string) config.ExperienceLevel {
	return ExperienceLevelForState(GetUserJourneyState(workspace))
}

// ExperienceLevelForState maps a journey state to an experience level. The
// help renderer and the tip generator each carried a copy of this switch.
func ExperienceLevelForState(state UserJourneyState) config.ExperienceLevel {
	switch state {
	case StatePower:
		return config.ExperienceExpert
	case StateProductive:
		return config.ExperienceAdvanced
	case StateLearning:
		return config.ExperienceIntermediate
	default:
		return config.ExperienceBeginner
	}
}
