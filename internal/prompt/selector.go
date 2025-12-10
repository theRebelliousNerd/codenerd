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
		vectorWeight:      0.3, // 70% logic, 30% vector
		minScoreThreshold: 0.1, // Minimum 10% match
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
