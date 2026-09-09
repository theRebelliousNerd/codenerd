package core

import (
	"testing"

	"codenerd/internal/types"
)

// DreamRouter asserted DreamLearning.Confidence — a 0..1 float — into slots
// schemas_dreamer.mg bounds /number. The kernel's Decl coercion refuses a
// fractional float there (this Mangle fork compares int64 only, and one such
// fact aborts the whole fixpoint), so dream_tool_need, dream_risk_pattern and
// dream_preference were dropped on every single route. Two of the three then
// reported Destination "Kernel:..." with Success true, and RouteLearnings
// marked the learning Persisted, so it was never retried either.

func routedPreference() *DreamLearning {
	return &DreamLearning{
		ID:         "learn-pref",
		Type:       LearningTypePreference,
		Content:    "prefers table-driven tests",
		Confidence: 0.85,
		Confirmed:  true,
	}
}

func TestDreamRouter_PreferenceReachesTheKernel(t *testing.T) {
	k := setupMockKernel(t)
	router := NewDreamRouter(k, nil, nil) // no cold store: kernel is the only destination

	results := router.RouteLearnings([]*DreamLearning{routedPreference()})
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("preference route failed: %s", results[0].ErrorMessage)
	}

	facts, err := k.Query("dream_preference")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("router claimed Kernel:dream_preference but the kernel holds %d facts", len(facts))
	}
	if got, ok := types.ExtractInt64(facts[0].Args[1]); !ok || got != 85 {
		t.Errorf("confidence should be stored as integer percent, got %v", facts[0].Args[1])
	}
}

func TestDreamRouter_RiskPatternReachesTheKernel(t *testing.T) {
	k := setupMockKernel(t)
	router := NewDreamRouter(k, nil, nil)

	results := router.RouteLearnings([]*DreamLearning{{
		ID:           "learn-risk",
		Type:         LearningTypeRiskPattern,
		Content:      "deleting migrations loses history",
		Hypothetical: "what if we drop the migrations directory",
		Confidence:   0.6, // below the 0.7 cold-storage threshold: kernel-only
		Confirmed:    true,
		Metadata:     map[string]string{"risk_type": "data_integrity"},
	}})
	if !results[0].Success {
		t.Fatalf("risk route failed: %s", results[0].ErrorMessage)
	}

	facts, err := k.Query("dream_risk_pattern")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("router claimed Kernel:dream_risk_pattern but the kernel holds %d facts", len(facts))
	}
}

func TestDreamRouter_ToolNeedReachesTheKernel(t *testing.T) {
	k := setupMockKernel(t)
	router := NewDreamRouter(k, nil, nil)

	results := router.RouteLearnings([]*DreamLearning{{
		ID:           "learn-tool",
		Type:         LearningTypeToolNeed,
		Content:      "no way to diff two Mangle programs",
		Hypothetical: "what if we could diff programs",
		Confidence:   0.75,
		Confirmed:    true,
		Metadata:     map[string]string{"tool_name": "mangle_diff"},
	}})
	if !results[0].Success {
		t.Fatalf("tool-need route failed: %s", results[0].ErrorMessage)
	}

	facts, err := k.Query("dream_tool_need")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("router claimed Kernel:dream_tool_need but the kernel holds %d facts", len(facts))
	}
}
