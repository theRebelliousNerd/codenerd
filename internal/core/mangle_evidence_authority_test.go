package core

import "testing"

func TestModelCannotForgeCompletionEvidence(t *testing.T) {
	updates := []string{`turn_acceptance(/turn_1, "contract", "snapshot").`, `turn_done(/turn_1).`, `turn_evidence(/turn_1, /fix, 1, 1, 1, /false, /false).`}
	for _, policy := range []MangleUpdatePolicy{{}, {AllowedPrefixes: []string{"turn_"}}} {
		facts, blocked := FilterMangleUpdates(nil, updates, policy)
		if len(facts) != 0 || len(blocked) != len(updates) {
			t.Fatalf("model evidence admitted: facts=%v blocked=%v", facts, blocked)
		}
	}
}
