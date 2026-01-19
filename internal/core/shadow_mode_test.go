package core

import (
	"context"
	"testing"
)

func TestShadowMode_New(t *testing.T) {
	k := setupMockKernel(t)

	shadow := NewShadowMode(k)
	if shadow == nil {
		t.Fatal("NewShadowMode returned nil")
	}
}

func TestShadowMode_StartSimulation(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	ctx := context.Background()
	sim, err := shadow.StartSimulation(ctx, "Test what-if scenario")

	if err != nil {
		t.Fatalf("StartSimulation failed: %v", err)
	}

	if sim == nil {
		t.Fatal("Simulation is nil")
	}

	if sim.ID == "" {
		t.Error("Simulation missing ID")
	}

	if sim.Description != "Test what-if scenario" {
		t.Errorf("Expected description 'Test what-if scenario', got %q", sim.Description)
	}
}

func TestShadowMode_SimulateAction(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	ctx := context.Background()
	shadow.StartSimulation(ctx, "Test simulation")

	action := SimulatedAction{
		ID:          "sim-action-1",
		Type:        ActionTypeFileWrite,
		Target:      "test.go",
		Description: "Write test file",
	}

	result, err := shadow.SimulateAction(ctx, action)
	if err != nil {
		t.Fatalf("SimulateAction failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	t.Logf("Simulation result: safe=%v, effects=%d, violations=%d",
		result.IsSafe, len(result.Effects), len(result.Violations))
}

func TestShadowMode_IsShadowModeActive(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	if shadow.IsShadowModeActive() {
		t.Error("Expected inactive initially")
	}

	ctx := context.Background()
	shadow.StartSimulation(ctx, "Test")

	if !shadow.IsShadowModeActive() {
		t.Error("Expected active after StartSimulation")
	}
}

func TestShadowMode_AbortSimulation(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	ctx := context.Background()
	shadow.StartSimulation(ctx, "Test")

	shadow.AbortSimulation("User cancelled")

	if shadow.IsShadowModeActive() {
		t.Error("Expected inactive after abort")
	}
}

func TestShadowMode_CommitSimulation(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	ctx := context.Background()
	shadow.StartSimulation(ctx, "Test")

	err := shadow.CommitSimulation(ctx)
	if err != nil {
		t.Logf("CommitSimulation: %v (may require safe simulation)", err)
	}
}

func TestShadowMode_GetSimulation(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	ctx := context.Background()
	sim, _ := shadow.StartSimulation(ctx, "Test")

	found, ok := shadow.GetSimulation(sim.ID)
	if !ok {
		t.Fatal("Expected to find simulation")
	}

	if found.ID != sim.ID {
		t.Errorf("Expected ID %s, got %s", sim.ID, found.ID)
	}
}

func TestShadowMode_GetActiveSimulation(t *testing.T) {
	k := setupMockKernel(t)
	shadow := NewShadowMode(k)

	// No active sim initially
	_, ok := shadow.GetActiveSimulation()
	if ok {
		t.Error("Expected no active simulation initially")
	}

	ctx := context.Background()
	shadow.StartSimulation(ctx, "Test")

	active, ok := shadow.GetActiveSimulation()
	if !ok {
		t.Fatal("Expected active simulation")
	}
	if active == nil {
		t.Fatal("Active simulation is nil")
	}
}
