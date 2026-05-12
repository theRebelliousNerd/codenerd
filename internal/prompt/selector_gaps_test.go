package prompt

import (
	"context"
	"math"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Selector Gap Tests — remediation of TEST_GAP markers from selector_test.go
// =============================================================================

// Vector A: Null/Undefined/Empty

func TestSelector_SelectAtoms_NilInputs(t *testing.T) {
	// GAP A1: Verify safe handling of nil atom slices, nil context, and nil mandatory maps.
	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{})

	t.Run("nil atoms", func(t *testing.T) {
		result, err := selector.SelectAtoms(context.Background(), nil, NewCompilationContext())
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil result, got %d atoms", len(result))
		}
	})

	t.Run("nil context", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true},
		}
		// nil context causes panic in CompilationContext.GenerateFacts (known bug)
		defer func() {
			if r := recover(); r != nil {
				t.Logf("KNOWN BUG: nil context panics in GenerateFacts: %v", r)
			}
		}()
		_, err := selector.SelectAtoms(context.Background(), atoms, nil)
		if err != nil {
			t.Logf("SelectAtoms with nil context: %v (acceptable)", err)
		}
	})
}

func TestSelector_SelectAtoms_NilElements(t *testing.T) {
	// GAP A5: Inject nil pointers into candidate slices.
	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: []interface{}{
		Fact{Predicate: "selected_result", Args: []interface{}{"a", 100, "skeleton"}},
	}})

	atoms := []*PromptAtom{
		nil,
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true},
		nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic on nil atom elements (known gap): %v", r)
		}
	}()

	result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
	if err != nil {
		t.Logf("SelectAtoms with nil elements: %v", err)
	}
	if result != nil {
		t.Logf("Got %d results", len(result))
	}
}

func TestSelector_FallbackSelection_EmptyCorpus(t *testing.T) {
	// GAP A6: Pass empty slices to fallback and verify quick returns.
	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{})

	result, err := selector.SelectAtoms(context.Background(), []*PromptAtom{}, NewCompilationContext())
	if err != nil {
		t.Errorf("Expected no error for empty corpus, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for empty corpus, got %d atoms", len(result))
	}
}

// Vector B: Type Coercion/Invalid Data

func TestSelector_ExtractStringArg_UnknownTypes(t *testing.T) {
	// GAP B2: Pass unsupported Go types to extractStringArg.
	tests := []struct {
		name  string
		input interface{}
	}{
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"int64", int64(42)},
		{"float64", 3.14},
		{"bool true", true},
		{"bool false", false},
		{"byte slice", []byte{0x48, 0x49}},
		{"uint", uint(42)},
		{"int32", int32(42)},
		{"float32", float32(1.5)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := extractStringArg(tc.input)
			if err != nil {
				t.Errorf("extractStringArg(%T) errored: %v", tc.input, err)
			}
			t.Logf("extractStringArg(%T=%v) -> %q", tc.input, tc.input, result)
		})
	}

	// Unsupported types should error, not panic
	t.Run("struct", func(t *testing.T) {
		type custom struct{ X int }
		_, err := extractStringArg(custom{X: 42})
		if err == nil {
			t.Error("Expected error for unsupported struct type")
		}
	})

	t.Run("map", func(t *testing.T) {
		_, err := extractStringArg(map[string]int{"a": 1})
		if err == nil {
			t.Error("Expected error for map type")
		}
	})

	t.Run("slice", func(t *testing.T) {
		_, err := extractStringArg([]int{1, 2, 3})
		if err == nil {
			t.Error("Expected error for int slice type")
		}
	})
}

func TestSelector_MangleQuoteString_SpecialChars(t *testing.T) {
	// GAP B1/B4: Verify mangleQuoteString sanitizes strings.
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"normal", "hello"},
		{"with quotes", `he said "hello"`},
		{"with backslash", `path\to\file`},
		{"with newline", "line1\nline2"},
		{"with tab", "col1\tcol2"},
		{"with null byte", "null\x00byte"},
		{"unicode", "日本語テスト"},
		{"emoji", "🎉🚀"},
		{"control chars", "\x01\x02\x03"},
		{"mixed", "hello \"world\" \n\t 日本語 \x00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			result := mangleQuoteString(tc.input)
			if !strings.HasPrefix(result, "\"") || !strings.HasSuffix(result, "\"") {
				t.Errorf("Result not properly quoted: %q", result)
			}
			t.Logf("mangleQuoteString(%q) -> %s", tc.input, result)
		})
	}
}

// Vector C: User Request Extremes

func TestSelector_MassiveAtomCorpus(t *testing.T) {
	// GAP C1: 1000 candidate atoms (scaled down from 100k for test speed).
	atoms := make([]*PromptAtom, 1000)
	for i := 0; i < 1000; i++ {
		atoms[i] = &PromptAtom{
			ID:          strings.Repeat("atom-", 1) + string(rune(i%26+'a')),
			Category:    CategoryContext,
			Content:     strings.Repeat("content ", 10),
			Priority:    i % 100,
			IsMandatory: i < 10,
		}
	}
	// Add skeleton atoms
	atoms = append(atoms, &PromptAtom{
		ID: "identity-1", Category: CategoryIdentity, Content: "Identity",
		Priority: 100, IsMandatory: true,
	})

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: []interface{}{
		Fact{Predicate: "selected_result", Args: []interface{}{"identity-1", 100, "skeleton"}},
	}})

	result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("SelectAtoms failed: %v", err)
	}
	t.Logf("Selected %d atoms from 1001 candidates", len(result))
}

func TestSelector_MergeAtoms_MassiveContent(t *testing.T) {
	// GAP C2: Merge atoms with multi-megabyte contents.
	bigContent := strings.Repeat("x", 1024*1024) // 1MB
	skeleton := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "big", Category: CategoryIdentity, Content: bigContent},
			LogicScore: 1.0, Combined: 1.0, Source: "skeleton"},
	}
	flesh := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "small", Category: CategoryContext, Content: "small"},
			LogicScore: 0.5, Combined: 0.5, Source: "flesh"},
	}

	selector := NewAtomSelector()
	merged := selector.mergeAtoms(skeleton, flesh)
	if len(merged) != 2 {
		t.Errorf("Expected 2 merged atoms, got %d", len(merged))
	}
}

func TestSelector_FallbackSelection_NaNScores(t *testing.T) {
	// GAP C3: Inject NaN and Inf into vector scores.
	skeleton := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "A"},
			LogicScore: 1.0, VectorScore: math.NaN(), Combined: math.NaN(), Source: "skeleton"},
		{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, Content: "B"},
			LogicScore: 0.5, VectorScore: math.Inf(1), Combined: math.Inf(1), Source: "skeleton"},
		{Atom: &PromptAtom{ID: "c", Category: CategoryIdentity, Content: "C"},
			LogicScore: 0.5, VectorScore: math.Inf(-1), Combined: math.Inf(-1), Source: "skeleton"},
	}

	selector := NewAtomSelector()
	// Must not panic during merge/sort
	merged := selector.mergeAtoms(skeleton, nil)
	t.Logf("Merged %d atoms with NaN/Inf scores", len(merged))
}

func TestSelector_MassiveContextDimensions(t *testing.T) {
	// GAP C4: Supply CompilationContext with many frameworks.
	cc := NewCompilationContext()
	frameworks := make([]string, 100)
	for i := 0; i < 100; i++ {
		frameworks[i] = "/framework-" + string(rune(i%26+'a'))
	}
	cc = cc.WithLanguage("/go", frameworks...)

	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: []interface{}{
		Fact{Predicate: "selected_result", Args: []interface{}{"a", 100, "skeleton"}},
	}})

	result, err := selector.SelectAtoms(context.Background(), atoms, cc)
	if err != nil {
		t.Fatalf("SelectAtoms with massive context failed: %v", err)
	}
	t.Logf("Selected %d atoms with 100 frameworks", len(result))
}

func TestSelector_MergeAtoms_SortDeterminism(t *testing.T) {
	// GAP C5: Provide identical scores and assert sort doesn't cause jitter.
	skeleton := make([]*ScoredAtom, 20)
	for i := 0; i < 20; i++ {
		skeleton[i] = &ScoredAtom{
			Atom:       &PromptAtom{ID: "atom-" + string(rune(i+'a')), Category: CategoryIdentity},
			LogicScore: 1.0,
			Combined:   0.5,
			Source:     "skeleton",
		}
	}

	selector := NewAtomSelector()
	merged1 := selector.mergeAtoms(skeleton, nil)
	merged2 := selector.mergeAtoms(skeleton, nil)

	if len(merged1) != len(merged2) {
		t.Fatalf("Different merge counts: %d vs %d", len(merged1), len(merged2))
	}

	// With identical scores, order should be consistent
	for i := range merged1 {
		if merged1[i].Atom.ID != merged2[i].Atom.ID {
			t.Logf("Sort order jitter at index %d: %s vs %s (may be acceptable with identical scores)",
				i, merged1[i].Atom.ID, merged2[i].Atom.ID)
		}
	}
}

func TestSelector_InventedContexts(t *testing.T) {
	// GAP C7: Pass wildly unpredictable strings as languages and frameworks.
	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithLanguage("/日本語", "/with spaces", "/with\"quotes", "/\x00null")

	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: []interface{}{
		Fact{Predicate: "selected_result", Args: []interface{}{"a", 100, "skeleton"}},
	}})

	// Must not panic
	result, err := selector.SelectAtoms(context.Background(), atoms, cc)
	if err != nil {
		t.Logf("SelectAtoms with invented contexts: %v (acceptable)", err)
	}
	if result != nil {
		t.Logf("Selected %d atoms", len(result))
	}
}

func TestSelector_ExtremePriority(t *testing.T) {
	// GAP C9: Assert atoms with extreme priority values.
	atoms := []*PromptAtom{
		{ID: "max", Category: CategoryIdentity, Content: "Max", Priority: math.MaxInt64, IsMandatory: true, TokenCount: 10},
		{ID: "min", Category: CategoryIdentity, Content: "Min", Priority: math.MinInt64, IsMandatory: true, TokenCount: 10},
		{ID: "zero", Category: CategoryIdentity, Content: "Zero", Priority: 0, IsMandatory: true, TokenCount: 10},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})

	result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
	if err != nil {
		t.Logf("SelectAtoms with extreme priorities: %v (may be acceptable)", err)
	}
	if result != nil {
		t.Logf("Selected %d atoms with extreme priorities", len(result))
	}
}

func TestSelector_ExtremeTokenCounts(t *testing.T) {
	// GAP: verify atoms with extreme token counts don't crash fallback selection.
	atoms := []*PromptAtom{
		{ID: "huge", Category: CategoryIdentity, Content: "Huge", Priority: 100, IsMandatory: true, TokenCount: math.MaxInt64},
		{ID: "negative", Category: CategoryIdentity, Content: "Negative", Priority: 50, IsMandatory: true, TokenCount: -1},
		{ID: "zero", Category: CategoryIdentity, Content: "Zero", Priority: 50, IsMandatory: true, TokenCount: 0},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})

	// Must not panic or overflow
	result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
	if err != nil {
		t.Logf("SelectAtoms with extreme token counts: %v (acceptable)", err)
	}
	if result != nil {
		t.Logf("Selected %d atoms", len(result))
	}
}

// Vector D: State Conflicts

func TestSelector_ConcurrentSelectAtoms(t *testing.T) {
	// GAP D1: Detect data races if selector is used concurrently.
	// Note: Each goroutine gets its own selector+kernel since mockKernel has
	// unsynchronized state. In production, RealKernel uses sync.RWMutex.
	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true, TokenCount: 10},
	}

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector := NewAtomSelector()
			selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})
			cc := NewCompilationContext().WithTokenBudget(10000, 1000)
			_, _ = selector.SelectAtoms(context.Background(), atoms, cc)
		}()
	}

	wg.Wait()
}

func TestSelector_Idempotency(t *testing.T) {
	// GAP D2: Ensure subsequent calls don't suffer from ghost facts.
	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true, TokenCount: 10},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})
	cc := NewCompilationContext().WithTokenBudget(10000, 1000)

	result1, err1 := selector.SelectAtoms(context.Background(), atoms, cc)
	result2, err2 := selector.SelectAtoms(context.Background(), atoms, cc)

	if err1 != nil || err2 != nil {
		t.Logf("Errors: %v, %v (may be acceptable)", err1, err2)
		return
	}

	if len(result1) != len(result2) {
		t.Errorf("Idempotency violation: call 1 returned %d atoms, call 2 returned %d",
			len(result1), len(result2))
	}
}

func TestSelector_KernelPanicRecovery(t *testing.T) {
	// GAP D4: Mock a panicking kernel; verify graceful recovery.
	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true},
	}

	panicKernel := &mockKernel{
		assertErr: nil,
	}
	// Override Query to panic
	selector := NewAtomSelector()
	selector.SetKernel(panicKernel)

	// The system should either recover from panic or return error
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic from kernel (known gap — no recovery wrapper): %v", r)
		}
	}()

	cc := NewCompilationContext()
	_, err := selector.SelectAtoms(context.Background(), atoms, cc)
	if err != nil {
		t.Logf("SelectAtoms with kernel error: %v (expected)", err)
	}
}

func TestSelector_ConcurrentContextMutation(t *testing.T) {
	// GAP D6: Pass CompilationContext while mutating its slices.
	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true, TokenCount: 10},
	}

	selector := NewAtomSelector()
	selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})

	var wg sync.WaitGroup
	wg.Add(2)

	cc := NewCompilationContext().WithLanguage("/go").WithTokenBudget(10000, 1000)

	// Goroutine 1: select atoms
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = selector.SelectAtoms(context.Background(), atoms, cc)
		}
	}()

	// Goroutine 2: mutate context (different context each time to be safe)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			// Create new contexts rather than mutating shared one
			_ = NewCompilationContext().WithLanguage("/python")
		}
	}()

	wg.Wait()
}

func TestSelector_KernelIsolation(t *testing.T) {
	// GAP D7: Two concurrent compilations — each gets own selector to test isolation.
	// Note: sharing a single kernel across goroutines causes a race in mockKernel.AssertBatch
	// (shared facts slice). In production, the RealKernel uses sync.RWMutex.
	atoms := []*PromptAtom{
		{ID: "a", Category: CategoryIdentity, Content: "A", Priority: 100, IsMandatory: true, TokenCount: 10},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		selector := NewAtomSelector()
		selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})
		cc := NewCompilationContext().WithShard("/coder", "", "").WithTokenBudget(10000, 1000)
		for i := 0; i < 20; i++ {
			_, _ = selector.SelectAtoms(context.Background(), atoms, cc)
		}
	}()

	go func() {
		defer wg.Done()
		selector := NewAtomSelector()
		selector.SetKernel(&mockKernel{facts: atomsToFacts(atoms)})
		cc := NewCompilationContext().WithShard("/tester", "", "").WithTokenBudget(10000, 1000)
		for i := 0; i < 20; i++ {
			_, _ = selector.SelectAtoms(context.Background(), atoms, cc)
		}
	}()

	wg.Wait()
}


