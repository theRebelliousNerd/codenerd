package store

import (
	"testing"
)

func TestLocalStore_Session_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Activation Log
	err = s.LogActivation("fact-1", 0.95)
	if err != nil {
		t.Errorf("LogActivation failed: %v", err)
	}

	acts, err := s.GetRecentActivations(10, 0.9)
	if err != nil {
		t.Errorf("GetRecentActivations failed: %v", err)
	}
	if acts["fact-1"] != 0.95 {
		t.Errorf("Expected activation score 0.95, got %v", acts["fact-1"])
	}

	acts2, _ := s.GetRecentActivations(0, 0.9) // default limit
	if len(acts2) != 1 {
		t.Errorf("Expected 1 activation with default limit")
	}

	// 2. Session Turn
	err = s.StoreSessionTurn("session-1", 1, "hello", `{"intent":"test"}`, "hi", `["atom"]`)
	if err != nil {
		t.Errorf("StoreSessionTurn failed: %v", err)
	}
	
	// Update session turn (conflict resolution)
	err = s.StoreSessionTurn("session-1", 1, "hello2", "", "", "")
	if err != nil {
		t.Errorf("StoreSessionTurn update failed: %v", err)
	}

	history, err := s.GetSessionHistory("session-1", 10)
	if err != nil {
		t.Errorf("GetSessionHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 turn, got %d", len(history))
	}
	
	history2, _ := s.GetSessionHistory("session-1", 0) // default limit
	if len(history2) != 1 {
		t.Errorf("Expected 1 turn with default limit")
	}

	// 3. Compressed State
	err = s.StoreCompressedState("session-1", 1, `{"state":"test"}`, 0.5)
	if err != nil {
		t.Errorf("StoreCompressedState failed: %v", err)
	}
	
	// empty sessionID / stateJSON
	err = s.StoreCompressedState("", 1, "test", 0.5)
	if err != nil {
		t.Errorf("StoreCompressedState empty session failed: %v", err)
	}
	
	stateJSON, turn, ratio, err := s.LoadLatestCompressedState("session-1")
	if err != nil {
		t.Errorf("LoadLatestCompressedState failed: %v", err)
	}
	if stateJSON != `{"state":"test"}` || turn != 1 || ratio != 0.5 {
		t.Errorf("Loaded compressed state unexpected")
	}
	
	// empty sessionID
	sJSON, _, _, _ := s.LoadLatestCompressedState("")
	if sJSON != "" {
		t.Errorf("Expected empty string for empty sessionID")
	}
	
	// no rows
	sJSON, _, _, _ = s.LoadLatestCompressedState("non-existent")
	if sJSON != "" {
		t.Errorf("Expected empty string for non-existent sessionID")
	}
}
