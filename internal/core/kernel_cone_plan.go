package core

import (
	"sort"
	"strings"
	"sync"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// CONE PLANS
// =============================================================================
// Preparing a cone used to touch the whole world even when the cone was one
// rule: it listed every store predicate, walked every EDB row, and scanned
// every rule and stratum of the program (Docs/audits/mangle-kernel-
// performance.md §4). A plan is that preparation done once per write set and
// program, through access paths built with the program:
//
//   - rulesByHead and strataByPred select the cone's rules and strata from its
//     predicates, in program order, instead of scanning all of them;
//   - syms names every predicate symbol the program can hold under a name, so
//     the cone's old rows are removed by symbol instead of listing the store;
//   - the EDB rows to restore come from the predicate buckets of edbIndex.
//
// A plan holds no facts and no answers: it is the physical shape of the
// re-derivation (which rules, which strata in which order), keyed by the
// program it was cut from (it lives on that program's coneIndex) and the exact
// written-predicate set. Strata keep their original order and are never split:
// a stratum is a strongly connected component, so the soundness argument in
// kernel_eval_cone.go is unchanged.

// conePlanCacheMax bounds the plans one program keeps. Write sets recur (the
// hot predicates are a few dozen), so a small cache holds the working set; on
// overflow it is cleared, like compiledProgramCache.
const conePlanCacheMax = 256

// conePlan is the immutable, shareable preparation of one cone evaluation.
type conePlan struct {
	cone          map[string]struct{} // derived predicates re-derived
	touched       []string            // written ∪ cone, sorted: the predicates whose rows are replaced
	rules         []ast.Clause        // the cone's rules, in program order
	strata        []analysis.Nodeset  // the cone's strata, in program order
	predToStratum map[ast.PredicateSym]int
}

// conePlans is the per-program plan cache. coneIndex is shared by every
// kernel compiled from the same program text, so the cache is locked.
type conePlans struct {
	mu    sync.Mutex
	plans map[string]*conePlan
}

// planKey is the canonical identity of a written-predicate set.
func planKey(written map[string]struct{}) string {
	names := make([]string, 0, len(written))
	for name := range written {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "\x00")
}

// plan returns the cone plan for a written-predicate set, computing it on
// first use. The returned plan is shared and must not be modified.
func (c *coneIndex) plan(written map[string]struct{}) *conePlan {
	key := planKey(written)
	c.plans.mu.Lock()
	if p, ok := c.plans.plans[key]; ok {
		c.plans.mu.Unlock()
		return p
	}
	c.plans.mu.Unlock()

	p := c.buildPlan(written)

	c.plans.mu.Lock()
	if c.plans.plans == nil || len(c.plans.plans) >= conePlanCacheMax {
		c.plans.plans = make(map[string]*conePlan, 16)
	}
	c.plans.plans[key] = p
	c.plans.mu.Unlock()
	return p
}

func (c *coneIndex) buildPlan(written map[string]struct{}) *conePlan {
	cone := c.closure(written)
	touchedSet := make(map[string]struct{}, len(cone)+len(written))
	for pred := range written {
		touchedSet[pred] = struct{}{}
	}
	ruleIdx := make([]int, 0, 64)
	stratumIdx := make(map[int]struct{})
	for pred := range cone {
		touchedSet[pred] = struct{}{}
		ruleIdx = append(ruleIdx, c.rulesByHead[pred]...)
		for _, s := range c.strataByPred[pred] {
			stratumIdx[s] = struct{}{}
		}
	}
	// The rules the cone's rules run top-down (deferred premises, lattice
	// merges), and theirs in turn. They join the program the engine sees, not
	// the cone: nothing derives them bottom-up, so they hold no rows to
	// replace and need no stratum.
	for pred := range c.topDownClosure(cone) {
		if _, inCone := cone[pred]; !inCone {
			ruleIdx = append(ruleIdx, c.rulesByHead[pred]...)
		}
	}
	touched := make([]string, 0, len(touchedSet))
	for pred := range touchedSet {
		touched = append(touched, pred)
	}
	sort.Strings(touched)

	sort.Ints(ruleIdx)
	rules := make([]ast.Clause, len(ruleIdx))
	for i, idx := range ruleIdx {
		rules[i] = c.rules[idx]
	}

	order := make([]int, 0, len(stratumIdx))
	for s := range stratumIdx {
		order = append(order, s)
	}
	sort.Ints(order)
	strata := make([]analysis.Nodeset, len(order))
	predToStratum := make(map[ast.PredicateSym]int)
	for i, s := range order {
		strata[i] = c.strata[s]
		for sym := range c.strata[s] {
			predToStratum[sym] = i
		}
	}
	return &conePlan{cone: cone, touched: touched, rules: rules, strata: strata, predToStratum: predToStratum}
}

// topDownClosure is every predicate the engine evaluates top-down while
// deriving the given ones, transitively (coneIndex.topDown).
func (c *coneIndex) topDownClosure(preds map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	queue := make([]string, 0, len(preds))
	for pred := range preds {
		queue = append(queue, pred)
	}
	for len(queue) > 0 {
		pred := queue[0]
		queue = queue[1:]
		for _, dep := range c.topDown[pred] {
			if _, seen := out[dep]; !seen {
				out[dep] = struct{}{}
				queue = append(queue, dep)
			}
		}
	}
	return out
}

// symbolsOf names every predicate symbol a store can hold under name: the
// program's (declared, derived, initial facts) plus every arity the EDB has
// held under it. The EDB's arities matter for undeclared predicates and for a
// row retracted since the last evaluation, whose atom the store still holds.
func (c *coneIndex) symbolsOf(name string, edbArities map[int]struct{}) []ast.PredicateSym {
	known := c.syms[name]
	if len(edbArities) == 0 {
		return known
	}
	out := make([]ast.PredicateSym, 0, len(known)+len(edbArities))
	out = append(out, known...)
	for arity := range edbArities {
		sym := ast.PredicateSym{Symbol: name, Arity: arity}
		dup := false
		for _, k := range known {
			if k == sym {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, sym)
		}
	}
	return out
}
