package prompt

import (
	"fmt"
	"sort"

	"codenerd/internal/logging"
)

// BudgetPriority defines priority levels for category budget allocation.
type BudgetPriority int

const (
	// PriorityMandatory items are always included regardless of budget.
	PriorityMandatory BudgetPriority = iota

	// PriorityHigh items are included first after mandatory.
	PriorityHigh

	// PriorityMedium items are included if budget allows.
	PriorityMedium

	// PriorityLow items are included last if space remains.
	PriorityLow

	// PriorityConditional items are only included if specific conditions are met.
	PriorityConditional
)

// String returns the string representation of BudgetPriority.
func (p BudgetPriority) String() string {
	switch p {
	case PriorityMandatory:
		return "mandatory"
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	case PriorityConditional:
		return "conditional"
	default:
		return "unknown"
	}
}

// CategoryBudget defines allocation parameters for a category.
type CategoryBudget struct {
	// Category this budget applies to
	Category AtomCategory

	// BasePercent is the target percentage of total budget (0.0-1.0)
	BasePercent float64

	// MinTokens is the minimum tokens to allocate (absolute floor)
	MinTokens int

	// MaxTokens is the maximum tokens to allocate (absolute ceiling)
	MaxTokens int

	// Priority determines allocation order
	Priority BudgetPriority

	// CanExceedMax allows this category to exceed MaxTokens if budget remains
	CanExceedMax bool
}

// TokenBudgetManager allocates tokens across categories.
// It implements a priority-based allocation strategy:
// 1. Mandatory atoms are always included
// 2. Categories are allocated in priority order
// 3. High-scored atoms within categories are preferred
// 4. Remaining budget is distributed to lower priorities
type TokenBudgetManager struct {
	budgets map[AtomCategory]CategoryBudget

	// Allocation strategy
	strategy AllocationStrategy

	// Reserved headroom (tokens kept as buffer)
	reservedHeadroom int
}

// AllocationStrategy defines how tokens are distributed.
type AllocationStrategy int

const (
	// StrategyProportional distributes proportionally by BasePercent.
	StrategyProportional AllocationStrategy = iota

	// StrategyPriorityFirst fills higher priorities before moving to lower.
	StrategyPriorityFirst

	// StrategyBalanced attempts equal distribution then adds extras.
	StrategyBalanced
)

// NewTokenBudgetManager creates a new budget manager with default allocations.
func NewTokenBudgetManager() *TokenBudgetManager {
	mgr := &TokenBudgetManager{
		budgets:          make(map[AtomCategory]CategoryBudget),
		strategy:         StrategyPriorityFirst,
		reservedHeadroom: 500, // Keep 500 tokens as buffer
	}

	// Set default budgets for each category
	mgr.setDefaultBudgets()

	return mgr
}

// setDefaultBudgets configures default allocations per category.
func (m *TokenBudgetManager) setDefaultBudgets() {
	// Safety and identity are mandatory
	m.budgets[CategorySafety] = CategoryBudget{
		Category:    CategorySafety,
		BasePercent: 0.05,
		MinTokens:   500,
		MaxTokens:   5000,
		Priority:    PriorityMandatory,
	}

	m.budgets[CategoryIdentity] = CategoryBudget{
		Category:    CategoryIdentity,
		BasePercent: 0.08,
		MinTokens:   1000,
		MaxTokens:   8000,
		Priority:    PriorityMandatory,
	}

	// Protocol and methodology are high priority
	m.budgets[CategoryProtocol] = CategoryBudget{
		Category:    CategoryProtocol,
		BasePercent: 0.10,
		MinTokens:   500,
		MaxTokens:   10000,
		Priority:    PriorityHigh,
	}

	m.budgets[CategoryMethodology] = CategoryBudget{
		Category:    CategoryMethodology,
		BasePercent: 0.08,
		MinTokens:   500,
		MaxTokens:   8000,
		Priority:    PriorityHigh,
	}

	m.budgets[CategoryHallucination] = CategoryBudget{
		Category:    CategoryHallucination,
		BasePercent: 0.05,
		MinTokens:   500,
		MaxTokens:   5000,
		Priority:    PriorityHigh,
	}

	// Language and framework are medium priority (context-dependent)
	m.budgets[CategoryLanguage] = CategoryBudget{
		Category:     CategoryLanguage,
		BasePercent:  0.15,
		MinTokens:    1000,
		MaxTokens:    15000,
		Priority:     PriorityMedium,
		CanExceedMax: true,
	}

	m.budgets[CategoryFramework] = CategoryBudget{
		Category:     CategoryFramework,
		BasePercent:  0.10,
		MinTokens:    500,
		MaxTokens:    10000,
		Priority:     PriorityMedium,
		CanExceedMax: true,
	}

	// Domain and context are medium priority
	m.budgets[CategoryDomain] = CategoryBudget{
		Category:    CategoryDomain,
		BasePercent: 0.10,
		MinTokens:   500,
		MaxTokens:   10000,
		Priority:    PriorityMedium,
	}

	m.budgets[CategoryContext] = CategoryBudget{
		Category:     CategoryContext,
		BasePercent:  0.15,
		MinTokens:    500,
		MaxTokens:    15000,
		Priority:     PriorityMedium,
		CanExceedMax: true,
	}

	// Campaign and specialized phases are conditional
	m.budgets[CategoryCampaign] = CategoryBudget{
		Category:    CategoryCampaign,
		BasePercent: 0.05,
		MinTokens:   0,
		MaxTokens:   5000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryInit] = CategoryBudget{
		Category:    CategoryInit,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryNorthstar] = CategoryBudget{
		Category:    CategoryNorthstar,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryOuroboros] = CategoryBudget{
		Category:    CategoryOuroboros,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	// Exemplars are low priority (only if space)
	m.budgets[CategoryExemplar] = CategoryBudget{
		Category:     CategoryExemplar,
		BasePercent:  0.05,
		MinTokens:    0,
		MaxTokens:    5000,
		Priority:     PriorityLow,
		CanExceedMax: true,
	}
}

// SetCategoryBudget configures the budget for a specific category.
func (m *TokenBudgetManager) SetCategoryBudget(budget CategoryBudget) {
	m.budgets[budget.Category] = budget
}

// SetStrategy sets the allocation strategy.
func (m *TokenBudgetManager) SetStrategy(strategy AllocationStrategy) {
	m.strategy = strategy
}

// SetReservedHeadroom sets the buffer tokens to keep as reserve.
func (m *TokenBudgetManager) SetReservedHeadroom(tokens int) {
	m.reservedHeadroom = tokens
}

// Fit selects atoms that fit within the total budget.
// Implements the core budget allocation algorithm.
func (m *TokenBudgetManager) Fit(atoms []*OrderedAtom, totalBudget int) ([]*OrderedAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "TokenBudgetManager.Fit")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, nil
	}

	availableBudget := totalBudget - m.reservedHeadroom
	if availableBudget <= 0 {
		return nil, fmt.Errorf("total budget %d is less than reserved headroom %d",
			totalBudget, m.reservedHeadroom)
	}

	// Group atoms by category
	byCategory := make(map[AtomCategory][]*OrderedAtom)
	for _, oa := range atoms {
		cat := oa.Atom.Category
		byCategory[cat] = append(byCategory[cat], oa)
	}

	// Sort atoms within each category by score (descending)
	for cat := range byCategory {
		sort.Slice(byCategory[cat], func(i, j int) bool {
			return byCategory[cat][i].Score > byCategory[cat][j].Score
		})
	}

	// Calculate category allocations
	allocations := m.calculateAllocations(availableBudget, byCategory)

	// Select atoms within allocations
	var result []*OrderedAtom
	usedTokens := 0

	// Process categories in priority order
	categories := m.categoriesByPriority()
	for _, cat := range categories {
		atomsInCat, exists := byCategory[cat]
		if !exists {
			continue
		}

		allocation, hasAlloc := allocations[cat]
		if !hasAlloc {
			allocation = 0
		}

		catTokens := 0
		for _, oa := range atomsInCat {
			atomTokens := oa.Atom.TokenCount

			// Mandatory atoms always included
			if oa.Atom.IsMandatory {
				result = append(result, oa)
				catTokens += atomTokens
				usedTokens += atomTokens
				continue
			}

			// Check if atom fits in category allocation
			if catTokens+atomTokens <= allocation {
				result = append(result, oa)
				catTokens += atomTokens
				usedTokens += atomTokens
			}
		}

		logging.Get(logging.CategoryContext).Debug(
			"Category %s: allocated %d tokens, used %d tokens",
			cat, allocation, catTokens,
		)
	}

	// Second pass: fill remaining budget with best remaining atoms
	remaining := availableBudget - usedTokens
	if remaining > 0 {
		result = m.fillRemainingBudget(result, byCategory, remaining)
	}

	logging.Get(logging.CategoryContext).Debug(
		"Fitted %d atoms within budget of %d tokens (used %d)",
		len(result), totalBudget, usedTokens,
	)

	return result, nil
}

// calculateAllocations determines token allocation per category.
func (m *TokenBudgetManager) calculateAllocations(
	totalBudget int,
	byCategory map[AtomCategory][]*OrderedAtom,
) map[AtomCategory]int {
	allocations := make(map[AtomCategory]int)

	switch m.strategy {
	case StrategyProportional:
		for cat, budget := range m.budgets {
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			allocation := int(float64(totalBudget) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
		}

	case StrategyPriorityFirst:
		remaining := totalBudget

		// Allocate mandatory first
		for cat, budget := range m.budgets {
			if budget.Priority != PriorityMandatory {
				continue
			}
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			allocation := int(float64(totalBudget) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
			remaining -= allocation
		}

		// Then high priority
		for cat, budget := range m.budgets {
			if budget.Priority != PriorityHigh {
				continue
			}
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			allocation := int(float64(remaining) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
			remaining -= allocation
		}

		// Then medium priority
		for cat, budget := range m.budgets {
			if budget.Priority != PriorityMedium {
				continue
			}
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			allocation := int(float64(remaining) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
			remaining -= allocation
		}

		// Low and conditional get what's left
		for cat, budget := range m.budgets {
			if budget.Priority != PriorityLow && budget.Priority != PriorityConditional {
				continue
			}
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			if remaining <= 0 {
				allocations[cat] = 0
				continue
			}
			allocation := int(float64(remaining) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
			remaining -= allocation
		}

	case StrategyBalanced:
		// Start with minimum allocations
		remaining := totalBudget
		for cat, budget := range m.budgets {
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			allocations[cat] = budget.MinTokens
			remaining -= budget.MinTokens
		}

		// Distribute remaining proportionally
		for cat, budget := range m.budgets {
			if _, exists := byCategory[cat]; !exists {
				continue
			}
			extra := int(float64(remaining) * budget.BasePercent)
			allocations[cat] = clamp(allocations[cat]+extra, budget.MinTokens, budget.MaxTokens)
		}
	}

	return allocations
}

// categoriesByPriority returns categories sorted by budget priority.
func (m *TokenBudgetManager) categoriesByPriority() []AtomCategory {
	type catPriority struct {
		cat      AtomCategory
		priority BudgetPriority
	}

	var cats []catPriority
	for cat, budget := range m.budgets {
		cats = append(cats, catPriority{cat, budget.Priority})
	}

	sort.Slice(cats, func(i, j int) bool {
		return cats[i].priority < cats[j].priority
	})

	result := make([]AtomCategory, len(cats))
	for i, cp := range cats {
		result[i] = cp.cat
	}

	return result
}

// fillRemainingBudget adds more atoms if budget remains.
func (m *TokenBudgetManager) fillRemainingBudget(
	selected []*OrderedAtom,
	byCategory map[AtomCategory][]*OrderedAtom,
	remaining int,
) []*OrderedAtom {
	// Build set of already selected atoms
	selectedSet := make(map[string]bool, len(selected))
	for _, oa := range selected {
		selectedSet[oa.Atom.ID] = true
	}

	// Collect all unselected atoms, sorted by score
	var unselected []*OrderedAtom
	for _, atoms := range byCategory {
		for _, oa := range atoms {
			if !selectedSet[oa.Atom.ID] {
				unselected = append(unselected, oa)
			}
		}
	}

	sort.Slice(unselected, func(i, j int) bool {
		return unselected[i].Score > unselected[j].Score
	})

	// Add atoms until budget is exhausted
	for _, oa := range unselected {
		if oa.Atom.TokenCount <= remaining {
			selected = append(selected, oa)
			remaining -= oa.Atom.TokenCount
		}
	}

	return selected
}

// clamp restricts a value to a range.
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// BudgetReport summarizes budget allocation and usage.
type BudgetReport struct {
	TotalBudget      int
	UsedTokens       int
	RemainingTokens  int
	CategoryUsage    map[AtomCategory]CategoryUsage
	MandatoryTokens  int
	OptionalTokens   int
	OverBudgetAmount int
}

// CategoryUsage tracks usage for a single category.
type CategoryUsage struct {
	Allocated int
	Used      int
	AtomCount int
	Priority  BudgetPriority
}

// GenerateReport creates a budget report for a set of fitted atoms.
func (m *TokenBudgetManager) GenerateReport(atoms []*OrderedAtom, totalBudget int) BudgetReport {
	report := BudgetReport{
		TotalBudget:   totalBudget,
		CategoryUsage: make(map[AtomCategory]CategoryUsage),
	}

	for _, oa := range atoms {
		cat := oa.Atom.Category
		usage := report.CategoryUsage[cat]
		usage.Used += oa.Atom.TokenCount
		usage.AtomCount++
		if budget, ok := m.budgets[cat]; ok {
			usage.Priority = budget.Priority
		}
		report.CategoryUsage[cat] = usage
		report.UsedTokens += oa.Atom.TokenCount

		if oa.Atom.IsMandatory {
			report.MandatoryTokens += oa.Atom.TokenCount
		} else {
			report.OptionalTokens += oa.Atom.TokenCount
		}
	}

	report.RemainingTokens = totalBudget - report.UsedTokens
	if report.RemainingTokens < 0 {
		report.OverBudgetAmount = -report.RemainingTokens
		report.RemainingTokens = 0
	}

	return report
}
