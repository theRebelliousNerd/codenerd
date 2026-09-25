package prompt

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// skeletonCategories defines the categories forming the deterministic
// "skeleton" tier of every prompt. Only mandatory atoms in these categories
// are guaranteed selection; every non-mandatory atom, whatever its category,
// competes in the flesh tier (see filterFleshAtoms).
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

// factBuilder is a specialized buffer for constructing Mangle facts efficiently
// without excessive string allocations.
type factBuilder struct {
	strings.Builder
	numBuf [32]byte
}

// Reset clears the builder for reuse.
func (b *factBuilder) Reset() {
	b.Builder.Reset()
}

// WriteInt formats an integer directly into the builder without allocating a string.
func (b *factBuilder) WriteInt(n int) {
	b.Write(strconv.AppendInt(b.numBuf[:0], int64(n), 10))
}

// WriteQuotedString writes a Mangle-quoted string directly to the builder.

func (b *factBuilder) writeAtom(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	needsPrefix := !strings.HasPrefix(s, "/")

	// No quoting escape hatch: bytes outside the name-constant alphabet fold
	// to '_' in the loop below. An earlier revision single-quoted values with
	// whitespace ("Coder Shard" -> '/Coder Shard'), but Mangle strings are
	// double-quoted -- the lexer rejects single quotes, so every fact routed
	// through that branch died at assert time. Normalizing keeps facts parseable.

	var hasWritten bool
	start := 0
	if !needsPrefix {
		start = 1
	}

	for i := start; i <= len(s); i++ {
		if i == len(s) || s[i] == '/' {
			part := s[start:i]
			start = i + 1
			if len(part) == 0 {
				continue
			}

			first := -1
			last := -1
			for j := 0; j < len(part); j++ {
				c := part[j]
				if c >= 'A' && c <= 'Z' {
					c += 32
				}
				isValid := false
				switch {
				case c >= 'a' && c <= 'z':
					isValid = true
				case c >= '0' && c <= '9':
					isValid = true
				case c == '.' || c == '-' || c == '_' || c == '~' || c == '%':
					isValid = true
				}

				if !isValid {
					c = '_'
				}

				if c != '_' {
					if first == -1 {
						first = j
					}
					last = j
				}
			}

			if first != -1 {
				if !hasWritten {
					b.WriteByte('/')
					hasWritten = true
				} else {
					b.WriteByte('/')
				}

				for j := first; j <= last; j++ {
					c := part[j]
					if c >= 'A' && c <= 'Z' {
						c += 32
					}
					switch {
					case c >= 'a' && c <= 'z':
						b.WriteByte(c)
					case c >= '0' && c <= '9':
						b.WriteByte(c)
					case c == '.' || c == '-' || c == '_' || c == '~' || c == '%':
						b.WriteByte(c)
					default:
						b.WriteByte('_')
					}
				}
			}
		}
	}

	return hasWritten
}

func (b *factBuilder) writeStringLiteral(s string) {
	writeMangleQuoted(&b.Builder, s)
}

// mangleNormalizeNameConst is kept for backwards compatibility but implemented via factBuilder
func mangleNormalizeNameConst(s string) string {
	var fb factBuilder
	if fb.writeAtom(s) {
		return fb.String()
	}
	return ""
}
func (b *factBuilder) WriteQuotedString(s string) {
	writeMangleQuoted(&b.Builder, s)
}

// vectorScoreToPercent maps a similarity score to the integer 0-100 scale the
// JIT selection policy compares against.
//
// The kernel's /number bound is int64 in this Mangle fork, and
// jit_selection.mg gates candidates on `Score > 30` "on 0-100 scale". Rounding
// rather than truncating keeps 0.999 from landing below an identical score
// recorded as 1.0, and clamping guards against a searcher returning a value
// slightly outside 0..1 (dot-product backends can).
func vectorScoreToPercent(score float64) int64 {
	if math.IsNaN(score) {
		return 0
	}
	pct := math.Round(score * 100)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return int64(pct)
}

// topKEligibleVectorScores reduces a whole-corpus vector score map to the k
// highest scores among flesh-eligible atoms, so the vector channel spends its
// slots on atoms the kernel can actually select.
//
// An atom is eligible when it is in the flesh set, is not mandatory
// (mandatory atoms are selected regardless and need no vector slot), carries
// no RetrievedContext witness (retrieved context reaches selection through
// its own fact, not through vector_hit), matches the fail-closed context
// matcher, and has a non-NaN score in the map.
//
// Of the eligible scores, only those at or above both floors survive: the
// relative floor (eligible mean plus 1.5 population standard deviations,
// skipped when fewer than 8 atoms are eligible) and s.minScoreThreshold as
// the absolute lower bound. The relative floor is needed because embedding
// models differ in range (nomic-embed-text: median ~0.52, best ~0.70), which
// makes any fixed absolute threshold meaningless on its own. Survivors are
// ordered by score descending with ties broken by atom ID ascending, and the
// first k are returned; k <= 0 means 10.
func (s *AtomSelector) topKEligibleVectorScores(
	scores map[string]float64,
	fleshAtoms []*PromptAtom,
	cc *CompilationContext,
	k int,
) map[string]float64 {
	if len(scores) == 0 || len(fleshAtoms) == 0 {
		// Say so. This return used to be silent, and the "Vector tier:" line
		// below is the signal the live validation of the selector reads for;
		// with every stored vector unstamped (the state before `nerd embedding
		// reembed` runs) the search returns no scores, this path is taken on
		// every compile, and the log showed nothing to distinguish "the vector
		// channel kept 0 of N" from "the vector channel never ran".
		logging.Get(logging.CategoryJIT).Debug(
			"Vector tier: nothing to rank (%d scored, %d flesh atoms); the vector channel contributes no atoms this compile",
			len(scores), len(fleshAtoms),
		)
		return nil
	}
	if k <= 0 {
		k = 10
	}
	absoluteFloor := 0.0
	if s != nil {
		absoluteFloor = s.minScoreThreshold
	}

	type candidate struct {
		id    string
		score float64
	}
	var eligible []candidate
	for _, atom := range fleshAtoms {
		if atom == nil || atom.IsMandatory || atom.RetrievedContext {
			continue
		}
		score, ok := scores[atom.ID]
		if !ok || math.IsNaN(score) {
			continue
		}
		if !atom.MatchesContext(cc) {
			continue
		}
		eligible = append(eligible, candidate{id: atom.ID, score: score})
	}

	floor := absoluteFloor
	if len(eligible) >= 8 {
		var sum float64
		for _, c := range eligible {
			sum += c.score
		}
		mean := sum / float64(len(eligible))
		var variance float64
		for _, c := range eligible {
			d := c.score - mean
			variance += d * d
		}
		variance /= float64(len(eligible))
		floor = mean + 1.5*math.Sqrt(variance)
	}

	kept := make([]candidate, 0, len(eligible))
	for _, c := range eligible {
		if c.score >= absoluteFloor && c.score >= floor {
			kept = append(kept, c)
		}
	}
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].score != kept[j].score {
			return kept[i].score > kept[j].score
		}
		return kept[i].id < kept[j].id
	})
	if len(kept) > k {
		kept = kept[:k]
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Vector tier: %d scored, %d eligible, floor=%.3f, kept %d",
		len(scores), len(eligible), floor, len(kept),
	)

	if len(kept) == 0 {
		return nil
	}
	reduced := make(map[string]float64, len(kept))
	for _, c := range kept {
		reduced[c.id] = c.score
	}
	return reduced
}

func mangleQuoteString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	writeMangleQuoted(&sb, s)
	return sb.String()
}

// writeMangleQuoted writes s as a double-quoted Mangle string literal,
// escaping with the escapes the Mangle lexer supports:
//
//	\" \\ \n \t \xHH \u{HHHH[HH]}
//
// Output is ASCII-only: raw bytes above 0x7F cannot survive the lexer, and
// \u{...} demands 4-6 hex digits. This is the single engine behind
// mangleQuoteString, factBuilder.WriteQuotedString and
// factBuilder.writeStringLiteral, which used to be three loops that disagreed:
// writeStringLiteral passed raw UTF-8 through (lex-rejected) and
// WriteQuotedString emitted unpadded \u{e9} escapes (also lex-rejected). Any
// context value with non-ASCII text — a unicode path, a pasted snippet —
// produced facts the kernel refused to assert.
func writeMangleQuoted(sb *strings.Builder, s string) {
	const hex = "0123456789abcdef"

	if s == "" {
		sb.WriteString("\"\"")
		return
	}

	sb.WriteByte('"')

	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\t':
			sb.WriteString("\\t")
		// No \r case on purpose: the lexer accepts \n \t \\ " ' ` \xHH
		// \u{h..} and nothing else, so \r (like \b \f \v) falls through to
		// the \xHH branch below.
		default:
			// Printable ASCII (excluding backslash/quote handled above).
			if r >= 0x20 && r <= 0x7e {
				sb.WriteRune(r)
				continue
			}

			// Control bytes and low bytes: \xHH
			if r >= 0 && r <= 0xff {
				sb.WriteString("\\x")
				b := byte(r)
				sb.WriteByte(hex[b>>4])
				sb.WriteByte(hex[b&0x0f])
				continue
			}

			// Unicode: \u{hhhh} (4-6 lowercase hex digits)
			sb.WriteString("\\u{")
			width := 4
			if r > 0xffff {
				width = 6
			}
			for shift := (width - 1) * 4; shift >= 0; shift -= 4 {
				d := byte((r >> shift) & 0x0f)
				sb.WriteByte(hex[d])
			}
			sb.WriteByte('}')
		}
	}

	sb.WriteByte('"')
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
		if atom == nil {
			continue
		}
		if !atom.IsMandatory {
			continue
		}
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

	// Timeout for vector searches
	vectorSearchTimeout time.Duration
}

// NewAtomSelector creates a new atom selector with default settings.
func NewAtomSelector() *AtomSelector {
	return &AtomSelector{
		vectorWeight:        0.3, // 70% logic, 30% vector
		minScoreThreshold:   0.1, // Minimum 10% match
		vectorSearchTimeout: 10 * time.Second,
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

// SetVectorSearchTimeout sets the timeout duration for vector searches.
func (s *AtomSelector) SetVectorSearchTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	s.vectorSearchTimeout = timeout
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
//

func (s *AtomSelector) SelectAtoms(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
) ([]*ScoredAtom, error) {
	merged, _, err := s.runSelection(ctx, atoms, cc, s.kernel, false)
	return merged, err
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
	return s.selectAtomsWithTimingKernel(ctx, atoms, cc, s.kernel)
}

func (s *AtomSelector) selectAtomsWithTimingKernel(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
	kernel KernelQuerier,
) ([]*ScoredAtom, int64, error) {
	return s.runSelection(ctx, atoms, cc, kernel, true)
}

// runSelection is the single engine behind SelectAtoms and
// SelectAtomsWithTiming, which were two copies of the same three phases.
//
// The phases run build-all, assert-all, query-all: both phases build their
// fact sets in parallel (pure computation, plus the flesh vector search),
// both fact sets land before either query runs, and the two read-only queries
// run in parallel. The old shape ran each phase as build+assert+query in its
// own goroutine, so a flesh query could run before the skeleton's facts
// landed: conflict_loser and the dependency closure in jit_compiler.mg are
// union-sensitive, and a flesh candidate that should have lost to a skeleton
// mandatory survived whenever the goroutines interleaved that way. Same
// input, different prompt, depending on the scheduler.
//
// With wantTiming the flesh vector search itself is timed (previously the
// timer wrapped the whole flesh load — build, assert and query included —
// under the name "vector query time").
func (s *AtomSelector) runSelection(
	ctx context.Context,
	atoms []*PromptAtom,
	cc *CompilationContext,
	kernel KernelQuerier,
	wantTiming bool,
) ([]*ScoredAtom, int64, error) {
	timer := logging.StartTimer(logging.CategoryJIT, "AtomSelector.runSelection")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, 0, nil
	}

	if cc == nil {
		cc = NewCompilationContext()
	}

	atoms = filterAtomsForStructuredOutput(atoms, cc)
	if len(atoms) == 0 {
		return nil, 0, nil
	}

	forcedMandatory := selectMangleMandatoryIDs(cc, atoms)

	// =========================================================================
	// PHASE A: build both fact sets in parallel (no kernel writes yet)
	// =========================================================================
	var skeletonAtoms, fleshAtoms []*PromptAtom
	var skeletonFacts, fleshFacts []any
	var vectorScores map[string]float64
	var skeletonBuildErr, fleshBuildErr error
	var vectorMs int64
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer recoverSelectionPanic("skeleton build", &skeletonBuildErr)
		skeletonAtoms = filterSkeletonAtoms(atoms, cc)
		if len(skeletonAtoms) == 0 {
			skeletonBuildErr = fmt.Errorf("CRITICAL: no skeleton atoms found in corpus")
			return
		}
		var err error
		skeletonFacts, err = s.buildContextFacts(cc, skeletonAtoms, forcedMandatory)
		if err != nil {
			skeletonBuildErr = fmt.Errorf("CRITICAL: failed to build skeleton context facts: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer recoverSelectionPanic("flesh build", &fleshBuildErr)
		fleshAtoms = filterFleshAtoms(atoms, cc)
		if len(fleshAtoms) == 0 {
			logging.Get(logging.CategoryContext).Debug("No flesh atoms in corpus")
			return
		}
		vectorSearch := func() map[string]float64 {
			if s.vectorSearcher == nil || cc.SemanticQuery == "" {
				return nil
			}
			// Fetch scores for the whole corpus, not just the global top-K:
			// the global top-K is dominated by atoms that are already
			// mandatory or gated to another shard or language, whose slots
			// are then discarded, while the relevant eligible atom ranked
			// just below never receives a vector_hit. Reduction to the k
			// highest eligible scores happens below.
			return s.getVectorScores(ctx, cc.SemanticQuery, len(atoms))
		}
		var rawScores map[string]float64
		if wantTiming {
			vectorStart := time.Now()
			rawScores = vectorSearch()
			vectorMs = time.Since(vectorStart).Milliseconds()
		} else {
			rawScores = vectorSearch()
		}
		vectorScores = s.topKEligibleVectorScores(rawScores, fleshAtoms, cc, cc.SemanticTopK)
		var err error
		fleshFacts, err = s.buildFleshFacts(cc, fleshAtoms, forcedMandatory, vectorScores)
		if err != nil {
			// Flesh fact failure degrades to no flesh facts; the query
			// phase falls back to keyword matching below.
			logging.Get(logging.CategoryContext).Warn("Failed to build flesh context facts: %v", err)
			fleshFacts = nil
		}
	}()

	wg.Wait()

	if skeletonBuildErr != nil {
		return nil, 0, skeletonBuildErr
	}
	// A flesh build panic degrades to no flesh (the retired sequential
	// flesh loader turned the same panic into a flesh error, which also
	// dropped flesh). Everything else about a failed flesh build falls back to
	// keyword matching in the query phase.
	fleshFailed := fleshBuildErr != nil
	if fleshFailed {
		logging.Get(logging.CategoryContext).Warn(
			"Flesh atoms failed, continuing with skeleton only: %v", fleshBuildErr,
		)
		fleshAtoms = nil
		fleshFacts = nil
	}

	// =========================================================================
	// ASSERT: both fact sets land before either query runs
	// =========================================================================
	fleshReady := false
	if kernel == nil {
		return nil, 0, fmt.Errorf("CRITICAL: Mangle kernel not configured for skeleton selection")
	}
	if err := kernel.AssertBatch(skeletonFacts); err != nil {
		return nil, 0, fmt.Errorf("CRITICAL: failed to assert skeleton facts: %w", err)
	}
	if !fleshFailed && fleshFacts != nil {
		if err := kernel.AssertBatch(fleshFacts); err != nil {
			// Logged at Error: the selector falls back to keyword
			// matching, which INVERTS selection for every situational
			// dimension rather than approximating it. jit_compiler.mg is
			// permissive by default and fail-closed only for the nine
			// regime_dimension entries; matchSelector is fail-closed for
			// everything, with one hand-made exception for frameworks. So
			// a context missing a language admits all 326 language-gated
			// atoms on the kernel path and none of them here; the same
			// holds for 195 intent-gated and 21 world-state-gated
			// entries (542 of 918 corpus entries select the opposite way).
			// Two turns either side of a transient kernel failure are
			// compiled by two different policies, which is why this is an
			// Error and not a Warn.
			logging.Get(logging.CategoryContext).Error("Failed to assert flesh facts (selector falls back to keyword matching — selection semantics INVERT for situational dimensions): %v", err)
		} else {
			fleshReady = true
		}
	}

	// =========================================================================
	// PHASE B: query both phases in parallel (read-only on a complete EDB)
	// =========================================================================
	var skeleton, flesh []*ScoredAtom
	var skeletonErr, fleshErr error
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer recoverSelectionPanic("skeleton query", &skeletonErr)
		skeleton, skeletonErr = s.querySkeletonAtoms(kernel, skeletonAtoms, forcedMandatory)
	}()

	go func() {
		defer wg.Done()
		defer recoverSelectionPanic("flesh query", &fleshErr)
		flesh = s.queryFleshAtoms(kernel, fleshAtoms, fleshReady, forcedMandatory, vectorScores, cc)
	}()

	wg.Wait()

	if skeletonErr != nil {
		return nil, 0, skeletonErr
	}
	if fleshErr != nil {
		// Flesh failure is NOT critical - continue with skeleton only.
		// (Only a query-phase panic lands here; assert/query failures
		// already fell back to keyword matching inside queryFleshAtoms.)
		logging.Get(logging.CategoryContext).Warn(
			"Flesh atoms failed, continuing with skeleton only: %v", fleshErr,
		)
		flesh = nil
	}

	// =========================================================================
	// PHASE 3: Merge and dedupe
	// =========================================================================
	merged := s.mergeAtoms(skeleton, flesh)

	logging.Get(logging.CategoryJIT).Debug(
		"Selection complete: %d skeleton + %d flesh = %d total atoms (vector=%dms)",
		len(skeleton), len(flesh), len(merged), vectorMs,
	)

	return merged, vectorMs, nil
}

func recoverSelectionPanic(phase string, errp *error) {
	if recovered := recover(); recovered != nil {
		*errp = fmt.Errorf("%s selector panic: %v", phase, recovered)
	}
}

// getVectorScores retrieves semantic similarity scores for atoms.
// Uses a configurable sub-timeout to prevent blocking JIT compilation.
// Gracefully degrades (returns nil vector scores) if vector search returns an error or times out.
func (s *AtomSelector) getVectorScores(
	ctx context.Context,
	query string,
	topK int,
) map[string]float64 {
	if s.vectorSearcher == nil {
		return nil
	}

	// Use a sub-deadline to prevent vector search from blocking the entire compilation.
	// If embedding/search takes too long, we skip vector scoring rather than failing.
	searchCtx, cancel := context.WithTimeout(ctx, s.vectorSearchTimeout)
	defer cancel()

	results, err := s.vectorSearcher.Search(searchCtx, query, topK)
	if err != nil {
		if searchCtx.Err() != nil {
			logging.Get(logging.CategoryJIT).Warn("Vector search timed out (%v limit)", s.vectorSearchTimeout)
		} else {
			logging.Get(logging.CategoryJIT).Warn("Vector search failed: %v", err)
		}
		return nil
	}

	scores := make(map[string]float64, len(results))
	for _, r := range results {
		scores[r.AtomID] = r.Score
	}

	return scores
}

// =========================================================================
// Skeleton/Flesh Bifurcation Methods
// =========================================================================

// filterSkeletonAtoms keeps the skeleton-category atoms whose world state
// matches and whose required tools are all present. The skeleton tier is the
// set of mandatory atoms in skeleton categories; this filter is unchanged and
// keeps non-mandatory skeleton-category atoms here too so depends_on
// resolution is unaffected (mergeAtoms de-duplicates by ID, skeleton first).
// An explicit requires_tools entry means the same thing here as in flesh:
// the atom is omitted when the effective catalog lacks any required tool.
// Tool-agnostic constitutional, evidence and general identity atoms carry no
// requires_tools and are always retained.
func filterSkeletonAtoms(atoms []*PromptAtom, cc *CompilationContext) []*PromptAtom {
	available := availableToolSet(cc)
	var skeletonAtoms []*PromptAtom
	for _, atom := range atoms {
		if atom != nil && isSkeletonCategory(atom.Category) && atomMatchesActiveWorldState(atom, cc) && atomToolSatisfied(atom, available) {
			skeletonAtoms = append(skeletonAtoms, atom)
		}
	}
	return skeletonAtoms
}

// querySkeletonAtoms runs the skeleton read phase against a kernel whose
// skeleton facts are already asserted: diagnostic queries, then the
// selected_result mapping. It writes nothing, so runSelection can run it in
// parallel with the flesh query once both fact sets have landed.
func (s *AtomSelector) querySkeletonAtoms(
	kernel KernelQuerier,
	skeletonAtoms []*PromptAtom,
	forcedMandatory map[string]struct{},
) ([]*ScoredAtom, error) {
	// Debug: Query blocked atoms to diagnose context matching issues
	blockedResults, blockedErr := kernel.Query("blocked_by_context(Atom)")
	if blockedErr == nil && len(blockedResults) > 0 {
		logging.Get(logging.CategoryJIT).Debug(
			"JIT: %d atoms blocked by context constraints", len(blockedResults),
		)
	}

	// Debug: Query mandatory selection to see what passed
	mandatoryResults, mandatoryErr := kernel.Query("mandatory_selection(Atom)")
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
	results, err := kernel.Query("selected_result(Atom, Priority, Source)")
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

		atomID, err1 := extractStringArg(fact.Args[0])
		source, err2 := extractStringArg(fact.Args[2])
		if err1 != nil || err2 != nil {
			logging.Get(logging.CategoryContext).Warn("querySkeletonAtoms: Skipping invalid fact args: %v, %v", err1, err2)
			continue
		}

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

// filterFleshAtoms keeps the atoms whose world state matches and whose
// required tools are all present, excluding only mandatory atoms in skeleton
// categories (those are already guaranteed by the skeleton tier). Every
// non-mandatory atom, whatever its category, competes in the flesh tier, so a
// non-mandatory skeleton-category atom appears in both the skeleton set (for
// depends_on resolution) and the flesh set (for vector-hit relevance). An
// explicit requires_tools entry omits the atom when the effective catalog
// lacks any required tool, the same rule skeleton selection enforces.
func filterFleshAtoms(atoms []*PromptAtom, cc *CompilationContext) []*PromptAtom {
	available := availableToolSet(cc)
	var fleshAtoms []*PromptAtom
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		if !atomMatchesActiveWorldState(atom, cc) || !atomToolSatisfied(atom, available) {
			continue
		}
		if isSkeletonCategory(atom.Category) && atom.IsMandatory {
			continue
		}
		fleshAtoms = append(fleshAtoms, atom)
	}
	return fleshAtoms
}

// buildFleshFacts builds the context facts for the flesh atoms plus the
// vector-hit and retrieved-context witness facts.
func (s *AtomSelector) buildFleshFacts(
	cc *CompilationContext,
	fleshAtoms []*PromptAtom,
	forcedMandatory map[string]struct{},
	vectorScores map[string]float64,
) ([]any, error) {
	facts, err := s.buildContextFacts(cc, fleshAtoms, forcedMandatory)
	if err != nil {
		return nil, err
	}

	// Add vector hits as facts, scaled to the integer 0-100 the policy expects.
	//
	// This emitted strconv.FormatFloat(score, 'g', -1, 64) — a float in 0..1 —
	// and was wrong twice over:
	//
	//  1. Wrong type. schemas_prompts.mg:359 declares
	//     `vector_hit(AtomID, Score) bound [/string, /number]`, and /number is
	//     int64 in this Mangle fork. Every float-valued fact was rejected
	//     outright: 1,209 "rejecting fact that fails ToAtom: vector_hit"
	//     entries in a single day.
	//  2. Wrong scale. jit_selection.mg:211 filters `Score > 30` and says so in
	//     its own comment — "sufficient similarity (> 30 on 0-100 scale)". A
	//     cosine score never exceeds 1, so even had the type been accepted no
	//     candidate could ever have passed that gate.
	//
	// Between them, Mangle flesh selection had never once seen a usable vector
	// score, and every turn fell back to keyword matching. Rounding to an
	// integer percentage is order-preserving, so the rules' comparisons keep
	// their meaning — the same treatment the Ouroboros stability score needed
	// for the same int64-only reason.
	for id, score := range vectorScores {
		facts = append(facts, "vector_hit("+mangleQuoteString(id)+", "+strconv.FormatInt(vectorScoreToPercent(score), 10)+")")
	}
	// Runtime retrieval has already matched these ephemeral records to the
	// task. They cannot appear in the persistent prompt-vector index. Emit a
	// distinct witness so Mangle still applies context, conflict and dependency
	// rules without pretending this was a cosine similarity or mandatory atom.
	for _, atom := range fleshAtoms {
		if atom.RetrievedContext {
			facts = append(facts, "retrieved_context("+mangleQuoteString(atom.ID)+")")
		}
	}
	return facts, nil
}

// queryFleshAtoms runs the flesh read phase. When ready is false the flesh
// facts never landed (no kernel, or the assert failed) and it goes straight
// to keyword matching; a query failure degrades the same way. It never
// returns an error: flesh is the degradable phase.
func (s *AtomSelector) queryFleshAtoms(
	kernel KernelQuerier,
	fleshAtoms []*PromptAtom,
	ready bool,
	forcedMandatory map[string]struct{},
	vectorScores map[string]float64,
	cc *CompilationContext,
) []*ScoredAtom {
	if !ready {
		if kernel == nil {
			logging.Get(logging.CategoryContext).Warn("No kernel for flesh selection, using context matching")
		}
		return s.fallbackFleshSelection(fleshAtoms, vectorScores, cc, forcedMandatory)
	}

	results, err := kernel.Query("selected_result(Atom, Priority, Source)")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("Flesh query failed: %v", err)
		return s.fallbackFleshSelection(fleshAtoms, vectorScores, cc, forcedMandatory)
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

		atomID, err1 := extractStringArg(fact.Args[0])
		source, err2 := extractStringArg(fact.Args[2])
		if err1 != nil || err2 != nil {
			logging.Get(logging.CategoryContext).Warn("queryFleshAtoms: Skipping invalid fact args: %v, %v", err1, err2)
			continue
		}

		// Only include flesh category atoms from results
		atom, exists := atomMap[atomID]
		if !exists {
			continue
		}
		atom = applyMandatoryOverride(atom, forcedMandatory)

		// Calculate combined score
		vScore := vectorScores[atomID]
		logicScore := 1.0
		weight := s.vectorWeight
		if cc != nil && cc.HasVectorWeight {
			weight = cc.VectorWeight
		}
		combined := (1.0-weight)*logicScore + weight*vScore

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

	return selected
}

func atomMatchesActiveWorldState(atom *PromptAtom, cc *CompilationContext) bool {
	if atom == nil || len(atom.WorldStates) == 0 {
		return true
	}
	if cc == nil {
		return false
	}
	for _, state := range atom.WorldStates {
		if hasWorldState(cc, normalizeTagValue(state)) {
			return true
		}
	}
	return false
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
	available := availableToolSet(cc)

	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		// Check context match
		if !atom.MatchesContext(cc) {
			continue
		}
		// Capability gating: omit tool-gated atoms whose tools are absent.
		if !atomToolSatisfied(atom, available) {
			continue
		}

		// Calculate score
		vScore := vectorScores[atom.ID]
		weight := s.vectorWeight
		if cc != nil && cc.HasVectorWeight {
			weight = cc.VectorWeight
		}
		combined := (1.0 - weight) + weight*vScore // Base logic score is implicitly 1.0 for the multiplier before vector weight
		atom = applyMandatoryOverride(atom, forcedMandatory)

		selected = append(selected, &ScoredAtom{
			Atom:            atom,
			LogicScore:      0.5, // Reduced score for fallback
			VectorScore:     vScore,
			Combined:        combined,
			SelectionReason: "flesh:fallback_context_match",
			Source:          "flesh",
		})
	}

	// Sort by combined score, ties broken on atom ID so the fallback order is
	// stable run to run.
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Combined != selected[j].Combined {
			return selected[i].Combined > selected[j].Combined
		}
		return selected[i].Atom.ID < selected[j].Atom.ID
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

	// Sort: skeleton categories first, mandatory first, then by combined
	// score — with ties broken on atom ID. Flesh order arrives from kernel
	// query results, and without the tiebreak equal-scored atoms flap run to
	// run: the same prompt compiling to different text for no reason.
	sort.SliceStable(result, func(i, j int) bool {
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
		if result[i].Combined != result[j].Combined {
			return result[i].Combined > result[j].Combined
		}
		return result[i].Atom.ID < result[j].Atom.ID
	})

	logging.Get(logging.CategoryContext).Debug(
		"Merged %d skeleton + %d flesh = %d total atoms",
		len(skeleton), len(flesh), len(result),
	)

	return result
}

// buildContextFacts builds Mangle facts from context and atoms.
// This function allocates thousands of strings per compilation.
func (s *AtomSelector) buildContextFacts(cc *CompilationContext, atoms []*PromptAtom, forcedMandatory map[string]struct{}) ([]any, error) {
	// Generate base context facts using the unified generator
	baseFacts := cc.GenerateFacts(FactStyle{
		Predicate:  "current_context",
		UseShort:   true,
		ForceAtoms: true,
		AddDot:     false,
	})

	// Pre-allocate facts array to minimize reallocation for candidate facts.
	facts := make([]any, 0, len(baseFacts)+len(atoms)*15)
	facts = append(facts, baseFacts...)

	var fb factBuilder
	if cc != nil {
		if shardType := strings.TrimSpace(cc.ShardType); shardType != "" {
			fb.Reset()
			fb.WriteString("compile_shard(")
			fb.WriteQuotedString(cc.ShardID)
			fb.WriteString(", ")
			if fb.writeAtom(shardType) {
				fb.WriteString(")")
				facts = append(facts, fb.String())
			}
		}
	}

	// Candidate Facts
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		id := atom.ID
		isMandatory := atom.IsMandatory
		if forcedMandatory != nil {
			if _, ok := forcedMandatory[id]; ok {
				isMandatory = true
			}
		}

		fb.Reset()
		fb.WriteString("atom(")
		fb.WriteQuotedString(id)
		fb.WriteString(")")
		facts = append(facts, fb.String())

		fb.Reset()
		fb.WriteString("atom_category(")
		fb.WriteQuotedString(id)
		fb.WriteString(", ")
		fb.WriteQuotedString(string(atom.Category))
		fb.WriteString(")")
		facts = append(facts, fb.String())

		fb.Reset()
		fb.WriteString("atom_priority(")
		fb.WriteQuotedString(id)
		fb.WriteString(", ")
		fb.WriteInt(atom.Priority)
		fb.WriteString(")")
		facts = append(facts, fb.String())

		if isMandatory {
			fb.Reset()
			fb.WriteString("is_mandatory(")
			fb.WriteQuotedString(id)
			fb.WriteString(")")
			facts = append(facts, fb.String())
		}

		// GAP-FIX: Emit unified prompt_atom/5 fact required by jit_selection.mg

		// prompt_atom(ID, Category, Priority, Hash, IsMandatory)

		isMandatoryAtom := "/false"
		if isMandatory {
			isMandatoryAtom = "/true"
		}

		// Category must be an atom (e.g. /identity) not a string ('identity')

		category := string(atom.Category)

		if !strings.HasPrefix(category, "/") {

			category = "/" + category

		}

		// Schema (schemas_prompts.mg:194):
		//   Decl prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
		//                    /string  /name    /number   /number    /name
		// Position 3 MUST be a number (TokenCount). Earlier this code put
		// atom.ContentHash there, which is a hex string — every rule that
		// did fn:plus / fn:max over the TokenCount slot blew up with
		// "value 0x... (1) is not a number", killing the kernel's fixpoint
		// evaluation and cascading into ConstitutionGate / ExecutivePolicy
		// errors. ContentHash is not part of the prompt_atom schema; if
		// it needs to flow into the kernel, add a dedicated predicate
		// like prompt_atom_hash(AtomID, Hash) with its own Decl.
		fb.Reset()
		fb.WriteString("prompt_atom(")
		fb.WriteQuotedString(id)
		fb.WriteString(", ")
		fb.WriteString(mangleNormalizeNameConst(category))
		fb.WriteString(", ")
		fb.WriteInt(atom.Priority)
		fb.WriteString(", ")
		fb.WriteInt(atom.TokenCount)
		fb.WriteString(", ")
		fb.WriteString(isMandatoryAtom)
		fb.WriteString(")")
		facts = append(facts, fb.String())

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
				atomDim = mangleNormalizeNameConst(atomDim)
				atomVal = mangleNormalizeNameConst(atomVal)
				if atomDim == "" || atomVal == "" {
					continue
				}
				fb.Reset()
				fb.WriteString("atom_tag(")
				fb.WriteQuotedString(id)
				fb.WriteString(", ")
				fb.WriteString(atomDim)
				fb.WriteString(", ")
				fb.WriteString(atomVal)
				fb.WriteString(")")
				facts = append(facts, fb.String())
			}
		}
		addTags("mode", atom.OperationalModes)
		addTags("phase", atom.CampaignPhases)
		addTags("layer", atom.BuildLayers)
		addTags("init_phase", atom.InitPhases)
		addTags("northstar_phase", atom.NorthstarPhases)
		addTags("ouroboros_stage", atom.OuroborosStages)
		addTags("intent", atom.IntentVerbs)
		// Emit both /shard and /shard_type because policy uses both (jit_selection.mg:196 reads /shard, :253 reads /shard_type); remove duplication once corpus settles on one name.
		addTags("shard", atom.ShardTypes)
		addTags("shard_type", atom.ShardTypes)
		addTags("lang", atom.Languages)
		addTags("framework", atom.Frameworks)
		addTags("state", atom.WorldStates)
		// Pin dimensions. Both are regime dimensions in jit_compiler.mg, so
		// emitting the tag is what arms the fail-closed block: an atom with
		// a /provider or /model tag is admitted only on a compile whose
		// current_context carries a matching one.
		addTags("provider", atom.Providers)
		addTags("model", atom.Models)

		// Dependencies - needed for atom_requires() in jit_compiler.mg
		for _, dep := range atom.DependsOn {
			if dep != "" {
				fb.Reset()
				fb.WriteString("atom_requires(")
				fb.WriteQuotedString(id)
				fb.WriteString(", ")
				fb.WriteQuotedString(dep)
				fb.WriteString(")")
				facts = append(facts, fb.String())
			}
		}

		// Conflicts - needed for atom_conflicts() in jit_compiler.mg
		for _, conflict := range atom.ConflictsWith {
			if conflict != "" {
				fb.Reset()
				fb.WriteString("atom_conflicts(")
				fb.WriteQuotedString(id)
				fb.WriteString(", ")
				fb.WriteQuotedString(conflict)
				fb.WriteString(")")
				facts = append(facts, fb.String())
			}
		}

		// Capability requirements - needed for blocked_by_missing_tool in
		// jit_selection.mg. Emitted as quoted strings (tool names, not Mangle
		// atoms). Never widen authority here; the policy omits the atom when
		// any required tool is absent.
		for _, tool := range atom.RequiresTools {
			if strings.TrimSpace(tool) == "" {
				continue
			}
			fb.Reset()
			fb.WriteString("atom_requires_tool(")
			fb.WriteQuotedString(id)
			fb.WriteString(", ")
			fb.WriteQuotedString(tool)
			fb.WriteString(")")
			facts = append(facts, fb.String())
		}
	}

	// Effective executable catalog for this compile. Empty means fail-closed.
	if cc != nil {
		for _, tool := range cc.AvailableTools {
			if strings.TrimSpace(tool) == "" {
				continue
			}
			fb.Reset()
			fb.WriteString("available_tool(")
			fb.WriteQuotedString(tool)
			fb.WriteString(")")
			facts = append(facts, fb.String())
		}
	}

	return facts, nil
}

// extractStringArg safely extracts a string from a Mangle fact argument.
// Returns an error if the argument is of an unsupported or complex type.
func extractStringArg(arg any) (string, error) {
	if arg == nil {
		return "", nil
	}
	switch v := arg.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case []byte:
		return string(v), nil
	case error:
		return v.Error(), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return "", fmt.Errorf("unsupported type for string extraction: %T", v)
	}
}
