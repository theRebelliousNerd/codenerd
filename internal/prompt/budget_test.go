package prompt

import (
	"math"
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
			CategoryCapability,
			CategoryHallucination,
			CategoryLanguage,
			CategoryFramework,
			CategoryDomain,
			CategoryContext,
			CategoryCampaign,
			CategoryInit,
			CategoryNorthstar,
			CategoryOuroboros,
			CategoryAutopoiesis,
			CategoryEval,
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
	for i := range 20 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 100 + i*10, Category: categories[i%len(categories)]},
			Score: float64(i) / 20.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for b.Loop() {
		_, _ = mgr.Fit(atoms, 10000)
	}
}

func BenchmarkFit_MediumSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 100)
	categories := AllCategories()
	for i := range 100 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 50 + i*5, Category: categories[i%len(categories)]},
			Score: float64(i) / 100.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for b.Loop() {
		_, _ = mgr.Fit(atoms, 50000)
	}
}

func BenchmarkFit_LargeSet(b *testing.B) {
	atoms := make([]*OrderedAtom, 500)
	categories := AllCategories()
	for i := range 500 {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), TokenCount: 100, Category: categories[i%len(categories)]},
			Score: float64(i) / 500.0,
			Order: i,
		}
	}

	mgr := NewTokenBudgetManager()
	b.ResetTimer()

	for b.Loop() {
		_, _ = mgr.Fit(atoms, 100000)
	}
}

func BenchmarkGenerateReport(b *testing.B) {
	atoms := make([]*OrderedAtom, 50)
	categories := AllCategories()
	for i := range 50 {
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

	for b.Loop() {
		mgr.GenerateReport(atoms, 10000)
	}
}

// TODO: TEST_GAP: Null/Undefined/Empty: Test empty or nil configurations such as `budgets` being empty map, `atoms` array being completely nil rather than just containing nil items, and `totalBudget` being 0 or negative.
// TODO: TEST_GAP: Type Coercion / Precision Loss: `totalBudget` interacting with `budget.BasePercent` (float64) can lead to truncation/precision loss. Test with odd numbers where exact float division loses fractional tokens.
// TODO: TEST_GAP: User Request Extremes: Test `totalBudget` near math.MaxInt (or math.MaxInt32). Validate that calculating `int(float64(totalBudget) * budget.BasePercent)` does not overflow int causing negative allocations.
// TODO: TEST_GAP: User Request Extremes: Test with >5000 items in `atoms` to ensure the `maxAtomsLimit` branch executes correctly and unselected atoms are correctly bypassed or skipped.
// TODO: TEST_GAP: User Request Extremes: Validate token addition (`catTokens += tokens`) behaves correctly if an atom is artificially crafted with near `math.MaxInt64` tokens (testing for overflow wrap-around in int64 math).
// TODO: TEST_GAP: State Conflicts: Test concurrent access. Run `Fit` or `GenerateReport` in multiple goroutines while simultaneously calling `SetCategoryBudget`, `SetStrategy`, or `SetReservedHeadroom` to ensure `m.mu` locking prevents race conditions and crashes.

// TestTokenBudgetManager_Fit_Extremes covers boundary inputs:
// - Massive atom count that exceeds the input cap (truncation path)
// - Zero / negative budgets that should error
// - Budget that equals reservedHeadroom (boundary on the <= guard)
// GAP: User Request Extremes from boundary analysis QA.
func TestTokenBudgetManager_Fit_Extremes(t *testing.T) {
	t.Run("massive atom count is truncated without panic", func(t *testing.T) {
		// Build > maxAtomsInput atoms to exercise the truncation guard.
		count := maxAtomsInput + 1234
		atoms := make([]*OrderedAtom, count)
		for i := range count {
			atoms[i] = &OrderedAtom{
				Atom: &PromptAtom{
					ID:         "atom",
					TokenCount: 1,
					Category:   CategoryIdentity,
				},
				Score: 0.5,
				Order: i,
			}
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		// Should not panic and should return a non-error result.
		result, err := mgr.Fit(atoms, 1000000)
		require.NoError(t, err)
		// Result is capped by maxAtomsLimit (5000) inside Fit.
		assert.LessOrEqual(t, len(result), 5000)
	})

	t.Run("zero budget returns error", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "a", TokenCount: 10, Category: CategoryIdentity}, Score: 1.0},
		}
		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		_, err := mgr.Fit(atoms, 0)
		require.Error(t, err)
	})

	t.Run("negative budget returns error", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "a", TokenCount: 10, Category: CategoryIdentity}, Score: 1.0},
		}
		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		_, err := mgr.Fit(atoms, -1)
		require.Error(t, err)

		_, err2 := mgr.Fit(atoms, math.MinInt32)
		require.Error(t, err2)
	})

	t.Run("budget equal to reserved headroom errors", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "a", TokenCount: 10, Category: CategoryIdentity}, Score: 1.0},
		}
		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(500)

		_, err := mgr.Fit(atoms, 500)
		require.Error(t, err)
	})

	t.Run("very large per-atom token count does not overflow", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{
				ID:         "huge",
				TokenCount: math.MaxInt32, // close to but not at MaxInt64
				Category:   CategoryIdentity,
			}, Score: 1.0},
			{Atom: &PromptAtom{
				ID:         "small",
				TokenCount: 10,
				Category:   CategoryIdentity,
			}, Score: 0.9},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		// Should not panic / loop forever; either fit small, or skip both.
		result, err := mgr.Fit(atoms, 10000)
		require.NoError(t, err)
		// The huge atom must not be wrongly included via overflow.
		for _, oa := range result {
			assert.NotEqual(t, "huge", oa.Atom.ID, "oversized atom should not be selected")
		}
	})
}

// TestTokenBudgetManager_SetReservedHeadroom_Negative verifies that
// negative reserved headroom is rejected (clamped to 0) rather than
// silently inflating the available budget.
// GAP: Null/Undefined/Empty from boundary analysis QA.
func TestTokenBudgetManager_SetReservedHeadroom_Negative(t *testing.T) {
	mgr := NewTokenBudgetManager()

	mgr.SetReservedHeadroom(-100)
	assert.Equal(t, 0, mgr.reservedHeadroom, "negative headroom should clamp to 0")

	mgr.SetReservedHeadroom(math.MinInt32)
	assert.Equal(t, 0, mgr.reservedHeadroom, "very negative headroom should clamp to 0")

	// Non-negative values pass through unchanged.
	mgr.SetReservedHeadroom(0)
	assert.Equal(t, 0, mgr.reservedHeadroom)

	mgr.SetReservedHeadroom(750)
	assert.Equal(t, 750, mgr.reservedHeadroom)
}

// TestTokenBudgetManager_Fit_MandatoryOverflow verifies that a single
// mandatory atom larger than the total budget is skipped (with a warning)
// rather than included and exploding the context window.
// GAP: Enormous single atom from boundary analysis QA.
func TestTokenBudgetManager_Fit_MandatoryOverflow(t *testing.T) {
	t.Run("oversized mandatory atom is skipped", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{
				ID:          "huge-mandatory",
				TokenCount:  2_000_000, // far exceeds budget
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 1.0},
			{Atom: &PromptAtom{
				ID:          "small-mandatory",
				TokenCount:  100,
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 0.9},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		result, err := mgr.Fit(atoms, 8000)
		require.NoError(t, err)

		// huge-mandatory must not appear; small-mandatory should.
		var sawHuge, sawSmall bool
		for _, oa := range result {
			if oa.Atom.ID == "huge-mandatory" {
				sawHuge = true
			}
			if oa.Atom.ID == "small-mandatory" {
				sawSmall = true
			}
		}
		assert.False(t, sawHuge, "oversized mandatory atom should be skipped")
		assert.True(t, sawSmall, "in-budget mandatory atom should be included")
	})

	t.Run("mandatory atom at MaxInt64 token count does not overflow", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{
				ID:          "max-int",
				TokenCount:  math.MaxInt64 - 10,
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 1.0},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		// Must not panic or wrap. Atom should be skipped because it cannot fit.
		result, err := mgr.Fit(atoms, 10000)
		require.NoError(t, err)
		for _, oa := range result {
			assert.NotEqual(t, "max-int", oa.Atom.ID)
		}
	})

	t.Run("multiple mandatory atoms summing past budget are partially included", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{
				ID:          "m1",
				TokenCount:  5000,
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 1.0},
			{Atom: &PromptAtom{
				ID:          "m2",
				TokenCount:  5000,
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 0.9},
			{Atom: &PromptAtom{
				ID:          "m3",
				TokenCount:  5000,
				Category:    CategorySafety,
				IsMandatory: true,
			}, Score: 0.8},
		}

		mgr := NewTokenBudgetManager()
		mgr.SetReservedHeadroom(0)

		// Budget allows two but not three.
		result, err := mgr.Fit(atoms, 11000)
		require.NoError(t, err)
		// At most two of the three mandatory atoms fit; the third must be skipped.
		assert.LessOrEqual(t, len(result), 2)
	})
}

func TestTokenBudgetManager_Fit_InvalidData(t *testing.T) {
	manager := NewTokenBudgetManager()

	atoms := []*OrderedAtom{
		nil, // Nil OrderedAtom pointer
		{
			Atom:  nil, // Nil PromptAtom pointer
			Score: 1.0,
		},
		{
			Atom: &PromptAtom{
				Category:   CategoryCapability, // using a known category
				TokenCount: -10,                // Negative TokenCount
			},
			Score: 0.8,
		},
	}

	// Make sure total budget > reserved headroom (500)
	fitted, err := manager.Fit(atoms, 1000)
	require.NoError(t, err)

	// Ensure no nil pointers were appended and negative token counts were defaulted to 0
	assert.Len(t, fitted, 1)
	assert.Equal(t, 0, fitted[0].Atom.TokenCount)
}

func TestTokenBudgetManager_calculateAllocations(t *testing.T) {
	t.Run("proportional exact distribution", func(t *testing.T) {
		m := &TokenBudgetManager{
			strategy: StrategyProportional,
			budgets: map[AtomCategory]CategoryBudget{
				CategoryInit:      {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100},
				CategoryContext:   {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100},
				CategoryNorthstar: {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100},
			},
		}

		totalBudget := 100
		presentCategories := map[AtomCategory]bool{
			CategoryInit:      true,
			CategoryContext:   true,
			CategoryNorthstar: true,
		}

		allocs := m.calculateAllocations(totalBudget, presentCategories)

		totalAllocated := 0
		for _, v := range allocs {
			totalAllocated += v
		}

		assert.Equal(t, 100, totalAllocated)
	})

	t.Run("priority first handles remaining correctly", func(t *testing.T) {
		m := &TokenBudgetManager{
			strategy: StrategyPriorityFirst,
			budgets: map[AtomCategory]CategoryBudget{
				CategoryInit:      {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100, Priority: PriorityHigh},
				CategoryContext:   {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100, Priority: PriorityHigh},
				CategoryNorthstar: {BasePercent: 0.333333, MinTokens: 0, MaxTokens: 100, Priority: PriorityHigh},
			},
		}

		totalBudget := 100
		presentCategories := map[AtomCategory]bool{
			CategoryInit:      true,
			CategoryContext:   true,
			CategoryNorthstar: true,
		}

		allocs := m.calculateAllocations(totalBudget, presentCategories)

		totalAllocated := 0
		for _, v := range allocs {
			totalAllocated += v
		}

		assert.Equal(t, 100, totalAllocated)
	})

	t.Run("balanced exact distribution", func(t *testing.T) {
		m := &TokenBudgetManager{
			strategy: StrategyBalanced,
			budgets: map[AtomCategory]CategoryBudget{
				CategoryInit:      {BasePercent: 0.333333, MinTokens: 10, MaxTokens: 100},
				CategoryContext:   {BasePercent: 0.333333, MinTokens: 10, MaxTokens: 100},
				CategoryNorthstar: {BasePercent: 0.333333, MinTokens: 10, MaxTokens: 100},
			},
		}

		totalBudget := 100
		presentCategories := map[AtomCategory]bool{
			CategoryInit:      true,
			CategoryContext:   true,
			CategoryNorthstar: true,
		}

		allocs := m.calculateAllocations(totalBudget, presentCategories)

		totalAllocated := 0
		for _, v := range allocs {
			totalAllocated += v
		}

		assert.Equal(t, 100, totalAllocated)
	})
}
