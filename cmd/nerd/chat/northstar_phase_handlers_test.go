package chat

import (
	"strings"
	"testing"
)

func TestNorthstarPhaseWelcome(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	got, _ := m.handleNorthstarWelcome("no")
	gm := got.(Model)
	if gm.northstarWizard.Phase != NorthstarProblemStatement {
		t.Fatalf("welcome 'no' should advance to problem, got %v", gm.northstarWizard.Phase)
	}
	if !strings.Contains(lastAssistantMessage(gm), "Problem Statement") {
		t.Fatalf("expected problem prompt, got %q", lastAssistantMessage(gm))
	}
	m2 := InitChat(Config{})
	m2.northstarWizard = NewNorthstarWizard()
	got2, _ := m2.handleNorthstarWelcome("yes")
	gm2 := got2.(Model)
	if gm2.northstarWizard.Phase != NorthstarDocIngestion {
		t.Fatalf("welcome 'yes' should go to doc ingestion, got %v", gm2.northstarWizard.Phase)
	}
}

func TestNorthstarPhaseDocIngestion(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	got, _ := m.handleNorthstarDocIngestion("done")
	gm := got.(Model)
	if gm.northstarWizard.Phase != NorthstarProblemStatement {
		t.Fatalf("empty doc ingestion should advance to problem, got %v", gm.northstarWizard.Phase)
	}
	if !strings.Contains(lastAssistantMessage(gm), "Problem Statement") {
		t.Fatalf("expected problem prompt after done, got %q", lastAssistantMessage(gm))
	}
}

func TestNorthstarPhaseProblem(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	got, _ := m.handleNorthstarProblem("short")
	gm := got.(Model)
	if gm.northstarWizard.Problem != "" {
		t.Fatalf("short problem should be rejected, got %q", gm.northstarWizard.Problem)
	}
	if !strings.Contains(lastAssistantMessage(gm), "more detailed problem") {
		t.Fatalf("expected validation message, got %q", lastAssistantMessage(gm))
	}
	m2 := InitChat(Config{})
	m2.northstarWizard = NewNorthstarWizard()
	valid := "Deployments fail because flaky integration tests block every release train"
	got2, _ := m2.handleNorthstarProblem(valid)
	gm2 := got2.(Model)
	if gm2.northstarWizard.Problem != valid {
		t.Fatalf("valid problem should be stored, got %q", gm2.northstarWizard.Problem)
	}
	if gm2.northstarWizard.Phase != NorthstarVisionStatement {
		t.Fatalf("valid problem should advance to vision, got %v", gm2.northstarWizard.Phase)
	}
}

func TestNorthstarPhaseVision(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	got, _ := m.handleNorthstarVision("too short")
	gm := got.(Model)
	if gm.northstarWizard.Vision != "" {
		t.Fatalf("short vision should be rejected")
	}
	if !strings.Contains(lastAssistantMessage(gm), "more detailed vision") {
		t.Fatalf("expected vision validation, got %q", lastAssistantMessage(gm))
	}
	m2 := InitChat(Config{})
	m2.northstarWizard = NewNorthstarWizard()
	vision := "Every team ships confidently with zero flaky failures and instant rollback safety nets"
	got2, _ := m2.handleNorthstarVision(vision)
	gm2 := got2.(Model)
	if gm2.northstarWizard.Vision != vision {
		t.Fatalf("vision should be stored, got %q", gm2.northstarWizard.Vision)
	}
	if gm2.northstarWizard.SubStep != 1 {
		t.Fatalf("vision accept should move to mission substep, got %d", gm2.northstarWizard.SubStep)
	}
	got3, _ := gm2.handleNorthstarVision("short")
	gm3 := got3.(Model)
	if gm3.northstarWizard.Mission != "" {
		t.Fatalf("short mission should be rejected")
	}
	got4, _ := gm3.handleNorthstarVision("Make coding accessible to everyone")
	gm4 := got4.(Model)
	if gm4.northstarWizard.Mission == "" {
		t.Fatalf("mission should be stored")
	}
	if gm4.northstarWizard.Phase != NorthstarTargetUsers {
		t.Fatalf("mission accept should advance to users, got %v", gm4.northstarWizard.Phase)
	}
}

func TestNorthstarPhaseConstraints(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.workspace = t.TempDir()
	got, _ := m.handleNorthstarConstraints("Must run on Linux")
	gm := got.(Model)
	if len(gm.northstarWizard.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %v", gm.northstarWizard.Constraints)
	}
	m2 := InitChat(Config{})
	m2.northstarWizard = NewNorthstarWizard()
	m2.workspace = t.TempDir()
	got2, _ := m2.handleNorthstarConstraints("done")
	gm2 := got2.(Model)
	if gm2.northstarWizard.Phase != NorthstarSummary {
		t.Fatalf("constraints done should go to summary, got %v", gm2.northstarWizard.Phase)
	}
}

func TestNorthstarPhaseShowSummary(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.workspace = t.TempDir()
	got, _ := m.showNorthstarSummary()
	gm := got.(Model)
	if !strings.Contains(lastAssistantMessage(gm), "NORTHSTAR SUMMARY") {
		t.Fatalf("expected summary content, got %q", lastAssistantMessage(gm))
	}
}

func TestNorthstarPhaseSummaryHandler(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.workspace = t.TempDir()
	got, _ := m.handleNorthstarSummary("bogus-choice")
	gm := got.(Model)
	if !strings.Contains(lastAssistantMessage(gm), "save") {
		t.Fatalf("expected save/edit/campaign hint, got %q", lastAssistantMessage(gm))
	}
	m2 := InitChat(Config{})
	m2.northstarWizard = NewNorthstarWizard()
	m2.workspace = t.TempDir()
	got2, _ := m2.handleNorthstarSummary("edit")
	gm2 := got2.(Model)
	if gm2.northstarWizard.Phase != NorthstarProblemStatement {
		t.Fatalf("edit should return to problem statement, got %v", gm2.northstarWizard.Phase)
	}
}

func TestNorthstarPhaseCapabilityCapSuppressesRiskAtBoundary(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	for i := 0; i < 8; i++ {
		w.Capabilities = append(w.Capabilities, Capability{Description: "boundary-low-cap-" + string(rune('a'+i)), Priority: "low"})
	}
	w.Risks = []Risk{{Description: "r-boundary", Impact: "low", Mitigation: ""}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if n := strings.Count(content, "Consider a requirement for capability"); n != 8 {
		t.Fatalf("expected 8 capability suggestions at boundary, got %d in %q", n, content)
	}
	if strings.Contains(content, "have no mitigation yet") {
		t.Fatalf("risk suggestion must be suppressed once cap reached, got %q", content)
	}
}
