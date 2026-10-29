package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// =============================================================================
// CONE EVALUATION
// =============================================================================
// A write to predicate P can only change the derived predicates that depend on
// P, transitively: its cone. evaluate() used to rebuild the whole store and run
// every rule on every write. Measured 2026-09-21 on a 24-task campaign: 4,161
// evaluations, 4,033 s summed, against a 59k-fact world shard with 2,187 rules,
// where the hot writes (pending_action, action_verified) reach 6-8% of the rules
// and three more (tool_execution, validation_method_used, world_model_heartbeat)
// reach none.
//
// Soundness. The differential path deleted in S23 kept derived facts across
// evaluations and only ever added to them, so a negated premise turning true
// un-derived nothing and an aggregate gained a second value. This path removes
// every fact of every predicate in the cone before re-deriving it, which is the
// condition evaluate()'s comment sets for any incremental evaluator. A rule
// outside the cone reads no written predicate and no cone predicate, so its
// inputs are unchanged and its old output is still its fixpoint.
//
// A cone never splits a strongly connected component: if one member depends
// on a written predicate, every member does. A stratum is a layer of such
// components that read nothing of each other (mangle.LayerStrata), so the
// cone's members of a layer evaluate as they would alone.

// coneIndex is the rule-dependency graph of one programInfo, by predicate
// symbol name. Names, not PredicateSym: a Fact carries a name, and a name with
// two arities is treated as one node, which only ever widens a cone.
//
// It also carries the access paths a cone plan is cut through
// (kernel_cone_plan.go). All of it is immutable once built except the plan
// cache, which locks; it is shared by every kernel and clone compiled from the
// same program.
type coneIndex struct {
	consumers map[string][]string // body predicate -> heads of the rules reading it
	heads     map[string]struct{} // every predicate some rule derives
	volatile  []string            // heads with an external() premise, re-derived every time
	// topDown names, per predicate, the predicates whose rules the engine runs
	// top-down while deriving it: a deferred() predicate in a rule body, and a
	// lattice's merge predicate (descr merge([...], "p")), evaluated on every
	// replacement. Neither is derived bottom-up, so a cone that holds only
	// its consumers would run them with no rules: a deferred premise would
	// have no answer, and every lattice row would be kept as incomparable.
	topDown map[string][]string

	rules        []ast.Clause                  // the program's rules, in program order
	strata       []analysis.Nodeset            // the program's strata, in evaluation order
	rulesByHead  map[string][]int              // head name -> indexes into rules, ascending
	strataByPred map[string][]int              // predicate name -> indexes into strata, ascending
	syms         map[string][]ast.PredicateSym // name -> every symbol the program knows under it
	plans        conePlans
}

// errConeIneligible makes evaluate() take the full path. It is not a fault.
var errConeIneligible = errors.New("cone evaluation not eligible")

func premisePredicates(t ast.Term, out map[string]struct{}) {
	switch v := t.(type) {
	case ast.Atom:
		out[v.Predicate.Symbol] = struct{}{}
	case ast.NegAtom:
		out[v.Atom.Predicate.Symbol] = struct{}{}
	case ast.TemporalAtom:
		out[v.Atom.Predicate.Symbol] = struct{}{}
	case ast.TemporalLiteral:
		premisePredicates(v.Literal, out)
	}
}

// buildConeIndex indexes one analyzed, stratified program.
func buildConeIndex(info *analysis.ProgramInfo, strata []analysis.Nodeset, predToStratum map[ast.PredicateSym]int) *coneIndex {
	idx := &coneIndex{
		consumers:    make(map[string][]string),
		heads:        make(map[string]struct{}),
		rules:        info.Rules,
		strata:       strata,
		rulesByHead:  make(map[string][]int),
		strataByPred: make(map[string][]int),
		syms:         make(map[string][]ast.PredicateSym),
	}
	for i, rule := range info.Rules {
		head := rule.Head.Predicate.Symbol
		idx.rulesByHead[head] = append(idx.rulesByHead[head], i)
	}
	byName := make(map[string]map[int]struct{})
	for sym, stratum := range predToStratum {
		if byName[sym.Symbol] == nil {
			byName[sym.Symbol] = make(map[int]struct{}, 1)
		}
		byName[sym.Symbol][stratum] = struct{}{}
	}
	for name, set := range byName {
		list := make([]int, 0, len(set))
		for s := range set {
			list = append(list, s)
		}
		sort.Ints(list)
		idx.strataByPred[name] = list
	}
	known := make(map[ast.PredicateSym]struct{}, len(info.Decls)+len(info.IdbPredicates))
	for sym := range info.Decls {
		known[sym] = struct{}{}
	}
	for sym := range info.IdbPredicates {
		known[sym] = struct{}{}
	}
	for sym := range info.EdbPredicates {
		known[sym] = struct{}{}
	}
	for _, rule := range info.Rules {
		known[rule.Head.Predicate] = struct{}{}
	}
	for _, fact := range info.InitialFacts {
		known[fact.Predicate] = struct{}{}
	}
	for sym := range known {
		idx.syms[sym.Symbol] = append(idx.syms[sym.Symbol], sym)
	}
	external := make(map[string]struct{})
	for sym, decl := range info.Decls {
		if decl != nil && decl.IsExternal() {
			external[sym.Symbol] = struct{}{}
		}
	}
	edges := make(map[string]map[string]struct{})
	volatile := make(map[string]struct{})
	deferred := make(map[string]struct{})
	topDown := make(map[string]map[string]struct{})
	needs := func(pred, dep string) {
		if topDown[pred] == nil {
			topDown[pred] = make(map[string]struct{})
		}
		topDown[pred][dep] = struct{}{}
	}
	for sym, decl := range info.Decls {
		if decl == nil {
			continue
		}
		if decl.DeferredPredicate() {
			deferred[sym.Symbol] = struct{}{}
		}
		if len(decl.FunDeps()) == 1 {
			if _, merge := decl.MergePredicate(); merge.Symbol != "" {
				needs(sym.Symbol, merge.Symbol)
				// A change to what the merge reads changes which row the
				// lattice keeps: the lattice is downstream of its merge.
				if edges[merge.Symbol] == nil {
					edges[merge.Symbol] = make(map[string]struct{})
				}
				edges[merge.Symbol][sym.Symbol] = struct{}{}
			}
		}
	}
	for _, rule := range info.Rules {
		head := rule.Head.Predicate.Symbol
		idx.heads[head] = struct{}{}
		body := make(map[string]struct{})
		for _, premise := range rule.Premises {
			premisePredicates(premise, body)
		}
		for pred := range body {
			if edges[pred] == nil {
				edges[pred] = make(map[string]struct{})
			}
			edges[pred][head] = struct{}{}
			if _, ok := external[pred]; ok {
				volatile[head] = struct{}{}
			}
			if _, ok := deferred[pred]; ok {
				needs(head, pred)
			}
		}
	}
	idx.topDown = make(map[string][]string, len(topDown))
	for pred, deps := range topDown {
		idx.topDown[pred] = predicateNames(deps)
	}
	for pred, heads := range edges {
		list := make([]string, 0, len(heads))
		for head := range heads {
			list = append(list, head)
		}
		sort.Strings(list)
		idx.consumers[pred] = list
	}
	for head := range volatile {
		idx.volatile = append(idx.volatile, head)
	}
	sort.Strings(idx.volatile)
	return idx
}

// closure returns every derived predicate reachable from the written ones,
// plus the written ones that are themselves derived, plus the volatile heads
// and everything downstream of them.
func (c *coneIndex) closure(written map[string]struct{}) map[string]struct{} {
	cone := make(map[string]struct{})
	queue := make([]string, 0, len(written)+len(c.volatile))
	for pred := range written {
		if _, ok := c.heads[pred]; ok {
			cone[pred] = struct{}{}
		}
		queue = append(queue, pred)
	}
	for _, head := range c.volatile {
		cone[head] = struct{}{}
		queue = append(queue, head)
	}
	for len(queue) > 0 {
		pred := queue[0]
		queue = queue[1:]
		for _, head := range c.consumers[pred] {
			if _, seen := cone[head]; !seen {
				cone[head] = struct{}{}
				queue = append(queue, head)
			}
		}
	}
	return cone
}

// downstream returns every predicate a rule derives, directly or through other
// rules, from facts of pred. Unlike closure it adds no volatile heads: it is
// the question "where can this predicate's facts flow", not "what must be
// re-derived".
func (c *coneIndex) downstream(pred string) map[string]struct{} {
	out := make(map[string]struct{})
	queue := []string{pred}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, head := range c.consumers[p] {
			if _, seen := out[head]; !seen {
				out[head] = struct{}{}
				queue = append(queue, head)
			}
		}
	}
	return out
}

// markDirtyLocked is the only place factsDirty is raised. Naming the written
// predicates keeps the next evaluate() inside their cone; naming none declares
// the write set unknown and the next evaluate() is a full one. Caller holds
// k.mu, or the kernel is not yet shared.
func (k *RealKernel) markDirtyLocked(preds ...string) {
	if len(preds) == 0 {
		k.dirtyAll = true
	} else if !k.dirtyAll {
		if k.dirtyPreds == nil {
			k.dirtyPreds = make(map[string]struct{}, len(preds))
		}
		for _, pred := range preds {
			k.dirtyPreds[pred] = struct{}{}
		}
	}
	k.generation.Add(1)
	k.factsDirty.Store(true)
}

// markPolicyDirtyLocked records that the program itself changed -- a rule, a
// schema, a learned clause, the external predicates -- and so that every
// derived fact is stale. Raising policyDirty alone used to leave factsDirty
// down, and ensureEvaluated reads only factsDirty: a query after AppendPolicy
// answered from the old program until some unrelated write forced an
// evaluation (found 2026-09-22 by TestProseOnly_ARuleIntoAnExecSinkWithdraws
// TheExemption). Caller holds k.mu, or the kernel is not yet shared.
func (k *RealKernel) markPolicyDirtyLocked() {
	k.policyDirty = true
	k.markDirtyLocked()
}

// predicateNames flattens a predicate set for markDirtyLocked. An empty set
// yields no names, which markDirtyLocked reads as "unknown": the safe side.
func predicateNames(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names
}

// clearDirtyLocked resets the write set after a successful evaluate().
func (k *RealKernel) clearDirtyLocked() {
	k.dirtyAll = false
	k.dirtyPreds = nil
	if k.factIndex != nil {
		k.factIndex.delta.reset() // the store now matches the EDB
	}
}

// coneIneligibilityLocked says why the next evaluate() cannot be a cone
// evaluation, or "" when it can. Caller holds k.mu.
func (k *RealKernel) coneIneligibilityLocked() string {
	switch {
	case k.dirtyAll || len(k.dirtyPreds) == 0:
		return "write set unknown"
	case !k.initialized || k.store == nil:
		return "no evaluated store"
	case k.policyDirty || k.programInfo == nil || k.cone == nil:
		return "program changed"
	case k.atomCacheStale:
		return "atoms converted under an older program"
	case k.proofRecorder != nil:
		return "provenance recording"
	case k.factIndex == nil:
		return "no EDB index"
	case !k.factIndex.delta.known:
		return "EDB write delta unknown"
	}
	if _, ok := k.store.(factstore.FactStoreWithRemove); !ok {
		return "store does not support removal"
	}
	return ""
}

// evaluateConeLocked re-derives the cone of the written predicates in place.
// It returns errConeIneligible when the full path must run instead; any other
// error leaves k.store half-rebuilt, and the caller falls back to the full
// path, which builds a fresh store and so repairs it.
//
// On success it fills stats; on failure stats is left for the full path.
func (k *RealKernel) evaluateConeLocked(stats *EvaluationStats) error {
	if k.coneIneligibilityLocked() != "" {
		return errConeIneligible
	}
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate.cone")
	defer timer.Stop()
	prepareStarted := time.Now()

	plan := k.cone.plan(k.dirtyPreds)

	removedCount, restored, listed, err := k.prepareConeStoreLocked(plan)
	if err != nil {
		return err
	}

	// Run the engine even when the cone holds no rule: it is what re-adds a
	// program fact of a written predicate, removed above with the rest.
	sub := &analysis.ProgramInfo{
		EdbPredicates:    k.programInfo.EdbPredicates,
		IdbPredicates:    k.programInfo.IdbPredicates,
		InitialFacts:     k.programInfo.InitialFacts,
		InitialFactTimes: k.programInfo.InitialFactTimes,
		Rules:            plan.rules,
		Decls:            k.programInfo.Decls,
	}
	opts := append([]engine.EvalOption{engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked())},
		k.externalPredicateOptionsLocked()...)
	prepared := time.Since(prepareStarted)
	engineStarted := time.Now()
	if _, err := engine.EvalStratifiedProgramWithStats(sub, plan.strata, plan.predToStratum, k.store, opts...); err != nil {
		return fmt.Errorf("cone: fixpoint: %w", err)
	}
	*stats = EvaluationStats{
		InputFacts: stats.InputFacts, Mode: "cone", StoreKind: storeKind(k.store),
		PrepareDuration: prepared, EngineDuration: time.Since(engineStarted),
		Touched: len(plan.touched), RemovedFacts: removedCount, RestoredFacts: restored, ListedFacts: listed,
		Rules: len(plan.rules), Strata: len(plan.strata),
	}

	written := make([]string, 0, len(k.dirtyPreds))
	for pred := range k.dirtyPreds {
		written = append(written, pred)
	}
	sort.Strings(written)
	logging.KernelDebug("evaluate.cone: written=[%s] cone=%d predicates, %d/%d rules, %d strata, %d facts removed, %d restored",
		strings.Join(written, ","), len(plan.cone), len(plan.rules), len(k.cone.rules), len(plan.strata), removedCount, restored)
	return nil
}
