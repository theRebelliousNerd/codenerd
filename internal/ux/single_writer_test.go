package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writePrefsFile(t *testing.T, workspace, body string) string {
	t.Helper()
	dir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readPrefsMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("preferences.json does not parse after save: %v\n%s", err, data)
	}
	return out
}

// preferences.json has three writers. A UX metric save wrote this package's
// struct over the whole file, deleting what `nerd init` had put there; the
// keys UserPreferences does not declare now survive every save.
func TestSave_WhenOtherWritersOwnKeys_ShouldKeepThem(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := writePrefsFile(t, ws, `{
  "version": "2.0",
  "user_journey": {"state": "learning", "onboarding_completed": true},
  "test_style": "table_driven",
  "require_tests": true
}`)

	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := pm.RecordSessionStart(); err != nil {
		t.Fatalf("RecordSessionStart: %v", err)
	}

	got := readPrefsMap(t, path)
	if got["test_style"] != "table_driven" || got["require_tests"] != true {
		t.Fatalf("keys owned by nerd init were dropped by a UX save: %v", got)
	}
	metrics, _ := got["metrics"].(map[string]any)
	if metrics["sessions_count"] != float64(1) {
		t.Fatalf("the save did not record the session: %v", got)
	}
}

// A file that does not parse is moved aside, not silently discarded.
func TestSave_WhenExistingFileIsCorrupt_ShouldMoveItAside(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := writePrefsFile(t, ws, `{"truncated": `)

	pm := NewPreferencesManager(ws)
	pm.preferences = DefaultUserPreferences()
	if err := pm.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if data, err := os.ReadFile(path + ".corrupt"); err != nil || string(data) != `{"truncated": ` {
		t.Fatalf("the corrupt file was not kept at %s.corrupt: %q, %v", path, data, err)
	}
	if got := readPrefsMap(t, path); got["version"] != PreferencesVersion {
		t.Fatalf("fresh preferences were not written: %v", got)
	}
}

// A schema migration keeps the user's history: learned intent corrections and
// usage metrics used to be reset to zero with the version bump.
func TestMigratePreferences_WhenOldSchemaHasHistory_ShouldPreserveIt(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writePrefsFile(t, ws, `{
  "version": "1.0",
  "metrics": {"sessions_count": 42, "commands_executed": 310},
  "learned_patterns": {"intent_corrections": [
    {"original_parse": "fix", "user_correction": "refactor", "learned_at": "2026-01-01T00:00:00Z", "reinforcement_count": 3}
  ]}
}`)

	result, err := MigratePreferences(ws)
	if err != nil {
		t.Fatalf("MigratePreferences: %v", err)
	}
	if !result.WasMigrated {
		t.Fatal("a 1.0 file was not migrated")
	}

	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prefs := pm.Get()
	if prefs.Metrics.SessionsCount != 42 || prefs.Metrics.CommandsExecuted != 310 {
		t.Errorf("metrics were reset by the migration: %+v", prefs.Metrics)
	}
	if len(prefs.LearnedPatterns.IntentCorrections) != 1 || prefs.LearnedPatterns.IntentCorrections[0].ReinforcementCount != 3 {
		t.Errorf("learned corrections were dropped: %+v", prefs.LearnedPatterns)
	}
}
