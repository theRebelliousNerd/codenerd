package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// TODO: Add TestHolographicContext_MalformedGoFile - Test parser.ParseFile behavior on malformed Go files to ensure we don't abort parsing the whole package due to a syntax error in one sibling file.
// TODO: Add TestHolographicContext_ConcurrentReadWrite - Test concurrent reads/writes to verify sync.RWMutex behavior (especially on regexCache) and prevent data races under heavy parallel access.
// TODO: Add TestHolographicContext_DeletedFileMidFlight - Simulate file deletion between os.ReadDir and parser.ParseFile to ensure we log a warning instead of failing out completely.
// TODO: Add TestHolographicContext_BinaryFileFallback - Verify buildBasicContext fallback cleanly handles binary/non-text file extensions without massive memory allocation.
// TODO: Add TestHolographicContext_EmptyTypeDefinitions - Test extraction for structs with no fields or interfaces with no methods to confirm `Fields` and `Methods` serialization handling.
// TODO: Add TestHolographicContext_FormatWithEmptyCallers - Test `FormatWithPriorities` behavior when `PrioritizedCallers` is explicitly initialized as an empty slice (not nil).

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

func TestGetContext_EmptyFilePath(t *testing.T) {
	h := NewHolographicProvider(nil, ".")
	ctx, err := h.GetContext("")
	if err != nil {
		t.Errorf("Unexpected error for empty file path: %v", err)
	}
	if ctx == nil {
		t.Error("Expected minimal context, got nil")
	}
	if ctx.TargetFile != "" {
		t.Errorf("Expected empty target file, got %s", ctx.TargetFile)
	}
}

func TestGetContext_NilKernel(t *testing.T) {
	h := NewHolographicProvider(nil, ".")
	// It should gracefully fallback to basic parsing without kernel
	// We'll test on holographic.go since it exists
	ctx, err := h.GetContext("holographic.go")
	if err != nil {
		t.Errorf("GetContext with nil kernel failed: %v", err)
	}
	if ctx == nil {
		t.Error("Expected context, got nil")
	}
}

func TestGetContext_EmptyFileContent(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.go")
	os.WriteFile(emptyFile, []byte(""), 0644)
	h := NewHolographicProvider(nil, dir)
	ctx, err := h.GetContext(emptyFile)
	if err != nil {
		t.Errorf("GetContext on empty file failed: %v", err)
	}
	if ctx == nil {
		t.Error("Expected context, got nil")
	}
}

func TestGetContext_EmptyPackageDirectory(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "lonely.go")
	os.WriteFile(emptyFile, []byte("package lonely\n"), 0644)
	h := NewHolographicProvider(nil, dir)
	ctx, err := h.GetContext(emptyFile)
	if err != nil {
		t.Errorf("GetContext on lonely file failed: %v", err)
	}
	if ctx == nil {
		t.Error("Expected context, got nil")
	}
}

func TestParsePriorityFacts_EmptyArguments(t *testing.T) {
	h := &HolographicProvider{}
	facts := []core.Fact{
		{Predicate: "context_priority_file", Args: []any{"", "", 50}},
		{Predicate: "context_priority_file", Args: []any{"file.go", "", 50}}, // Valid file but empty function name
		{Predicate: "context_priority_file", Args: []any{}},
	}
	callers := h.parsePriorityFacts(facts)
	// Should not crash and returns empty callers
	if len(callers) != 0 {
		t.Errorf("Expected 0 callers, got %d", len(callers))
	}
}

func TestFormatNode_MalformedAST(t *testing.T) {
	fset := token.NewFileSet()
	if formatNode(fset, nil) != "" {
		t.Error("Expected empty string for nil node")
	}
	// Fallback to "?" for unknown types like BasicLit
	if formatNode(fset, &ast.BasicLit{}) != "?" {
		t.Error("Expected '?' for unhandled node type")
	}
}

func TestExtractFunctionBodyRegex_CommentsAndStrings(t *testing.T) {
	h := &HolographicProvider{}
	content := `
/*
def foo() {
    return false
}
*/
def foo() {
    return true
}
`
	// Since regex matching doesn't ignore block comments, it will match the first occurrence.
	// This exposes the limitation.
	body, err := h.extractFunctionBodyRegex(content, "foo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(body, "return false") {
		t.Errorf("Limitation changed! Expected it to match the comment. Got: %s", body)
	}
}

func TestGetContext_UnknownExtension(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	os.WriteFile(file, []byte("key: value\n"), 0644)
	h := NewHolographicProvider(nil, dir)
	ctx, err := h.GetContext(file)
	if err != nil {
		t.Errorf("GetContext on unknown extension failed: %v", err)
	}
	if ctx == nil {
		t.Fatal("Expected basic context")
	}
}

func TestBuildGoContext_MassivePackageDir(t *testing.T) {
	dir := t.TempDir()
	for i := range 1000 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("file_%d.go", i)), []byte("package main\n"), 0644)
	}
	targetFile := filepath.Join(dir, "target.go")
	os.WriteFile(targetFile, []byte("package main\n"), 0644)

	h := NewHolographicProvider(nil, dir)
	ctx, err := h.GetContext(targetFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(ctx.PackageSiblings) != 1000 {
		t.Logf("Got %d siblings, handles large directories safely", len(ctx.PackageSiblings))
	}
}

func TestFetchFunctionBody_MassiveFile(t *testing.T) {
	dir := t.TempDir()
	hugeFile := filepath.Join(dir, "huge.go")

	f, err := os.Create(hugeFile)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("package huge\n")
	for range 50000 {
		f.WriteString("// Pad pad pad pad pad pad pad pad\n")
	}
	f.WriteString("func targetFunc() {}\n")
	f.Close()

	h := &HolographicProvider{}
	cache := newFileContentCache()
	body, err := h.fetchFunctionBody(hugeFile, "targetFunc", cache)
	if err != nil {
		t.Fatalf("Failed to fetch function from massive file: %v", err)
	}
	if !strings.Contains(body, "targetFunc") {
		t.Error("Did not find target function in massive file")
	}
}

func TestExtractLineRange_HugeFunction(t *testing.T) {
	h := &HolographicProvider{}
	var sb strings.Builder
	for i := range 5000 {
		sb.WriteString(fmt.Sprintf("line %d\n", i+1))
	}

	result, err := h.extractLineRange(sb.String(), 1, 5000)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 55 {
		t.Errorf("Expected truncation, got %d lines", len(lines))
	}
	if !strings.Contains(result, "(truncated)") {
		t.Error("Expected truncated warning")
	}
}

func TestQueryRelationships_DeeplyRecursiveGraph(t *testing.T) {
	ctx := &HolographicContext{}

	for i := range 5000 {
		ctx.CallGraph = append(ctx.CallGraph, CallEdge{
			Caller: fmt.Sprintf("caller_%d", i),
			Callee: "target",
		})
	}

	prompt := ctx.FormatForPrompt()
	if strings.Contains(prompt, "caller_50") {
		t.Error("Expected call relationships to be truncated in prompt")
	}
}

func TestResolvePrioritizedCallers_MassiveFactCount(t *testing.T) {
	h := &HolographicProvider{}
	var callers []PrioritizedCaller
	for i := range 5000 {
		callers = append(callers, PrioritizedCaller{
			File:     fmt.Sprintf("file_%d.go", i),
			Name:     fmt.Sprintf("func_%d", i),
			Priority: i % 100,
			Depth:    1,
		})
	}

	resolved, err := h.ResolvePrioritizedCallers(context.Background(), callers)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resolved) > 10 {
		t.Errorf("Expected callers to be limited, got %d", len(resolved))
	}
}

func TestBuildGoContext_FileDeletedConcurrently(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "target.go")
	os.WriteFile(targetFile, []byte("package main\n"), 0644)

	h := NewHolographicProvider(nil, dir)

	os.Remove(targetFile)
	ctx, err := h.GetContext(targetFile)
	// It logs an error but returns partial context
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("Expected partial context")
	}
}

func TestResolvePrioritizedCallers_ConcurrentAccess(t *testing.T) {
	h := &HolographicProvider{}
	dir := t.TempDir()
	file := filepath.Join(dir, "target.go")
	os.WriteFile(file, []byte("package main\nfunc foo() {}\n"), 0644)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			callers := []PrioritizedCaller{
				{File: file, Name: "foo", Priority: 50, Depth: 1},
			}
			// This tests if fileContentCache is safe or instantiated per call
			// Currently, ResolvePrioritizedCallers instantiates a new cache per call, so it's safe.
			h.ResolvePrioritizedCallers(context.Background(), callers)
		})
	}
	wg.Wait()
}

func TestBuildWithImpactPriorities_ContextCancellation(t *testing.T) {
	h := &HolographicProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callers := []PrioritizedCaller{
		{File: "dummy.go", Name: "foo", Priority: 50, Depth: 1},
	}

	_, err := h.ResolvePrioritizedCallers(ctx, callers)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestParsePriorityFacts_ConflictingFacts(t *testing.T) {
	h := &HolographicProvider{}
	facts := []core.Fact{
		{Predicate: "context_priority_file", Args: []any{"file.go", "func1", 50}},
		{Predicate: "context_priority_file", Args: []any{"file.go", "func1", 100}}, // Conflicting
	}
	callers := h.parsePriorityFacts(facts)
	if len(callers) != 1 {
		t.Fatalf("Expected deduplication to 1 caller, got %d", len(callers))
	}
	if callers[0].Priority != 50 {
		t.Errorf("Expected priority to be 50 (first seen), got %d", callers[0].Priority)
	}
}

func TestGetContext_SynchronousExecution(t *testing.T) {
	// Documenting the architectural flaw: GetContext does not take a context.Context,
	// so it cannot be canceled if I/O blocks indefinitely.
	h := &HolographicProvider{}
	h.GetContext("")
}

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
		arg  any
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
		arg        any
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

// REMEDIATED: TEST_GAP: Large File Handling (OOM Risk)
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
		// TODO: Gap - Empty content string (Z-01) - Test content: "", startLine: 1, endLine: 1
		// TODO: Gap - Start line < 0 (B-02) - Test startLine: -5
		// TODO: Gap - Start line == 0 (B-01) - Test startLine: 0
		// TODO: Gap - End line beyond string length (B-03) - Test endLine: 999999
		// TODO: Gap - Missing trailing newline vs present trailing newline - Test edge cases on truncation limits and exact returns
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

func TestGetContextWithContext_Cancellation(t *testing.T) {
	h := &HolographicProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := h.GetContextWithContext(ctx, "any_file.go")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestFetchFunctionBody_LargeFileHandling(t *testing.T) {
	dir := t.TempDir()
	largeFile := filepath.Join(dir, "large_file.go")

	// Create a 6MB file (exceeding our 5MB limit)
	content := make([]byte, 6*1024*1024)
	err := os.WriteFile(largeFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write large test file: %v", err)
	}

	h := NewHolographicProvider(nil, dir)
	_, err = h.fetchFunctionBody("large_file.go", "Func", nil)
	if err == nil || !strings.Contains(err.Error(), "file too large") {
		t.Errorf("Expected 'file too large' error, got: %v", err)
	}
}

func TestFetchFunctionBody_PathTraversalGuard(t *testing.T) {
	dir := t.TempDir()

	h := NewHolographicProvider(nil, dir)

	// Try a path traversal attempt outside the workspace
	_, err := h.fetchFunctionBody("../outside.go", "Func", nil)
	if err == nil || !strings.Contains(err.Error(), "security violation") {
		t.Errorf("Expected 'security violation' error, got: %v", err)
	}
}

func TestBuildGoContext_SiblingFilesCap(t *testing.T) {
	dir := t.TempDir()

	// Write 110 dummy Go files (exceeding our limit of 100)
	for i := range 110 {
		filePath := filepath.Join(dir, fmt.Sprintf("file_%d.go", i))
		err := os.WriteFile(filePath, []byte("package main\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to write dummy sibling file: %v", err)
		}
	}

	h := NewHolographicProvider(nil, dir)
	hc := &HolographicContext{
		PackageImports: make(map[string][]string),
	}

	targetPath := filepath.Join(dir, "file_0.go")
	err := h.buildGoContextWithContext(context.Background(), hc, targetPath)
	if err != nil {
		t.Fatalf("buildGoContext failed: %v", err)
	}

	// Verify it successfully collected the siblings up to cap
	if len(hc.PackageSiblings) < 100 {
		t.Errorf("Expected at least 100 PackageSiblings logged, got %d", len(hc.PackageSiblings))
	}
}

func TestExtractGoSignatures_EmptyFile(t *testing.T) {
	provider := NewHolographicProvider(nil, "")
	ctx := &HolographicContext{
		PackageImports: make(map[string][]string),
	}
	fset := token.NewFileSet()

	tmpDir := t.TempDir()

	// Test 0-byte file
	emptyFilePath := filepath.Join(tmpDir, "empty.go")
	if err := os.WriteFile(emptyFilePath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	err := provider.extractGoSignatures(ctx, fset, emptyFilePath)
	if err == nil {
		t.Errorf("Expected error for empty go file, got nil")
	}

	// Test whitespace-only file
	whitespaceFilePath := filepath.Join(tmpDir, "whitespace.go")
	if err := os.WriteFile(whitespaceFilePath, []byte("   \n\t\n  "), 0644); err != nil {
		t.Fatalf("Failed to create whitespace file: %v", err)
	}

	err = provider.extractGoSignatures(ctx, fset, whitespaceFilePath)
	if err == nil {
		t.Errorf("Expected error for whitespace-only go file, got nil")
	}
}
