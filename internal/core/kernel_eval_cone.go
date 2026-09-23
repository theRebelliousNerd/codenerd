package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

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
// A stratum is a strongly connected component, so a cone never splits one: if
// one member depends on a written predicate, every member does.

// coneIndex is the rule-dependency graph of one programInfo, by predicate
// symbol name. Names, not PredicateSym: a Fact carries a name, and a name with
// two arities is treated as one node, which only ever widens a cone.
type coneIndex struct {
	consumers map[string][]string // body predicate -> heads of the rules reading it
	heads     map[string]struct{} // every predicate some rule derives
	volatile  []string            // heads with an external() premise, re-derived every time
	rules     int
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

func buildConeIndex(info *analysis.ProgramInfo) *coneIndex {
	idx := &coneIndex{
		consumers: make(map[string][]string),
		heads:     make(map[string]struct{}),
		rules:     len(info.Rules),
	}
	external := make(map[string]struct{})
	for sym, decl := range info.Decls {
		if decl != nil && decl.IsExternal() {
			external[sym.Symbol] = struct{}{}
		}
	}
	edges := make(map[string]map[string]struct{})
	volatile := make(map[string]struct{})
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
		}
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
}

// evaluateConeLocked re-derives the cone of the written predicates in place.
// It returns errConeIneligible when the full path must run instead; any other
// error leaves k.store half-rebuilt, and the caller falls back to the full
// path, which builds a fresh store and so repairs it.
func (k *RealKernel) evaluateConeLocked() error {
	if k.dirtyAll || len(k.dirtyPreds) == 0 || !k.initialized || k.store == nil ||
		k.policyDirty || k.programInfo == nil || k.cone == nil || k.atomCacheStale ||
		k.proofRecorder != nil {
		return errConeIneligible
	}
	remover, ok := k.store.(factstore.FactStoreWithRemove)
	if !ok {
		return errConeIneligible
	}
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate.cone")
	defer timer.Stop()

	cone := k.cone.closure(k.dirtyPreds)
	touched := make(map[string]struct{}, len(cone)+len(k.dirtyPreds))
	for pred := range k.dirtyPreds {
		touched[pred] = struct{}{}
	}
	for pred := range cone {
		touched[pred] = struct{}{}
	}

	// Remove every fact, extensional and derived, of every touched predicate.
	var doomed []ast.Atom
	for _, sym := range k.store.ListPredicates() {
		if _, ok := touched[sym.Symbol]; !ok {
			continue
		}
		if err := k.store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			doomed = append(doomed, a)
			return nil
		}); err != nil {
			return fmt.Errorf("cone: list %s: %w", sym.Symbol, err)
		}
	}
	for _, atom := range doomed {
		remover.Remove(atom)
	}

	// Put back what the EDB holds for them now.
	restored := 0
	for _, f := range k.facts {
		if _, ok := touched[f.Predicate]; !ok {
			continue
		}
		atom, err := k.factToAtomLocked(f)
		if err != nil {
			return fmt.Errorf("cone: convert %s: %w", f.Predicate, err)
		}
		k.store.Add(atom)
		restored++
	}

	// The sub-program: the cone's rules, over the cone's strata in order.
	rules := make([]ast.Clause, 0, 256)
	for _, rule := range k.programInfo.Rules {
		if _, ok := cone[rule.Head.Predicate.Symbol]; ok {
			rules = append(rules, rule)
		}
	}
	var strata []analysis.Nodeset
	predToStratum := make(map[ast.PredicateSym]int)
	for _, stratum := range k.strata {
		inCone := false
		for sym := range stratum {
			if _, ok := cone[sym.Symbol]; ok {
				inCone = true
				break
			}
		}
		if !inCone {
			continue
		}
		for sym := range stratum {
			predToStratum[sym] = len(strata)
		}
		strata = append(strata, stratum)
	}

	// Run the engine even when the cone holds no rule: it is what re-adds a
	// program fact of a written predicate, removed above with the rest.
	sub := &analysis.ProgramInfo{
		EdbPredicates:    k.programInfo.EdbPredicates,
		IdbPredicates:    k.programInfo.IdbPredicates,
		InitialFacts:     k.programInfo.InitialFacts,
		InitialFactTimes: k.programInfo.InitialFactTimes,
		Rules:            rules,
		Decls:            k.programInfo.Decls,
	}
	opts := append([]engine.EvalOption{engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked())},
		k.externalPredicateOptionsLocked()...)
	if _, err := engine.EvalStratifiedProgramWithStats(sub, strata, predToStratum, k.store, opts...); err != nil {
		return fmt.Errorf("cone: fixpoint: %w", err)
	}

	written := make([]string, 0, len(k.dirtyPreds))
	for pred := range k.dirtyPreds {
		written = append(written, pred)
	}
	sort.Strings(written)
	logging.KernelDebug("evaluate.cone: written=[%s] cone=%d predicates, %d/%d rules, %d strata, %d facts removed, %d restored",
		strings.Join(written, ","), len(cone), len(rules), k.cone.rules, len(strata), len(doomed), restored)
	return nil
}
