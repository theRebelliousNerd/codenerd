package context

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"slices"
)

// =============================================================================
// Metrics & State
// =============================================================================

// GetMetrics returns compression metrics.
func (c *Compressor) GetMetrics() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ratio := 1.0
	if c.totalCompressedTokens > 0 {
		ratio = float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
	}

	return map[string]any{
		"turn_number":             c.turnNumber,
		"recent_turns":            len(c.recentTurns),
		"compressed_segments":     len(c.rollingSummary.Segments),
		"total_compressed_turns":  c.rollingSummary.TotalTurns,
		"total_masked_turns":      c.rollingSummary.TotalMaskedTurns,
		"total_original_tokens":   c.totalOriginalTokens,
		"total_compressed_tokens": c.totalCompressedTokens,
		"compression_ratio":       ratio,
		"target_ratio":            c.config.TargetCompressionRatio,
		// Kernel/Go dual-path visibility (context TODO P1).
		"kernel_selections":      c.selection.KernelSelections,
		"go_fallbacks":           c.selection.GoFallbacks,
		"kernel_inclusion_rate":  c.selection.KernelInclusionRate(),
		"last_selection_mode":    string(c.selection.LastMode),
		"last_selection_reason":  c.selection.LastReason,
		"unresolved_kernel_ents": c.selection.UnresolvedKernelFacts,
	}
}

// GetCompressionRatio returns the current compression ratio.
func (c *Compressor) GetCompressionRatio() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.totalCompressedTokens == 0 {
		return 1.0
	}
	return float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
}

// GetBudgetUtilization returns the fraction of the configured context budget used.
// Safe for UI display; returns 0 when budget is unavailable.
func (c *Compressor) GetBudgetUtilization() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.budget == nil || c.config.TotalBudget == 0 {
		return 0
	}
	return c.budget.Utilization()
}

// GetBudgetUsage returns (used, total) token counts for the context window.
// This is approximate and based on the internal token counter heuristic.
func (c *Compressor) GetBudgetUsage() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.budget == nil {
		return 0, c.config.TotalBudget
	}
	return c.budget.TotalUsed(), c.config.TotalBudget
}

// RefreshBudget recalculates the token budget based on current state.
//
// LoadState now does this itself, so rehydration cannot ship an unpaired load.
// This stays exported and idempotent for callers that mutate state by other
// means (e.g. a kernel reload behind the compressor's back).
//
// It runs under c.mu. The previous version dropped the lock first, claiming
// deadlock risk — but recalcBudget takes no compressor lock (ProcessTurn
// already calls it while holding c.mu), so all that comment bought was a data
// race on recentTurns/rollingSummary/budget against a concurrent ProcessTurn.
func (c *Compressor) RefreshBudget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recalcBudget(c.turnNumber, 0)
}

// IsCompressionActive returns true if callers should use compressed context
// instead of raw conversation history. This is token-budget driven:
// - Returns false when we have room for raw history (use full context)
// - Returns true when approaching token limit (switch to compressed)
func (c *Compressor) IsCompressionActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// If we have compressed segments, always use compressed context
	if len(c.rollingSummary.Segments) > 0 {
		logging.ContextDebug("Compression active: %d segments exist", len(c.rollingSummary.Segments))
		return true
	}

	// If approaching token limit, signal that we should use compressed context
	// This prevents the "dump 50 messages" problem on rehydrated sessions
	shouldCompress := c.budget.ShouldCompress()
	if shouldCompress {
		logging.ContextDebug("Compression active: budget threshold reached (%.1f%%)", c.budget.Utilization()*100)
	}
	return shouldCompress
}

// recordSelectionLocked updates the kernel-vs-Go selection counters.
// Caller must hold c.mu.
func (c *Compressor) recordSelectionLocked(reason string, kernelFacts, selectedFacts int) {
	if reason == reasonKernelSelected {
		c.selection.KernelSelections++
		c.selection.LastMode = SelectionKernel
	} else {
		c.selection.GoFallbacks++
		c.selection.LastMode = SelectionGoFallback
	}
	if reason == reasonUnresolved {
		c.selection.UnresolvedKernelFacts += kernelFacts
	}
	c.selection.LastReason = reason
	c.selection.LastKernelFacts = kernelFacts
	c.selection.LastSelectedFacts = selectedFacts
}

// GetSelectionStats reports how often the Mangle should_include_context gate
// decided the context block versus the Go activation fallback. Use this to
// track dual-path drift: a kernel inclusion rate that collapses means the C1/C4
// rules stopped resolving against the fact store.
func (c *Compressor) GetSelectionStats() SelectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.selection
}

// GetRecentTurnWindow returns the configured recent turn window size.
func (c *Compressor) GetRecentTurnWindow() int {
	return c.config.RecentTurnWindow
}

// buildStateLocked constructs a CompressedState assuming c.mu is already held.
func (c *Compressor) buildStateLocked() *CompressedState {
	// Get hot facts
	allFacts := slices.Collect(c.kernel.GetAllFactsSeq())
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}
	hotFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	ratio := 1.0
	if c.totalCompressedTokens > 0 {
		ratio = float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
	}

	return &CompressedState{
		SessionID:            c.sessionID,
		Version:              "1.0.0",
		TurnNumber:           c.turnNumber,
		Timestamp:            time.Now(),
		RollingSummary:       c.rollingSummary,
		RecentTurns:          c.recentTurns,
		HotFacts:             hotFacts,
		TotalCompressedTurns: c.rollingSummary.TotalTurns,
		CompressionRatio:     ratio,
	}
}

// GetState returns the full compressed state for persistence.
func (c *Compressor) GetState() *CompressedState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	logging.ContextDebug("Getting compressed state for persistence")
	state := c.buildStateLocked()
	logging.ContextDebug("State: turn=%d, recent=%d, segments=%d, hot_facts=%d",
		state.TurnNumber, len(state.RecentTurns), len(state.RollingSummary.Segments), len(state.HotFacts))
	return state
}

// LoadState restores state from a persisted CompressedState.
func (c *Compressor) LoadState(state *CompressedState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Context("Loading compressed state: session=%s, turn=%d, segments=%d",
		state.SessionID, state.TurnNumber, len(state.RollingSummary.Segments))

	c.sessionID = state.SessionID
	c.turnNumber = state.TurnNumber
	c.rollingSummary = state.RollingSummary
	c.recentTurns = state.RecentTurns

	// Restore hot facts to kernel
	restoredCount := 0
	if c.kernel != nil {
		existing := make(map[string]struct{})
		for f := range c.kernel.GetAllFactsSeq() {
			existing[f.String()] = struct{}{}
		}
		missing := make([]core.Fact, 0, len(state.HotFacts))
		for _, sf := range state.HotFacts {
			key := sf.Fact.String()
			if _, ok := existing[key]; ok {
				continue
			}
			existing[key] = struct{}{}
			missing = append(missing, sf.Fact)
		}
		if len(missing) > 0 {
			// Best-effort batch restore; fallback to per-assert.
			if err := c.kernel.AssertBatch(missing); err != nil {
				for _, f := range missing {
					_ = c.kernel.Assert(f)
				}
			}
			for _, f := range missing {
				c.activation.RecordFactTimestamp(f)
			}
			restoredCount = len(missing)
		}
	}

	// Rehydration must leave the budget describing what was just restored.
	// This used to be the caller's job via RefreshBudget(); every caller that
	// forgot rehydrated a session whose budget read 0/200000, so
	// IsCompressionActive() answered false and the chat layer dumped the whole
	// raw history it had just compressed away. Pairing it here makes the
	// unpaired call impossible rather than merely discouraged.
	c.recalcBudget(c.turnNumber, 0)

	logging.Context("State loaded: restored %d hot facts, %d recent turns, budget %d/%d tokens",
		restoredCount, len(c.recentTurns), c.budget.TotalUsed(), c.config.TotalBudget)
	return nil
}

// Reset clears all compression state.
func (c *Compressor) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Context("Resetting compressor state")

	c.turnNumber = 0
	c.recentTurns = nil
	c.rollingSummary = RollingSummary{}
	c.totalOriginalTokens = 0
	c.totalCompressedTokens = 0
	c.activation.ClearState()
	c.budget.Reset()
	c.sessionID = fmt.Sprintf("session_%d", time.Now().Unix())

	logging.Context("Compressor reset complete, new session: %s", c.sessionID)
}

// countOriginalTokens sums the pre-compression token estimates for turns.
func (c *Compressor) countOriginalTokens(turns []CompressedTurn) int {
	total := 0
	for _, t := range turns {
		if t.OriginalTokens > 0 {
			total += t.OriginalTokens
			continue
		}
		total += c.counter.CountTurn(t)
	}
	return total
}

// collectKeyAtoms extracts a bounded set of high-signal atoms to persist with the summary.
func (c *Compressor) collectKeyAtoms(turns []CompressedTurn, limit int) []core.Fact {
	seen := make(map[string]bool)
	var atoms []core.Fact

	add := func(f core.Fact) {
		if len(atoms) >= limit {
			return
		}
		key := f.String()
		if seen[key] {
			return
		}
		seen[key] = true
		atoms = append(atoms, f)
	}

	for _, turn := range turns {
		if turn.IntentAtom != nil {
			add(*turn.IntentAtom)
		}
		for _, f := range turn.FocusAtoms {
			add(f)
		}
		for i, f := range turn.ResultAtoms {
			if i >= 5 { // keep summaries small; first few results capture state
				break
			}
			add(f)
		}
		for _, f := range turn.ActionAtoms {
			add(f)
		}
	}

	return atoms
}

// trimToTokens truncates a string to fit within the approximate token budget.
func (c *Compressor) trimToTokens(s string, maxTokens int) string {
	if maxTokens <= 0 || c.counter.CountString(s) <= maxTokens {
		return strings.TrimSpace(s)
	}

	runes := []rune(s)
	// Find the largest prefix length whose token count is still within budget.
	// Invariant: CountString(runes[:low]) <= maxTokens (true at low=0). The +1
	// bias on mid guarantees progress and prevents the previous off-by-one that
	// could return a prefix one token over budget.
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		if c.counter.CountString(string(runes[:mid])) <= maxTokens {
			low = mid
		} else {
			high = mid - 1
		}
	}

	cut := max(1, low)
	return strings.TrimSpace(string(runes[:cut]))
}

// LoadPrioritiesFromCorpus loads predicate priorities from the kernel's corpus.
// GAP-003 FIX: This enables activation engine to use corpus-defined priorities.
func (c *Compressor) LoadPrioritiesFromCorpus(corpus *core.PredicateCorpus) error {
	if c.activation == nil {
		return nil
	}
	return c.activation.LoadPrioritiesFromCorpus(corpus)
}

// GetActivationScores returns current activation scores for all facts.
// Used by JIT Prompt Compiler to boost atoms related to highly-activated facts.
// Returns a map of fact string representation → activation score (0.0-1.0).
func (c *Compressor) GetActivationScores() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scores := make(map[string]float64)

	if c.kernel == nil {
		return scores
	}

	// Get all facts and their activation scores
	allFacts := slices.Collect(c.kernel.GetAllFactsSeq())
	if len(allFacts) == 0 {
		return scores
	}

	// Get current intent for context-aware scoring
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}

	// Score all facts using the activation engine
	scoredFacts := c.activation.ScoreFacts(allFacts, currentIntent)
	for _, sf := range scoredFacts {
		// Normalize score to 0.0-1.0 range (scores are typically 0-100)
		normalizedScore := sf.Score / 100.0
		if normalizedScore > 1.0 {
			normalizedScore = 1.0
		}
		scores[sf.Fact.String()] = normalizedScore
	}

	return scores
}

// GetHighActivationFactKeys returns fact keys with activation above threshold.
// Used by JIT compiler to find atoms related to "hot" facts.
func (c *Compressor) GetHighActivationFactKeys(threshold float64) []string {
	var keys []string
	scores := c.GetActivationScores()

	for key, score := range scores {
		if score >= threshold {
			keys = append(keys, key)
		}
	}

	return keys
}

// NERD-EVOLVE-START: context_compilation_pipeline
// buildKernelDerivedContext converts kernel-derived should_include_context facts into
// ScoredFact slices for use by BuildContext().
//
// The kernel query returns should_include_context(Entity, Priority) facts where:
//   - Entity is a string identifier (file path, symbol, intent target, or, rarely,
//     a whole fact string)
//   - Priority is a name atom: /p100, /p95, /p90, /p85, /p80, /p70, /p60
//
// Steps:
//  1. Parses priority atoms: /pN -> N (e.g. "/p100" -> 100.0)
//  2. Resolves each entity to concrete facts (exact fact string, else every
//     fact carrying that entity as a string argument)
//  3. Sorts by priority descending, deduplicating on max priority
//  4. Returns budget-limited ScoredFact slice via SelectWithinBudgetPreFiltered
//
// Entity resolution matters: every C1/C4 rule head binds a file/symbol/target,
// never a serialized fact, and core.Fact.String() carries a trailing "." that a
// bare identifier can never match. Matching only on fact strings therefore
// resolved nothing on every real session, and the caller's "fall back to Go"
// comment was not implemented — so a live session with a user_intent shipped an
// empty ACTIVE CONTEXT block. Resolution by argument makes the kernel's
// decision actually reach the LLM.
//
// Returns nil when no matching facts are found in the kernel's fact store,
// causing the caller to fall back to the Go activation engine.
func (c *Compressor) buildKernelDerivedContext(kernelFacts []core.Fact, allFacts []core.Fact) []ScoredFact {
	if len(kernelFacts) == 0 {
		return nil
	}

	// Two lookups: exact fact string, and entity -> facts mentioning it.
	factLookup := make(map[string]core.Fact, len(allFacts))
	byEntity := make(map[string][]core.Fact, len(allFacts))
	for _, f := range allFacts {
		factLookup[f.String()] = f
		for _, arg := range f.Args {
			s, ok := arg.(string)
			if !ok || s == "" {
				continue
			}
			byEntity[s] = append(byEntity[s], f)
		}
	}

	// parsePriority converts /pN atom to numeric score.
	// "/p100" -> 100.0, "/p95" -> 95.0, etc. Returns 0 on parse failure.
	parsePriority := func(priorityArg any) float64 {
		var s string
		switch v := priorityArg.(type) {
		case string:
			s = v
		default:
			return 0.0
		}
		// Mangle name atom /p100 is stored as the string "/p100"
		if len(s) > 2 && s[0] == '/' && s[1] == 'p' {
			if n, err := strconv.Atoi(s[2:]); err == nil {
				return float64(n)
			}
		}
		return 0.0
	}

	// Collect (fact string, priority score) pairs from kernel results.
	type prioritized struct {
		factStr  string
		priority float64
	}
	pairs := make([]prioritized, 0, len(kernelFacts))
	for _, kf := range kernelFacts {
		// should_include_context has arity 2: (Fact, Priority)
		if len(kf.Args) != 2 {
			continue
		}
		factStr, ok := kf.Args[0].(string)
		if !ok {
			continue
		}
		priority := parsePriority(kf.Args[1])
		if priority <= 0 {
			continue
		}
		pairs = append(pairs, prioritized{factStr: factStr, priority: priority})
	}

	if len(pairs) == 0 {
		return nil
	}

	// Sort by priority descending (highest priority facts first). Ties break on
	// the entity name so the same kernel state always yields the same block —
	// an unstable order here would make the context non-reproducible across
	// otherwise identical turns.
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].priority != pairs[j].priority {
			return pairs[i].priority > pairs[j].priority
		}
		return pairs[i].factStr < pairs[j].factStr
	})

	// Build ScoredFact list for facts that exist in the kernel's fact store.
	// pairs is already sorted by priority descending, so the first time a fact
	// is seen it carries its highest kernel priority.
	scored := make([]ScoredFact, 0, len(pairs))
	seen := make(map[string]struct{}, len(pairs))
	addFact := func(f core.Fact, priority float64) {
		key := f.String()
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		scored = append(scored, ScoredFact{Fact: f, Score: priority})
	}
	for _, p := range pairs {
		if f, found := factLookup[p.factStr]; found {
			addFact(f, p.priority)
			continue
		}
		// Entity form (file path from C4 traversal, intent target, symbol):
		// pull in every fact that names it. Deterministic because allFacts
		// ordering is stable and pairs is sorted.
		for _, f := range byEntity[p.factStr] {
			addFact(f, p.priority)
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// Kernel-derived facts already passed the authoritative should_include_context
	// gate, so select within budget WITHOUT re-applying the activation threshold.
	// The threshold is calibrated for the Go heuristic scale (100–250); kernel
	// priorities top out at 100, so SelectWithinBudget's threshold filter (default
	// 105) would silently prune every kernel fact and ship an empty context.
	return c.activation.SelectWithinBudgetPreFiltered(scored, c.config.AtomReserve)
}

// assertTurnAgeCategories asserts turn_age_category(TurnID, Category) facts into the kernel
// for the observation masking rules in context_compilation.mg to derive should_mask_observation.
//
// Age is computed as (currentTurnNumber - turn.TurnNumber). The previous
// implementation used `len(c.recentTurns) - turn.TurnNumber` which mixed
// a SLICE LENGTH with a MONOTONIC TURN ID. After compression slices
// `recentTurns`, the length drops below the current turn number — every
// fact then fell into the `/recent` bucket because `age` went negative
// and tripped `case age <= 3`. The mask-observation rules were therefore
// effectively dead.
func (c *Compressor) assertTurnAgeCategories(turns []CompressedTurn) {
	if c.kernel == nil {
		return
	}
	currentTurn := c.turnNumber
	for _, turn := range turns {
		turnID := fmt.Sprintf("turn_%d", turn.TurnNumber)
		age := currentTurn - turn.TurnNumber
		var category string
		switch {
		case age <= 3:
			category = "/recent"
		case age <= 8:
			category = "/mid"
		case age <= 15:
			category = "/old"
		default:
			category = "/ancient"
		}
		// core.ParseFactString appends the clause terminator itself, so the
		// trailing "." this used to pass produced "turn_age_category(...)..",
		// which never parsed. The error was discarded (`_ =`), so no
		// turn_age_category fact ever reached the kernel and every C3 masking
		// rule sat on an empty relation: masking was dead in production while
		// looking wired.
		if err := c.kernel.AssertString(fmt.Sprintf("turn_age_category(%q, %s)", turnID, category)); err != nil {
			logging.Get(logging.CategoryContext).Warn("assertTurnAgeCategories: %s -> %s failed: %v", turnID, category, err)
		}
	}
}

// turnMaskID is the kernel-side identifier for a compressed turn. The Go and
// Mangle sides must agree on this exact shape or masking silently no-ops.
func turnMaskID(turnNumber int) string {
	return fmt.Sprintf("turn_%d", turnNumber)
}

// maskedObservationTurns asks the kernel which turns' observations may be
// dropped from the rolling summary. The decision is Mangle's
// (should_mask_observation/1, derived from turn_age_category); Go only obeys.
//
// It also enforces the C3 safety net: any turn the kernel marks for masking
// must also appear in should_preserve_reasoning, otherwise we would be
// dropping observations on a turn whose reasoning chain the kernel never
// promised to keep. A violation means the rules drifted apart, so we refuse to
// mask that turn rather than lose the turn entirely.
func (c *Compressor) maskedObservationTurns() map[string]bool {
	if c.kernel == nil {
		return nil
	}

	maskFacts, err := c.kernel.Query("should_mask_observation")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("maskedObservationTurns: should_mask_observation query failed: %v", err)
		return nil
	}
	if len(maskFacts) == 0 {
		return nil
	}

	preserve := make(map[string]bool)
	preserveFacts, err := c.kernel.Query("should_preserve_reasoning")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("maskedObservationTurns: should_preserve_reasoning query failed: %v", err)
		return nil
	}
	for _, f := range preserveFacts {
		if len(f.Args) < 1 {
			continue
		}
		if id, ok := f.Args[0].(string); ok {
			preserve[id] = true
		}
	}

	masked := make(map[string]bool, len(maskFacts))
	for _, f := range maskFacts {
		if len(f.Args) < 1 {
			continue
		}
		id, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		if !preserve[id] {
			logging.Get(logging.CategoryContext).Warn(
				"maskedObservationTurns: %s marked for masking but not for reasoning preservation; refusing to mask", id)
			continue
		}
		masked[id] = true
	}

	logging.ContextDebug("C3 masking: kernel marked %d/%d turns for observation masking", len(masked), len(maskFacts))
	return masked
}

// NERD-EVOLVE-END: context_compilation_pipeline
