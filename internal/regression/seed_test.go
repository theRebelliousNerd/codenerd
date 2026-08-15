package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeed_WhenWorkspaceHasNoBattery_ShouldWriteTheTemplate(t *testing.T) {
	workspace := t.TempDir()

	path, created, err := Seed(workspace)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if !created {
		t.Fatalf("Seed reported created=false for an empty workspace")
	}
	if want := DefaultBatteryPath(workspace); path != want {
		t.Fatalf("Seed wrote to %s, want %s", path, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded battery: %v", err)
	}
	if string(data) != TemplateBattery {
		t.Fatalf("seeded content does not match TemplateBattery")
	}
	if filepath.Base(filepath.Dir(path)) != "regression" {
		t.Fatalf("battery was not written under .nerd/regression/: %s", path)
	}
}

// TestSeed_WhenBatteryAlreadyExists_ShouldNotOverwrite is the property that
// makes Seed safe to call from every `nerd init`, including --force: the
// operator's suite is the thing the package exists to protect.
func TestSeed_WhenBatteryAlreadyExists_ShouldNotOverwrite(t *testing.T) {
	workspace := t.TempDir()
	path := DefaultBatteryPath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := "version: 1\ntasks:\n  - id: mine\n    command: echo mine\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, created, err := Seed(workspace)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if created {
		t.Fatalf("Seed reported it created a battery that already existed")
	}
	if got != path {
		t.Fatalf("Seed returned %s, want %s", got, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != existing {
		t.Fatalf("Seed overwrote an existing battery")
	}
}

// TestSeed_ShouldProduceALoadableBattery keeps the template honest. A seeded
// suite that fails Validate would send every first-run operator to a parse
// error instead of a result.
func TestSeed_ShouldProduceALoadableBattery(t *testing.T) {
	workspace := t.TempDir()
	path, _, err := Seed(workspace)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	battery, err := LoadBattery(path)
	if err != nil {
		t.Fatalf("seeded battery does not load: %v", err)
	}
	if len(battery.Tasks) == 0 {
		t.Fatalf("seeded battery has no tasks")
	}
	if battery.Version != SupportedVersion {
		t.Fatalf("seeded battery version = %d, want %d", battery.Version, SupportedVersion)
	}
	for _, task := range battery.Tasks {
		if task.ExpectExit == nil && len(task.ExpectContains) == 0 && len(task.ExpectNotContains) == 0 {
			t.Fatalf("seeded task %q declares no expectation; exit code alone is a weak assertion", task.ID)
		}
	}
}

func TestRequiredShell_ShouldNameThePlatformInterpreter(t *testing.T) {
	shell := RequiredShell()
	if shell != "bash" && shell != "powershell" {
		t.Fatalf("RequiredShell() = %q, want bash or powershell", shell)
	}
}

// TestRunBattery_WhenTheShellIsMissing_ShouldFailTheRunNotEveryTask pins the
// preflight. Without it a missing interpreter surfaced as N identical
// "executable file not found" task errors with the actual cause nowhere stated.
func TestRunBattery_WhenTheShellIsMissing_ShouldFailTheRunNotEveryTask(t *testing.T) {
	// exec.LookPath reads PATH from the current process environment, so an
	// empty PATH makes the interpreter unresolvable without touching the disk.
	t.Setenv("PATH", "")

	if err := CheckShell(); err == nil {
		t.Skip("shell still resolvable with an empty PATH on this platform")
	}

	battery := &Battery{
		Version: 1,
		Tasks: []Task{
			{ID: "one", Type: "shell", Command: "echo one"},
			{ID: "two", Type: "shell", Command: "echo two"},
		},
	}

	summary, err := RunBatteryWithOptions(t.Context(), battery, RunOptions{})
	if err == nil {
		t.Fatalf("expected a run-level error when %s is unavailable", RequiredShell())
	}
	if !strings.Contains(err.Error(), RequiredShell()) {
		t.Fatalf("error does not name the missing interpreter: %v", err)
	}
	if len(summary.Results) != 0 {
		t.Fatalf("expected no task results when the shell is missing, got %d", len(summary.Results))
	}
}
