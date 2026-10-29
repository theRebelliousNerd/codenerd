package ui

import (
	"testing"
)

func TestComputeHash(t *testing.T) {
	// Test Int
	h1 := ComputeKey(123)
	h2 := ComputeKey(123)
	if h1 != h2 {
		t.Errorf("expected same hash for same int, got %d != %d", h1, h2)
	}

	// Test Float
	f1 := ComputeKey(0.123)
	f2 := ComputeKey(0.123)
	if f1 != f2 {
		t.Errorf("expected same hash for same float, got %d != %d", f1, f2)
	}

	f3 := ComputeKey(0.124)
	if f1 == f3 {
		t.Errorf("expected different hash for different float, got %d == %d", f1, f3)
	}

	// Test Mixed
	m1 := ComputeKey("test", 123, 0.456, true)
	m2 := ComputeKey("test", 123, 0.456, true)
	if m1 != m2 {
		t.Errorf("expected same hash for same mixed inputs, got %d != %d", m1, m2)
	}

	// Test Float bits distinction
	// Ensure that float logic actually works (not ignoring it)
	// If ignored, ComputeKey(1.0) would equal ComputeKey(2.0) if only float was passed?
	// Wait, if ignored it returns hash of empty? No, it iterates.

	// Case where float was previously ignored:
	// ComputeKey("a", 1.0) vs ComputeKey("a", 2.0)
	// If float ignored, both hash "a".

	k1 := ComputeKey("prefix", 1.0)
	k2 := ComputeKey("prefix", 2.0)
	if k1 == k2 {
		t.Errorf("Hash collision for different floats! Float handling might be broken/ignored.")
	}
}
