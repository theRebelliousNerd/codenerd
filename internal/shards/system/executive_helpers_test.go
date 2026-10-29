package system

import "testing"

func TestParseIntentTimestamp(t *testing.T) {
	ts, ok := parseIntentTimestamp("/intent_1700000000")
	if !ok || ts != 1700000000 {
		t.Errorf("parseIntentTimestamp valid: got (%d,%v), want (1700000000,true)", ts, ok)
	}
	for _, bad := range []string{"intent_123", "/intent_", "/intent_abc", "", "/other_123"} {
		if _, ok := parseIntentTimestamp(bad); ok {
			t.Errorf("parseIntentTimestamp(%q) should be invalid", bad)
		}
	}
}

func TestExecutiveStrategiesAndMetrics(t *testing.T) {
	e := NewExecutivePolicyShard()
	e.activeStrategies = []Strategy{{Name: "alpha"}, {Name: "beta"}}

	got := e.GetActiveStrategies()
	if len(got) != 2 {
		t.Fatalf("GetActiveStrategies len=%d, want 2", len(got))
	}
	// Returned slice is a copy: mutating it must not affect internal state.
	got[0].Name = "mutated"
	if e.activeStrategies[0].Name != "alpha" {
		t.Error("GetActiveStrategies must return a defensive copy")
	}

	// strategiesEqual is order-independent and name-based.
	if !e.strategiesEqual([]Strategy{{Name: "beta"}, {Name: "alpha"}}) {
		t.Error("same strategy names in different order should be equal")
	}
	if e.strategiesEqual([]Strategy{{Name: "alpha"}}) {
		t.Error("different counts should not be equal")
	}
	if e.strategiesEqual([]Strategy{{Name: "alpha"}, {Name: "gamma"}}) {
		t.Error("different names should not be equal")
	}

	m := e.GetMetrics()
	for _, key := range []string{"decisions", "blocked", "strategy_changes"} {
		if _, ok := m[key]; !ok {
			t.Errorf("GetMetrics missing key %q", key)
		}
	}
}
