package world

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrioritizedCallerStruct(t *testing.T) {
	// Test that PrioritizedCaller struct can be created and fields are accessible
	caller := PrioritizedCaller{
		Name:     "TestFunction",
		File:     "test.go",
		Body:     "func TestFunction() {}",
		Priority: 80,
		Depth:    1,
	}

	if caller.Name != "TestFunction" {
		t.Errorf("Name mismatch: got %s, want TestFunction", caller.Name)
	}
	if caller.Priority != 80 {
		t.Errorf("Priority mismatch: got %d, want 80", caller.Priority)
	}
}

func TestHolographicContextWithPrioritizedCallers(t *testing.T) {
	hc := &HolographicContext{
		TargetFile: "target.go",
		TargetPkg:  "world",
		PrioritizedCallers: []PrioritizedCaller{
			{Name: "HighPriority", File: "high.go", Priority: 100, Depth: 1},
			{Name: "MedPriority", File: "med.go", Priority: 50, Depth: 2},
			{Name: "LowPriority", File: "low.go", Priority: 25, Depth: 3},
		},
		ImpactPriority: 100,
	}

	if !hc.HasPrioritizedCallers() {
		t.Error("HasPrioritizedCallers should return true")
	}

	// Test GetHighPriorityCallers
	highCallers := hc.GetHighPriorityCallers(80)
	if len(highCallers) != 1 {
		t.Errorf("GetHighPriorityCallers(80): got %d callers, want 1", len(highCallers))
	}
	if highCallers[0].Name != "HighPriority" {
		t.Errorf("GetHighPriorityCallers(80)[0].Name: got %s, want HighPriority", highCallers[0].Name)
	}

	medCallers := hc.GetHighPriorityCallers(50)
	if len(medCallers) != 2 {
		t.Errorf("GetHighPriorityCallers(50): got %d callers, want 2", len(medCallers))
	}
}

func TestHolographicContextNilSafety(t *testing.T) {
	var hc *HolographicContext

	if hc.HasPrioritizedCallers() {
		t.Error("HasPrioritizedCallers should return false for nil context")
	}

	callers := hc.GetHighPriorityCallers(50)
	if callers != nil {
		t.Error("GetHighPriorityCallers should return nil for nil context")
	}

	formatted := hc.FormatWithPriorities()
	if formatted != "" {
		t.Error("FormatWithPriorities should return empty string for nil context")
	}

	compact := hc.FormatPrioritizedCallersCompact()
	if compact != "" {
		t.Error("FormatPrioritizedCallersCompact should return empty string for nil context")
	}
}

func TestPriorityLevelString(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     string
	}{
		{name: "critical", priority: 95, want: "CRITICAL"},
		{name: "high", priority: 85, want: "HIGH"},
		{name: "medium", priority: 60, want: "MEDIUM"},
		{name: "low", priority: 30, want: "LOW"},
		{name: "minimal", priority: 10, want: "MINIMAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := priorityLevelString(tt.priority)
			if got != tt.want {
				t.Errorf("priorityLevelString(%d) = %s, want %s", tt.priority, got, tt.want)
			}
		})
	}
}

func TestFormatWithPriorities(t *testing.T) {
	hc := &HolographicContext{
		TargetFile: "target.go",
		TargetPkg:  "world",
		PrioritizedCallers: []PrioritizedCaller{
			{
				Name:     "CallsTarget",
				File:     "caller.go",
				Body:     "func CallsTarget() {\n    Target()\n}",
				Priority: 80,
				Depth:    1,
			},
		},
		ImpactPriority: 80,
	}

	formatted := hc.FormatWithPriorities()

	// Check for expected content
	expectedStrings := []string{
		"Impact-Prioritized Context",
		"HIGH",
		"CallsTarget",
		"caller.go",
		"Prioritized Callers",
	}

	for _, expected := range expectedStrings {
		if !containsStr(formatted, expected) {
			t.Errorf("FormatWithPriorities missing expected content: %s", expected)
		}
	}
}

func TestFormatPrioritizedCallersCompact(t *testing.T) {
	hc := &HolographicContext{
		PrioritizedCallers: []PrioritizedCaller{
			{Name: "High", File: "high.go", Priority: 90, Depth: 1},
			{Name: "Med", File: "med.go", Priority: 50, Depth: 2},
			{Name: "Low", File: "low.go", Priority: 20, Depth: 3},
		},
	}

	compact := hc.FormatPrioritizedCallersCompact()

	expectedStrings := []string{
		"[HIGH]",
		"[MED]",
		"[LOW]",
		"High",
		"Med",
		"Low",
		"high.go",
		"(depth=2)",
		"(depth=3)",
	}

	for _, expected := range expectedStrings {
		if !containsStr(compact, expected) {
			t.Errorf("FormatPrioritizedCallersCompact missing: %s\nGot:\n%s", expected, compact)
		}
	}
}

func TestHolographicProviderPriorityAtomToInt(t *testing.T) {
	// TODO: TEST_GAP: Type Coercion Safety
	// priorityAtomToInt assumes atoms are strings (e.g., "/high").
	// If the kernel returns a custom atom type or a raw number, the integration might fail.
	// Tests should verify behavior with non-string inputs and malformed atom strings.

	h := &HolographicProvider{}

	tests := []struct {
		name  string
		atom  string
		want  int
	}{
		{name: "critical", atom: "/critical", want: 100},
		{name: "high", atom: "/high", want: 80},
		{name: "medium", atom: "/medium", want: 50},
		{name: "normal", atom: "/normal", want: 50},
		{name: "low", atom: "/low", want: 25},
		{name: "lowest", atom: "/lowest", want: 10},
		{name: "unknown", atom: "/unknown", want: 50},
		{name: "no_slash", atom: "high", want: 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.priorityAtomToInt(tt.atom)
			if got != tt.want {
				t.Errorf("priorityAtomToInt(%s) = %d, want %d", tt.atom, got, tt.want)
			}
		})
	}
}

func TestHolographicProviderStringArg(t *testing.T) {
	h := &HolographicProvider{}

	tests := []struct {
		name string
		arg  interface{}
		want string
	}{
		{name: "string", arg: "hello", want: "hello"},
		{name: "int", arg: 42, want: "42"},
		{name: "float", arg: 3.14, want: "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.stringArg(tt.arg)
			if got != tt.want {
				t.Errorf("stringArg(%v) = %s, want %s", tt.arg, got, tt.want)
			}
		})
	}
}

func TestHolographicProviderIntArg(t *testing.T) {
	// TODO: TEST_GAP: Numeric String Handling
	// Verify that intArg correctly handles numeric strings (e.g., "80") which might be returned
	// by Mangle if the type system isn't strictly enforced. Currently it might default to 50.

	h := &HolographicProvider{}

	tests := []struct {
		name       string
		arg        interface{}
		defaultVal int
		want       int
	}{
		{name: "int", arg: 42, defaultVal: 0, want: 42},
		{name: "int64", arg: int64(100), defaultVal: 0, want: 100},
		{name: "float64", arg: float64(75.9), defaultVal: 0, want: 75},
		{name: "string_high", arg: "/high", defaultVal: 0, want: 80},
		{name: "unknown_type", arg: struct{}{}, defaultVal: 50, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.intArg(tt.arg, tt.defaultVal)
			if got != tt.want {
				t.Errorf("intArg(%v, %d) = %d, want %d", tt.arg, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestBuildWithImpactPrioritiesNilContext(t *testing.T) {
	h := NewHolographicProvider(nil, ".")

	_, err := h.BuildWithImpactPriorities(nil, "test.go")
	if err == nil {
		t.Error("BuildWithImpactPriorities should return error for nil context")
	}
}

func TestBuildWithImpactPrioritiesNoKernel(t *testing.T) {
	// TODO: TEST_GAP: Context Cancellation
	// We should verify that BuildWithImpactPriorities respects ctx.Done() and aborts
	// processing, especially during the loop where it fetches function bodies.

	h := NewHolographicProvider(nil, ".")

	ctx := context.Background()
	hc, err := h.BuildWithImpactPriorities(ctx, "testdata/nonexistent.go")
	if err != nil {
		// Error is acceptable since file doesn't exist
		return
	}

	// If no error, should return context without prioritized callers
	if hc != nil && len(hc.PrioritizedCallers) > 0 {
		t.Error("BuildWithImpactPriorities without kernel should not have prioritized callers")
	}
}

// TODO: TEST_GAP: Large File Handling (OOM Risk)
// fetchFunctionBody uses os.ReadFile which reads the entire file into memory.
// We need a test that mocks a large file (e.g., via a mocked file system interface)
// or generates a temporary large file to verify that the system handles it gracefully
// (e.g., returns error or truncates reading) instead of crashing with OOM.

func TestExtractLineRange(t *testing.T) {
	h := &HolographicProvider{}

	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		name      string
		startLine int
		endLine   int
		wantErr   bool
		want      string
	}{
		{
			name:      "full_range",
			startLine: 1,
			endLine:   5,
			wantErr:   false,
			want:      "line1\nline2\nline3\nline4\nline5",
		},
		{
			name:      "partial_range",
			startLine: 2,
			endLine:   4,
			wantErr:   false,
			want:      "line2\nline3\nline4",
		},
		{
			name:      "invalid_range",
			startLine: 10,
			endLine:   5,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.extractLineRange(content, tt.startLine, tt.endLine)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractLineRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("extractLineRange() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestFindFunctionEnd(t *testing.T) {
	// TODO: TEST_GAP: Python Docstrings and Complex Syntax
	// The current brace counting logic may fail on Python multi-line strings (""" ... """)
	// if they contain braces. It treats " as a single-line string delimiter.
	// We need a test case with Python docstrings containing braces to verify this failure mode.

	h := &HolographicProvider{}

	tests := []struct {
		name     string
		lines    []string
		startIdx int
		want     int
	}{
		{
			name: "simple_function",
			lines: []string{
				"func foo() {",
				"    return",
				"}",
			},
			startIdx: 0,
			want:     2,
		},
		{
			name: "nested_braces",
			lines: []string{
				"func foo() {",
				"    if true {",
				"        return",
				"    }",
				"}",
			},
			startIdx: 0,
			want:     4,
		},
		{
			name: "no_closing_brace",
			lines: []string{
				"func foo() {",
				"    return",
			},
			startIdx: 0,
			want:     1, // Falls back to startIdx + maxCallerBodyLines or len-1
		},

		{
			name: "brace_in_string",
			lines: []string{
				"func foo() {",
				"    s := \"}\"",
				"    return",
				"}",
			},
			startIdx: 0,
			want:     3,
		},
		{
			name: "brace_in_comment",
			lines: []string{
				"func foo() {",
				"    // }",
				"    return",
				"}",
			},
			startIdx: 0,
			want:     3,
		},
		{
			name: "brace_in_multiline_comment",
			lines: []string{
				"func foo() {",
				"    /*",
				"    }",
				"    */",
				"    return",
				"}",
			},
			startIdx: 0,
			want:     5,
		},
		{
			name: "brace_in_backtick",
			lines: []string{
				"func foo() {",
				"    s := `",
				"    }",
				"    `",
				"    return",
				"}",
			},
			startIdx: 0,
			want:     5,
		},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.findFunctionEnd(tt.lines, tt.startIdx)
			// For "no_closing_brace" case, we accept any reasonable fallback
			if tt.name == "no_closing_brace" {
				if got < tt.startIdx {
					t.Errorf("findFunctionEnd() returned %d, which is before startIdx %d", got, tt.startIdx)
				}
			} else if got != tt.want {
				t.Errorf("findFunctionEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Helper function for string containment check
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
