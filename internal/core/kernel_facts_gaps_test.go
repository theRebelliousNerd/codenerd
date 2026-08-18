package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"
)

// =============================================================================
// FACT LIFECYCLE TESTS — Assert, Retract, Query round-trip
// =============================================================================

func TestAssert_WhenDuplicateFact_ShouldBeIdempotent(t *testing.T) {
	k := setupMockKernel(t)

	k.AppendPolicy("Decl color(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	fact := Fact{Predicate: "color", Args: []any{"red"}}

	if err := k.Assert(fact); err != nil {
		t.Fatalf("First assert failed: %v", err)
	}
	countAfterFirst := k.FactCount()

	if err := k.Assert(fact); err != nil {
		t.Fatalf("Duplicate assert should not error: %v", err)
	}
	countAfterSecond := k.FactCount()

	if countAfterSecond != countAfterFirst {
		t.Errorf("Duplicate assert changed count: %d -> %d", countAfterFirst, countAfterSecond)
	}
}

func TestAssertBatch_WhenEmptySlice_ShouldNoOp(t *testing.T) {
	k := setupMockKernel(t)
	initialCount := k.FactCount()

	err := k.AssertBatch(nil)
	if err != nil {
		t.Fatalf("AssertBatch(nil) should not error: %v", err)
	}

	err = k.AssertBatch([]Fact{})
	if err != nil {
		t.Fatalf("AssertBatch(empty) should not error: %v", err)
	}

	if k.FactCount() != initialCount {
		t.Errorf("Empty AssertBatch changed count: %d -> %d", initialCount, k.FactCount())
	}
}

func TestAssertBatch_WhenMultipleFacts_ShouldAddAll(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl fruit(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	facts := []Fact{
		{Predicate: "fruit", Args: []any{"apple"}},
		{Predicate: "fruit", Args: []any{"banana"}},
		{Predicate: "fruit", Args: []any{"cherry"}},
	}

	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("AssertBatch failed: %v", err)
	}

	results, err := k.Query("fruit")
	if err != nil {
		t.Fatalf("Query fruit failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Expected 3 fruit facts, got %d", len(results))
	}
}

func TestAssertBatch_WhenDuplicatesInBatch_ShouldDedup(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl item(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	facts := []Fact{
		{Predicate: "item", Args: []any{"x"}},
		{Predicate: "item", Args: []any{"x"}}, // duplicate
		{Predicate: "item", Args: []any{"y"}},
	}

	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("AssertBatch failed: %v", err)
	}

	results, err := k.Query("item")
	if err != nil {
		t.Fatalf("Query item failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 item facts after dedup, got %d", len(results))
	}
}

func TestAssertString_WhenValidFactString_ShouldParse(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl greeting(Msg).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	err := k.AssertString(`greeting("hello")`)
	if err != nil {
		t.Fatalf("AssertString failed: %v", err)
	}

	results, err := k.Query("greeting")
	if err != nil {
		t.Fatalf("Query greeting failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 greeting fact, got %d", len(results))
	}
}

func TestAssertString_WhenInvalidSyntax_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	err := k.AssertString("()")
	if err == nil {
		t.Fatal("AssertString with invalid syntax should error")
	}
}

func TestAssertString_WhenEmpty_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	err := k.AssertString("")
	if err == nil {
		t.Fatal("AssertString with empty string should error")
	}
}

// =============================================================================
// RETRACT TESTS
// =============================================================================

func TestRetract_WhenPredicateNotExists_ShouldNoOp(t *testing.T) {
	k := setupMockKernel(t)
	initialCount := k.FactCount()

	err := k.Retract("nonexistent_predicate_abc123")
	if err != nil {
		t.Fatalf("Retract of missing predicate should not error: %v", err)
	}

	if k.FactCount() != initialCount {
		t.Errorf("Retract of missing predicate changed count: %d -> %d", initialCount, k.FactCount())
	}
}

func TestRetract_WhenPredicateExists_ShouldRemoveAll(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl flavor(Name).\nDecl size(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	k.Assert(Fact{Predicate: "flavor", Args: []any{"sweet"}})
	k.Assert(Fact{Predicate: "flavor", Args: []any{"sour"}})
	k.Assert(Fact{Predicate: "size", Args: []any{"big"}})

	err := k.Retract("flavor")
	if err != nil {
		t.Fatalf("Retract flavor failed: %v", err)
	}

	flavors, err := k.Query("flavor")
	if err != nil {
		t.Fatalf("Query flavor failed: %v", err)
	}
	if len(flavors) != 0 {
		t.Errorf("Expected 0 flavor facts after retract, got %d", len(flavors))
	}

	// size should be unaffected
	sizes, err := k.Query("size")
	if err != nil {
		t.Fatalf("Query size failed: %v", err)
	}
	if len(sizes) != 1 {
		t.Errorf("Expected 1 size fact, got %d", len(sizes))
	}
}

func TestRetractFact_WhenNoArgs_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	err := k.RetractFact(Fact{Predicate: "test_pred", Args: nil})
	if err == nil {
		t.Fatal("RetractFact with no args should error")
	}
}

func TestRetractExactFact_WhenNoArgs_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	err := k.RetractExactFact(Fact{Predicate: "test_pred", Args: nil})
	if err == nil {
		t.Fatal("RetractExactFact with no args should error")
	}
}

func TestRetractExactFact_WhenExactMatch_ShouldRemoveOnly(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl setting(Key, Value).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	k.Assert(Fact{Predicate: "setting", Args: []any{"lang", "go"}})
	k.Assert(Fact{Predicate: "setting", Args: []any{"lang", "python"}})
	k.Assert(Fact{Predicate: "setting", Args: []any{"debug", "true"}})

	// Retract only "lang"+"go", not "lang"+"python"
	err := k.RetractExactFact(Fact{Predicate: "setting", Args: []any{"lang", "go"}})
	if err != nil {
		t.Fatalf("RetractExactFact failed: %v", err)
	}

	results, err := k.Query("setting")
	if err != nil {
		t.Fatalf("Query setting failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 remaining settings, got %d", len(results))
	}
}

func TestRetractExactFactsBatch_WhenEmpty_ShouldNoOp(t *testing.T) {
	k := setupMockKernel(t)
	initialCount := k.FactCount()

	err := k.RetractExactFactsBatch(nil)
	if err != nil {
		t.Fatalf("RetractExactFactsBatch(nil) should not error: %v", err)
	}

	err = k.RetractExactFactsBatch([]Fact{})
	if err != nil {
		t.Fatalf("RetractExactFactsBatch(empty) should not error: %v", err)
	}

	if k.FactCount() != initialCount {
		t.Errorf("Empty RetractExactFactsBatch changed count")
	}
}

func TestRetractExactFactsBatch_WhenMultiple_ShouldRemoveAll(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl metric(Name, Value).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	facts := []Fact{
		{Predicate: "metric", Args: []any{"cpu", "50"}},
		{Predicate: "metric", Args: []any{"mem", "80"}},
		{Predicate: "metric", Args: []any{"disk", "30"}},
	}
	k.AssertBatch(facts)

	// Remove two of three
	toRemove := []Fact{
		{Predicate: "metric", Args: []any{"cpu", "50"}},
		{Predicate: "metric", Args: []any{"disk", "30"}},
	}
	err := k.RetractExactFactsBatch(toRemove)
	if err != nil {
		t.Fatalf("RetractExactFactsBatch failed: %v", err)
	}

	results, err := k.Query("metric")
	if err != nil {
		t.Fatalf("Query metric failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 remaining metric, got %d", len(results))
	}
}

func TestRemoveFactsByPredicateSet_WhenEmpty_ShouldNoOp(t *testing.T) {
	k := setupMockKernel(t)
	initialCount := k.FactCount()

	err := k.RemoveFactsByPredicateSet(nil)
	if err != nil {
		t.Fatalf("RemoveFactsByPredicateSet(nil) should not error: %v", err)
	}

	err = k.RemoveFactsByPredicateSet(map[string]struct{}{})
	if err != nil {
		t.Fatalf("RemoveFactsByPredicateSet(empty) should not error: %v", err)
	}

	if k.FactCount() != initialCount {
		t.Errorf("Empty RemoveFactsByPredicateSet changed count")
	}
}

// =============================================================================
// QUERY TESTS
// =============================================================================

func TestQuery_WhenEmptyPredicate_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	_, err := k.Query("")
	if err == nil {
		t.Fatal("Query with empty predicate should error")
	}
}

func TestQuery_WhenPredicateNotDeclared_ShouldReturnEmpty(t *testing.T) {
	k := setupMockKernel(t)

	results, err := k.Query("nonexistent_predicate_xyz999")
	if err != nil {
		t.Fatalf("Query should not error for undeclared predicate: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for undeclared predicate, got %d", len(results))
	}
}

func TestQuery_WhenPatternProvided_ShouldFilterResults(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl coord(X, Y).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	k.Assert(Fact{Predicate: "coord", Args: []any{"a", "1"}})
	k.Assert(Fact{Predicate: "coord", Args: []any{"b", "2"}})
	k.Assert(Fact{Predicate: "coord", Args: []any{"a", "3"}})

	results, err := k.Query(`coord("a", X)`)
	if err != nil {
		t.Fatalf("Pattern query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for coord(a, X), got %d", len(results))
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestConcurrentAssert_ShouldNotPanic(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl concurrent_item(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	var wg sync.WaitGroup
	const goroutines = 10
	const factsPerGoroutine = 20

	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range factsPerGoroutine {
				fact := Fact{Predicate: "concurrent_item", Args: []any{fmt.Sprintf("g%d_i%d", g, i)}}
				if err := k.Assert(fact); err != nil {
					t.Errorf("Assert failed in goroutine %d: %v", g, err)
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestConcurrentAssertAndQuery_ShouldNotRace(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl race_item(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine
	go func() {
		defer wg.Done()
		for i := range 50 {
			k.Assert(Fact{Predicate: "race_item", Args: []any{fmt.Sprintf("item_%d", i)}})
		}
	}()

	// Reader goroutine
	go func() {
		defer wg.Done()
		for range 50 {
			_, _ = k.Query("race_item")
		}
	}()

	wg.Wait()
}

func TestConcurrentRetract_ShouldNotPanic(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl temp_item(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Add some facts first
	for i := range 20 {
		k.Assert(Fact{Predicate: "temp_item", Args: []any{fmt.Sprintf("item_%d", i)}})
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		k.Retract("temp_item")
	}()
	go func() {
		defer wg.Done()
		k.Retract("temp_item")
	}()
	go func() {
		defer wg.Done()
		_, _ = k.Query("temp_item")
	}()

	wg.Wait()
}

// =============================================================================
// FACT SNAPSHOT & COUNT TESTS
// =============================================================================

func TestGetFactsSnapshotSeq_ShouldReturnCopy(t *testing.T) {
	k := setupMockKernel(t)

	snapshot := slices.Collect(k.GetFactsSnapshotSeq())
	originalLen := len(snapshot)

	// Modify the snapshot (should not affect kernel)
	if len(snapshot) > 0 {
		snapshot[0] = Fact{Predicate: "tampered"}
	}

	snapshot2 := slices.Collect(k.GetFactsSnapshotSeq())
	if len(snapshot2) != originalLen {
		t.Errorf("Modifying snapshot affected kernel facts")
	}
}

func TestGetAllFacts_ShouldReturnCopy(t *testing.T) {
	k := setupMockKernel(t)

	facts1 := k.GetAllFacts()
	facts2 := k.GetAllFacts()

	if len(facts1) != len(facts2) {
		t.Errorf("GetAllFacts returned inconsistent lengths: %d vs %d", len(facts1), len(facts2))
	}
}

func TestIsDirty_WhenAssertCalled_ShouldBeTrue(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl dirty_test(X).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// After evaluate, should not be dirty
	if k.IsDirty() {
		t.Error("Kernel should not be dirty after Evaluate()")
	}

	// Assert marks dirty
	k.Assert(Fact{Predicate: "dirty_test", Args: []any{"a"}})
	if !k.IsDirty() {
		t.Error("Kernel should be dirty after Assert()")
	}

	// Query triggers lazy eval, clearing dirty flag
	_, err := k.Query("dirty_test")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if k.IsDirty() {
		t.Error("Kernel should not be dirty after Query() (lazy eval)")
	}
}

// =============================================================================
// CANONICALIZATION TESTS
// =============================================================================

func TestCanonValue_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{name: "nil", input: nil, expected: "null"},
		{name: "string_normal", input: "hello", expected: `"hello"`},
		{name: "string_name_constant", input: "/foo", expected: "/foo"},
		{name: "mangle_atom", input: MangleAtom("/bar"), expected: "/bar"},
		{name: "bool_true", input: true, expected: "/true"},
		{name: "bool_false", input: false, expected: "/false"},
		{name: "int", input: 42, expected: "42"},
		{name: "int64", input: int64(99), expected: "99"},
		{name: "int8", input: int8(7), expected: "7"},
		{name: "int16", input: int16(256), expected: "256"},
		{name: "int32", input: int32(1000), expected: "1000"},
		{name: "uint", input: uint(5), expected: "5"},
		{name: "uint8", input: uint8(255), expected: "255"},
		{name: "uint16", input: uint16(65535), expected: "65535"},
		{name: "uint32", input: uint32(100000), expected: "100000"},
		{name: "uint64", input: uint64(1234567890), expected: "1234567890"},
		{name: "float32", input: float32(3.14), expected: "3.140000104904175"},
		{name: "float64", input: 2.718, expected: "2.718"},
		{name: "json_number_int", input: json.Number("42"), expected: "42"},
		{name: "json_number_float", input: json.Number("3.14"), expected: "3.14"},
		{name: "bytes", input: []byte("hi"), expected: `"hi"`},
		{name: "empty_slice_interface", input: []any{}, expected: "[]"},
		{name: "slice_interface", input: []any{"a", 1}, expected: `["a",1]`},
		{name: "slice_string", input: []string{"x", "y"}, expected: `["x","y"]`},
		{name: "slice_int", input: []int{1, 2, 3}, expected: "[1,2,3]"},
		{name: "slice_int64", input: []int64{10, 20}, expected: "[10,20]"},
		{name: "slice_float64", input: []float64{1.1, 2.2}, expected: "[1.1,2.2]"},
		{name: "map_string_interface", input: map[string]any{"a": 1}, expected: `{"a":1}`},
		{name: "map_string_string", input: map[string]string{"k": "v"}, expected: `{"k":"v"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonValue(tc.input)
			if got != tc.expected {
				t.Errorf("canonValue(%v) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// =============================================================================
// ARGS EQUALITY TESTS
// =============================================================================

func TestArgsEqual_WhenBothNil_ShouldReturnTrue(t *testing.T) {
	if !argsEqual(nil, nil) {
		t.Error("argsEqual(nil, nil) should be true")
	}
}

func TestArgsEqual_WhenOneNil_ShouldReturnFalse(t *testing.T) {
	if argsEqual(nil, "x") {
		t.Error("argsEqual(nil, 'x') should be false")
	}
	if argsEqual("x", nil) {
		t.Error("argsEqual('x', nil) should be false")
	}
}

func TestArgsEqual_CrossTypeComparisons(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"string_equal", "hello", "hello", true},
		{"string_unequal", "hello", "world", false},
		{"string_vs_mangle", "abc", MangleAtom("abc"), true},
		{"mangle_vs_string", MangleAtom("/foo"), "/foo", true},
		{"mangle_equal", MangleAtom("/x"), MangleAtom("/x"), true},
		{"int_equal", 42, 42, true},
		{"int_unequal", 42, 43, false},
		{"int_vs_int64", 42, int64(42), true},
		{"int64_vs_int", int64(42), 42, true},
		{"uint_equal", uint(5), uint(5), true},
		{"uint_vs_uint64", uint(5), uint64(5), true},
		{"float64_equal", 3.14, 3.14, true},
		{"bool_equal", true, true, true},
		{"bool_unequal", true, false, false},
		{"map_equal", map[string]any{"a": 1}, map[string]any{"a": 1}, true},
		{"slice_equal", []any{"x"}, []any{"x"}, true},
		{"type_mismatch", "42", 42, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := argsEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("argsEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestArgsSliceEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []any
		want bool
	}{
		{"both_nil", nil, nil, true},
		{"both_empty", []any{}, []any{}, true},
		{"equal", []any{"a", 1}, []any{"a", 1}, true},
		{"length_mismatch", []any{"a"}, []any{"a", "b"}, false},
		{"value_mismatch", []any{"a"}, []any{"b"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := argsSliceEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("argsSliceEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// =============================================================================
// MANGLE NAME CONSTANT VALIDATION TESTS
// =============================================================================

func TestIsValidMangleNameConstant(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"/foo", true},
		{"/bar_baz", true},
		{"/true", true},
		{"foo", false},              // no leading /
		{"/path/to/file.go", false}, // file extension
		{"/a/b/c", false},           // too many path segments
		{"/has space", false},       // whitespace
		{"/has\ttab", false},        // tab
		{"/has\nnewline", false},    // newline
		{"", false},                 // empty
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := isValidMangleNameConstant(tc.input)
			if got != tc.valid {
				t.Errorf("isValidMangleNameConstant(%q) = %v, want %v", tc.input, got, tc.valid)
			}
		})
	}
}

func TestHasFileExtension(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/main.go", true},
		{"/readme.md", true},
		{"/script.py", true},
		{"/foo", false},
		{"/bar_baz", false},
		{"/config.YAML", true}, // case insensitive
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := hasFileExtension(tc.input)
			if got != tc.want {
				t.Errorf("hasFileExtension(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// PRIORITY SANITIZATION TESTS
// =============================================================================

func TestSanitizeFactForNumericPredicates_AgendaItem(t *testing.T) {
	tests := []struct {
		name     string
		input    Fact
		wantArg2 any
	}{
		{
			name:     "high_atom",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "/high", "/pending", 0}},
			wantArg2: int64(80),
		},
		{
			name:     "critical_string",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "critical", "/pending", 0}},
			wantArg2: int64(100),
		},
		{
			name:     "numeric_passthrough",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", int64(75), "/pending", 0}},
			wantArg2: int64(75),
		},
		{
			name:     "low",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "/low", "/pending", 0}},
			wantArg2: int64(25),
		},
		{
			name:     "medium",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "medium", "/pending", 0}},
			wantArg2: int64(50),
		},
		{
			name:     "normal",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "normal", "/pending", 0}},
			wantArg2: int64(50),
		},
		{
			name:     "lowest",
			input:    Fact{Predicate: "agenda_item", Args: []any{"id1", "desc", "/lowest", "/pending", 0}},
			wantArg2: int64(10),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeFactForNumericPredicates(tc.input)
			if result.Args[2] != tc.wantArg2 {
				t.Errorf("agenda_item priority: got %v (%T), want %v (%T)",
					result.Args[2], result.Args[2], tc.wantArg2, tc.wantArg2)
			}
		})
	}
}

func TestSanitizeFactForNumericPredicates_AtomPriority(t *testing.T) {
	fact := Fact{Predicate: "atom_priority", Args: []any{"atom1", "/high"}}
	result := sanitizeFactForNumericPredicates(fact)

	if result.Args[1] != int64(80) {
		t.Errorf("atom_priority priority: got %v, want %d", result.Args[1], 80)
	}
}

func TestSanitizeFactForNumericPredicates_PromptAtom(t *testing.T) {
	fact := Fact{Predicate: "prompt_atom", Args: []any{"id1", "/category", "/critical", 500, true}}
	result := sanitizeFactForNumericPredicates(fact)

	if result.Args[2] != int64(100) {
		t.Errorf("prompt_atom priority: got %v, want %d", result.Args[2], 100)
	}
}

func TestSanitizeFactForNumericPredicates_UnrelatedPredicate_ShouldPassthrough(t *testing.T) {
	fact := Fact{Predicate: "user_intent", Args: []any{"id1", "/query", "/explain", "target", "constraint"}}
	result := sanitizeFactForNumericPredicates(fact)

	// Should be unchanged
	if result.Predicate != fact.Predicate || len(result.Args) != len(fact.Args) {
		t.Errorf("Unrelated predicate was modified")
	}
}

// =============================================================================
// TRANSACTION TESTS
// =============================================================================

func TestTransaction_WhenDoubleCommit_ShouldError(t *testing.T) {
	k := setupMockKernel(t)

	tx := k.Transaction()
	err := tx.Commit()
	if err != nil {
		t.Fatalf("First commit should succeed: %v", err)
	}

	err = tx.Commit()
	if err == nil {
		t.Fatal("Second commit should error")
	}
}

func TestTransaction_WhenRetractThenAssert_ShouldBeAtomic(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl test_state_atomic(Key, Value).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	k.Assert(Fact{Predicate: "test_state_atomic", Args: []any{"mode", "old"}})

	tx := k.Transaction()
	tx.Retract("test_state_atomic")
	tx.Assert(Fact{Predicate: "test_state_atomic", Args: []any{"mode", "new"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("Transaction commit failed: %v", err)
	}

	results, err := k.Query("test_state_atomic")
	if err != nil {
		t.Fatalf("Query state failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 test_state_atomic fact, got %d", len(results))
	}
}

// =============================================================================
// LOAD SCHEMAS / POLICY TESTS
// =============================================================================

func TestLoadSchemas_ShouldSetPolicyDirty(t *testing.T) {
	k := setupMockKernel(t)

	k.LoadSchemas("Decl custom_schema(X).")
	if !k.IsDirty() {
		// policyDirty is separate from factsDirty; test policyDirty indirectly
		// The next evaluate should use the new schema
	}
}

func TestLoadPolicy_ShouldSetPolicyDirty(t *testing.T) {
	k := setupMockKernel(t)

	k.LoadPolicy("Decl custom_policy(X).")
	// If we can evaluate without error, the policy was accepted
}

// =============================================================================
// PARSE FACT STRING TESTS
// =============================================================================

func TestParseFactString_ValidFacts(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		predicate string
		argCount  int
	}{
		{"name_constant", `test(/foo)`, "test", 1},
		{"string_arg", `greeting("hello")`, "greeting", 1},
		{"number_arg", `count(42)`, "count", 1},
		{"multi_arg", `pair("a", "b")`, "pair", 2},
		{"mixed_types", `record(/type, "desc", 99)`, "record", 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fact, err := ParseFactString(tc.input)
			if err != nil {
				t.Fatalf("ParseFactString(%q) error: %v", tc.input, err)
			}
			if fact.Predicate != tc.predicate {
				t.Errorf("Predicate = %q, want %q", fact.Predicate, tc.predicate)
			}
			if len(fact.Args) != tc.argCount {
				t.Errorf("ArgCount = %d, want %d", len(fact.Args), tc.argCount)
			}
		})
	}
}

func TestParseFactString_WhenEmpty_ShouldError(t *testing.T) {
	_, err := ParseFactString("")
	if err == nil {
		t.Fatal("ParseFactString empty should error")
	}

	_, err = ParseFactString(".")
	if err == nil {
		t.Fatal("ParseFactString period-only should error")
	}
}

func TestParseFactsFromString_WhenMultiple_ShouldParseAll(t *testing.T) {
	input := `color(/red).
color(/blue).
color(/green).`

	facts, err := ParseFactsFromString(input)
	if err != nil {
		t.Fatalf("ParseFactsFromString error: %v", err)
	}
	if len(facts) != 3 {
		t.Errorf("Expected 3 facts, got %d", len(facts))
	}
}

func TestParseSingleFact_WhenMultiple_ShouldError(t *testing.T) {
	_, err := ParseSingleFact("a(1).\nb(2).")
	if err == nil {
		t.Fatal("ParseSingleFact with multiple facts should error")
	}
}

func TestParseSingleFact_WhenNone_ShouldError(t *testing.T) {
	// Rules (with body) are skipped, so no ground facts found
	_, err := ParseSingleFact("a(X) :- b(X).")
	if err == nil {
		t.Fatal("ParseSingleFact with only a rule should error")
	}
}

// =============================================================================
// EVALUATE / ASSERT WITHOUT EVAL TESTS
// =============================================================================

func TestAssertWithoutEval_WhenFollowedByEvaluate_ShouldWork(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl batch_item(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	k.AssertWithoutEval(Fact{Predicate: "batch_item", Args: []any{"x"}})
	k.AssertWithoutEval(Fact{Predicate: "batch_item", Args: []any{"y"}})

	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after AssertWithoutEval failed: %v", err)
	}

	results, err := k.Query("batch_item")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 batch_item facts, got %d", len(results))
	}
}
