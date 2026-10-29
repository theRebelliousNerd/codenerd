package core

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

func (x *edbIndex) len() int {
	return x.size
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
