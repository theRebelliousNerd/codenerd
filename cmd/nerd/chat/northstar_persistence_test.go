package chat

import (
	"errors"
	"strings"
	"testing"
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

type stubKernel struct {
	facts       []string
	failContain string
}

func (s *stubKernel) AssertString(fact string) error {
	s.facts = append(s.facts, fact)
	if s.failContain != "" && strings.Contains(fact, s.failContain) {
		return errors.New("fail")
	}
	return nil
}

func TestGenerateNorthstarMangle(t *testing.T) {
	wizard := &NorthstarWizardState{
		Mission: "mission",
		Problem: "problem",
		Vision:  "vision",
		Personas: []UserPersona{
			{Name: "Persona", PainPoints: []string{"pain"}, Needs: []string{"need"}},
		},
		Capabilities: []Capability{
			{Description: "Cap", Timeline: "6 mo", Priority: "high"},
		},
		Risks: []Risk{
			{Description: "Risk", Likelihood: "high", Impact: "low", Mitigation: "mitigate"},
		},
		Requirements: []NorthstarRequirement{
			{ID: "REQ-A", Type: "functional", Description: "Req", Priority: "must-have"},
		},
		Constraints: []string{"constraint"},
	}

	mangleFacts := generateNorthstarMangle(wizard)
	expect := []string{
		`northstar_mission(/ns_mission, "mission").`,
		`northstar_problem(/ns_problem, "problem").`,
		`northstar_vision(/ns_vision, "vision").`,
		`northstar_persona(/persona_1, "Persona").`,
		`northstar_pain_point(/persona_1, "pain").`,
		`northstar_need(/persona_1, "need").`,
		`northstar_capability(/cap_1, "Cap", /6_mo, /high).`,
		`northstar_risk(/risk_1, "Risk", /high, /low).`,
		`northstar_mitigation(/risk_1, "mitigate").`,
		`northstar_requirement(/req-a, /functional, "Req", /must_have).`,
		`northstar_constraint(/constraint_1, "constraint").`,
	}
	for _, fragment := range expect {
		if !strings.Contains(mangleFacts, fragment) {
			t.Fatalf("expected mangle facts to include %q", fragment)
		}
	}
}

func TestSaveNorthstarToKnowledgeBase(t *testing.T) {
	wizard := &NorthstarWizardState{
		Mission: "mission",
		Problem: "problem",
		Vision:  "vision",
		Personas: []UserPersona{
			{Name: "Persona", Needs: []string{"need"}},
		},
		Capabilities: []Capability{
			{Description: "Cap", Timeline: "now", Priority: "high"},
		},
		Risks: []Risk{
			{Description: "Risk", Likelihood: "high", Impact: "low", Mitigation: "none"},
		},
		Requirements: []NorthstarRequirement{
			{ID: "REQ-A", Type: "functional", Description: "Req", Priority: "must-have"},
		},
		Constraints: []string{"constraint"},
	}

	kb := &stubKB{failConcept: "northstar:vision"}
	errs := saveNorthstarToKnowledgeBase(kb, wizard)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if len(kb.calls) == 0 {
		t.Fatalf("expected calls to store knowledge atoms")
	}
}

func TestAssertNorthstarFacts(t *testing.T) {
	wizard := &NorthstarWizardState{
		Mission: "mission",
		Problem: "problem",
		Vision:  "vision",
	}
	kernel := &stubKernel{failContain: "northstar_problem"}
	errs := assertNorthstarFacts(kernel, wizard)
	if len(kernel.facts) == 0 {
		t.Fatalf("expected facts to be asserted")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !containsFact(kernel.facts, "northstar_defined().") {
		t.Fatalf("expected northstar_defined fact")
	}
}

func containsFact(facts []string, match string) bool {
	for _, fact := range facts {
		if fact == match {
			return true
		}
	}
	return false
}
