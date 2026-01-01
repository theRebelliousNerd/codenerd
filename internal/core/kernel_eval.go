package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/logging"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
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
	analyzeTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.analyze")
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: analysis failed: %v", err)
		// DEBUG: Dump program when analysis fails
		_ = os.WriteFile("debug_program_ERROR.mg", []byte(programStr), 0644)
		logging.KernelDebug("Dumped failed program to debug_program_ERROR.mg")
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	analyzeTimer.Stop()

	k.programInfo = programInfo
	k.policyDirty = false

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
	if len(k.cachedAtoms) != len(k.facts) {
		logging.Get(logging.CategoryKernel).Warn("evaluate: cache desync (atoms=%d facts=%d), rebuilding cache", len(k.cachedAtoms), len(k.facts))
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
	evalStore := factstore.FactStore(baseStore)
	if k.virtualStore != nil {
		evalStore = newVirtualFactStore(baseStore, k.virtualStore)
	}

	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	const derivedFactLimit = 500000
	logging.KernelDebug("evaluate: running fixpoint evaluation (derivedFactLimit=%d)", derivedFactLimit)

	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.fixpoint")
	stats, err := engine.EvalProgramWithStats(k.programInfo, evalStore,
		engine.WithCreatedFactLimit(derivedFactLimit)) // Hard cap: max 500K derived facts
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
	k.wrapStoreLocked()

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

// rebuild is kept for backward compatibility - now delegates to evaluate().
func (k *RealKernel) rebuild() error {
	logging.KernelDebug("rebuild: delegating to evaluate()")
	return k.evaluate()
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
	k.wrapStoreLocked()
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
	k.wrapStoreLocked()
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
		cachedAtoms:       make([]ast.Atom, len(k.cachedAtoms)), // OPTIMIZATION: Clone atom cache
		factIndex:         make(map[string]struct{}, len(k.factIndex)),
		bootFacts:         make([]Fact, len(k.bootFacts)),
		bootIntents:       make([]HybridIntent, len(k.bootIntents)),
		bootPrompts:       make([]HybridPrompt, len(k.bootPrompts)),
		store:             factstore.NewSimpleInMemoryStore(), // Fresh store
		programInfo:       k.programInfo,                      // Share programInfo (immutable after analysis)
		schemas:           k.schemas,
		policy:            k.policy,
		learned:           k.learned,
		loadedPolicyFiles: make(map[string]struct{}, len(k.loadedPolicyFiles)),
		schemaValidator:   k.schemaValidator, // Share validator (read-only)
		initialized:       k.initialized,
		manglePath:        k.manglePath,
		workspaceRoot:     k.workspaceRoot,
		policyDirty:       k.policyDirty,
		userLearnedPath:   k.userLearnedPath,
		predicateCorpus:   k.predicateCorpus,   // Share corpus (read-only)
		repairInterceptor: k.repairInterceptor, // Share interceptor
		virtualStore:      k.virtualStore,
	}

	// Deep copy facts and cached atoms
	copy(clone.facts, k.facts)
	copy(clone.cachedAtoms, k.cachedAtoms) // OPTIMIZATION: Copy atom cache
	copy(clone.bootFacts, k.bootFacts)
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

	// OPTIMIZATION: Use cached atoms instead of re-converting
	for _, atom := range clone.cachedAtoms {
		clone.store.Add(atom)
	}
	clone.wrapStoreLocked()

	logging.KernelDebug("Kernel cloned (facts=%d, policy=%d bytes)", len(clone.facts), len(clone.policy))
	return clone
}
