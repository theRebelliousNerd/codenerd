package reviewer

import (
	"strings"
	"testing"
)

func TestFormatSpecialistReviewTask_Basic(t *testing.T) {
	task := SpecialistReviewTask{
		AgentName:   "GoExpert",
		Files:       []string{"main.go", "handler.go"},
		DomainFocus: "Go language patterns and idioms",
	}

	result := FormatSpecialistReviewTask(task)

	// Should contain key sections
	expectedParts := []string{
		"SPECIALIST DOMAIN REVIEW",
		"GoExpert domain expert",
		"## Files to Review",
		"main.go",
		"handler.go",
		"## Domain Focus: Go language patterns and idioms",
		"## Your Mission",
		"Domain-Specific Issues",
		"## Output Format",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected to find '%s' in output:\n%s", part, result)
		}
	}
}

func TestFormatSpecialistReviewTask_WithKnowledge(t *testing.T) {
	task := SpecialistReviewTask{
		AgentName: "MangleExpert",
		Files:     []string{"policy.mg"},
		Knowledge: []RetrievedKnowledge{
			{
				Content:    "All variables must be bound in negation",
				Concept:    "best_practice",
				Confidence: 0.9,
			},
		},
		DomainFocus: "Mangle/Datalog logic programming",
	}

	result := FormatSpecialistReviewTask(task)

	// Should contain knowledge section
	if !strings.Contains(result, "## Your Knowledge Base") {
		t.Error("Expected knowledge base section in output")
	}
	if !strings.Contains(result, "All variables must be bound in negation") {
		t.Error("Expected knowledge content in output")
	}
}

func TestFormatSpecialistReviewTask_WithContextHints(t *testing.T) {
	task := SpecialistReviewTask{
		AgentName:    "BubbleTeaExpert",
		Files:        []string{"model.go"},
		ContextHints: []string{"File type: .go", "Contains tea.Model interface"},
	}

	result := FormatSpecialistReviewTask(task)

	if !strings.Contains(result, "## Additional Context") {
		t.Error("Expected additional context section in output")
	}
	if !strings.Contains(result, "File type: .go") {
		t.Error("Expected context hint in output")
	}
}

func TestBuildSpecialistTask(t *testing.T) {
	match := SpecialistMatch{
		AgentName:     "GoExpert",
		KnowledgePath: "/path/to/kb",
		Files:         []string{"main.go"},
		Score:         0.8,
		Reason:        "Go language patterns",
	}

	allFiles := []string{"main.go", "handler.go", "util.go"}
	knowledge := []RetrievedKnowledge{
		{Content: "Test knowledge", Concept: "pattern"},
	}

	task := BuildSpecialistTask(match, allFiles, knowledge)

	if task.AgentName != "GoExpert" {
		t.Errorf("Expected AgentName 'GoExpert', got '%s'", task.AgentName)
	}
	if task.DomainFocus != "Go language patterns" {
		t.Errorf("Expected DomainFocus 'Go language patterns', got '%s'", task.DomainFocus)
	}
	if len(task.Files) != 1 {
		t.Errorf("Expected 1 file (from match), got %d", len(task.Files))
	}
	if len(task.Knowledge) != 1 {
		t.Errorf("Expected 1 knowledge item, got %d", len(task.Knowledge))
	}
}

func TestBuildSpecialistTask_UsesAllFilesWhenMatchHasNone(t *testing.T) {
	match := SpecialistMatch{
		AgentName: "GoExpert",
		Files:     []string{}, // Empty
		Reason:    "Go patterns",
	}

	allFiles := []string{"main.go", "handler.go"}
	task := BuildSpecialistTask(match, allFiles, nil)

	if len(task.Files) != 2 {
		t.Errorf("Expected 2 files (from allFiles), got %d", len(task.Files))
	}
}

func TestFormatMultiShardReviewHeader_Complete(t *testing.T) {
	result := FormatMultiShardReviewHeader(
		"internal/core/",
		[]string{"Reviewer", "GoExpert", "MangleExpert"},
		true,
	)

	expectedParts := []string{
		"# Multi-Shard Code Review: internal/core/",
		"**Status**: Complete",
		"**Participants**: Reviewer, GoExpert, MangleExpert",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected to find '%s' in output:\n%s", part, result)
		}
	}
}

func TestFormatMultiShardReviewHeader_Incomplete(t *testing.T) {
	result := FormatMultiShardReviewHeader(
		"main.go",
		[]string{"Reviewer"},
		false,
	)

	if !strings.Contains(result, "**Status**: Partial (some specialists failed)") {
		t.Error("Expected incomplete status in output")
	}
}

func TestFormatShardSection_WithFindings(t *testing.T) {
	findings := []ParsedFinding{
		{
			File:           "main.go",
			Line:           10,
			Severity:       "error",
			Message:        "Missing error handling",
			Recommendation: "Add error check",
		},
		{
			File:     "main.go",
			Line:     20,
			Severity: "warning",
			Message:  "Unused variable",
		},
	}

	result := FormatShardSection("GoExpert", findings)

	expectedParts := []string{
		"## GoExpert (2 findings)",
		"**main.go:10** [ERROR] Missing error handling",
		"_Recommendation_: Add error check",
		"**main.go:20** [WARNING] Unused variable",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected to find '%s' in output:\n%s", part, result)
		}
	}
}

func TestFormatShardSection_NoFindings(t *testing.T) {
	result := FormatShardSection("TestShard", []ParsedFinding{})

	if !strings.Contains(result, "## TestShard (0 findings)") {
		t.Error("Expected 0 findings header")
	}
	if !strings.Contains(result, "_No issues found._") {
		t.Error("Expected 'No issues found' message")
	}
}

func TestFormatShardSection_GroupsBySeverity(t *testing.T) {
	findings := []ParsedFinding{
		{File: "a.go", Line: 1, Severity: "info", Message: "Info message"},
		{File: "a.go", Line: 2, Severity: "critical", Message: "Critical message"},
		{File: "a.go", Line: 3, Severity: "warning", Message: "Warning message"},
	}

	result := FormatShardSection("Test", findings)

	// Critical should appear before warning and info
	criticalIdx := strings.Index(result, "Critical message")
	warningIdx := strings.Index(result, "Warning message")
	infoIdx := strings.Index(result, "Info message")

	if criticalIdx > warningIdx || warningIdx > infoIdx {
		t.Error("Expected findings to be ordered by severity (critical > warning > info)")
	}
}
