package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/ux"
)

func uxWorkspace(t *testing.T, prefsJSON string) string {
	t.Helper()
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte(prefsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	return ws
}

func uxModel(t *testing.T, ws string) Model {
	t.Helper()
	m := NewTestModel(WithWorkspace(ws))
	m.preferencesMgr = ux.NewPreferencesManager(ws)
	if err := m.preferencesMgr.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return m
}

// The adaptive loop closes: a session opens and is counted, the turns it runs
// are counted, and closing it moves a user whose metrics earned it to the next
// journey state. Every half existed; nothing counted anything, so no user ever
// left the state their preferences file started in.
func TestUXJourney_WhenASessionEarnsIt_ShouldMoveTheUserOn(t *testing.T) {
	ws := uxWorkspace(t, `{"version":"2.0",
  "user_journey":{"state":"learning","onboarding_completed":true},
  "metrics":{"sessions_count":14,"successful_tasks":19,"clarifications_needed":1}}`)
	m := uxModel(t, ws)

	m = m.openSessionRecord(nil, nil)
	m.recordUXMetric("successful_tasks")
	m.closeSessionRecord()

	pm := ux.NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	prefs := pm.Get()
	if prefs.Metrics.SessionsCount != 15 || prefs.Metrics.SuccessfulTasks != 20 {
		t.Fatalf("session and task counts not persisted: %+v", prefs.Metrics)
	}
	if prefs.UserJourney.State != ux.StateProductive {
		t.Fatalf("journey state = %s, want productive after 15 sessions and 20 tasks", prefs.UserJourney.State)
	}
}

// Submitting input counts a command, and /help also counts a help request.
func TestUXJourney_WhenHelpIsSubmitted_ShouldCountACommandAndAHelpRequest(t *testing.T) {
	ws := uxWorkspace(t, `{"version":"2.0","user_journey":{"state":"learning","onboarding_completed":true}}`)
	m := uxModel(t, ws)

	m.textarea.SetValue("/help")
	next, _ := m.handleSubmit()
	got := next.(Model).preferencesMgr.Get().Metrics
	if got.CommandsExecuted != 1 || got.HelpRequests != 1 {
		t.Fatalf("metrics after /help = %+v, want one command and one help request", got)
	}
}

// A model whose session never opened -- a failed boot, a test model -- writes
// nothing when it shuts down.
func TestUXJourney_WhenSessionNeverOpened_ShouldRecordNothing(t *testing.T) {
	ws := t.TempDir()
	m := uxModel(t, ws)
	m.recordUXMetric("commands_executed")
	m.closeSessionRecord()
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "preferences.json")); !os.IsNotExist(err) {
		t.Fatalf("a session that never opened wrote preferences: %v", err)
	}
}

// Guidance "none" collapses progressive help to the compact full reference,
// however early in the journey the user is.
func TestHelpRenderer_WhenGuidanceIsNone_ShouldShowTheMinimalReference(t *testing.T) {
	ws := uxWorkspace(t, `{"version":"2.0","user_journey":{"state":"learning","onboarding_completed":true}}`)

	byJourney := NewHelpRenderer(ws).WithGuidance(config.GuidanceNormal).RenderHelp("")
	if strings.Contains(byJourney, "Full Command Reference") {
		t.Fatalf("a learning user with normal guidance got the minimal reference:\n%s", byJourney)
	}
	minimal := NewHelpRenderer(ws).WithGuidance(config.GuidanceNone).RenderHelp("")
	if !strings.Contains(minimal, "Full Command Reference") || !strings.Contains(minimal, "Help detail: minimal") {
		t.Fatalf("guidance none did not collapse help:\n%s", minimal)
	}
}
