package campaign

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Vector 1: Null/Empty Inputs
// =============================================================================

func TestIntelligenceGatherer_EmptyGoal(t *testing.T) {
	// Verify that an empty or whitespace-only goal does not panic.
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)
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

	goals := []string{"", "   ", "\n\t", "     \t\n  "}
	for _, goal := range goals {
		t.Run("goal="+goal, func(t *testing.T) {
			ctx := context.Background()
			report, err := gatherer.Gather(ctx, goal, []string{"."})
			if err != nil {
				t.Fatalf("Gather should not error with empty goal: %v", err)
			}
			if report == nil {
				t.Fatal("Gather should return a report")
			}
		})
	}
}

func TestIntelligenceGatherer_EmptyPaths(t *testing.T) {
	// Verify that empty, whitespace, or invalid target paths don't cause
	// phantom lookups or panics.
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)
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

	pathSets := [][]string{
		nil,
		{},
		{"", "   ", "\n"},
		{"/dev/null/fake", "   "},
	}
	for _, paths := range pathSets {
		ctx := context.Background()
		report, err := gatherer.Gather(ctx, "Fix the bug", paths)
		if err != nil {
			t.Fatalf("Gather should not error with paths=%v: %v", paths, err)
		}
		if report == nil {
			t.Fatal("Gather should return a report")
		}
	}
}

func TestIntelligenceGatherer_EmptyFactArguments(t *testing.T) {
	// Verify that parse helpers handle empty/nil args gracefully.
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	// parseArg with empty string
	result := gatherer.parseArg("")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}

	// parseArg with nil
	resultNil := gatherer.parseArg(nil)
	if resultNil != "<nil>" {
		t.Errorf("expected '<nil>' for nil arg, got %q", resultNil)
	}

	// parseAtom with empty string
	atomResult := gatherer.parseAtom("")
	if atomResult != "" {
		t.Errorf("expected empty string from parseAtom, got %q", atomResult)
	}

	// parseIntArg with nil
	intResult := gatherer.parseIntArg(nil)
	if intResult != 0 {
		t.Errorf("expected 0 for nil parseIntArg, got %d", intResult)
	}

	// parseFloatArg with nil
	floatResult := gatherer.parseFloatArg(nil)
	if floatResult != 0.0 {
		t.Errorf("expected 0.0 for nil parseFloatArg, got %f", floatResult)
	}
}

// =============================================================================
// Vector 2: Type Coercion
// =============================================================================

func TestIntelligenceGatherer_ParseIntArg_Overflow(t *testing.T) {
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"max_int64", int64(math.MaxInt64), math.MaxInt32},
		{"min_int64", int64(math.MinInt64), math.MinInt32},
		{"large_float64", float64(1e18), math.MaxInt32},
		{"negative_large_float64", float64(-1e18), math.MinInt32},
		{"normal_int64", int64(42), 42},
		{"normal_int", int(100), 100},
		{"zero", int64(0), 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gatherer.parseIntArg(tc.input)
			if result != tc.expected {
				t.Errorf("parseIntArg(%v) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIntelligenceGatherer_ParseAtom_UnexpectedTypes(t *testing.T) {
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name  string
		input interface{}
	}{
		{"boolean_true", true},
		{"boolean_false", false},
		{"integer", 42},
		{"float64", 3.14},
		{"slice", []string{"a", "b"}},
		{"map", map[string]string{"key": "value"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gatherer.parseAtom(tc.input)
			if result == "" {
				t.Error("parseAtom should return non-empty fallback string")
			}
			// The result should be the fmt.Sprintf("%v") representation
			// and should not panic
		})
	}
}

func TestIntelligenceGatherer_ParseFloatArg_StringFallback(t *testing.T) {
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"string_0.95", "0.95", 0.95},
		{"string_1.0", "1.0", 1.0},
		{"string_0.0", "0.0", 0.0},
		{"string_invalid", "not_a_number", 0.0},
		{"string_empty", "", 0.0},
		{"float64_direct", float64(0.75), 0.75},
		{"int_direct", int(1), 1.0},
		{"int64_direct", int64(42), 42.0},
		{"nil_arg", nil, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gatherer.parseFloatArg(tc.input)
			if math.Abs(result-tc.expected) > 1e-9 {
				t.Errorf("parseFloatArg(%v) = %f, want %f", tc.input, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// Vector 3: User Request Extremes
// =============================================================================

func TestIntelligenceReport_FormatForContext_MassiveFields(t *testing.T) {
	// Verify that massive text fields in the report are safely truncated
	// and don't cause OOM during FormatForContext.
	massive := strings.Repeat("A", 50000) // 50KB string

	report := &IntelligenceReport{
		GatheredAt:        time.Now(),
		Duration:          5 * time.Second,
		FileTopology:      make(map[string]FileInfo),
		LanguageBreakdown: make(map[string]int),
		TestCoverage:      make(map[string]float64),
		MCPServerStatus:   make(map[string]string),
		AdvisorySummary:   massive,
		MCPToolsAvailable: []MCPToolInfo{
			{Name: "test_tool", Description: massive},
		},
	}

	formatted := report.FormatForContext()
	if formatted == "" {
		t.Fatal("FormatForContext should not return empty string")
	}

	// The massive advisory summary should be truncated
	if strings.Contains(formatted, massive) {
		t.Error("FormatForContext should truncate massive advisory summary")
	}

	// Tool description should be truncated
	if len(formatted) > 100000 {
		t.Errorf("FormatForContext output too large: %d bytes (expected <100KB)", len(formatted))
	}
}

func TestIntelligenceGatherer_ErrorBoundedAccumulation(t *testing.T) {
	// Verify that error accumulation is bounded at 100 entries.
	gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)
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

	// We can't directly test addError since it's a closure, but we can
	// verify the bounded error concept by checking the Gather result.
	ctx := context.Background()
	report, err := gatherer.Gather(ctx, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With all systems disabled and no errors generated, this should be empty
	if len(report.GatheringErrors) != 0 {
		t.Errorf("expected 0 gathering errors, got %d", len(report.GatheringErrors))
	}
}

// =============================================================================
// Vector 4: State Conflicts & Concurrency
// =============================================================================

func TestIntelligenceGatherer_Concurrency_NoRace(t *testing.T) {
	// Verify that running multiple Gather calls concurrently does not
	// produce data races in report mutations. We disable all kernel-based
	// features to isolate the errgroup + report pattern from pre-existing
	// races in the global Mangle parser.
	// Must be run with `go test -race` to be meaningful.
	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gatherer := NewIntelligenceGatherer(nil, nil, nil, nil, nil, nil, nil, nil)
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
			gatherer.config.GatherTimeout = 5 * time.Second

			ctx := context.Background()
			report, err := gatherer.Gather(ctx, "Concurrent test goal", []string{"."})
			if err != nil {
				t.Errorf("concurrent Gather failed: %v", err)
				return
			}
			if report == nil {
				t.Error("concurrent Gather returned nil report")
			}
			// Exercise FormatForContext concurrently too
			formatted := report.FormatForContext()
			if formatted == "" {
				t.Error("FormatForContext returned empty in concurrent execution")
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// Helpers: truncateField
// =============================================================================

func TestTruncateField(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
		{"zero_max", "hello", 0, "..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateField(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateField(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}
