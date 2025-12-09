package researcher

import (
	"testing"
)

func TestAnalyzeTopicCoverage_EmptyExisting(t *testing.T) {
	coverage := AnalyzeTopicCoverage("go concurrency", nil)

	if coverage.ShouldSkipAPI {
		t.Error("Expected ShouldSkipAPI=false for nil existing atoms")
	}
	if coverage.TotalAtoms != 0 {
		t.Errorf("Expected 0 atoms, got %d", coverage.TotalAtoms)
	}
	if coverage.RecommendedQuery != "go concurrency" {
		t.Errorf("Expected original topic as query, got %s", coverage.RecommendedQuery)
	}
}

func TestAnalyzeTopicCoverage_SufficientCoverage(t *testing.T) {
	// Create 25 atoms about Go concurrency
	atoms := make([]KnowledgeAtom, 25)
	for i := 0; i < 25; i++ {
		atoms[i] = KnowledgeAtom{
			Concept:    "goroutine",
			Content:    "Go concurrency with goroutines and channels allows parallel execution",
			Confidence: 0.9,
		}
	}

	existing := NewExistingKnowledge(atoms)
	coverage := AnalyzeTopicCoverage("go concurrency", existing)

	if !coverage.ShouldSkipAPI {
		t.Errorf("Expected ShouldSkipAPI=true for 25 relevant atoms, got quality=%.2f, atoms=%d",
			coverage.QualityScore, coverage.TotalAtoms)
	}
}

func TestAnalyzeTopicCoverage_InsufficientCoverage(t *testing.T) {
	// Create only 3 atoms - all matching "go concurrency" keywords
	atoms := []KnowledgeAtom{
		{Concept: "concurrency", Content: "Go concurrency with goroutines", Confidence: 0.8},
		{Concept: "concurrency", Content: "Go concurrency patterns", Confidence: 0.8},
		{Concept: "concurrency", Content: "Go concurrency best practices", Confidence: 0.8},
	}

	existing := NewExistingKnowledge(atoms)
	coverage := AnalyzeTopicCoverage("go concurrency", existing)

	if coverage.ShouldSkipAPI {
		t.Error("Expected ShouldSkipAPI=false for only 3 atoms")
	}
	if coverage.TotalAtoms != 3 {
		t.Errorf("Expected 3 relevant atoms, got %d", coverage.TotalAtoms)
	}
}

func TestAnalyzeTopicCoverage_IrrelevantAtoms(t *testing.T) {
	// Create atoms about unrelated topic
	atoms := []KnowledgeAtom{
		{Concept: "python", Content: "Python programming language", Confidence: 0.9},
		{Concept: "django", Content: "Django web framework", Confidence: 0.9},
		{Concept: "flask", Content: "Flask microframework", Confidence: 0.9},
	}

	existing := NewExistingKnowledge(atoms)
	coverage := AnalyzeTopicCoverage("go concurrency", existing)

	if coverage.ShouldSkipAPI {
		t.Error("Expected ShouldSkipAPI=false for irrelevant atoms")
	}
	// Irrelevant atoms shouldn't count
	if coverage.TotalAtoms > 0 {
		t.Errorf("Expected 0 relevant atoms for irrelevant content, got %d", coverage.TotalAtoms)
	}
}

func TestFilterTopicsWithCoverage(t *testing.T) {
	// Create atoms that cover "bubbletea" but not "lipgloss"
	atoms := make([]KnowledgeAtom, 0)
	for i := 0; i < 25; i++ {
		atoms = append(atoms, KnowledgeAtom{
			Concept:    "bubbletea",
			Content:    "Bubbletea is a TUI framework for Go with Model-Update-View pattern",
			Confidence: 0.9,
		})
	}

	existing := NewExistingKnowledge(atoms)
	topics := []string{"bubbletea", "lipgloss"}

	needResearch, skipped, _ := FilterTopicsWithCoverage(topics, existing)

	// Bubbletea should be skipped, lipgloss needs research
	if len(skipped) != 1 || skipped[0] != "bubbletea" {
		t.Errorf("Expected bubbletea to be skipped, got skipped=%v", skipped)
	}
	if len(needResearch) != 1 {
		t.Errorf("Expected 1 topic needing research, got %d: %v", len(needResearch), needResearch)
	}
}

func TestExtractKeywords(t *testing.T) {
	keywords := extractKeywords("go concurrency patterns and best practices")

	expected := map[string]bool{
		"concurrency": true,
		"patterns":    true,
		"best":        true,
		"practices":   true,
	}

	// Check some expected keywords are present
	for kw := range expected {
		if !keywords[kw] {
			t.Errorf("Expected keyword '%s' to be extracted", kw)
		}
	}

	// Stopwords should not be present
	stopwords := []string{"and", "the", "for"}
	for _, sw := range stopwords {
		if keywords[sw] {
			t.Errorf("Stopword '%s' should not be extracted", sw)
		}
	}
}

func TestBuildTargetedQuery(t *testing.T) {
	// Test with gaps
	query := buildTargetedQuery("bubbletea", []string{"keyboard", "events"})
	if query != "bubbletea keyboard events" {
		t.Errorf("Expected targeted query with gaps, got: %s", query)
	}

	// Test with no gaps
	query = buildTargetedQuery("bubbletea", nil)
	if query != "bubbletea" {
		t.Errorf("Expected original topic when no gaps, got: %s", query)
	}

	// Test with many gaps (should limit to 3)
	query = buildTargetedQuery("topic", []string{"a", "b", "c", "d", "e"})
	if query != "topic a b c" {
		t.Errorf("Expected at most 3 gaps, got: %s", query)
	}
}
