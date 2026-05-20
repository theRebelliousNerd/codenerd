package core

import (
	"testing"
)

// --- FactCategory.String ---

func TestFactCategory_String_AllValues(t *testing.T) {
	tests := []struct {
		cat  FactCategory
		want string
	}{
		{FactCategoryPersistent, "persistent"},
		{FactCategoryEphemeral, "ephemeral"},
		{FactCategoryDerived, "derived"},
		{FactCategory(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.cat.String(); got != tt.want {
				t.Errorf("FactCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}

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

func TestIsDerived_WhenKnownDerived_ShouldReturnTrue(t *testing.T) {
	derived := []string{"permitted", "blocked", "safe_action", "context_atom"}
	for _, p := range derived {
		if !IsDerived(p) {
			t.Errorf("IsDerived(%q) = false, want true", p)
		}
	}
}

func TestIsDerived_WhenUnknown_ShouldReturnFalse(t *testing.T) {
	if IsDerived("my_custom_fact") {
		t.Error("expected false for unknown predicate")
	}
}

// --- IsPersistent ---

func TestIsPersistent_WhenNeitherEphemeralNorDerived_ShouldReturnTrue(t *testing.T) {
	if !IsPersistent("tool_registered") {
		t.Error("expected true for a persistent predicate")
	}
}

func TestIsPersistent_WhenEphemeral_ShouldReturnFalse(t *testing.T) {
	if IsPersistent("user_intent") {
		t.Error("expected false for ephemeral predicate")
	}
}

func TestIsPersistent_WhenDerived_ShouldReturnFalse(t *testing.T) {
	if IsPersistent("permitted") {
		t.Error("expected false for derived predicate")
	}
}

// --- GetCategory ---

func TestGetCategory_AllTypes(t *testing.T) {
	tests := []struct {
		predicate string
		want      FactCategory
	}{
		{"permitted", FactCategoryDerived},
		{"user_intent", FactCategoryEphemeral},
		{"tool_registered", FactCategoryPersistent},
	}
	for _, tt := range tests {
		t.Run(tt.predicate, func(t *testing.T) {
			got := GetCategory(tt.predicate)
			if got != tt.want {
				t.Errorf("GetCategory(%q) = %v, want %v", tt.predicate, got, tt.want)
			}
		})
	}
}

// --- FilterPersistent ---

func TestFilterPersistent_ShouldFilterCorrectly(t *testing.T) {
	input := []string{"user_intent", "tool_registered", "permitted", "file_topology"}
	got := FilterPersistent(input)
	if len(got) != 2 { // tool_registered and file_topology
		t.Errorf("FilterPersistent returned %d items, want 2: %v", len(got), got)
	}
}

func TestFilterPersistent_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	got := FilterPersistent(nil)
	if len(got) != 0 {
		t.Errorf("FilterPersistent(nil) returned %d items, want 0", len(got))
	}
}

// --- FilterEphemeral ---

func TestFilterEphemeral_ShouldFilterCorrectly(t *testing.T) {
	input := []string{"user_intent", "tool_registered", "pending_action"}
	got := FilterEphemeral(input)
	if len(got) != 2 { // user_intent and pending_action
		t.Errorf("FilterEphemeral returned %d items, want 2: %v", len(got), got)
	}
}

// --- ShouldLoadFromDisk / ShouldPersistToDisk ---

func TestShouldLoadFromDisk_WhenPersistent_ShouldReturnTrue(t *testing.T) {
	if !ShouldLoadFromDisk("tool_registered") {
		t.Error("expected true for persistent predicate")
	}
}

func TestShouldLoadFromDisk_WhenEphemeral_ShouldReturnFalse(t *testing.T) {
	if ShouldLoadFromDisk("user_intent") {
		t.Error("expected false for ephemeral predicate")
	}
}

func TestShouldPersistToDisk_WhenPersistent_ShouldReturnTrue(t *testing.T) {
	if !ShouldPersistToDisk("tool_registered") {
		t.Error("expected true for persistent predicate")
	}
}

func TestShouldPersistToDisk_WhenDerived_ShouldReturnFalse(t *testing.T) {
	if ShouldPersistToDisk("permitted") {
		t.Error("expected false for derived predicate")
	}
}
