package core

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

func (x *edbIndex) len() int {
	return x.size
}

// bucketsMap is every held fact, by predicate and canonical key, as plain Go
// maps: what tests compare an incremental index against a rebuilt one with.
func (x *edbIndex) bucketsMap() map[string]map[string]Fact {
	out := make(map[string]map[string]Fact, len(x.rows))
	for pred, rows := range x.rows {
		out[pred] = make(map[string]Fact, rows.len())
		for key, f := range rows.all() {
			out[pred][key] = f
		}
	}
	return out
}

// parseQueryPattern retains the historical decoded helper for tests and
// callers inside this package. Runtime filtering uses the AST-preserving
// parseQueryPatternDetails path above.
func parseQueryPattern(query string) (predicateName string, pattern Fact, hasPattern bool, arity int) {
	parsed := parseQueryPatternDetails(query)
	if !parsed.hasPattern {
		return parsed.predicate, Fact{}, false, 0
	}
	return parsed.predicate, atomToFact(parsed.atom), true, len(parsed.atom.Args)
}
