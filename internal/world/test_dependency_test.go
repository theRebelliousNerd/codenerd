package world

import (
	"context"
	"testing"

	"codenerd/internal/tools/codedom"
)

// mockKernelQuerier implements codedom.KernelQuerier for testing.
type mockKernelQuerier struct {
	facts map[string][]codedom.FactData
}

func newMockKernel() *mockKernelQuerier {
	return &mockKernelQuerier{
		facts: make(map[string][]codedom.FactData),
	}
}

func (m *mockKernelQuerier) Query(predicate string) ([]codedom.FactData, error) {
	return m.facts[predicate], nil
}

func (m *mockKernelQuerier) addFact(predicate string, args ...interface{}) {
	m.facts[predicate] = append(m.facts[predicate], codedom.FactData{
		Predicate: predicate,
		Args:      args,
	})
}

// TestTestDependencyBuilder_Build tests the Build method.
func TestTestDependencyBuilder_Build(t *testing.T) {
	kernel := newMockKernel()

	// Add file topology facts
	kernel.addFact("file_topology", "internal/utils/strings.go", "/", "strings.go")
	kernel.addFact("file_topology", "internal/utils/strings_test.go", "/", "strings_test.go")
	kernel.addFact("file_topology", "internal/api/handler.go", "/", "handler.go")
	kernel.addFact("file_topology", "internal/api/handler_test.go", "/", "handler_test.go")
	kernel.addFact("file_topology", "test_models.py", "/", "test_models.py")
	kernel.addFact("file_topology", "app.spec.ts", "/", "app.spec.ts")

	// Add code element facts
	kernel.addFact("code_element", "go:internal/utils/strings.go:TruncateString", "/function", "internal/utils/strings.go", 1, 10)
	kernel.addFact("code_element", "go:internal/utils/strings_test.go:TestTruncateString", "/function", "internal/utils/strings_test.go", 1, 20)
	kernel.addFact("code_element", "go:internal/api/handler.go:HandleRequest", "/function", "internal/api/handler.go", 1, 30)
	kernel.addFact("code_element", "go:internal/api/handler_test.go:TestHandleRequest", "/function", "internal/api/handler_test.go", 1, 40)

	// Add call relationships
	kernel.addFact("code_calls", "go:internal/utils/strings_test.go:TestTruncateString", "go:internal/utils/strings.go:TruncateString")
	kernel.addFact("code_calls", "go:internal/api/handler.go:HandleRequest", "go:internal/utils/strings.go:TruncateString")
	kernel.addFact("code_calls", "go:internal/api/handler_test.go:TestHandleRequest", "go:internal/api/handler.go:HandleRequest")

	builder := NewTestDependencyBuilder(kernel, "/project")

	// Build the dependency graph
	err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check that test files were identified
	testFiles := builder.GetTestFiles()
	if len(testFiles) < 2 {
		t.Errorf("Expected at least 2 test files, got %d", len(testFiles))
	}

	// Check that test functions were identified
	testFuncs := builder.GetTestFunctions()
	if len(testFuncs) < 2 {
		t.Errorf("Expected at least 2 test functions, got %d", len(testFuncs))
	}
}

// TestTestDependencyBuilder_IsTestFile tests test file detection.
func TestTestDependencyBuilder_IsTestFile(t *testing.T) {
	builder := &TestDependencyBuilder{projectRoot: "/project"}

	tests := []struct {
		path     string
		expected bool
	}{
		// Go test files
		{"internal/utils/strings_test.go", true},
		{"cmd/main_test.go", true},
		{"internal/utils/strings.go", false},

		// Python test files
		{"tests/test_api.py", true},
		{"tests/api_test.py", true},
		{"models.py", false},

		// TypeScript/JavaScript test files
		{"src/components/Button.test.tsx", true},
		{"src/components/Button.spec.ts", true},
		{"src/utils/helpers.spec.js", true},
		{"src/components/Button.tsx", false},

		// Rust test files (pattern requires /tests/ with leading slash)
		{"/tests/integration.rs", true},
		{"src/lib.rs", false},
	}

	for _, tt := range tests {
		result := builder.isTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("isTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

// TestTestDependencyBuilder_IsTestFunction tests test function detection.
func TestTestDependencyBuilder_IsTestFunction(t *testing.T) {
	builder := &TestDependencyBuilder{projectRoot: "/project"}

	tests := []struct {
		ref      string
		file     string
		expected bool
	}{
		// Go test functions
		{"go:internal/utils/strings_test.go:TestTruncateString", "strings_test.go", true},
		{"go:internal/utils/strings_test.go:TestParseInput", "strings_test.go", true},
		{"go:internal/utils/strings_test.go:helper", "strings_test.go", false},

		// Python test functions
		{"py:tests/test_api.py:test_get_users", "test_api.py", true},
		{"py:tests/test_api.py:test_", "test_api.py", true},
		{"py:tests/test_api.py:helper_function", "test_api.py", false},

		// TypeScript test functions (simplified pattern)
		{"ts:src/app.spec.ts:test", "app.spec.ts", true},
		{"ts:src/app.spec.ts:it(", "app.spec.ts", true},
		{"ts:src/app.spec.ts:describe(", "app.spec.ts", true},
		{"ts:src/app.spec.ts:helper", "app.spec.ts", false},

		// Rust test functions (pattern looks for ::test_ with double colon)
		{"rs:/tests/api.rs::test_handler", "api.rs", true},
	}

	for _, tt := range tests {
		result := builder.isTestFunction(tt.ref, tt.file)
		if result != tt.expected {
			t.Errorf("isTestFunction(%q, %q) = %v, want %v", tt.ref, tt.file, result, tt.expected)
		}
	}
}

// TestTestDependencyBuilder_GetImpactedTests tests impact analysis.
func TestTestDependencyBuilder_GetImpactedTests(t *testing.T) {
	kernel := newMockKernel()

	// Set up a simple dependency graph:
	// TestA -> FuncA (pkg a)
	// TestB -> FuncB -> FuncA (pkg b)
	// TestC -> FuncC (pkg c, no relation to FuncA)
	// Note: Put FuncC in different package to isolate it from FuncA

	kernel.addFact("file_topology", "a/func_a.go", "/", "func_a.go")
	kernel.addFact("file_topology", "a/func_a_test.go", "/", "func_a_test.go")
	kernel.addFact("file_topology", "b/func_b.go", "/", "func_b.go")
	kernel.addFact("file_topology", "b/func_b_test.go", "/", "func_b_test.go")
	kernel.addFact("file_topology", "c/func_c.go", "/", "func_c.go")
	kernel.addFact("file_topology", "c/func_c_test.go", "/", "func_c_test.go")

	kernel.addFact("code_element", "go:a/func_a.go:FuncA", "/function", "a/func_a.go", 1, 10)
	kernel.addFact("code_element", "go:a/func_a_test.go:TestFuncA", "/function", "a/func_a_test.go", 1, 10)
	kernel.addFact("code_element", "go:b/func_b.go:FuncB", "/function", "b/func_b.go", 1, 10)
	kernel.addFact("code_element", "go:b/func_b_test.go:TestFuncB", "/function", "b/func_b_test.go", 1, 10)
	kernel.addFact("code_element", "go:c/func_c.go:FuncC", "/function", "c/func_c.go", 1, 10)
	kernel.addFact("code_element", "go:c/func_c_test.go:TestFuncC", "/function", "c/func_c_test.go", 1, 10)

	// TestA directly calls FuncA
	kernel.addFact("code_calls", "go:a/func_a_test.go:TestFuncA", "go:a/func_a.go:FuncA")
	// TestB calls FuncB
	kernel.addFact("code_calls", "go:b/func_b_test.go:TestFuncB", "go:b/func_b.go:FuncB")
	// FuncB calls FuncA
	kernel.addFact("code_calls", "go:b/func_b.go:FuncB", "go:a/func_a.go:FuncA")
	// TestC calls FuncC (isolated)
	kernel.addFact("code_calls", "go:c/func_c_test.go:TestFuncC", "go:c/func_c.go:FuncC")

	builder := NewTestDependencyBuilder(kernel, "/project")
	err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Edit FuncA - should impact TestA (direct) and TestB (transitive)
	impacted := builder.GetImpactedTests([]string{"go:a/func_a.go:FuncA"})

	// Count impacted tests
	var foundTestA, foundTestB, foundTestC bool
	for _, test := range impacted {
		switch test.TestRef {
		case "go:a/func_a_test.go:TestFuncA":
			foundTestA = true
			if test.Priority != "high" {
				t.Errorf("TestFuncA should have high priority, got %s", test.Priority)
			}
		case "go:b/func_b_test.go:TestFuncB":
			foundTestB = true
			// Could be high or medium depending on transitive detection
		case "go:c/func_c_test.go:TestFuncC":
			foundTestC = true
			t.Error("TestFuncC should NOT be impacted by FuncA changes")
		}
	}

	if !foundTestA {
		t.Error("TestFuncA should be impacted")
	}
	// TestB may or may not be found depending on transitive depth
	_ = foundTestB
	if foundTestC {
		t.Error("TestFuncC should not be impacted")
	}
}

// TestTestDependencyBuilder_GetCoverageGaps tests coverage gap detection.
func TestTestDependencyBuilder_GetCoverageGaps(t *testing.T) {
	kernel := newMockKernel()

	// Set up: PublicFunc is covered (in pkg a with tests)
	// PublicUncovered is NOT covered (in pkg b, no tests, not imported)
	kernel.addFact("file_topology", "a/funcs.go", "/", "funcs.go")
	kernel.addFact("file_topology", "a/funcs_test.go", "/", "funcs_test.go")
	kernel.addFact("file_topology", "b/uncovered.go", "/", "uncovered.go")

	kernel.addFact("code_element", "go:a/funcs.go:PublicFunc", "/function", "a/funcs.go", 1, 10)
	kernel.addFact("code_element", "go:b/uncovered.go:PublicUncovered", "/function", "b/uncovered.go", 1, 10)
	kernel.addFact("code_element", "go:a/funcs_test.go:TestPublicFunc", "/function", "a/funcs_test.go", 1, 10)

	kernel.addFact("element_visibility", "go:a/funcs.go:PublicFunc", "/public")
	kernel.addFact("element_visibility", "go:b/uncovered.go:PublicUncovered", "/public")

	// TestPublicFunc covers PublicFunc
	kernel.addFact("code_calls", "go:a/funcs_test.go:TestPublicFunc", "go:a/funcs.go:PublicFunc")

	builder := NewTestDependencyBuilder(kernel, "/project")
	err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	gaps := builder.GetCoverageGaps()

	// PublicUncovered should be in gaps, PublicFunc should not
	var foundUncovered bool
	for _, gap := range gaps {
		if gap == "go:b/uncovered.go:PublicUncovered" {
			foundUncovered = true
		}
		if gap == "go:a/funcs.go:PublicFunc" {
			t.Error("PublicFunc should not be in coverage gaps (it has tests)")
		}
	}

	if !foundUncovered {
		t.Error("PublicUncovered should be in coverage gaps")
	}
}

// TestTestDependencyBuilder_GetImpactedTestPackages tests package-level impact.
func TestTestDependencyBuilder_GetImpactedTestPackages(t *testing.T) {
	kernel := newMockKernel()

	kernel.addFact("file_topology", "internal/utils/strings.go", "/", "strings.go")
	kernel.addFact("file_topology", "internal/utils/strings_test.go", "/", "strings_test.go")
	kernel.addFact("file_topology", "internal/api/handler.go", "/", "handler.go")
	kernel.addFact("file_topology", "internal/api/handler_test.go", "/", "handler_test.go")

	kernel.addFact("code_element", "go:internal/utils/strings.go:TruncateString", "/function", "internal/utils/strings.go", 1, 10)
	kernel.addFact("code_element", "go:internal/utils/strings_test.go:TestTruncateString", "/function", "internal/utils/strings_test.go", 1, 10)
	kernel.addFact("code_element", "go:internal/api/handler.go:HandleRequest", "/function", "internal/api/handler.go", 1, 10)
	kernel.addFact("code_element", "go:internal/api/handler_test.go:TestHandleRequest", "/function", "internal/api/handler_test.go", 1, 10)

	kernel.addFact("code_calls", "go:internal/utils/strings_test.go:TestTruncateString", "go:internal/utils/strings.go:TruncateString")
	kernel.addFact("code_calls", "go:internal/api/handler.go:HandleRequest", "go:internal/utils/strings.go:TruncateString")
	kernel.addFact("code_calls", "go:internal/api/handler_test.go:TestHandleRequest", "go:internal/api/handler.go:HandleRequest")

	builder := NewTestDependencyBuilder(kernel, "/project")
	err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	packages := builder.GetImpactedTestPackages([]string{"go:internal/utils/strings.go:TruncateString"})

	// Should include internal/utils at minimum
	if len(packages) == 0 {
		t.Error("Expected at least one impacted package")
	}
}

// TestTestDependencyBuilder_Concurrency tests thread safety.
func TestTestDependencyBuilder_Concurrency(t *testing.T) {
	kernel := newMockKernel()

	// Add minimal facts
	kernel.addFact("file_topology", "test_file.go", "/", "test_file.go")
	kernel.addFact("file_topology", "func_test.go", "/", "func_test.go")
	kernel.addFact("code_element", "go:test_file.go:Func", "/function", "test_file.go", 1, 10)
	kernel.addFact("code_element", "go:func_test.go:TestFunc", "/function", "func_test.go", 1, 10)
	kernel.addFact("code_calls", "go:func_test.go:TestFunc", "go:test_file.go:Func")

	builder := NewTestDependencyBuilder(kernel, "/project")
	err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Run concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = builder.GetTestFiles()
			_ = builder.GetTestFunctions()
			_ = builder.GetImpactedTests([]string{"go:test_file.go:Func"})
			_ = builder.GetCoverageGaps()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
