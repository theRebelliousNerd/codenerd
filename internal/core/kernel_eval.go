package core

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

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

// compiledProgramCache shares one analyzed program across the many kernels a
// boot constructs from identical schemas+policy+learned bytes. ProgramInfo is
// already shared between a kernel and its clones as immutable after analysis;
// evaluation and cone indexing only read it. The cache is deliberately tiny:
// the live boot program plus a few variants, not an unbounded archive of every
// candidate ever compiled. Sandbox trial programs bypass it entirely.
type compiledProgram struct {
	info          *analysis.ProgramInfo
	strata        []analysis.Nodeset
	predToStratum map[ast.PredicateSym]int
	cone          *coneIndex
}

const compiledProgramCacheMax = 8

var (
	compiledProgramMu    sync.RWMutex
	compiledProgramCache = map[[sha256.Size]byte]compiledProgram{}
)

func programCacheKey(program string) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(program))
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))
	return key
}

func lookupCompiledProgram(key [sha256.Size]byte) (compiledProgram, bool) {
	compiledProgramMu.RLock()
	defer compiledProgramMu.RUnlock()
	compiled, ok := compiledProgramCache[key]
	return compiled, ok
}

func storeCompiledProgram(key [sha256.Size]byte, compiled compiledProgram) {
	compiledProgramMu.Lock()
	defer compiledProgramMu.Unlock()
	if _, exists := compiledProgramCache[key]; !exists && len(compiledProgramCache) >= compiledProgramCacheMax {
		clear(compiledProgramCache)
	}
	compiledProgramCache[key] = compiled
}

// writeProgramLocked appends schemas, policy, and learned rules to sb in
// canonical order. Load order carries meaning (stratified trust: constitution
// before learned rules), so it lives in exactly one place.
// Caller must hold k.mu (or the kernel is not yet shared).
func (k *RealKernel) writeProgramLocked(sb *strings.Builder) {
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
		logging.KernelDebug("program: included schemas (%d bytes)", len(k.schemas))
	}
	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
		logging.KernelDebug("program: included policy (%d bytes)", len(k.policy))
	}
	// Load learned rules AFTER constitution (stratified trust)
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
		logging.KernelDebug("program: included learned rules (%d bytes)", len(k.learned))
	}
}

// rebuildProgram parses schemas+policy and caches programInfo.
// This is only called when policyDirty is true.
// Caller must hold k.mu, or the kernel must not be shared yet (boot, sandbox trial).
func (k *RealKernel) rebuildProgram() error {
	timer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram")
	logging.Kernel("Rebuilding Mangle program (parsing schemas+policy+learned)")

	// Construct program from schemas + policy + learned (no facts)
	sb := programBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	defer programBuilderPool.Put(sb)
	k.writeProgramLocked(sb)

	programStr := sb.String()
	logging.KernelDebug("rebuildProgram: total program size = %d bytes", len(programStr))

	var cacheKey [sha256.Size]byte
	cacheable := !k.sandbox
	if cacheable {
		cacheKey = programCacheKey(programStr)
		if compiled, ok := lookupCompiledProgram(cacheKey); ok {
			k.programInfo = compiled.info
			k.strata = compiled.strata
			k.predToStratum = compiled.predToStratum
			k.cone = compiled.cone
			k.policyDirty = false
			k.atomCacheStale = true
			timer.StopWithInfo()
			logging.KernelDebug("rebuildProgram: reused analyzed program from cache")
			return nil
		}
	}

	// Parse
	parseTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.parse")
	parsed, err := parseUnit(strings.NewReader(programStr))
	if err != nil {
		// In a sandbox this is a trial compile of a candidate rule and a
		// rejection is the expected outcome the caller handles; see the
		// `sandbox` field on RealKernel.
		if k.sandbox {
			logging.KernelDebug("rebuildProgram (sandbox): candidate rejected at parse: %v", err)
		} else {
			logging.Get(logging.CategoryKernel).Error("rebuildProgram: parse failed: %v", err)
		}
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
		// A sandbox rejection is expected and handled by the caller, and its
		// program is a throwaway — dumping it to disk on every rejected
		// candidate would churn debug_program_ERROR.mg with non-faults.
		if k.sandbox {
			logging.KernelDebug("rebuildProgram (sandbox): candidate rejected at analysis: %v", err)
			return fmt.Errorf("failed to analyze program: %w", err)
		}
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: analysis failed: %v", err)
		if dumpPath, writeErr := k.writeFailedProgramDump(programStr); writeErr != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to write debug dump: %v", writeErr)
		} else {
			logging.KernelDebug("Dumped failed program to %s", dumpPath)
		}
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	analyzeTimer.Stop()

	// Cache stratification for EvalStratifiedProgramWithStats. Nothing is
	// installed until it succeeds: a program that analyzes but does not
	// stratify must not leave the new rules beside the old strata and cone,
	// with policyDirty already cleared.
	strata, predToStratum, err := analysis.Stratify(analysis.Program{
		EdbPredicates: programInfo.EdbPredicates,
		IdbPredicates: programInfo.IdbPredicates,
		Rules:         programInfo.Rules,
	})
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: stratification failed: %v", err)
		return fmt.Errorf("failed to stratify program: %w", err)
	}
	k.programInfo = programInfo
	k.policyDirty = false
	// New Decls may declare different bounds than the ones cachedAtoms were
	// converted under, so force one reconversion. Also covers the boot case
	// where facts were admitted before any programInfo existed.
	k.atomCacheStale = true
	k.strata = strata
	k.predToStratum = predToStratum
	k.cone = buildConeIndex(programInfo, strata, predToStratum)
	if cacheable {
		storeCompiledProgram(cacheKey, compiledProgram{
			info: programInfo, strata: strata, predToStratum: predToStratum, cone: k.cone,
		})
	}

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

// evaluate builds a fresh store from the EDB and evaluates it to fixpoint.
// Uses cached programInfo for efficiency.
//
// The store is rebuilt from cachedAtoms on EVERY call, deliberately. Bottom-up
// evaluation over a store that already holds previously derived facts is
// monotone — it can only add — so a derived fact whose negated premise later
// becomes true is never retracted, and an aggregate is added beside its old
// value instead of replacing it. That is exactly what the differential path
// did, and why it was deleted; see Docs/journeys/impl/S23-differential-path.md
// and the regression in kernel_eval_soundness_test.go. Any future incremental
// evaluator must remove every IDB fact before re-evaluating.
//
// Caller must hold k.mu, or the kernel must not be shared yet (boot).
func (k *RealKernel) evaluate() (err error) {
	defer func() { k.settleEvaluationLocked(err) }()
	started := time.Now()
	stats := EvaluationStats{InputFacts: len(k.facts)}
	defer func() { stats.Duration = time.Since(started); k.lastEvaluation = stats }()
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate")
	defer timer.Stop()

	// Rebuild program if policy changed.
	if k.policyDirty || k.programInfo == nil {
		logging.KernelDebug("evaluate: policy dirty or programInfo nil, rebuilding program")
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	} else {
		logging.KernelDebug("evaluate: using cached programInfo")
	}

	// A write whose predicates were named re-derives only their cone.
	named := !k.dirtyAll && len(k.dirtyPreds) > 0
	if err := k.evaluateConeLocked(&stats); err == nil {
		k.clearDirtyLocked()
		return nil
	} else if !errors.Is(err, errConeIneligible) {
		logging.Get(logging.CategoryKernel).Warn("evaluate: cone evaluation failed, running the full fixpoint: %v", err)
		stats.ConeFallback = err.Error()
	} else if named {
		stats.ConeFallback = k.coneIneligibilityLocked()
	}
	stats = EvaluationStats{InputFacts: stats.InputFacts, ConeFallback: stats.ConeFallback, Mode: "full"}
	prepareStarted := time.Now()

	// Create fresh store and populate with EDB facts
	// OPTIMIZATION: Use cached atoms instead of converting every time
	logging.KernelDebug("evaluate: populating store with %d EDB facts", len(k.facts))
	baseStore := newEvaluationFactStore(len(k.facts))

	// Defensive sync check: ensure cache is valid
	if k.cachedAtoms == nil || len(k.cachedAtoms) != len(k.facts) || k.atomCacheStale {
		switch {
		case k.atomCacheStale:
			logging.KernelDebug("evaluate: atom cache built without Decls or under an older policy, reconverting %d facts", len(k.facts))
		case len(k.cachedAtoms) > 0:
			logging.Get(logging.CategoryKernel).Warn("evaluate: cache desync (atoms=%d facts=%d), rebuilding cache", len(k.cachedAtoms), len(k.facts))
		default:
			logging.KernelDebug("evaluate: cache empty (facts=%d), populating cache", len(k.facts))
		}
		// Cleared first so factToAtomLocked's own staleness signal (programInfo
		// still nil) is not immediately overwritten by this pass.
		k.atomCacheStale = false
		k.cachedAtoms = make([]ast.Atom, 0, len(k.facts))
		kept := make([]Fact, 0, len(k.facts))
		dropped := 0
		for _, f := range k.facts {
			atom, err := k.factToAtomLocked(f)
			if err != nil {
				// A single malformed fact must not stop the kernel from
				// deriving everything else — that is precisely the outage this
				// conversion exists to prevent. Evict it, name it, carry on.
				// Reachable only for facts admitted before programInfo existed
				// (boot facts); addFactIfNewLocked rejects the rest up front.
				logging.Get(logging.CategoryKernel).Error("evaluate: evicting unconvertible fact %s: %v", f.Predicate, err)
				dropped++
				continue
			}
			kept = append(kept, f)
			k.cachedAtoms = append(k.cachedAtoms, atom)
		}
		// Keep facts, atoms and the dedupe index in lockstep; leaving evicted
		// facts in k.facts would make every later evaluate() see a length
		// mismatch and redo this whole conversion.
		if dropped > 0 {
			k.facts = kept
			k.rebuildFactIndexLocked()
		}
	}

	// Use cached atoms (fast path - no conversions!)
	for _, atom := range k.cachedAtoms {
		baseStore.Add(atom)
	}
	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	derivedFactLimit := k.effectiveDerivedFactLimitLocked()
	logging.KernelDebug("evaluate: running fixpoint evaluation (derivedFactLimit=%d)", derivedFactLimit)

	// Build eval options
	evalOpts := []engine.EvalOption{
		engine.WithCreatedFactLimit(derivedFactLimit),
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

	evalOpts = append(evalOpts, k.externalPredicateOptionsLocked()...)

	stats.PrepareDuration = time.Since(prepareStarted)
	stats.StoreKind = storeKind(baseStore)
	stats.Rules, stats.Strata = len(k.programInfo.Rules), len(k.strata)
	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.fixpoint")
	engineStats, err := engine.EvalStratifiedProgramWithStats(k.programInfo, k.strata, k.predToStratum, baseStore,
		evalOpts...)
	evalDuration := evalTimer.Stop()
	stats.EngineDuration = evalDuration

	if err != nil {
		logging.Get(logging.CategoryKernel).Error("evaluate: fixpoint evaluation failed: %v", err)
		// Check if this is a derived fact limit error
		if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "exceeded") {
			logging.Get(logging.CategoryKernel).Warn("evaluate: POSSIBLE FACT EXPLOSION - derived facts exceeded %d limit", derivedFactLimit)
		}
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	k.store = baseStore
	k.clearDirtyLocked()

	// Log evaluation stats
	totalDuration := time.Duration(0)
	for _, d := range engineStats.Duration {
		totalDuration += d
	}
	strataCount := len(engineStats.Strata)
	logging.KernelDebug("evaluate: fixpoint reached - strata=%d, evalTime=%v, wallTime=%v",
		strataCount, totalDuration, evalDuration)

	k.initialized = true
	logging.KernelDebug("evaluate: complete, kernel initialized")
	return nil
}

// externalPredicateOptionsLocked registers the VirtualStore's external
// predicates (#17), and only those with a matching external() Decl in the
// current program: tests and minimal configs load a subset of the schemas, and
// a callback without its Decl is a validation error.
func (k *RealKernel) externalPredicateOptionsLocked() []engine.EvalOption {
	if k.virtualStore == nil || k.programInfo == nil || k.programInfo.Decls == nil {
		return nil
	}
	allCallbacks := k.virtualStore.BuildExternalPredicates()
	callbacks := make(map[ast.PredicateSym]engine.ExternalPredicateCallback, len(allCallbacks))
	for pred, cb := range allCallbacks {
		if decl, declared := k.programInfo.Decls[pred]; declared && decl.IsExternal() {
			callbacks[pred] = cb
		}
	}
	if len(callbacks) == 0 {
		return nil
	}
	logging.KernelDebug("evaluate: registered %d/%d external predicates (filtered by Decl)", len(callbacks), len(allCallbacks))
	return []engine.EvalOption{engine.WithExternalPredicates(callbacks)}
}

// rebuild invalidates cached atoms and marks the kernel for lazy re-evaluation.
// Callers should not expect the store to be up-to-date after this call;
// the next Query/QueryAll will trigger evaluate() on demand.
//
// rebuild() is the funnel for retract paths (Retract, RetractFact,
// RetractExactFact, RetractExactFactsBatch, RemoveFactsByPredicateSet,
// LoadFactsSeq's seq path). Dropping cachedAtoms is what makes a retract
// visible: evaluate() reconverts from k.facts and derives over a store that
// no longer contains the removed fact or anything derived from it.
// Callers must hold k.mu.
func (k *RealKernel) rebuild(preds ...string) error {
	logging.KernelDebug("rebuild: invalidating cached atoms, marking factsDirty")
	k.cachedAtoms = nil
	k.markDirtyLocked(preds...)
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
	if k == nil {
		return fmt.Errorf("ensureEvaluated: kernel is nil")
	}
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

// clearFactsLocked empties the EDB while retaining schemas, policy, and
// learned rules. It marks factsDirty so the next read lazily re-evaluates
// to a fresh empty store; without that, reads after a clear would error
// with "kernel not initialized". Caller must hold k.mu.
func (k *RealKernel) clearFactsLocked() {
	k.facts = make([]Fact, 0)
	k.cachedAtoms = make([]ast.Atom, 0) // OPTIMIZATION: Clear atom cache
	k.factIndex = newEDBIndex()
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	k.evalErr = nil // the store is new, there is no failed evaluation to answer for
	// factsDirty stays as-is (Clear does not set dirty): the !initialized
	// state alone drives the next read to re-evaluate via ensureEvaluated.
}

// Clear removes all facts from the kernel (but keeps schemas/policy).
func (k *RealKernel) Clear() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.clearFactsLocked()
	logging.KernelDebug("Kernel cleared (facts removed, schemas/policy retained)")
}

// Reset resets the kernel to initial state (removes facts, keeps loaded policy).
func (k *RealKernel) Reset() {
	if k == nil {
		logging.Get(logging.CategoryKernel).Error("Reset: kernel is nil")
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.clearFactsLocked()
	// Keep schemas, policy, learned - only reset facts
	logging.KernelDebug("Kernel reset (facts cleared, policy retained)")
}

// Clone creates an independent copy of the kernel for simulation/shadow mode.
//
// Writes to the clone never reach the parent and the reverse: the EDB slice,
// dedupe index, store and dirty state are the clone's own. The stored Fact
// values are shared, not deep-copied: the kernel never mutates a stored fact
// (a write replaces or removes it), so a copy bought nothing but one
// allocation per fact on every dream safety check.
func (k *RealKernel) Clone() *RealKernel {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cloneLocked()
}

// cloneLocked is Clone's body; the caller holds at least k.mu.RLock.
func (k *RealKernel) cloneLocked() *RealKernel {
	clone := &RealKernel{
		facts:             slices.Clone(k.facts),
		cachedAtoms:       nil, // Rebuild fresh to avoid shared memory pointers
		bootFacts:         slices.Clone(k.bootFacts),
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
		sandbox:           k.sandbox,         // A clone of a trial kernel is still a trial kernel
		schemaValidator:   k.schemaValidator, // Share validator (read-only)
		initialized:       k.initialized,
		manglePath:        k.manglePath,
		workspaceRoot:     k.workspaceRoot,
		policyDirty:       k.policyDirty,
		derivedFactLimit:  k.derivedFactLimit, // Custom inference ceilings must survive cloning
		maxFacts:          k.maxFacts,
		// factsDirty is atomic.Bool — cannot be copied by value; set on the clone below.
		userLearnedPath:   k.userLearnedPath,
		predicateCorpus:   k.predicateCorpus,   // Share corpus (read-only)
		repairInterceptor: k.repairInterceptor, // Share interceptor
		virtualStore:      k.virtualStore,
		simulateCommitErr: k.simulateCommitErr,
		eventBus:          NewFactEventBus(), // Fresh bus: every kernel needs a non-nil one
		cone:              k.cone,            // Share the dependency index (immutable)
		dirtyAll:          true,              // The clone has no store; its first evaluate is a full one
		// Deliberately fresh: diff engine state (rebuilt lazily), proof
		// recorder (sharing it would race), lastEvaluation, undeclared
		// warnings (re-warn on the clone is benign).
	}
	clone.factsDirty.Store(true)
	// A clean, evaluated parent hands over its store, so the clone's next
	// evaluate re-derives only the cone of what is written to it. Without
	// this every clone paid a full fixpoint: a dream safety check, which
	// stages one action's projected facts on a clone and asks panic_state,
	// cost ~2.5 s per gated action (measured 2026-10-27, 102 s over 50).
	if k.initialized && k.store != nil && k.evalErr == nil && !k.factsDirty.Load() && !k.dirtyAll &&
		len(k.dirtyPreds) == 0 && !k.policyDirty && !k.atomCacheStale {
		// The copy is a simple store even when the parent's is indexed:
		// rebuilding argument indexes per clone cost more than the joins it
		// saved on every measured dream shape (BenchmarkCloneStoreKind,
		// Docs/audits/mangle-kernel-performance.md "As built").
		if copied, ok := copyFactStore(k.store, factstore.NewSimpleInMemoryStore()); ok {
			clone.store = copied
			clone.dirtyAll = false
			clone.factsDirty.Store(false)
		}
	}

	if clone.facts == nil {
		clone.facts = make([]Fact, 0)
	}
	if k.factIndex != nil {
		clone.factIndex = k.factIndex.clone()
	}

	copy(clone.bootIntents, k.bootIntents)
	copy(clone.bootPrompts, k.bootPrompts)

	// Deep copy loadedPolicyFiles
	for key := range k.loadedPolicyFiles {
		clone.loadedPolicyFiles[key] = struct{}{}
	}

	logging.KernelDebug("Kernel cloned (facts=%d, policy=%d bytes)", len(clone.facts), len(clone.policy))
	return clone
}

// ClearSchemas removes all loaded schemas, policy, and learned rules from
// the kernel. Learned rules are cleared too: they reference schema
// predicates, so keeping them across a schema swap would fail the next
// analysis with undeclared predicates.
func (k *RealKernel) ClearSchemas() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = ""
	k.policy = ""
	k.learned = ""
	k.programInfo = nil
	k.markPolicyDirtyLocked()
}

// writeFailedProgramDump saves the combined Mangle program that failed analysis
// and returns the path it wrote.
//
// It writes under .nerd/debug/ rather than the process working directory. The
// dump used to land wherever the process happened to be running, which put a
// 700 KB debug_program_ERROR.mg *inside the scanned source tree* — and the
// world scanner duly ingested it, asserting knowledge_link facts like
// pred:panic_state/2 "defined_in" debug_program_ERROR.mg. A crash artifact was
// being indexed as if it were real source, polluting the knowledge graph with
// duplicate predicate definitions. .nerd/ is excluded from scans, so putting it
// there fixes every ingestion path at once instead of adding a skip list to
// each one.
func (k *RealKernel) writeFailedProgramDump(programStr string) (string, error) {
	dir := k.nerdPath("debug")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create debug dir: %w", err)
	}
	path := filepath.Join(dir, "debug_program_ERROR.mg")
	if err := os.WriteFile(path, []byte(programStr), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// indexedStoreMinFacts is the store size from which evaluation uses the
// argument-indexed store. Below it the simple store's allocation profile wins.
const indexedStoreMinFacts = 1024

// Large bound joins must not scan every row of a predicate on each premise.
// Keep the small-store allocation profile for small kernels. The indexed array
// implementation also checks equality within hash buckets, including collisions.
// Both kinds support removal, which the cone evaluator relies on.
func newEvaluationFactStore(facts int) factstore.FactStoreWithRemove {
	if facts >= indexedStoreMinFacts {
		return factstore.NewMultiIndexedArrayInMemoryStore()
	}
	return factstore.NewSimpleInMemoryStore()
}

// copyFactStore copies every atom of an evaluated store, extensional and
// derived, into dst, which must support removal (the cone evaluator removes
// and re-derives in place). The atoms are immutable values and are shared.
func copyFactStore(src factstore.FactStore, dst factstore.FactStoreWithRemove) (factstore.FactStoreWithRemove, bool) {
	for _, sym := range src.ListPredicates() {
		if err := src.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			dst.Add(a)
			return nil
		}); err != nil {
			return nil, false
		}
	}
	return dst, true
}
