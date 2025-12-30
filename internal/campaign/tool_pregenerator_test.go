package campaign

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewToolPregenerator(t *testing.T) {
	// Test with nil dependencies
	pregenerator := NewToolPregenerator(nil, nil, nil)
	if pregenerator == nil {
		t.Fatal("NewToolPregenerator returned nil")
	}

	// Check default config
	if pregenerator.config.DetectionTimeout != 30*time.Second {
		t.Errorf("expected DetectionTimeout 30s, got %v", pregenerator.config.DetectionTimeout)
	}
	if pregenerator.config.GenerationTimeout != 5*time.Minute {
		t.Errorf("expected GenerationTimeout 5m, got %v", pregenerator.config.GenerationTimeout)
	}
	if !pregenerator.config.RequireThunderdome {
		t.Error("expected RequireThunderdome true by default")
	}
}

func TestDefaultPregeneratorConfig(t *testing.T) {
	cfg := DefaultPregeneratorConfig()

	if cfg.DetectionTimeout != 30*time.Second {
		t.Errorf("expected DetectionTimeout 30s, got %v", cfg.DetectionTimeout)
	}
	if cfg.GenerationTimeout != 5*time.Minute {
		t.Errorf("expected GenerationTimeout 5m, got %v", cfg.GenerationTimeout)
	}
	if cfg.MaxToolsToGenerate != 5 {
		t.Errorf("expected MaxToolsToGenerate 5, got %d", cfg.MaxToolsToGenerate)
	}
	if cfg.MinConfidence != 0.6 {
		t.Errorf("expected MinConfidence 0.6, got %f", cfg.MinConfidence)
	}
	if !cfg.RequireThunderdome {
		t.Error("expected RequireThunderdome true")
	}
	if cfg.RequireSimulation {
		t.Error("expected RequireSimulation false")
	}
	if !cfg.EnableMCPFallback {
		t.Error("expected EnableMCPFallback true")
	}
}

func TestToolPregenerator_WithConfig(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)

	customConfig := PregeneratorConfig{
		DetectionTimeout:   time.Minute,
		GenerationTimeout:  10 * time.Minute,
		MaxToolsToGenerate: 10,
		RequireThunderdome: false,
	}

	result := pregenerator.WithConfig(customConfig)

	if result != pregenerator {
		t.Error("WithConfig should return same pregenerator for chaining")
	}
	if pregenerator.config.DetectionTimeout != time.Minute {
		t.Errorf("expected DetectionTimeout 1m, got %v", pregenerator.config.DetectionTimeout)
	}
	if pregenerator.config.MaxToolsToGenerate != 10 {
		t.Errorf("expected MaxToolsToGenerate 10, got %d", pregenerator.config.MaxToolsToGenerate)
	}
	if pregenerator.config.RequireThunderdome {
		t.Error("expected RequireThunderdome false")
	}
}

func TestToolGap_Fields(t *testing.T) {
	gap := ToolGap{
		ID:          "gap-123",
		Capability:  "json_validator",
		Description: "Validates JSON against schema",
		Confidence:  0.85,
		Priority:    0.9,
		RequiredBy:  []string{"task-1", "task-2"},
		ResolvedBy:  "",
	}

	if gap.ID != "gap-123" {
		t.Errorf("expected ID 'gap-123', got %s", gap.ID)
	}
	if gap.Capability != "json_validator" {
		t.Errorf("expected Capability 'json_validator', got %s", gap.Capability)
	}
	if gap.Confidence != 0.85 {
		t.Errorf("expected Confidence 0.85, got %f", gap.Confidence)
	}
	if len(gap.RequiredBy) != 2 {
		t.Errorf("expected 2 RequiredBy, got %d", len(gap.RequiredBy))
	}
}

func TestGeneratedTool_Fields(t *testing.T) {
	tool := GeneratedTool{
		ID:                "gen-123",
		Name:              "json_validator",
		Purpose:           "Validates JSON",
		InputType:         "string",
		OutputType:        "bool",
		GeneratedAt:       time.Now(),
		PassedThunderdome: true,
		PassedSimulation:  true,
		ValidationErrors:  []string{},
		SourceGap:         "gap-123",
		Status:            "ready",
		RegistryID:        "registry-456",
	}

	if tool.Name != "json_validator" {
		t.Errorf("expected Name 'json_validator', got %s", tool.Name)
	}
	if !tool.PassedThunderdome {
		t.Error("expected PassedThunderdome true")
	}
	if tool.Status != "ready" {
		t.Errorf("expected Status 'ready', got %s", tool.Status)
	}
}

func TestToolPregenerator_DetectGaps_EmptyTasks(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)

	ctx := context.Background()
	// DetectGaps takes (ctx, goal, tasks, *IntelligenceReport)
	gaps, err := pregenerator.DetectGaps(ctx, "Test goal", []TaskInfo{}, nil)

	// Should handle gracefully
	if err != nil {
		t.Errorf("DetectGaps with empty tasks should not error: %v", err)
	}
	if gaps == nil {
		t.Fatal("DetectGaps should return non-nil slice")
	}
}

func TestToolPregenerator_PregenerateTools_EmptyGaps(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)

	ctx := context.Background()
	result, err := pregenerator.PregenerateTools(ctx, []ToolGap{})

	if err != nil {
		t.Errorf("PregenerateTools with empty gaps should not error: %v", err)
	}
	if result == nil {
		t.Fatal("PregenerateTools should return non-nil result")
	}
	if len(result.ToolsGenerated) != 0 {
		t.Errorf("expected 0 tools for empty gaps, got %d", len(result.ToolsGenerated))
	}
}

func TestToolPregenerator_PregenerateTools_LowConfidence(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	pregenerator.config.MinConfidence = 0.7

	ctx := context.Background()
	gaps := []ToolGap{
		{
			ID:         "gap-1",
			Capability: "low_confidence_tool",
			Confidence: 0.5, // Below threshold
		},
	}

	result, err := pregenerator.PregenerateTools(ctx, gaps)

	if err != nil {
		t.Errorf("PregenerateTools should not error: %v", err)
	}
	// Low confidence gaps should be skipped
	if len(result.ToolsGenerated) != 0 {
		t.Errorf("expected 0 tools (low confidence skipped), got %d", len(result.ToolsGenerated))
	}
}

func TestPregenerationResult_Fields(t *testing.T) {
	result := &PregenerationResult{
		GapsDetected:   []ToolGap{{ID: "gap-1"}, {ID: "gap-2"}},
		ToolsGenerated: []GeneratedTool{{Name: "tool-1"}},
		UnresolvedGaps: []ToolGap{{ID: "gap-2"}},
		TotalGaps:      2,
		ResolvedGaps:   1,
		FailedTools:    0,
		Duration:       5 * time.Minute,
		Errors:         []string{},
	}

	if result.TotalGaps != 2 {
		t.Errorf("expected TotalGaps 2, got %d", result.TotalGaps)
	}
	if result.ResolvedGaps != 1 {
		t.Errorf("expected ResolvedGaps 1, got %d", result.ResolvedGaps)
	}
	if len(result.ToolsGenerated) != 1 {
		t.Errorf("expected 1 tool generated, got %d", len(result.ToolsGenerated))
	}
}

func TestPregenerationResult_FormatForContext(t *testing.T) {
	result := &PregenerationResult{
		GapsDetected: []ToolGap{{ID: "gap-1"}},
		ToolsGenerated: []GeneratedTool{
			{Name: "tool1", Status: "ready"},
			{Name: "tool2", Status: "ready"},
		},
		TotalGaps:    3,
		ResolvedGaps: 2,
		FailedTools:  1,
		Duration:     5 * time.Minute,
		Errors:       []string{"Failed to generate tool3"},
	}

	formatted := result.FormatForContext()

	if formatted == "" {
		t.Fatal("FormatForContext should not return empty string")
	}
	if !strings.Contains(formatted, "TOOL PRE-GENERATION") {
		t.Error("should contain header")
	}
}

func TestGeneratedTool_StatusTransitions(t *testing.T) {
	// Test valid status values
	validStatuses := []string{"pending", "validated", "ready", "failed"}

	for _, status := range validStatuses {
		tool := GeneratedTool{
			Name:   "test_tool",
			Status: status,
		}
		if tool.Status != status {
			t.Errorf("expected status %s, got %s", status, tool.Status)
		}
	}
}

func TestToolPregenerator_CountResolvedGaps(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)

	gaps := []ToolGap{
		{ID: "gap-1", ResolvedBy: "tool-1"},
		{ID: "gap-2", ResolvedBy: ""},
		{ID: "gap-3", ResolvedBy: "tool-3"},
		{ID: "gap-4", ResolvedBy: ""},
	}

	count := pregenerator.countResolvedGaps(gaps)

	if count != 2 {
		t.Errorf("expected 2 resolved gaps, got %d", count)
	}
}

func TestToolGap_RequiredByTracking(t *testing.T) {
	gap := ToolGap{
		ID:         "gap-1",
		Capability: "test_tool",
		RequiredBy: []string{"task-1", "task-2", "task-3"},
	}

	if len(gap.RequiredBy) != 3 {
		t.Errorf("expected 3 RequiredBy tasks, got %d", len(gap.RequiredBy))
	}

	// Verify specific task IDs
	expectedTasks := map[string]bool{"task-1": true, "task-2": true, "task-3": true}
	for _, taskID := range gap.RequiredBy {
		if !expectedTasks[taskID] {
			t.Errorf("unexpected task ID: %s", taskID)
		}
	}
}

func TestPregeneratorConfig_Validation(t *testing.T) {
	// Test that config values are reasonable
	cfg := DefaultPregeneratorConfig()

	if cfg.DetectionTimeout <= 0 {
		t.Error("DetectionTimeout should be positive")
	}
	if cfg.GenerationTimeout <= 0 {
		t.Error("GenerationTimeout should be positive")
	}
	if cfg.MaxToolsToGenerate <= 0 {
		t.Error("MaxToolsToGenerate should be positive")
	}
	if cfg.MinConfidence < 0 || cfg.MinConfidence > 1 {
		t.Error("MinConfidence should be between 0 and 1")
	}
}

func TestGeneratedTool_ValidationErrors(t *testing.T) {
	tool := GeneratedTool{
		Name:             "test_tool",
		Status:           "failed",
		ValidationErrors: []string{"Compilation error", "Test failure"},
	}

	if len(tool.ValidationErrors) != 2 {
		t.Errorf("expected 2 validation errors, got %d", len(tool.ValidationErrors))
	}
	if tool.Status != "failed" {
		t.Error("tool with validation errors should have 'failed' status")
	}
}

func TestTaskInfo_Fields(t *testing.T) {
	task := TaskInfo{
		ID:          "task-1",
		Description: "Implement feature X",
		Type:        "implement",
		Actions:     []string{"create_file", "write_code"},
		FilePaths:   []string{"src/feature.go", "src/feature_test.go"},
	}

	if task.ID != "task-1" {
		t.Errorf("expected ID 'task-1', got %s", task.ID)
	}
	if task.Type != "implement" {
		t.Errorf("expected Type 'implement', got %s", task.Type)
	}
	if len(task.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(task.Actions))
	}
	if len(task.FilePaths) != 2 {
		t.Errorf("expected 2 file paths, got %d", len(task.FilePaths))
	}
}

func TestToolGap_Resolution(t *testing.T) {
	gap := ToolGap{
		ID:             "gap-1",
		Capability:     "test_capability",
		ResolvedBy:     "tool-123",
		ResolutionType: "generated",
	}

	if gap.ResolvedBy != "tool-123" {
		t.Errorf("expected ResolvedBy 'tool-123', got %s", gap.ResolvedBy)
	}
	if gap.ResolutionType != "generated" {
		t.Errorf("expected ResolutionType 'generated', got %s", gap.ResolutionType)
	}
}

func TestPregeneratorConfig_SafetyFlags(t *testing.T) {
	// Test with safety disabled
	cfg := PregeneratorConfig{
		RequireThunderdome: false,
		RequireSimulation:  false,
	}

	if cfg.RequireThunderdome {
		t.Error("expected RequireThunderdome false")
	}
	if cfg.RequireSimulation {
		t.Error("expected RequireSimulation false")
	}

	// Test with safety enabled
	cfg = PregeneratorConfig{
		RequireThunderdome: true,
		RequireSimulation:  true,
	}

	if !cfg.RequireThunderdome {
		t.Error("expected RequireThunderdome true")
	}
	if !cfg.RequireSimulation {
		t.Error("expected RequireSimulation true")
	}
}
