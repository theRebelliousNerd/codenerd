package init

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// `.nerd/preferences.json` is user data written by three owners: internal/ux
// (journey, guidance, telemetry, metrics, learned patterns),
// SaveAgentPreferences (agent_selection) and init's own phase 8. Phase 8 used
// to marshal a freshly defaulted UserPreferences straight over the file, so
// `nerd init --force` — advertised as "preserves learned preferences" — reset
// onboarding, discarded recorded intent corrections and dropped the agent
// accept/reject history. These tests are the guard on that.

func writePreferences(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal preferences fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write preferences fixture: %v", err)
	}
}

func readPreferences(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preferences: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse preferences: %v", err)
	}
	return doc
}

func TestSavePreferences_WhenForceReinitOverExistingFile_ShouldPreserveForeignAndLearnedKeys(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".nerd", "preferences.json")

	writePreferences(t, path, map[string]any{
		"version": "2.0",
		"user_journey": map[string]any{
			"state":                "productive",
			"onboarding_completed": true,
		},
		"learned_patterns": map[string]any{
			"intent_corrections": []any{
				map[string]any{"original_parse": "fix", "user_correction": "refactor"},
			},
		},
		"agent_selection": map[string]any{
			"accepted_agents": []any{"GoExpert"},
			"rejected_agents": []any{"K8sExpert"},
		},
		// An init-owned key the user (or autopoiesis) already changed.
		"verbosity":     "detailed",
		"require_tests": true,
	})

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	effective, err := ini.savePreferences(path, ini.initPreferences())
	if err != nil {
		t.Fatalf("savePreferences: %v", err)
	}

	doc := readPreferences(t, path)

	if doc["version"] != "2.0" {
		t.Errorf("ux schema version lost: %v", doc["version"])
	}
	journey, ok := doc["user_journey"].(map[string]any)
	if !ok || journey["onboarding_completed"] != true {
		t.Errorf("user_journey block lost or reset: %v", doc["user_journey"])
	}
	learned, ok := doc["learned_patterns"].(map[string]any)
	if !ok {
		t.Fatalf("learned_patterns block lost: %v", doc["learned_patterns"])
	}
	if corrections, _ := learned["intent_corrections"].([]any); len(corrections) != 1 {
		t.Errorf("intent corrections lost: %v", learned["intent_corrections"])
	}
	selection, ok := doc["agent_selection"].(map[string]any)
	if !ok {
		t.Fatalf("agent_selection block lost: %v", doc["agent_selection"])
	}
	if accepted, _ := selection["accepted_agents"].([]any); len(accepted) != 1 {
		t.Errorf("agent accept history lost: %v", selection["accepted_agents"])
	}

	// Init-owned keys already on disk are learned values and must win.
	if doc["verbosity"] != "detailed" {
		t.Errorf("verbosity = %v, want the on-disk value \"detailed\"", doc["verbosity"])
	}
	if doc["require_tests"] != true {
		t.Errorf("require_tests = %v, want the on-disk value true", doc["require_tests"])
	}
	if effective.Verbosity != "detailed" || !effective.RequireTests {
		t.Errorf("returned preferences %+v do not match what is on disk", effective)
	}

	// Keys that were absent are still seeded with the defaults.
	if doc["explanation_level"] != "intermediate" {
		t.Errorf("explanation_level = %v, want the seeded default", doc["explanation_level"])
	}
}

func TestSavePreferences_WhenFileMissing_ShouldWriteDefaults(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".nerd", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	if _, err := ini.savePreferences(path, ini.initPreferences()); err != nil {
		t.Fatalf("savePreferences: %v", err)
	}

	doc := readPreferences(t, path)
	if doc["verbosity"] != "concise" {
		t.Errorf("verbosity = %v, want \"concise\"", doc["verbosity"])
	}
	if doc["explanation_level"] != "intermediate" {
		t.Errorf("explanation_level = %v, want \"intermediate\"", doc["explanation_level"])
	}
}

func TestSavePreferences_WhenExistingFileIsCorrupt_ShouldRefuseToOverwrite(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".nerd", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	corrupt := []byte(`{"version": "2.0", "user_journey": {`)
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatalf("write corrupt fixture: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}
	if _, err := ini.savePreferences(path, ini.initPreferences()); err == nil {
		t.Fatal("expected an error rather than clobbering a corrupt preferences file")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != string(corrupt) {
		t.Errorf("corrupt file was modified; the user can no longer recover it:\n%s", after)
	}
}

func TestSavePreferences_WhenHintGiven_ShouldOverrideTheStoredValue(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".nerd", "preferences.json")
	writePreferences(t, path, map[string]any{
		"explanation_level": "beginner",
		"verbosity":         "detailed",
	})

	// An explicit hint on this run is an instruction, unlike the merge-preserved
	// keys around it.
	ini := &Initializer{config: InitConfig{Workspace: workspace, PreferenceHints: []string{"expert"}}}
	if _, err := ini.savePreferences(path, ini.initPreferences()); err != nil {
		t.Fatalf("savePreferences: %v", err)
	}

	doc := readPreferences(t, path)
	if doc["explanation_level"] != "expert" {
		t.Errorf("explanation_level = %v, want the hinted \"expert\"", doc["explanation_level"])
	}
	if doc["verbosity"] != "detailed" {
		t.Errorf("verbosity = %v, want the unhinted on-disk value preserved", doc["verbosity"])
	}
}

// TestForceReinit_WhenRerunOverInitializedWorkspace_ShouldNotDeletePreferences is
// the end-to-end form of the P0 audit: run the real preference phase twice, the
// second time over a file that already carries foreign blocks, exactly as
// `nerd init --force` does.
func TestForceReinit_WhenRerunOverInitializedWorkspace_ShouldNotDeletePreferences(t *testing.T) {
	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ini := &Initializer{config: InitConfig{Workspace: workspace}}

	first := &InitResult{}
	ini.runPhase8Preferences(newPhaseRunner(ini), first, nerdDir)

	// Between runs the user onboards and curates agents.
	prefsPath := filepath.Join(nerdDir, "preferences.json")
	doc := readPreferences(t, prefsPath)
	doc["user_journey"] = map[string]any{"onboarding_completed": true}
	doc["agent_selection"] = map[string]any{"rejected_agents": []any{"K8sExpert"}}
	writePreferences(t, prefsPath, doc)

	second := &InitResult{}
	ini.runPhase8Preferences(newPhaseRunner(ini), second, nerdDir)
	if len(second.Failures) != 0 {
		t.Fatalf("force reinit reported failures: %v", second.Failures)
	}

	after := readPreferences(t, prefsPath)
	journey, ok := after["user_journey"].(map[string]any)
	if !ok || journey["onboarding_completed"] != true {
		t.Errorf("force reinit destroyed onboarding state: %v", after["user_journey"])
	}
	selection, ok := after["agent_selection"].(map[string]any)
	if !ok {
		t.Fatalf("force reinit destroyed agent_selection: %v", after["agent_selection"])
	}
	if rejected, _ := selection["rejected_agents"].([]any); len(rejected) != 1 {
		t.Errorf("force reinit destroyed the agent reject list: %v", selection["rejected_agents"])
	}
}
