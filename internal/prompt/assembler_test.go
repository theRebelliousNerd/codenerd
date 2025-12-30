package prompt

import (
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

	t.Run("exemplar comes last", func(t *testing.T) {
		assert.Equal(t, CategoryExemplar, order[len(order)-1])
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
			"token_budget", "world_states",
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

func TestDefaultAssemblyOptions(t *testing.T) {
	opts := DefaultAssemblyOptions()

	assert.False(t, opts.IncludeSectionHeaders)
	assert.False(t, opts.MinifyWhitespace)
	assert.False(t, opts.IncludeMetadata)
	assert.Equal(t, 0, opts.MaxLength)
}

func TestFinalAssembler_AssembleWithOptions(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "identity", Category: CategoryIdentity, Content: "Identity content"}, Order: 0},
		{Atom: &PromptAtom{ID: "protocol", Category: CategoryProtocol, Content: "Protocol content"}, Order: 1},
	}

	t.Run("with section headers", func(t *testing.T) {
		assembler := NewFinalAssembler()
		opts := AssemblyOptions{IncludeSectionHeaders: true}

		result, err := assembler.AssembleWithOptions(atoms, NewCompilationContext(), opts)

		require.NoError(t, err)
		assert.Contains(t, result, "## Identity")
	})

	t.Run("with minified whitespace", func(t *testing.T) {
		atomsWithWhitespace := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "Line 1\n\n\n\nLine 2"}, Order: 0},
		}

		assembler := NewFinalAssembler()
		opts := AssemblyOptions{MinifyWhitespace: true}

		result, err := assembler.AssembleWithOptions(atomsWithWhitespace, NewCompilationContext(), opts)

		require.NoError(t, err)
		assert.NotContains(t, result, "\n\n\n\n")
		assert.Contains(t, result, "\n\n")
	})

	t.Run("with max length truncation", func(t *testing.T) {
		longAtoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "long", Category: CategoryIdentity, Content: strings.Repeat("a", 1000)}, Order: 0},
		}

		assembler := NewFinalAssembler()
		opts := AssemblyOptions{MaxLength: 100}

		result, err := assembler.AssembleWithOptions(longAtoms, NewCompilationContext(), opts)

		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 200) // Some overhead for truncation message
		assert.Contains(t, result, "[Content truncated")
	})
}

func TestMinifyWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "triple newlines reduced to double",
			input:    "a\n\n\nb",
			expected: "a\n\nb",
		},
		{
			name:     "quadruple newlines reduced",
			input:    "a\n\n\n\nb",
			expected: "a\n\nb",
		},
		{
			name:     "trailing whitespace removed",
			input:    "a   \nb  \t\nc",
			expected: "a\nb\nc",
		},
		{
			name:     "double newlines preserved",
			input:    "a\n\nb",
			expected: "a\n\nb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := minifyWhitespace(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncatePrompt(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		maxLen         int
		shouldTruncate bool
		containsMsg    bool
	}{
		{
			name:           "no truncation needed",
			content:        "short content",
			maxLen:         100,
			shouldTruncate: false,
			containsMsg:    false,
		},
		{
			name:           "truncation at paragraph",
			content:        "First paragraph.\n\nSecond paragraph.\n\nThird paragraph that is very long.",
			maxLen:         40,
			shouldTruncate: true,
			containsMsg:    true,
		},
		{
			name:           "truncation message added",
			content:        strings.Repeat("a", 200),
			maxLen:         100,
			shouldTruncate: true,
			containsMsg:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePrompt(tt.content, tt.maxLen)

			if tt.shouldTruncate {
				// Result might be longer due to truncation message
				assert.LessOrEqual(t, len(result), tt.maxLen+100)
			} else {
				assert.Equal(t, tt.content, result)
			}

			if tt.containsMsg {
				assert.Contains(t, result, "truncated")
			}
		})
	}
}

func TestAnalyzePrompt(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategoryIdentity, IsMandatory: true, Content: "Short"}, Order: 0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, IsMandatory: false, Content: "Medium content here"}, Order: 1},
		{Atom: &PromptAtom{ID: "c", Category: CategoryProtocol, IsMandatory: false, Content: strings.Repeat("x", 100)}, Order: 2},
	}

	prompt := "Short\n\nMedium content here\n\n" + strings.Repeat("x", 100)

	stats := AnalyzePrompt(prompt, atoms)

	t.Run("char count", func(t *testing.T) {
		assert.Equal(t, len(prompt), stats.CharCount)
	})

	t.Run("token count estimated", func(t *testing.T) {
		assert.Greater(t, stats.TokenCount, 0)
	})

	t.Run("line count", func(t *testing.T) {
		assert.Greater(t, stats.LineCount, 0)
	})

	t.Run("atom count", func(t *testing.T) {
		assert.Equal(t, 3, stats.AtomCount)
	})

	t.Run("category counts", func(t *testing.T) {
		assert.Equal(t, 2, stats.CategoryCounts[CategoryIdentity])
		assert.Equal(t, 1, stats.CategoryCounts[CategoryProtocol])
	})

	t.Run("section count", func(t *testing.T) {
		assert.Equal(t, 2, stats.SectionCount)
	})

	t.Run("mandatory count", func(t *testing.T) {
		assert.Equal(t, 1, stats.MandatoryCount)
	})

	t.Run("longest atom length", func(t *testing.T) {
		assert.Equal(t, 100, stats.LongestAtomLen)
	})

	t.Run("shortest atom length", func(t *testing.T) {
		assert.Equal(t, 5, stats.ShortestAtomLen)
	})
}

// Benchmark tests

func BenchmarkAssemble_SmallSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 10)
	for i := 0; i < 10; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: CategoryIdentity, Content: "Content " + string(rune(i))},
			Order: i,
		}
	}

	assembler := NewFinalAssembler()
	cc := NewCompilationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = assembler.Assemble(atoms, cc)
	}
}

func BenchmarkAssemble_MediumSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 50)
	categories := AllCategories()
	for i := 0; i < 50; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: categories[i%len(categories)], Content: strings.Repeat("content ", 20)},
			Order: i,
		}
	}

	assembler := NewFinalAssembler()
	cc := NewCompilationContext()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = assembler.Assemble(atoms, cc)
	}
}

func BenchmarkAssemble_WithTemplates(b *testing.B) {
	atoms := make([]*OrderedAtom, 20)
	for i := 0; i < 20; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
		te.Process(content, cc)
	}
}

func BenchmarkAnalyzePrompt(b *testing.B) {
	prompt := strings.Repeat("This is a test prompt with some content. ", 100)
	atoms := make([]*OrderedAtom, 20)
	categories := AllCategories()
	for i := 0; i < 20; i++ {
		atoms[i] = &OrderedAtom{
			Atom: &PromptAtom{
				ID:          string(rune(i)),
				Category:    categories[i%len(categories)],
				IsMandatory: i%5 == 0,
				Content:     strings.Repeat("x", 50+i*10),
			},
			Order: i,
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		AnalyzePrompt(prompt, atoms)
	}
}
