package chat

import (
	"strings"
	"testing"
)

func TestNorthstarRouter_InitializesAMissingWizard(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = nil
	m.inputMode = InputModeNorthstar
	got, _ := m.handleNorthstarWizardInput("back")
	gm := got.(Model)
	if gm.northstarWizard == nil {
		t.Fatalf("expected router to initialize nil wizard")
	}
	if !strings.Contains(lastAssistantMessage(gm), "Problem Statement") {
		t.Fatalf("expected router back to show problem prompt, got %q", lastAssistantMessage(gm))
	}
}

func TestAutoHeuristic_CapabilitySuggestionCapBreaksAtEight(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	for i := 0; i < 10; i++ {
		w.Capabilities = append(w.Capabilities, Capability{Description: "low-cap-break-" + string(rune('a'+i)) + string(rune('a'+i)) + string(rune('a'+i)), Priority: "low"})
	}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if n := strings.Count(content, "Consider a requirement for capability"); n != 8 {
		t.Fatalf("expected break to cap capability suggestions at 8, got %d in %q", n, content)
	}
	if len(w.Requirements) != 0 {
		t.Fatalf("low-priority caps must not fabricate requirements, got %v", w.Requirements)
	}
}
