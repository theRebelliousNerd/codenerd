package chat

import (
	"errors"
	"testing"

	"codenerd/internal/northstar"
)

type stubKB struct {
	calls       []kbCall
	failConcept string
}

type kbCall struct {
	concept    string
	content    string
	confidence float64
}

func (s *stubKB) StoreKnowledgeAtom(concept, content string, confidence float64) error {
	s.calls = append(s.calls, kbCall{
		concept:    concept,
		content:    content,
		confidence: confidence,
	})
	if concept == s.failConcept {
		return errors.New("boom")
	}
	return nil
}

func TestSaveNorthstarToKnowledgeBase(t *testing.T) {
	vision := &northstar.Vision{
		Mission:    "mission",
		Problem:    "problem",
		VisionStmt: "vision",
		Personas:   []northstar.Persona{{Name: "Persona", Needs: []string{"need"}}},
		Capabilities: []northstar.Capability{
			{ID: "cap_1", Description: "Cap", Timeline: "now", Priority: "high"},
		},
		Risks: []northstar.Risk{
			{ID: "risk_1", Description: "Risk", Likelihood: "high", Impact: "low", Mitigation: "none"},
		},
		Requirements: []northstar.Requirement{
			{ID: "req-a", Type: "functional", Description: "Req", Priority: "must-have"},
		},
		Constraints: []string{"constraint"},
	}

	kb := &stubKB{failConcept: "northstar:vision"}
	errs := saveNorthstarToKnowledgeBase(kb, vision)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if len(kb.calls) == 0 {
		t.Fatalf("expected calls to store knowledge atoms")
	}
}
