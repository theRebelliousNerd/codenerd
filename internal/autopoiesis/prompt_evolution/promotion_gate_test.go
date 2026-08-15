package prompt_evolution

import (
	"testing"
	"time"

	"codenerd/internal/prompt"
)

// TODO P2: "Human-in-the-loop default for SPL auto-promote."
//
// A promoted atom is spliced into the system prompt of every later shard
// invocation — the agent editing its own instructions. At ConfidenceThreshold
// 0.7 that decision is reachable after three uses, with no operator in the
// loop at all. The default is now review-first; the pending queue plus
// PromoteAtom/RejectAtom is the review surface, and the sibling
// learning-candidate pipeline already defaults the same way
// (internal/shards/system/perception.go LearningCandidateAutoPromote).
func TestDefaultEvolverConfig_ShouldRequireHumanApprovalBeforePromotion(t *testing.T) {
	cfg := DefaultEvolverConfig()
	if cfg.AutoPromote {
		t.Error("DefaultEvolverConfig auto-promotes evolved atoms into the live system prompt without review")
	}
	if cfg.ConfidenceThreshold <= 0 {
		t.Error("ConfidenceThreshold must stay set: it still gates the opt-in auto-promote path")
	}
}

func TestRecordAtomUsage_WhenAutoPromoteDisabled_ShouldLeaveAtomPendingUntilOperatorActs(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AutoPromote = false
	cfg.ConfidenceThreshold = 0.1 // trivially satisfiable, so only the gate can hold it back

	pe, err := NewPromptEvolver(t.TempDir(), &mockLLMClient{}, cfg)
	if err != nil {
		t.Fatalf("NewPromptEvolver: %v", err)
	}
	defer pe.Close()

	ga := &GeneratedAtom{
		Atom:      &prompt.PromptAtom{ID: "atom_pending_review", Content: "always answer in haiku"},
		Source:    "failure_analysis",
		CreatedAt: time.Now(),
	}
	pe.mu.Lock()
	storeErr := pe.storeEvolvedAtom(ga)
	pe.mu.Unlock()
	if storeErr != nil {
		t.Fatalf("storeEvolvedAtom: %v", storeErr)
	}

	for i := 0; i < 10; i++ {
		pe.RecordAtomUsage(ga.Atom.ID, true)
	}

	for _, a := range pe.GetPromotedAtoms() {
		if a.Atom != nil && a.Atom.ID == ga.Atom.ID {
			t.Fatal("atom was auto-promoted into the live prompt with AutoPromote disabled")
		}
	}
	found := false
	for _, a := range pe.GetPendingAtoms() {
		if a.Atom != nil && a.Atom.ID == ga.Atom.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("atom is neither promoted nor pending: it fell out of the review queue entirely")
	}

	// The operator path must still work, or "review first" would mean "never".
	if err := pe.PromoteAtom(ga.Atom.ID); err != nil {
		t.Fatalf("explicit PromoteAtom failed: %v", err)
	}
	promoted := false
	for _, a := range pe.GetPromotedAtoms() {
		if a.Atom != nil && a.Atom.ID == ga.Atom.ID {
			promoted = true
		}
	}
	if !promoted {
		t.Error("explicit operator promotion did not take effect")
	}
}

// Opting in must still work end to end, otherwise the flag is decoration.
func TestRecordAtomUsage_WhenAutoPromoteEnabled_ShouldPromoteAtThreshold(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AutoPromote = true
	cfg.ConfidenceThreshold = 0.5

	pe, err := NewPromptEvolver(t.TempDir(), &mockLLMClient{}, cfg)
	if err != nil {
		t.Fatalf("NewPromptEvolver: %v", err)
	}
	defer pe.Close()

	ga := &GeneratedAtom{
		Atom:      &prompt.PromptAtom{ID: "atom_auto_promoted", Content: "prefer table output"},
		Source:    "failure_analysis",
		CreatedAt: time.Now(),
	}
	pe.mu.Lock()
	storeErr := pe.storeEvolvedAtom(ga)
	pe.mu.Unlock()
	if storeErr != nil {
		t.Fatalf("storeEvolvedAtom: %v", storeErr)
	}

	for i := 0; i < 5; i++ {
		pe.RecordAtomUsage(ga.Atom.ID, true)
	}

	for _, a := range pe.GetPromotedAtoms() {
		if a.Atom != nil && a.Atom.ID == ga.Atom.ID {
			return
		}
	}
	t.Error("AutoPromote=true did not promote an atom that cleared the threshold")
}
