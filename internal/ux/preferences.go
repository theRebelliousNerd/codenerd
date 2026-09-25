package ux

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
)

// PreferencesVersion is the current schema version for preferences.json.
const PreferencesVersion = "2.0"

// UserPreferences is the extended preferences schema for UX tracking.
type UserPreferences struct {
	// Version is the schema version for migration detection
	Version string `json:"version"`

	// UserJourney tracks the user's progression through codeNERD
	UserJourney JourneyPrefs `json:"user_journey"`

	// Guidance controls help and tips behavior
	Guidance GuidancePrefs `json:"guidance"`

	// Telemetry controls optional anonymous usage tracking
	Telemetry TelemetryPrefs `json:"telemetry"`

	// Metrics tracks local usage statistics
	Metrics UserMetrics `json:"metrics"`

	// LearnedPatterns stores corrections and preferences
	LearnedPatterns LearnedPatterns `json:"learned_patterns"`

	// AgentSelection tracks agent accept/reject history
	AgentSelection AgentSelectionPrefs `json:"agent_selection"`
}

// JourneyPrefs tracks user journey state.
type JourneyPrefs struct {
	State               UserJourneyState `json:"state"`
	TransitionTimestamp string           `json:"transition_timestamp,omitempty"`
	OnboardingCompleted bool             `json:"onboarding_completed"`
	OnboardingSkippedAt string           `json:"onboarding_skipped_at,omitempty"`
	CompletedSteps      []string         `json:"completed_steps,omitempty"`
}

// GuidancePrefs controls guidance behavior.
type GuidancePrefs struct {
	Level               config.GuidanceLevel `json:"level"`
	ShowHints           bool                 `json:"show_hints"`
	ShowWhyExplanations bool                 `json:"show_why_explanations"`
	AutoSuggestHelp     bool                 `json:"auto_suggest_help"`
}

// TelemetryPrefs controls optional telemetry.
type TelemetryPrefs struct {
	Enabled        bool `json:"enabled"`
	AnonymousUsage bool `json:"anonymous_usage"`
}

// LearnedPatterns stores user corrections and preferences.
type LearnedPatterns struct {
	IntentCorrections  []IntentCorrection      `json:"intent_corrections,omitempty"`
	CommandPreferences map[string]CommandPrefs `json:"command_preferences,omitempty"`
}

// IntentCorrection records when the user corrected a parse.
type IntentCorrection struct {
	OriginalParse      string `json:"original_parse"`
	UserCorrection     string `json:"user_correction"`
	LearnedAt          string `json:"learned_at"`
	ReinforcementCount int    `json:"reinforcement_count"`
}

// CommandPrefs stores per-command preferences.
type CommandPrefs struct {
	DefaultFlags  []string `json:"default_flags,omitempty"`
	DefaultTarget string   `json:"default_target,omitempty"`
}

// AgentSelectionPrefs tracks agent recommendations.
type AgentSelectionPrefs struct {
	AcceptedAgents        []string `json:"accepted_agents,omitempty"`
	RejectedAgents        []string `json:"rejected_agents,omitempty"`
	LastInteractive       string   `json:"last_interactive,omitempty"`
	AutoAcceptRecommended bool     `json:"auto_accept_recommended"`
}

// PreferencesManager handles loading/saving preferences.
type PreferencesManager struct {
	mu          sync.RWMutex
	path        string
	preferences *UserPreferences
}

// NewPreferencesManager creates a preferences manager for the given workspace.
func NewPreferencesManager(workspace string) *PreferencesManager {
	return &PreferencesManager{
		path: filepath.Join(workspace, ".nerd", "preferences.json"),
	}
}

// Load reads preferences from disk, creating defaults if not exists.
func (pm *PreferencesManager) Load() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := os.ReadFile(pm.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default preferences
			pm.preferences = DefaultUserPreferences()
			return nil
		}
		return fmt.Errorf("failed to read preferences: %w", err)
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return fmt.Errorf("failed to parse preferences: %w", err)
	}

	pm.preferences = &prefs
	return nil
}

// Save writes preferences to disk.
//
// preferences.json has three writers -- this package, `nerd init`
// (internal/init savePreferences) and the agent-selection prompt
// (SaveAgentPreferences) -- and each owns different keys. Save used to write
// this package's struct over the whole file, which deleted every key init had
// written the first time a UX metric changed, and it wrote in place, so a crash
// mid-write left a half file that init then refused to parse. It now
// read-modify-writes: the keys UserPreferences declares are replaced, every
// other key already in the file is kept, and the result lands atomically. An
// existing file that does not parse is moved aside to preferences.json.corrupt
// rather than silently discarded.
func (pm *PreferencesManager) Save() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	// Ensure directory exists
	dir := filepath.Dir(pm.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	merged := make(map[string]json.RawMessage)
	if existing, err := os.ReadFile(pm.path); err == nil && len(existing) > 0 {
		if jerr := json.Unmarshal(existing, &merged); jerr != nil {
			merged = make(map[string]json.RawMessage)
			if rerr := os.Rename(pm.path, pm.path+".corrupt"); rerr != nil {
				return fmt.Errorf("preferences.json does not parse (%v) and could not be moved aside: %w", jerr, rerr)
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read preferences before saving: %w", err)
	}

	owned, err := json.Marshal(pm.preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	var ownedKeys map[string]json.RawMessage
	if err := json.Unmarshal(owned, &ownedKeys); err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	for key, value := range ownedKeys {
		merged[key] = value
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	if err := atomicfile.WriteFile(pm.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write preferences: %w", err)
	}

	return nil
}

// Get returns a copy of the current preferences (thread-safe). The copy is
// shallow: callers read it; they change preferences through the setters, which
// hold the lock.
func (pm *PreferencesManager) Get() *UserPreferences {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.preferences == nil {
		return DefaultUserPreferences()
	}
	cp := *pm.preferences
	return &cp
}

// RecordSessionStart counts a session and stamps its start, then saves. The
// chat calls it once per session open, on the manager it loaded at boot, so
// one in-process writer owns the metrics.
func (pm *PreferencesManager) RecordSessionStart() error {
	if err := pm.IncrementMetric("sessions_count"); err != nil {
		return err
	}
	pm.mu.Lock()
	pm.preferences.Metrics.LastSession = time.Now().Format(time.RFC3339)
	pm.mu.Unlock()
	return pm.Save()
}

// RecordMetric increments one UserMetrics counter and saves.
func (pm *PreferencesManager) RecordMetric(metric string) error {
	if err := pm.IncrementMetric(metric); err != nil {
		return err
	}
	return pm.Save()
}

// CheckJourneyTransition moves the user to the next journey state when their
// metrics warrant it (UserMetrics.ShouldTransition), saving the change. It
// reports the state the user is in afterwards and whether it changed.
func (pm *PreferencesManager) CheckJourneyTransition() (UserJourneyState, bool, error) {
	prefs := pm.Get()
	currentState := prefs.UserJourney.State

	newState, shouldTransition := prefs.Metrics.ShouldTransition(currentState)
	if !shouldTransition {
		return currentState, false, nil
	}
	if err := pm.SetJourneyState(newState); err != nil {
		return currentState, false, err
	}
	if err := pm.Save(); err != nil {
		return currentState, false, err
	}
	return newState, true, nil
}

// GetJourneyState returns the current journey state.
func (pm *PreferencesManager) GetJourneyState() UserJourneyState {
	prefs := pm.Get()
	if prefs.UserJourney.State == "" {
		return StateNew
	}
	return prefs.UserJourney.State
}

// SetJourneyState updates the journey state.
func (pm *PreferencesManager) SetJourneyState(state UserJourneyState) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	pm.preferences.UserJourney.State = state
	pm.preferences.UserJourney.TransitionTimestamp = time.Now().Format(time.RFC3339)

	return nil
}

// IncrementMetric increments a numeric metric.
func (pm *PreferencesManager) IncrementMetric(metric string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	switch metric {
	case "sessions_count":
		pm.preferences.Metrics.SessionsCount++
	case "commands_executed":
		pm.preferences.Metrics.CommandsExecuted++
	case "clarifications_needed":
		pm.preferences.Metrics.ClarificationsNeeded++
	case "help_requests":
		pm.preferences.Metrics.HelpRequests++
	case "successful_tasks":
		pm.preferences.Metrics.SuccessfulTasks++
	case "errors_encountered":
		pm.preferences.Metrics.ErrorsEncountered++
	default:
		return fmt.Errorf("unknown metric: %s", metric)
	}

	return nil
}

// RecordCorrection stores a user's intent correction.
func (pm *PreferencesManager) RecordCorrection(original, corrected string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	// Check if this correction already exists
	for i, c := range pm.preferences.LearnedPatterns.IntentCorrections {
		if c.OriginalParse == original && c.UserCorrection == corrected {
			// Reinforce existing correction
			pm.preferences.LearnedPatterns.IntentCorrections[i].ReinforcementCount++
			return nil
		}
	}

	// Add new correction
	pm.preferences.LearnedPatterns.IntentCorrections = append(
		pm.preferences.LearnedPatterns.IntentCorrections,
		IntentCorrection{
			OriginalParse:      original,
			UserCorrection:     corrected,
			LearnedAt:          time.Now().Format(time.RFC3339),
			ReinforcementCount: 1,
		},
	)

	return nil
}

// GetGuidanceLevel returns the current guidance level.
func (pm *PreferencesManager) GetGuidanceLevel() config.GuidanceLevel {
	prefs := pm.Get()
	if prefs.Guidance.Level == "" {
		return config.GuidanceNormal
	}
	return prefs.Guidance.Level
}

// SetGuidanceLevel updates the guidance level.
func (pm *PreferencesManager) SetGuidanceLevel(level config.GuidanceLevel) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	pm.preferences.Guidance.Level = level
	return nil
}

// CompleteOnboardingStep marks an onboarding step as completed.
func (pm *PreferencesManager) CompleteOnboardingStep(step string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	// Check if already completed
	if slices.Contains(pm.preferences.UserJourney.CompletedSteps, step) {
		return nil
	}

	pm.preferences.UserJourney.CompletedSteps = append(
		pm.preferences.UserJourney.CompletedSteps,
		step,
	)

	return nil
}

// SkipOnboarding marks onboarding as skipped.
func (pm *PreferencesManager) SkipOnboarding() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	pm.preferences.UserJourney.OnboardingSkippedAt = time.Now().Format(time.RFC3339)
	pm.preferences.UserJourney.State = StateLearning
	pm.preferences.UserJourney.TransitionTimestamp = time.Now().Format(time.RFC3339)

	return nil
}

// MarkOnboardingComplete marks onboarding as completed.
func (pm *PreferencesManager) MarkOnboardingComplete() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.preferences == nil {
		pm.preferences = DefaultUserPreferences()
	}

	pm.preferences.UserJourney.OnboardingCompleted = true
	pm.preferences.UserJourney.State = StateLearning
	pm.preferences.UserJourney.TransitionTimestamp = time.Now().Format(time.RFC3339)

	return nil
}

// IsOnboardingComplete returns true if onboarding is done.
func (pm *PreferencesManager) IsOnboardingComplete() bool {
	prefs := pm.Get()
	return prefs.UserJourney.OnboardingCompleted || prefs.UserJourney.OnboardingSkippedAt != ""
}

// DefaultUserPreferences returns sensible defaults for new users.
func DefaultUserPreferences() *UserPreferences {
	return &UserPreferences{
		Version: PreferencesVersion,
		UserJourney: JourneyPrefs{
			State:               StateNew,
			TransitionTimestamp: time.Now().Format(time.RFC3339),
			OnboardingCompleted: false,
		},
		Guidance: GuidancePrefs{
			Level:               config.GuidanceNormal,
			ShowHints:           true,
			ShowWhyExplanations: true,
			AutoSuggestHelp:     true,
		},
		Telemetry: TelemetryPrefs{
			Enabled:        false,
			AnonymousUsage: false,
		},
		Metrics: UserMetrics{},
		LearnedPatterns: LearnedPatterns{
			CommandPreferences: make(map[string]CommandPrefs),
		},
		AgentSelection: AgentSelectionPrefs{
			AutoAcceptRecommended: true,
		},
	}
}
