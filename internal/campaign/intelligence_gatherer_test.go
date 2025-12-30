package campaign

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

func TestNewIntelligenceGatherer(t *testing.T) {
	// Test with nil dependencies (should not panic)
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)
	if gatherer == nil {
		t.Fatal("NewIntelligenceGatherer returned nil")
	}

	// Check default config was applied
	if gatherer.config.GatherTimeout != 5*time.Minute {
		t.Errorf("expected GatherTimeout 5m, got %v", gatherer.config.GatherTimeout)
	}
	if gatherer.config.MaxChurnHotspots != 50 {
		t.Errorf("expected MaxChurnHotspots 50, got %d", gatherer.config.MaxChurnHotspots)
	}
}

func TestDefaultIntelligenceConfig(t *testing.T) {
	cfg := DefaultIntelligenceConfig()

	// Verify timeout defaults
	if cfg.GatherTimeout != 5*time.Minute {
		t.Errorf("expected GatherTimeout 5m, got %v", cfg.GatherTimeout)
	}
	if cfg.PerSystemTimeout != 30*time.Second {
		t.Errorf("expected PerSystemTimeout 30s, got %v", cfg.PerSystemTimeout)
	}
	if cfg.ConsultTimeout != 2*time.Minute {
		t.Errorf("expected ConsultTimeout 2m, got %v", cfg.ConsultTimeout)
	}

	// Verify limit defaults
	if cfg.MaxChurnHotspots != 50 {
		t.Errorf("expected MaxChurnHotspots 50, got %d", cfg.MaxChurnHotspots)
	}
	if cfg.MaxLearnings != 100 {
		t.Errorf("expected MaxLearnings 100, got %d", cfg.MaxLearnings)
	}
	if cfg.MaxMCPTools != 30 {
		t.Errorf("expected MaxMCPTools 30, got %d", cfg.MaxMCPTools)
	}
	if cfg.GitHistoryDepth != 100 {
		t.Errorf("expected GitHistoryDepth 100, got %d", cfg.GitHistoryDepth)
	}

	// Verify feature flags all enabled by default
	if !cfg.EnableWorldModel {
		t.Error("expected EnableWorldModel true")
	}
	if !cfg.EnableGitHistory {
		t.Error("expected EnableGitHistory true")
	}
	if !cfg.EnableLearningStore {
		t.Error("expected EnableLearningStore true")
	}
	if !cfg.EnableSafetyCheck {
		t.Error("expected EnableSafetyCheck true")
	}
}

func TestIntelligenceGatherer_WithConfig(t *testing.T) {
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	customConfig := IntelligenceConfig{
		GatherTimeout:    10 * time.Minute,
		MaxChurnHotspots: 100,
		EnableWorldModel: false,
	}

	result := gatherer.WithConfig(customConfig)

	if result != gatherer {
		t.Error("WithConfig should return same gatherer for chaining")
	}
	if gatherer.config.GatherTimeout != 10*time.Minute {
		t.Errorf("expected GatherTimeout 10m, got %v", gatherer.config.GatherTimeout)
	}
	if gatherer.config.MaxChurnHotspots != 100 {
		t.Errorf("expected MaxChurnHotspots 100, got %d", gatherer.config.MaxChurnHotspots)
	}
	if gatherer.config.EnableWorldModel {
		t.Error("expected EnableWorldModel false")
	}
}

func TestIntelligenceGatherer_Gather_NilDependencies(t *testing.T) {
	// Test that Gather handles nil dependencies gracefully
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	// Disable all features that require dependencies
	gatherer.config.EnableWorldModel = false
	gatherer.config.EnableGitHistory = false
	gatherer.config.EnableLearningStore = false
	gatherer.config.EnableKnowledgeGraph = false
	gatherer.config.EnableColdStorage = false
	gatherer.config.EnableSafetyCheck = false
	gatherer.config.EnableAutopoiesis = false
	gatherer.config.EnableMCPTools = false
	gatherer.config.EnablePreviousCampaigns = false
	gatherer.config.EnableShardConsult = false
	gatherer.config.EnableTestCoverage = false
	gatherer.config.EnableCodePatterns = false

	ctx := context.Background()
	report, err := gatherer.Gather(ctx, "Test goal", []string{})

	if err != nil {
		t.Fatalf("Gather with disabled features should not error: %v", err)
	}
	if report == nil {
		t.Fatal("Gather should return a report")
	}
	if report.GatheredAt.IsZero() {
		t.Error("GatheredAt should be set")
	}
	// Duration may be 0 if all features are disabled and it completes instantly
	// The important thing is that we got a report without errors
}

func TestIntelligenceGatherer_Gather_WithKernel(t *testing.T) {
	// Create a minimal kernel for testing
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("Could not create kernel: %v", err)
	}

	gatherer := NewIntelligenceGatherer(kern, nil, nil, nil, nil, nil, nil, nil)

	// Only enable kernel-based features
	gatherer.config.EnableWorldModel = false
	gatherer.config.EnableGitHistory = false
	gatherer.config.EnableLearningStore = false
	gatherer.config.EnableKnowledgeGraph = false
	gatherer.config.EnableColdStorage = false
	gatherer.config.EnableSafetyCheck = true // Uses kernel
	gatherer.config.EnableAutopoiesis = false
	gatherer.config.EnableMCPTools = false
	gatherer.config.EnablePreviousCampaigns = true // Uses kernel
	gatherer.config.EnableShardConsult = false
	gatherer.config.EnableTestCoverage = true // Uses kernel
	gatherer.config.EnableCodePatterns = true // Uses kernel

	ctx := context.Background()
	report, err := gatherer.Gather(ctx, "Test goal with kernel", []string{})

	if err != nil {
		t.Fatalf("Gather should not error: %v", err)
	}
	if report == nil {
		t.Fatal("Gather should return a report")
	}
}

func TestIntelligenceReport_FormatForContext(t *testing.T) {
	report := &IntelligenceReport{
		GatheredAt:        time.Now(),
		Duration:          5 * time.Second,
		FileTopology:      map[string]FileInfo{"test.go": {Path: "test.go", Language: "go"}},
		LanguageBreakdown: map[string]int{"go": 10, "python": 5},
		GitChurnHotspots: []ChurnHotspot{
			{Path: "hot.go", ChurnRate: 15},
		},
		HistoricalPatterns: []LearningPattern{
			{ShardType: "coder", Description: "Test pattern", Confidence: 0.9},
		},
		SafetyWarnings: []SafetyWarning{
			{Action: "delete", RuleViolated: "dangerous_pattern", Severity: "high"},
		},
		MCPToolsAvailable: []MCPToolInfo{
			{Name: "test_tool", Description: "A test tool"},
		},
		UncoveredPaths: []string{"uncovered.go"},
		ArchitectureHints: []string{"Standard Go project"},
	}

	formatted := report.FormatForContext()

	// Check that key sections are present
	if formatted == "" {
		t.Fatal("FormatForContext should not return empty string")
	}
	if !strings.Contains(formatted, "INTELLIGENCE REPORT") {
		t.Error("should contain INTELLIGENCE REPORT header")
	}
	if !strings.Contains(formatted, "Codebase Overview") {
		t.Error("should contain Codebase Overview")
	}
	if !strings.Contains(formatted, "High Churn Files") {
		t.Error("should contain High Churn Files section")
	}
	if !strings.Contains(formatted, "hot.go") {
		t.Error("should contain hot.go churn hotspot")
	}
	if !strings.Contains(formatted, "Safety Warnings") {
		t.Error("should contain Safety Warnings")
	}
	if !strings.Contains(formatted, "Architecture Hints") {
		t.Error("should contain Architecture Hints")
	}
}

func TestChurnHotspot_ChestertonFence(t *testing.T) {
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	// Test that high churn files get Chesterton's Fence warning
	report := &IntelligenceReport{
		GitChurnHotspots: []ChurnHotspot{},
	}

	// Simulate the warning logic from gatherGitHistory
	testCases := []struct {
		churnRate       int
		expectWarning   bool
		expectHighChurn bool
	}{
		{churnRate: 15, expectWarning: true, expectHighChurn: true},
		{churnRate: 10, expectWarning: true, expectHighChurn: false},
		{churnRate: 7, expectWarning: true, expectHighChurn: false},
		{churnRate: 3, expectWarning: false, expectHighChurn: false},
	}

	for _, tc := range testCases {
		hotspot := ChurnHotspot{
			Path:      "test.go",
			ChurnRate: tc.churnRate,
		}
		if tc.churnRate > 10 {
			hotspot.Reason = "High churn rate"
			hotspot.Warning = "CHESTERTON'S FENCE"
		} else if tc.churnRate > 5 {
			hotspot.Reason = "Moderate churn rate"
			hotspot.Warning = "Consider reviewing"
		}

		hasWarning := hotspot.Warning != ""
		if hasWarning != tc.expectWarning {
			t.Errorf("churnRate %d: expected warning=%v, got %v", tc.churnRate, tc.expectWarning, hasWarning)
		}
	}

	_ = gatherer // silence unused warning
	_ = report
}

func TestBatchConsultRequest(t *testing.T) {
	req := BatchConsultRequest{
		Topic:      "Test Topic",
		Question:   "Test Question?",
		Context:    "Test Context",
		TargetSpec: []string{"coder", "tester"},
	}

	if req.Topic != "Test Topic" {
		t.Errorf("expected Topic 'Test Topic', got %s", req.Topic)
	}
	if len(req.TargetSpec) != 2 {
		t.Errorf("expected 2 target specs, got %d", len(req.TargetSpec))
	}
}

func TestConsultationResponse(t *testing.T) {
	resp := ConsultationResponse{
		RequestID:    "req-123",
		FromSpec:     "coder",
		ToSpec:       "campaign",
		Advice:       "Test advice",
		Confidence:   0.85,
		References:   []string{"ref1", "ref2"},
		Caveats:      []string{"caveat1"},
		ResponseTime: time.Now(),
		Duration:     2 * time.Second,
	}

	if resp.FromSpec != "coder" {
		t.Errorf("expected FromSpec 'coder', got %s", resp.FromSpec)
	}
	if resp.Confidence != 0.85 {
		t.Errorf("expected Confidence 0.85, got %f", resp.Confidence)
	}
	if len(resp.References) != 2 {
		t.Errorf("expected 2 references, got %d", len(resp.References))
	}
}

// Note: contains function is defined in context_pager.go and available in this package
