package chat

import (
	"codenerd/internal/shards/reviewer"
	"strings"
	"testing"
	"time"
)

func TestDeduplicateFindings_EmptyInput(t *testing.T) {
	findingsByShard := make(map[string][]reviewer.ParsedFinding)
	result := deduplicateFindings(findingsByShard)
	if len(result) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(result))
	}
}

func TestDeduplicateFindings_NoDuplicates(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {
			{File: "a.go", Line: 10, Severity: "error", Message: "Error 1"},
			{File: "a.go", Line: 20, Severity: "warning", Message: "Warning 1"},
		},
		"shard2": {
			{File: "b.go", Line: 15, Severity: "info", Message: "Info 1"},
		},
	}

	result := deduplicateFindings(findingsByShard)
	if len(result) != 3 {
		t.Errorf("Expected 3 findings, got %d", len(result))
	}
}

func TestDeduplicateFindings_WithDuplicates_KeepsHigherSeverity(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {
			{File: "a.go", Line: 10, Severity: "warning", Message: "Warning from shard1"},
		},
		"shard2": {
			{File: "a.go", Line: 10, Severity: "error", Message: "Error from shard2"}, // Same location, higher severity
		},
	}

	result := deduplicateFindings(findingsByShard)
	if len(result) != 1 {
		t.Fatalf("Expected 1 finding after dedup, got %d", len(result))
	}

	// Should keep the error (higher severity)
	if result[0].Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", result[0].Severity)
	}
}

func TestDeduplicateFindings_Critical_BeatsAll(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {
			{File: "a.go", Line: 10, Severity: "error", Message: "Error"},
		},
		"shard2": {
			{File: "a.go", Line: 10, Severity: "critical", Message: "Critical"},
		},
		"shard3": {
			{File: "a.go", Line: 10, Severity: "warning", Message: "Warning"},
		},
	}

	result := deduplicateFindings(findingsByShard)
	if len(result) != 1 {
		t.Fatalf("Expected 1 finding after dedup, got %d", len(result))
	}

	if result[0].Severity != "critical" {
		t.Errorf("Expected severity 'critical', got '%s'", result[0].Severity)
	}
}

func TestGenerateHolisticSummary_Basic(t *testing.T) {
	agg := &AggregatedReview{
		Target:       "internal/core/",
		Participants: []string{"Reviewer", "GoExpert"},
		Files:        []string{"kernel.go", "store.go"},
		Duration:     5 * time.Second,
		FindingsByShard: map[string][]reviewer.ParsedFinding{
			"Reviewer": {
				{Severity: "error", Message: "Test error"},
				{Severity: "warning", Message: "Test warning"},
			},
			"GoExpert": {
				{Severity: "info", Message: "Test info"},
			},
		},
	}

	summary := generateHolisticSummary(agg)

	// Check for key elements
	expectedParts := []string{
		"Multi-shard review of internal/core/ completed",
		"Error: 1",
		"Warning: 1",
		"Info: 1",
		"**Participants**: Reviewer, GoExpert",
		"**Files Reviewed**: 2",
	}

	for _, part := range expectedParts {
		if !strings.Contains(summary, part) {
			t.Errorf("Expected summary to contain '%s', got:\n%s", part, summary)
		}
	}
}

func TestGenerateHolisticSummary_NoFindings(t *testing.T) {
	agg := &AggregatedReview{
		Target:          "clean.go",
		Participants:    []string{"Reviewer"},
		Files:           []string{"clean.go"},
		Duration:        1 * time.Second,
		FindingsByShard: map[string][]reviewer.ParsedFinding{},
	}

	summary := generateHolisticSummary(agg)

	if !strings.Contains(summary, "Multi-shard review of clean.go completed") {
		t.Error("Expected summary header")
	}
	if !strings.Contains(summary, "**Files Reviewed**: 1") {
		t.Error("Expected file count")
	}
}

func TestExtractCrossShardInsights_NoOverlap(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {{File: "a.go", Line: 10}},
		"shard2": {{File: "b.go", Line: 20}},
	}

	insights := extractCrossShardInsights(findingsByShard)

	// No hot spots expected since different files
	for _, insight := range insights {
		if strings.Contains(insight, "Hot spot") {
			t.Error("Unexpected hot spot insight when files don't overlap")
		}
	}
}

func TestExtractCrossShardInsights_HotSpotDetection(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {{File: "problem.go", Line: 10}},
		"shard2": {{File: "problem.go", Line: 20}}, // Same file, different line
	}

	insights := extractCrossShardInsights(findingsByShard)

	foundHotSpot := false
	for _, insight := range insights {
		if strings.Contains(insight, "Hot spot") && strings.Contains(insight, "problem.go") {
			foundHotSpot = true
		}
	}

	if !foundHotSpot {
		t.Error("Expected hot spot insight for file flagged by multiple shards")
	}
}

func TestExtractCrossShardInsights_CriticalAttention(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {
			{File: "a.go", Line: 10, Severity: "critical"},
			{File: "b.go", Line: 20, Severity: "critical"},
		},
	}

	insights := extractCrossShardInsights(findingsByShard)

	foundCritical := false
	for _, insight := range insights {
		if strings.Contains(insight, "critical issues require immediate attention") {
			foundCritical = true
		}
	}

	if !foundCritical {
		t.Error("Expected critical attention insight")
	}
}

func TestExtractCrossShardInsights_MultipleErrors(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {
			{Severity: "error"},
			{Severity: "error"},
			{Severity: "error"},
			{Severity: "error"}, // 4 errors > 3 threshold
		},
	}

	insights := extractCrossShardInsights(findingsByShard)

	foundPattern := false
	for _, insight := range insights {
		if strings.Contains(insight, "Multiple error-level issues") {
			foundPattern = true
		}
	}

	if !foundPattern {
		t.Error("Expected pattern insight for multiple errors")
	}
}

func TestExtractCrossShardInsights_CrossDomainReview(t *testing.T) {
	findingsByShard := map[string][]reviewer.ParsedFinding{
		"shard1": {{File: "a.go"}},
		"shard2": {{File: "b.go"}},
		"shard3": {{File: "c.go"}}, // 3 shards > 2 threshold
	}

	insights := extractCrossShardInsights(findingsByShard)

	foundCrossDomain := false
	for _, insight := range insights {
		if strings.Contains(insight, "Cross-domain review") {
			foundCrossDomain = true
		}
	}

	if !foundCrossDomain {
		t.Error("Expected cross-domain review insight")
	}
}

func TestFormatMultiShardResponse_Complete(t *testing.T) {
	review := &AggregatedReview{
		Target:       "test/",
		Participants: []string{"Reviewer", "GoExpert"},
		IsComplete:   true,
		Summary:      "Test summary",
		FindingsByShard: map[string][]reviewer.ParsedFinding{
			"Reviewer": {{File: "a.go", Line: 1, Severity: "warning", Message: "Test"}},
		},
		HolisticInsights: []string{"Insight 1", "Insight 2"},
	}

	output := formatMultiShardResponse(review)

	expectedParts := []string{
		"# Multi-Shard Code Review: test/",
		"**Status**: Complete",
		"## Summary",
		"## Cross-Shard Insights",
		"Insight 1",
		"## Findings by Specialist",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain '%s'", part)
		}
	}
}

func TestFormatMultiShardResponse_Incomplete(t *testing.T) {
	review := &AggregatedReview{
		Target:           "test/",
		Participants:     []string{"Reviewer"},
		IsComplete:       false,
		IncompleteReason: []string{"GoExpert: failed after 2 attempts"},
		Summary:          "Partial review",
		FindingsByShard:  map[string][]reviewer.ParsedFinding{},
	}

	output := formatMultiShardResponse(review)

	if !strings.Contains(output, "## Incomplete Reasons") {
		t.Error("Expected incomplete reasons section")
	}
	if !strings.Contains(output, "GoExpert: failed after 2 attempts") {
		t.Error("Expected failure reason in output")
	}
}
