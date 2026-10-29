package store

import (
	"testing"
)

func TestStoreSessionTurn(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	sessionID := "sess-1"

	// Store turn 1
	err = store.StoreSessionTurn(sessionID, 1, "hello", "{}", "hi", "[]")
	if err != nil {
		t.Fatalf("StoreSessionTurn failed: %v", err)
	}

	// Store duplicate turn 1 with placeholder payloads.
	// The row should remain a single row and not lose non-placeholder data.
	err = store.StoreSessionTurn(sessionID, 1, "hello2", "{}", "hi2", "[]")
	if err != nil {
		t.Fatalf("StoreSessionTurn failed on duplicate: %v", err)
	}

	// Retrieve
	history, err := store.GetSessionHistory(sessionID, 10)
	if err != nil {
		t.Fatalf("GetSessionHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 history item, got %d", len(history))
	}
}

func TestStoreSessionTurn_UpgradesPlaceholderPayloads(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	sessionID := "sess-upgrade"

	err = store.StoreSessionTurn(sessionID, 1, "hello", "{}", "hi", "[]")
	if err != nil {
		t.Fatalf("initial StoreSessionTurn failed: %v", err)
	}

	err = store.StoreSessionTurn(sessionID, 1, "hello", `{"verb":"/fix"}`, "hi", `["fact(/x)."]`)
	if err != nil {
		t.Fatalf("upgrade StoreSessionTurn failed: %v", err)
	}

	// A later placeholder sync should not downgrade the richer payloads.
	err = store.StoreSessionTurn(sessionID, 1, "hello", "{}", "hi", "[]")
	if err != nil {
		t.Fatalf("placeholder StoreSessionTurn failed: %v", err)
	}

	history, err := store.GetSessionHistory(sessionID, 10)
	if err != nil {
		t.Fatalf("GetSessionHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 history item, got %d", len(history))
	}
	if history[0]["intent"] != `{"verb":"/fix"}` {
		t.Fatalf("Expected upgraded intent JSON, got %q", history[0]["intent"])
	}
	if history[0]["atoms"] != `["fact(/x)."]` {
		t.Fatalf("Expected upgraded atoms JSON, got %q", history[0]["atoms"])
	}
}

func TestLogActivation(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Log activation
	store.LogActivation("fact1", 0.9)
	store.LogActivation("fact2", 0.5)

	// Get recent
	activations, err := store.GetRecentActivations(10, 0.8)
	if err != nil {
		t.Fatalf("GetRecentActivations failed: %v", err)
	}

	if len(activations) != 1 {
		t.Errorf("Expected 1 high score activation, got %d", len(activations))
	}
	if _, ok := activations["fact1"]; !ok {
		t.Error("Missing fact1")
	}
}

func TestCompressedState(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	sessionID := "sess-state"

	// Store state
	err = store.StoreCompressedState(sessionID, 1, `{"k":"v"}`, 2.0)
	if err != nil {
		t.Fatalf("StoreCompressedState failed: %v", err)
	}

	// Load latest
	state, turn, ratio, err := store.LoadLatestCompressedState(sessionID)
	if err != nil {
		t.Fatalf("LoadLatestCompressedState failed: %v", err)
	}

	if state != `{"k":"v"}` {
		t.Errorf("Unexpected state: %s", state)
	}
	if turn != 1 {
		t.Errorf("Unexpected turn: %d", turn)
	}
	if ratio != 2.0 {
		t.Errorf("Unexpected ratio: %f", ratio)
	}

	// Update with newer turn
	store.StoreCompressedState(sessionID, 2, `{"k":"v2"}`, 3.0)
	state, turn, _, _ = store.LoadLatestCompressedState(sessionID)

	if turn != 2 {
		t.Errorf("Expected turn 2, got %d", turn)
	}
	if state != `{"k":"v2"}` {
		t.Errorf("Expected state v2, got %s", state)
	}
}
