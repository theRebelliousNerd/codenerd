package prompt

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/transparency"
)

// CompilationStats provides comprehensive metrics for a single compilation.
// This is the "god-tier" observability structure for JIT prompt compilation.
type CompilationStats struct {
	// --- Timing Metrics ---
	// Total compilation duration from start to finish
	Duration time.Duration

	// Phase-level timing breakdown (all in milliseconds for log parsing)
	CollectAtomsMs int64 // Time to collect atoms from all sources
	SelectAtomsMs  int64 // Time for Mangle selection + vector search
	ResolveDepsMs  int64 // Time for dependency resolution
	FitBudgetMs    int64 // Time for budget fitting with polymorphism
	AssembleMs     int64 // Time to assemble final prompt text
	VectorQueryMs  int64 // Time spent specifically on vector search (subset of SelectAtomsMs)

	// --- Atom Counts ---
	// AtomsSelected is the number of atoms in the final compiled prompt
	AtomsSelected int

	// SkeletonAtoms is the count of mandatory/skeleton atoms (always included)
	SkeletonAtoms int

	// FleshAtoms is the count of probabilistic/flesh atoms (context-dependent)
	FleshAtoms int

	// AtomsCandidates is total atoms considered before filtering
	AtomsCandidates int

	// AtomsDropped is atoms rejected by selector or budget
	AtomsDropped int

	// --- Token Metrics ---
	// TokensUsed is the total tokens in the compiled prompt
	TokensUsed int

	// TokenBudget is the target budget for this compilation
	TokenBudget int

	// BudgetUtilization is TokensUsed/TokenBudget as percentage (0.0-1.0)
	BudgetUtilization float64

	// SkeletonTokens is tokens used by mandatory atoms
	SkeletonTokens int

	// FleshTokens is tokens used by optional atoms
	FleshTokens int

	// --- Source Breakdown ---
	// EmbeddedAtoms is count of atoms from embedded corpus
	EmbeddedAtoms int

	// ProjectAtoms is count of atoms from project database
	ProjectAtoms int

	// ShardAtoms is count of atoms from shard-specific database
	ShardAtoms int

	// EvolvedAtoms is count of atoms from prompt evolution (SPL system)
	EvolvedAtoms int

	// StrategyAtoms is count of atoms from the learned-strategy provider.
	StrategyAtoms int

	// --- Render Mode Distribution ---
	// StandardModeCount is atoms rendered in full/standard mode
	StandardModeCount int

	// ConciseModeCount is atoms rendered in concise mode
	ConciseModeCount int

	// MinModeCount is atoms rendered in minimal mode
	MinModeCount int

	// --- Cache & Fallback ---
	// FallbackUsed indicates whether legacy fallback compilation was used.
	//
	// Dark by MISSING SOURCE rather than missing wire, and the tests say so out
	// loud: TestCompiler_Fallback* builds the two situations that would set it
	// -- a nil kernel and a kernel query error -- finds the compiler returns an
	// error in both, and skips with "Fallback for nil kernel not yet
	// implemented - test documents expected behavior". There is no fallback
	// compile path to report on.
	//
	// So the "fallback_used" key in Map() below always reads false, which is
	// true for the reason that nothing can make it true. Wiring this means
	// BUILDING the fallback -- deciding what a compile does when the kernel is
	// unavailable -- which is the same open question internal/session's 52-
	// character fallback prompt raises, and it is a feature, not a wire.
	FallbackUsed bool

	// CacheHit indicates whether a cached skeleton was used
	CacheHit bool

	// CacheKey is the cache key used (for debugging)
	CacheKey string

	// --- Context Info (for correlation) ---
	// ShardID is the shard this compilation was for
	ShardID string

	// OperationalMode is the mode during compilation
	OperationalMode string

	// IntentVerb is the user intent that triggered compilation
	IntentVerb string
}

// String returns a human-readable summary of the compilation stats.
func (s *CompilationStats) String() string {
	var util float64
	if s.TokenBudget > 0 {
		util = s.BudgetUtilization * 100
	}
	return fmt.Sprintf(
		"JIT[%s] %dms | atoms=%d (skel=%d flesh=%d) | tokens=%d/%d (%.1f%%) | vector=%dms | cache=%v",
		s.ShardID,
		s.Duration.Milliseconds(),
		s.AtomsSelected,
		s.SkeletonAtoms,
		s.FleshAtoms,
		s.TokensUsed,
		s.TokenBudget,
		util,
		s.VectorQueryMs,
		s.CacheHit,
	)
}

// ToLogFields returns the stats as a map for structured logging.
func (s *CompilationStats) ToLogFields() map[string]any {
	return map[string]any{
		"duration_ms":      s.Duration.Milliseconds(),
		"collect_ms":       s.CollectAtomsMs,
		"select_ms":        s.SelectAtomsMs,
		"resolve_ms":       s.ResolveDepsMs,
		"fit_ms":           s.FitBudgetMs,
		"assemble_ms":      s.AssembleMs,
		"vector_ms":        s.VectorQueryMs,
		"atoms_selected":   s.AtomsSelected,
		"skeleton_atoms":   s.SkeletonAtoms,
		"flesh_atoms":      s.FleshAtoms,
		"atoms_candidates": s.AtomsCandidates,
		"atoms_dropped":    s.AtomsDropped,
		"tokens_used":      s.TokensUsed,
		"token_budget":     s.TokenBudget,
		"budget_util":      s.BudgetUtilization,
		"skeleton_tokens":  s.SkeletonTokens,
		"flesh_tokens":     s.FleshTokens,
		"embedded_atoms":   s.EmbeddedAtoms,
		"project_atoms":    s.ProjectAtoms,
		"shard_atoms":      s.ShardAtoms,
		"standard_mode":    s.StandardModeCount,
		"concise_mode":     s.ConciseModeCount,
		"min_mode":         s.MinModeCount,
		"fallback_used":    s.FallbackUsed,
		"cache_hit":        s.CacheHit,
		"shard_id":         s.ShardID,
		"operational_mode": s.OperationalMode,
		"intent_verb":      s.IntentVerb,
	}
}

// buildResultWithStats constructs the compilation result with comprehensive stats.
func (c *JITPromptCompiler) buildResultWithStats(
	candidates []*PromptAtom,
	scored []*ScoredAtom,
	fitted []*OrderedAtom,
	prompt string,
	budget int,
	stats *CompilationStats,
) *CompilationResult {
	result := &CompilationResult{
		Prompt:          prompt,
		IncludedAtoms:   make([]*PromptAtom, 0, len(fitted)),
		CategoryTokens:  make(map[AtomCategory]int),
		AtomsCandidates: len(candidates),
		AtomsSelected:   len(scored),
		AtomsIncluded:   len(fitted),
	}

	// Count atoms and tokens by type
	var skeletonTokens, fleshTokens int
	var standardCount, conciseCount, minCount int

	for _, oa := range fitted {
		if oa == nil || oa.Atom == nil {
			continue
		}
		result.IncludedAtoms = append(result.IncludedAtoms, oa.Atom)
		// Count the render-mode variant actually emitted (see contentForMode),
		// not the standard TokenCount — otherwise degraded (concise/min) atoms
		// over-report their token usage in the result stats.
		atomTokens := tokenCountForMode(oa.Atom, oa.RenderMode)
		result.TotalTokens += atomTokens
		result.CategoryTokens[oa.Atom.Category] += atomTokens

		// Skeleton vs flesh is a CATEGORY distinction (identity, protocol,
		// safety, methodology are skeleton — see isSkeletonCategory), not a
		// mandatory-flag distinction.
		//
		// These counters classified on oa.Atom.IsMandatory and were reported as
		// "skel=" / "flesh=" in the JIT summary line and as
		// skeleton_atoms/flesh_atoms in the compilation_complete event. Because
		// flesh-category atoms can also be mandatory, the summary printed
		// "atoms=23 (skel=23 flesh=0)" on a compilation whose own selector had
		// just logged "Merged 13 skeleton + 10 flesh = 23 total atoms".
		//
		// flesh=0 reads as "the probabilistic half of the architecture selected
		// nothing", which is a wrong and expensive conclusion to hand a reader
		// — it was drawn from this very line while diagnosing F-JIT-4. The two
		// logs now measure the same thing and must agree.
		if isSkeletonCategory(oa.Atom.Category) {
			result.MandatoryCount++
			skeletonTokens += atomTokens
		} else {
			result.OptionalCount++
			fleshTokens += atomTokens
		}

		// Track render mode distribution
		switch oa.RenderMode {
		case "concise":
			conciseCount++
		case "min":
			minCount++
		default:
			standardCount++
		}
	}

	if budget > 0 {
		result.BudgetUsed = float64(result.TotalTokens) / float64(budget)
	} else if budget <= 0 && result.TotalTokens > 0 {
		result.BudgetUsed = 1.0 // Over budget when budget is <= 0 and we have tokens
	} else {
		result.BudgetUsed = 0.0
	}

	result.CompilationTimeMs = stats.Duration.Milliseconds()

	// Update stats with computed values
	stats.AtomsSelected = len(fitted)
	stats.SkeletonAtoms = result.MandatoryCount
	stats.FleshAtoms = result.OptionalCount
	stats.AtomsDropped = len(candidates) - len(fitted)
	stats.TokensUsed = result.TotalTokens
	stats.BudgetUtilization = result.BudgetUsed
	stats.SkeletonTokens = skeletonTokens
	stats.FleshTokens = fleshTokens
	stats.StandardModeCount = standardCount
	stats.ConciseModeCount = conciseCount
	stats.MinModeCount = minCount

	// Build Manifest
	manifest := c.buildManifest(candidates, scored, fitted, budget)
	result.Manifest = manifest
	result.Stats = stats

	return result
}

// buildManifest constructs the prompt manifest for flight recording.
func (c *JITPromptCompiler) buildManifest(
	candidates []*PromptAtom,
	scored []*ScoredAtom,
	fitted []*OrderedAtom,
	budget int,
) *PromptManifest {
	manifest := &PromptManifest{
		Timestamp:   time.Now(),
		BudgetLimit: budget,
		Selected:    make([]AtomManifestEntry, 0, len(fitted)),
		Dropped:     make([]DroppedAtomEntry, 0),
	}

	// Calculate total tokens using the render-mode variant actually emitted, so
	// the manifest matches contentForMode's output rather than over-reporting
	// the standard size of degraded (concise/min) atoms.
	for _, oa := range fitted {
		if oa == nil || oa.Atom == nil {
			continue
		}
		manifest.TokenUsage += tokenCountForMode(oa.Atom, oa.RenderMode)
	}

	// Lookup map for scores
	scoreMap := make(map[string]*ScoredAtom)
	for _, sa := range scored {
		if sa == nil || sa.Atom == nil {
			continue
		}
		scoreMap[sa.Atom.ID] = sa
	}

	// Lookup map for fitted
	fittedMap := make(map[string]bool)
	for _, oa := range fitted {
		if oa == nil || oa.Atom == nil {
			continue
		}
		fittedMap[oa.Atom.ID] = true

		// Find scores
		var logic, vector float64
		source := "flesh"
		if sa, ok := scoreMap[oa.Atom.ID]; ok {
			logic = sa.LogicScore
			vector = sa.VectorScore
			source = sa.Source
		}

		if oa.Atom.IsMandatory {
			source = "skeleton"
		}

		manifest.Selected = append(manifest.Selected, AtomManifestEntry{
			ID:          oa.Atom.ID,
			Category:    string(oa.Atom.Category),
			Source:      source,
			Priority:    oa.Atom.Priority,
			LogicScore:  logic,
			VectorScore: vector,
			RenderMode:  oa.RenderMode,
		})
	}

	// Identify dropped atoms
	// 1. Candidates rejected by Selector (Logic/Vector)
	for _, cand := range candidates {
		if cand == nil {
			continue
		}
		if _, ok := scoreMap[cand.ID]; !ok {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     cand.ID,
				Reason: "Selector (Logic/Vector)",
			})
		}
	}

	// 2. Scored atoms rejected by Budget/Resolver
	for _, sa := range scored {
		if sa == nil || sa.Atom == nil {
			continue
		}
		if !fittedMap[sa.Atom.ID] {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     sa.Atom.ID,
				Reason: "Budget/Dependency",
			})
		}
	}

	return manifest
}

// emitJITGlassBox publishes atom-selection telemetry to the Glass Box stream
// when the operator has JIT explain on.
//
// The compiler is constructed at boot before the Glass Box bus exists and
// takes no bus setter, so this goes through the process-wide transparency
// facade rather than an injected handle. It is a no-op (one atomic load) when
// JIT explain is off, which is the default.
func emitJITGlassBox(stats *CompilationStats, result *CompilationResult) {
	if stats == nil || !transparency.JITExplainEnabled() {
		return
	}

	source := stats.ShardID
	if source == "" {
		source = "jit"
	}
	summary := fmt.Sprintf("%d atoms · %d/%d tokens", stats.AtomsSelected, stats.TokensUsed, stats.TokenBudget)
	if stats.CacheHit {
		summary += " (cache hit)"
	}

	var details strings.Builder
	fmt.Fprintf(&details, "skeleton=%d flesh=%d candidates=%d dropped=%d\n",
		stats.SkeletonAtoms, stats.FleshAtoms, stats.AtomsCandidates, stats.AtomsDropped)
	fmt.Fprintf(&details, "sources: embedded=%d project=%d shard=%d evolved=%d strategy=%d\n",
		stats.EmbeddedAtoms, stats.ProjectAtoms, stats.ShardAtoms, stats.EvolvedAtoms,
		stats.StrategyAtoms)
	if stats.IntentVerb != "" {
		fmt.Fprintf(&details, "intent=%s\n", stats.IntentVerb)
	}
	if result != nil && result.Manifest != nil {
		ids := make([]string, 0, len(result.Manifest.Selected))
		for _, entry := range result.Manifest.Selected {
			ids = append(ids, entry.ID)
		}
		if len(ids) > 12 {
			ids = append(ids[:12], "…")
		}
		fmt.Fprintf(&details, "selected: %s", strings.Join(ids, ", "))
	}

	transparency.EmitJIT(summary, details.String(), source, stats.Duration)
}

// logCompilationStats logs comprehensive stats at the end of compilation.
func (c *JITPromptCompiler) logCompilationStats(stats *CompilationStats, result *CompilationResult) {
	logger := logging.Get(logging.CategoryJIT)

	emitJITGlassBox(stats, result)

	// Budget breach detection: budgetMgr.Fit accounts only per-atom render-mode
	// tokens. The final assembled prompt picks up extra tokens from headers,
	// category markers, and separators that Fit doesn't see. When that drift
	// pushes TokensUsed above TokenBudget, the prior code shipped the
	// over-budget prompt silently (logged at INFO) and the JIT cache would
	// re-serve it for every cache HIT on the same context, contaminating the
	// LLM with truncation-prone payloads. Surface it loudly so callers (and
	// log audits) can see when the budget contract is violated.
	// Post-enforcement this should be unreachable: Compile step 5b sheds
	// optional atoms and, failing that, truncates. It is kept as the assertion
	// that enforcement worked — if it ever fires again, enforceAssembledBudget
	// has a hole in it, and the log line is the only place that will say so.
	if stats.TokenBudget > 0 && stats.TokensUsed > stats.TokenBudget {
		overshoot := stats.TokensUsed - stats.TokenBudget
		logger.Warn(
			"JIT[%s] budget breach SURVIVED ENFORCEMENT: assembled %d tokens vs budget %d (+%d, %.1f%%) — enforceAssembledBudget failed to shed or truncate; this is a bug in step 5b, not a headroom problem",
			stats.ShardID, stats.TokensUsed, stats.TokenBudget, overshoot, stats.BudgetUtilization*100,
		)
	}

	// Summary line (INFO level)
	logger.Info("%s", stats.String())

	// Detailed breakdown (DEBUG level)
	logger.Debug(
		"Phase timing: collect=%dms select=%dms resolve=%dms fit=%dms assemble=%dms",
		stats.CollectAtomsMs, stats.SelectAtomsMs, stats.ResolveDepsMs,
		stats.FitBudgetMs, stats.AssembleMs,
	)

	logger.Debug(
		"Source breakdown: embedded=%d project=%d shard=%d evolved=%d strategy=%d",
		stats.EmbeddedAtoms, stats.ProjectAtoms, stats.ShardAtoms, stats.EvolvedAtoms,
		stats.StrategyAtoms,
	)

	logger.Debug(
		"Render modes: standard=%d concise=%d min=%d",
		stats.StandardModeCount, stats.ConciseModeCount, stats.MinModeCount,
	)

	logger.Debug(
		"Token distribution: skeleton=%d flesh=%d total=%d/%d (%.1f%%)",
		stats.SkeletonTokens, stats.FleshTokens,
		stats.TokensUsed, stats.TokenBudget, stats.BudgetUtilization*100,
	)

	// Category breakdown (DEBUG level)
	if len(result.CategoryTokens) > 0 {
		for cat, tokens := range result.CategoryTokens {
			logger.Debug("Category %s: %d tokens", cat, tokens)
		}
	}

	// Structured log for machine parsing (if JSON mode enabled)
	logger.StructuredLog("info", "compilation_complete", stats.ToLogFields())
}

// logCompilationManifest logs the prompt manifest (selected/dropped atom IDs) for debugging.
func (c *JITPromptCompiler) logCompilationManifest(stats *CompilationStats, result *CompilationResult) {
	if result == nil || result.Manifest == nil {
		return
	}

	manifest := result.Manifest
	selectedIDs := make([]string, 0, len(manifest.Selected))
	for _, entry := range manifest.Selected {
		selectedIDs = append(selectedIDs, entry.ID)
	}

	droppedIDs := make([]string, 0, len(manifest.Dropped))
	for _, entry := range manifest.Dropped {
		droppedIDs = append(droppedIDs, entry.ID)
	}

	fields := map[string]any{
		"context_hash":   manifest.ContextHash,
		"shard_id":       stats.ShardID,
		"intent_verb":    stats.IntentVerb,
		"selected_ids":   selectedIDs,
		"selected":       manifest.Selected,
		"selected_count": len(manifest.Selected),
		"dropped_ids":    droppedIDs,
		"dropped":        manifest.Dropped,
		"dropped_count":  len(manifest.Dropped),
	}

	// Use info level so manifests persist even when log level is set to info.
	logging.Get(logging.CategoryJIT).StructuredLog("info", "prompt_manifest", fields)
}

// GetLastResult returns a caller-owned copy of the most recent compilation
// result. The TUI Prompt Inspector and other observers cannot mutate cache or
// last-result state through the returned pointer.
func (c *JITPromptCompiler) GetLastResult() *CompilationResult {
	return cloneCompilationResult(c.lastResult.Load())
}

// CompilerStats is what `nerd jit` prints.
//
// Three of these five were filled by nothing. GetStats built the struct with
// ShardDBCount and EmbeddedAtomCount and returned it, so the command printed
// "Project Atoms: 0 / Compilations: 0 / Avg Time (ms): 0.00" on a machine that
// had been compiling prompts all day — and statsCompilations, which the
// explanatory lines of the same command read, returned zero with it. Zeros are
// the worst possible failure for a stats command: they read as a system that
// has not run rather than as a command that is not looking.
type CompilerStats struct {
	EmbeddedAtomCount int
	ProjectAtomCount  int
	ShardDBCount      int

	// TotalCompilations counts compiles that actually RAN. A cache hit is not
	// a compilation, it is an avoided one, and singleflight coalescing means
	// two concurrent callers of the same context produce one. This is the
	// denominator AverageTimeMs uses, so the two numbers can be read together.
	TotalCompilations int64

	// AverageTimeMs is the mean wall time of those compiles.
	AverageTimeMs float64
}

// GetStats returns current compiler statistics.
func (c *JITPromptCompiler) GetStats() CompilerStats {
	c.shardMu.RLock()
	shardCount := len(c.shardDBs)
	c.shardMu.RUnlock()

	compiles := c.totalCompilations.Load()
	stats := CompilerStats{
		ShardDBCount:      shardCount,
		TotalCompilations: compiles,
	}
	if compiles > 0 {
		nanos := atomic.LoadInt64(&c.compileNanos)
		stats.AverageTimeMs = float64(nanos) / float64(compiles) / float64(time.Millisecond)
	}

	c.dbMu.RLock()
	if c.embeddedCorpus != nil {
		stats.EmbeddedAtomCount = c.embeddedCorpus.Count()
	}
	projectDB := c.projectDB
	c.dbMu.RUnlock()

	// The project corpus is a database rather than a slice, so its size is a
	// query. A COUNT(*) on a status getter is cheap and always current, unlike
	// a cached value that reconciles and marathon runs would silently stale.
	// A failure here is reported as zero and not as an error: this is a stats
	// call, and a closed or missing .nerd/prompts/corpus.db is the ordinary
	// state of a workspace nobody has run `nerd init` in.
	if projectDB != nil {
		var count int
		if err := projectDB.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&count); err == nil {
			stats.ProjectAtomCount = count
		} else {
			logging.Get(logging.CategoryJIT).Debug("GetStats: counting project atoms: %v", err)
		}
	}

	return stats
}
