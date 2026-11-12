package core

import (
	"fmt"
	"sort"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// =============================================================================
// PROOF SLICES: RECORDING ON DEMAND, FOR ONE GOAL
// =============================================================================
//
// A derivation recorder installed on evaluate() records every solution of
// every rule in every semi-naive round. On a kernel that holds the world model
// that is a full fixpoint per write (the incremental paths cannot feed a
// recorder) and close to a gigabyte of events, and it was paid on every write
// while recording was on, whether or not anyone asked "why". Measured
// 2026-11-12 on codeNERD's own world model (575k EDB facts): a per-turn write
// took 3 ms with recording off and 8 s with it on, and the heap grew by about
// 900 MiB; a live deliberation spent minutes of kernel lock time this way.
//
// A proof of a goal only needs the derivations of the predicates the goal
// depends on. So evaluation never records, and Explain records what it
// needs when it is asked: the slice of the program upstream of the goal's
// predicate (every rule whose head the goal reaches through rule bodies,
// negation, aggregation, deferred premises and lattice merges), evaluated
// with a recorder over the EDB rows of exactly those predicates.
//
// The slice's model equals the kernel's on every slice predicate. A stratified
// program's model restricted to a set of predicates closed under dependency
// is determined by the rules and EDB of that set alone, and semi-naive
// evaluation of a predicate reads only its dependencies, so the slice's
// rounds produce the same solutions, and the recorder the same events, for
// those predicates as a recorded full pass would.
// TestProofSlice_MatchesTheFullRecordedPass checks the proofs against a
// recorded full evaluation of the whole program.
//
// A slice is a recording of one evaluation: it is keyed by the kernel's write
// generation and program, and is dropped the moment either changes, so a proof
// can never describe an older store than the one the kernel answers from.

// proofSliceCacheMax bounds the goals one evaluation keeps recordings for.
const proofSliceCacheMax = 16

// proofSlice is one goal predicate's recorded slice of one evaluation.
type proofSlice struct {
	recorder *provenance.MemoryRecorder
	store    factstore.FactStoreWithRemove
	rules    int
	edbRows  int
	elapsed  time.Duration
}

// proofSliceCache holds the slices recorded for the current evaluation.
type proofSliceCache struct {
	generation uint64
	program    *analysis.ProgramInfo
	byPred     map[string]*proofSlice
}

// proofSliceLocked returns the recorded slice for goal's predicate, recording
// it if this evaluation has none yet. The kernel must be evaluated. Caller
// holds k.mu exclusively.
func (k *RealKernel) proofSliceLocked(goal ast.Atom) (*proofSlice, error) {
	if k.programInfo == nil || k.cone == nil {
		return nil, fmt.Errorf("Explain: kernel program not built")
	}
	gen := k.generation.Load()
	if k.proofs.generation != gen || k.proofs.program != k.programInfo || k.proofs.byPred == nil {
		k.proofs = proofSliceCache{generation: gen, program: k.programInfo, byPred: make(map[string]*proofSlice)}
	}
	pred := goal.Predicate.Symbol
	if s, ok := k.proofs.byPred[pred]; ok {
		return s, nil
	}
	s, err := k.recordProofSliceLocked(pred)
	if err != nil {
		return nil, err
	}
	if len(k.proofs.byPred) >= proofSliceCacheMax {
		k.proofs.byPred = make(map[string]*proofSlice)
	}
	k.proofs.byPred[pred] = s
	return s, nil
}

// recordProofSliceLocked evaluates the slice of the program upstream of pred
// with a derivation recorder, over a fresh store holding only that slice's
// EDB rows and program facts.
func (k *RealKernel) recordProofSliceLocked(pred string) (*proofSlice, error) {
	started := time.Now()
	up := k.cone.upstream(pred)

	ruleIdx := make([]int, 0, 64)
	stratumIdx := make(map[int]struct{})
	for p := range up {
		ruleIdx = append(ruleIdx, k.cone.rulesByHead[p]...)
		for _, s := range k.cone.strataByPred[p] {
			stratumIdx[s] = struct{}{}
		}
	}
	sort.Ints(ruleIdx)
	rules := make([]ast.Clause, len(ruleIdx))
	for i, idx := range ruleIdx {
		rules[i] = k.cone.rules[idx]
	}
	order := make([]int, 0, len(stratumIdx))
	for s := range stratumIdx {
		order = append(order, s)
	}
	sort.Ints(order)
	// A stratum is a layer of independent components; the slice keeps its own
	// members of each, and a component's members are all upstream or none is.
	strata := make([]analysis.Nodeset, len(order))
	predToStratum := make(map[ast.PredicateSym]int)
	for i, s := range order {
		strata[i] = coneMembers(k.cone.strata[s], up)
		for sym := range strata[i] {
			predToStratum[sym] = i
		}
	}

	var initial []ast.Atom
	var initialTimes []*ast.Interval
	for i, fact := range k.programInfo.InitialFacts {
		if _, ok := up[fact.Predicate.Symbol]; !ok {
			continue
		}
		initial = append(initial, fact)
		if i < len(k.programInfo.InitialFactTimes) {
			initialTimes = append(initialTimes, k.programInfo.InitialFactTimes[i])
		}
	}
	if len(initialTimes) != len(initial) {
		initialTimes = nil
	}
	sub := &analysis.ProgramInfo{
		EdbPredicates:    k.programInfo.EdbPredicates,
		IdbPredicates:    k.programInfo.IdbPredicates,
		InitialFacts:     initial,
		InitialFactTimes: initialTimes,
		Rules:            rules,
		Decls:            k.programInfo.Decls,
	}

	atoms, err := k.edbAtomsOfLocked(up)
	if err != nil {
		return nil, err
	}
	store := newEvaluationFactStore(len(atoms))
	for _, atom := range atoms {
		store.Add(atom)
	}
	recorder := provenance.NewMemoryRecorder()
	opts := append([]engine.EvalOption{
		engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked()),
		engine.WithDerivationRecorder(recorder),
	}, k.externalPredicateOptionsLocked()...)
	if _, err := engine.EvalStratifiedProgramWithStats(sub, strata, predToStratum, store, opts...); err != nil {
		return nil, fmt.Errorf("Explain: recording the proof slice of %s: %w", pred, err)
	}
	s := &proofSlice{recorder: recorder, store: store, rules: len(rules), edbRows: len(atoms), elapsed: time.Since(started)}
	logging.KernelDebug("Explain: recorded the proof slice of %s: %d predicates, %d rules, %d EDB rows, %d events in %v",
		pred, len(up), s.rules, s.edbRows, len(recorder.Events()), s.elapsed)
	return s, nil
}

// edbAtomsOfLocked is every EDB row of the named predicates as its store atom.
func (k *RealKernel) edbAtomsOfLocked(preds map[string]struct{}) ([]ast.Atom, error) {
	var atoms []ast.Atom
	if k.factIndex != nil {
		names := make([]string, 0, len(preds))
		for p := range preds {
			names = append(names, p)
		}
		sort.Strings(names)
		for _, p := range names {
			held, err := k.heldAtomsLocked(p)
			if err != nil {
				return nil, err
			}
			atoms = append(atoms, held...)
		}
		return atoms, nil
	}
	for f := range k.facts.all() {
		if _, ok := preds[f.Predicate]; !ok {
			continue
		}
		atom, err := k.factToAtomLocked(f)
		if err != nil {
			return nil, err
		}
		atoms = append(atoms, atom)
	}
	return atoms, nil
}

// upstream is pred and every predicate its derivation can read: rule bodies
// (positive, negated, temporal, aggregated), deferred premises and lattice
// merge predicates, transitively. Built once per program.
func (c *coneIndex) upstream(pred string) map[string]struct{} {
	c.producersOnce.Do(c.buildProducers)
	out := map[string]struct{}{pred: {}}
	queue := []string{pred}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, dep := range c.producers[p] {
			if _, seen := out[dep]; !seen {
				out[dep] = struct{}{}
				queue = append(queue, dep)
			}
		}
	}
	return out
}

// buildProducers inverts the dependency edges: head -> what it reads.
func (c *coneIndex) buildProducers() {
	deps := make(map[string]map[string]struct{})
	add := func(head, dep string) {
		if deps[head] == nil {
			deps[head] = make(map[string]struct{})
		}
		deps[head][dep] = struct{}{}
	}
	for body, heads := range c.consumers {
		for _, head := range heads {
			add(head, body)
		}
	}
	for pred, needs := range c.topDown {
		for _, dep := range needs {
			add(pred, dep)
		}
	}
	c.producers = make(map[string][]string, len(deps))
	for head, set := range deps {
		c.producers[head] = predicateNames(set)
	}
}
