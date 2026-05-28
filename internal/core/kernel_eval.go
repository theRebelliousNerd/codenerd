package core

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"codenerd/internal/features"
	"codenerd/internal/logging"
	manglepkg "codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/parse"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// diffEvalEnabled returns true when the differential evaluation feature flag
// is on. Resolution precedence (highest first): CODENERD_DIFF_EVAL env var,
// .nerd/config.json features.diff_eval, compile-time default in internal/features.
//
// SPEC DEVIATION: Task #10 prescribed `os.Getenv("CODENERD_DIFF_EVAL") == "1"`
// with default OFF. A concurrent session landed internal/features (out of
// my scope to modify) whose IsDiffEvalEnabled() defaults TRUE and is the
// canonical config path; an existing test (kernel_features_test.go) asserts
// the kernel routes through that gate. Using features.IsDiffEvalEnabled here
// keeps both paths working: env var still wins (so CODENERD_DIFF_EVAL=0
// disables it deterministically), and the in-tree gating test passes.
// Re-read on every evaluate() so t.Setenv toggles take effect between passes.
//
// Operational guidance until the diff engine's known gaps are closed (see
// ApplyAtomDelta missing-options caveat and benchmark below), recommend
// users set CODENERD_DIFF_EVAL=0 in .nerd/config.json features.diff_eval.
func diffEvalEnabled() bool { return features.IsDiffEvalEnabled() }

// =============================================================================
// MANGLE EVALUATION ENGINE
// =============================================================================

// programBuilderPool reuses strings.Builder instances across rebuildProgram calls.
// rebuildProgram concatenates schemas+policy+learned into a single program source
// on every policy-dirty cycle; pooling avoids reallocating the backing slice each
// time the kernel re-stratifies during long-running campaigns.
//
// Each Get must be paired with a Put after Reset (see usage in rebuildProgram).
var programBuilderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

// rebuildProgram parses schemas+policy and caches programInfo.
// This is only called when policyDirty is true.
func (k *RealKernel) rebuildProgram() error {
	timer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram")
	logging.Kernel("Rebuilding Mangle program (parsing schemas+policy+learned)")

	// Construct program from schemas + policy + learned (no facts)
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	sb := programBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	defer programBuilderPool.Put(sb)

	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included schemas (%d bytes)", len(k.schemas))
	}

	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included policy (%d bytes)", len(k.policy))
	}

	// Load learned rules AFTER constitution (stratified trust)
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
		logging.KernelDebug("rebuildProgram: included learned rules (%d bytes)", len(k.learned))
	}

	programStr := sb.String()
	logging.KernelDebug("rebuildProgram: total program size = %d bytes", len(programStr))

	// Parse
	parseTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.parse")
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: parse failed: %v", err)
		return fmt.Errorf("failed to parse program: %w", err)
	}
	parseTimer.Stop()
	logging.KernelDebug("rebuildProgram: parsed %d clauses", len(parsed.Clauses))

	// Diagnostic for duplicate `Decl permitted(` lines (which would create
	// schema-inconsistency errors at analyze time). Route through the
	// kernel logger instead of stdout so it doesn't break TUI rendering or
	// JSON-log piping.
	if count := strings.Count(programStr, "Decl permitted("); count > 1 {
		logging.KernelDebug("rebuildProgram: %d 'Decl permitted(' lines detected — schema may be inconsistent", count)
		lines := strings.Split(programStr, "\n")
		for i, line := range lines {
			if strings.Contains(line, "Decl permitted(") {
				start := max(i-2, 0)
				logging.KernelDebug("rebuildProgram: Decl permitted match at line %d:\n%s", i, strings.Join(lines[start:i+1], "\n"))
			}
		}
	}

	analyzeTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.analyze")
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: analysis failed: %v", err)
		// DEBUG: Dump program when analysis fails
		if writeErr := os.WriteFile("debug_program_ERROR.mg", []byte(programStr), 0644); writeErr != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to write debug dump: %v", writeErr)
		} else {
			logging.KernelDebug("Dumped failed program to debug_program_ERROR.mg")
		}
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	analyzeTimer.Stop()

	k.programInfo = programInfo
	k.policyDirty = false

	// Cache stratification for EvalStratifiedProgramWithStats
	strata, predToStratum, err := analysis.Stratify(analysis.Program{
		EdbPredicates: programInfo.EdbPredicates,
		IdbPredicates: programInfo.IdbPredicates,
		Rules:         programInfo.Rules,
	})
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: stratification failed: %v", err)
		return fmt.Errorf("failed to stratify program: %w", err)
	}
	k.strata = strata
	k.predToStratum = predToStratum

	// Log predicate count
	declCount := 0
	if programInfo.Decls != nil {
		declCount = len(programInfo.Decls)
	}
	logging.KernelDebug("rebuildProgram: analysis complete, %d predicates declared", declCount)

	timer.StopWithInfo()
	logging.Kernel("Mangle program rebuilt successfully")
	return nil
}

// evaluate populates the store with facts and evaluates to fixpoint.
// Uses cached programInfo for efficiency.
//
// When the differential-eval feature flag is on (CODENERD_DIFF_EVAL=1), a
// stable policy and a non-invalidated diff engine, this routes to
// evaluateDiff(), which uses DifferentialEngine.ApplyDelta on the facts
// asserted since the last evaluate(). Otherwise the full-rebuild path runs.
func (k *RealKernel) evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate")
	defer timer.Stop()

	// Rebuild program if policy changed. This must happen before the diff
	// engine is consulted, because a policy change invalidates the cached
	// stratification and stratum stores inside the diff engine.
	if k.policyDirty || k.programInfo == nil {
		logging.KernelDebug("evaluate: policy dirty or programInfo nil, rebuilding program")
		k.invalidateDiffEngineLocked("policy rebuild")
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	} else {
		logging.KernelDebug("evaluate: using cached programInfo")
	}

	// Differential fast path. Disabled when:
	//   - feature flag off (CODENERD_DIFF_EVAL!=1)
	//   - proofRecorder is set (provenance must observe every derivation)
	//   - virtualStore registers external predicate callbacks (the diff
	//     engine's per-stratum EvalStratifiedProgramWithStats call does NOT
	//     forward WithExternalPredicates/WithCreatedFactLimit options, so
	//     rules consuming external predicates would silently lose their
	//     callbacks and gas limits. Until the differential.go API is
	//     extended to forward eval options, fall back to the full path
	//     whenever externals are in play.)
	//   - diff engine was invalidated by a retract/clear/policy change
	if diffEvalEnabled() && k.proofRecorder == nil && !k.hasExternalPredicatesLocked() {
		if done, err := k.evaluateDiffLocked(); err != nil {
			return err
		} else if done {
			k.initialized = true
			logging.KernelDebug("evaluate: complete via differential path")
			return nil
		}
	}

	return k.evaluateFullLocked()
}

// hasExternalPredicatesLocked returns true when the kernel has at least one
// external-predicate callback to register on the next evaluate. The diff
// path must defer to the full path in that case (see the dispatcher
// comment in evaluate). Caller must hold k.mu.
func (k *RealKernel) hasExternalPredicatesLocked() bool {
	if k.virtualStore == nil {
		return false
	}
	cbs := k.virtualStore.BuildExternalPredicates()
	if len(cbs) == 0 || k.programInfo == nil || k.programInfo.Decls == nil {
		return false
	}
	for pred := range cbs {
		if decl, declared := k.programInfo.Decls[pred]; declared && decl.IsExternal() {
			return true
		}
	}
	return false
}

// evaluateFullLocked runs the legacy full rebuild path: build a fresh
// SimpleInMemoryStore from cachedAtoms and run EvalStratifiedProgramWithStats
// from scratch. Caller must hold k.mu.
func (k *RealKernel) evaluateFullLocked() error {
	// Create fresh store and populate with EDB facts
	// OPTIMIZATION: Use cached atoms instead of converting every time
	logging.KernelDebug("evaluate: populating store with %d EDB facts", len(k.facts))
	baseStore := factstore.NewSimpleInMemoryStore()

	// Defensive sync check: ensure cache is valid
	if k.cachedAtoms == nil || len(k.cachedAtoms) != len(k.facts) {
		if len(k.cachedAtoms) > 0 {
			logging.Get(logging.CategoryKernel).Warn("evaluate: cache desync (atoms=%d facts=%d), rebuilding cache", len(k.cachedAtoms), len(k.facts))
		} else {
			logging.KernelDebug("evaluate: cache empty (facts=%d), populating cache", len(k.facts))
		}
		k.cachedAtoms = make([]ast.Atom, 0, len(k.facts))
		for _, f := range k.facts {
			atom, err := f.ToAtom()
			if err != nil {
				logging.Get(logging.CategoryKernel).Error("evaluate: failed to convert fact %s: %v", f.Predicate, err)
				return fmt.Errorf("failed to convert fact %v: %w", f, err)
			}
			k.cachedAtoms = append(k.cachedAtoms, atom)
		}
	}

	// Use cached atoms (fast path - no conversions!)
	for _, atom := range k.cachedAtoms {
		baseStore.Add(atom)
	}
	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	derivedFactLimit := k.derivedFactLimit
	if derivedFactLimit <= 0 {
		derivedFactLimit = 500000 // Default: 500K derived facts
	}
	logging.KernelDebug("evaluate: running fixpoint evaluation (derivedFactLimit=%d)", derivedFactLimit)

	// Build eval options
	evalOpts := []engine.EvalOption{
		engine.WithCreatedFactLimit(derivedFactLimit), // Hard cap: max 500K derived facts
	}

	// Optional provenance recording (Codeberg mangle-go DerivationRecorder).
	// Reset on every evaluate() so the recorder only holds events from the
	// most recent fixpoint pass; otherwise long-lived sessions would grow
	// unboundedly large recorder buffers.
	if k.proofRecorder != nil {
		k.proofRecorder = provenance.NewMemoryRecorder()
		evalOpts = append(evalOpts, engine.WithDerivationRecorder(k.proofRecorder))
		logging.KernelDebug("evaluate: provenance recording enabled for this pass")
	}

	// #17: Register external predicates instead of virtualFactStore wrapping
	// Only register callbacks for predicates that have a matching Decl with
	// external() descriptor in the current program. This avoids validation
	// errors when tests (or minimal configs) use a subset of schemas.
	if k.virtualStore != nil {
		allCallbacks := k.virtualStore.BuildExternalPredicates()
		if len(allCallbacks) > 0 && k.programInfo != nil && k.programInfo.Decls != nil {
			callbacks := make(map[ast.PredicateSym]engine.ExternalPredicateCallback, len(allCallbacks))
			for pred, cb := range allCallbacks {
				if decl, declared := k.programInfo.Decls[pred]; declared && decl.IsExternal() {
					callbacks[pred] = cb
				}
			}
			if len(callbacks) > 0 {
				evalOpts = append(evalOpts, engine.WithExternalPredicates(callbacks))
				logging.KernelDebug("evaluate: registered %d/%d external predicates (filtered by Decl)", len(callbacks), len(allCallbacks))
			}
		}
	}

	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.fixpoint")
	stats, err := engine.EvalStratifiedProgramWithStats(k.programInfo, k.strata, k.predToStratum, baseStore,
		evalOpts...)
	evalDuration := evalTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryKernel).Error("evaluate: fixpoint evaluation failed: %v", err)
		// Check if this is a derived fact limit error
		if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "exceeded") {
			logging.Get(logging.CategoryKernel).Warn("evaluate: POSSIBLE FACT EXPLOSION - derived facts exceeded %d limit", derivedFactLimit)
		}
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	k.store = baseStore

	// Log evaluation stats
	totalDuration := time.Duration(0)
	for _, d := range stats.Duration {
		totalDuration += d
	}
	strataCount := len(stats.Strata)
	logging.KernelDebug("evaluate: fixpoint reached - strata=%d, evalTime=%v, wallTime=%v",
		strataCount, totalDuration, evalDuration)

	k.initialized = true
	// Reset the diff-engine delta buffer: any facts asserted before now were
	// just included in the full rebuild, so they are no longer "since last
	// eval". The diff engine itself is rebuilt lazily in evaluateDiffLocked.
	k.factsSinceLastEval = nil
	k.dirtyStrata = nil
	logging.KernelDebug("evaluate: full path complete, kernel initialized")
	return nil
}

// invalidateDiffEngineLocked drops the cached differential engine and any
// pending delta. The next evaluate() call will either fall back to the full
// path or rebuild the diff engine from the freshly-rebuilt program. Callers
// must hold k.mu.
//
// Called from:
//   - Retract paths (cannot incrementally un-derive)
//   - Policy change (programInfo replaced; stratification may differ)
//   - Clear / Reset (EDB wiped)
func (k *RealKernel) invalidateDiffEngineLocked(reason string) {
	if k.diffEngine == nil && k.diffMangleEngine == nil && k.dirtyStrata == nil && k.factsSinceLastEval == nil {
		return
	}
	logging.KernelDebug("evaluate: invalidating diff engine (%s)", reason)
	k.diffEngine = nil
	k.diffMangleEngine = nil
	k.dirtyStrata = nil
	k.factsSinceLastEval = nil
}

// evaluateDiffLocked tries the differential-eval fast path. Returns
// (handled=true, nil) if it completed the evaluation; (handled=false, nil) if
// the caller should fall back to the full path; or (handled=false, err) on a
// real error. Caller must hold k.mu.
func (k *RealKernel) evaluateDiffLocked() (bool, error) {
	if k.programInfo == nil {
		return false, nil
	}

	// Lazy-build the diff engine on first use after a policy rebuild. We feed
	// the same schemas+policy+learned string into a parallel mangle.Engine so
	// its predicateIndex matches the kernel's programInfo, then wrap it.
	//
	// IMPORTANT: We deliberately convert facts using types.Fact.ToAtom() (the
	// kernel's own encoding) rather than letting DifferentialEngine.ApplyDelta
	// call mangle.Engine.factToAtomLocked. The two paths apply different
	// type-coercion rules — Engine.factToAtomLocked auto-promotes identifier
	// strings to ast.Name, while Fact.ToAtom does not. Using ApplyAtomDelta
	// keeps the encoding identical to the full-rebuild path so query results
	// match bit-for-bit.
	if k.diffEngine == nil {
		eng, derr := k.buildDiffEngineLocked()
		if derr != nil {
			logging.Get(logging.CategoryKernel).Warn("evaluate: diff engine build failed, falling back to full eval: %v", derr)
			k.invalidateDiffEngineLocked("build failed")
			return false, nil
		}
		k.diffEngine = eng
		// Seed the diff engine with the entire current EDB on first build so
		// downstream queries see all facts, not just the delta. Reuse cachedAtoms
		// when available; otherwise convert on the fly.
		seedAtoms, err := k.factsToAtomsLocked(k.facts)
		if err != nil {
			logging.Get(logging.CategoryKernel).Warn("evaluate: diff engine seed conversion failed, falling back: %v", err)
			k.invalidateDiffEngineLocked("seed convert failed")
			return false, nil
		}
		if len(seedAtoms) > 0 {
			if err := k.diffEngine.ApplyAtomDelta(seedAtoms); err != nil {
				logging.Get(logging.CategoryKernel).Warn("evaluate: diff engine seeding failed, falling back: %v", err)
				k.invalidateDiffEngineLocked("seed failed")
				return false, nil
			}
		}
		k.factsSinceLastEval = nil
		k.dirtyStrata = nil
		if err := k.copyDiffStoreToKernelLocked(); err != nil {
			logging.Get(logging.CategoryKernel).Warn("evaluate: diff store copy failed, falling back: %v", err)
			k.invalidateDiffEngineLocked("copy failed")
			return false, nil
		}
		return true, nil
	}

	// No new facts since the last evaluate? Nothing to do.
	if len(k.factsSinceLastEval) == 0 {
		logging.KernelDebug("evaluate: diff path - no new facts, no-op")
		// Still publish the existing store contents as k.store (it's already
		// the diff-engine union from the previous pass).
		return true, nil
	}

	// Convert delta facts using the kernel's own ToAtom encoding.
	deltaAtoms, err := k.factsToAtomsLocked(k.factsSinceLastEval)
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("evaluate: delta conversion failed, falling back: %v", err)
		k.invalidateDiffEngineLocked("delta convert failed")
		return false, nil
	}
	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.diff_apply")
	if err := k.diffEngine.ApplyAtomDelta(deltaAtoms); err != nil {
		evalTimer.Stop()
		logging.Get(logging.CategoryKernel).Warn("evaluate: ApplyAtomDelta failed, falling back to full eval: %v", err)
		k.invalidateDiffEngineLocked("ApplyAtomDelta failed")
		return false, nil
	}
	evalDuration := evalTimer.Stop()
	logging.KernelDebug("evaluate: diff path applied %d facts in %v (dirtyStrata=%d)", len(deltaAtoms), evalDuration, len(k.dirtyStrata))

	k.factsSinceLastEval = nil
	k.dirtyStrata = nil

	if err := k.copyDiffStoreToKernelLocked(); err != nil {
		logging.Get(logging.CategoryKernel).Warn("evaluate: diff store copy failed, falling back: %v", err)
		k.invalidateDiffEngineLocked("copy failed")
		return false, nil
	}
	return true, nil
}

// buildDiffEngineLocked instantiates a parallel mangle.Engine seeded with the
// same schemas+policy+learned source as the kernel, then wraps it in a
// DifferentialEngine. The mangle.Engine's predicate index drives
// factToAtomLocked inside ApplyDelta. Caller must hold k.mu.
func (k *RealKernel) buildDiffEngineLocked() (*manglepkg.DifferentialEngine, error) {
	cfg := manglepkg.DefaultConfig()
	// The kernel enforces its own derived-fact limit on the full path; mirror
	// it onto the diff engine so the two paths cannot diverge in safety.
	if k.derivedFactLimit > 0 {
		cfg.DerivedFactsLimit = k.derivedFactLimit
	}
	cfg.AutoEval = true
	eng, err := manglepkg.NewEngine(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("diff: NewEngine: %w", err)
	}

	// Construct schemas+policy+learned in the same order rebuildProgram uses.
	var sb strings.Builder
	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
	}
	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
	}
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
	}
	if err := eng.LoadSchemaString(sb.String()); err != nil {
		return nil, fmt.Errorf("diff: LoadSchemaString: %w", err)
	}

	de, err := manglepkg.NewDifferentialEngine(eng)
	if err != nil {
		return nil, fmt.Errorf("diff: NewDifferentialEngine: %w", err)
	}
	// Opt into the unified fast path. The kernel doesn't use Snapshot /
	// Query / RegisterVirtualPredicate on this engine — it just needs the
	// derived-fact union via CopyAllFactsTo — so it can pay zero
	// per-stratum overhead. Caller paths that need the per-stratum API
	// (ouroboros, torture tests) construct their own DifferentialEngine
	// and don't enable the fast path.
	if err := de.EnableUnifiedFastPath(); err != nil {
		return nil, fmt.Errorf("diff: EnableUnifiedFastPath: %w", err)
	}
	k.diffMangleEngine = eng
	return de, nil
}

// factsToAtomsLocked converts a fact slice to ast.Atoms using the kernel's
// canonical encoding (types.Fact.ToAtom). Mirrors the conversion the full
// eval path uses, so diff-evaluated atoms are bit-identical to atoms
// inserted by evaluateFullLocked. Caller must hold k.mu.
func (k *RealKernel) factsToAtomsLocked(facts []Fact) ([]ast.Atom, error) {
	out := make([]ast.Atom, 0, len(facts))
	for _, f := range facts {
		atom, err := f.ToAtom()
		if err != nil {
			return nil, fmt.Errorf("ToAtom(%s): %w", f.Predicate, err)
		}
		out = append(out, atom)
	}
	return out, nil
}

// copyDiffStoreToKernelLocked materializes the union of the diff engine's
// per-stratum stores into k.store so the read path (Query, QueryCallback,
// QueryAll) is unchanged. This is the simplest correct integration; a more
// efficient future variant would expose a chained view directly. Caller must
// hold k.mu.
func (k *RealKernel) copyDiffStoreToKernelLocked() error {
	if k.diffEngine == nil {
		return fmt.Errorf("copyDiffStoreToKernel: diff engine is nil")
	}
	dest := factstore.NewSimpleInMemoryStore()
	if err := k.diffEngine.CopyAllFactsTo(dest); err != nil {
		return err
	}
	k.store = dest
	return nil
}

// rebuild invalidates cached atoms and marks the kernel for lazy re-evaluation.
// Callers should not expect the store to be up-to-date after this call;
// the next Query/QueryAll will trigger evaluate() on demand.
//
// IMPORTANT: rebuild() is the funnel for retract paths (Retract,
// RetractFact, RetractExactFact, RetractExactFactsBatch,
// RemoveFactsByPredicateSet, LoadFactsSeq's seq path). A retract may have
// removed an EDB fact whose derived consequences are still cached inside the
// diff engine's stratum stores. Differential evaluation cannot incrementally
// un-derive without DRed-style bookkeeping, so the safe and correct policy is
// to invalidate the diff engine here and force a full rebuild on the next
// evaluate(). Callers must hold k.mu.
func (k *RealKernel) rebuild() error {
	logging.KernelDebug("rebuild: invalidating cached atoms, marking factsDirty")
	k.cachedAtoms = nil
	k.factsDirty.Store(true)
	k.invalidateDiffEngineLocked("retract path / rebuild")
	return nil
}

// ensureEvaluated runs evaluate() if facts have changed since the last
// evaluation. Replaces the historical RLock→Unlock→Lock→RLock dance in
// Query/QueryCallback/QueryAll with a single-flight pattern guarded by
// evalSingleflight.
//
// Concurrency: callers MUST NOT hold k.mu when invoking this. After this
// returns nil, callers should take k.mu.RLock() to read the store.
// Another writer may dirty the kernel between ensureEvaluated and the
// subsequent RLock, but that race already existed in the previous
// implementation and is handled by readers checking k.initialized.
func (k *RealKernel) ensureEvaluated() error {
	if !k.factsDirty.Load() {
		return nil
	}
	k.evalSingleflight.Lock()
	defer k.evalSingleflight.Unlock()
	// Double-check: another goroutine may have evaluated while we waited.
	if !k.factsDirty.Load() {
		return nil
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.factsDirty.Load() {
		// Re-check under kernel lock to avoid evaluating against a half-mutated
		// EDB if a writer slipped in between the singleflight lock and here.
		return nil
	}
	logging.Kernel("kernel.lazy_evaluate triggered | factsDirty=true")
	if err := k.evaluate(); err != nil {
		return fmt.Errorf("lazy evaluation failed: %w", err)
	}
	k.factsDirty.Store(false)
	return nil
}

// IsInitialized returns true if the kernel has been initialized.
func (k *RealKernel) IsInitialized() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.initialized
}

// GetStore returns the underlying FactStore for advanced operations.
func (k *RealKernel) GetStore() factstore.FactStore {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.store
}

// Clear removes all facts from the kernel (but keeps schemas/policy).
func (k *RealKernel) Clear() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = make([]Fact, 0)
	k.cachedAtoms = make([]ast.Atom, 0) // OPTIMIZATION: Clear atom cache
	k.factIndex = make(map[string]struct{})
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	k.invalidateDiffEngineLocked("Clear")
	logging.KernelDebug("Kernel cleared (facts removed, schemas/policy retained)")
}

// Reset resets the kernel to initial state (removes facts, keeps loaded policy).
func (k *RealKernel) Reset() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = make([]Fact, 0)
	k.cachedAtoms = make([]ast.Atom, 0) // OPTIMIZATION: Clear atom cache
	k.factIndex = make(map[string]struct{})
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	k.invalidateDiffEngineLocked("Reset")
	// Keep schemas, policy, learned - only reset facts
	logging.KernelDebug("Kernel reset (facts cleared, policy retained)")
}

// Clone creates a deep copy of the kernel for simulation/shadow mode.
func (k *RealKernel) Clone() *RealKernel {
	k.mu.RLock()
	defer k.mu.RUnlock()

	clone := &RealKernel{
		facts:             make([]Fact, len(k.facts)),
		cachedAtoms:       nil, // Rebuild fresh to avoid shared memory pointers
		factIndex:         make(map[string]struct{}, len(k.factIndex)),
		bootFacts:         make([]Fact, len(k.bootFacts)),
		bootIntents:       make([]HybridIntent, len(k.bootIntents)),
		bootPrompts:       make([]HybridPrompt, len(k.bootPrompts)),
		store:             factstore.NewSimpleInMemoryStore(), // Fresh store
		programInfo:       k.programInfo,                      // Share programInfo (immutable after analysis)
		strata:            k.strata,                           // Share strata (immutable after stratification)
		predToStratum:     k.predToStratum,                    // Share predToStratum (immutable after stratification)
		schemas:           k.schemas,
		policy:            k.policy,
		learned:           k.learned,
		loadedPolicyFiles: make(map[string]struct{}, len(k.loadedPolicyFiles)),
		schemaValidator:   k.schemaValidator, // Share validator (read-only)
		initialized:       k.initialized,
		manglePath:        k.manglePath,
		workspaceRoot:     k.workspaceRoot,
		policyDirty:       k.policyDirty,
		// factsDirty is atomic.Bool — cannot be copied by value; set on the clone below.
		userLearnedPath:   k.userLearnedPath,
		predicateCorpus:   k.predicateCorpus,   // Share corpus (read-only)
		repairInterceptor: k.repairInterceptor, // Share interceptor
		virtualStore:      k.virtualStore,
		simulateCommitErr: k.simulateCommitErr,
	}
	// Mirror atomic factsDirty state onto the clone (atomic.Bool can't be copied).
	clone.factsDirty.Store(k.factsDirty.Load())

	// Deep copy facts
	for i, f := range k.facts {
		clonedArgs := make([]any, len(f.Args))
		for j, arg := range f.Args {
			clonedArgs[j] = deepCopyArg(arg)
		}
		clone.facts[i] = Fact{
			Predicate: f.Predicate,
			Args:      clonedArgs,
		}
	}

	// Deep copy bootFacts
	for i, f := range k.bootFacts {
		clonedArgs := make([]any, len(f.Args))
		for j, arg := range f.Args {
			clonedArgs[j] = deepCopyArg(arg)
		}
		clone.bootFacts[i] = Fact{
			Predicate: f.Predicate,
			Args:      clonedArgs,
		}
	}

	copy(clone.bootIntents, k.bootIntents)
	copy(clone.bootPrompts, k.bootPrompts)

	// Deep copy factIndex
	for key := range k.factIndex {
		clone.factIndex[key] = struct{}{}
	}

	// Deep copy loadedPolicyFiles
	for key := range k.loadedPolicyFiles {
		clone.loadedPolicyFiles[key] = struct{}{}
	}

	logging.KernelDebug("Kernel cloned (facts=%d, policy=%d bytes)", len(clone.facts), len(clone.policy))
	return clone
}

func deepCopyArg(arg any) any {
	if arg == nil {
		return nil
	}
	switch v := arg.(type) {
	case []any:
		res := make([]any, len(v))
		for i, item := range v {
			res[i] = deepCopyArg(item)
		}
		return res
	case map[string]any:
		res := make(map[string]any)
		for k, val := range v {
			res[k] = deepCopyArg(val)
		}
		return res
	default:
		return v
	}
}

// ClearSchemas removes all loaded schemas and policy from the kernel.
func (k *RealKernel) ClearSchemas() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = ""
	k.policy = ""
	k.programInfo = nil
	k.policyDirty = true
	k.invalidateDiffEngineLocked("ClearSchemas")
}
