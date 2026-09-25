package main

import (
	"strings"
	"testing"

	"codenerd/internal/regression"
)

// The battery policy's verdict reaches the operator. regression_battery.mg
// derives regression_battery_refused from a battery's projected commands, but
// nothing projected them, so the rules decided nothing for anyone.
// The batteries here are never run: only their policy verdict is computed.
func TestBatteryPolicyRefusals_ShouldNameWhatTheConstitutionRefuses(t *testing.T) {
	path := t.TempDir() + "/battery.yaml"

	clean := &regression.Battery{Tasks: []regression.Task{{ID: "build", Type: "shell", Command: "go build ./..."}}}
	reasons, err := batteryPolicyRefusals(path, clean)
	if err != nil {
		t.Fatalf("batteryPolicyRefusals(clean): %v", err)
	}
	if len(reasons) != 0 {
		t.Fatalf("a clean battery was refused: %v", reasons)
	}

	laundering := &regression.Battery{Tasks: []regression.Task{
		{ID: "build", Type: "shell", Command: "go build ./..."},
		{ID: "ship", Type: "shell", Command: "git push --force origin main"},
	}}
	reasons, err = batteryPolicyRefusals(path, laundering)
	if err != nil {
		t.Fatalf("batteryPolicyRefusals(laundering): %v", err)
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0], "blocked pattern") {
		t.Fatalf("a battery that force-pushes was not refused for its blocked pattern: %v", reasons)
	}
}
