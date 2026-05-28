package core

import (
	"testing"

	"codenerd/internal/features"
)

// TestDebugDiff: temporary trace to figure out why the diff engine isn't built.
func TestDebugDiff(t *testing.T) {
	t.Cleanup(func() { features.SetActive(nil) })
	ta := true
	features.SetActive(&features.FeaturesConfig{DiffEval: &ta})
	t.Logf("IsDiffEvalEnabled=%v", features.IsDiffEvalEnabled())

	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl widget(Name).
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	k.mu.RLock()
	t.Logf("after eval1: diffEngine=%v programInfo=%v proofRecorder=%v",
		k.diffEngine != nil, k.programInfo != nil, k.proofRecorder != nil)
	k.mu.RUnlock()

	if err := k.Assert(Fact{Predicate: "widget", Args: []any{"/a"}}); err != nil {
		t.Fatal(err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("evaluate2: %v", err)
	}
	k.mu.RLock()
	t.Logf("after eval2: diffEngine=%v", k.diffEngine != nil)
	k.mu.RUnlock()
}
