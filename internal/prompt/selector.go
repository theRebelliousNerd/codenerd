package prompt

import (
	"context"
	"sort"

	"codenerd/internal/logging"
)

// ScoredAtom is an atom with its selection score.
// The score determines priority when fitting within budget.
type ScoredAtom struct {
	Atom *PromptAtom

	// LogicScore from Mangle rule evaluation (0.0-1.0)
	LogicScore float64

	// VectorScore from semantic similarity (0.0-1.0)
	VectorScore float64

	// Combined weighted score
	Combined float64

	// Selection reason for debugging
	SelectionReason string
}

// AtomSelector selects atoms based on context using Mangle rules + vector search.
// It implements a hybrid selection strategy:
// 1. Rule-based filtering using Mangle predicates
// 2. Semantic scoring using vector embeddings
// 3. Context matching using selector dimensions
type AtomSelector struct {
	kernel         KernelQuerier
	vectorSearcher VectorSearcher

	// Weight for vector score in combined calculation (0.0-1.0)
	vectorWeight float64

	// Minimum score threshold for inclusion
	minScoreThreshold float64
}

// NewAtomSelector creates a new atom selector with default settings.
func NewAtomSelector() *AtomSelector {
	return &AtomSelector{
		vectorWeight:      0.3,  // 70% logic, 30% vector
		minScoreThreshold: 0.1,  // Minimum 10% match
	}
}

// SetKernel sets the Mangle kernel for rule-based selection.
func (s *AtomSelector) SetKernel(kernel KernelQuerier) {
	s.kernel = kernel
}

// SetVectorSearcher sets the vector searcher for semantic selection.
func (s *AtomSelector) SetVectorSearcher(vs VectorSearcher) {
	s.vectorSearcher = vs
}

// SetVectorWeight sets the weight of vector scores in combined calculation.
func (s *AtomSelector) SetVectorWeight(weight float64) {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	s.vectorWeight = weight
}

// SetMinScoreThreshold sets the minimum score for atom inclusion.
func (s *AtomSelector) SetMinScoreThreshold(threshold float64) {
	s.minScoreThreshold = threshold
}

// SelectAtoms returns scored atoms matching the context.
// The selection process:
// 1. Filter atoms by context matching (selector dimensions)
// 2. Score by logic rules (Mangle queries)
// 3. Score by semantic similarity (vector search)
// 4. Combine scores with configurable weighting
// 5. Filter by minimum threshold
func (s *AtomSelector) SelectAtoms(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
) ([]*ScoredAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "AtomSelector.SelectAtoms")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, nil
	}

	// Step 1: Context filtering
	contextMatched := s.filterByContext(atoms, cc)
	logging.Get(logging.CategoryContext).Debug(
		"Context filter: %d/%d atoms matched", len(contextMatched), len(atoms),
	)

	// Step 2: Get vector scores (if enabled and query provided)
	vectorScores := make(map[string]float64)
	if s.vectorSearcher != nil && cc.SemanticQuery != "" {
		var err error
		vectorScores, err = s.getVectorScores(ctx, cc.SemanticQuery, cc.SemanticTopK)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Vector search failed: %v", err)
			// Continue without vector scores
		}
	}

	// Step 3: Score each atom
	scored := make([]*ScoredAtom, 0, len(contextMatched))
	for _, atom := range contextMatched {
		sa := s.scoreAtom(atom, cc, vectorScores)

		// Skip atoms below threshold (unless mandatory)
		if sa.Combined < s.minScoreThreshold && !atom.IsMandatory {
			LogAtomSelection(atom.ID, false, "below threshold")
			continue
		}

		scored = append(scored, sa)
		LogAtomSelection(atom.ID, true, sa.SelectionReason)
	}

	// Step 4: Sort by combined score (descending)
	sort.Slice(scored, func(i, j int) bool {
		// Mandatory atoms always come first
		if scored[i].Atom.IsMandatory != scored[j].Atom.IsMandatory {
			return scored[i].Atom.IsMandatory
		}
		// Then by combined score
		return scored[i].Combined > scored[j].Combined
	})

	logging.Get(logging.CategoryContext).Debug(
		"Selected %d atoms after scoring", len(scored),
	)

	return scored, nil
}

// filterByContext filters atoms by context selector matching.
func (s *AtomSelector) filterByContext(atoms []*PromptAtom, cc *CompilationContext) []*PromptAtom {
	matched := make([]*PromptAtom, 0, len(atoms))

	for _, atom := range atoms {
		if atom.MatchesContext(cc) {
			matched = append(matched, atom)
		}
	}

	return matched
}

// getVectorScores retrieves semantic similarity scores for atoms.
func (s *AtomSelector) getVectorScores(
	ctx context.Context,
	query string,
	topK int,
) (map[string]float64, error) {
	if s.vectorSearcher == nil {
		return nil, nil
	}

	results, err := s.vectorSearcher.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}

	scores := make(map[string]float64, len(results))
	for _, r := range results {
		scores[r.AtomID] = r.Score
	}

	return scores, nil
}

// scoreAtom calculates the combined score for an atom.
func (s *AtomSelector) scoreAtom(
	atom *PromptAtom,
	cc *CompilationContext,
	vectorScores map[string]float64,
) *ScoredAtom {
	sa := &ScoredAtom{
		Atom: atom,
	}

	// Logic score based on context match quality
	sa.LogicScore = s.calculateLogicScore(atom, cc)

	// Vector score from semantic search
	if vs, ok := vectorScores[atom.ID]; ok {
		sa.VectorScore = vs
	}

	// Combined score with weighting
	logicWeight := 1.0 - s.vectorWeight
	sa.Combined = (logicWeight * sa.LogicScore) + (s.vectorWeight * sa.VectorScore)

	// Boost mandatory atoms
	if atom.IsMandatory {
		sa.Combined = 1.0 // Always maximum score
		sa.SelectionReason = "mandatory"
	} else {
		sa.SelectionReason = reasonFromScores(sa.LogicScore, sa.VectorScore)
	}

	// Priority boost (normalize priority to 0.0-0.1 boost)
	if atom.Priority > 0 {
		priorityBoost := float64(atom.Priority) / 1000.0
		if priorityBoost > 0.1 {
			priorityBoost = 0.1
		}
		sa.Combined += priorityBoost
	}

	// Cap at 1.0
	if sa.Combined > 1.0 {
		sa.Combined = 1.0
	}

	return sa
}

// calculateLogicScore determines how well an atom matches the context.
// This is a heuristic based on how many selector dimensions match.
func (s *AtomSelector) calculateLogicScore(atom *PromptAtom, cc *CompilationContext) float64 {
	if cc == nil {
		return 0.5 // Neutral score without context
	}

	// Count matching dimensions (out of total non-empty selector dimensions)
	totalDimensions := 0
	matchingDimensions := 0

	checkDimension := func(selector []string, value string) {
		if len(selector) == 0 {
			return // Empty selector = no constraint
		}
		totalDimensions++
		for _, s := range selector {
			if s == value {
				matchingDimensions++
				return
			}
		}
	}

	checkDimension(atom.OperationalModes, cc.OperationalMode)
	checkDimension(atom.CampaignPhases, cc.CampaignPhase)
	checkDimension(atom.BuildLayers, cc.BuildLayer)
	checkDimension(atom.InitPhases, cc.InitPhase)
	checkDimension(atom.NorthstarPhases, cc.NorthstarPhase)
	checkDimension(atom.OuroborosStages, cc.OuroborosStage)
	checkDimension(atom.IntentVerbs, cc.IntentVerb)
	checkDimension(atom.ShardTypes, cc.ShardType)
	checkDimension(atom.Languages, cc.Language)

	// Frameworks: check if any match
	if len(atom.Frameworks) > 0 {
		totalDimensions++
		for _, af := range atom.Frameworks {
			for _, cf := range cc.Frameworks {
				if af == cf {
					matchingDimensions++
					break
				}
			}
		}
	}

	// WorldStates: check if any required state is present
	if len(atom.WorldStates) > 0 {
		totalDimensions++
		contextStates := cc.WorldStates()
		for _, ws := range atom.WorldStates {
			for _, cs := range contextStates {
				if ws == cs {
					matchingDimensions++
					break
				}
			}
		}
	}

	// Calculate score
	if totalDimensions == 0 {
		// No selector constraints = generic atom, give moderate score
		return 0.5
	}

	// Perfect match = 1.0, partial match scales linearly
	return float64(matchingDimensions) / float64(totalDimensions)
}

// reasonFromScores generates a human-readable selection reason.
func reasonFromScores(logicScore, vectorScore float64) string {
	if logicScore > 0.8 && vectorScore > 0.8 {
		return "high logic + high vector"
	}
	if logicScore > 0.8 {
		return "high logic match"
	}
	if vectorScore > 0.8 {
		return "high semantic match"
	}
	if logicScore > 0.5 && vectorScore > 0.5 {
		return "moderate match"
	}
	return "threshold match"
}

// SelectMandatory returns all mandatory atoms regardless of context.
func (s *AtomSelector) SelectMandatory(atoms []*PromptAtom) []*ScoredAtom {
	var mandatory []*ScoredAtom

	for _, atom := range atoms {
		if atom.IsMandatory {
			mandatory = append(mandatory, &ScoredAtom{
				Atom:            atom,
				LogicScore:      1.0,
				VectorScore:     1.0,
				Combined:        1.0,
				SelectionReason: "mandatory",
			})
		}
	}

	return mandatory
}

// SelectByCategory returns atoms filtered by category.
func (s *AtomSelector) SelectByCategory(
	atoms []*PromptAtom,
	category AtomCategory,
	cc *CompilationContext,
) []*ScoredAtom {
	var filtered []*PromptAtom

	for _, atom := range atoms {
		if atom.Category == category && atom.MatchesContext(cc) {
			filtered = append(filtered, atom)
		}
	}

	// Score the filtered atoms
	scored := make([]*ScoredAtom, 0, len(filtered))
	for _, atom := range filtered {
		scored = append(scored, s.scoreAtom(atom, cc, nil))
	}

	// Sort by score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Combined > scored[j].Combined
	})

	return scored
}

// FilterByExclusionGroups handles exclusive atom groups.
// Only one atom per exclusion group is selected (highest scored).
func (s *AtomSelector) FilterByExclusionGroups(atoms []*ScoredAtom) []*ScoredAtom {
	// Track best atom per exclusion group
	exclusionWinners := make(map[string]*ScoredAtom)
	var result []*ScoredAtom

	for _, sa := range atoms {
		if sa.Atom.IsExclusive == "" {
			// No exclusion group, always include
			result = append(result, sa)
			continue
		}

		// Check if we already have a winner for this group
		group := sa.Atom.IsExclusive
		if existing, ok := exclusionWinners[group]; ok {
			// Keep the higher scored atom
			if sa.Combined > existing.Combined {
				exclusionWinners[group] = sa
			}
		} else {
			exclusionWinners[group] = sa
		}
	}

	// Add exclusion group winners to result
	for _, winner := range exclusionWinners {
		result = append(result, winner)
	}

	return result
}
