package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newHeuristicTestModel(w *NorthstarWizardState) Model {
	m := InitChat(Config{})
	m.northstarWizard = w
	if m.northstarWizard == nil {
		m.northstarWizard = NewNorthstarWizard()
	}
	return m
}

func lastAssistantMessage(m Model) string {
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Role == "assistant" {
			return m.history[i].Content
		}
	}
	return ""
}

func TestAutoHeuristic_PersonaSourceCounted(t *testing.T) {
	w := NewNorthstarWizard()
	w.Personas = []UserPersona{{Name: "Ada", PainPoints: []string{"p"}, Needs: []string{"n"}}}
	w.Problem = "prob"
	w.Vision = "vision"
	w.ResearchDocs = []string{"doc"}
	w.ExtractedFacts = []string{"system must handle load"}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "1 persona(s)") {
		t.Fatalf("expected persona source counted, got %q", content)
	}
}

func TestAutoHeuristic_CoverageInitAndMarking(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Capabilities = []Capability{{Description: "low cap", Priority: "low"}}
	w.ExtractedFacts = []string{"unrelated prose without keywords"}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "Consider a requirement for capability") {
		t.Fatalf("expected uncovered capability suggestion, got %q", content)
	}
	if !strings.Contains(content, "low cap") {
		t.Fatalf("expected suggestion to name capability, got %q", content)
	}
}

func TestAutoHeuristic_UnmitigatedRiskSuggestion(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Risks = []Risk{{Description: "r1", Impact: "low", Mitigation: ""}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "have no mitigation yet") {
		t.Fatalf("expected unmitigated risk suggestion, got %q", content)
	}
}

func TestAutoHeuristic_MitigatedRiskNoSuggestion(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Risks = []Risk{{Description: "r1", Impact: "high", Mitigation: "fix it"}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if strings.Contains(content, "have no mitigation yet") {
		t.Fatalf("did not expect mitigation suggestion, got %q", content)
	}
	if len(w.Requirements) == 0 {
		t.Fatalf("expected high-impact mitigated risk to generate requirement")
	}
}

func TestAutoHeuristic_EmptyInterviewQuestions(t *testing.T) {
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	for _, want := range []string{
		"Who is the primary user?",
		"What is the one capability",
		"Which part of the codebase",
		"What problem does this solve",
		"If this succeeds wildly",
		"Explore the codebase",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected interview prompt %q in %q", want, content)
		}
	}
}

func TestAutoHeuristic_FullContextSuppressesQuestions(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.Capabilities = []Capability{{Description: "fast", Priority: "critical"}}
	w.ResearchDocs = []string{"doc"}
	w.ExtractedFacts = []string{"system must be fast"}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	for _, unwanted := range []string{
		"Who is the primary user?",
		"What is the one capability",
		"Which part of the codebase",
		"What problem does this solve",
		"If this succeeds wildly",
	} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("did not expect question %q in full context: %q", unwanted, content)
		}
	}
	if !strings.Contains(content, "Auto-generated") {
		t.Fatalf("expected summary line, got %q", content)
	}
}

func TestAutoHeuristic_CoveredCapabilityNoSuggestion(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Capabilities = []Capability{{Description: "fast search", Priority: "critical"}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if strings.Contains(content, "Consider a requirement for capability") {
		t.Fatalf("covered critical capability should not be suggested, got %q", content)
	}
}

func TestNorthstarSurveyWorkspace_EmptyAndMissing(t *testing.T) {
	if got := northstarSurveyWorkspace(""); len(got) != 0 {
		t.Fatalf("expected empty survey for empty workspace, got %v", got)
	}
	if got := northstarSurveyWorkspace(filepath.Join(t.TempDir(), "does-not-exist")); len(got) != 0 {
		t.Fatalf("expected empty survey for missing dir, got %v", got)
	}
}

func TestNorthstarSurveyWorkspace_DetectsDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"cmd", "internal", "docs"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := northstarSurveyWorkspace(root)
	joined := strings.Join(got, ",")
	for _, want := range []string{"cmd", "internal", "go.mod"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected survey to contain %q, got %v", want, got)
		}
	}
}

func TestAutoHeuristic_WorkspaceGrounding(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"chat", "kernel"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	// Agentic behaviors: explores codebase (names detected areas),
	// asks a targeted question, and suggests concrete next steps.
	for _, want := range []string{"Explore the codebase", "detected", "chat", "Which part of the codebase", "Consider a requirement covering"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected workspace-grounded auto mode to contain %q, got %q", want, content)
		}
	}
}

func TestAutoHeuristic_DisplayTruncation(t *testing.T) {
	root := t.TempDir()
	// Create many long directory names so joined display exceeds 160 chars.
	for i := 0; i < 10; i++ {
		name := strings.Repeat(string(rune('a'+i)), 4) + "-very-long-directory-name-00"
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "Explore the codebase: detected") {
		t.Fatalf("expected detected codebase suggestion, got %q", content)
	}
	if !strings.Contains(content, "...") {
		t.Fatalf("expected truncated display with ellipsis, got %q", content)
	}
	if !strings.Contains(content, "Which part of the codebase should requirements cover first (detected:") {
		t.Fatalf("expected detected question, got %q", content)
	}
}

func TestAutoHeuristic_NoAreasFallback(t *testing.T) {
	root := t.TempDir()
	// Empty directory yields no survey areas, exercising the fallback branch.
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "Explore the codebase: run /scan") {
		t.Fatalf("expected fallback explore suggestion, got %q", content)
	}
	if !strings.Contains(content, "(e.g. CLI, kernel, chat)?") {
		t.Fatalf("expected fallback codebase question, got %q", content)
	}
	if strings.Contains(content, "(detected:") {
		t.Fatalf("did not expect detected question in fallback, got %q", content)
	}
}

func TestAutoHeuristic_AgenticInterviewExploresAsksAndSuggests(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"cmd", "internal", ".git", ".nerd", "node_modules", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example"), 0o644); err != nil {
		t.Fatal(err)
	}
	areas := northstarSurveyWorkspace(root)
	joined := strings.Join(areas, ",")
	for _, want := range []string{"cmd", "internal", "go.mod"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected survey to explore %q, got %v", want, areas)
		}
	}
	for _, unwanted := range []string{".git", ".nerd", "node_modules", ".hidden"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("survey must skip noise %q, got %v", unwanted, areas)
		}
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	// Explore: names detected areas instead of generic placeholder.
	for _, want := range []string{"Explore the codebase", "detected", "cmd", "internal"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected exploration to contain %q, got %q", want, content)
		}
	}
	// Ask: targeted codebase question grounded in detected areas.
	if !strings.Contains(content, "Which part of the codebase should requirements cover first (detected:") {
		t.Fatalf("expected targeted codebase question, got %q", content)
	}
	// Suggest: concrete per-area next steps plus interview sharpen section.
	for _, want := range []string{"Consider a requirement covering", "To sharpen this", "done"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected suggestion/interview marker %q in %q", want, content)
		}
	}
	// Empty context must not fabricate structured requirements.
	if len(w.Requirements) != 0 {
		t.Fatalf("expected no requirements from empty context, got %v", w.Requirements)
	}
	// Answering the interview suppresses the answered questions: state-driven, not static text.
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.Capabilities = []Capability{{Description: "fast", Priority: "critical"}}
	w.ResearchDocs = []string{"doc"}
	w.ExtractedFacts = []string{"system must be fast"}
	m2 := newHeuristicTestModel(w)
	m2.workspace = root
	got2, _ := m2.autoGenerateRequirementsHeuristic()
	content2 := lastAssistantMessage(got2.(Model))
	for _, answered := range []string{"Who is the primary user?", "Which part of the codebase"} {
		if strings.Contains(content2, answered) {
			t.Fatalf("answered interview question %q should be suppressed, got %q", answered, content2)
		}
	}
}

func TestAutoHeuristic_IdempotentRerun(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.Capabilities = []Capability{{Description: "fast search", Priority: "critical"}}
	w.Risks = []Risk{{Description: "outage", Impact: "high", Mitigation: "add retries"}}
	w.ExtractedFacts = []string{"system must handle load"}
	w.ResearchDocs = []string{"doc"}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	first := len(w.Requirements)
	if first == 0 {
		t.Fatalf("expected structured requirements, got none")
	}
	seen := map[string]bool{}
	for _, r := range w.Requirements {
		if r.ID == "" || r.Description == "" || r.Source == "" {
			t.Fatalf("requirement must carry structured fields, got %+v", r)
		}
		if !strings.HasPrefix(r.ID, "REQ-") {
			t.Fatalf("requirement ID must be REQ- prefixed, got %q", r.ID)
		}
		if seen[r.ID] {
			t.Fatalf("duplicate requirement ID %q", r.ID)
		}
		seen[r.ID] = true
	}
	got2, _ := got.(Model).autoGenerateRequirementsHeuristic()
	second := len(w.Requirements)
	if second != first {
		t.Fatalf("second auto run must be idempotent: first=%d second=%d", first, second)
	}
	content := lastAssistantMessage(got2.(Model))
	if !strings.Contains(content, "Auto-generated **0 requirements**") {
		t.Fatalf("expected rerun to report 0 new requirements, got %q", content)
	}
}

func TestAutoHeuristic_EmptyWizardInterviews(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.workspace = ""
	got, _ := m.autoGenerateRequirementsHeuristic()
	gotModel, ok := got.(Model)
	if !ok {
		t.Fatalf("expected Model return, got %T", got)
	}
	if len(gotModel.northstarWizard.Requirements) != 0 {
		t.Fatalf("expected no requirements from empty context, got %v", gotModel.northstarWizard.Requirements)
	}
	content := lastAssistantMessage(gotModel)
	for _, want := range []string{
		"Auto-generated **0 requirements**",
		"Explore the codebase",
		"Who is the primary user?",
		"What is the one capability",
		"To sharpen this",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected empty-wizard interview to contain %q, got %q", want, content)
		}
	}
}

func TestAutoHeuristic_EmptyWizardWithWorkspaceGrounding(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"cmd", "internal"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	gotModel, ok := got.(Model)
	if !ok {
		t.Fatalf("expected Model return, got %T", got)
	}
	content := lastAssistantMessage(gotModel)
	for _, want := range []string{"Explore the codebase", "detected", "cmd", "Which part of the codebase", "Consider a requirement covering"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected empty-wizard workspace grounding to contain %q, got %q", want, content)
		}
	}
}

func TestAutoHeuristic_PinCapabilityRequirementViaCaller(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Capabilities = []Capability{{Description: "unique-critical-capability-xyz", Priority: "critical"}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	_ = lastAssistantMessage(got.(Model))
	if len(w.Requirements) != 1 {
		t.Fatalf("expected 1 requirement from uncovered critical capability, got %v", w.Requirements)
	}
	r := w.Requirements[0]
	if r.Source != "capability" {
		t.Fatalf("expected capability source, got %+v", r)
	}
	if !strings.Contains(r.Description, "unique-critical-capability-xyz") {
		t.Fatalf("expected requirement to contain capability description, got %q", r.Description)
	}
}

func TestAutoHeuristic_PinRiskMitigationRequirementViaCaller(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.Risks = []Risk{{Description: "outage-risk-xyz", Impact: "high", Mitigation: "unique-mitigation-xyz"}}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	_ = lastAssistantMessage(got.(Model))
	if len(w.Requirements) != 1 {
		t.Fatalf("expected 1 requirement from high-impact mitigated risk, got %v", w.Requirements)
	}
	r := w.Requirements[0]
	if r.Source != "risk-mitigation" {
		t.Fatalf("expected risk-mitigation source, got %+v", r)
	}
	if !strings.Contains(r.Description, "unique-mitigation-xyz") {
		t.Fatalf("expected requirement to contain mitigation, got %q", r.Description)
	}
}

func TestAutoHeuristic_PinFactRequirementViaCaller(t *testing.T) {
	w := NewNorthstarWizard()
	w.Problem = "prob"
	w.Vision = "vision"
	w.Personas = []UserPersona{{Name: "Ada"}}
	w.ResearchDocs = []string{"doc"}
	w.ExtractedFacts = []string{"system must handle unique-fact-xyz load"}
	m := newHeuristicTestModel(w)
	got, _ := m.autoGenerateRequirementsHeuristic()
	_ = lastAssistantMessage(got.(Model))
	if len(w.Requirements) != 1 {
		t.Fatalf("expected 1 requirement from keyword fact, got %v", w.Requirements)
	}
	r := w.Requirements[0]
	if r.Source != "research" {
		t.Fatalf("expected research source, got %+v", r)
	}
	if !strings.Contains(r.Description, "unique-fact-xyz") {
		t.Fatalf("expected requirement to contain fact, got %q", r.Description)
	}
}

func TestAutoHeuristic_PinSurveyViaCallerFiltersNoise(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"cmd", "internal"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{".git", ".nerd", "node_modules", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	for _, want := range []string{"cmd", "internal", "go.mod", "detected", "Explore the codebase", "Consider a requirement covering", "Which part of the codebase should requirements cover first (detected:"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected workspace-grounded output to contain %q, got %q", want, content)
		}
	}
	for _, unwanted := range []string{".git", ".nerd", "node_modules", ".hidden"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("survey via caller must filter noise %q, got %q", unwanted, content)
		}
	}
}

func TestAutoHeuristic_PinSurveySortingViaCaller(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"zebra", "apple"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example"), 0o644); err != nil {
		t.Fatal(err)
	}
	areas := northstarSurveyWorkspace(root)
	if len(areas) != 3 {
		t.Fatalf("expected 3 areas, got %v", areas)
	}
	if areas[0] != "apple" || areas[1] != "zebra" || areas[2] != "go.mod" {
		t.Fatalf("expected sorted dirs-first [apple zebra go.mod], got %v", areas)
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "apple, zebra, go.mod") {
		t.Fatalf("expected caller display to preserve dirs-first sorted order, got %q", content)
	}
}

func TestAutoHeuristic_PinSurveyLimitViaCaller(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		name := string(rune('a'+i)) + "-dir"
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	areas := northstarSurveyWorkspace(root)
	if len(areas) != 8 {
		t.Fatalf("expected survey to cap at 8 areas, got %v", areas)
	}
	w := NewNorthstarWizard()
	m := newHeuristicTestModel(w)
	m.workspace = root
	got, _ := m.autoGenerateRequirementsHeuristic()
	content := lastAssistantMessage(got.(Model))
	if !strings.Contains(content, "detected") {
		t.Fatalf("expected detected branch with many areas, got %q", content)
	}
	for _, want := range []string{"a-dir", "h-dir"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected capped output to contain %q, got %q", want, content)
		}
	}
	for _, unwanted := range []string{"i-dir", "j-dir"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("expected capped output to omit %q beyond 8, got %q", unwanted, content)
		}
	}
}

func TestNorthstarSurveyWorkspace_PinSkipsHiddenFilesAndRegularFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := northstarSurveyWorkspace(root)
	joined := strings.Join(got, ",")
	if strings.Contains(joined, ".hidden") || strings.Contains(joined, "notes.txt") {
		t.Fatalf("expected hidden and regular files skipped, got %v", got)
	}
	if !strings.Contains(joined, "cmd") {
		t.Fatalf("expected cmd preserved, got %v", got)
	}
}
