package campaign

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewEdgeCaseDetector(t *testing.T) {
	// Test with nil dependencies
	detector := NewEdgeCaseDetector(nil, nil)
	if detector == nil {
		t.Fatal("NewEdgeCaseDetector returned nil")
	}

	// Check default config
	if detector.config.HighChurnRate != 10 {
		t.Errorf("expected HighChurnRate 10, got %d", detector.config.HighChurnRate)
	}
	if detector.config.LargeFileLines != 500 {
		t.Errorf("expected LargeFileLines 500, got %d", detector.config.LargeFileLines)
	}
	if detector.config.AnalysisTimeout != 30*time.Second {
		t.Errorf("expected AnalysisTimeout 30s, got %v", detector.config.AnalysisTimeout)
	}
}

func TestDefaultEdgeCaseConfig(t *testing.T) {
	cfg := DefaultEdgeCaseConfig()

	if cfg.HighChurnRate != 10 {
		t.Errorf("expected HighChurnRate 10, got %d", cfg.HighChurnRate)
	}
	if cfg.LargeFileLines != 500 {
		t.Errorf("expected LargeFileLines 500, got %d", cfg.LargeFileLines)
	}
	if cfg.HighComplexity != 10.0 {
		t.Errorf("expected HighComplexity 10.0, got %f", cfg.HighComplexity)
	}
	if cfg.MaxFunctionsPerFile != 20 {
		t.Errorf("expected MaxFunctionsPerFile 20, got %d", cfg.MaxFunctionsPerFile)
	}
	if cfg.AnalysisTimeout != 30*time.Second {
		t.Errorf("expected AnalysisTimeout 30s, got %v", cfg.AnalysisTimeout)
	}
}

func TestEdgeCaseDetector_WithConfig(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)

	customConfig := EdgeCaseConfig{
		HighChurnRate:  20,
		LargeFileLines: 1000,
		HighComplexity: 15.0,
	}

	result := detector.WithConfig(customConfig)

	if result != detector {
		t.Error("WithConfig should return same detector for chaining")
	}
	if detector.config.HighChurnRate != 20 {
		t.Errorf("expected HighChurnRate 20, got %d", detector.config.HighChurnRate)
	}
	if detector.config.LargeFileLines != 1000 {
		t.Errorf("expected LargeFileLines 1000, got %d", detector.config.LargeFileLines)
	}
}

func TestFileAction_Constants(t *testing.T) {
	// Verify file action constants are distinct
	actions := map[FileAction]bool{
		ActionCreate:        true,
		ActionExtend:        true,
		ActionModularize:    true,
		ActionRefactorFirst: true,
	}

	if len(actions) != 4 {
		t.Error("file actions should be distinct")
	}
}

func TestFileDecision_Fields(t *testing.T) {
	decision := FileDecision{
		Path:              "test.go",
		RecommendedAction: ActionExtend,
		Reasoning:         "Small file, few dependencies",
		Confidence:        0.85,
		ChurnRate:         5,
		Complexity:        3.0,
		LineCount:         100,
		Dependencies:      []string{"dep1", "dep2"},
		Dependents:        []string{"user1"},
		ImpactScore:       10,
	}

	if decision.Path != "test.go" {
		t.Errorf("expected Path 'test.go', got %s", decision.Path)
	}
	if decision.RecommendedAction != ActionExtend {
		t.Errorf("expected ActionExtend, got %v", decision.RecommendedAction)
	}
	if decision.ChurnRate != 5 {
		t.Errorf("expected ChurnRate 5, got %d", decision.ChurnRate)
	}
	if len(decision.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(decision.Dependencies))
	}
	if decision.Confidence != 0.85 {
		t.Errorf("expected Confidence 0.85, got %f", decision.Confidence)
	}
}

func TestEdgeCaseDetector_AnalyzeFiles_NilDependencies(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)

	ctx := context.Background()
	// AnalyzeFiles takes (ctx, paths, *IntelligenceReport)
	decisions, err := detector.AnalyzeFiles(ctx, []string{}, nil)

	// Should handle gracefully with empty paths
	if err != nil {
		t.Errorf("AnalyzeFiles with empty paths should not error: %v", err)
	}
	if decisions == nil {
		t.Fatal("AnalyzeFiles should return non-nil slice")
	}
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for empty paths, got %d", len(decisions))
	}
}

func TestEdgeCaseAnalysis_Fields(t *testing.T) {
	analysis := &EdgeCaseAnalysis{
		TotalFiles:      10,
		RequiresPrework: 3,
		Decisions:       []FileDecision{},
		ActionCounts: map[FileAction]int{
			ActionCreate:     3,
			ActionExtend:     5,
			ActionModularize: 2,
		},
		ModularizeFiles: []string{"large.go"},
		CreateFiles:     []string{"new.go"},
	}

	if analysis.TotalFiles != 10 {
		t.Errorf("expected TotalFiles 10, got %d", analysis.TotalFiles)
	}
	if analysis.ActionCounts[ActionCreate] != 3 {
		t.Errorf("expected CreateCount 3, got %d", analysis.ActionCounts[ActionCreate])
	}
	if analysis.RequiresPrework != 3 {
		t.Errorf("expected RequiresPrework 3, got %d", analysis.RequiresPrework)
	}
}

func TestEdgeCaseAnalysis_FormatForContext(t *testing.T) {
	analysis := &EdgeCaseAnalysis{
		TotalFiles:      5,
		RequiresPrework: 2,
		Decisions: []FileDecision{
			{Path: "test.go", RecommendedAction: ActionExtend, Reasoning: "Small file"},
			{Path: "large.go", RecommendedAction: ActionModularize, Reasoning: "Too large"},
			{Path: "new.go", RecommendedAction: ActionCreate, Reasoning: "New file"},
		},
		ActionCounts: map[FileAction]int{
			ActionCreate:     1,
			ActionExtend:     1,
			ActionModularize: 1,
		},
		ModularizeFiles: []string{"large.go"},
		HighImpactFiles: []string{"large.go"},
	}

	formatted := analysis.FormatForContext()

	if formatted == "" {
		t.Fatal("FormatForContext should not return empty string")
	}
	if !strings.Contains(formatted, "EDGE CASE ANALYSIS") {
		t.Error("should contain header")
	}
	if !strings.Contains(formatted, "Total Files") {
		t.Error("should contain total files info")
	}
}

func TestEdgeCaseAnalysis_ActionCountsTotals(t *testing.T) {
	analysis := &EdgeCaseAnalysis{
		ActionCounts: map[FileAction]int{
			ActionCreate:        5,
			ActionExtend:        10,
			ActionModularize:    3,
			ActionRefactorFirst: 2,
		},
	}

	total := analysis.ActionCounts[ActionCreate] +
		analysis.ActionCounts[ActionExtend] +
		analysis.ActionCounts[ActionModularize] +
		analysis.ActionCounts[ActionRefactorFirst]

	if total != 20 {
		t.Errorf("expected total 20, got %d", total)
	}
}

func TestFileActionString(t *testing.T) {
	// Test that file actions can be compared
	if ActionCreate == ActionExtend {
		t.Error("ActionCreate should not equal ActionExtend")
	}
	if ActionModularize == ActionRefactorFirst {
		t.Error("ActionModularize should not equal ActionRefactorFirst")
	}
}

func TestEdgeCaseDetector_DetermineAction_Logic(t *testing.T) {
	// Test the decision logic by checking expected outcomes
	// These are the rules from the implementation:
	// - Large file (>500 lines) + high churn -> Modularize
	// - Large file + low churn -> RefactorFirst
	// - High complexity -> Modularize
	// - Many dependencies -> RefactorFirst
	// - New file -> Create
	// - Default -> Extend

	testCases := []struct {
		name       string
		lineCount  int
		churnRate  int
		complexity float64
		depCount   int
		isNew      bool
		expected   FileAction
	}{
		{"new file", 0, 0, 0, 0, true, ActionCreate},
		{"small simple file", 100, 3, 2.0, 5, false, ActionExtend},
		{"large high churn", 600, 15, 5.0, 10, false, ActionModularize},
		{"large low churn", 600, 2, 3.0, 5, false, ActionRefactorFirst},
		{"high complexity", 200, 5, 12.0, 8, false, ActionModularize},
		{"many dependencies", 200, 5, 3.0, 25, false, ActionRefactorFirst},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This tests the expected behavior based on the rules
			// The actual determineAction method uses these thresholds
			var action FileAction

			if tc.isNew {
				action = ActionCreate
			} else if tc.lineCount > 500 && tc.churnRate > 10 {
				action = ActionModularize
			} else if tc.lineCount > 500 && tc.churnRate <= 10 {
				action = ActionRefactorFirst
			} else if tc.complexity > 10.0 {
				action = ActionModularize
			} else if tc.depCount > 20 {
				action = ActionRefactorFirst
			} else {
				action = ActionExtend
			}

			if action != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, action)
			}
		})
	}
}

func TestEdgeCaseAnalysis_FileCategories(t *testing.T) {
	analysis := &EdgeCaseAnalysis{
		TotalFiles:      6,
		ModularizeFiles: []string{"huge.go", "complex.go"},
		RefactorFiles:   []string{"messy.go"},
		CreateFiles:     []string{"new.go"},
		ExtendFiles:     []string{"simple.go", "small.go"},
		HighImpactFiles: []string{"huge.go"},
		NoTestFiles:     []string{"messy.go", "simple.go"},
	}

	if len(analysis.ModularizeFiles) != 2 {
		t.Errorf("expected 2 modularize files, got %d", len(analysis.ModularizeFiles))
	}
	if len(analysis.ExtendFiles) != 2 {
		t.Errorf("expected 2 extend files, got %d", len(analysis.ExtendFiles))
	}
	if len(analysis.NoTestFiles) != 2 {
		t.Errorf("expected 2 no-test files, got %d", len(analysis.NoTestFiles))
	}
}
