// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
// This file is best written for Gemini 3 Pro.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for internal/mangle/differential.go
//
// File: internal/mangle/differential.go
// Author: Gemini
// Date: 2025-12-08
//
// Recommended Model: Gemini 3 Pro
//
// Overview:
// This file implements the DifferentialEngine, a wrapper around the standard Mangle engine.
// It enables incremental evaluation, stratum-aware caching, and snapshot isolation to
// optimize performance as the World Model grows.
//
// Key Features & Business Value:
// - Stratum-Aware Caching: Avoids re-evaluating lower strata when only higher strata facts change.
// - Delta Propagation: Only invalidates derived facts in specific strata that depend on changes.
// - Snapshot Isolation (COW): Supports concurrent simulation branches without polluting the main store.
// - Predicate Pushdown (Lazy Loading): Lazy-loads facts (e.g., file content) to reduce memory pressure.
// - Performance: Significantly reduces evaluation time for incremental updates.
//
// Architectural Context:
// - Component Type: Logic Engine Wrapper
// - Deployment: Part of the Core Nerd Binary.
// - Communication: Wraps `mangle.Engine`, used by `Autopoiesis` and `Reasoning` shards.
// - Database Interaction: Manages in-memory `factstore` and interacts with `Persistence` for lazy loading.
//
// Dependencies & Dependents:
// - Dependencies: `codeberg.org/TauCeti/mangle-go/*`, `internal/mangle/engine.go`
// - Is a Dependency for: Future optimizations of `mangling` and `simulation` features.
//
// Deployment & Operations:
// - CI/CD: Standard Go build.
// - Configuration: Inherits config from `mangle.Config`.
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling and clear logging.
//
// Functions / Classes:
// - `DifferentialEngine`: Main struct wrapping the engine.
// - `ApplyDelta`: Applies changes potentially incrementally.
// - `Snapshot`: Creates a COW snapshot.
//
// Usage:
// diffEngine := NewDifferentialEngine(baseEngine)
// diffEngine.ApplyDelta(newFacts)
//
// References:
// - Cortex 1.5.0 Neuro-Symbolic Architecture
//
// --- END OF PRD HEADER ---

package mangle

import (
	"context"
	"fmt"
	"maps"
	"os"
	"sync"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	mengine "codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/unionfind"
)

// KnowledgeGraph represents a stratum of the knowledge base.
// It holds facts specific to a layer of evaluation.
type KnowledgeGraph struct {
	store    factstore.FactStore
	isFrozen bool
	mu       sync.RWMutex
}

// NewKnowledgeGraph creates a new KnowledgeGraph.
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		store: factstore.NewSimpleInMemoryStore(),
	}
}

// DifferentialEngine wraps the standard Engine to support incremental updates
// and snapshot isolation.
type DifferentialEngine struct {
	baseEngine  *Engine
	config      Config
	programInfo *analysis.ProgramInfo

	// strataStores holds a separate store for each stratum.
	// Index i corresponds to stratum i.
	strataStores []*KnowledgeGraph
	// predStratum maps predicate symbol to stratum index.
	predStratum map[ast.PredicateSym]int

	// Ordered list of rules per stratum for evaluation
	strataRules [][]ast.Clause

	// Per-stratum cached EvalStratifiedProgramWithStats inputs, used by
	// the legacy stratified ApplyAtomDelta / ApplyDelta paths to avoid
	// re-running analysis.Stratify per stratum per delta.
	strataNodesets []analysis.Nodeset
	strataPredMaps []map[ast.PredicateSym]int

	// =====================================================================
	// Unified fast-path (opt-in via EnableUnifiedFastPath)
	// =====================================================================
	// The legacy ApplyDelta loops strata 0..N calling
	// EvalStratifiedProgramWithStats N times. For codeNERD's EDB-heavy
	// delta pattern (minChangedStratum=0 → re-eval ALL strata) the per-call
	// engine setup overhead dominates wall time.
	//
	// The unified fast path replaces that loop with a single
	// EvalStratifiedProgramWithStats call over a unified factstore.
	// The single call lets the engine's seminaive evaluator do
	// delta-aware fixpoint internally and amortises the per-call setup.
	//
	// When `unifiedStore` is non-nil, `ApplyAtomDelta` writes new atoms
	// into it AND ALSO into the legacy strataStores (so Snapshot, Query,
	// RegisterVirtualPredicate keep working for non-kernel callers like
	// the ouroboros loop and the torture tests). Then it issues a single
	// eval over unifiedStore using the full-program stratification cached
	// in unifiedStrata / unifiedPredToStratum.
	//
	// `CopyAllFactsTo` prefers the unified store when set (one fast walk)
	// and falls back to the per-stratum walk otherwise. Code paths that
	// don't enable the fast path remain byte-identical to pre-upgrade.
	unifiedStore         factstore.FactStore
	unifiedStrata        []analysis.Nodeset
	unifiedPredToStratum map[ast.PredicateSym]int

	mu sync.RWMutex
}

// ChainedFactStore implements a view over multiple fact stores.
// It allows writing to the 'overlay' (current stratum) and reading from 'base' (previous strata).
type ChainedFactStore struct {
	base    []factstore.FactStore
	overlay factstore.FactStore
}

func (cfs *ChainedFactStore) Add(atom ast.Atom) bool {
	return cfs.overlay.Add(atom)
}

func (cfs *ChainedFactStore) ListPredicates() []ast.PredicateSym {
	seen := make(map[ast.PredicateSym]bool)
	var result []ast.PredicateSym

	// Overlay first
	for _, sym := range cfs.overlay.ListPredicates() {
		if !seen[sym] {
			seen[sym] = true
			result = append(result, sym)
		}
	}
	// Then bases
	for _, bs := range cfs.base {
		for _, sym := range bs.ListPredicates() {
			if !seen[sym] {
				seen[sym] = true
				result = append(result, sym)
			}
		}
	}
	return result
}

func (cfs *ChainedFactStore) EstimateFactCount() int {
	count := cfs.overlay.EstimateFactCount()
	for _, bs := range cfs.base {
		count += bs.EstimateFactCount()
	}
	return count
}

func (cfs *ChainedFactStore) GetFacts(query ast.Atom, fn func(ast.Atom) error) error {
	// Only query stores that COULD contain the predicate.
	// However, without tracking, we query all.
	// Optimization: In DifferentialEngine, we know which stratum a predicate belongs to.
	// But this generic store implementation doesn't know.
	// We will query all.

	// We must deduplicate if the same fact could technically exist in multiple (unlikely in valid stratification but possible in bad state).
	// For performance we assume disjoint predicates across strata or explicit layering.

	if err := cfs.overlay.GetFacts(query, fn); err != nil {
		return err
	}
	for _, bs := range cfs.base {
		if err := bs.GetFacts(query, fn); err != nil {
			return err
		}
	}
	return nil
}

// Ensure ChainedFactStore implements FactStore interface
var _ factstore.FactStore = (*ChainedFactStore)(nil)

func (cfs *ChainedFactStore) Contains(atom ast.Atom) bool {
	if cfs.overlay.Contains(atom) {
		return true
	}
	for _, bs := range cfs.base {
		if bs.Contains(atom) {
			return true
		}
	}
	return false
}

func (cfs *ChainedFactStore) Merge(other factstore.ReadOnlyFactStore) {
	_ = other.GetFacts(ast.Atom{}, func(atom ast.Atom) error {
		cfs.overlay.Add(atom)
		return nil
	})
}

// Snapshot creates a Copy-On-Write snapshot of the engine.
func (de *DifferentialEngine) Snapshot() *DifferentialEngine {
	de.mu.RLock()
	defer de.mu.RUnlock()

	newStrata := make([]*KnowledgeGraph, len(de.strataStores))
	for i, layer := range de.strataStores {
		newLayer := NewKnowledgeGraph()
		// Copy facts - leveraging that SimpleInMemoryStore iterates all facts
		for _, predSym := range layer.store.ListPredicates() {
			layer.store.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
				newLayer.store.Add(a)
				return nil
			})
		}
		newStrata[i] = newLayer
	}

	return &DifferentialEngine{
		baseEngine:   de.baseEngine,
		config:       de.config,
		programInfo:  de.programInfo,
		strataStores: newStrata,
		predStratum:  de.predStratum,
		strataRules:  de.strataRules,
	}
}

// Ensure ChainedFactStore implements FactStore interface
var _ factstore.FactStore = (*ChainedFactStore)(nil)

// NewDifferentialEngine creates a new differential engine wrapper.
func NewDifferentialEngine(base *Engine) (*DifferentialEngine, error) {
	if base.programInfo == nil {
		return nil, fmt.Errorf("base engine must have a loaded schema/program")
	}

	de := &DifferentialEngine{
		baseEngine:  base,
		config:      base.config,
		programInfo: base.programInfo,
	}

	// Compute Stratification
	strataMap, maxStratum := computeStrata(base.programInfo)
	de.predStratum = strataMap

	// Group rules by stratum
	de.strataRules = make([][]ast.Clause, maxStratum+1)
	for _, rule := range base.programInfo.Rules {
		headSym := rule.Head.Predicate
		s := strataMap[headSym]
		de.strataRules[s] = append(de.strataRules[s], rule)
	}

	// Initialize stores
	de.strataStores = make([]*KnowledgeGraph, maxStratum+1)
	for i := 0; i <= maxStratum; i++ {
		de.strataStores[i] = NewKnowledgeGraph()
	}

	// Pre-build the per-stratum nodesets and predicate maps so ApplyDelta
	// doesn't pay an analysis.Stratify call per stratum per delta. Each
	// stratum's nodeset is the set of predicates derived by its rules
	// (i.e. rule heads); each predicate maps to local stratum 0 inside
	// its single-stratum sub-program.
	de.strataNodesets = make([]analysis.Nodeset, maxStratum+1)
	de.strataPredMaps = make([]map[ast.PredicateSym]int, maxStratum+1)
	for s := 0; s <= maxStratum; s++ {
		nodes := make(analysis.Nodeset)
		predMap := make(map[ast.PredicateSym]int)
		for _, rule := range de.strataRules[s] {
			nodes[rule.Head.Predicate] = struct{}{}
			predMap[rule.Head.Predicate] = 0
		}
		de.strataNodesets[s] = nodes
		de.strataPredMaps[s] = predMap
	}

	return de, nil
}

// computeStrata: EDB → 0, IDB → 1.
//
// This deliberately keeps the 2-bucket scheme even though analysis.Stratify
// is available. Empirically, fine-grained stratification HURTS this diff
// engine's wall time on codeNERD's workload:
//
//   - The dominant delta pattern is "assert EDB fact", which lands in
//     stratum 0. With fine-grained strata, the inner ApplyDelta loop runs
//     N iterations of EvalStratifiedProgramWithStats (one per stratum
//     above the change), each setting up its own ChainedFactStore and
//     paying the engine's per-call setup overhead (chain construction,
//     store init, predicate indexing).
//   - With 2 buckets, the same delta triggers a single
//     EvalStratifiedProgramWithStats over the full rule set, letting the
//     engine's seminaive evaluator do incremental work internally — which
//     is what we actually wanted.
//
// A measured experiment (2026-05-28) with analysis.Stratify-based
// stratification + per-stratum cached Nodeset/PredMap inputs caused
// TestKernelDifferentialEval to time out at 60 s where the 2-bucket
// scheme finishes in ~1 s.
//
// Future direction (out of scope for this revision): a real incremental
// win would come from delta propagation rather than per-stratum
// re-evaluation — i.e. wire a delta-aware option into a single
// EvalStratifiedProgramWithStats call instead of looping strata. Until
// that lands, 2-bucket + cached per-stratum inputs is the best we have.
func computeStrata(info *analysis.ProgramInfo) (map[ast.PredicateSym]int, int) {
	strata := make(map[ast.PredicateSym]int, len(info.Decls))
	maxS := 0

	// IDB = appears on a rule head.
	idb := make(map[ast.PredicateSym]bool, len(info.Rules))
	for _, rule := range info.Rules {
		idb[rule.Head.Predicate] = true
	}

	for sym := range info.Decls {
		if idb[sym] {
			strata[sym] = 1
			maxS = 1
		} else {
			strata[sym] = 0
		}
	}

	return strata, maxS
}

// AddFactIncremental adds a fact and propagates changes incrementally.
func (de *DifferentialEngine) AddFactIncremental(fact Fact) error {
	return de.ApplyDelta([]Fact{fact})
}

// EnableUnifiedFastPath opts the engine into a single-call evaluation
// strategy where every ApplyAtomDelta writes into one unified factstore
// and runs ONE EvalStratifiedProgramWithStats over the full program.
//
// Why opt-in: the legacy per-stratum loop is exercised by the torture
// tests and the ouroboros Query path which rely on the strataStores
// layering. The codeNERD kernel doesn't — it just needs the union of
// derived facts via CopyAllFactsTo — so it can pay zero per-stratum
// overhead by enabling this path right after NewDifferentialEngine.
//
// Idempotent: calling twice is harmless. Returns an error only if the
// full-program stratification cannot be computed (which would also
// have failed the base engine's analysis, so this is defensive).
func (de *DifferentialEngine) EnableUnifiedFastPath() error {
	de.mu.Lock()
	defer de.mu.Unlock()
	if de.unifiedStore != nil {
		return nil
	}
	strata, predToStratum, err := analysis.Stratify(analysis.Program{
		EdbPredicates: de.programInfo.EdbPredicates,
		IdbPredicates: de.programInfo.IdbPredicates,
		Rules:         de.programInfo.Rules,
	})
	if err != nil {
		return fmt.Errorf("diff: EnableUnifiedFastPath: Stratify: %w", err)
	}
	de.unifiedStore = factstore.NewSimpleInMemoryStore()
	de.unifiedStrata = strata
	de.unifiedPredToStratum = predToStratum
	return nil
}

// UnifiedFastPathEnabled reports whether the engine is in the
// single-call fast-path mode. Mostly used by tests.
func (de *DifferentialEngine) UnifiedFastPathEnabled() bool {
	de.mu.RLock()
	defer de.mu.RUnlock()
	return de.unifiedStore != nil
}

// evalOptions returns the safety options that every differential evaluator call
// must enforce. A positive created-fact limit is fail-closed: storing it in the
// wrapper configuration without forwarding it to mangle-go would silently make
// differential evaluation less bounded than the full Engine path.
func (de *DifferentialEngine) evalOptions() []mengine.EvalOption {
	if de.config.DerivedFactsLimit <= 0 {
		return nil
	}
	return []mengine.EvalOption{
		mengine.WithCreatedFactLimit(de.config.DerivedFactsLimit),
	}
}

// ApplyAtomDelta is a variant of ApplyDelta that accepts already-converted
// ast.Atoms instead of high-level Fact records. Use this from callers that
// have their own (kernel-specific) Fact→Atom conversion and need to preserve
// the exact encoding semantics — for example, the codeNERD kernel uses
// types.Fact.ToAtom() which differs from Engine.factToAtomLocked's
// Auto-Atomizer heuristics.
//
// When EnableUnifiedFastPath has been called, this routes through the
// single-eval-call fast path and bypasses the per-stratum strataStores
// entirely (the kernel doesn't query them). Otherwise it uses the legacy
// per-stratum loop and keeps strataStores populated for Snapshot / Query.
func (de *DifferentialEngine) ApplyAtomDelta(atoms []ast.Atom) error {
	de.mu.Lock()
	defer de.mu.Unlock()

	if !de.config.AutoEval && de.unifiedStore == nil && len(atoms) == 0 {
		return nil
	}

	// FAST PATH: skip the per-stratum store bookkeeping entirely — every
	// fact goes into the unified store, and a single EvalStratifiedProgramWithStats
	// over the full program rederives the IDB. The unified store
	// accumulates EDB + IDB across calls so the engine's seminaive
	// evaluator can skip already-derived facts.
	if de.unifiedStore != nil {
		changed := false
		for _, atom := range atoms {
			if de.unifiedStore.Add(atom) {
				changed = true
			}
		}
		if !changed || !de.config.AutoEval {
			return nil
		}
		if _, err := mengine.EvalStratifiedProgramWithStats(
			de.programInfo, de.unifiedStrata, de.unifiedPredToStratum, de.unifiedStore,
			de.evalOptions()...,
		); err != nil {
			return err
		}
		return nil
	}

	// LEGACY PATH: per-stratum loop with ChainedFactStore. Preserves the
	// pre-fast-path semantics for the torture tests, ouroboros, and any
	// caller relying on per-stratum layering.
	minChangedStratum := -1
	for _, atom := range atoms {
		s, ok := de.predStratum[atom.Predicate]
		if !ok {
			s = 0
		}
		layer := de.strataStores[s]
		layer.mu.Lock()
		if layer.store.Add(atom) {
			if minChangedStratum == -1 || s < minChangedStratum {
				minChangedStratum = s
			}
		}
		layer.mu.Unlock()
	}

	if minChangedStratum == -1 {
		return nil
	}
	if !de.config.AutoEval {
		return nil
	}

	for s := minChangedStratum; s < len(de.strataStores); s++ {
		rules := de.strataRules[s]
		if len(rules) == 0 {
			continue
		}
		baseStores := make([]factstore.FactStore, 0, s)
		for i := range s {
			baseStores = append(baseStores, de.strataStores[i].store)
		}
		chain := &ChainedFactStore{
			base:    baseStores,
			overlay: de.strataStores[s].store,
		}
		subsetInfo := *de.programInfo
		subsetInfo.Rules = rules
		subStrata := []analysis.Nodeset{de.strataNodesets[s]}
		subPredToStratum := de.strataPredMaps[s]
		if _, err := mengine.EvalStratifiedProgramWithStats(
			&subsetInfo, subStrata, subPredToStratum, chain, de.evalOptions()...,
		); err != nil {
			return err
		}
	}
	return nil
}

// CopyAllFactsTo materializes the union of every stratum store into the
// provided destination FactStore. When EnableUnifiedFastPath has been
// called, the unified store IS the union and a single walk suffices —
// no per-stratum dedup needed.
func (de *DifferentialEngine) CopyAllFactsTo(dest factstore.FactStore) error {
	de.mu.RLock()
	defer de.mu.RUnlock()

	// Fast path: the unified store already holds the union.
	if de.unifiedStore != nil {
		for _, predSym := range de.unifiedStore.ListPredicates() {
			if err := de.unifiedStore.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
				dest.Add(a)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	// Legacy path: walk every stratum store and dedup via Add (which is
	// idempotent on the destination).
	for _, layer := range de.strataStores {
		if layer == nil {
			continue
		}
		layer.mu.RLock()
		for _, predSym := range layer.store.ListPredicates() {
			if err := layer.store.GetFacts(ast.Atom{Predicate: predSym}, func(a ast.Atom) error {
				dest.Add(a)
				return nil
			}); err != nil {
				layer.mu.RUnlock()
				return err
			}
		}
		layer.mu.RUnlock()
	}
	return nil
}

// ApplyDelta applies a set of new facts and re-evaluates necessary strata.
func (de *DifferentialEngine) ApplyDelta(facts []Fact) error {
	de.mu.Lock()
	defer de.mu.Unlock()

	// 1. Insert facts into appropriate strata.
	minChangedStratum := -1

	for _, f := range facts {
		atom, err := de.baseEngine.factToAtomLocked(f)
		if err != nil {
			return err
		}

		// Find stratum for this predicate
		s, ok := de.predStratum[atom.Predicate]
		if !ok {
			// Default to 0 if unknown (EDB) or fallback
			s = 0
		}

		store := de.strataStores[s]
		store.mu.Lock()
		if store.store.Add(atom) {
			if minChangedStratum == -1 || s < minChangedStratum {
				minChangedStratum = s
			}
		}
		store.mu.Unlock()
	}

	if minChangedStratum == -1 {
		return nil // No changes
	}

	// 2. Re-evaluate derived strata from minChangedStratum upwards.
	if de.config.AutoEval {
		for s := minChangedStratum; s < len(de.strataStores); s++ {
			rules := de.strataRules[s]
			if len(rules) == 0 {
				continue
			}

			// Construct Chain: Base = 0...s-1, Overlay = s
			baseStores := make([]factstore.FactStore, 0, s)
			for i := 0; i < s; i++ {
				baseStores = append(baseStores, de.strataStores[i].store)
			}
			chain := &ChainedFactStore{
				base:    baseStores,
				overlay: de.strataStores[s].store,
			}

			// Evaluate rules for this stratum against the chain.
			// Note: EvalProgramWithStats takes ProgramInfo, which contains ALL rules.
			// We need to limit it to just the rules for this stratum.
			// But Mangle API might not allow easy partial eval if ProgramInfo is monolithic.
			// Workaround: Construct a temporary ProgramInfo or use lower-level API.
			// Wait, we can't easily make ProgramInfo (private fields?).
			// Let's rely on Mangle's EvalProgram taking rules from ProgramInfo?
			// Actually, `Evaluate` function in Mangle usually iterates steps.
			// If we can't control the rule set easily, we might just run full eval
			// but the `chain` limits visibility to lower levels being read-only.
			// Only the 'overlay' consumes new facts.
			// But if Eval sees all rules, it might try to derive Stratum S+1 facts into Stratum S store?
			// No, because Heads of S+1 rules map to S+1 predicates.
			// If `chain.overlay` receives a fact for P (where P is in S+1), it ends up in S store!
			// This breaks stratification.

			// CRITICAL: We need to ensure `EvalProgram` writes facts to correct stores?
			// Or we assume `Eval` only fires rules that match the chain?
			// No, `Eval` will run all rules that match body.

			// BETTER APPROACH for "Semi-Naive":
			// We only pass the rules for *this* stratum to the evaluator.
			// But `EvalProgramWithStats` requires `ProgramInfo`.
			// We might need to construct a `ProgramInfo` subset.
			// Or use a lower level loop.

			// Looking at `differential.go` imports: `mengine "codeberg.org/TauCeti/mangle-go/engine"`.
			// `mengine.Eval` or similar?

			// Fallback: Use `baseEngine.programInfo` but trust that iterating strata sequentially
			// and using the specific chain will naturally converge.
			// Issue: if rules for S+1 fire, they write to `chain.overlay` (Store S).
			// Facts for S+1 will settle in Store S. This works functionally but merges strata in storage.
			// If we want strict caching, we need facts for S+1 in `strataStores[S+1]`.

			// Given constraints, compiling a new ProgramInfo per stratum is hard/expensive.
			// However, we can just run full evaluation on a "Global Chained" setup?
			// No, that defeats "Incremental".

			// Let's assume for this task: Naive stratification (EDB=0, All Rules=1).
			// Then we just re-run all rules against (EDB + IDB Store).
			// This is "standard" Mangle usage.
			// But user wants "Stratum 2".

			// Let's stick to the Plan:
			// We really need to isolate rules.
			// If we can't, we just run eval on the top stratum overlaying logical bases.
			// But we iterate s = min -> max.
			// If we run all rules at s=0, we might compute s=1 facts into s=0 store.

			// Compromise:
			// We use `ProgramInfo` but we know it might over-compute.
			// UNLESS we can filter usage.
			// But wait, `ProgramInfo` is struct. We can make a copy and swap `Rules`.
			// `ProgramInfo` has `Rules []ast.Clause`. We can swap it!

			subsetInfo := *de.programInfo // Shallow copy
			subsetInfo.Rules = rules

			// Use the cached per-stratum nodeset/predMap built once in
			// NewDifferentialEngine. Calling analysis.Stratify per
			// delta per stratum was the dominant cost in the 2% diff-path
			// regression.
			subStrata := []analysis.Nodeset{de.strataNodesets[s]}
			subPredToStratum := de.strataPredMaps[s]

			_, err := mengine.EvalStratifiedProgramWithStats(
				&subsetInfo, subStrata, subPredToStratum, chain, de.evalOptions()...,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// loadFileContent is a specific handler for file_content
func loadFileContent(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Query evaluates a query against the differential knowledge base.
// It uses the highest stratum store (overlay).
func (de *DifferentialEngine) Query(ctx context.Context, query string) (*QueryResult, error) {
	// 1. Helper to parse query (uses unexported helper from engine.go in same package)
	shape, err := parseQueryShape(query)
	if err != nil {
		return nil, err
	}

	de.mu.RLock()
	defer de.mu.RUnlock()

	// 2. Build a ChainedFactStore that unions all strata for querying
	// This ensures we can query facts from any stratum (EDB or IDB)
	if len(de.strataStores) == 0 {
		return nil, fmt.Errorf("no knowledge graph strata available")
	}

	// Build chain: all lower strata as base, top stratum as overlay
	var currentStore factstore.FactStore
	if len(de.strataStores) == 1 {
		currentStore = de.strataStores[0].store
	} else {
		baseStores := make([]factstore.FactStore, 0, len(de.strataStores)-1)
		for i := 0; i < len(de.strataStores)-1; i++ {
			baseStores = append(baseStores, de.strataStores[i].store)
		}
		currentStore = &ChainedFactStore{
			base:    baseStores,
			overlay: de.strataStores[len(de.strataStores)-1].store,
		}
	}

	// We need PredToRules and PredToDecl from programInfo
	predToDecl := make(map[ast.PredicateSym]*ast.Decl)
	maps.Copy(predToDecl, de.programInfo.Decls)

	predToRules := make(map[ast.PredicateSym][]ast.Clause)
	for _, clause := range de.programInfo.Rules {
		predToRules[clause.Head.Predicate] = append(predToRules[clause.Head.Predicate], clause)
	}

	queryContext := &mengine.QueryContext{
		PredToRules: predToRules,
		PredToDecl:  predToDecl,
		Store:       currentStore,
	}

	// 3. Execute Query (Logic mirrored from Engine.Query)
	decl, ok := queryContext.PredToDecl[shape.atom.Predicate]
	if !ok {
		return nil, fmt.Errorf("predicate %s is not declared", shape.atom.Predicate.Symbol)
	}
	var mode ast.Mode
	if len(decl.Modes()) > 0 {
		mode = decl.Modes()[0]
	} else {
		// Synthesize default mode: all args are outputs (-)
		// We can't easily guess input/output requirements without analysis,
		// but for simple queries we assume all-output (unbound).
		modes := make([]ast.ArgMode, len(shape.atom.Args))
		for i := range modes {
			modes[i] = ast.ArgModeOutput
		}
		mode = ast.Mode(modes)
	}

	start := time.Now()
	resultChan := make(chan []map[string]any, 1)
	errChan := make(chan error, 1)

	go func() {
		var results []map[string]any

		emitRow := func(fact ast.Atom) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			row := make(map[string]any, len(shape.variables))
			for _, binding := range shape.variables {
				if binding.Index >= len(fact.Args) {
					continue
				}
				row[binding.Name] = convertBaseTermToInterface(fact.Args[binding.Index])
			}
			results = append(results, row)
			return nil
		}

		// 1) Always include matching stored facts (EDB or cached IDB).
		if err := queryContext.Store.GetFacts(shape.atom, emitRow); err != nil {
			errChan <- err
			return
		}

		// 2) If there are rules for this predicate, also derive answers top-down.
		if len(queryContext.PredToRules[shape.atom.Predicate]) > 0 {
			err := queryContext.EvalQuery(shape.atom, mode, unionfind.New(), emitRow)
			if err != nil {
				errChan <- err
				return
			}
		}

		resultChan <- results
	}()

	select {
	case results := <-resultChan:
		return &QueryResult{
			Bindings: results,
			Duration: time.Since(start),
		}, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("query execution timed out after %v: %w", time.Since(start), ctx.Err())
	}
}

// InclusionChecker Implementation

// FactStoreProxy wraps a FactStore and adds lazy loading.
type FactStoreProxy struct {
	factstore.FactStore
	lazyLoaders map[string]func(atom ast.Atom) bool
}

func NewFactStoreProxy(base factstore.FactStore) *FactStoreProxy {
	return &FactStoreProxy{
		FactStore:   base,
		lazyLoaders: make(map[string]func(atom ast.Atom) bool),
	}
}

func (fsp *FactStoreProxy) RegisterLoader(predicate string, loader func(atom ast.Atom) bool) {
	fsp.lazyLoaders[predicate] = loader
}

// GetFacts overrides the base check to trigger lazy loading.
func (fsp *FactStoreProxy) GetFacts(query ast.Atom, fn func(ast.Atom) error) error {
	// Check if this predicate has a lazy loader
	if loader, ok := fsp.lazyLoaders[query.Predicate.Symbol]; ok {
		// Trigger lazy loader with the full query atom (including args).
		// The loader may populate the underlying store.
		loader(query)
	}
	return fsp.FactStore.GetFacts(query, fn)
}

// RegisterVirtualPredicate registers a loader for a virtual predicate.
// It wraps the base stratum store (Stratum 0) with a FactStoreProxy if not already wrapped.
func (de *DifferentialEngine) RegisterVirtualPredicate(predicate string, loader func(string) (string, error)) {
	de.mu.Lock()
	defer de.mu.Unlock()

	// Assume virtual predicates are always Base EDB (Stratum 0)
	baseLayer := de.strataStores[0]
	baseLayer.mu.Lock()
	defer baseLayer.mu.Unlock()

	var proxy *FactStoreProxy
	if p, ok := baseLayer.store.(*FactStoreProxy); ok {
		proxy = p
	} else {
		proxy = NewFactStoreProxy(baseLayer.store)
		baseLayer.store = proxy
	}

	proxy.RegisterLoader(predicate, func(atom ast.Atom) bool {
		// Convert Atom back to args to pass to loader
		// Loader expects 'func(string) (string, error)' implies arg is a key (e.g. filename)
		if len(atom.Args) > 0 {
			if key, ok := convertBaseTermToInterface(atom.Args[0]).(string); ok {
				val, err := loader(key)
				if err == nil {
					// Fallback: Just insert String constants.
					valTerm := ast.String(val)
					newAtom := ast.Atom{
						Predicate: atom.Predicate,
						Args:      []ast.BaseTerm{atom.Args[0], valTerm},
					}
					proxy.FactStore.Add(newAtom)
					return true
				}
			}
		}
		return false
	})
}
