package chat

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// /learn turned every UI message into its own "successful" exchange: a user
// correction was labelled an agent action, an answer had no request, and the
// command's own status notice was one more success (audit
// manual-learning-evidence-conservation). The critic's input is the
// history's exchanges -- a user message and the reply that followed it --
// with outcomes the TUI never measured reported as unknown, and a trailing
// correction kept, marked as having no reply.
func TestLearnTraces_PairExchangesAndKeepOutcomesUnknown(t *testing.T) {
	history := []Message{
		{Role: "user", Content: "add a feature"},
		{Role: "assistant", Content: "Added it."},
		{Role: "system", Content: "glass box event"},
		{Role: "assistant", Content: "Learning candidates: none."},
		{Role: "user", Content: "no, I meant with a regression test"},
	}
	traces := learnTracesFromHistory(history)
	if len(traces) != 2 {
		t.Fatalf("traces = %+v, want the exchange and the trailing correction", traces)
	}
	if traces[0].UserPrompt != "add a feature" || traces[0].Response != "Added it." {
		t.Fatalf("first trace = %+v, want the request with its reply", traces[0])
	}
	if traces[1].UserPrompt != "no, I meant with a regression test" || traces[1].Response != "" {
		t.Fatalf("second trace = %+v, want the correction with no reply", traces[1])
	}
	for i, tr := range traces {
		if tr.Success || !tr.OutcomeUnknown {
			t.Fatalf("trace %d claims an outcome the TUI did not measure: %+v", i, tr)
		}
	}
}

// A proposal confirmed by id is the proposal staged: its qualifier and
// confidence survive. Staging kept only phrase, verb and target, and the id
// route rebuilt the fact with an empty qualifier and 0.90.
func TestLearnCandidate_StagedFactSurvivesConfirmationByID(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	db, err := store.NewLocalStore(t.TempDir() + "/knowledge.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m.localDB = db
	fact := `learned_exemplar("add a feature", /create, "feature", "include a regression test", 0.97).`
	if _, err := m.stageLearningCandidateFromFact(fact, criticManualLearnReason); err != nil {
		t.Fatal(err)
	}
	cands, err := db.ListLearningCandidates("", 10)
	if err != nil || len(cands) != 1 {
		t.Fatalf("candidates = %+v, %v", cands, err)
	}
	_, _, _, qualifier, confidence, err := perception.ParseLearnedFact(cands[0].Fact)
	if err != nil {
		t.Fatalf("the staged candidate holds no proposal: %q (%v)", cands[0].Fact, err)
	}
	if qualifier != "include a regression test" || confidence != 97 {
		t.Fatalf("staged proposal = qualifier %q confidence %v, want the critic's", qualifier, confidence)
	}
}
