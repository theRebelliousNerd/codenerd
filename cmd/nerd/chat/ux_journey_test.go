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
	t.Parallel()
	ws := uxWorkspace(t, `{"version":"2.0",
  "user_journey":{"state":"learning","onboarding_completed":true},
  "metrics":{"sessions_count":14,"successful_tasks":19,"clarifications_needed":1}}`)
	m := uxModel(t, ws)

	m = m.openSessionRecord(openPreferences(ws, m.preferencesMgr, false))
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

// The first-run check opens the session's preferences in its command -- the
// migration, the reload, the session counted -- and its handler on the Update
// thread touches no file.
func TestOnboardingCheck_OpensPreferencesOffTheUpdateThread(t *testing.T) {
	t.Parallel()
	const old = `{"version":"1.0","user_journey":{"state":"learning","onboarding_completed":true},"metrics":{"sessions_count":3}}`
	ws := uxWorkspace(t, old)
	m := uxModel(t, ws)
	prefsPath := filepath.Join(ws, ".nerd", "preferences.json")

	msg, ok := checkFirstRun(ws, m.preferencesMgr)().(onboardingCheckMsg)
	if !ok || msg.IsFirstRun {
		t.Fatalf("check = %+v, want an existing user", msg)
	}
	if msg.opening.migration == nil || !msg.opening.migration.WasMigrated {
		t.Fatalf("the command did not migrate the preferences: %+v", msg.opening)
	}
	if got := m.preferencesMgr.Get().Metrics.SessionsCount; got != 4 {
		t.Fatalf("sessions counted in the command = %d, want 4", got)
	}
	// Whatever the file holds now, the handler leaves it be.
	if err := os.WriteFile(prefsPath, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(msg)
	if data, _ := os.ReadFile(prefsPath); string(data) != old {
		t.Fatalf("the handler wrote the preferences on the Update thread:\n%s", data)
	}
	if next.(Model).sessionOpenedAt.IsZero() {
		t.Fatal("the session record was not opened")
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
