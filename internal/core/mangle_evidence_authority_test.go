package core

import "testing"

func TestModelCannotForgeCompletionEvidence(t *testing.T) {
	updates := []string{`turn_acceptance(/fix, "contract", "snapshot").`, `turn_done(/fix).`, `turn_evidence(/fix, 1, 1, 1, 0, 0).`}
	for _, policy := range []MangleUpdatePolicy{{}, {AllowedPrefixes: []string{"turn_"}}} {
		facts, blocked := FilterMangleUpdates(nil, updates, policy)
		if len(facts) != 0 || len(blocked) != len(updates) {
			t.Fatalf("model evidence admitted: facts=%v blocked=%v", facts, blocked)
		}
	}
}
