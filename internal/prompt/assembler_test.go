package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFinalAssembler(t *testing.T) {
	t.Run("creates assembler with defaults", func(t *testing.T) {
		assembler := NewFinalAssembler()
		require.NotNil(t, assembler)

		assert.NotEmpty(t, assembler.categoryOrder)
		assert.False(t, assembler.addSectionHeaders)
		assert.Equal(t, "\n\n", assembler.sectionSeparator)
		assert.Equal(t, "\n\n", assembler.atomSeparator)
		assert.NotNil(t, assembler.templateEngine)
	})
}

func TestDefaultCategoryOrder(t *testing.T) {
	order := defaultCategoryOrder()

	t.Run("identity comes first", func(t *testing.T) {
		assert.Equal(t, CategoryIdentity, order[0])
	})

	t.Run("context comes last", func(t *testing.T) {
		assert.Equal(t, CategoryContext, order[len(order)-1])
	})

	t.Run("safety before methodology", func(t *testing.T) {
		var safetyIdx, methodologyIdx int
		for i, cat := range order {
			if cat == CategorySafety {
				safetyIdx = i
			}
			if cat == CategoryMethodology {
				methodologyIdx = i
			}
		}
		assert.Less(t, safetyIdx, methodologyIdx)
	})

	t.Run("protocol before methodology", func(t *testing.T) {
		var protocolIdx, methodologyIdx int
		for i, cat := range order {
			if cat == CategoryProtocol {
				protocolIdx = i
			}
			if cat == CategoryMethodology {
				methodologyIdx = i
			}
		}
		assert.Less(t, protocolIdx, methodologyIdx)
	})
}

func TestFinalAssembler_SetCategoryOrder(t *testing.T) {
	assembler := NewFinalAssembler()

	customOrder := []AtomCategory{CategoryExemplar, CategoryIdentity}
	assembler.SetCategoryOrder(customOrder)

	assert.Equal(t, customOrder, assembler.categoryOrder)
}

func TestFinalAssembler_SetSectionHeaders(t *testing.T) {
	assembler := NewFinalAssembler()

	assembler.SetSectionHeaders(true)
	assert.True(t, assembler.addSectionHeaders)

	assembler.SetSectionHeaders(false)
	assert.False(t, assembler.addSectionHeaders)
}

func TestFinalAssembler_SetSeparators(t *testing.T) {
	assembler := NewFinalAssembler()

	assembler.SetSeparators("---\n", "\n")
	assert.Equal(t, "---\n", assembler.sectionSeparator)
	assert.Equal(t, "\n", assembler.atomSeparator)
}

func TestFinalAssembler_Assemble(t *testing.T) {
	tests := []struct {
		name        string
		atoms       []*OrderedAtom
		context     *CompilationContext
		contains    []string
		notContains []string
	}{
		{
			name:     "empty atoms returns empty string",
			atoms:    nil,
			context:  NewCompilationContext(),
			contains: nil,
		},
		{
			name: "single atom",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Identity content"}, Order: 0},
			},
			context:  NewCompilationContext(),
			contains: []string{"Identity content"},
		},
		{
			name: "multiple atoms same category",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Identity A"}, Order: 0},
				{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, Content: "Identity B"}, Order: 1},
			},
			context:  NewCompilationContext(),
			contains: []string{"Identity A", "Identity B"},
		},
		{
			name: "atoms from different categories ordered correctly",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "exemplar", Category: CategoryExemplar, Content: "Exemplar"}, Order: 2},
				{Atom: &PromptAtom{ID: "identity", Category: CategoryIdentity, Content: "Identity"}, Order: 0},
				{Atom: &PromptAtom{ID: "protocol", Category: CategoryProtocol, Content: "Protocol"}, Order: 1},
			},
			context:  NewCompilationContext(),
			contains: []string{"Identity", "Protocol", "Exemplar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assembler := NewFinalAssembler()

			result, err := assembler.Assemble(tt.atoms, tt.context)

			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, result, notExpected)
			}
		})
	}
}

func TestFinalAssembler_AssembleWithHeaders(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "identity", Category: CategoryIdentity, Content: "Identity content"}, Order: 0},
		{Atom: &PromptAtom{ID: "protocol", Category: CategoryProtocol, Content: "Protocol content"}, Order: 1},
	}

	assembler := NewFinalAssembler()
	assembler.SetSectionHeaders(true)

	result, err := assembler.Assemble(atoms, NewCompilationContext())

	require.NoError(t, err)
	assert.Contains(t, result, "## Identity")
	assert.Contains(t, result, "## Protocols")
}

func TestFinalAssembler_AssembleCategoryOrder(t *testing.T) {
	t.Run("identity appears before exemplar", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "exemplar", Category: CategoryExemplar, Content: "EXEMPLAR_CONTENT"}, Order: 1},
			{Atom: &PromptAtom{ID: "identity", Category: CategoryIdentity, Content: "IDENTITY_CONTENT"}, Order: 0},
		}

		assembler := NewFinalAssembler()
		result, err := assembler.Assemble(atoms, NewCompilationContext())

		require.NoError(t, err)

		identityIdx := strings.Index(result, "IDENTITY_CONTENT")
		exemplarIdx := strings.Index(result, "EXEMPLAR_CONTENT")

		assert.Less(t, identityIdx, exemplarIdx, "Identity should appear before Exemplar")
	})

	t.Run("respects custom category order", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "identity", Category: CategoryIdentity, Content: "IDENTITY"}, Order: 0},
			{Atom: &PromptAtom{ID: "exemplar", Category: CategoryExemplar, Content: "EXEMPLAR"}, Order: 1},
		}

		assembler := NewFinalAssembler()
		assembler.SetCategoryOrder([]AtomCategory{CategoryExemplar, CategoryIdentity})

		result, err := assembler.Assemble(atoms, NewCompilationContext())

		require.NoError(t, err)

		exemplarIdx := strings.Index(result, "EXEMPLAR")
		identityIdx := strings.Index(result, "IDENTITY")

		assert.Less(t, exemplarIdx, identityIdx, "Exemplar should appear before Identity with custom order")
	})
}

func TestFinalAssembler_AssembleWithTemplates(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{
			ID:       "template-atom",
			Category: CategoryIdentity,
			Content:  "You are a {{shard_type}} agent working in {{operational_mode}} mode.",
		}, Order: 0},
	}

	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithOperationalMode("/active")

	assembler := NewFinalAssembler()
	result, err := assembler.Assemble(atoms, cc)

	require.NoError(t, err)
	assert.Contains(t, result, "coder agent")
	assert.Contains(t, result, "active mode")
	assert.NotContains(t, result, "{{shard_type}}")
	assert.NotContains(t, result, "{{operational_mode}}")
}

func TestCategoryHeader(t *testing.T) {
	tests := []struct {
		category AtomCategory
		expected string
	}{
		{CategoryIdentity, "## Identity"},
		{CategorySafety, "## Safety & Constraints"},
		{CategoryProtocol, "## Protocols"},
		{CategoryCapability, "## Capabilities"},
		{CategoryMethodology, "## Methodology"},
		{CategoryHallucination, "## Guardrails"},
		{CategoryLanguage, "## Language Guidelines"},
		{CategoryFramework, "## Framework Guidelines"},
		{CategoryDomain, "## Domain Context"},
		{CategoryCampaign, "## Campaign Context"},
		{CategoryInit, "## Initialization"},
		{CategoryNorthstar, "## Planning"},
		{CategoryOuroboros, "## Self-Improvement"},
		{CategoryAutopoiesis, "## Autopoiesis"},
		{CategoryContext, "## Current Context"},
		{CategoryReviewer, "## Reviewer Guidance"},
		{CategoryEval, "## Evaluation"},
		{CategoryExemplar, "## Examples"},
		{AtomCategory("unknown"), "## unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			result := categoryHeader(tt.category)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTemplateEngine(t *testing.T) {
	t.Run("creates engine with default functions", func(t *testing.T) {
		te := NewTemplateEngine()
		require.NotNil(t, te)
		assert.NotEmpty(t, te.functions)
	})

	t.Run("has all expected functions", func(t *testing.T) {
		te := NewTemplateEngine()

		expectedFuncs := []string{
			"language", "shard_type", "operational_mode",
			"campaign_phase", "intent_verb", "frameworks",
			"token_budget", "world_states", "available_specialists",
		}

		for _, fn := range expectedFuncs {
			_, exists := te.functions[fn]
			assert.True(t, exists, "missing function: %s", fn)
		}
	})
}

func TestTemplateEngine_RegisterFunction(t *testing.T) {
	te := NewTemplateEngine()

	customFunc := func(cc *CompilationContext, args ...string) string {
		return "custom_value"
	}

	te.RegisterFunction("custom", customFunc)

	result := te.Process("{{custom}}", nil)
	assert.Equal(t, "custom_value", result)
}

func TestTemplateEngine_Process(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		context  *CompilationContext
		expected string
	}{
		{
			name:     "no templates",
			content:  "Plain text without templates",
			context:  NewCompilationContext(),
			expected: "Plain text without templates",
		},
		{
			name:     "language template",
			content:  "Using {{language}}",
			context:  NewCompilationContext().WithLanguage("/go"),
			expected: "Using go",
		},
		{
			name:     "shard_type template",
			content:  "You are a {{shard_type}}",
			context:  NewCompilationContext().WithShard("/coder", "", ""),
			expected: "You are a coder",
		},
		{
			name:     "operational_mode template",
			content:  "Mode: {{operational_mode}}",
			context:  NewCompilationContext().WithOperationalMode("/debugging"),
			expected: "Mode: debugging",
		},
		{
			name:     "campaign_phase template",
			content:  "Phase: {{campaign_phase}}",
			context:  NewCompilationContext().WithCampaign("", "", "/planning"),
			expected: "Phase: planning",
		},
		{
			name:     "intent_verb template",
			content:  "Action: {{intent_verb}}",
			context:  NewCompilationContext().WithIntent("/fix", ""),
			expected: "Action: fix",
		},
		{
			name:     "frameworks template",
			content:  "Using: {{frameworks}}",
			context:  NewCompilationContext().WithLanguage("/go", "/bubbletea", "/gin"),
			expected: "Using: bubbletea, gin",
		},
		{
			name:     "token_budget template",
			content:  "Budget: {{token_budget}}",
			context:  NewCompilationContext().WithTokenBudget(50000, 5000),
			expected: "Budget: 45000",
		},
		{
			name:     "world_states template - normal",
			content:  "States: {{world_states}}",
			context:  NewCompilationContext(),
			expected: "States: normal",
		},
		{
			name:    "world_states template - with issues",
			content: "States: {{world_states}}",
			context: &CompilationContext{
				FailingTestCount: 5,
				DiagnosticCount:  3,
			},
			expected: "States: failing_tests, diagnostics",
		},
		{
			name:     "available_specialists template",
			content:  "Specialists:\n{{available_specialists}}",
			context:  &CompilationContext{AvailableSpecialists: "- **goexpert**: Go specialist"},
			expected: "Specialists:\n- **goexpert**: Go specialist",
		},
		{
			name:     "multiple templates",
			content:  "{{shard_type}} using {{language}} in {{operational_mode}} mode",
			context:  NewCompilationContext().WithShard("/coder", "", "").WithLanguage("/go").WithOperationalMode("/active"),
			expected: "coder using go in active mode",
		},
		{
			name:     "nil context uses defaults",
			content:  "Mode: {{operational_mode}}",
			context:  nil,
			expected: "Mode: active",
		},
		{
			name:     "empty language fallback",
			content:  "Lang: {{language}}",
			context:  &CompilationContext{},
			expected: "Lang: unknown",
		},
		{
			name:     "empty shard_type fallback",
			content:  "Type: {{shard_type}}",
			context:  &CompilationContext{},
			expected: "Type: agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := NewTemplateEngine()
			result := te.Process(tt.content, tt.context)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTemplateEngine_ProcessFastPath(t *testing.T) {
	te := NewTemplateEngine()

	// Content without {{ should be returned unchanged
	content := "No templates here"
	result := te.Process(content, nil)

	assert.Equal(t, content, result)
}

func TestFinalAssembler_Assemble_InjectsAvailableSpecialists(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	registry := `{"agents":[{"name":"goexpert","type":"persistent","status":"ready","description":"Go specialist"}]}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".nerd", "agents.json"), []byte(registry), 0o644); err != nil {
		t.Fatalf("write agents.json failed: %v", err)
	}

	t.Chdir(tmpDir)

	assembler := NewFinalAssembler()
	cc := NewCompilationContext()
	prompt, err := assembler.Assemble([]*OrderedAtom{
		{Atom: &PromptAtom{ID: "knowledge", Category: CategoryKnowledge, Content: "{{available_specialists}}"}, Order: 0},
	}, cc)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	if !strings.Contains(prompt, "goexpert") {
		t.Fatalf("expected assembled prompt to include discovered specialist, got %q", prompt)
	}
}

// Boundary Value Analysis: Remediated. See assembler_gaps_test.go for comprehensive coverage.
// Findings: TemplateEngine + FinalAssembler data races fixed with sync.RWMutex.
// Known bug: truncatePrompt slices at byte boundaries (see assembler.go TODO).

// Benchmark tests

func BenchmarkAssemble_SmallSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 10)
	for i := range 10 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: CategoryIdentity, Content: "Content " + string(rune(i))},
			Order: i,
		}
	}

	assembler := NewFinalAssembler()
	cc := NewCompilationContext()
	b.ResetTimer()

	for b.Loop() {
		_, _ = assembler.Assemble(atoms, cc)
	}
}

func BenchmarkAssemble_MediumSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 50)
	categories := AllCategories()
	for i := range 50 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: categories[i%len(categories)], Content: strings.Repeat("content ", 20)},
			Order: i,
		}
	}

	assembler := NewFinalAssembler()
	cc := NewCompilationContext()
	b.ResetTimer()

	for b.Loop() {
		_, _ = assembler.Assemble(atoms, cc)
	}
}

func BenchmarkAssemble_WithTemplates(b *testing.B) {
	atoms := make([]*OrderedAtom, 20)
	for i := range 20 {
		atoms[i] = &OrderedAtom{
			Atom: &PromptAtom{
				ID:       string(rune(i)),
				Category: CategoryIdentity,
				Content:  "You are a {{shard_type}} using {{language}} in {{operational_mode}} mode.",
			},
			Order: i,
		}
	}

	assembler := NewFinalAssembler()
	cc := NewCompilationContext().WithShard("/coder", "", "").WithLanguage("/go").WithOperationalMode("/active")
	b.ResetTimer()

	for b.Loop() {
		_, _ = assembler.Assemble(atoms, cc)
	}
}

func BenchmarkTemplateProcess(b *testing.B) {
	te := NewTemplateEngine()
	content := "{{shard_type}} {{language}} {{operational_mode}} {{frameworks}} {{token_budget}}"
	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithLanguage("/go", "/bubbletea", "/gin").
		WithOperationalMode("/active").
		WithTokenBudget(100000, 8000)

	b.ResetTimer()

	for b.Loop() {
		te.Process(content, cc)
	}
}

func TestAssemble_UnknownCategoriesSortDeterministically(t *testing.T) {
	mk := func(id string, cat AtomCategory) *OrderedAtom {
		return &OrderedAtom{Atom: &PromptAtom{ID: id, Category: cat, Content: "body-" + id}, Order: 0, RenderMode: "standard"}
	}
	atoms := []*OrderedAtom{
		mk("z1", AtomCategory("zzz_custom")),
		mk("a1", AtomCategory("aaa_custom")),
		mk("m1", AtomCategory("mmm_custom")),
	}
	assembler := NewFinalAssembler()
	first, err := assembler.Assemble(atoms, NewCompilationContext())
	require.NoError(t, err)
	ia := strings.Index(first, "body-a1")
	im := strings.Index(first, "body-m1")
	iz := strings.Index(first, "body-z1")
	require.NotEqual(t, -1, ia)
	assert.True(t, ia < im && im < iz, "unknown categories must emit sorted, got:\n%s", first)
	for i := 0; i < 20; i++ {
		again, err := assembler.Assemble(atoms, NewCompilationContext())
		require.NoError(t, err)
		assert.Equal(t, first, again, "assembly must be stable across runs")
	}
}

func TestAssemble_SystemCategoryRendersBeforeContext(t *testing.T) {
	mk := func(id string, cat AtomCategory) *OrderedAtom {
		return &OrderedAtom{Atom: &PromptAtom{ID: id, Category: cat, Content: "body-" + id}, Order: 0, RenderMode: "standard"}
	}
	atoms := []*OrderedAtom{
		mk("s1", CategorySystem),
		mk("c1", CategoryContext),
		mk("i1", CategoryIdentity),
	}
	out, err := NewFinalAssembler().Assemble(atoms, NewCompilationContext())
	require.NoError(t, err)
	ii := strings.Index(out, "body-i1")
	ic := strings.Index(out, "body-c1")
	is := strings.Index(out, "body-s1")
	require.NotEqual(t, -1, is)
	assert.True(t, ii < is && is < ic, "system renders ahead of context (context stays last), got:\n%s", out)
}

func TestAssemble_SkipsNilAtoms(t *testing.T) {
	atoms := []*OrderedAtom{
		nil,
		{Atom: nil},
		{Atom: &PromptAtom{ID: "ok", Category: CategoryIdentity, Content: "hello"}, Order: 0, RenderMode: "standard"},
	}
	out, err := NewFinalAssembler().Assemble(atoms, NewCompilationContext())
	require.NoError(t, err)
	assert.Contains(t, out, "hello")
}
