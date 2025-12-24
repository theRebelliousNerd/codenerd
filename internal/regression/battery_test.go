package regression

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadBattery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "battery.yaml")
	content := `version: 1
tasks:
  - id: smoke
    type: shell
    command: echo ok
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write battery: %v", err)
	}

	b, err := LoadBattery(path)
	if err != nil {
		t.Fatalf("LoadBattery failed: %v", err)
	}
	if b.Version != 1 {
		t.Fatalf("Version = %d, want 1", b.Version)
	}
	if len(b.Tasks) != 1 || b.Tasks[0].ID != "smoke" {
		t.Fatalf("unexpected tasks: %+v", b.Tasks)
	}
}

func TestRunBatterySuccess(t *testing.T) {
	b := &Battery{
		Version: 1,
		Tasks: []Task{
			{ID: "smoke", Type: "shell", Command: "echo ok", TimeoutSec: 5},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := RunBattery(ctx, b, "")
	if err != nil {
		t.Fatalf("RunBattery failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected success, got error: %s", results[0].Error)
	}
	if !strings.Contains(results[0].Output, "ok") {
		t.Fatalf("expected output to contain ok, got %q", results[0].Output)
	}
}

func TestRunBatteryUnsupportedTask(t *testing.T) {
	b := &Battery{
		Version: 1,
		Tasks: []Task{
			{ID: "bad", Type: "unknown", Command: "echo ok"},
			{ID: "after", Type: "shell", Command: "echo skip"},
		},
	}

	results, err := RunBattery(context.Background(), b, "")
	if err != nil {
		t.Fatalf("RunBattery failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected fail-fast after first task, got %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("expected unsupported task to fail")
	}
	if !strings.Contains(results[0].Error, "unsupported task type") {
		t.Fatalf("unexpected error: %s", results[0].Error)
	}
}

func TestRunBatteryEmpty(t *testing.T) {
	results, err := RunBattery(context.Background(), &Battery{}, "")
	if err != nil {
		t.Fatalf("RunBattery returned error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}

func TestDefaultBatteryPath(t *testing.T) {
	path := DefaultBatteryPath("C:\\workspace")
	if !strings.Contains(path, ".nerd") || !strings.Contains(path, "battery.yaml") {
		t.Fatalf("unexpected battery path: %s", path)
	}
}
