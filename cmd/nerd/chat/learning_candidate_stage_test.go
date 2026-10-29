package chat

import (
	"testing"

	"codenerd/internal/config"
)

// A learned fact stages as a candidate on the real kernel. Every producer
// asserts the verb as a name constant; the schema declared it /string, so the
// kernel refused each one ("Auto-learning candidate staging failed").
func TestStageLearningCandidate_AcceptedByTheRealKernel(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	m.localDB = nil
	fact := `learned_exemplar("make it faster", /optimize, "the retriever", "", 0.9).`
	if _, err := m.stageLearningCandidateFromFact(fact, criticAutoLearnReason); err != nil {
		t.Fatalf("staging a learned fact failed: %v", err)
	}
	for _, pred := range []string{"learning_candidate", "learning_candidate_fact", "learning_candidate_ready"} {
		rows, err := m.kernel.Query(pred)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s rows = %d (%v), want 1", pred, len(rows), err)
		}
	}
}
