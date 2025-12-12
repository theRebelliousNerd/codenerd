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

	// Knowledge and build-layer/intent/world-state are medium/conditional priorities.
	// These categories carry encyclopedic but selector-gated atoms.
	m.budgets[CategoryKnowledge] = CategoryBudget{
		Category:     CategoryKnowledge,
		BasePercent:  0.05,
		MinTokens:    300,
		MaxTokens:    8000,
		Priority:     PriorityMedium,
		CanExceedMax: true,
	}

	m.budgets[CategoryBuildLayer] = CategoryBudget{
		Category:    CategoryBuildLayer,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryIntent] = CategoryBudget{
		Category:    CategoryIntent,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryWorldState] = CategoryBudget{
		Category:    CategoryWorldState,
		BasePercent: 0.03,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	// Reviewer-specific atoms are low priority; include if budget remains.
	m.budgets[CategoryReviewer] = CategoryBudget{
		Category:     CategoryReviewer,
		BasePercent:  0.02,
		MinTokens:    0,
		MaxTokens:    2000,
		Priority:     PriorityLow,
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
// Implements the core budget allocation algorithm with polymorphism (Standard -> Concise -> Min).
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

	// Helper to get token count for a mode
	getTokenCount := func(atom *PromptAtom, mode string) int {
		switch mode {
		case "concise":
			// If pre-calculated, use it. Otherwise estimate.
			// We didn't store token counts for variants, so we estimate on the fly.
			if atom.ContentConcise != "" {
				return EstimateTokens(atom.ContentConcise)
			}
			return atom.TokenCount // Fallback
		case "min":
			if atom.ContentMin != "" {
				return EstimateTokens(atom.ContentMin)
			}
			return atom.TokenCount // Fallback
		default:
			return atom.TokenCount
		}
	}

	// Process categories in priority order
	categories := m.categoriesByPriority()
	for _, cat := range categories {
		atomsInCat, exists := byCategory[cat]
		if !exists {
			continue
		}

		allocation, hasAlloc := allocations[cat]
		if !hasAlloc {
			// If generic allocation (0) but CanExceedMax is true?
			// Usually allocation=0 means no budget.
			// But for Low Priority, allocation might be 0 but fillRemainingBudget handles it.
			// We follow the strict allocation first.
			allocation = 0
		}

		// Checking CanExceed here or later?
		// Existing logic tried to fit strictly.
		// We'll stick to strict fit then overflow.

		catTokens := 0
		for _, oa := range atomsInCat {
			// Determine best mode
			mode := "standard"
			tokens := getTokenCount(oa.Atom, mode)

			// Mandatory atoms always included (Standard unless forced?)
			// Let's keep mandatory as Standard for safety.
			if oa.Atom.IsMandatory {
				oa.RenderMode = mode
				result = append(result, oa)
				catTokens += tokens
				usedTokens += tokens
				continue
			}

			// Try Standard
			if catTokens+tokens <= allocation {
				oa.RenderMode = mode
				result = append(result, oa)
				catTokens += tokens
				usedTokens += tokens
				continue
			}

			// Try Concise
			if oa.Atom.ContentConcise != "" {
				mode = "concise"
				tokens = getTokenCount(oa.Atom, mode)
				if catTokens+tokens <= allocation {
					oa.RenderMode = mode
					result = append(result, oa)
					catTokens += tokens
					usedTokens += tokens
					continue
				}
			}

			// Try Min
			if oa.Atom.ContentMin != "" {
				mode = "min"
				tokens = getTokenCount(oa.Atom, mode)
				if catTokens+tokens <= allocation {
					oa.RenderMode = mode
					result = append(result, oa)
					catTokens += tokens
					usedTokens += tokens
					continue
				}
			}

			// If we get here, atom doesn't fit even in Min mode.
			// Log exclusion?
			logging.Get(logging.CategoryContext).Debug(
				"Atom %s excluded from cat %s (budget full)", oa.Atom.ID, cat,
			)
		}

		logging.Get(logging.CategoryContext).Debug(
			"Category %s: allocated %d tokens, used %d tokens",
			cat, allocation, catTokens,
		)
	}

	// Second pass: fill remaining budget with best remaining atoms
	remaining := availableBudget - usedTokens
	if remaining > 0 {
		// Polymorphic fill?
		// The fillRemainingBudget function needs update too.
		// For now, let's just call it, assuming it uses Standard.
		// Or update it to use the new logic?
		// Let's implement inline fill logic here to support polymorphism.

		// Collect unselected
		selectedSet := make(map[string]bool)
		for _, oa := range result {
			selectedSet[oa.Atom.ID] = true
		}

		var unselected []*OrderedAtom
		for _, catList := range byCategory {
			for _, oa := range catList {
				if !selectedSet[oa.Atom.ID] {
					unselected = append(unselected, oa)
				}
			}
		}

		sort.Slice(unselected, func(i, j int) bool {
			return unselected[i].Score > unselected[j].Score
		})

		for _, oa := range unselected {
			// Try Standard
			tokens := getTokenCount(oa.Atom, "standard")
			if tokens <= remaining {
				oa.RenderMode = "standard"
				result = append(result, oa)
				remaining -= tokens
				continue
			}

			// Try Concise
			if oa.Atom.ContentConcise != "" {
				tokens = getTokenCount(oa.Atom, "concise")
				if tokens <= remaining {
					oa.RenderMode = "concise"
					result = append(result, oa)
					remaining -= tokens
					continue
				}
			}

			// Try Min
			if oa.Atom.ContentMin != "" {
				tokens = getTokenCount(oa.Atom, "min")
				if tokens <= remaining {
					oa.RenderMode = "min"
					result = append(result, oa)
					remaining -= tokens
					continue
				}
			}
		}
	}

	logging.Get(logging.CategoryContext).Debug(
		"Fitted %d atoms within budget of %d tokens (used %d)",
		len(result), totalBudget, availableBudget-remaining,
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
