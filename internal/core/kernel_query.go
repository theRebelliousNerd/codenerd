package core

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"codenerd/internal/logging"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
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
	timer := logging.StartTimer(logging.CategoryKernel, "Query")
	logging.KernelDebug("Query: predicate=%s", predicate)

	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("Query: %v", err)
		return nil, err
	}

	// Parse optional pattern form, using the official Mangle parser for correctness.
	// If parsing fails, fall back to predicate-only query.
	var (
		patternFact   Fact
		hasPattern    bool
		desiredArity  int
		predicateName = predicate
	)
	if idx := strings.Index(predicate, "("); idx > 0 {
		// Fast path: extract predicate name even if full parse fails.
		predicateName = strings.TrimSpace(predicate[:idx])
		if parsedFact, err := ParseFactString(predicate); err == nil {
			patternFact = parsedFact
			hasPattern = true
			desiredArity = len(parsedFact.Args)
			predicateName = parsedFact.Predicate
		}
	}

	results := make([]Fact, 0)

	// Get the predicate symbol from the program
	if k.programInfo == nil {
		logging.KernelDebug("Query: programInfo is nil, returning empty results")
		timer.Stop()
		return results, nil
	}

	// Find the predicate in the decls
	predicateFound := false
	for pred := range k.programInfo.Decls {
		if pred.Symbol == predicateName && (!hasPattern || pred.Arity == desiredArity) {
			predicateFound = true
			// Query the store for all atoms of this predicate
			k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				// If a pattern was provided, filter by constants.
				if !hasPattern || factMatchesPattern(fact, patternFact) {
					results = append(results, fact)
				}
				return nil
			})
			break
		}
	}

	if !predicateFound {
		logging.KernelDebug("Query: predicate '%s' not found in declarations", predicateName)
	}

	// JIT-related predicate debugging - log at INFO level for visibility
	jitPredicates := map[string]bool{
		"selected_result": true, "is_mandatory": true, "mandatory_selection": true,
		"blocked_by_context": true, "final_valid": true, "tentative": true,
	}
	if jitPredicates[predicateName] {
		logging.Kernel("JIT-Query: %s found=%v results=%d", predicateName, predicateFound, len(results))
	}

	elapsed := timer.Stop()
	logging.KernelDebug("Query: predicate=%s returned %d results", predicate, len(results))
	logging.Audit().KernelQuery(predicate, len(results), elapsed.Milliseconds())
	return results, nil
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

func patternArgMatches(pattern interface{}, value interface{}) bool {
	// Variables are represented as strings like "?X" by atomToFact/baseTermToValue.
	if s, ok := pattern.(string); ok && strings.HasPrefix(s, "?") {
		return true
	}
	return reflect.DeepEqual(normalizeQueryValue(pattern), normalizeQueryValue(value))
}

func normalizeQueryValue(v interface{}) interface{} {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		// Mangle numeric constants are integers; normalize defensively.
		return int64(t)
	default:
		return v
	}
}

// QueryAll retrieves all derived facts organized by predicate.
func (k *RealKernel) QueryAll() (map[string][]Fact, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "QueryAll")
	logging.KernelDebug("QueryAll: retrieving all derived facts")

	k.mu.RLock()
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
		results[predName] = make([]Fact, 0)

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

// LoadFactsFromFile loads facts from a .mg file and adds them to the EDB.
func (k *RealKernel) LoadFactsFromFile(path string) error {
	logging.KernelDebug("LoadFactsFromFile: loading facts from %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to read %s: %v", path, err)
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	facts, err := ParseFactsFromString(string(data))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to parse facts from %s: %v", path, err)
		return fmt.Errorf("failed to parse facts from %s: %w", path, err)
	}

	if len(facts) == 0 {
		logging.KernelDebug("LoadFactsFromFile: no facts found in %s", path)
		return nil
	}

	logging.Kernel("LoadFactsFromFile: parsed %d facts from %s", len(facts), path)
	return k.LoadFacts(facts)
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
	args := make([]interface{}, len(a.Args))
	for i, term := range a.Args {
		args[i] = baseTermToValue(term)
	}
	return Fact{
		Predicate: a.Predicate.Symbol,
		Args:      args,
	}
}

// baseTermToValue extracts the Go value from a Mangle BaseTerm.
func baseTermToValue(term ast.BaseTerm) interface{} {
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
			return t.Float64Value
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
	programStr := factStr + "."
	parsed, err := parse.Unit(strings.NewReader(programStr))
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
	parsed, err := parse.Unit(strings.NewReader(content))
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

// UpdateSystemFacts updates system-level facts (e.g., time, git state).
// This is a placeholder for dynamic system fact injection.
func (k *RealKernel) UpdateSystemFacts() error {
	// TODO: Inject system facts like current_time, git_branch, etc.
	logging.KernelDebug("UpdateSystemFacts: placeholder (no-op for now)")
	return nil
}
