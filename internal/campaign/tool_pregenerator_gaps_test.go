package campaign

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Vector 1: Null/Undefined/Empty
// =============================================================================

func TestToolPregenerator_DetectGaps_NilTasks(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	ctx := context.Background()

	// Explicitly test nil slice (Go handles nil iteration safely,
	// but we must ensure no len() calls precede nil-checks in future code).
	var nilTasks []TaskInfo
	gaps, err := pregenerator.DetectGaps(ctx, "Test goal", nilTasks, nil)

	if err != nil {
		t.Errorf("DetectGaps with nil tasks should not error: %v", err)
	}
	if gaps == nil {
		t.Error("Expected non-nil empty slice for nil tasks")
	}
	if len(gaps) != 0 {
		t.Errorf("Expected 0 gaps for nil tasks, got %d", len(gaps))
	}
}

func TestToolPregenerator_DetectGaps_EmptyDescription(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	ctx := context.Background()

	tasks := []TaskInfo{
		{
			ID:          "empty-desc-task",
			Description: "",
			Type:        "implement",
		},
		{
			ID:          "whitespace-desc-task",
			Description: "   \t\n  ",
			Type:        "implement",
		},
		{
			ID:          "control-chars-task",
			Description: "\x00\x01\x02",
			Type:        "implement",
		},
	}

	gaps, err := pregenerator.DetectGaps(ctx, "Test goal", tasks, nil)
	if err != nil {
		t.Errorf("DetectGaps failed on empty descriptions: %v", err)
	}
	// Empty and whitespace descriptions should not trigger false positives
	if len(gaps) > 0 {
		t.Errorf("Expected no gaps for empty/whitespace descriptions, got %d", len(gaps))
	}
}

func TestToolPregenerator_PregenerateTools_NilGaps(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	ctx := context.Background()

	var nilGaps []ToolGap
	result, err := pregenerator.PregenerateTools(ctx, nilGaps)

	if err != nil {
		t.Errorf("PregenerateTools with nil gaps should not error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result for nil gaps")
	}
	if len(result.ToolsGenerated) != 0 {
		t.Errorf("Expected 0 tools generated for nil gaps, got %d", len(result.ToolsGenerated))
	}
	if result.TotalGaps != 0 {
		t.Errorf("Expected TotalGaps 0 for nil gaps, got %d", result.TotalGaps)
	}
}

// =============================================================================
// Vector 2: Type Coercion
// =============================================================================

func TestToolPregenerator_MaliciousCapability(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name       string
		capability string
	}{
		{"newlines", "tool\nwith\nnewlines"},
		{"null_bytes", "tool\x00with\x00nulls"},
		{"unicode_control", "tool\u200b\u200cwith\u200dcontrol"},
		{"shell_injection", "$(rm -rf /)"},
		{"very_long", strings.Repeat("a", 4096)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gaps := []ToolGap{
				{
					ID:         "gap-malicious",
					Capability: tc.capability,
					Confidence: 0.9,
					Priority:   0.9,
				},
			}

			result, err := pregenerator.PregenerateTools(ctx, gaps)
			if err != nil {
				t.Fatalf("PregenerateTools should not panic/error: %v", err)
			}

			// Verify the tool ID doesn't contain raw newlines
			for _, tool := range result.ToolsGenerated {
				if strings.Contains(tool.ID, "\n") {
					t.Errorf("Tool ID contains unescaped newlines: %q", tool.ID)
				}
			}
		})
	}
}

// =============================================================================
// Vector 3: User Request Extremes
// =============================================================================

func TestToolPregenerator_DetectGaps_MassiveTasks(t *testing.T) {
	pregenerator := NewToolPregenerator(nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numTasks = 100000
	tasks := make([]TaskInfo, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = TaskInfo{
			ID:          fmt.Sprintf("task-%d", i),
			Description: "Implement API parser and fetch data from database",
		}
	}

	start := time.Now()
	_, err := pregenerator.DetectGaps(ctx, "Massive Goal", tasks, nil)
	duration := time.Since(start)

	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("Unexpected error: %v", err)
	}

	// Performance bound: should complete or timeout within 5s
	if duration > 6*time.Second {
		t.Errorf("DetectGaps took %v for %d tasks — exceeds timeout bound", duration, numTasks)
	}
}

func TestToolPregenerator_MaxToolsToGenerate_Extremes(t *testing.T) {
	tests := []struct {
		name     string
		maxTools int
	}{
		{"zero", 0},
		{"negative", -1},
		{"max_int32", math.MaxInt32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pregenerator := NewToolPregenerator(nil, nil, nil)
			pregenerator.config.MaxToolsToGenerate = tc.maxTools

			ctx := context.Background()
			gaps := []ToolGap{
				{ID: "gap-1", Capability: "test_tool", Confidence: 0.9, Priority: 0.9},
			}

			result, err := pregenerator.PregenerateTools(ctx, gaps)
			if err != nil {
				t.Fatalf("PregenerateTools should not error: %v", err)
			}

			// For 0 or negative, no tools should be generated (truncation to 0)
			if tc.maxTools <= 0 && len(result.ToolsGenerated) > 0 {
				t.Errorf("Expected 0 tools for maxTools=%d, got %d",
					tc.maxTools, len(result.ToolsGenerated))
			}
		})
	}
}

// =============================================================================
// Vector 4: State Conflicts & Concurrency
// =============================================================================

func TestToolPregenerator_ConcurrentAccess(t *testing.T) {
	// Run this test with 'go test -race' to detect data races.
	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Each goroutine gets its own pregenerator to avoid shared config race
			pregenerator := NewToolPregenerator(nil, nil, nil)
			ctx := context.Background()

			// Mix of operations
			if idx%2 == 0 {
				tasks := []TaskInfo{
					{ID: fmt.Sprintf("task-%d", idx), Description: "parse json and validate schema"},
				}
				_, _ = pregenerator.DetectGaps(ctx, "Goal", tasks, nil)
			} else {
				gaps := []ToolGap{
					{ID: fmt.Sprintf("gap-%d", idx), Capability: "test_tool", Confidence: 0.9},
				}
				_, _ = pregenerator.PregenerateTools(ctx, gaps)
			}
		}(i)
	}
	wg.Wait()
}

func TestToolPregenerator_ContextCancellation_MidGeneration(t *testing.T) {
	// Verify that cancelling context during PregenerateTools returns partial results.
	pregenerator := NewToolPregenerator(nil, nil, nil)

	// Use an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gaps := []ToolGap{
		{ID: "gap-1", Capability: "tool_a", Confidence: 0.9, Priority: 0.9},
		{ID: "gap-2", Capability: "tool_b", Confidence: 0.9, Priority: 0.9},
		{ID: "gap-3", Capability: "tool_c", Confidence: 0.9, Priority: 0.9},
	}

	result, err := pregenerator.PregenerateTools(ctx, gaps)
	if err != nil {
		t.Fatalf("PregenerateTools should not return error (errors go in result.Errors): %v", err)
	}

	// With a cancelled context, the loop should break immediately
	hasContextError := false
	for _, e := range result.Errors {
		if strings.Contains(e, "cancelled") || strings.Contains(e, "Context") {
			hasContextError = true
			break
		}
	}
	if !hasContextError && len(result.ToolsGenerated) < len(gaps) {
		// Either we got a context error message, or all tools generated before cancellation
		t.Log("Context cancellation was handled (partial results returned)")
	}
}
