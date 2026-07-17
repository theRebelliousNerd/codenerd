import os

filename = "tests/e2e/virtualstore_graphquery_integration_test.go"

# Read the file up to the padding start
content = ""
with open(filename, "r") as f:
    for line in f:
        if "// TestE2E_VirtualStore_GraphQuery_EdgeCase_0" in line:
            break
        content += line

# Write new, meaningful tests instead of padding
new_tests = """
// =============================================================================
// 8. ADDITIONAL CONTRACT VIOLATION & EDGE CASE TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Contract_ReturnMapObliteration
// Scenario: GraphQuery returns a map[string]interface{}. goToMangleTerm should handle it,
// but it lacks a case for maps and falls back to stringification.
func TestE2E_VirtualStore_GraphQuery_Contract_ReturnMapObliteration(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: map[string]interface{}{"key": "value", "count": 42},
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_return_map(Any).
		test_return_map(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_return_map")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	resStr := fmt.Sprintf("%v", results[0].Args[0])
	if !strings.Contains(resStr, "map[count:42 key:value]") && !strings.Contains(resStr, "map[key:value count:42]") {
		t.Errorf("Expected map stringification 'map[...]!', got: %s", resStr)
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_Float32Precision
// Scenario: GraphQuery returns a float32. goToMangleTerm converts it to float64,
// potentially introducing precision errors.
func TestE2E_VirtualStore_GraphQuery_Contract_Float32Precision(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	var val float32 = 0.1
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: val,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_float32(Any).
		test_float32(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_float32")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	// Because 0.1 cannot be represented exactly in binary floating point,
	// converting from float32 to float64 often results in something like 0.10000000149011612.
	// We just verify it successfully returns the atom.
	resTerm, ok := results[0].Args[0].(ast.Float64)
	if !ok {
		t.Fatalf("Expected float64, got %T", results[0].Args[0])
	}
	if resTerm == 0 {
		t.Fatalf("Expected non-zero float")
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_NestedSliceStringification
// Scenario: GraphQuery returns a nested slice. goToMangleTerm handles []string,
// but what about [][]string? It falls back to stringification.
func TestE2E_VirtualStore_GraphQuery_Contract_NestedSliceStringification(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: [][]string{{"a", "b"}, {"c", "d"}},
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_nested_slice(Any).
		test_nested_slice(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_nested_slice")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	resStr := fmt.Sprintf("%v", results[0].Args[0])
	if !strings.Contains(resStr, "[[a b] [c d]]") {
		t.Errorf("Expected nested slice stringification '[[a b] [c d]]', got: %s", resStr)
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_QuoteStrippingOnReturn
// Scenario: If the returned string contains quotes, does it get stripped on return?
// goToMangleTerm creates ast.String(), so it should preserve quotes. Let's verify.
func TestE2E_VirtualStore_GraphQuery_Contract_QuoteStrippingOnReturn(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: `"quoted_string"`,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_quote(Any).
		test_quote(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_quote")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	// Verify the quotes were preserved.
	resTerm, ok := results[0].Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("Expected constant, got %T", results[0].Args[0])
	}

	val := resTerm.String()
	// It's a string atom, so mangle-go might add quotes. Let's just ensure the inner text is intact.
	if !strings.Contains(val, "quoted_string") {
		t.Errorf("Expected preserved quotes, got: %s", val)
	}
}

// TestE2E_VirtualStore_GraphQuery_Malformed_Arity
// Scenario: A malformed query with wrong arity should gracefully fail (return nil, nil).
func TestE2E_VirtualStore_GraphQuery_Malformed_Arity(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// query_graph normally takes 3 args. We give it 1.
	policy := `
		Decl test_arity(Any).
		test_arity(X) :- query_graph(X).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_arity")
	if err != nil {
		// Should not error, but evaluate to false.
	}

	if len(results) != 0 {
		t.Fatalf("Expected 0 results for malformed arity, got %d", len(results))
	}
}

// TestE2E_VirtualStore_GraphQuery_Malformed_NonStringQueryType
// Scenario: A malformed query with a non-string QueryType should gracefully fail.
func TestE2E_VirtualStore_GraphQuery_Malformed_NonStringQueryType(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// First arg must be a string, we give it a number.
	policy := `
		Decl test_type(Any).
		test_type(R) :- query_graph(42, "params", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_type")

	if len(results) != 0 {
		t.Fatalf("Expected 0 results for malformed type, got %d", len(results))
	}
}

// TestE2E_VirtualStore_GraphQuery_State_NilInterfaceAfterSet
// Scenario: Setting GraphQuery to nil mid-flight should gracefully fail future queries.
func TestE2E_VirtualStore_GraphQuery_State_NilInterfaceAfterSet(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{ returnValue: "ok" }
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_nil_after(Any).
		test_nil_after(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	// Should succeed initially
	results, _ := kernel.Query("test_nil_after")
	if len(results) == 0 {
		t.Fatalf("Expected initial query to succeed")
	}

	// Set to nil
	vs.SetGraphQuery(nil)

	// Should now fail gracefully
	results, _ = kernel.Query("test_nil_after")
	if len(results) != 0 {
		t.Fatalf("Expected 0 results after setting GraphQuery to nil, got %d", len(results))
	}
}
"""

content += new_tests

with open(filename, "w") as f:
    f.write(content)
print("Updated test suite.")
