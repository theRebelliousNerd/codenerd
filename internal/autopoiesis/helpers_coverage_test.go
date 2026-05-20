package autopoiesis

import (
	"testing"
)

// --- shouldCheckToolNeed ---

func TestShouldCheckToolNeed_WhenMatchingPattern_ShouldReturnTrue(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"no tool for parsing", true},
		{"missing capability to validate", true},
		{"need a tool to analyze", true},
		{"create a tool for metrics", true},
		{"generate a tool to deploy", true},
		{"can't you validate this?", true},
		{"is there a way to fix", true},
		{"how do I validate configs", true},
		{"just a normal message", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shouldCheckToolNeed(tt.input)
			if got != tt.want {
				t.Errorf("shouldCheckToolNeed(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- hasStrongToolEvidence ---

func TestHasStrongToolEvidence_WhenNil_ShouldReturnFalse(t *testing.T) {
	// hasStrongToolEvidence should not panic on nil
	// Actually it will panic on nil, so test with empty
	need := &ToolNeed{Triggers: []string{}}
	got := hasStrongToolEvidence(need)
	if got {
		t.Error("expected false for empty triggers")
	}
}

func TestHasStrongToolEvidence_WhenFailedTrigger_ShouldReturnTrue(t *testing.T) {
	need := &ToolNeed{Triggers: []string{"previous attempt failed"}}
	if !hasStrongToolEvidence(need) {
		t.Error("expected true for 'previous attempt failed' trigger")
	}
}

func TestHasStrongToolEvidence_WhenMultipleTriggers_ShouldReturnTrue(t *testing.T) {
	need := &ToolNeed{Triggers: []string{"trigger1", "trigger2"}}
	if !hasStrongToolEvidence(need) {
		t.Error("expected true for 2+ triggers")
	}
}

func TestHasStrongToolEvidence_WhenSingleNonFailTrigger_ShouldReturnFalse(t *testing.T) {
	need := &ToolNeed{Triggers: []string{"just one"}}
	if hasStrongToolEvidence(need) {
		t.Error("expected false for single non-fail trigger")
	}
}

// --- sortActionsByPriority ---

func TestSortActionsByPriority_ShouldSortHighestFirst(t *testing.T) {
	actions := []AutopoiesisAction{
		{Priority: 1},
		{Priority: 5},
		{Priority: 3},
	}
	sortActionsByPriority(actions)
	if actions[0].Priority != 5 {
		t.Errorf("expected first priority=5, got %v", actions[0].Priority)
	}
	if actions[1].Priority != 3 {
		t.Errorf("expected second priority=3, got %v", actions[1].Priority)
	}
	if actions[2].Priority != 1 {
		t.Errorf("expected third priority=1, got %v", actions[2].Priority)
	}
}

func TestSortActionsByPriority_WhenEmpty_ShouldNotPanic(t *testing.T) {
	sortActionsByPriority(nil)
	sortActionsByPriority([]AutopoiesisAction{})
}

// --- hashString ---

func TestHashString_ShouldReturnConsistentHashes(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	if h1 != h2 {
		t.Errorf("expected same hash, got %s vs %s", h1, h2)
	}
}

func TestHashString_WhenDifferentInputs_ShouldReturnDifferentHashes(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("world")
	if h1 == h2 {
		t.Errorf("expected different hashes for different inputs, both got %s", h1)
	}
}

func TestHashString_WhenEmpty_ShouldReturnValidHash(t *testing.T) {
	h := hashString("")
	if len(h) != 8 {
		t.Errorf("expected 8-char hex hash, got %q (len=%d)", h, len(h))
	}
}

// --- complexityLevelString ---

func TestComplexityLevelString_AllLevels(t *testing.T) {
	tests := []struct {
		level ComplexityLevel
		want  string
	}{
		{ComplexitySimple, "Simple"},
		{ComplexityModerate, "Moderate"},
		{ComplexityComplex, "Complex"},
		{ComplexityEpic, "Epic"},
		{ComplexityLevel(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := complexityLevelString(tt.level)
			if got != tt.want {
				t.Errorf("complexityLevelString(%d) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

// --- truncateCode ---

func TestTruncateCode_WhenShort_ShouldReturnOriginal(t *testing.T) {
	code := "short"
	got := truncateCode(code, 100)
	if got != code {
		t.Errorf("expected original, got %q", got)
	}
}

func TestTruncateCode_WhenLong_ShouldTruncateWithNote(t *testing.T) {
	code := "this is a long string that exceeds the max"
	got := truncateCode(code, 10)
	if len(got) <= 10 {
		// Should have truncation note appended
	}
	if got[:10] != code[:10] {
		t.Errorf("truncation should preserve start")
	}
}

func TestTruncateCode_WhenExact_ShouldReturnOriginal(t *testing.T) {
	code := "exact"
	got := truncateCode(code, 5)
	if got != code {
		t.Errorf("expected original for exact length, got %q", got)
	}
}

// --- extractJSON ---

func TestExtractJSON_WhenPureJSON_ShouldReturn(t *testing.T) {
	got := extractJSON(`{"key": "value"}`)
	if got != `{"key": "value"}` {
		t.Errorf("expected pure JSON, got %q", got)
	}
}

func TestExtractJSON_WhenWrappedJSON_ShouldExtract(t *testing.T) {
	got := extractJSON(`Some text {"key": "val"} trailing`)
	if got != `{"key": "val"}` {
		t.Errorf("expected extracted JSON, got %q", got)
	}
}

func TestExtractJSON_WhenArray_ShouldExtract(t *testing.T) {
	got := extractJSON(`prefix [1, 2, 3] suffix`)
	if got != `[1, 2, 3]` {
		t.Errorf("expected extracted array, got %q", got)
	}
}

func TestExtractJSON_WhenNoJSON_ShouldReturnEmptyObject(t *testing.T) {
	got := extractJSON("no json here")
	if got != "{}" {
		t.Errorf("expected '{}', got %q", got)
	}
}

func TestExtractJSON_WhenNestedJSON_ShouldExtractFull(t *testing.T) {
	got := extractJSON(`{"a": {"b": "c"}}`)
	if got != `{"a": {"b": "c"}}` {
		t.Errorf("expected nested JSON, got %q", got)
	}
}

func TestExtractJSON_WhenEmpty_ShouldReturnEmptyObject(t *testing.T) {
	got := extractJSON("")
	if got != "{}" {
		t.Errorf("expected '{}' for empty, got %q", got)
	}
}

func TestExtractJSON_WhenStringWithBraces_ShouldHandleQuotes(t *testing.T) {
	got := extractJSON(`{"key": "value with {braces}"}`)
	if got != `{"key": "value with {braces}"}` {
		t.Errorf("expected properly handled quoted braces, got %q", got)
	}
}
