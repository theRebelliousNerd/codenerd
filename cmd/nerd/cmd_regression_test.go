package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/regression"
)

// The leaf commands are exercised through their RunE directly. Calling
// regressionCmd.Execute() would run the ROOT command — the subcommand is
// registered on rootCmd — and fall through to the interactive chat.

func TestRegressionInitCmd_WhenWorkspaceHasNoBattery_ShouldSeedIt(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)

	if err := regressionInitCmd.RunE(regressionInitCmd, nil); err != nil {
		t.Fatalf("regression init: %v", err)
	}

	path := regression.DefaultBatteryPath(ws)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("battery not written to %s: %v", path, err)
	}
	if string(data) != regression.TemplateBattery {
		t.Fatalf("seeded battery does not match regression.TemplateBattery")
	}
	if _, err := regression.LoadBattery(path); err != nil {
		t.Fatalf("seeded battery does not load: %v", err)
	}
}

// TestRegressionInitCmd_WhenBatteryExists_ShouldRefuseRatherThanOverwrite:
// regression.Seed is a silent no-op so `nerd init` can call it unconditionally,
// but a command the operator typed by name must say why nothing happened.
func TestRegressionInitCmd_WhenBatteryExists_ShouldRefuseRatherThanOverwrite(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)

	path := regression.DefaultBatteryPath(ws)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mine := "version: 1\ntasks:\n  - id: mine\n    command: echo mine\n"
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := regressionInitCmd.RunE(regressionInitCmd, nil)
	if err == nil {
		t.Fatal("expected an error when a battery already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error should say the battery already exists, got: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != mine {
		t.Fatal("the existing battery was overwritten")
	}
}

func TestRegressionRunCmd_WhenNoBatteryExists_ShouldPointAtInit(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)
	regressionFile = ""
	t.Cleanup(func() { regressionFile = "" })

	err := regressionRunCmd.RunE(regressionRunCmd, nil)
	if err == nil {
		t.Fatal("expected an error when the workspace has no battery")
	}
	if !strings.Contains(err.Error(), "nerd regression init") {
		t.Fatalf("a missing battery is the common first-run case and should point at init, got: %v", err)
	}
}

func TestRegressionRunCmd_WhenATaskFails_ShouldReturnANonNilError(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)

	path := filepath.Join(ws, "battery.yaml")
	body := "version: 1\ntasks:\n  - id: fails\n    command: exit 3\n    timeout_sec: 30\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	regressionFile = path
	regressionNoSave = true
	t.Cleanup(func() { regressionFile = ""; regressionNoSave = false })

	err := regressionRunCmd.RunE(regressionRunCmd, nil)
	if err == nil {
		t.Fatal("a failing battery must exit non-zero so CI can gate on it without parsing output")
	}
	if !strings.Contains(err.Error(), "regression battery failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegressionListCmd_WhenNoRunsRecorded_ShouldSucceedQuietly(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)

	if err := regressionListCmd.RunE(regressionListCmd, nil); err != nil {
		t.Fatalf("list with no runs should not error: %v", err)
	}
}

func TestRegressionRunCmd_ShouldPersistARunRecordListReadsBack(t *testing.T) {
	ws := t.TempDir()
	withWorkspace(t, ws)

	path := filepath.Join(ws, "battery.yaml")
	body := "version: 1\ntasks:\n  - id: ok\n    command: echo ok\n    timeout_sec: 30\n    expect_contains: [\"ok\"]\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	regressionFile = path
	regressionNoSave = false
	t.Cleanup(func() { regressionFile = "" })

	if err := regressionRunCmd.RunE(regressionRunCmd, nil); err != nil {
		t.Fatalf("regression run: %v", err)
	}

	runs, err := regression.ListRuns(ws)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected exactly one persisted run, got %d", len(runs))
	}
	if err := regressionListCmd.RunE(regressionListCmd, nil); err != nil {
		t.Fatalf("regression list: %v", err)
	}
}
