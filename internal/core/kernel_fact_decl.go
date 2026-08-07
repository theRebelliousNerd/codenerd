package core

import (
	"fmt"
	"math"
	"slices"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// numberBoundSymbol is the Decl bound that means "int64" in the corpus.
const numberBoundSymbol = "/number"

// declBoundsLocked returns the positional bounds declared for atom's predicate,
// or nil when the predicate is undeclared or unbounded. Call holding k.mu.
func (k *RealKernel) declBoundsLocked(pred ast.PredicateSym) []ast.BaseTerm {
	if k.programInfo == nil || k.programInfo.Decls == nil {
		return nil
	}
	decl, ok := k.programInfo.Decls[pred]
	if !ok || decl == nil || len(decl.Bounds) == 0 {
		return nil
	}
	return decl.Bounds[0].Bounds
}

// coerceAtomToDeclLocked reconciles an atom's constant types with the bounds its
// predicate declares, and is the single choke point that keeps a float out of an
// int64 slot.
//
// It exists because types.Fact.ToAtom is Decl-blind: it maps every Go float64 to
// ast.Float64 regardless of what the predicate declares. That is fatal here, not
// merely wrong. The pinned Mangle fork implements <, <=, >, >= over int64 ONLY
// (builtin.go sends each through getNumberValues -> getNumberValue, which errors
// on any Type != ast.NumberType; getFloatValue has no caller at all). The error
// propagates out of EvalStratifiedProgram, so evaluate() returns at
// kernel_eval.go before assigning k.store — one bad fact stops the WHOLE kernel
// from deriving anything, on every subsequent pass, and the message names only
// the value ("value 110 (4) is not a number"), never the predicate that carried
// it. Observed live: ~4 aborts every 2 seconds for an entire session.
//
// So: an integral float bound for a /number slot is silently narrowed to
// ast.Number, and a fractional one is rejected with a message naming the
// predicate and argument index. Rejecting one fact is recoverable; poisoning the
// fixpoint is not.
func (k *RealKernel) coerceAtomToDeclLocked(atom ast.Atom) (ast.Atom, error) {
	bounds := k.declBoundsLocked(atom.Predicate)
	if bounds == nil {
		return atom, nil
	}

	var coerced []ast.BaseTerm
	for i, arg := range atom.Args {
		if i >= len(bounds) {
			break
		}
		c, ok := arg.(ast.Constant)
		if !ok || c.Type != ast.Float64Type {
			continue
		}
		// Union or otherwise non-simple bounds are left alone.
		b, ok := bounds[i].(ast.Constant)
		if !ok || b.Symbol != numberBoundSymbol {
			continue
		}

		f, err := c.Float64Value()
		if err != nil {
			return ast.Atom{}, fmt.Errorf("%s arg %d: unreadable float: %w", atom.Predicate.Symbol, i, err)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
			return ast.Atom{}, fmt.Errorf(
				"%s arg %d is declared %s but got the non-integral float %v; "+
					"scale ratios to integer percent at the assert site (see types.PercentScale)",
				atom.Predicate.Symbol, i, numberBoundSymbol, f)
		}
		if coerced == nil {
			coerced = slices.Clone(atom.Args)
		}
		coerced[i] = ast.Number(int64(f))
	}

	if coerced == nil {
		return atom, nil
	}
	return ast.Atom{Predicate: atom.Predicate, Args: coerced}, nil
}

// factToAtomLocked converts a fact and applies Decl-directed coercion. Every
// path that fills k.cachedAtoms must go through here, otherwise an uncoerced
// atom reaches the store and takes the fixpoint down. Call holding k.mu.
func (k *RealKernel) factToAtomLocked(f Fact) (ast.Atom, error) {
	atom, err := f.ToAtom()
	if err != nil {
		return ast.Atom{}, err
	}
	// Before the first evaluate() there is no programInfo, so no bounds are
	// known yet. Mark the cache so evaluateFullLocked reconverts once the
	// Decls exist rather than leaving those boot facts uncoerced forever.
	if k.programInfo == nil {
		k.atomCacheStale = true
		return atom, nil
	}
	return k.coerceAtomToDeclLocked(atom)
}
