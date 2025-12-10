package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetPriority_String(t *testing.T) {
	tests := []struct {
		priority BudgetPriority
		expected string
	}{
		{PriorityMandatory, "mandatory"},
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
		{PriorityConditional, "conditional"},
		{BudgetPriority(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.String())
		})
	}
}

func TestNewTokenBudgetManager(t *testing.T) {
	t.Run("creates manager with defaults", func(t *testing.T) {
		mgr := NewTokenBudgetManager()
		require.NotNil(t, mgr)

		assert.Equal(t, StrategyPriorityFirst, mgr.strategy)
		assert.Equal(t, 500, mgr.reservedHeadroom)
		assert.NotEmpty(t, mgr.budgets)
	})

	t.Run("has default budgets for all categories", func(t *testing.T) {
		mgr := NewTokenBudgetManager()

		expectedCategories := []AtomCategory{
			CategorySafety,
			CategoryIdentity,
			CategoryProtocol,
			CategoryMethodology,
			CategoryHallucination,
			CategoryLanguage,
			CategoryFramework,
			CategoryDomain,
			CategoryContext,
			CategoryCampaign,
			CategoryInit,
			CategoryNorthstar,
			CategoryOuroboros,
			CategoryExemplar,
		}

		for _, cat := range expectedCategories {
			_, exists := mgr.budgets[cat]
			assert.True(t, exists, "missing budget for category: %s", cat)
		}
	})
}

func TestTokenBudgetManager_SetCategoryBudget(t *testing.T) {
	mgr := NewTokenBudgetManager()

	customBudget := CategoryBudget{
		Category:    CategoryDomain,
		BasePercent: 0.25,
		MinTokens:   2000,
		MaxTokens:   25000,
		Priority:    PriorityHigh,
	}

	mgr.SetCategoryBudget(customBudget)

	assert.Equal(t, customBudget, mgr.budgets[CategoryDomain])
}

func TestTokenBudgetManager_SetStrategy(t *testing.T) {
	mgr := NewTokenBudgetManager()

	mgr.SetStrategy(StrategyBalanced)
	assert.Equal(t, StrategyBalanced, mgr.strategy)

	mgr.SetStrategy(StrategyProportional)
	assert.Equal(t, StrategyProportional, mgr.strategy)
}

func TestTokenBudgetManager_SetReservedHeadroom(t *testing.T) {
	mgr := NewTokenBudgetManager()

	mgr.SetReservedHeadroom(1000)
	assert.Equal(t, 1000, mgr.reservedHeadroom)
}

func TestTokenBudgetManager_Fit(t *testing.T) {
	tests := []struct {
		name          string
		atoms         []*OrderedAtom
		budget        int
		expectedLen   int
		expectError   bool
		checkContains []string
	}{
		{
			name:        "empty input",
			atoms:       nil,
			budget:      1000,
			expectedLen: 0,
		},
		{
			name: "all atoms fit within budget",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "a", TokenCount: 100, Category: CategoryIdentity}, Score: 0.8, Order: 0},
				{Atom: &PromptAtom{ID: "b", TokenCount: 200, Category: CategoryProtocol}, Score: 0.7, Order: 1},
				{Atom: &PromptAtom{ID: "c", TokenCount: 300, Category: CategoryMethodology}, Score: 0.6, Order: 2},
			},
			budget:        1500,
			expectedLen:   3,
			checkContains: []string{"a", "b", "c"},
		},
		{
			name: "atoms within reasonable budget",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "a", TokenCount: 100, Category: CategoryIdentity}, Score: 0.9, Order: 0},
				{Atom: &PromptAtom{ID: "b", TokenCount: 200, Category: CategoryProtocol}, Score: 0.8, Order: 1},
				{Atom: &PromptAtom{ID: "c", TokenCount: 500, Category: CategoryMethodology}, Score: 0.7, Order: 2},
			},
			budget:      1500, // 1500 - 500 headroom = 1000 available, should fit all 800 tokens
			expectedLen: 3,
		},
		{
			name: "budget less than headroom",
			atoms: []*OrderedAtom{
				{Atom: &PromptAtom{ID: "a", TokenCount: 100, Category: CategoryIdentity}, Score: 0.8, Order: 0},
			},
			budget:      400, // Less than default 500 headroom
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewTokenBudgetManager()

			result, err := mgr.Fit(tt.atoms, tt.budget)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedLen)

			if tt.checkContains != nil {
				ids := make([]string, len(result))
				for i, oa := range result {
					ids[i] = oa.Atom.ID
				}
				for _, expected := range tt.checkContains {
					assert.Contains(t, ids, expected)
				}
			}
		})
	}
}

func TestTokenBudgetManager_FitMandatory(t *testing.T) {
	t.Run("mandatory atoms always included", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "mandatory1", TokenCount: 300, Category: CategorySafety, IsMandatory: true}, Score: 0.5, Order: 0},
			{Atom: &PromptAtom{ID: "optional1", TokenCount: 100, Category: CategoryExemplar, IsMandatory: false}, Score: 0.9, Order: 1},
		}

		mgr := NewTokenBudgetManager()
		// Set low headroom for this test
		mgr.SetReservedHeadroom(100)

		result, err := mgr.Fit(atoms, 500) // 500 - 100 = 400 available

		require.NoError(t, err)

		// Mandatory should always be included
		var hasMandatory bool
		for _, oa := range result {
			if oa.Atom.ID == "mandatory1" {
				hasMandatory = true
			}
		}
		assert.True(t, hasMandatory, "mandatory atom should be included")
	})

	t.Run("mandatory even if it exceeds category allocation", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "mandatory1", TokenCount: 5000, Category: CategorySafety, IsMandatory: true}, Score: 0.5, Order: 0},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 10000)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "mandatory1", result[0].Atom.ID)
	})
}

func TestTokenBudgetManager_FitHigherScoresPreferred(t *testing.T) {
	t.Run("higher scored atoms preferred within category", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "low", TokenCount: 1000, Category: CategoryIdentity}, Score: 0.3, Order: 0},
			{Atom: &PromptAtom{ID: "high", TokenCount: 1000, Category: CategoryIdentity}, Score: 0.9, Order: 1},
			{Atom: &PromptAtom{ID: "medium", TokenCount: 1000, Category: CategoryIdentity}, Score: 0.6, Order: 2},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		// Set custom budget that allows only one atom
		mgr.SetCategoryBudget(CategoryBudget{
			Category:    CategoryIdentity,
			BasePercent: 0.5,
			MinTokens:   500,
			MaxTokens:   1500, // Only one atom can fit
			Priority:    PriorityMandatory,
		})

		result, err := mgr.Fit(atoms, 3000)

		require.NoError(t, err)

		// Should include the highest scored
		var found bool
		for _, oa := range result {
			if oa.Atom.ID == "high" {
				found = true
			}
		}
		assert.True(t, found, "highest scored atom should be included")
	})
}

func TestTokenBudgetManager_FitFillsRemainingBudget(t *testing.T) {
	t.Run("remaining budget filled with best unselected atoms", func(t *testing.T) {
		// Create atoms across categories with varying scores
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "identity1", TokenCount: 100, Category: CategoryIdentity}, Score: 0.8, Order: 0},
			{Atom: &PromptAtom{ID: "protocol1", TokenCount: 100, Category: CategoryProtocol}, Score: 0.7, Order: 1},
			{Atom: &PromptAtom{ID: "exemplar1", TokenCount: 100, Category: CategoryExemplar}, Score: 0.9, Order: 2},
			{Atom: &PromptAtom{ID: "exemplar2", TokenCount: 100, Category: CategoryExemplar}, Score: 0.5, Order: 3},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 10000)

		require.NoError(t, err)

		// All atoms should be included since budget is large
		assert.Len(t, result, 4)
	})
}

func TestTokenBudgetManager_AllocationStrategies(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "identity", TokenCount: 500, Category: CategoryIdentity}, Score: 0.8, Order: 0},
		{Atom: &PromptAtom{ID: "protocol", TokenCount: 500, Category: CategoryProtocol}, Score: 0.7, Order: 1},
		{Atom: &PromptAtom{ID: "domain", TokenCount: 500, Category: CategoryDomain}, Score: 0.6, Order: 2},
	}

	t.Run("proportional strategy", func(t *testing.T) {
		mgr := NewTokenBudgetManager()
		mgr.SetStrategy(StrategyProportional)
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 5000)

		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("priority first strategy", func(t *testing.T) {
		mgr := NewTokenBudgetManager()
		mgr.SetStrategy(StrategyPriorityFirst)
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 5000)

		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("balanced strategy", func(t *testing.T) {
		mgr := NewTokenBudgetManager()
		mgr.SetStrategy(StrategyBalanced)
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 5000)

		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

func TestTokenBudgetManager_GenerateReport(t *testing.T) {
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", TokenCount: 100, Category: CategoryIdentity, IsMandatory: true}, Score: 0.8, Order: 0},
		{Atom: &PromptAtom{ID: "b", TokenCount: 200, Category: CategoryProtocol, IsMandatory: false}, Score: 0.7, Order: 1},
		{Atom: &PromptAtom{ID: "c", TokenCount: 150, Category: CategoryIdentity, IsMandatory: false}, Score: 0.6, Order: 2},
	}

	mgr := NewTokenBudgetManager()
	report := mgr.GenerateReport(atoms, 1000)

	t.Run("calculates total budget correctly", func(t *testing.T) {
		assert.Equal(t, 1000, report.TotalBudget)
	})

	t.Run("calculates used tokens correctly", func(t *testing.T) {
		assert.Equal(t, 450, report.UsedTokens)
	})

	t.Run("calculates remaining tokens correctly", func(t *testing.T) {
		assert.Equal(t, 550, report.RemainingTokens)
	})

	t.Run("tracks mandatory vs optional tokens", func(t *testing.T) {
		assert.Equal(t, 100, report.MandatoryTokens)
		assert.Equal(t, 350, report.OptionalTokens)
	})

	t.Run("tracks category usage", func(t *testing.T) {
		identityUsage := report.CategoryUsage[CategoryIdentity]
		assert.Equal(t, 250, identityUsage.Used)
		assert.Equal(t, 2, identityUsage.AtomCount)

		protocolUsage := report.CategoryUsage[CategoryProtocol]
		assert.Equal(t, 200, protocolUsage.Used)
		assert.Equal(t, 1, protocolUsage.AtomCount)
	})

	t.Run("detects over budget condition", func(t *testing.T) {
		smallBudgetReport := mgr.GenerateReport(atoms, 300)

		assert.Greater(t, smallBudgetReport.OverBudgetAmount, 0)
		assert.Equal(t, 0, smallBudgetReport.RemainingTokens)
	})
}

func TestTokenBudgetManager_CategoriesByPriority(t *testing.T) {
	mgr := NewTokenBudgetManager()
	categories := mgr.categoriesByPriority()

	t.Run("returns all categories", func(t *testing.T) {
		assert.NotEmpty(t, categories)
	})

	t.Run("mandatory categories come first", func(t *testing.T) {
		// Find first non-mandatory category
		var firstNonMandatoryIdx int
		for i, cat := range categories {
			budget := mgr.budgets[cat]
			if budget.Priority != PriorityMandatory {
				firstNonMandatoryIdx = i
				break
			}
		}

		// All categories before that index should be mandatory
		for i := 0; i < firstNonMandatoryIdx; i++ {
			budget := mgr.budgets[categories[i]]
			assert.Equal(t, PriorityMandatory, budget.Priority)
		}
	})
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		min      int
		max      int
		expected int
	}{
		{
			name:     "value within range",
			value:    50,
			min:      0,
			max:      100,
			expected: 50,
		},
		{
			name:     "value below min",
			value:    -10,
			min:      0,
			max:      100,
			expected: 0,
		},
		{
			name:     "value above max",
			value:    150,
			min:      0,
			max:      100,
			expected: 100,
		},
		{
			name:     "value equals min",
			value:    0,
			min:      0,
			max:      100,
			expected: 0,
		},
		{
			name:     "value equals max",
			value:    100,
			min:      0,
			max:      100,
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clamp(tt.value, tt.min, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests

func BenchmarkFit_SmallSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 20)
	categories := AllCategories()
	for i := 0; i < 20; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 100 + i*10, Category: categories[i%len(categories)]},
			Score: float64(i) / 20.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = mgr.Fit(atoms, 10000)
	}
}

func BenchmarkFit_MediumSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 100)
	categories := AllCategories()
	for i := 0; i < 100; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 50 + i*5, Category: categories[i%len(categories)]},
			Score: float64(i) / 100.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = mgr.Fit(atoms, 50000)
	}
}

func BenchmarkFit_LargeSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 500)
	categories := AllCategories()
	for i := 0; i < 500; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 100, Category: categories[i%len(categories)]},
			Score: float64(i) / 500.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = mgr.Fit(atoms, 100000)
	}
}

func BenchmarkGenerateReport(b *testing.B) {
	atoms := make([]*OrderedAtom, 50)
	categories := AllCategories()
	for i := 0; i < 50; i++ {
		atoms[i] = &OrderedAtom{
			Atom: &PromptAtom{
				ID:          string(rune(i)),
				TokenCount:  100,
				Category:    categories[i%len(categories)],
				IsMandatory: i%10 == 0,
			},
			Score: float64(i) / 50.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mgr.GenerateReport(atoms, 10000)
	}
}
