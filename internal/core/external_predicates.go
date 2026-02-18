package core

import (
	"codenerd/internal/logging"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/engine"
)

// =============================================================================
// EXTERNAL PREDICATE ADAPTERS (#17)
// =============================================================================
// These adapters bridge VirtualStore handler methods to the Mangle engine's
// native ExternalPredicateCallback interface.
//
// Previously, virtual predicates were resolved via a virtualFactStore wrapper
// that intercepted GetFacts() calls. This was fragile: it bypassed the engine's
// built-in external predicate support, couldn't validate binding patterns at
// analysis time, and re-queried handlers on every GetFacts call.
//
// Now each virtual predicate is registered as a native external predicate with
// proper mode declarations, enabling the engine to:
//   - Validate binding patterns at analysis time (mode checking)
//   - Cache results within evaluation cycles (no redundant handler calls)
//   - Apply filter pushdown for constant output filters

// virtualExternalPredicate adapts a VirtualStore handler method to the Mangle
// ExternalPredicateCallback interface.
type virtualExternalPredicate struct {
	predSym ast.PredicateSym
	handler func(query ast.Atom) ([]ast.Atom, error)
	mode    []ast.ArgMode
}

// ShouldPushdown returns false — handlers are simple in-process calls that
// don't benefit from subgoal pushdown.
func (v *virtualExternalPredicate) ShouldPushdown() bool { return false }

// ShouldQuery always returns true — handlers are cheap in-process calls
// (no network or disk I/O that we'd want to gate).
func (v *virtualExternalPredicate) ShouldQuery(inputs []ast.Constant, filters []ast.BaseTerm, pushdown []ast.Term) bool {
	return true
}

// ExecuteQuery reconstructs the full query atom from inputs and filters,
// delegates to the legacy handler, and emits output tuples via the callback.
func (v *virtualExternalPredicate) ExecuteQuery(inputs []ast.Constant, filters []ast.BaseTerm, pushdown []ast.Term, cb func([]ast.BaseTerm)) error {
	// Reconstruct the full query atom for the legacy handler.
	// Input positions get constants from `inputs`, output positions get
	// their original terms (variable or constant) from `filters`.
	args := make([]ast.BaseTerm, len(v.mode))
	inputIdx, filterIdx := 0, 0
	for i, m := range v.mode {
		if m == ast.ArgModeInput {
			args[i] = inputs[inputIdx]
			inputIdx++
		} else {
			args[i] = filters[filterIdx]
			filterIdx++
		}
	}
	query := ast.Atom{Predicate: v.predSym, Args: args}

	// Call the legacy handler.
	results, err := v.handler(query)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error(
			"External predicate %s query failed: %v", v.predSym.Symbol, err)
		return err
	}

	// Extract output positions from each result atom and emit via callback.
	// For all-input predicates (e.g. string_contains with mode('+','+')),
	// outputs will be nil — cb(nil) signals "yes, this fact exists" to the engine.
	for _, atom := range results {
		var outputs []ast.BaseTerm
		for i, m := range v.mode {
			if m != ast.ArgModeInput && i < len(atom.Args) {
				outputs = append(outputs, atom.Args[i])
			}
		}
		cb(outputs)
	}
	return nil
}

// BuildExternalPredicates constructs the external predicate callback map
// for the Mangle engine. This replaces the virtualFactStore interception
// pattern with the engine's native external predicate API.
//
// Mode assignments:
//
//	query_learned/2:         mode('-', '-')                    — arg0 optional
//	query_session/3:         mode('+', '-', '-')               — SessionID required
//	recall_similar/3:        mode('+', '-', '-')               — Query required
//	query_knowledge_graph/3: mode('+', '-', '-')               — Entity required
//	query_strategic/3:       mode('-', '-', '-')               — Category optional
//	query_activations/2:     mode('-', '-')                    — no inputs
//	has_learned/1:           mode('-')                         — Predicate optional
//	query_traces/5:          mode('+', '-', '-', '-', '-')     — ShardType required
//	query_trace_stats/4:     mode('+', '-', '-', '-')          — ShardType required
//	string_contains/2:       mode('+', '+')                    — both required
func (vs *VirtualStore) BuildExternalPredicates() map[ast.PredicateSym]engine.ExternalPredicateCallback {
	if vs == nil {
		return nil
	}

	mkPred := func(sym string, arity int, handler func(ast.Atom) ([]ast.Atom, error), mode ...ast.ArgMode) (ast.PredicateSym, engine.ExternalPredicateCallback) {
		ps := ast.PredicateSym{Symbol: sym, Arity: arity}
		return ps, &virtualExternalPredicate{
			predSym: ps,
			handler: handler,
			mode:    mode,
		}
	}

	in := ast.ArgMode(ast.ArgModeInput)
	out := ast.ArgMode(ast.ArgModeOutput)
	callbacks := make(map[ast.PredicateSym]engine.ExternalPredicateCallback, 10)

	sym, cb := mkPred("query_learned", 2, vs.getQueryLearnedAtoms, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_session", 3, vs.getQuerySessionAtoms, in, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("recall_similar", 3, vs.getRecallSimilarAtoms, in, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_knowledge_graph", 3, vs.getQueryKnowledgeGraphAtoms, in, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_strategic", 3, vs.getQueryStrategicAtoms, out, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_activations", 2, vs.getQueryActivationsAtoms, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("has_learned", 1, vs.getHasLearnedAtoms, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_traces", 5, vs.getQueryTracesAtoms, in, out, out, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("query_trace_stats", 4, vs.getQueryTraceStatsAtoms, in, out, out, out)
	callbacks[sym] = cb

	sym, cb = mkPred("string_contains", 2, vs.getStringContainsAtoms, in, in)
	callbacks[sym] = cb

	logging.VirtualStoreDebug("Built %d external predicate callbacks", len(callbacks))
	return callbacks
}
