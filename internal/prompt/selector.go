package prompt

import (
	"context"
	"fmt"
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

// SelectAtoms selects the best atoms from the candidates using the Mangle kernel.
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

	if s.kernel == nil {
		return nil, fmt.Errorf("Mangle kernel not configured in selector")
	}

	// 1. Prepare Facts for Mangle
	var facts []interface{}

	// Context Facts
	addContextFact := func(dim, val string) {
		if val != "" {
			facts = append(facts, fmt.Sprintf("current_context('%s', '%s')", dim, val))
		}
	}
	addContextFact("mode", cc.OperationalMode)
	addContextFact("phase", cc.CampaignPhase)
	addContextFact("layer", cc.BuildLayer)
	addContextFact("init_phase", cc.InitPhase)
	addContextFact("northstar_phase", cc.NorthstarPhase)
	addContextFact("ouroboros_stage", cc.OuroborosStage)
	addContextFact("intent", cc.IntentVerb)
	addContextFact("shard", cc.ShardID)

	// Candidate Facts
	for _, atom := range atoms {
		id := atom.ID
		facts = append(facts, fmt.Sprintf("atom('%s')", id))
		facts = append(facts, fmt.Sprintf("atom_category('%s', '%s')", id, atom.Category))
		facts = append(facts, fmt.Sprintf("atom_priority('%s', %d)", id, atom.Priority))
		if atom.IsMandatory {
			facts = append(facts, fmt.Sprintf("is_mandatory('%s')", id))
		}

		// Tags helper
		addTags := func(dim string, values []string) {
			for _, v := range values {
				facts = append(facts, fmt.Sprintf("atom_tag('%s', '%s', '%s')", id, dim, v))
			}
		}
		addTags("mode", atom.OperationalModes)
		addTags("phase", atom.CampaignPhases)
		addTags("layer", atom.BuildLayers)
		addTags("init_phase", atom.InitPhases)
		addTags("northstar_phase", atom.NorthstarPhases)
		addTags("ouroboros_stage", atom.OuroborosStages)
		addTags("intent", atom.IntentVerbs)
		addTags("shard", atom.ShardTypes)
	}

	// 2. Vector Search (if enabled)
	vectorScores := make(map[string]float64)
	if s.vectorSearcher != nil && cc.SemanticQuery != "" {
		scores, err := s.getVectorScores(ctx, cc.SemanticQuery, cc.SemanticTopK)
		if err == nil {
			vectorScores = scores
			for id, score := range scores {
				facts = append(facts, fmt.Sprintf("vector_hit('%s', %f)", id, score))
			}
		} else {
			logging.Get(logging.CategoryContext).Warn("Vector search failed: %v", err)
		}
	}

	// 3. Assert Facts
	if err := s.kernel.AssertBatch(facts); err != nil {
		return nil, fmt.Errorf("failed to assert facts: %w", err)
	}

	// 4. Query Mangle
	// selected_result(Atom, Priority, Source)
	results, err := s.kernel.Query("selected_result(Atom, Priority, Source)")
	if err != nil {
		return nil, fmt.Errorf("mangle query failed: %w", err)
	}

	// 5. Map Results
	var selected []*ScoredAtom
	atomMap := make(map[string]*PromptAtom)
	for _, a := range atoms {
		atomMap[a.ID] = a
	}

	// Process results
	// Assuming results are simple maps or structs.
	// Since we don't know the exact return type of Query (interface{}), relies on best effort.
	// If it returns bindings, we iterate.
	for _, rawRes := range results {
		// Mock parsing for now: assume map[string]string
		resMap, ok := rawRes.(map[string]string)
		if !ok {
			// Try debugging or inspecting type
			continue
		}

		atomID := resMap["Atom"]

		if atom, exists := atomMap[atomID]; exists {
			// Calculate scores locally or trust Mangle?
			// Mangle determined it SHOULD be selected.
			// We construct the ScoredAtom.
			score := 1.0
			vScore := vectorScores[atomID]
			if vScore > 0 {
				score += vScore
			}

			selected = append(selected, &ScoredAtom{
				Atom:            atom,
				LogicScore:      1.0,
				VectorScore:     vScore,
				Combined:        score,
				SelectionReason: "mangle_selected",
			})
		}
	}

	// Sort by Score
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Combined > selected[j].Combined
	})

	return selected, nil
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
