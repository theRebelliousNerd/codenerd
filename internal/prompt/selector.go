package prompt

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// =========================================================================
// System 2 Architecture: Skeleton/Flesh Bifurcation
// =========================================================================
//
// Skeleton (Deterministic): identity, protocol, safety, methodology
//   - ALWAYS included
//   - Selected via Mangle rules, not vector search
//   - Failure is CRITICAL
//
// Flesh (Probabilistic): exemplars, domain, context, language, framework, etc.
//   - Selected via vector search + Mangle filter
//   - Failure is acceptable (degraded but safe)
// =========================================================================

// skeletonCategories defines categories that MUST always be included.
// These form the deterministic "skeleton" of every prompt.
var skeletonCategories = map[AtomCategory]bool{
	CategoryIdentity:    true,
	CategoryProtocol:    true,
	CategorySafety:      true,
	CategoryMethodology: true,
}

// isSkeletonCategory returns true if the category is part of the deterministic skeleton.
// Skeleton categories: identity, protocol, safety, methodology
// These MUST always be included and failure to load them is CRITICAL.
func isSkeletonCategory(cat AtomCategory) bool {
	return skeletonCategories[cat]
}

const (
	mangleMandatoryTokenCap    = 900000
	mangleMandatoryAtomCap     = 600
	mangleMandatoryBudgetRatio = 0.90
)

func estimateAtomTokens(atom *PromptAtom) int {
	if atom == nil {
		return 0
	}
	if atom.TokenCount > 0 {
		return atom.TokenCount
	}
	return EstimateTokens(atom.Content)
}

func mangleMandatoryLimits(cc *CompilationContext) (int, int) {
	tokenCap := mangleMandatoryTokenCap
	atomCap := mangleMandatoryAtomCap

	if cc == nil || cc.TokenBudget <= 0 {
		return tokenCap, atomCap
	}

	budget := cc.TokenBudget
	if cc.ReservedTokens > 0 && cc.ReservedTokens < budget {
		budget -= cc.ReservedTokens
	}

	budgetCap := int(float64(budget) * mangleMandatoryBudgetRatio)
	if budgetCap > 0 && budgetCap < tokenCap {
		tokenCap = budgetCap
	}

	if tokenCap < 0 {
		tokenCap = 0
	}

	return tokenCap, atomCap
}

func selectMangleMandatoryIDs(cc *CompilationContext, atoms []*PromptAtom) map[string]struct{} {
	if !isMangleMandatoryContext(cc) || len(atoms) == 0 {
		return nil
	}

	candidates := make([]*PromptAtom, 0, len(atoms))
	for _, atom := range atoms {
		if atomHasLanguage(atom, "mangle") {
			candidates = append(candidates, atom)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	tokenCap, atomCap := mangleMandatoryLimits(cc)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		tokensI := estimateAtomTokens(candidates[i])
		tokensJ := estimateAtomTokens(candidates[j])
		if tokensI != tokensJ {
			return tokensI < tokensJ
		}
		return candidates[i].ID < candidates[j].ID
	})

	selected := make(map[string]struct{}, len(candidates))
	tokensUsed := 0
	for _, atom := range candidates {
		if atomCap > 0 && len(selected) >= atomCap {
			break
		}

		tokens := estimateAtomTokens(atom)
		if tokenCap > 0 && tokensUsed+tokens > tokenCap {
			continue
		}
		selected[atom.ID] = struct{}{}
		tokensUsed += tokens
	}

	if len(selected) < len(candidates) {
		logging.Get(logging.CategoryContext).Debug(
			"Mangle mandatory cap applied: selected %d/%d atoms, tokens=%d cap=%d",
			len(selected), len(candidates), tokensUsed, tokenCap,
		)
	}

	return selected
}

func normalizeTagValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	return strings.ToLower(value)
}

func isMangleMandatoryContext(cc *CompilationContext) bool {
	if cc == nil {
		return false
	}
	shard := normalizeTagValue(cc.ShardType)
	if shard != "legislator" && shard != "mangle_repair" {
		return false
	}
	return normalizeTagValue(cc.Language) == "mangle"
}

func atomHasLanguage(atom *PromptAtom, language string) bool {
	if atom == nil {
		return false
	}
	target := normalizeTagValue(language)
	if target == "" {
		return false
	}
	for _, lang := range atom.Languages {
		if normalizeTagValue(lang) == target {
			return true
		}
	}
	return false
}

func applyMandatoryOverride(atom *PromptAtom, forcedMandatory map[string]struct{}) *PromptAtom {
	if atom == nil || atom.IsMandatory {
		return atom
	}
	if forcedMandatory == nil {
		return atom
	}
	if _, ok := forcedMandatory[atom.ID]; !ok {
		return atom
	}
	clone := *atom
	clone.IsMandatory = true
	return &clone
}

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

	// Source of selection ("skeleton" or "flesh")
	Source string
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

// SelectAtoms selects the best atoms from the candidates using System 2 bifurcation.
//
// System 2 Architecture - Skeleton/Flesh Bifurcation:
//
//	PHASE 1: Load Skeleton (deterministic)
//	  - Categories: identity, protocol, safety, methodology
//	  - Selected via Mangle rules, not vector search
//	  - Failure is CRITICAL (returns error)
//
//	PHASE 2: Load Flesh (probabilistic)
//	  - Categories: exemplars, domain, context, language, framework, etc.
//	  - Selected via vector search + Mangle filter
//	  - Failure is acceptable (degraded but safe)
//
//	PHASE 3: Merge and dedupe
//	  - Skeleton atoms take precedence
//	  - Deduplication by atom ID
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

	forcedMandatory := selectMangleMandatoryIDs(cc, atoms)

	// =========================================================================
	// PHASE 1: Load Skeleton (deterministic, CRITICAL)
	// =========================================================================
	skeleton, err := s.loadSkeletonAtoms(ctx, atoms, cc, forcedMandatory)
	if err != nil {
		return nil, fmt.Errorf("CRITICAL: skeleton atoms failed: %w", err)
	}

	logging.Get(logging.CategoryContext).Debug(
		"Phase 1 complete: %d skeleton atoms loaded", len(skeleton),
	)

	// =========================================================================
	// PHASE 2: Load Flesh (probabilistic, degradable)
	// =========================================================================
	flesh, err := s.loadFleshAtoms(ctx, atoms, cc, forcedMandatory)
	if err != nil {
		// Flesh failure is NOT critical - continue with skeleton only
		logging.Get(logging.CategoryContext).Warn(
			"Flesh atoms failed, continuing with skeleton only: %v", err,
		)
		flesh = nil
	}

	logging.Get(logging.CategoryContext).Debug(
		"Phase 2 complete: %d flesh atoms loaded", len(flesh),
	)

	// =========================================================================
	// PHASE 3: Merge and dedupe
	// =========================================================================
	merged := s.mergeAtoms(skeleton, flesh)

	logging.Get(logging.CategoryContext).Debug(
		"Phase 3 complete: %d total atoms after merge", len(merged),
	)

	return merged, nil
}

// SelectAtomsWithTiming wraps SelectAtoms and returns vector search timing.
// This method is used by the JIT compiler for comprehensive stats tracking.
// Returns:
//   - Selected atoms with scores
//   - Vector query time in milliseconds (0 if no vector search performed)
//   - Error if selection fails
func (s *AtomSelector) SelectAtomsWithTiming(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
) ([]*ScoredAtom, int64, error) {
	timer := logging.StartTimer(logging.CategoryJIT, "AtomSelector.SelectAtomsWithTiming")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, 0, nil
	}

	forcedMandatory := selectMangleMandatoryIDs(cc, atoms)

	// Track vector search timing via the flesh atom loader
	// The actual vector search happens inside loadFleshAtoms
	var vectorMs int64

	// =========================================================================
	// PHASE 1: Load Skeleton (deterministic, CRITICAL) - no vector search
	// =========================================================================
	skeleton, err := s.loadSkeletonAtoms(ctx, atoms, cc, forcedMandatory)
	if err != nil {
		return nil, 0, fmt.Errorf("CRITICAL: skeleton atoms failed: %w", err)
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Phase 1 complete: %d skeleton atoms loaded", len(skeleton),
	)

	// =========================================================================
	// PHASE 2: Load Flesh (probabilistic, degradable) - includes vector search
	// =========================================================================
	var flesh []*ScoredAtom
	if s.vectorSearcher != nil && cc != nil && cc.SemanticQuery != "" {
		vectorStart := time.Now()
		flesh, err = s.loadFleshAtoms(ctx, atoms, cc, forcedMandatory)
		vectorMs = time.Since(vectorStart).Milliseconds()

		logging.Get(logging.CategoryJIT).Debug(
			"Vector-enabled flesh loading took %dms", vectorMs,
		)
	} else {
		flesh, err = s.loadFleshAtoms(ctx, atoms, cc, forcedMandatory)
	}

	if err != nil {
		// Flesh failure is NOT critical - continue with skeleton only
		logging.Get(logging.CategoryJIT).Warn(
			"Flesh atoms failed, continuing with skeleton only: %v", err,
		)
		flesh = nil
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Phase 2 complete: %d flesh atoms loaded", len(flesh),
	)

	// =========================================================================
	// PHASE 3: Merge and dedupe
	// =========================================================================
	merged := s.mergeAtoms(skeleton, flesh)

	logging.Get(logging.CategoryJIT).Debug(
		"Phase 3 complete: %d total atoms after merge (vector=%dms)", len(merged), vectorMs,
	)

	return merged, vectorMs, nil
}

// SelectAtomsLegacy is the original implementation for backwards compatibility.
// Deprecated: Use SelectAtoms with System 2 bifurcation instead.
func (s *AtomSelector) SelectAtomsLegacy(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
) ([]*ScoredAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "AtomSelector.SelectAtomsLegacy")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, nil
	}

	if s.kernel == nil {
		return nil, fmt.Errorf("Mangle kernel not configured in selector")
	}

	// 1. Prepare Facts for Mangle
	var facts []interface{}

	// Context Facts - dimension names must be Mangle constants (start with /)
	addContextFact := func(dim, val string) {
		if val != "" {
			// Ensure dimension has leading /
			if !strings.HasPrefix(dim, "/") {
				dim = "/" + dim
			}
			// Ensure value has leading / for Mangle constants
			if !strings.HasPrefix(val, "/") {
				val = "/" + val
			}
			facts = append(facts, fmt.Sprintf("current_context(%s, %s)", dim, val))
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

	forcedMandatory := selectMangleMandatoryIDs(cc, atoms)

	// Candidate Facts
	for _, atom := range atoms {
		id := atom.ID
		isMandatory := atom.IsMandatory
		if forcedMandatory != nil {
			if _, ok := forcedMandatory[id]; ok {
				isMandatory = true
			}
		}
		facts = append(facts, fmt.Sprintf("atom('%s')", id))
		facts = append(facts, fmt.Sprintf("atom_category('%s', '%s')", id, atom.Category))
		facts = append(facts, fmt.Sprintf("atom_priority('%s', %d)", id, atom.Priority))
		if isMandatory {
			facts = append(facts, fmt.Sprintf("is_mandatory('%s')", id))
		}

		// Tags helper
		// CRITICAL: Use atoms (unquoted /dim, /value) to match current_context format
		// current_context(/shard, /coder) must match atom_tag(ID, /shard, /coder)
		// String 'shard' != atom /shard in Mangle (disjoint types)
		addTags := func(dim string, values []string) {
			for _, v := range values {
				// Ensure dimension has leading / for atom format
				atomDim := dim
				if !strings.HasPrefix(atomDim, "/") {
					atomDim = "/" + atomDim
				}
				// Ensure value has leading / for atom format
				atomVal := v
				if !strings.HasPrefix(atomVal, "/") {
					atomVal = "/" + atomVal
				}
				facts = append(facts, fmt.Sprintf("atom_tag('%s', %s, %s)", id, atomDim, atomVal))
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

	for _, fact := range results {
		// Fact: selected_result(atomID, priority, source)
		if len(fact.Args) != 3 {
			continue
		}

		atomID := extractStringArg(fact.Args[0])
		source := extractStringArg(fact.Args[2])

		if atom, exists := atomMap[atomID]; exists {
			atom = applyMandatoryOverride(atom, forcedMandatory)
			// Calculate scores locally
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
				SelectionReason: fmt.Sprintf("mangle:%s", source),
				Source:          source,
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
// Uses a 10-second sub-timeout to prevent blocking JIT compilation.
func (s *AtomSelector) getVectorScores(
	ctx context.Context,
	query string,
	topK int,
) (map[string]float64, error) {
	if s.vectorSearcher == nil {
		return nil, nil
	}

	// Use a sub-deadline to prevent vector search from blocking the entire compilation.
	// If embedding/search takes too long, we skip vector scoring rather than failing.
	searchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	results, err := s.vectorSearcher.Search(searchCtx, query, topK)
	if err != nil {
		if searchCtx.Err() != nil {
			logging.Get(logging.CategoryJIT).Warn("Vector search timed out (10s limit)")
		}
		return nil, err
	}

	scores := make(map[string]float64, len(results))
	for _, r := range results {
		scores[r.AtomID] = r.Score
	}

	return scores, nil
}

// =========================================================================
// Skeleton/Flesh Bifurcation Methods
// =========================================================================

// loadSkeletonAtoms loads mandatory atoms via Mangle rules.
// Skeleton atoms are from categories: identity, protocol, safety, methodology.
// Returns error if skeleton cannot be loaded (CRITICAL failure).
func (s *AtomSelector) loadSkeletonAtoms(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
	forcedMandatory map[string]struct{},
) ([]*ScoredAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "AtomSelector.loadSkeletonAtoms")
	defer timer.Stop()

	if s.kernel == nil {
		return nil, fmt.Errorf("CRITICAL: Mangle kernel not configured for skeleton selection")
	}

	// Filter to skeleton atoms only
	var skeletonAtoms []*PromptAtom
	for _, atom := range atoms {
		if isSkeletonCategory(atom.Category) {
			skeletonAtoms = append(skeletonAtoms, atom)
		}
	}

	if len(skeletonAtoms) == 0 {
		return nil, fmt.Errorf("CRITICAL: no skeleton atoms found in corpus")
	}

	// Build facts for Mangle query
	facts, err := s.buildContextFacts(cc, skeletonAtoms, forcedMandatory)
	if err != nil {
		return nil, fmt.Errorf("CRITICAL: failed to build skeleton context facts: %w", err)
	}

	// Assert facts to kernel
	if err := s.kernel.AssertBatch(facts); err != nil {
		return nil, fmt.Errorf("CRITICAL: failed to assert skeleton facts: %w", err)
	}

	// Debug: Query blocked atoms to diagnose context matching issues
	blockedResults, blockedErr := s.kernel.Query("blocked_by_context(Atom)")
	if blockedErr == nil && len(blockedResults) > 0 {
		logging.Get(logging.CategoryJIT).Debug(
			"JIT: %d atoms blocked by context constraints", len(blockedResults),
		)
	}

	// Debug: Query mandatory selection to see what passed
	mandatoryResults, mandatoryErr := s.kernel.Query("mandatory_selection(Atom)")
	if mandatoryErr == nil {
		logging.Get(logging.CategoryJIT).Debug(
			"JIT: %d atoms passed mandatory_selection", len(mandatoryResults),
		)
	}

	// Query for selected skeleton atoms
	// The Mangle rule should match based on:
	// - is_mandatory flag
	// - Context matching (mode, phase, shard, etc.)
	// - Category being skeleton category
	results, err := s.kernel.Query("selected_result(Atom, Priority, Source)")
	if err != nil {
		return nil, fmt.Errorf("CRITICAL: skeleton query failed: %w", err)
	}

	// Map results to ScoredAtoms
	atomMap := make(map[string]*PromptAtom, len(skeletonAtoms))
	for _, a := range skeletonAtoms {
		atomMap[a.ID] = a
	}

	var selected []*ScoredAtom
	for _, fact := range results {
		if len(fact.Args) != 3 {
			continue
		}

		atomID := extractStringArg(fact.Args[0])
		source := extractStringArg(fact.Args[2])

		// Only include skeleton category atoms from results
		atom, exists := atomMap[atomID]
		if !exists {
			continue
		}
		atom = applyMandatoryOverride(atom, forcedMandatory)

		selected = append(selected, &ScoredAtom{
			Atom:            atom,
			LogicScore:      1.0, // Skeleton atoms get full logic score
			VectorScore:     0.0, // No vector search for skeleton
			Combined:        1.0,
			SelectionReason: fmt.Sprintf("skeleton:%s", source),
			Source:          "skeleton",
		})
	}

	// Validate we have at least one atom from each skeleton category
	categoryFound := make(map[AtomCategory]bool)
	for _, sa := range selected {
		categoryFound[sa.Atom.Category] = true
	}

	for cat := range skeletonCategories {
		if !categoryFound[cat] {
			logging.Get(logging.CategoryContext).Warn(
				"Skeleton category %s has no selected atoms", cat,
			)
		}
	}

	logging.Get(logging.CategoryContext).Debug(
		"Loaded %d skeleton atoms from %d candidates", len(selected), len(skeletonAtoms),
	)

	return selected, nil
}

// loadFleshAtoms loads probabilistic atoms via vector search + Mangle filter.
// Flesh atoms are from categories: exemplars, domain, context, language, framework, etc.
// Returns nil on failure (degraded but safe operation continues with skeleton only).
func (s *AtomSelector) loadFleshAtoms(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
	forcedMandatory map[string]struct{},
) ([]*ScoredAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "AtomSelector.loadFleshAtoms")
	defer timer.Stop()

	// Filter to flesh atoms only
	var fleshAtoms []*PromptAtom
	for _, atom := range atoms {
		if !isSkeletonCategory(atom.Category) {
			fleshAtoms = append(fleshAtoms, atom)
		}
	}

	if len(fleshAtoms) == 0 {
		logging.Get(logging.CategoryContext).Debug("No flesh atoms in corpus")
		return nil, nil
	}

	// Step 1: Vector search (if enabled and query provided)
	vectorScores := make(map[string]float64)
	if s.vectorSearcher != nil && cc.SemanticQuery != "" {
		scores, err := s.getVectorScores(ctx, cc.SemanticQuery, cc.SemanticTopK)
		if err != nil {
			// Vector search failure is acceptable for flesh
			logging.Get(logging.CategoryContext).Warn("Flesh vector search failed: %v", err)
		} else {
			vectorScores = scores
		}
	}

	// Step 2: Build facts for Mangle
	facts, err := s.buildContextFacts(cc, fleshAtoms, forcedMandatory)
	if err != nil {
		// Fact building failure is logged but we continue
		logging.Get(logging.CategoryContext).Warn("Failed to build flesh context facts: %v", err)
		return nil, nil
	}

	// Add vector hits as facts
	for id, score := range vectorScores {
		facts = append(facts, fmt.Sprintf("vector_hit('%s', %f)", id, score))
	}

	// Step 3: Query Mangle (if kernel available)
	if s.kernel == nil {
		// No kernel - fall back to context matching only
		logging.Get(logging.CategoryContext).Warn("No kernel for flesh selection, using context matching")
		return s.fallbackFleshSelection(fleshAtoms, vectorScores, cc, forcedMandatory), nil
	}

	if err := s.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to assert flesh facts: %v", err)
		return s.fallbackFleshSelection(fleshAtoms, vectorScores, cc, forcedMandatory), nil
	}

	results, err := s.kernel.Query("selected_result(Atom, Priority, Source)")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("Flesh query failed: %v", err)
		return s.fallbackFleshSelection(fleshAtoms, vectorScores, cc, forcedMandatory), nil
	}

	// Step 4: Map results to ScoredAtoms
	atomMap := make(map[string]*PromptAtom, len(fleshAtoms))
	for _, a := range fleshAtoms {
		atomMap[a.ID] = a
	}

	var selected []*ScoredAtom
	for _, fact := range results {
		if len(fact.Args) != 3 {
			continue
		}

		atomID := extractStringArg(fact.Args[0])
		source := extractStringArg(fact.Args[2])

		// Only include flesh category atoms from results
		atom, exists := atomMap[atomID]
		if !exists {
			continue
		}
		atom = applyMandatoryOverride(atom, forcedMandatory)

		// Calculate combined score
		vScore := vectorScores[atomID]
		logicScore := 1.0
		combined := (1.0-s.vectorWeight)*logicScore + s.vectorWeight*vScore

		selected = append(selected, &ScoredAtom{
			Atom:            atom,
			LogicScore:      logicScore,
			VectorScore:     vScore,
			Combined:        combined,
			SelectionReason: fmt.Sprintf("flesh:%s", source),
			Source:          "flesh",
		})
	}

	logging.Get(logging.CategoryContext).Debug(
		"Loaded %d flesh atoms from %d candidates", len(selected), len(fleshAtoms),
	)

	return selected, nil
}

// fallbackFleshSelection provides flesh selection when Mangle is unavailable.
// Uses direct context matching and vector scores.
func (s *AtomSelector) fallbackFleshSelection(
	atoms []*PromptAtom,
	vectorScores map[string]float64,
	cc *CompilationContext,
	forcedMandatory map[string]struct{},
) []*ScoredAtom {
	var selected []*ScoredAtom

	for _, atom := range atoms {
		// Check context match
		if !atom.MatchesContext(cc) {
			continue
		}
		atom = applyMandatoryOverride(atom, forcedMandatory)

		// Calculate score
		vScore := vectorScores[atom.ID]
		combined := 0.5 + 0.5*vScore // Base 0.5 for context match, plus vector boost

		selected = append(selected, &ScoredAtom{
			Atom:            atom,
			LogicScore:      0.5, // Reduced score for fallback
			VectorScore:     vScore,
			Combined:        combined,
			SelectionReason: "flesh:fallback_context_match",
			Source:          "flesh",
		})
	}

	// Sort by combined score
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Combined > selected[j].Combined
	})

	return selected
}

// mergeAtoms combines skeleton and flesh atoms, deduplicating by ID.
// Skeleton atoms always take precedence (appear first, higher priority).
func (s *AtomSelector) mergeAtoms(skeleton, flesh []*ScoredAtom) []*ScoredAtom {
	timer := logging.StartTimer(logging.CategoryContext, "AtomSelector.mergeAtoms")
	defer timer.Stop()

	// Track seen IDs to deduplicate
	seen := make(map[string]bool, len(skeleton)+len(flesh))

	// Skeleton first (mandatory, deterministic)
	var result []*ScoredAtom
	for _, sa := range skeleton {
		if !seen[sa.Atom.ID] {
			seen[sa.Atom.ID] = true
			result = append(result, sa)
		}
	}

	// Then flesh (probabilistic)
	for _, sa := range flesh {
		if !seen[sa.Atom.ID] {
			seen[sa.Atom.ID] = true
			result = append(result, sa)
		}
	}

	// Sort: skeleton categories first, then by combined score
	sort.Slice(result, func(i, j int) bool {
		iSkel := isSkeletonCategory(result[i].Atom.Category)
		jSkel := isSkeletonCategory(result[j].Atom.Category)

		// Skeleton categories always first
		if iSkel != jSkel {
			return iSkel
		}

		// Within same type, mandatory first
		if result[i].Atom.IsMandatory != result[j].Atom.IsMandatory {
			return result[i].Atom.IsMandatory
		}

		// Then by combined score
		return result[i].Combined > result[j].Combined
	})

	logging.Get(logging.CategoryContext).Debug(
		"Merged %d skeleton + %d flesh = %d total atoms",
		len(skeleton), len(flesh), len(result),
	)

	return result
}

// buildContextFacts builds Mangle facts from context and atoms.
func (s *AtomSelector) buildContextFacts(cc *CompilationContext, atoms []*PromptAtom, forcedMandatory map[string]struct{}) ([]interface{}, error) {
	var facts []interface{}

	// Context Facts - dimension names must be Mangle constants (start with /)
	addContextFact := func(dim, val string) {
		if val != "" {
			// Ensure dimension has leading /
			if !strings.HasPrefix(dim, "/") {
				dim = "/" + dim
			}
			// Ensure value has leading / for Mangle constants
			if !strings.HasPrefix(val, "/") {
				val = "/" + val
			}
			facts = append(facts, fmt.Sprintf("current_context(%s, %s)", dim, val))
		}
	}
	addContextFact("mode", cc.OperationalMode)
	addContextFact("phase", cc.CampaignPhase)
	addContextFact("layer", cc.BuildLayer)
	addContextFact("init_phase", cc.InitPhase)
	addContextFact("northstar_phase", cc.NorthstarPhase)
	addContextFact("ouroboros_stage", cc.OuroborosStage)
	addContextFact("intent", cc.IntentVerb)
	addContextFact("shard", cc.ShardType)
	addContextFact("lang", cc.Language)
	for _, fw := range cc.Frameworks {
		addContextFact("framework", fw)
	}
	for _, ws := range cc.WorldStates() {
		addContextFact("state", ws)
	}

	// Candidate Facts
	for _, atom := range atoms {
		id := atom.ID
		isMandatory := atom.IsMandatory
		if forcedMandatory != nil {
			if _, ok := forcedMandatory[id]; ok {
				isMandatory = true
			}
		}
		facts = append(facts, fmt.Sprintf("atom('%s')", id))
		facts = append(facts, fmt.Sprintf("atom_category('%s', '%s')", id, atom.Category))
		facts = append(facts, fmt.Sprintf("atom_priority('%s', %d)", id, atom.Priority))
		if isMandatory {
			facts = append(facts, fmt.Sprintf("is_mandatory('%s')", id))
		}

		// GAP-FIX: Emit unified prompt_atom/5 fact required by jit_selection.mg

		// prompt_atom(ID, Category, Priority, Hash, IsMandatory)

		isMandatoryAtom := "/false"
		if isMandatory {
			isMandatoryAtom = "/true"
		}

		hash := atom.ContentHash

		if hash == "" {

			hash = "nohash"

		}

		// Category must be an atom (e.g. /identity) not a string ('identity')

		category := string(atom.Category)

		if !strings.HasPrefix(category, "/") {

			category = "/" + category

		}

		facts = append(facts, fmt.Sprintf("prompt_atom('%s', %s, %d, '%s', %s)",

			id, category, atom.Priority, hash, isMandatoryAtom))

		// Tags helper		// CRITICAL: Use atoms (unquoted /dim, /value) to match current_context format
		// current_context(/shard, /coder) must match atom_tag(ID, /shard, /coder)
		// String 'shard' != atom /shard in Mangle (disjoint types)
		addTags := func(dim string, values []string) {
			for _, v := range values {
				// Ensure dimension has leading / for atom format
				atomDim := dim
				if !strings.HasPrefix(atomDim, "/") {
					atomDim = "/" + atomDim
				}
				// Ensure value has leading / for atom format
				atomVal := v
				if !strings.HasPrefix(atomVal, "/") {
					atomVal = "/" + atomVal
				}
				facts = append(facts, fmt.Sprintf("atom_tag('%s', %s, %s)", id, atomDim, atomVal))
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
		addTags("lang", atom.Languages)
		addTags("framework", atom.Frameworks)
		addTags("state", atom.WorldStates)

		// Dependencies - needed for atom_requires() in jit_compiler.mg
		for _, dep := range atom.DependsOn {
			if dep != "" {
				facts = append(facts, fmt.Sprintf("atom_requires('%s', '%s')", id, dep))
			}
		}

		// Conflicts - needed for atom_conflicts() in jit_compiler.mg
		for _, conflict := range atom.ConflictsWith {
			if conflict != "" {
				facts = append(facts, fmt.Sprintf("atom_conflicts('%s', '%s')", id, conflict))
			}
		}
	}

	return facts, nil
}

// extractStringArg safely extracts a string from a Mangle fact argument.
func extractStringArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
