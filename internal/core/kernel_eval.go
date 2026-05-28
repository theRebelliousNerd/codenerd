package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/parse"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// =============================================================================
// MANGLE EVALUATION ENGINE
// =============================================================================

// rebuildProgram parses schemas+policy and caches programInfo.
// This is only called when policyDirty is true.
func (k *RealKernel) rebuildProgram() error {
	timer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram")
	logging.Kernel("Rebuilding Mangle program (parsing schemas+policy+learned)")

	// Construct program from schemas + policy + learned (no facts)
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	var sb strings.Builder

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

	// Analyze
	if count := strings.Count(programStr, "Decl permitted("); count > 1 {
		fmt.Printf("DEBUG: Found %d 'Decl permitted(' in programStr!\n", count)
		lines := strings.Split(programStr, "\n")
		for i, line := range lines {
			if strings.Contains(line, "Decl permitted(") {
				start := max(i-2, 0)
				fmt.Printf("MATCH AT LINE %d:\n%s\n", i, strings.Join(lines[start:i+1], "\n"))
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
func (k *RealKernel) evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate")

	// Rebuild program if policy changed
	if k.policyDirty || k.programInfo == nil {
		logging.KernelDebug("evaluate: policy dirty or programInfo nil, rebuilding program")
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	} else {
		logging.KernelDebug("evaluate: using cached programInfo")
	}

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
	timer.Stop()
	logging.KernelDebug("evaluate: complete, kernel initialized")
	return nil
}

// rebuild invalidates cached atoms and marks the kernel for lazy re-evaluation.
// Callers should not expect the store to be up-to-date after this call;
// the next Query/QueryAll will trigger evaluate() on demand.
func (k *RealKernel) rebuild() error {
	logging.KernelDebug("rebuild: invalidating cached atoms, marking factsDirty")
	k.cachedAtoms = nil
	k.factsDirty.Store(true)
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
}
