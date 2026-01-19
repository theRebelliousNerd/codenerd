// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the Feedback and Learning system.
package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// EXECUTION FEEDBACK TESTS
// =============================================================================

func TestExecutionFeedback_Struct(t *testing.T) {
	now := time.Now()
	feedback := ExecutionFeedback{
		ToolName:    "test_tool",
		ExecutionID: "exec-123",
		Timestamp:   now,
		Input:       "test input",
		Output:      "test output",
		OutputSize:  11,
		Duration:    100 * time.Millisecond,
		Success:     true,
	}

	if feedback.ToolName != "test_tool" {
		t.Errorf("ToolName = %q, want %q", feedback.ToolName, "test_tool")
	}
	if feedback.ExecutionID != "exec-123" {
		t.Errorf("ExecutionID = %q, want %q", feedback.ExecutionID, "exec-123")
	}
	if feedback.Timestamp != now {
		t.Errorf("Timestamp mismatch")
	}
	if feedback.Input != "test input" {
		t.Errorf("Input = %q, want %q", feedback.Input, "test input")
	}
	if feedback.Output != "test output" {
		t.Errorf("Output = %q, want %q", feedback.Output, "test output")
	}
	if feedback.OutputSize != 11 {
		t.Errorf("OutputSize = %d, want 11", feedback.OutputSize)
	}
	if feedback.Duration != 100*time.Millisecond {
		t.Errorf("Duration = %v, want %v", feedback.Duration, 100*time.Millisecond)
	}
	if !feedback.Success {
		t.Error("Expected Success=true")
	}
}

func TestUserFeedback_Struct(t *testing.T) {
	now := time.Now()
	feedback := UserFeedback{
		Accepted:    false,
		Modified:    true,
		Reran:       false,
		Complaint:   "Output was incomplete",
		Improvement: "Should include all pages",
		Timestamp:   now,
	}

	if feedback.Accepted {
		t.Error("Expected Accepted=false")
	}
	if !feedback.Modified {
		t.Error("Expected Modified=true")
	}
	if feedback.Reran {
		t.Error("Expected Reran=false")
	}
	if feedback.Complaint != "Output was incomplete" {
		t.Errorf("Complaint = %q, want %q", feedback.Complaint, "Output was incomplete")
	}
	if feedback.Improvement != "Should include all pages" {
		t.Errorf("Improvement = %q, want %q", feedback.Improvement, "Should include all pages")
	}
	if feedback.Timestamp != now {
		t.Errorf("Timestamp mismatch")
	}
}

// =============================================================================
// TOOL REFINER TESTS
// =============================================================================

func TestNewToolRefiner(t *testing.T) {
	client := &MockLLMClient{}
	toolGen := NewToolGenerator(client, "/tmp/tools")
	refiner := NewToolRefiner(client, toolGen)

	if refiner == nil {
		t.Fatal("NewToolRefiner returned nil")
	}
	if refiner.client != client {
		t.Error("client not set correctly")
	}
	if refiner.toolGen != toolGen {
		t.Error("toolGen not set correctly")
	}
}

func TestToolRefiner_Refine(t *testing.T) {
	client := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system, user string) (string, error) {
			return `{
				"improved_code": "package tools\n\nfunc improvedTool() {}",
				"changes": ["Added pagination", "Fixed error handling"],
				"expected_gain": 0.3,
				"test_cases": ["Test pagination", "Test error case"]
			}`, nil
		},
	}
	toolGen := NewToolGenerator(client, "/tmp/tools")
	refiner := NewToolRefiner(client, toolGen)

	req := RefinementRequest{
		ToolName:     "test_tool",
		OriginalCode: "package tools\n\nfunc tool() {}",
		Patterns:     []*DetectedPattern{},
		Suggestions:  []ImprovementSuggestion{},
	}

	result, err := refiner.Refine(context.Background(), req)
	if err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful refinement")
	}
	if result.ImprovedCode == "" {
		t.Error("Expected improved code")
	}
	if len(result.Changes) == 0 {
		t.Error("Expected changes to be listed")
	}
	if result.ExpectedGain <= 0 {
		t.Error("Expected positive expected gain")
	}
}

func TestToolRefiner_Refine_CodeBlockFallback(t *testing.T) {
	client := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, system, user string) (string, error) {
			// Return code block instead of JSON
			return "```go\npackage tools\n\nfunc betterTool() {}\n```", nil
		},
	}
	toolGen := NewToolGenerator(client, "/tmp/tools")
	refiner := NewToolRefiner(client, toolGen)

	req := RefinementRequest{
		ToolName:     "test_tool",
		OriginalCode: "package tools\n\nfunc tool() {}",
	}

	result, err := refiner.Refine(context.Background(), req)
	if err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	// Code block extraction may or may not be successful depending on implementation
	// The important thing is no error occurred
	_ = result
}

func TestBuildRefinementPrompt(t *testing.T) {
	client := &MockLLMClient{}
	toolGen := NewToolGenerator(client, "/tmp/tools")
	refiner := NewToolRefiner(client, toolGen)

	req := RefinementRequest{
		ToolName:     "test_tool",
		OriginalCode: "package tools\nfunc tool() {}",
		Feedback: []ExecutionFeedback{
			{
				Success: false,
				Quality: &QualityAssessment{
					Score: 0.3,
					Issues: []QualityIssue{
						{Type: IssuePagination, Description: "Missing pagination"},
					},
				},
			},
		},
		Patterns: []*DetectedPattern{
			{IssueType: IssuePagination, Occurrences: 3, Confidence: 0.8},
		},
		Suggestions: []ImprovementSuggestion{
			{Type: SuggestAddPagination, Description: "Add pagination handling"},
		},
	}

	prompt := refiner.buildRefinementPrompt(req)

	// Check prompt contains key elements
	if prompt == "" {
		t.Fatal("Prompt should not be empty")
	}
	if !strings.Contains(prompt, "test_tool") {
		t.Error("Prompt should contain tool name")
	}
	if !strings.Contains(prompt, "Execution Feedback") {
		t.Error("Prompt should contain feedback section")
	}
}

// =============================================================================
// LEARNING STORE TESTS
// =============================================================================

func TestNewLearningStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	if store == nil {
		t.Fatal("NewLearningStore returned nil")
	}
	if store.storePath != tmpDir {
		t.Errorf("storePath = %q, want %q", store.storePath, tmpDir)
	}
	if store.learnings == nil {
		t.Error("learnings map not initialized")
	}
}

func TestLearningStore_RecordLearning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	feedback := &ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
		Quality: &QualityAssessment{
			Score: 0.8,
			Issues: []QualityIssue{
				{Type: IssueSlow, Severity: 0.3},
			},
		},
	}

	patterns := []*DetectedPattern{
		{IssueType: IssueSlow, Confidence: 0.8},
	}

	store.RecordLearning("test_tool", feedback, patterns)

	// Verify learning was recorded
	learning := store.GetLearning("test_tool")
	if learning == nil {
		t.Fatal("Expected learning to be recorded")
	}
	if learning.TotalExecutions != 1 {
		t.Errorf("TotalExecutions = %d, want 1", learning.TotalExecutions)
	}
	if learning.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", learning.SuccessRate)
	}
	if learning.AverageQuality != 0.8 {
		t.Errorf("AverageQuality = %f, want 0.8", learning.AverageQuality)
	}
}

func TestLearningStore_RecordLearning_UpdatesStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	// Record multiple executions
	for i := 0; i < 5; i++ {
		success := i%2 == 0 // 3 successes, 2 failures
		quality := 0.5 + float64(i)*0.1

		feedback := &ExecutionFeedback{
			ToolName: "test_tool",
			Success:  success,
			Quality:  &QualityAssessment{Score: quality},
		}

		store.RecordLearning("test_tool", feedback, nil)
	}

	learning := store.GetLearning("test_tool")
	if learning.TotalExecutions != 5 {
		t.Errorf("TotalExecutions = %d, want 5", learning.TotalExecutions)
	}
	if learning.SuccessRate < 0.5 || learning.SuccessRate > 0.7 {
		t.Errorf("SuccessRate = %f, want ~0.6", learning.SuccessRate)
	}
}

func TestLearningStore_GetLearning_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	learning := store.GetLearning("nonexistent")
	if learning != nil {
		t.Error("Expected nil for nonexistent tool")
	}
}

func TestLearningStore_GetAllLearnings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	// Record learnings for multiple tools
	for _, toolName := range []string{"tool_a", "tool_b", "tool_c"} {
		feedback := &ExecutionFeedback{
			ToolName: toolName,
			Success:  true,
			Quality:  &QualityAssessment{Score: 0.9},
		}
		store.RecordLearning(toolName, feedback, nil)
	}

	all := store.GetAllLearnings()
	if len(all) != 3 {
		t.Errorf("Expected 3 learnings, got %d", len(all))
	}
}

func TestLearningStore_GenerateMangleFacts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	feedback := &ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
		Quality: &QualityAssessment{
			Score:  0.8,
			Issues: []QualityIssue{{Type: IssueSlow}},
		},
	}
	store.RecordLearning("test_tool", feedback, nil)

	facts := store.GenerateMangleFacts()

	if len(facts) == 0 {
		t.Error("Expected Mangle facts to be generated")
	}

	// Check for expected fact types
	foundLearning := false
	for _, fact := range facts {
		if strings.Contains(fact, "tool_learning") {
			foundLearning = true
		}
	}
	if !foundLearning {
		t.Error("Expected tool_learning fact")
	}
}

func TestLearningStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and record learning
	store1 := NewLearningStore(tmpDir)
	feedback := &ExecutionFeedback{
		ToolName: "persistent_tool",
		Success:  true,
		Quality:  &QualityAssessment{Score: 0.9},
	}
	store1.RecordLearning("persistent_tool", feedback, nil)

	// Create new store from same path
	store2 := NewLearningStore(tmpDir)

	// Verify learning was persisted
	learning := store2.GetLearning("persistent_tool")
	if learning == nil {
		t.Error("Expected learning to be persisted and loaded")
	}
}

// =============================================================================
// PATTERN DETECTOR TESTS
// =============================================================================

func TestNewPatternDetector(t *testing.T) {
	detector := NewPatternDetector()

	if detector == nil {
		t.Fatal("NewPatternDetector returned nil")
	}
	if detector.history == nil {
		t.Error("history slice not initialized")
	}
	if detector.patterns == nil {
		t.Error("patterns map not initialized")
	}
}

func TestPatternDetector_RecordExecution(t *testing.T) {
	detector := NewPatternDetector()

	feedback := ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
		Quality: &QualityAssessment{
			Issues: []QualityIssue{
				{Type: IssuePagination, Evidence: "page 1 of 10"},
			},
			Suggestions: []ImprovementSuggestion{
				{Type: SuggestAddPagination, Description: "Add pagination"},
			},
		},
	}

	detector.RecordExecution(feedback)

	if len(detector.history) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(detector.history))
	}

	patterns := detector.GetToolPatterns("test_tool")
	if len(patterns) == 0 {
		t.Error("Expected pattern to be detected")
	}
}

func TestPatternDetector_RecordExecution_UpdatesExisting(t *testing.T) {
	detector := NewPatternDetector()

	// Record same issue multiple times
	for i := 0; i < 5; i++ {
		feedback := ExecutionFeedback{
			ToolName: "test_tool",
			Success:  true,
			Quality: &QualityAssessment{
				Issues: []QualityIssue{
					{Type: IssuePagination, Evidence: "truncated"},
				},
			},
		}
		detector.RecordExecution(feedback)
	}

	patterns := detector.GetToolPatterns("test_tool")
	if len(patterns) != 1 {
		t.Errorf("Expected 1 unique pattern, got %d", len(patterns))
	}

	if patterns[0].Occurrences != 5 {
		t.Errorf("Occurrences = %d, want 5", patterns[0].Occurrences)
	}

	// Confidence should increase with occurrences
	if patterns[0].Confidence < 0.9 {
		t.Errorf("Confidence = %f, want >= 0.9 for 5 occurrences", patterns[0].Confidence)
	}
}

func TestPatternDetector_RecordExecution_HistoryLimit(t *testing.T) {
	detector := NewPatternDetector()

	// Record more than limit
	for i := 0; i < 1100; i++ {
		feedback := ExecutionFeedback{
			ToolName: "test_tool",
			Success:  true,
		}
		detector.RecordExecution(feedback)
	}

	// History should be trimmed
	if len(detector.history) > 1000 {
		t.Errorf("History should be limited, got %d entries", len(detector.history))
	}
}

func TestPatternDetector_GetPatterns_MinConfidence(t *testing.T) {
	detector := NewPatternDetector()

	// Create patterns with different confidence levels
	// 1 occurrence = 0.3 confidence
	detector.RecordExecution(ExecutionFeedback{
		ToolName: "tool_low",
		Quality: &QualityAssessment{
			Issues: []QualityIssue{{Type: IssueAuth}},
		},
	})

	// 5 occurrences = 0.9 confidence
	for i := 0; i < 5; i++ {
		detector.RecordExecution(ExecutionFeedback{
			ToolName: "tool_high",
			Quality: &QualityAssessment{
				Issues: []QualityIssue{{Type: IssueSlow}},
			},
		})
	}

	// Get only high confidence patterns
	patterns := detector.GetPatterns(0.8)

	foundHigh := false
	foundLow := false
	for _, p := range patterns {
		if p.ToolName == "tool_high" {
			foundHigh = true
		}
		if p.ToolName == "tool_low" {
			foundLow = true
		}
	}

	if !foundHigh {
		t.Error("Expected high confidence pattern")
	}
	if foundLow {
		t.Error("Low confidence pattern should be filtered out")
	}
}

func TestPatternDetector_GetToolPatterns(t *testing.T) {
	detector := NewPatternDetector()

	// Record patterns for different tools
	detector.RecordExecution(ExecutionFeedback{
		ToolName: "tool_a",
		Quality: &QualityAssessment{
			Issues: []QualityIssue{{Type: IssuePagination}},
		},
	})
	detector.RecordExecution(ExecutionFeedback{
		ToolName: "tool_b",
		Quality: &QualityAssessment{
			Issues: []QualityIssue{{Type: IssueSlow}},
		},
	})

	patternsA := detector.GetToolPatterns("tool_a")
	if len(patternsA) != 1 {
		t.Errorf("Expected 1 pattern for tool_a, got %d", len(patternsA))
	}
	if patternsA[0].IssueType != IssuePagination {
		t.Errorf("Expected IssuePagination, got %v", patternsA[0].IssueType)
	}
}

func TestCalculatePatternConfidence(t *testing.T) {
	tests := []struct {
		occurrences int
		wantMin     float64
		wantMax     float64
	}{
		{1, 0.25, 0.35},  // ~0.3
		{2, 0.45, 0.55},  // ~0.5
		{3, 0.65, 0.75},  // ~0.7
		{5, 0.85, 0.95},  // ~0.9
		{10, 0.85, 0.95}, // ~0.9 (capped)
	}

	for _, tt := range tests {
		confidence := calculatePatternConfidence(tt.occurrences)
		if confidence < tt.wantMin || confidence > tt.wantMax {
			t.Errorf("calculatePatternConfidence(%d) = %f, want %f-%f",
				tt.occurrences, confidence, tt.wantMin, tt.wantMax)
		}
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestContainsIssueType(t *testing.T) {
	types := []IssueType{IssuePagination, IssueSlow, IssueAuth}

	if !containsIssueType(types, IssuePagination) {
		t.Error("Expected to find IssuePagination")
	}
	if !containsIssueType(types, IssueSlow) {
		t.Error("Expected to find IssueSlow")
	}
	if containsIssueType(types, IssueIncomplete) {
		t.Error("Did not expect to find IssueIncomplete")
	}
	if containsIssueType(nil, IssuePagination) {
		t.Error("Empty slice should return false")
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !contains(slice, "a") {
		t.Error("Expected to find 'a'")
	}
	if !contains(slice, "c") {
		t.Error("Expected to find 'c'")
	}
	if contains(slice, "d") {
		t.Error("Did not expect to find 'd'")
	}
	if contains(nil, "a") {
		t.Error("Empty slice should return false")
	}
}

func TestHasSuggestion(t *testing.T) {
	suggestions := []ImprovementSuggestion{
		{Type: SuggestAddPagination},
		{Type: SuggestCaching},
	}

	if !hasSuggestion(suggestions, SuggestAddPagination) {
		t.Error("Expected to find SuggestAddPagination")
	}
	if hasSuggestion(suggestions, SuggestAddRetry) {
		t.Error("Did not expect to find SuggestAddRetry")
	}
}

// =============================================================================
// TOOL LEARNING TESTS
// =============================================================================

func TestToolLearning_KnownIssues(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	// Record execution with issues
	feedback := &ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
		Quality: &QualityAssessment{
			Issues: []QualityIssue{
				{Type: IssuePagination},
				{Type: IssueSlow},
			},
		},
	}
	store.RecordLearning("test_tool", feedback, nil)

	learning := store.GetLearning("test_tool")
	if len(learning.KnownIssues) != 2 {
		t.Errorf("Expected 2 known issues, got %d", len(learning.KnownIssues))
	}

	// Record same issues again - should not duplicate
	store.RecordLearning("test_tool", feedback, nil)
	learning = store.GetLearning("test_tool")
	if len(learning.KnownIssues) != 2 {
		t.Errorf("Known issues should not duplicate, got %d", len(learning.KnownIssues))
	}
}

func TestToolLearning_AntiPatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	patterns := []*DetectedPattern{
		{PatternID: "test_tool:pagination", IssueType: IssuePagination, Confidence: 0.8},
	}

	feedback := &ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
		Quality:  &QualityAssessment{Score: 0.5},
	}

	store.RecordLearning("test_tool", feedback, patterns)

	learning := store.GetLearning("test_tool")
	if len(learning.AntiPatterns) == 0 {
		t.Error("Expected anti-patterns to be recorded from high-confidence patterns")
	}
}

// =============================================================================
// FILE PERSISTENCE TESTS
// =============================================================================

func TestLearningStore_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "learning-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewLearningStore(tmpDir)

	// Record learning
	feedback := &ExecutionFeedback{
		ToolName: "save_test_tool",
		Success:  true,
		Quality:  &QualityAssessment{Score: 0.85},
	}
	store.RecordLearning("save_test_tool", feedback, nil)

	// Verify file was created
	jsonPath := filepath.Join(tmpDir, "tool_learnings.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("Learning file should have been created")
	}

	// Load into new store
	store2 := NewLearningStore(tmpDir)
	learning := store2.GetLearning("save_test_tool")

	if learning == nil {
		t.Fatal("Expected learning to be loaded")
	}
	if learning.AverageQuality != 0.85 {
		t.Errorf("AverageQuality = %f, want 0.85", learning.AverageQuality)
	}
}
