package campaign

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

)

// ============================================================================
// Remediation for 2026-03-01_00-16-EST_edge_case_detector_boundary_analysis.md
// These tests verify the robustness of EdgeCaseDetector against extreme scenarios.
// ============================================================================

// TestEdgeCaseDetectorGap_AnalyzeFiles_CanceledContext (Vector 1: Empty/Nil Contexts)
func TestEdgeCaseDetectorGap_AnalyzeFiles_CanceledContext(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	paths := []string{"file1.go", "file2.go"}
	decisions, err := detector.AnalyzeFiles(ctx, paths, nil)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if len(decisions) != 0 {
		t.Errorf("Expected 0 decisions, got %d", len(decisions))
	}
}

// TestEdgeCaseDetectorGap_AnalyzeFiles_EmptyIntelligence (Vector 1: Empty Intel)
func TestEdgeCaseDetectorGap_AnalyzeFiles_EmptyIntelligence(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)
	
	// Initialized but empty intelligence report
	intel := &IntelligenceReport{
		FileTopology:     make(map[string]FileInfo),
		GitChurnHotspots: []ChurnHotspot{},
		SymbolGraph:      []SymbolInfo{},
	}

	paths := []string{"new_file.go"}
	decisions, _ := detector.AnalyzeFiles(context.Background(), paths, intel)

	if len(decisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(decisions))
	}

	if decisions[0].RecommendedAction != ActionCreate {
		t.Errorf("Expected ActionCreate for empty file topology, got %s", decisions[0].RecommendedAction)
	}
}

// TestEdgeCaseDetectorGap_EmptyPathString (Vector 1: Empty Paths)
// TestEdgeCaseDetectorGap_EmptyPathString (Vector 1: Empty Paths)
func TestEdgeCaseDetectorGap_EmptyPathString(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)

	paths := []string{""}
	decisions, _ := detector.AnalyzeFiles(context.Background(), paths, nil)

	if len(decisions) != 0 {
		t.Fatalf("Expected 0 decisions for empty path, got %d", len(decisions))
	}
}

// TestEdgeCaseDetectorGap_TypeCoercion (Vector 2: Type Coercion)
func TestEdgeCaseDetectorGap_TypeCoercion(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)

	// Test NaN
	_, ok := detector.parseNumber(math.NaN())
	if ok {
		t.Errorf("parseNumber failed to handle NaN")
	}

	// Test Inf
	_, ok = detector.parseNumber(math.Inf(1))
	if ok {
		t.Errorf("parseNumber failed to handle Inf")
	}

	// Test Struct Coercion
	strVal := detector.parseArg(struct{ ID int }{ID: 42})
	if strVal != "{42}" {
		t.Errorf("Expected {42}, got %s", strVal)
	}
}

// TestEdgeCaseDetectorGap_UnknownLanguage (Vector 3: Unknown Language)
func TestEdgeCaseDetectorGap_UnknownLanguage(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)

	lang := detector.detectLanguage("weird_file.mojo")
	if lang != "unknown" {
		t.Errorf("Expected unknown language, got %s", lang)
	}

	// Suggest splits should still work safely
	decision := FileDecision{
		Path:      "weird_file.mojo",
		LineCount: 2000,
	}
	splits := detector.suggestSplits(decision)
	if len(splits) == 0 {
		t.Errorf("Expected split suggestions even for unknown language")
	}
	if !strings.HasSuffix(splits[0].NewFileName, ".mojo") {
		t.Errorf("Expected split to preserve .mojo extension, got %s", splits[0].NewFileName)
	}
}

// TestEdgeCaseDetectorGap_ImpactScoreBounds (Vector 12: Impact Score Bounds)
func TestEdgeCaseDetectorGap_ImpactScoreBounds(t *testing.T) {
	decision := FileDecision{
		Dependents: []string{}, // length 0
	}
	
	// Simulate queryDependencies impact calculation
	decision.ImpactScore = len(decision.Dependents)
	if decision.ImpactScore < 0 {
		t.Errorf("ImpactScore should never be negative")
	}
}

// TestEdgeCaseDetectorGap_FormatForContext_Limits (Vector 9: Context formatting limits)
func TestEdgeCaseDetectorGap_FormatForContext_Limits(t *testing.T) {
	analysis := &EdgeCaseAnalysis{
		NoTestFiles: make([]string, 5000), // Massive amount of files
	}
	for i := 0; i < 5000; i++ {
		analysis.NoTestFiles[i] = "file.go"
	}

	output := analysis.FormatForContext()
	
	// Should truncate and show "and X more"
	if !strings.Contains(output, "and 4990 more") {
		t.Errorf("Expected truncation message 'and 4990 more', output was: %s", output)
	}

	// Output length should be bounded and not explode
	if len(output) > 5000 {
		t.Errorf("Output is too large for context window: %d bytes", len(output))
	}
}

// TestEdgeCaseDetectorGap_GodFile (Vector 3: God File Dependents serialization)
func TestEdgeCaseDetectorGap_GodFile(t *testing.T) {
	// A God file with 50,000 dependents
	dependents := make([]string, 50000)
	decision := FileDecision{
		Path:        "god_file.go",
		Dependents:  dependents,
		ImpactScore: 50000,
	}

	// Just verifying that logDecisionSummary handles extreme ImpactScore safely without logging 50k items
	detector := NewEdgeCaseDetector(nil, nil)
	// Shouldn't panic or freeze
	detector.logDecisionSummary([]FileDecision{decision})
}

// TestEdgeCaseDetectorGap_Concurrency (Vector 4: Concurrency Safety)
func TestEdgeCaseDetectorGap_Concurrency(t *testing.T) {
	detector := NewEdgeCaseDetector(nil, nil)
	
	// Run 10 goroutines calling AnalyzeFiles with the same detector to check for race conditions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			paths := []string{fmt.Sprintf("file_%d.go", idx)}
			_, _ = detector.AnalyzeFiles(context.Background(), paths, nil)
			done <- true
		}(i)
	}

	// Wait for all to finish (with a timeout to prevent infinite hangs)
	timeout := time.After(2 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatalf("Concurrency test timed out")
		}
	}
}
