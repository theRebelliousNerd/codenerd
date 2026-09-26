package core

import (
	"testing"
)

// --- FactCategory.String ---

// --- IsEphemeral ---

func TestIsEphemeral_WhenKnownEphemeral_ShouldReturnTrue(t *testing.T) {
	ephemerals := []string{"user_intent", "pending_action", "next_action", "session_active"}
	for _, p := range ephemerals {
		if !IsEphemeral(p) {
			t.Errorf("IsEphemeral(%q) = false, want true", p)
		}
	}
}

func TestIsEphemeral_WhenUnknown_ShouldReturnFalse(t *testing.T) {
	if IsEphemeral("custom_persistent_fact") {
		t.Error("expected false for unknown predicate")
	}
}

// --- IsDerived ---

// --- IsPersistent ---

// --- GetCategory ---

// --- FilterPersistent ---

// --- FilterEphemeral ---

// --- ShouldLoadFromDisk / ShouldPersistToDisk ---
