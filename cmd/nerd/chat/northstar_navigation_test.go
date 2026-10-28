package chat

import (
	"strings"
	"testing"
)

func TestNorthstarNavigation_Advance(t *testing.T) {
	w := NewNorthstarWizard()
	w.Phase = NorthstarProblemStatement
	m := newHeuristicTestModel(w)
	got, _ := m.advanceNorthstarPhase()
	gm := got.(Model)
	if gm.northstarWizard.Phase != NorthstarVisionStatement {
		t.Fatalf("expected advance Problem->Vision, got %v", gm.northstarWizard.Phase)
	}
}

func TestNorthstarNavigation_Previous(t *testing.T) {
	w := NewNorthstarWizard()
	w.Phase = NorthstarProblemStatement
	m := newHeuristicTestModel(w)
	got, _ := m.previousNorthstarPhase()
	gm := got.(Model)
	if gm.northstarWizard.Phase != NorthstarWelcome {
		t.Fatalf("expected previous Problem->Welcome, got %v", gm.northstarWizard.Phase)
	}
}

func TestNorthstarNavigation_ShowPrompt(t *testing.T) {
	w := NewNorthstarWizard()
	w.Phase = NorthstarProblemStatement
	m := newHeuristicTestModel(w)
	got, _ := m.showNorthstarPhasePrompt()
	gm := got.(Model)
	content := lastAssistantMessage(gm)
	if !strings.Contains(content, "Problem Statement") {
		t.Fatalf("expected problem prompt, got %q", content)
	}
}
