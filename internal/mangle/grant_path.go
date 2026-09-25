package mangle

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// THE CONSTITUTIONAL GRANT PATH
// =============================================================================
// Learning a new predicate is legitimate; learning new authority is not. A
// learned rule (or fact) that adds facts to permitted/3, or to anything
// permitted/3 is derived from, widens what the constitution lets through
// without anyone having written that into the constitution.
//
// forbiddenLearnedHeads names seven of those predicates by hand, and until
// 2026-09-25 that list was the whole defence. It was incomplete: the appeal
// path in constitution.mg derives permitted from has_active_override, which is
// derived from appeal_granted and temporary_override, and none of the three was
// on it. A learned fact such as
//
//	appeal_granted("a1", /exec_cmd, "x", "y").
//
// passed ValidateLearnedRule, compiled in the sandbox, was persisted to
// learned.mg, and permitted every pending /exec_cmd from then on. The same held
// for file_recoverable (delete permission) and for the world-model predicates
// safe_action reads.
//
// So the grant path is derived, not listed. GrantPathOf walks the program's
// rules backwards from each grant root, tracking polarity: a positive premise
// of a rule whose head can widen a root can widen it too; a negated premise
// can only narrow it, unless it sits under another negation. A learned rule may
// define a predicate that narrows a grant (dangerous_action, block_action, a
// fresh classifier), never one on the grant path. A new rule added to the
// constitution is covered the moment it is written.

// grantRoots are the predicates whose facts are authority. permitted/3 is the
// constitutional gate: every action must derive it (default deny).
var grantRoots = []string{"permitted"}

// grantConsent are premises that stand for an explicit human authorization.
// A rule that requires one widens its head only with that human's consent, so
// its other positive premises are not on the grant path through it -- this is
// what lets the constitution's autopoiesis keep learning dangerous_action,
// which reaches permitted only through the signed-approval rule. The consent
// predicates themselves always are on the path.
//
// Leaving a consent predicate off this list can only protect more, never less:
// the premises it would have guarded become grant path instead.
var grantConsent = map[string]bool{
	"signed_approval": true,
	"admin_override":  true,
}

// GrantPath maps each predicate whose facts can widen a grant root to the head
// of the rule through which they do (a root maps to itself).
type GrantPath map[string]string

// Contains reports whether pred is on the grant path, and through which head.
func (g GrantPath) Contains(pred string) (via string, ok bool) {
	via, ok = g[pred]
	return via, ok
}

// Sorted returns the grant path's predicates in order, for messages and tests.
func (g GrantPath) Sorted() []string {
	out := make([]string, 0, len(g))
	for p := range g {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ErrGrantPathUnknown marks a learned-rule verdict given without a derived
// grant path: the program the path is derived from could not be read, so no
// learned rule can be shown not to widen a grant, and none is admitted.
var ErrGrantPathUnknown = errors.New("the constitutional grant path could not be derived")

// GrantPathOf derives the grant path of a program from its rules.
//
// It over-approximates, in the safe direction: a rule with an aggregation is
// read as both widening and narrowing its premises (a count can move either
// way), and a predicate reached with both polarities is on the path. Builtins
// (":lt", "fn:...") are not predicates anyone can define and are skipped.
func GrantPathOf(clauses []ast.Clause) GrantPath {
	byHead := make(map[string][]ast.Clause)
	for _, c := range clauses {
		if len(c.Premises) == 0 {
			continue // a fact extends its predicate but reads nothing
		}
		byHead[c.Head.Predicate.Symbol] = append(byHead[c.Head.Predicate.Symbol], c)
	}

	type node struct {
		pred     string
		widening bool // true: more facts of pred can widen a root
	}
	path := make(GrantPath)
	narrowing := make(map[string]struct{})
	var queue []node
	for _, root := range grantRoots {
		path[root] = root
		queue = append(queue, node{root, true})
	}
	mark := func(pred, via string, widening bool) {
		if strings.Contains(pred, ":") {
			return // a builtin (":lt", "fn:..."), not a definable predicate
		}
		if widening {
			if _, seen := path[pred]; !seen {
				path[pred] = via
				queue = append(queue, node{pred, true})
			}
			return
		}
		if _, seen := narrowing[pred]; !seen {
			narrowing[pred] = struct{}{}
			queue = append(queue, node{pred, false})
		}
	}

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, rule := range byHead[n.pred] {
			aggregated := rule.Transform != nil
			consented := false
			for _, p := range rule.Premises {
				if a, ok := p.(ast.Atom); ok && grantConsent[a.Predicate.Symbol] {
					consented = true
				}
			}
			for _, premise := range rule.Premises {
				pred, negated, ok := premiseLiteral(premise)
				if !ok {
					continue
				}
				// Does more of pred make more of n.pred?
				same := !negated
				widens := same == n.widening // more pred -> more root
				if aggregated {
					mark(pred, n.pred, true)
					mark(pred, n.pred, false)
					continue
				}
				if n.widening && same && consented && !grantConsent[pred] {
					// More of pred widens the head only with a human's
					// consent, which the rule already requires: not a path.
					continue
				}
				mark(pred, n.pred, widens)
			}
		}
	}
	return path
}

// premiseLiteral names the predicate a premise reads and whether it is negated.
// Comparisons and other builtins report ok=false.
func premiseLiteral(t ast.Term) (pred string, negated bool, ok bool) {
	switch v := t.(type) {
	case ast.Atom:
		return v.Predicate.Symbol, false, true
	case ast.NegAtom:
		return v.Atom.Predicate.Symbol, true, true
	case ast.TemporalAtom:
		return v.Atom.Predicate.Symbol, false, true
	case ast.TemporalLiteral:
		pred, negated, ok = premiseLiteral(v.Literal)
		return pred, negated, ok
	}
	return "", false, false
}

// grantPathCache memoizes GrantPathOfSource by the source's hash. Every kernel
// in a process loads the same embedded constitution, and parsing it costs
// ~120ms; the hash costs ~1ms. It is bounded because a rule-court sandbox is a
// new program per proposed rule: past the bound it starts over, which costs
// one parse, never a wrong answer.
var grantPathCache = struct {
	sync.Mutex
	paths map[[32]byte]GrantPath
}{paths: make(map[[32]byte]GrantPath)}

const grantPathCacheMax = 64

// GrantPathOfSource parses src -- a program's schemas and policy -- and
// derives its grant path. The result is shared: callers must not modify it.
func GrantPathOfSource(src string) (GrantPath, error) {
	key := sha256.Sum256([]byte(src))
	grantPathCache.Lock()
	cached, ok := grantPathCache.paths[key]
	grantPathCache.Unlock()
	if ok {
		return cached, nil
	}
	unit, err := ParseUnit(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGrantPathUnknown, err)
	}
	path := GrantPathOf(unit.Clauses)
	grantPathCache.Lock()
	if len(grantPathCache.paths) >= grantPathCacheMax {
		clear(grantPathCache.paths)
	}
	grantPathCache.paths[key] = path
	grantPathCache.Unlock()
	return path, nil
}
