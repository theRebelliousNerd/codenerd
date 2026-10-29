package core

import (
	"context"
	"strings"
	"testing"
)

// Shadow Mode is the safety pre-flight: it projects an action's effects into a
// cloned kernel and asks the constitutional rules what they imply. Three of its
// kernel writes were bare statements with the error dropped, and every one of
// them fails OPEN — a fact that never lands cannot trigger a violation, so the
// action is pronounced safe precisely because the evidence against it was lost.

func TestStartSimulation_FailsWhenShadowStateCannotBeAsserted(t *testing.T) {
	k := setupMockKernel(t)
	// Cap the EDB at its current size so the next assert is rejected.
	k.SetMaxFacts(k.FactCount())

	sm := NewShadowMode(k)
	sim, err := sm.StartSimulation(context.Background(), "unwritable shadow")
	if err == nil {
		t.Fatal("StartSimulation handed back a simulation whose shadow_state fact never landed")
	}
	if sim != nil {
		t.Errorf("no simulation should be returned alongside the error: %+v", sim)
	}
	if !strings.Contains(err.Error(), "shadow_state") {
		t.Errorf("error should name the fact that was refused, got: %v", err)
	}

	// The failed start must not leave an active simulation behind: the next
	// StartSimulation would otherwise be refused as "already active".
	if _, err := sm.StartSimulation(context.Background(), "second attempt"); err != nil &&
		strings.Contains(err.Error(), "already active") {
		t.Error("a failed StartSimulation left the shadow mode wedged")
	}
}

func TestSimulateAction_FailsWhenAProjectedEffectCannotBeAsserted(t *testing.T) {
	k := setupMockKernel(t)
	sm := NewShadowMode(k)

	sim, err := sm.StartSimulation(context.Background(), "effect projection")
	if err != nil {
		t.Fatalf("start simulation: %v", err)
	}

	// Cap the shadow kernel now that the simulation is running, so the effect
	// facts for the action below are refused.
	sm.shadowKernel.SetMaxFacts(sm.shadowKernel.FactCount())

	result, err := sm.SimulateAction(context.Background(), SimulatedAction{
		ID:     "act-1",
		Type:   ActionTypeFileDelete,
		Target: "internal/core/kernel.go",
	})
	if err == nil {
		t.Fatalf("SimulateAction returned a verdict for a projection it could not complete: %+v", result)
	}
	if result != nil && result.IsSafe {
		t.Error("an incomplete projection was reported safe")
	}
	if sim.IsSafe {
		t.Error("the simulation should no longer be considered safe")
	}
}
