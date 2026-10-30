package system

import (
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// The executive stages a candidate for an intent it found no action for: the
// mapping, and the fact it implies, go to the local store, and at the
// threshold the kernel gets both learning_candidate and the
// learning_candidate_fact an inline confirmation persists. Without the fact
// the chat's confirmation has nothing staged to learn.
func TestExecutive_StagesItsCandidateWithItsFact(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutivePolicyShard()
	e.Kernel = kernel
	e.config.LearningCandidateThreshold = 1
	e.SetLearningCandidateStore(db)

	e.recordNoActionCandidate(&userIntentSnapshot{Verb: "/deploy", Target: "service", Constraint: "ship the service"}, "/no_action")

	cands, err := db.ListLearningCandidates("", 10)
	if err != nil || len(cands) != 1 {
		t.Fatalf("staged candidates = %+v, %v", cands, err)
	}
	if _, verb, target, _, _, err := perception.ParseLearnedFact(cands[0].Fact); err != nil || verb != "/deploy" || target != "service" {
		t.Fatalf("staged proposal = %q (%v), want the mapping's fact", cands[0].Fact, err)
	}
	rows, err := kernel.Query("learning_candidate_fact")
	if err != nil || len(rows) != 1 {
		t.Fatalf("learning_candidate_fact rows = %v, %v; want the staged proposal", rows, err)
	}
}
