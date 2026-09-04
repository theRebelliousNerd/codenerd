package session

import (
	"testing"
)

// TestInteractiveGateWarningOnce verifies that an executor whose VirtualStore
// lacks the InteractiveExecutiveGate interface logs the fallback warning
// exactly once across two turns. The flag gates the log, not the behavior:
// both turns proceed unsimulated (fail-open), but only the first emits the
// warning.
func TestInteractiveGateWarningOnce(t *testing.T) {
	mock := &MockVirtualStore{}
	// Premise: the stub store must NOT implement the gate, otherwise the
	// test would exercise the delegation path instead of the fallback.
	if _, ok := any(mock).(InteractiveExecutiveGate); ok {
		t.Fatal("MockVirtualStore implements InteractiveExecutiveGate; test premise violated")
	}

	e := &Executor{virtualStore: mock}

	// Turn 1: gate unavailable, warning fires.
	if _, ok := e.interactiveGate(); ok {
		t.Fatal("interactiveGate() = available, want unavailable for gate-less store")
	}
	if !e.gateUnavailableWarned.Load() {
		t.Fatal("gateUnavailableWarned = false after first turn, want true")
	}

	// Turn 2: gate still unavailable, but no second warning.
	if _, ok := e.interactiveGate(); ok {
		t.Fatal("interactiveGate() second call = available, want unavailable")
	}
	if e.warnInteractiveGateUnavailable() {
		t.Fatal("warnInteractiveGateUnavailable() second call returned true, want false (warning must fire exactly once)")
	}
}

// TestInteractiveGateNilStoreWarnsOnce covers the nil-store fallback: the
// executor must warn once rather than panic on the type assertion.
func TestInteractiveGateNilStoreWarnsOnce(t *testing.T) {
	e := &Executor{virtualStore: nil}

	if _, ok := e.interactiveGate(); ok {
		t.Fatal("interactiveGate() with nil store = available, want unavailable")
	}
	if !e.gateUnavailableWarned.Load() {
		t.Fatal("gateUnavailableWarned = false after nil-store turn, want true")
	}
	if e.warnInteractiveGateUnavailable() {
		t.Fatal("second nil-store warning fired; want exactly once per executor lifetime")
	}
}
