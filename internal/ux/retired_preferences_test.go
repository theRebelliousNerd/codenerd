package ux

import (
	"path/filepath"
	"testing"
)

// preferences.json carried a learned_patterns block (intent_corrections,
// command_preferences) that was {} for the life of every workspace: its only
// writer had no caller and nothing read it. The block is retired: a save
// removes it from an existing file, and a fresh file never has it.
func TestPreferences_TheRetiredLearnedPatternsBlockIsNotPersisted(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := writePrefsFile(t, ws, `{
  "version": "2.0",
  "metrics": {"sessions_count": 36, "commands_executed": 48},
  "learned_patterns": {"intent_corrections": [], "command_preferences": {}},
  "verbosity": "detailed"
}`)
	pm := NewPreferencesManager(ws)
	if err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	if err := pm.RecordSessionStart(); err != nil {
		t.Fatal(err)
	}
	got := readPrefsMap(t, path)
	if _, ok := got["learned_patterns"]; ok {
		t.Errorf("the retired learned_patterns block survived a save: %v", got)
	}
	if got["verbosity"] != "detailed" {
		t.Errorf("a key another writer owns was lost: %v", got)
	}

	fresh := t.TempDir()
	pm = NewPreferencesManager(fresh)
	if err := pm.Load(); err != nil {
		t.Fatal(err)
	}
	if err := pm.Save(); err != nil {
		t.Fatal(err)
	}
	if _, ok := readPrefsMap(t, filepath.Join(fresh, ".nerd", "preferences.json"))["learned_patterns"]; ok {
		t.Error("a fresh preferences file was written with the retired learned_patterns block")
	}
}
