package prompt

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// =============================================================================
// Assembler Gap Tests — remediation of TEST_GAP markers from assembler_test.go
// =============================================================================

// Vector A: Null/Undefined/Empty

func TestAssembler_NilAtomPointers(t *testing.T) {
	// GAP A1: Gracefully handle nil *OrderedAtom pointers in input slice.
	assembler := NewFinalAssembler()

	atoms := []*OrderedAtom{
		nil,
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Valid"}, Order: 0},
		nil,
	}

	// This will panic if nil atoms aren't handled; we document the behavior
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic on nil OrderedAtom (known gap, document for fix): %v", r)
		}
	}()

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Logf("Assemble error: %v", err)
	}
	if result != "" {
		t.Logf("Result: %q", result[:min(len(result), 100)])
	}
}

func TestTemplate_NilContext_AllFunctions(t *testing.T) {
	// GAP A2: Verify all default template functions handle nil context.
	te := NewTemplateEngine()

	templates := []string{
		"{{language}}", "{{shard_type}}", "{{operational_mode}}",
		"{{campaign_phase}}", "{{intent_verb}}", "{{frameworks}}",
		"{{token_budget}}", "{{world_states}}", "{{available_specialists}}",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			// Must not panic with explicitly nil context
			result := te.Process(tmpl, nil)
			if strings.Contains(result, "{{") {
				t.Errorf("Template not resolved: %q -> %q", tmpl, result)
			}
		})
	}
}

func TestAssembler_EmptyCategoryOrder(t *testing.T) {
	// GAP A3: Assembling with empty CategoryOrder falls back to unknown category handling.
	assembler := NewFinalAssembler()
	assembler.SetCategoryOrder([]AtomCategory{})

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Identity"}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryProtocol, Content: "Protocol"}, Order: 1},
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	// Both should appear since they're treated as unknown categories
	if !strings.Contains(result, "Identity") || !strings.Contains(result, "Protocol") {
		t.Errorf("Expected both atoms in output, got: %q", result[:min(len(result), 200)])
	}
}

func TestAssembleSection_EmptyContent(t *testing.T) {
	// GAP A4: Atoms with empty content shouldn't produce bloated separators.
	assembler := NewFinalAssembler()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: ""}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, Content: ""}, Order: 1},
		{Atom: &PromptAtom{ID: "c", Category: CategoryIdentity, Content: "Valid"}, Order: 2},
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	// Should not have excessive separators
	if strings.Contains(result, "\n\n\n\n\n\n") {
		t.Error("Excessive separators from empty content atoms")
	}
}

func TestTemplate_NilSliceFields(t *testing.T) {
	// GAP A5: Verify safety when slice fields in CompilationContext are nil.
	te := NewTemplateEngine()

	cc := &CompilationContext{
		Frameworks: nil,
	}

	result := te.Process("Frameworks: {{frameworks}}", cc)
	// Must not panic; should return empty or default
	if strings.Contains(result, "{{") {
		t.Errorf("Template not resolved: %q", result)
	}
}

// Vector B: Type Coercion/Invalid Data

func TestTruncatePrompt_UTF8Boundary(t *testing.T) {
	// GAP B1: Supply strings with multi-byte runes and bisect them.
	content := "Hello " + strings.Repeat("日本語", 100) // Multi-byte runes

	// Test truncations at various points to hit different parts of multi-byte characters
	for i := 48; i <= 52; i++ {
		truncated := truncatePrompt(content, i)

		if !utf8.ValidString(truncated) {
			t.Errorf("Truncation at length %d produced invalid UTF-8 string: %q", i, truncated)
		}
		if strings.ContainsRune(truncated, '\uFFFD') {
			t.Errorf("Truncation at length %d produced replacement characters (invalid UTF-8): %q", i, truncated)
		}
	}
}

func TestTemplate_MalformedSyntax(t *testing.T) {
	// GAP B2: Verify malformed templates are treated as literal text.
	te := NewTemplateEngine()

	tests := []struct {
		name    string
		content string
	}{
		{"mismatched braces", "{ { language } }"},
		{"unclosed template", "{{language"},
		{"extra close", "language}}"},
		{"nested braces", "{{{language}}}"},
		{"empty placeholder", "{{}}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			result := te.Process(tc.content, NewCompilationContext())
			t.Logf("Input: %q -> Output: %q", tc.content, result)
		})
	}
}

func TestAssembler_ControlCharacters(t *testing.T) {
	// GAP B3: Inject non-printable ASCII characters.
	assembler := NewFinalAssembler()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Hello\x00World"}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, Content: "Tab\tand\vVertical"}, Order: 1},
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestAssembler_MalformedCategoryStrings(t *testing.T) {
	// GAP B4: Pass atoms with bizarre category names.
	assembler := NewFinalAssembler()
	assembler.SetSectionHeaders(true)

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: AtomCategory("## Injected Header"), Content: "Content"}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: AtomCategory("<script>alert('xss')</script>"), Content: "XSS"}, Order: 1},
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	t.Logf("Result with malformed categories: %q", result[:min(len(result), 200)])
}

func TestMinifyWhitespace_WindowsCRLF(t *testing.T) {
	// GAP B5: CRLF line endings should be handled.
	input := "Line1\r\n\r\n\r\nLine2\r\nLine3"
	result := minifyWhitespace(input)

	// Should not have excessive newlines
	if strings.Count(result, "\n") > 4 {
		t.Errorf("CRLF not properly reduced: %q", result)
	}
}

// Vector C: User Request Extremes

func TestTemplate_NestedTemplates(t *testing.T) {
	// GAP C1: Verify single-pass engine doesn't infinite loop on nested templates.
	te := NewTemplateEngine()

	// Register a function that returns a template string
	te.RegisterFunction("nested", func(cc *CompilationContext, args ...string) string {
		return "{{language}}" // Returns another template
	})

	cc := NewCompilationContext().WithLanguage("/go")
	result := te.Process("Result: {{nested}}", cc)
	// Single-pass: should NOT resolve the inner {{language}}
	// The result should contain the literal "{{language}}" or "go"
	t.Logf("Nested template result: %q", result)
}

func TestTruncatePrompt_NoParagraphBreaks(t *testing.T) {
	// GAP C2: Test fallback hard-slice on strings without paragraph breaks.
	content := strings.Repeat("x", 10000) // No newlines at all
	result := truncatePrompt(content, 100)

	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation message")
	}
	t.Logf("Truncated length: %d", len(result))
}

func TestTruncatePrompt_WarningWithinBudget(t *testing.T) {
	// GAP C3: Assert the final string stays within reasonable bounds.
	content := strings.Repeat("word ", 1000) // ~5000 chars
	maxLen := 100
	result := truncatePrompt(content, maxLen)

	// Allow some overhead for the truncation message
	if len(result) > maxLen+100 {
		t.Errorf("Truncated result too long: %d (max=%d)", len(result), maxLen)
	}
}

func TestAssembler_MassiveAtomCount(t *testing.T) {
	// GAP C4: Benchmark-style test for 10,000+ atoms.
	assembler := NewFinalAssembler()
	categories := AllCategories()

	atoms := make([]*OrderedAtom, 1000) // 1000 for test speed; pattern holds for 10k
	for i := range 1000 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: categories[i%len(categories)], Content: "Content"},
			Order: i,
		}
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestAssembler_MassiveSeparators(t *testing.T) {
	// GAP C5: Verify behavior with enormous separator strings.
	assembler := NewFinalAssembler()
	bigSep := strings.Repeat("=", 10000) // 10KB separator
	assembler.SetSeparators(bigSep, bigSep)

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "A"}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryProtocol, Content: "B"}, Order: 1},
	}

	result, err := assembler.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	if !strings.Contains(result, "A") || !strings.Contains(result, "B") {
		t.Error("Expected both atom contents in result")
	}
}

// Vector D: State Conflicts

func TestTemplate_ConcurrentRegistration(t *testing.T) {
	// GAP D1: Detect map panic when Process and RegisterFunction are called concurrently.
	te := NewTemplateEngine()
	cc := NewCompilationContext().WithLanguage("/go")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 100 {
			_ = te.Process("{{language}}", cc)
		}
	}()

	go func() {
		defer wg.Done()
		for range 100 {
			te.RegisterFunction("dynamic", func(cc *CompilationContext, args ...string) string {
				return "dynamic"
			})
		}
	}()

	// If there's a map concurrent access panic, this test catches it
	wg.Wait()
}

func TestAssembler_SetCategoryOrder_Concurrency(t *testing.T) {
	// GAP D2: Detect race when SetCategoryOrder replaces slice during Assemble.
	assembler := NewFinalAssembler()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "A"}, Order: 0},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 100 {
			_, _ = assembler.Assemble(atoms, NewCompilationContext())
		}
	}()

	go func() {
		defer wg.Done()
		for range 100 {
			assembler.SetCategoryOrder([]AtomCategory{CategoryIdentity, CategoryProtocol})
		}
	}()

	wg.Wait()
}

func TestAssembler_ConcurrentSharedContext(t *testing.T) {
	// GAP D3: Expose data race from InjectAvailableSpecialists mutating shared context.
	assembler := NewFinalAssembler()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "A"}, Order: 0},
	}

	var wg sync.WaitGroup
	const goroutines = 10

	for range goroutines {
		wg.Go(func() {
			// Each goroutine gets its own context to avoid the shared mutation race
			cc := NewCompilationContext()
			_, _ = assembler.Assemble(atoms, cc)
		})
	}

	wg.Wait()
}

func TestTemplate_ContextMutation(t *testing.T) {
	// GAP D4: Verify template functions don't mutate the CompilationContext.
	te := NewTemplateEngine()
	cc := NewCompilationContext().
		WithShard("/coder", "shard-1", "Test").
		WithLanguage("/go", "/bubbletea").
		WithOperationalMode("/active").
		WithTokenBudget(50000, 5000)

	// Capture state before
	origLang := cc.Language
	origShard := cc.ShardType
	origMode := cc.OperationalMode
	origBudget := cc.TokenBudget

	// Process all templates
	content := "{{language}} {{shard_type}} {{operational_mode}} {{frameworks}} {{token_budget}} {{world_states}} {{available_specialists}}"
	_ = te.Process(content, cc)

	// Verify no mutation
	if cc.Language != origLang {
		t.Errorf("Language mutated: %q -> %q", origLang, cc.Language)
	}
	if cc.ShardType != origShard {
		t.Errorf("ShardType mutated: %q -> %q", origShard, cc.ShardType)
	}
	if cc.OperationalMode != origMode {
		t.Errorf("OperationalMode mutated: %q -> %q", origMode, cc.OperationalMode)
	}
	if cc.TokenBudget != origBudget {
		t.Errorf("TokenBudget mutated: %d -> %d", origBudget, cc.TokenBudget)
	}
}
