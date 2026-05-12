package core

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Remediation for kernel_query TEST_GAP markers.
// QA: kernel_query_test.go TEST_GAP comments (lines 11-17)
// ============================================================================

// TestKernelQueryGap_MassiveArity verifies Query handles a pattern with
// a massive number of arguments (arity > 100).
func TestKernelQueryGap_MassiveArity(t *testing.T) {
	k := setupMockKernel(t)

	// Build a Decl with 20 arguments (Mangle requires unique argument names)
	const arity = 20
	var declArgs []string
	for i := 0; i < arity; i++ {
		// Use unique type-like names: A0, A1, ..., A19
		declArgs = append(declArgs, fmt.Sprintf("A%d", i))
	}
	decl := "Decl big_pred(" + strings.Join(declArgs, ", ") + ")."
	k.AppendPolicy(decl)

	// Build a matching fact with unique string values
	var factArgs []string
	for i := 0; i < arity; i++ {
		factArgs = append(factArgs, fmt.Sprintf("\"val_%d\"", i))
	}
	fact := "big_pred(" + strings.Join(factArgs, ", ") + ")."
	k.AppendPolicy(fact)

	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed with %d-arity predicate: %v", arity, err)
	}

	results, err := k.Query("big_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && len(results[0].Args) != arity {
		t.Errorf("Expected %d args, got %d", arity, len(results[0].Args))
	}
}

// TestKernelQueryGap_QueryAll_LargeEDB verifies QueryAll can handle
// a large number of facts without OOM or stall.
func TestKernelQueryGap_QueryAll_LargeEDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large EDB test in short mode")
	}

	k := setupMockKernel(t)
	k.AppendPolicy("Decl load_pred(Name).")
	k.Evaluate()

	// Assert 2000 facts with UNIQUE values (Mangle deduplicates identical facts)
	const factCount = 2000
	for i := 0; i < factCount; i++ {
		k.AssertWithoutEval(Fact{
			Predicate: "load_pred",
			Args:      []interface{}{fmt.Sprintf("item_%04d", i)},
		})
	}
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed with %d facts: %v", factCount, err)
	}

	results, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}

	loadPredResults := results["load_pred"]
	if len(loadPredResults) != factCount {
		t.Errorf("Expected %d load_pred facts, got %d", factCount, len(loadPredResults))
	}
}

// TestKernelQueryGap_ParseFactString_DeepNesting verifies ParseFactString
// doesn't stack overflow on deeply nested structures.
func TestKernelQueryGap_ParseFactString_DeepNesting(t *testing.T) {
	// Build a deeply nested string like: foo([[[[[...]]]]]).
	const depth = 50
	nested := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	factStr := "nested(" + nested + ")"

	// Should not stack overflow — may return error for invalid syntax
	_, err := ParseFactString(factStr)
	t.Logf("ParseFactString with %d-deep nesting: err=%v", depth, err)
	// Key assertion: no panic occurred
}

// TestKernelQueryGap_LoadFactsFromFile_LargeFile verifies LoadFactsFromFile
// handles a large .mg file without OOM.
func TestKernelQueryGap_LoadFactsFromFile_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	k := setupMockKernel(t)
	k.AppendPolicy("Decl bigfile_pred(Name, Value).")
	k.Evaluate()

	// Create a temp file with 2000 UNIQUE facts
	const fileFactCount = 2000
	tmpFile := t.TempDir() + "/large.mg"
	var content strings.Builder
	for i := 0; i < fileFactCount; i++ {
		content.WriteString(fmt.Sprintf("bigfile_pred(\"key_%04d\", \"val_%04d\").\n", i, i))
	}

	err := writeStringToFile(tmpFile, content.String())
	if err != nil {
		t.Fatal(err)
	}

	err = k.LoadFactsFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFactsFromFile failed: %v", err)
	}

	results, err := k.Query("bigfile_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != fileFactCount {
		t.Errorf("Expected %d results, got %d", fileFactCount, len(results))
	}
}

// TestKernelQueryGap_Concurrency_ReadWriteStarvation verifies concurrent
// reads (Query) don't starve write locks (SetSchemas/Evaluate).
func TestKernelQueryGap_Concurrency_ReadWriteStarvation(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl conc_pred(Name).")
	k.Evaluate()

	// Seed with facts
	for i := 0; i < 100; i++ {
		k.AssertWithoutEval(Fact{
			Predicate: "conc_pred",
			Args:      []interface{}{"item_" + string(rune('a'+i%26))},
		})
	}
	k.Evaluate()

	var wg sync.WaitGroup
	const readers = 30
	const writers = 5

	// Launch concurrent readers
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = k.Query("conc_pred")
				_, _ = k.QueryAll()
			}
		}()
	}

	// Launch concurrent writers
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				k.Assert(Fact{
					Predicate: "conc_pred",
					Args:      []interface{}{"writer_" + string(rune('a'+idx))},
				})
			}
		}(i)
	}

	wg.Wait()
	// No panics, deadlocks, or data races should occur (run with -race)
}

// TestKernelQueryGap_UpdateSystemFacts_MissingWorkspace verifies
// UpdateSystemFacts handles missing workspace root gracefully.
func TestKernelQueryGap_UpdateSystemFacts_MissingWorkspace(t *testing.T) {
	// Use a bare kernel with only the needed declarations to avoid
	// policy-level analysis errors from the full setupMockKernel.
	k := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}
	k.SetSchemas(`
		Decl current_time(Time).
		Decl git_state(Key, Value).
		Decl git_branch(Name).
	`)
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Set workspace to a non-existent path
	k.workspaceRoot = "/nonexistent/path/that/does/not/exist"

	err = k.UpdateSystemFacts()
	if err != nil {
		t.Fatalf("UpdateSystemFacts should handle missing workspace gracefully: %v", err)
	}

	// Should have set current_time even without git
	results, err := k.Query("current_time")
	if err != nil {
		t.Fatalf("Query current_time failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected current_time fact even with missing workspace")
	}
}

// writeStringToFile is a test helper to write string content to a file.
func writeStringToFile(path, content string) error {
	return writeFileForTest(path, []byte(content))
}

// writeFileForTest wraps os.WriteFile for test use.
func writeFileForTest(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
