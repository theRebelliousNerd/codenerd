package world

import (
	"codenerd/internal/core"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDataFlowExtractor_BasicAssignments(t *testing.T) {
	// TODO: TEST_GAP: TOCTOU Race Condition
	// ExtractDataFlow performs parser.ParseFile (reading V1) then os.ReadFile (reading V2).
	// We need a test that modifies the file content between these two operations (e.g., via a hooked reader or race condition simulation)
	// to verify that inconsistencies are handled or that the function is refactored to read once.

	// Create a temporary Go file with test patterns
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_assigns.go")

	testCode := `package test

import "os"

func example() {
	// Nullable assignment from New* function
	f, err := os.Open("file.txt")
	if err != nil {
		return
	}
	defer f.Close()

	// Nullable from explicit nil
	var ptr *int = nil
	_ = ptr
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	// Verify we got some facts
	if len(facts) == 0 {
		t.Fatal("Expected facts to be extracted")
	}

	// Check for specific fact types
	var hasAssigns, hasErrorChecked, hasFunctionScope bool
	for _, fact := range facts {
		switch fact.Predicate {
		case "assigns":
			hasAssigns = true
		case "error_checked_return":
			hasErrorChecked = true
		case "function_scope":
			hasFunctionScope = true
		}
	}

	if !hasAssigns {
		t.Error("Expected assigns facts")
	}
	if !hasErrorChecked {
		t.Error("Expected error_checked_return facts")
	}
	if !hasFunctionScope {
		t.Error("Expected function_scope facts")
	}
}

func TestDataFlowExtractor_NilGuards(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_guards.go")

	testCode := `package test

func withNilGuard(x *int) int {
	// Early return pattern
	if x == nil {
		return 0
	}
	// x is guaranteed non-nil here
	return *x
}

func withNilBlock(x *int) int {
	// Block guard pattern
	if x != nil {
		return *x
	}
	return 0
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	var guardsReturn, guardsBlock int
	for _, fact := range facts {
		switch fact.Predicate {
		case "guards_return":
			guardsReturn++
		case "guards_block":
			guardsBlock++
		}
	}

	if guardsReturn == 0 {
		t.Error("Expected guards_return facts for early return pattern")
	}
	if guardsBlock == 0 {
		t.Error("Expected guards_block facts for block guard pattern")
	}
}

func TestDataFlowExtractor_ErrorChecks(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_errors.go")

	testCode := `package test

import "errors"

func withErrorCheck() error {
	err := doSomething()
	if err != nil {
		return err
	}
	return nil
}

func withErrorBlock() {
	err := doSomething()
	if err != nil {
		handleError(err)
	}
}

func doSomething() error {
	return errors.New("test")
}

func handleError(err error) {}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	var errorReturn, errorBlock int
	for _, fact := range facts {
		switch fact.Predicate {
		case "error_checked_return":
			errorReturn++
		case "error_checked_block":
			errorBlock++
		}
	}

	if errorReturn == 0 {
		t.Error("Expected error_checked_return facts")
	}
	if errorBlock == 0 {
		t.Error("Expected error_checked_block facts")
	}
}

func TestDataFlowExtractor_Uses(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_uses.go")

	testCode := `package test

type Foo struct {
	value int
}

func (f *Foo) GetValue() int {
	return f.value
}

func usePointer(foo *Foo) {
	// Method call on pointer
	foo.GetValue()

	// Field access
	_ = foo.value

	// Dereference
	_ = *foo
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	var usesCount int
	for _, fact := range facts {
		if fact.Predicate == "uses" {
			usesCount++
		}
	}

	if usesCount == 0 {
		t.Error("Expected uses facts for method calls and dereferences")
	}
}

func TestDataFlowExtractor_CallArgs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_call_args.go")

	testCode := `package test

func caller() {
	x := 1
	y := 2
	z := 3

	// Multiple arguments
	process(x, y, z)

	// Pointer argument
	processPtr(&x)
}

func process(a, b, c int) {}
func processPtr(p *int) {}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	var callArgCount int
	for _, fact := range facts {
		if fact.Predicate == "call_arg" {
			callArgCount++
		}
	}

	// Should have at least 4 call_arg facts (x, y, z for process, and &x for processPtr)
	if callArgCount < 4 {
		t.Errorf("Expected at least 4 call_arg facts, got %d", callArgCount)
	}
}

func TestDataFlowExtractor_SkipsNonGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("not go code"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow should not error on non-Go file: %v", err)
	}

	if facts != nil && len(facts) > 0 {
		t.Error("Expected no facts for non-Go file")
	}
}

func TestDataFlowExtractor_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a few test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	testFile := filepath.Join(tmpDir, "file1_test.go") // Should be skipped

	code1 := `package test
func foo() { x := 1; _ = x }
`
	code2 := `package test
func bar() { y := 2; _ = y }
`
	testCode := `package test
func TestFoo(t *testing.T) {}
`

	if err := os.WriteFile(file1, []byte(code1), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(code2), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlowForDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ExtractDataFlowForDirectory failed: %v", err)
	}

	// Should have facts from file1 and file2, but not from test file
	if len(facts) == 0 {
		t.Error("Expected facts from directory")
	}

	// Verify function_scope facts for both foo and bar
	var hasFoo, hasBar bool
	for _, fact := range facts {
		if fact.Predicate == "function_scope" && len(fact.Args) >= 2 {
			if funcName, ok := fact.Args[1].(core.MangleAtom); ok {
				if string(funcName) == "/foo" {
					hasFoo = true
				}
				if string(funcName) == "/bar" {
					hasBar = true
				}
			}
		}
	}

	if !hasFoo {
		t.Error("Expected function_scope for foo")
	}
	if !hasBar {
		t.Error("Expected function_scope for bar")
	}
}

func TestDataFlowExtractor_GuardDominates(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_dominates.go")

	testCode := `package test

func withEarlyReturn(x *int) int {
	if x == nil {
		return 0
	}
	// All these lines are dominated by the guard
	a := *x
	b := a + 1
	return b
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	var hasGuardDominates bool
	for _, fact := range facts {
		if fact.Predicate == "guard_dominates" {
			hasGuardDominates = true
			// Verify it has the right structure
			if len(fact.Args) != 4 {
				t.Errorf("guard_dominates should have 4 args, got %d", len(fact.Args))
			}
		}
	}

	if !hasGuardDominates {
		t.Error("Expected guard_dominates fact for early return pattern")
	}
}

func TestSummarizeDataFlow(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "assigns", Args: []interface{}{"/x", core.MangleAtom("/nullable"), "test.go", int64(1)}},
		{Predicate: "assigns", Args: []interface{}{"/err", core.MangleAtom("/error"), "test.go", int64(2)}},
		{Predicate: "guards_block", Args: []interface{}{"/x", "/nil_check", "test.go", int64(3), int64(5)}},
		{Predicate: "guards_return", Args: []interface{}{"/y", "/nil_check", "test.go", int64(6)}},
		{Predicate: "error_checked_return", Args: []interface{}{"/err", "test.go", int64(7)}},
		{Predicate: "uses", Args: []interface{}{"test.go", "/foo", "/x", int64(8)}},
		{Predicate: "call_arg", Args: []interface{}{"/callsite", int64(0), "/x", "test.go", int64(9)}},
		{Predicate: "function_scope", Args: []interface{}{"test.go", "/foo", int64(1), int64(10)}},
		{Predicate: "guard_dominates", Args: []interface{}{"test.go", "/foo", int64(4), int64(10)}},
	}

	summary := SummarizeDataFlow(facts)

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"TotalFacts", summary.TotalFacts, 9},
		{"AssignmentsFacts", summary.AssignmentsFacts, 2},
		{"NullableAssignments", summary.NullableAssignments, 1},
		{"ErrorAssignments", summary.ErrorAssignments, 1},
		{"GuardsBlockFacts", summary.GuardsBlockFacts, 1},
		{"GuardsReturnFacts", summary.GuardsReturnFacts, 1},
		{"ErrorCheckedFacts", summary.ErrorCheckedFacts, 1},
		{"UsesFacts", summary.UsesFacts, 1},
		{"CallArgFacts", summary.CallArgFacts, 1},
		{"FunctionScopeFacts", summary.FunctionScopeFacts, 1},
		{"GuardDominatesFacts", summary.GuardDominatesFacts, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.got); diff != "" {
				t.Errorf("%s mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestDataFlowExtractor_ComplexFunction(t *testing.T) {
	// TODO: TEST_GAP: Recursion/Stack Overflow
	// DataFlowExtractor uses ast.Inspect which is recursive.
	// We need a test case with deeply nested structures (e.g., 10k nested ifs)
	// to ensure the extractor doesn't crash the agent with a stack overflow.

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_complex.go")

	// A more realistic function with multiple patterns
	testCode := `package test

import (
	"io"
	"os"
)

type Config struct {
	Path string
}

func LoadConfig(path string) (*Config, error) {
	// Multiple assignment patterns
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, readErr := io.ReadAll(f)
	if readErr != nil {
		return nil, readErr
	}

	cfg := &Config{Path: path}
	if cfg == nil {
		return nil, nil
	}

	// Use data
	processData(data)

	return cfg, nil
}

func processData(data []byte) {}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	extractor := NewDataFlowExtractor()
	facts, err := extractor.ExtractDataFlow(testFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	summary := SummarizeDataFlow(facts)

	// Verify we extracted meaningful data
	if summary.AssignmentsFacts < 3 {
		t.Errorf("Expected at least 3 assignment facts, got %d", summary.AssignmentsFacts)
	}
	if summary.ErrorCheckedFacts < 2 {
		t.Errorf("Expected at least 2 error check facts, got %d", summary.ErrorCheckedFacts)
	}
	if summary.FunctionScopeFacts < 2 {
		t.Errorf("Expected at least 2 function scope facts (LoadConfig and processData), got %d", summary.FunctionScopeFacts)
	}
}
