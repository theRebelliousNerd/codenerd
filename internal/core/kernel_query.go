package core

import (
	"fmt"
	"reflect"
	"strings"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// QUERY METHODS
// =============================================================================

// Query retrieves facts for a predicate, optionally filtering by a pattern.
// Accepts either a bare predicate name (e.g., "user_intent") or a pattern with
// arguments (e.g., "selected_result(Atom, Priority, Source)" or "next_action(/generate_tool)").
//
// Pattern filtering rules:
// - Variables (e.g., Atom, X, _) are treated as wildcards.
// - Constants (name constants like /foo, strings like "bar", numbers) must match.
func (k *RealKernel) Query(predicate string) ([]Fact, error) {
	// A nil receiver is an error, not a crash. Query is reached through
	// interfaces (core.Kernel, world.FactQuerier, the shadow-mode querier), and
	// a nil *RealKernel stored in one of those is a NON-nil interface, so a
	// caller's `if k == nil` guard does not fire and ensureEvaluated
	// dereferences nil. Panicking there takes the whole agent down over a
	// wiring gap; returning an error lets the fail-closed paths — which are the
	// ones that ask a kernel whether something is safe — record a failed check
	// and refuse, which is what they are built to do.
	if k == nil {
		return nil, fmt.Errorf("query %q: kernel is nil", predicate)
	}
	if predicate == "" {
		return nil, fmt.Errorf("cannot query empty predicate string")
	}
	timer := logging.StartTimer(logging.CategoryKernel, "Query")
	// Suppress per-query debug logging for idle queries; only log at trace level

	// Lazy evaluation under singleflight; no lock-upgrade dance.
	if err := k.ensureEvaluated(); err != nil {
		return nil, err
	}
	k.mu.RLock()
	if !k.initialized && k.programInfo != nil {
		// Cleared kernel: the program survived but the EDB was wiped.
		// Re-evaluate lazily instead of erroring. A never-booted kernel
		// (no programInfo) has nothing to evaluate and still errors below.
		k.mu.RUnlock()
		k.factsDirty.Store(true)
		if err := k.ensureEvaluated(); err != nil {
			return nil, err
		}
		k.mu.RLock()
	}
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("Query: %v", err)
		return nil, err
	}

	predicateName, patternFact, hasPattern, desiredArity := parseQueryPattern(predicate)

	queryDeclExternal := false
	results := make([]Fact, 0)

	// Get the predicate symbol from the program
	if k.programInfo == nil {
		logging.KernelDebug("Query: programInfo is nil, returning empty results")
		timer.Stop()
		return results, nil
	}

	// Find the predicate in the decls
	predicateFound := false

	// Fast path: find predicate without iterating if we know the arity
	if hasPattern {
		pred := ast.PredicateSym{Symbol: predicateName, Arity: desiredArity}
		if d, ok := k.programInfo.Decls[pred]; ok {
			predicateFound = true
			queryDeclExternal = d.IsExternal()
			k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				if factMatchesPattern(fact, patternFact) {
					results = append(results, fact)
				}
				return nil
			})
		}
	} else {
		matchedDecls := 0
		for pred := range k.programInfo.Decls {
			if pred.Symbol == predicateName {
				predicateFound = true
				matchedDecls++
				perArityCount := 0
				k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
					fact := atomToFact(a)
					results = append(results, fact)
					perArityCount++
					return nil
				})
				logging.KernelDebug("Query: decl %s/%d -> %d facts", pred.Symbol, pred.Arity, perArityCount)
			}
		}
		logging.KernelDebug("Query: predicate=%s matchedDecls=%d totalResults=%d", predicateName, matchedDecls, len(results))
	}

	if vsFacts, handled, verr := k.queryExternalVirtualStore(predicate, patternFact, hasPattern, queryDeclExternal); handled {
		if verr != nil {
			return nil, verr
		}
		results = vsFacts
	}

	if !predicateFound {
		// Upgraded from Debug to Warn: missing predicates may indicate
		// schema drift or missing declarations, which can cause silent bugs.
		logging.Get(logging.CategoryKernel).Warn("Query: predicate '%s' not found in declarations", predicateName)
	}

	// JIT-related predicate debugging - log at INFO level for visibility
	jitPredicates := map[string]bool{
		"selected_atom": true, "is_mandatory": true, "mandatory_atom": true,
		"prohibited_atom": true, "compilation_valid": true, "candidate_atom": true,
	}
	if jitPredicates[predicateName] {
		logging.Kernel("JIT-Query: %s found=%v results=%d", predicateName, predicateFound, len(results))
	}

	elapsed := timer.Stop()
	if len(results) > 0 {
		logging.KernelDebug("Query: predicate=%s returned %d results", predicate, len(results))
	}
	logging.Audit().KernelQuery(predicate, len(results), elapsed.Milliseconds())
	return results, nil
}

// parseQueryPattern splits a query string into its predicate name and an
// optional argument pattern. Variables in the pattern act as wildcards;
// constants must match. A bare predicate name yields hasPattern=false.
// An unparseable pattern form falls back to a predicate-only query using
// the name before the paren, matching historical behavior.
func parseQueryPattern(query string) (predicateName string, pattern Fact, hasPattern bool, arity int) {
	predicateName = query
	if idx := strings.Index(query, "("); idx > 0 {
		// Fast path: extract predicate name even if full parse fails.
		predicateName = strings.TrimSpace(query[:idx])
		if parsedFact, err := ParseFactString(query); err == nil {
			return parsedFact.Predicate, parsedFact, true, len(parsedFact.Args)
		}
	}
	return predicateName, Fact{}, false, 0
}

func factMatchesPattern(f Fact, pattern Fact) bool {
	if f.Predicate != pattern.Predicate {
		return false
	}
	if len(f.Args) != len(pattern.Args) {
		return false
	}
	for i := range pattern.Args {
		if !patternArgMatches(pattern.Args[i], f.Args[i]) {
			return false
		}
	}
	return true
}

func patternArgMatches(pattern any, value any) bool {
	// Variables are represented as strings like "?X" by atomToFact/baseTermToValue.
	if s, ok := pattern.(string); ok && strings.HasPrefix(s, "?") {
		return true
	}

	// OPTIMIZATION: Replace reflect.DeepEqual with type switches
	// Normalize both values first
	normPattern := normalizeQueryValue(pattern)
	normValue := normalizeQueryValue(value)

	// Fast path: pointer equality
	if normPattern == normValue {
		return true
	}

	// Type-based comparison (avoid reflection)
	switch p := normPattern.(type) {
	case int64:
		if v, ok := normValue.(int64); ok {
			return p == v
		}
	case string:
		if v, ok := normValue.(string); ok {
			return p == v
		}
	case MangleAtom:
		if v, ok := normValue.(MangleAtom); ok {
			return p == v
		}
		// Cross-compare with string
		if v, ok := normValue.(string); ok {
			return string(p) == v
		}
	case bool:
		if v, ok := normValue.(bool); ok {
			return p == v
		}
	case float64:
		if v, ok := normValue.(float64); ok {
			return p == v
		}
	default:
		// FALLBACK: Only for truly unknown types
		// This should rarely execute with well-typed facts
		return reflect.DeepEqual(normPattern, normValue)
	}

	return false
}

func normalizeQueryValue(v any) any {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		// Fold only integral floats: 3.0 is 3, but 3.14 must never
		// match 3. Truncation here made every fractional pattern
		// over-match its integer neighbors.
		if t == float64(int64(t)) {
			return int64(t)
		}
		return t
	default:
		return v
	}
}

// QueryCallback retrieves facts for a predicate and invokes the callback for each fact.
// This allows streaming processing and avoids O(N) memory allocation for large result sets.
// Like Query, it accepts an optional pattern (e.g., "code_defines(/file.go, X)").
func (k *RealKernel) QueryCallback(predicate string, cb func(Fact) error) error {
	// Nil receiver is an error, not a crash (see Query for the interface-trap rationale).
	if k == nil {
		return fmt.Errorf("queryCallback %q: kernel is nil", predicate)
	}
	timer := logging.StartTimer(logging.CategoryKernel, "QueryCallback")
	logging.KernelDebug("QueryCallback: predicate=%s", predicate)

	if err := k.ensureEvaluated(); err != nil {
		return err
	}
	k.mu.RLock()
	if !k.initialized && k.programInfo != nil {
		// Cleared kernel: the program survived but the EDB was wiped.
		// Re-evaluate lazily instead of erroring. A never-booted kernel
		// (no programInfo) has nothing to evaluate and still errors below.
		k.mu.RUnlock()
		k.factsDirty.Store(true)
		if err := k.ensureEvaluated(); err != nil {
			return err
		}
		k.mu.RLock()
	}
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("QueryCallback: %v", err)
		return err
	}

	predicateName, patternFact, hasPattern, desiredArity := parseQueryPattern(predicate)

	if k.programInfo == nil {
		logging.KernelDebug("QueryCallback: programInfo is nil, returning")
		timer.Stop()
		return nil
	}

	// Find the predicate in the decls
	predicateFound := false
	queryDeclExternal := false
	count := 0

	if hasPattern {
		pred := ast.PredicateSym{Symbol: predicateName, Arity: desiredArity}
		if d, ok := k.programInfo.Decls[pred]; ok {
			predicateFound = true
			queryDeclExternal = d.IsExternal()
			err := k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				if factMatchesPattern(fact, patternFact) {
					if err := cb(fact); err != nil {
						return err
					}
					count++
				}
				return nil
			})
			if err != nil {
				timer.Stop()
				return err
			}
		}
	} else {
		for pred := range k.programInfo.Decls {
			if pred.Symbol == predicateName {
				predicateFound = true
				err := k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
					fact := atomToFact(a)
					if err := cb(fact); err != nil {
						return err
					}
					count++
					return nil
				})
				if err != nil {
					timer.Stop()
					return err
				}
			}
		}
	}

	if vsFacts, handled, verr := k.queryExternalVirtualStore(predicate, patternFact, hasPattern, queryDeclExternal); handled {
		if verr != nil {
			timer.Stop()
			return verr
		}
		for _, f := range vsFacts {
			if err := cb(f); err != nil {
				timer.Stop()
				return err
			}
			count++
		}
	}

	if !predicateFound {
		logging.Get(logging.CategoryKernel).Warn("QueryCallback: predicate '%s' not found in declarations", predicateName)
	}

	elapsed := timer.Stop()
	logging.KernelDebug("QueryCallback: predicate=%s processed %d results", predicate, count)
	logging.Audit().KernelQuery(predicate, count, elapsed.Milliseconds())
	return nil
}

// queryExternalVirtualStore resolves a direct query for an external predicate
// through the attached VirtualStore. External predicates are computed on
// demand, never stored: without this a direct query for one always answers
// empty even with an adapter wired, and the Mangle-World bridge only fires
// during rule evaluation. Live results supersede store rows: any stored atoms
// for an external are eval-cache artifacts that predate the call.
//
// handled=false means "not an external call": the caller falls back to its
// store rows. Callers must hold at least k.mu.RLock (for the virtualStore
// pointer read). External handlers must not call back into kernel.Query
// for external predicates: concurrent queries share one kernel, so no
// counter can tell nesting from parallelism — nesting would recurse.
// No handler does this today (audited); if one ever must, thread an
// explicit depth token through the handler chain.
func (k *RealKernel) queryExternalVirtualStore(predicate string, patternFact Fact, hasPattern, isExternal bool) ([]Fact, bool, error) {
	if !isExternal || !hasPattern || k.virtualStore == nil {
		return nil, false, nil
	}
	qatom, err := parseAtom(predicate)
	if err != nil {
		return nil, false, nil
	}
	vsAtoms, err := k.virtualStore.Get(qatom)
	if err != nil {
		return nil, true, err
	}
	vsFacts := make([]Fact, 0, len(vsAtoms))
	for _, a := range vsAtoms {
		f := atomToFact(a)
		if factMatchesPattern(f, patternFact) {
			vsFacts = append(vsFacts, f)
		}
	}
	return vsFacts, true, nil
}

// QueryAll retrieves all derived facts organized by predicate.
func (k *RealKernel) QueryAll() (map[string][]Fact, error) {
	// Nil receiver is an error, not a crash (see Query for the interface-trap rationale).
	if k == nil {
		return nil, fmt.Errorf("queryAll: kernel is nil")
	}
	timer := logging.StartTimer(logging.CategoryKernel, "QueryAll")
	logging.KernelDebug("QueryAll: retrieving all derived facts")

	if err := k.ensureEvaluated(); err != nil {
		return nil, err
	}
	k.mu.RLock()
	if !k.initialized && k.programInfo != nil {
		// Cleared kernel: the program survived but the EDB was wiped.
		// Re-evaluate lazily instead of erroring. A never-booted kernel
		// (no programInfo) has nothing to evaluate and still errors below.
		k.mu.RUnlock()
		k.factsDirty.Store(true)
		if err := k.ensureEvaluated(); err != nil {
			return nil, err
		}
		k.mu.RLock()
	}
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("QueryAll: %v", err)
		return nil, err
	}

	results := make(map[string][]Fact)

	if k.programInfo == nil {
		logging.KernelDebug("QueryAll: programInfo is nil, returning empty results")
		timer.Stop()
		return results, nil
	}

	// Iterate through all declared predicates
	totalFacts := 0
	for pred := range k.programInfo.Decls {
		predName := pred.Symbol
		if _, ok := results[predName]; !ok {
			results[predName] = make([]Fact, 0)
		}

		k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
			fact := atomToFact(a)
			results[predName] = append(results[predName], fact)
			totalFacts++
			return nil
		})
	}

	timer.Stop()
	logging.KernelDebug("QueryAll: returned %d predicates with %d total facts", len(results), totalFacts)
	return results, nil
}

// GetDerivedFacts returns all derived facts organized by predicate (alias for QueryAll).
func (k *RealKernel) GetDerivedFacts() (map[string][]Fact, error) {
	return k.QueryAll()
}

// =============================================================================
// PARSING HELPERS
// =============================================================================

// ParseSingleFact parses a single fact string safely.
func ParseSingleFact(content string) (Fact, error) {
	facts, err := ParseFactsFromString(content)
	if err != nil {
		return Fact{}, err
	}
	if len(facts) == 0 {
		return Fact{}, fmt.Errorf("no facts found")
	}
	if len(facts) > 1 {
		return Fact{}, fmt.Errorf("multiple facts found")
	}
	return facts[0], nil
}

// atomToFact converts a Mangle AST Atom back to our Fact type.
func atomToFact(a ast.Atom) Fact {
	args := make([]any, len(a.Args))
	for i, term := range a.Args {
		args[i] = baseTermToValue(term)
	}
	return Fact{
		Predicate: a.Predicate.Symbol,
		Args:      args,
	}
}

// baseTermToValue extracts the Go value from a Mangle BaseTerm.
func baseTermToValue(term ast.BaseTerm) any {
	switch t := term.(type) {
	case ast.Constant:
		switch t.Type {
		case ast.NameType:
			return t.Symbol
		case ast.StringType:
			return t.Symbol
		case ast.BytesType:
			return t.Symbol
		case ast.NumberType:
			return t.NumValue
		case ast.Float64Type:
			val, _ := t.Float64Value()
			return val
		case ast.ListShape, ast.MapShape, ast.PairShape, ast.TimeType, ast.DurationType:
			// Composite constants keep their value in struct fields, not
			// Symbol: reading Symbol yields "" and silently drops lists,
			// maps, and times crossing the Facts boundary. String()
			// renders them (e.g. ["depA", "depB"]).
			return t.String()
		default:
			// DEFENSIVE: Log unknown constant types to catch new AST types early
			logging.Kernel("baseTermToValue: unknown constant type %v, using Symbol fallback", t.Type)
			return t.Symbol
		}
	case ast.Variable:
		return fmt.Sprintf("?%s", t.Symbol)
	default:
		return fmt.Sprintf("%v", term)
	}
}

// ParseFactString parses a Mangle fact string into a Fact.
// Format: predicate(arg1, arg2, ...) where args can be:
//   - Name constants: /foo, /bar
//   - Strings: "quoted text"
//   - Numbers: 42, 3.14
func ParseFactString(factStr string) (Fact, error) {
	// Wrap in a minimal program to allow parsing
	if strings.TrimSpace(factStr) == "" || strings.TrimSpace(factStr) == "." {
		return Fact{}, fmt.Errorf("cannot parse empty fact string")
	}
	programStr := factStr + "."
	parsed, err := parseUnit(strings.NewReader(programStr))
	if err != nil {
		return Fact{}, fmt.Errorf("failed to parse fact string: %w", err)
	}

	if len(parsed.Clauses) == 0 {
		return Fact{}, fmt.Errorf("no clauses found in fact string")
	}

	// Extract the first clause's head atom
	clause := parsed.Clauses[0]
	// In Mangle AST, Head is an ast.Atom struct, not a pointer
	atom := clause.Head

	return atomToFact(atom), nil
}

// ParseFactsFromString parses multiple facts from a string (one per line or separated by '.').
func ParseFactsFromString(content string) ([]Fact, error) {
	// Parse as a Mangle program
	parsed, err := parseUnit(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse facts: %w", err)
	}

	facts := make([]Fact, 0, len(parsed.Clauses))
	for _, clause := range parsed.Clauses {
		// Only extract facts from clauses with no body (ground facts)
		if len(clause.Premises) > 0 {
			continue // Skip rules
		}

		// In Mangle AST, Head is an ast.Atom struct, not a pointer
		atom := clause.Head
		facts = append(facts, atomToFact(atom))
	}

	return facts, nil
}
