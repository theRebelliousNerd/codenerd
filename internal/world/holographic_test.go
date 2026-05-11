package world

import (
	"context"
	"testing"

	"codenerd/internal/core"

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

// TODO: TEST_GAP: [Null/Undefined/Empty] TestGetContext_EmptyFilePath
// Verify that calling GetContext("") returns a minimal empty context or an error without panicking.

// TODO: TEST_GAP: [Null/Undefined/Empty] TestGetContext_NilKernel
// Verify that GetContext handles a nil kernel safely, skipping relationship queries without dereferencing nil.

// TODO: TEST_GAP: [Null/Undefined/Empty] TestGetContext_EmptyFileContent
// Verify that parsing a 0-byte Go file is handled gracefully by the AST parser and formatting logic.

// TODO: TEST_GAP: [Null/Undefined/Empty] TestGetContext_EmptyPackageDirectory
// Verify behavior when the target file's directory contains no other Go files.

// TODO: TEST_GAP: [Null/Undefined/Empty] TestParsePriorityFacts_EmptyArguments
// Verify that Mangle facts with empty string arguments (e.g., context_priority_file("", "", 50)) are safely skipped.

// TODO: TEST_GAP: [Type Coercion] TestIntArg_MalformedString
// Verify fallback behavior of intArg when provided completely unparseable strings.

// TODO: TEST_GAP: [Type Coercion] TestPriorityAtomToInt_WhitespaceAndCase
// Verify aggressive normalization (trim space, lowercase) of priority atoms (e.g. " /CRITICAL ").

// TODO: TEST_GAP: [Type Coercion] TestFormatNode_MalformedAST
// Verify formatNode returns safe representations for deeply nested or corrupted ast.Node structures.

// TODO: TEST_GAP: [Type Coercion] TestExtractFunctionBodyRegex_CommentsAndStrings
// Expose limitations of the regex fallback when non-Go files contain function-like strings or comments.

// TODO: TEST_GAP: [Type Coercion] TestGetContext_UnknownExtension
// Verify graceful degradation to basic architectural context for unknown file extensions.

// TODO: TEST_GAP: [User Request Extremes] TestBuildGoContext_MassivePackageDir
// Verify OOM/CPU protection when parsing a directory containing 10,000+ files.

// TODO: TEST_GAP: [User Request Extremes] TestFetchFunctionBody_MassiveFile
// Verify bounded memory usage (e.g., io.LimitReader) when attempting to read a 50MB+ source file.

// TODO: TEST_GAP: [User Request Extremes] TestExtractLineRange_HugeFunction
// Ensure correct truncation and warning insertion for functions exceeding maxCallerBodyLines (e.g., 5,000 lines).

// TODO: TEST_GAP: [User Request Extremes] TestQueryRelationships_DeeplyRecursiveGraph
// Verify protection against unbounded call graph expansion when Mangle returns massive or cyclic code_calls facts.

// TODO: TEST_GAP: [User Request Extremes] TestResolvePrioritizedCallers_MassiveFactCount
// Verify performance and memory behavior when 5,000+ prioritized callers are provided to the sorter.

// TODO: TEST_GAP: [State Conflicts] TestBuildGoContext_FileDeletedConcurrently (TOCTOU)
// Verify error handling when a file is deleted from the filesystem just before the parser reads it.

// TODO: TEST_GAP: [State Conflicts] TestResolvePrioritizedCallers_ConcurrentAccess
// Detect race conditions (e.g., concurrent map read/write) by running the fileContentCache concurrently.

// TODO: TEST_GAP: [State Conflicts] TestBuildWithImpactPriorities_ContextCancellation
// Ensure massive I/O operations respect ctx.Done() and halt immediately upon context cancellation.

// TODO: TEST_GAP: [State Conflicts] TestParsePriorityFacts_ConflictingFacts
// Verify deduplication and resolution logic when identical facts have conflicting priorities.

// TODO: TEST_GAP: [State Conflicts] TestGetContext_SynchronousExecution
// Document the architectural flaw that GetContext cannot be canceled because it doesn't take context.Context.

func TestHolographicProviderPriorityAtomToInt(t *testing.T) {
	h := &HolographicProvider{}

	tests := []struct {
		name string
		atom string
		want int
	}{
		{name: "critical", atom: "/critical", want: 100},
		{name: "high", atom: "/high", want: 80},
		{name: "medium", atom: "/medium", want: 50},
		{name: "normal", atom: "/normal", want: 50},
		{name: "low", atom: "/low", want: 25},
		{name: "lowest", atom: "/lowest", want: 10},
		{name: "unknown", atom: "/unknown", want: 50},
		{name: "no_slash", atom: "high", want: 80},
		// Additional test cases for non-standard inputs
		{name: "empty_string", atom: "", want: 50},
		{name: "single_slash", atom: "/", want: 50},
		{name: "uppercase_critical", atom: "CRITICAL", want: 100},
		{name: "uppercase_slash_critical", atom: "/CRITICAL", want: 100},
		{name: "whitespace_padded", atom: "  high  ", want: 50}, // whitespace not trimmed currently
		{name: "numeric_string_100", atom: "100", want: 50},     // numeric strings return default
		{name: "numeric_string_0", atom: "0", want: 50},         // numeric strings return default
		{name: "malformed_slashes", atom: "//high", want: 50},   // double slash not handled
		{name: "malformed_path", atom: "/super/high", want: 50}, // path-like atom not handled
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.priorityAtomToInt(tt.atom)
			if got != tt.want {
				t.Errorf("priorityAtomToInt(%q) = %d, want %d", tt.atom, got, tt.want)
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
		{name: "numeric_string", arg: "80", defaultVal: 50, want: 80},
		{name: "negative_numeric_string", arg: "-10", defaultVal: 50, want: -10},
		{name: "invalid_numeric_string", arg: "80x", defaultVal: 50, want: 50},
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

func TestBuildWithImpactPrioritiesContextCancellation(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	h := NewHolographicProvider(k, ".")

	// Need to mock priority facts to enter the loop
	k.Assert(core.Fact{
		Predicate: "context_priority_file",
		Args:      []any{"testdata/large_file.go", "FuncA", int64(100)},
	})
	k.Assert(core.Fact{
		Predicate: "context_priority_file",
		Args:      []any{"testdata/large_file.go", "FuncB", int64(90)},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = h.BuildWithImpactPriorities(ctx, "test.go")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestBuildWithImpactPrioritiesNoKernel(t *testing.T) {
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
		{
			name: "python_docstring_with_braces",
			lines: []string{
				"def foo(): {",
				"    \"\"\"",
				"    docstring with {",
				"    and }",
				"    \"\"\"",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
			},
			startIdx: 0,
			want:     6,
		},
		{
			name: "python_single_quote_docstring",
			lines: []string{
				"def bar(): {",
				"    '''",
				"    {",
				"    '''",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
			},
			startIdx: 0,
			want:     5,
		},
		{
			name: "python_docstring_with_internal_quotes_and_braces",
			lines: []string{
				"def foo(): {",
				"    \"\"\"",
				"    docstring with a \" quote and {",
				"    and }",
				"    \"\"\"",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
			},
			startIdx: 0,
			want:     6,
		},
		{
			name: "python_docstring_unbalanced_open_brace",
			lines: []string{
				"def foo(): {",
				"    \"\"\"",
				"    unbalanced open {",
				"    \"\"\"",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
			},
			startIdx: 0,
			want:     5,
		},
		{
			name: "python_docstring_unbalanced_close_brace",
			lines: []string{
				"def foo(): {",
				"    \"\"\"",
				"    unbalanced close }",
				"    \"\"\"",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
			},
			startIdx: 0,
			want:     5,
		},
		{
			name: "python_single_quote_docstring_unbalanced",
			lines: []string{
				"def bar(): {",
				"    '''",
				"    {",
				"    '''",
				"    return",
				"}",
				"    # trailing 1",
				"    # trailing 2",
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
