package prompt

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"codenerd/internal/logging"
)

// maxAtomsInput caps the total number of atoms accepted by Fit().
// Inputs larger than this are truncated to prevent excessive allocation
// and sort cost. Chosen to comfortably exceed any realistic corpus size.
const maxAtomsInput = 100000

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
	mu      sync.RWMutex
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

	m.budgets[CategoryCapability] = CategoryBudget{
		Category:    CategoryCapability,
		BasePercent: 0.06,
		MinTokens:   300,
		MaxTokens:   6000,
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

	m.budgets[CategoryAutopoiesis] = CategoryBudget{
		Category:    CategoryAutopoiesis,
		BasePercent: 0.02,
		MinTokens:   0,
		MaxTokens:   3000,
		Priority:    PriorityConditional,
	}

	m.budgets[CategoryEval] = CategoryBudget{
		Category:    CategoryEval,
		BasePercent: 0.02,
		MinTokens:   0,
		MaxTokens:   2000,
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgets[budget.Category] = budget
}

// SetStrategy sets the allocation strategy.
func (m *TokenBudgetManager) SetStrategy(strategy AllocationStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strategy = strategy
}

// SetReservedHeadroom sets the buffer tokens to keep as reserve.
// Negative values are clamped to 0 (a negative buffer is nonsensical
// and would silently inflate the available budget).
func (m *TokenBudgetManager) SetReservedHeadroom(tokens int) {
	if tokens < 0 {
		logging.Get(logging.CategoryContext).Warn(
			"SetReservedHeadroom: negative value %d clamped to 0", tokens,
		)
		tokens = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservedHeadroom = tokens
}

// Fit selects atoms that fit within the total budget.
// Implements the core budget allocation algorithm with polymorphism (Standard -> Concise -> Min).
func (m *TokenBudgetManager) Fit(atoms []*OrderedAtom, totalBudget int) ([]*OrderedAtom, error) {

	timer := logging.StartTimer(logging.CategoryContext, "TokenBudgetManager.Fit")
	defer timer.Stop()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(atoms) == 0 {
		return nil, nil
	}

	// Bounds check input size up front to avoid pathological allocation /
	// sort cost on adversarially large inputs. We truncate rather than error
	// so callers degrade gracefully.
	if len(atoms) > maxAtomsInput {
		logging.Get(logging.CategoryContext).Warn(
			"TokenBudgetManager.Fit: input atom count %d exceeds cap %d; truncating",
			len(atoms), maxAtomsInput,
		)
		atoms = atoms[:maxAtomsInput]
	}

	availableBudget := totalBudget - m.reservedHeadroom
	if availableBudget <= 0 {
		return nil, fmt.Errorf("total budget %d is less than reserved headroom %d",
			totalBudget, m.reservedHeadroom)
	}

	// We'll work with a copy of the slice to avoid modifying the input order permanently
	// (though modifying it might be safe, a copy is safer and cleaner).
	sortedAtoms := make([]*OrderedAtom, 0, len(atoms))
	for _, atom := range atoms {
		if atom == nil || atom.Atom == nil {
			continue
		}
		// Deep copy to prevent side-effects on the caller's shared PromptAtom objects
		clonedAtom := *atom.Atom
		if clonedAtom.TokenCount < 0 {
			clonedAtom.TokenCount = 0
		}
		clonedOrdered := *atom
		clonedOrdered.Atom = &clonedAtom
		sortedAtoms = append(sortedAtoms, &clonedOrdered)
	}

	// Sort atoms: Priority -> Category -> Score
	// Use a helper closure to get priority safely
	getPriority := func(cat AtomCategory) int {
		if b, ok := m.budgets[cat]; ok {
			return int(b.Priority)
		}
		// Categories without budget config come last
		return int(PriorityConditional) + 1
	}

	sort.SliceStable(sortedAtoms, func(i, j int) bool {
		catI := sortedAtoms[i].Atom.Category
		catJ := sortedAtoms[j].Atom.Category

		// If same category, sort by Score descending
		if catI == catJ {
			return sortedAtoms[i].Score > sortedAtoms[j].Score
		}

		prioI := getPriority(catI)
		prioJ := getPriority(catJ)

		if prioI != prioJ {
			return prioI < prioJ
		}

		// Deterministic tie-break by category string
		return catI < catJ
	})

	// Scan for present categories (lightweight set)
	presentCategories := make(map[AtomCategory]bool)
	for _, oa := range sortedAtoms {
		presentCategories[oa.Atom.Category] = true
	}

	// Calculate category allocations
	allocations := m.calculateAllocations(availableBudget, presentCategories)

	// Select atoms.
	// Pre-allocate slices using min(len(atoms), maxAtomsLimit) to avoid
	// allocating very large backing arrays when callers pass enormous inputs.
	const maxAtomsLimit = 5000
	preAlloc := len(atoms)
	if preAlloc > maxAtomsLimit {
		preAlloc = maxAtomsLimit
	}
	result := make([]*OrderedAtom, 0, preAlloc)
	unselected := make([]*OrderedAtom, 0, preAlloc)
	var usedTokens int64 = 0
	var atomsIncluded int = 0

	// Helper to get token count for a mode
	getTokenCount := func(atom *PromptAtom, mode string) int {
		var cnt int
		switch mode {
		case "concise":
			if atom.ContentConcise != "" {
				cnt = EstimateTokens(atom.ContentConcise)
			} else {
				cnt = atom.TokenCount
			}
		case "min":
			if atom.ContentMin != "" {
				cnt = EstimateTokens(atom.ContentMin)
			} else {
				cnt = atom.TokenCount
			}
		default:
			cnt = atom.TokenCount
		}
		if cnt < 0 {
			return 0
		}
		return cnt
	}

	// Iterate through sorted atoms in contiguous category chunks
	for i := 0; i < len(sortedAtoms); {
		cat := sortedAtoms[i].Atom.Category

		// Find range for this category
		start := i
		end := i + 1
		for end < len(sortedAtoms) && sortedAtoms[end].Atom.Category == cat {
			end++
		}

		// Move iterator
		i = end

		// Process chunk [start, end)
		allocation, hasAlloc := allocations[cat]
		if !hasAlloc {
			// If no allocation, strictly 0 unless fillRemaining handles it later.
			// Or Mandatory atoms?
			// Existing behavior: Mandatory atoms in unbudgeted categories were skipped in Pass 1.
			// We replicate this by setting allocation 0 and letting logic flow.
			allocation = 0
		}

		var catTokens int64 = 0
		for k := start; k < end; k++ {
			if atomsIncluded >= maxAtomsLimit {
				unselected = append(unselected, sortedAtoms[k])
				continue
			}

			oa := sortedAtoms[k]
			mode := "standard"
			tokens := int64(getTokenCount(oa.Atom, mode))

			if oa.Atom.IsMandatory {
				// Enforce absolute totalBudget cap even for mandatory atoms.
				// Without this guard, a single oversized mandatory atom can
				// blow the context window (e.g. a 2M-token atom against an
				// 8K budget). Log a warning and skip if it would overflow.
				if tokens > int64(totalBudget) ||
					usedTokens > math.MaxInt64-tokens ||
					usedTokens+tokens > int64(totalBudget) {
					logging.Get(logging.CategoryContext).Warn(
						"Mandatory atom %s (%d tokens) exceeds total budget %d (used=%d); skipping",
						oa.Atom.ID, tokens, totalBudget, usedTokens,
					)
					unselected = append(unselected, oa)
					continue
				}
				oa.RenderMode = mode
				result = append(result, oa)
				catTokens += tokens
				usedTokens += tokens
				atomsIncluded++
				continue
			}

			if !hasAlloc {
				unselected = append(unselected, oa)
				continue
			}

			// Try Standard. Guard against int64 overflow on catTokens/usedTokens
			// before performing the inclusion check.
			if tokens >= 0 &&
				catTokens <= math.MaxInt64-tokens &&
				usedTokens <= math.MaxInt64-tokens &&
				catTokens+tokens <= int64(allocation) {
				oa.RenderMode = mode
				result = append(result, oa)
				catTokens += tokens
				usedTokens += tokens
				atomsIncluded++
				continue
			}

			// Try Concise
			if oa.Atom.ContentConcise != "" {
				mode = "concise"
				tokens = int64(getTokenCount(oa.Atom, mode))
				if tokens >= 0 &&
					catTokens <= math.MaxInt64-tokens &&
					usedTokens <= math.MaxInt64-tokens &&
					catTokens+tokens <= int64(allocation) {
					oa.RenderMode = mode
					result = append(result, oa)
					catTokens += tokens
					usedTokens += tokens
					atomsIncluded++
					continue
				}
			}

			// Try Min
			if oa.Atom.ContentMin != "" {
				mode = "min"
				tokens = int64(getTokenCount(oa.Atom, mode))
				if tokens >= 0 &&
					catTokens <= math.MaxInt64-tokens &&
					usedTokens <= math.MaxInt64-tokens &&
					catTokens+tokens <= int64(allocation) {
					oa.RenderMode = mode
					result = append(result, oa)
					catTokens += tokens
					usedTokens += tokens
					atomsIncluded++
					continue
				}
			}

			// Rejected
			unselected = append(unselected, oa)
		}

		if hasAlloc {
			logging.Get(logging.CategoryContext).Debug(
				"Category %s: allocated %d tokens, used %d tokens",
				cat, allocation, catTokens,
			)
		}
	}

	// Second pass: fill remaining budget with best remaining atoms
	var remaining int64 = int64(availableBudget) - usedTokens
	if remaining > 0 && len(unselected) > 0 {
		// Sort unselected by Score descending
		sort.SliceStable(unselected, func(i, j int) bool {
			return unselected[i].Score > unselected[j].Score
		})

		for _, oa := range unselected {
			if atomsIncluded >= maxAtomsLimit {
				break
			}

			// Try Standard
			tokens := int64(getTokenCount(oa.Atom, "standard"))
			if tokens >= 0 && tokens <= remaining &&
				usedTokens <= math.MaxInt64-tokens {
				oa.RenderMode = "standard"
				result = append(result, oa)
				remaining -= tokens
				atomsIncluded++
				usedTokens += tokens
				continue
			}

			// Try Concise
			if oa.Atom.ContentConcise != "" {
				tokens = int64(getTokenCount(oa.Atom, "concise"))
				if tokens >= 0 && tokens <= remaining &&
					usedTokens <= math.MaxInt64-tokens {
					oa.RenderMode = "concise"
					result = append(result, oa)
					remaining -= tokens
					atomsIncluded++
					usedTokens += tokens
					continue
				}
			}

			// Try Min
			if oa.Atom.ContentMin != "" {
				tokens = int64(getTokenCount(oa.Atom, "min"))
				if tokens >= 0 && tokens <= remaining &&
					usedTokens <= math.MaxInt64-tokens {
					oa.RenderMode = "min"
					result = append(result, oa)
					remaining -= tokens
					atomsIncluded++
					usedTokens += tokens
					continue
				}
			}
		}
	}

	logging.Get(logging.CategoryContext).Debug(
		"Fitted %d atoms within budget of %d tokens (used %d)",
		len(result), totalBudget, usedTokens,
	)

	return result, nil
}

// calculateAllocations determines token allocation per category.
// TODO: Reliability: Handle precision loss/rounding errors in `BasePercent` allocations to ensure 100% of the budget is distributed effectively when `StrategyProportional` or `StrategyPriorityFirst` is used.
func (m *TokenBudgetManager) calculateAllocations(
	totalBudget int,
	presentCategories map[AtomCategory]bool,
) map[AtomCategory]int {
	allocations := make(map[AtomCategory]int)

	switch m.strategy {
	case StrategyProportional:
		for cat, budget := range m.budgets {
			if !presentCategories[cat] {
				continue
			}
			allocation := int(float64(totalBudget) * budget.BasePercent)
			allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
			allocations[cat] = allocation
		}

	case StrategyPriorityFirst:
		remaining := totalBudget

		// Helper to allocate for a priority level
		allocateForPriority := func(p BudgetPriority) {
			for cat, budget := range m.budgets {
				if budget.Priority != p {
					continue
				}
				if !presentCategories[cat] {
					continue
				}

				var allocation int
				// Use totalBudget for Mandatory to ensure they get their share?
				// Existing logic used totalBudget for Mandatory.
				if p == PriorityMandatory {
					allocation = int(float64(totalBudget) * budget.BasePercent)
				} else {
					if remaining <= 0 {
						allocation = 0
					} else {
						allocation = int(float64(remaining) * budget.BasePercent)
					}
				}

				allocation = clamp(allocation, budget.MinTokens, budget.MaxTokens)
				allocations[cat] = allocation
				remaining -= allocation
			}
		}

		allocateForPriority(PriorityMandatory)
		allocateForPriority(PriorityHigh)
		allocateForPriority(PriorityMedium)

		// Low and conditional get what's left
		for cat, budget := range m.budgets {
			if budget.Priority != PriorityLow && budget.Priority != PriorityConditional {
				continue
			}
			if !presentCategories[cat] {
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
			if !presentCategories[cat] {
				continue
			}
			allocations[cat] = budget.MinTokens
			remaining -= budget.MinTokens
		}

		// Distribute remaining proportionally
		for cat, budget := range m.budgets {
			if !presentCategories[cat] {
				continue
			}
			extra := int(float64(remaining) * budget.BasePercent)
			allocations[cat] = clamp(allocations[cat]+extra, budget.MinTokens, budget.MaxTokens)
		}
	}

	return allocations
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
		if oa == nil || oa.Atom == nil {
			continue
		}
		cat := oa.Atom.Category
		usage := report.CategoryUsage[cat]
		tokenCount := max(oa.Atom.TokenCount, 0)
		usage.Used += tokenCount
		usage.AtomCount++
		if budget, ok := m.budgets[cat]; ok {
			usage.Priority = budget.Priority
		}
		report.CategoryUsage[cat] = usage
		report.UsedTokens += tokenCount

		if oa.Atom.IsMandatory {
			report.MandatoryTokens += tokenCount
		} else {
			report.OptionalTokens += tokenCount
		}
	}

	report.RemainingTokens = totalBudget - report.UsedTokens
	if report.RemainingTokens < 0 {
		report.OverBudgetAmount = -report.RemainingTokens
		report.RemainingTokens = 0
	}

	return report
}
