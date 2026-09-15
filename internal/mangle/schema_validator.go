package mangle

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// SCHEMA VALIDATOR (Bug #18 Fix - Schema Drift Prevention)
// =============================================================================
// Prevents the agent from hallucinating predicates in learned rules that have
// no data source. All predicates used in rule bodies MUST be declared in
// schemas.mg or already exist in learned.mg.
//
// Example BAD rule (will be rejected):
//   candidate_action(/monitor_server) :- server_health(/degraded).
//   ^ "server_health" is not declared - this rule will never fire!
//
// This enforces the constraint: ∀ predicate P in rule body: P ∈ DeclaredSchema

// SchemaValidator validates that rules only use declared predicates.
type SchemaValidator struct {
	declaredPredicates map[string]bool
	predicateArities   map[string]int // Tracks expected arity for each predicate
	schemasText        string
	learnedText        string
}

// NewSchemaValidator creates a validator with the system schemas.
func NewSchemaValidator(schemasText, learnedText string) *SchemaValidator {
	return &SchemaValidator{
		declaredPredicates: make(map[string]bool),
		predicateArities:   make(map[string]int),
		schemasText:        schemasText,
		learnedText:        learnedText,
	}
}

// LoadDeclaredPredicates parses schemas.mg and extracts all Decl statements.
func (sv *SchemaValidator) LoadDeclaredPredicates() error {
	// Parse schemas to extract declarations
	if sv.schemasText != "" {
		if err := sv.extractDeclsFromText(sv.schemasText); err != nil {
			return fmt.Errorf("failed to parse schemas: %w", err)
		}
	}

	// Also extract any predicates defined as heads in learned.mg
	if sv.learnedText != "" {
		if err := sv.extractHeadPredicatesFromText(sv.learnedText); err != nil {
			return fmt.Errorf("failed to parse learned rules: %w", err)
		}
	}

	return nil
}

// extractDeclsFromText parses text and extracts all predicates from Decl statements.
func (sv *SchemaValidator) extractDeclsFromText(text string) error {
	// The head pattern locates `Decl name(`; the argument list is then
	// scanned to its matching close paren, because a `[^)]*` capture
	// truncates nested terms (Decl foo(bar(1,2), X) would lose everything
	// after the first `)`).
	matches := declHeadPattern.FindAllStringSubmatchIndex(text, -1)

	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 {
			continue
		}
		predicate := text[m[2]:m[3]]
		sv.declaredPredicates[predicate] = true
		argsStr := balancedArgs(text, m[1])
		sv.predicateArities[predicate] = countTopLevelArgs(argsStr)
	}

	return nil
}

// balancedArgs returns the argument list starting at the open paren that
// ends at fromIdx (the index just past it), scanned to its match.
func balancedArgs(text string, fromIdx int) string {
	depth := 1
	for i := fromIdx; i < len(text); i++ {
		if text[i] == '(' {
			depth++
		} else if text[i] == ')' {
			depth--
			if depth == 0 {
				return text[fromIdx:i]
			}
		}
	}
	return ""
}

// extractHeadPredicatesFromText extracts predicates that are defined as rule heads.
// These are implicitly declared by being derived.
func (sv *SchemaValidator) extractHeadPredicatesFromText(text string) error {
	// Pattern: predicate(args) :- ...
	// or: predicate(args).
	matches := headPattern.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			predicate := match[1]
			sv.declaredPredicates[predicate] = true
		}
	}

	return nil
}

// ValidateRule checks if a rule only uses declared predicates in its body.
// Returns error if any undefined predicate is found.
func (sv *SchemaValidator) ValidateRule(ruleText string) error {
	body, isRule := splitRuleBody(ruleText)
	if !isRule {
		// Fact, not a rule - no body to validate
		return nil
	}

	// Extract all predicate calls from body
	// Pattern: predicate_name(
	matches := predicatePattern.FindAllStringSubmatch(body, -1)

	var undefined []string
	for _, match := range matches {
		if len(match) > 1 {
			predicate := match[1]
			// Skip built-in predicates and operators
			if sv.isBuiltin(predicate) {
				continue
			}
			if !sv.declaredPredicates[predicate] {
				undefined = append(undefined, predicate)
			}
		}
	}

	if len(undefined) > 0 {
		return fmt.Errorf("rule uses undefined predicates: %v (available: %v)",
			undefined, sv.getAvailablePredicates())
	}

	return nil
}

// ValidateLearnedRule validates a learned rule/fact.
//
// In addition to schema drift checks (undefined predicates in the body), learned rules are
// prevented from defining protected control-plane predicates that must remain deterministic.
// Also validates that head predicates match declared arities and that learned facts only
// assert into declared predicates (schema drift prevention).
func (sv *SchemaValidator) ValidateLearnedRule(ruleText string) error {
	trimmed := strings.TrimSpace(ruleText)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}

	head := sv.extractHeadPredicate(trimmed)
	isRule := strings.Contains(trimmed, ":-")

	if head != "" {
		if reason, forbidden := forbiddenLearnedHeads[head]; forbidden {
			return fmt.Errorf("learned rule defines protected predicate %q: %s", head, reason)
		}

		// Schema drift check for learned FACTS only (not rules).
		// A learned fact like undeclared_pred("oops"). asserts data for a predicate with no Decl —
		// it will never be consumed by any rule. But a learned rule like
		// candidate_action(/x) :- user_intent(...) legitimately derives a new predicate.
		if !isRule && !sv.declaredPredicates[head] && !sv.isBuiltin(head) {
			return fmt.Errorf("undeclared predicate %q in learned fact (available: %v)", head, sv.getAvailablePredicates())
		}

		// Validate arity of head predicate against declared schema
		if err := sv.validateHeadArity(trimmed, head); err != nil {
			return err
		}
	}

	return sv.ValidateRule(ruleText)
}

// HotLoadRule validates a single rule via syntax parsing and schema checks.
// This mirrors the RuleValidator contract used by the feedback loop.
func (sv *SchemaValidator) HotLoadRule(rule string) error {
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return fmt.Errorf("empty rule")
	}
	if !strings.HasSuffix(trimmed, ".") {
		trimmed += "."
	}
	if _, err := ParseUnit(strings.NewReader(trimmed)); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return sv.ValidateLearnedRule(trimmed)
}

// validateHeadArity checks that the head predicate's argument count matches the schema.
func (sv *SchemaValidator) validateHeadArity(line, headName string) error {
	expectedArity, hasDeclaredArity := sv.predicateArities[headName]
	if !hasDeclaredArity {
		// Not declared, skip arity check (schema validation will catch undefined)
		return nil
	}

	// Extract actual arity from the line: predicate_name(arg1, arg2, ...)
	// Find the opening paren after the predicate name
	_, after, ok := strings.Cut(line, headName)
	if !ok {
		return nil
	}

	// Find the argument list
	afterHead := after
	parenStart := strings.Index(afterHead, "(")
	if parenStart < 0 {
		return nil
	}

	// Find matching close paren (handle nested parens)
	depth := 0
	argStart := parenStart + 1
	argEnd := -1
	for i, c := range afterHead[parenStart:] {
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 {
				argEnd = parenStart + i
				break
			}
		}
	}

	if argEnd < 0 {
		return nil // Malformed, let parse handle it
	}

	argsStr := strings.TrimSpace(afterHead[argStart:argEnd])
	actualArity := countTopLevelArgs(argsStr)

	if actualArity != expectedArity {
		return fmt.Errorf("arity mismatch: %s has %d args but schema declares %d",
			headName, actualArity, expectedArity)
	}

	return nil
}

// countTopLevelArgs counts comma-separated arguments at paren depth 0.
// A naive comma count breaks on nested terms (foo(bar(1,2), X) has two
// args, not three); both schema extraction and head validation share this.
func countTopLevelArgs(argsStr string) int {
	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return 0
	}
	depth, count := 0, 1
	for _, c := range argsStr {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

var forbiddenLearnedHeads = map[string]string{
	// Constitutional gate is core-owned; learned rules must not grant permissions.
	"permitted":       "constitutional permission is core-owned (do not learn permissions)",
	"safe_action":     "constitutional allowlist is core-owned (do not extend via learned rules)",
	"admin_override":  "approvals must be user/admin driven, not learned",
	"signed_approval": "approvals must be user/admin driven, not learned",

	// Runtime pipeline facts are produced by system shards; learned rules must not spoof them.
	"pending_action":          "produced by executive_policy shard",
	"permitted_action":        "produced by constitution_gate shard",
	"permission_check_result": "produced by constitution_gate shard",
	"routing_result":          "produced by tactile_router shard",
	"execution_result":        "produced by virtual_store",
	"system_shard_state":      "produced by system shard supervisor",
}

var declHeadPattern = regexp.MustCompile(`(?m)^Decl\s+([a-z_][a-z0-9_]*)\s*\(`)

var headPattern = regexp.MustCompile(`(?m)^([a-z_][a-z0-9_]*)\s*\(`)

var predicatePattern = regexp.MustCompile(`([a-z_][a-z0-9_]*)\s*\(`)

var learnedHeadPattern = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s*\(`)

func (sv *SchemaValidator) extractHeadPredicate(line string) string {
	match := learnedHeadPattern.FindStringSubmatch(line)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

// splitRuleBody splits a rule into head and body at the first :- outside
// string literals. A naive strings.Split misfires when ":-" appears inside
// a quoted atom argument, validating a fragment of the head as the body.
func splitRuleBody(ruleText string) (body string, isRule bool) {
	inString, escaped := false, false
	for i := 0; i+1 < len(ruleText); i++ {
		c := ruleText[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == ':' && ruleText[i+1] == '-' {
			return ruleText[i+2:], true
		}
	}
	return "", false
}

// ValidateRules validates multiple rules at once.
func (sv *SchemaValidator) ValidateRules(rules []string) []error {
	var errors []error
	if len(rules) > 0 {
		errors = make([]error, 0, len(rules))
	}
	for i, rule := range rules {
		if err := sv.ValidateRule(rule); err != nil {
			errors = append(errors, fmt.Errorf("rule %d: %w", i+1, err))
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// ValidateProgram validates an entire Mangle program text.
//
// The analysis result used to be computed and then thrown away — `_ = programInfo`
// under the comment "Suppress 'programInfo declared but not used' error by using
// it" — while the actual check was strings.Split(programText, "\n") plus a
// substring test for ":-". That stand-in was blind in both directions. A rule
// wrapped across lines (which is every non-trivial rule in defaults/) had only
// its FIRST fragment validated: continuation lines carry no ":-" so they were
// skipped entirely, and the fragment's truncated body hid every predicate below
// the wrap. In the other direction, a ":-" inside a string literal or a trailing
// comment was validated as though it were a rule. A nil return from whole-program
// validation therefore meant close to nothing.
//
// The parser has already produced the clauses; use them. Premise predicates now
// come from the desugared AST, which has resolved wrapping, negation, string
// literals and comments before we look at a single name.
//
// Scope, so callers know what a nil return buys: this checks that every body
// predicate has a data source (Bug #18, schema drift). Syntax errors come from
// ParseUnit and binding/stratification errors from AnalyzeOneUnit; both are
// returned verbatim above.
func (sv *SchemaValidator) ValidateProgram(programText string) error {
	parsed, err := ParseUnit(strings.NewReader(programText))
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// A body predicate is sourced if the system schema declares it, if Mangle
	// builds it in, or if THIS program produces it (rule head or standalone
	// fact). Anything else is drift: analysis accepts the rule because the
	// program declared the predicate locally, and the rule then never fires
	// because nothing in the running system ever asserts it.
	produced := make(map[string]bool, len(programInfo.Rules)+len(programInfo.InitialFacts))
	for _, rule := range programInfo.Rules {
		produced[rule.Head.Predicate.Symbol] = true
	}
	for _, fact := range programInfo.InitialFacts {
		produced[fact.Predicate.Symbol] = true
	}

	var errors []string
	for _, rule := range programInfo.Rules {
		for _, name := range sv.unsourcedPremisePredicates(rule, produced) {
			errors = append(errors, fmt.Sprintf("rule %s uses undefined predicate %q (available: %v)",
				rule.Head.Predicate.Symbol, name, sv.getAvailablePredicates()))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// unsourcedPremisePredicates returns the body predicates of one clause that no
// schema declares, no builtin provides, and this program does not produce.
// Reported in premise order, once per name per clause.
func (sv *SchemaValidator) unsourcedPremisePredicates(rule ast.Clause, produced map[string]bool) []string {
	var undefined []string
	seen := make(map[string]bool)

	for _, premise := range rule.Premises {
		for _, sym := range premisePredicates(premise) {
			name := sym.Symbol
			if seen[name] {
				continue
			}
			// Builtins (:lt, :match_prefix, ...) and the synthetic predicates
			// analysis generates while desugaring transforms have no Decl and
			// are not drift.
			if sym.IsBuiltin() || sym.IsInternalPredicate() || sv.isBuiltin(name) {
				continue
			}
			if sv.declaredPredicates[name] || produced[name] {
				continue
			}
			seen[name] = true
			undefined = append(undefined, name)
		}
	}

	return undefined
}

// premisePredicates returns the predicate symbols a single premise references.
// Eq, Ineq and transform statements carry no predicate and yield nothing.
func premisePredicates(term ast.Term) []ast.PredicateSym {
	switch t := term.(type) {
	case ast.Atom:
		return []ast.PredicateSym{t.Predicate}
	case ast.NegAtom:
		return []ast.PredicateSym{t.Atom.Predicate}
	case ast.TemporalAtom:
		return []ast.PredicateSym{t.Atom.Predicate}
	case ast.TemporalLiteral:
		return premisePredicates(t.Literal)
	}
	return nil
}

// mangleBuiltins is the set of built-in Mangle operators (package-level to avoid per-call allocation).
var mangleBuiltins = map[string]bool{
	"count":   true,
	"sum":     true,
	"min":     true,
	"max":     true,
	"avg":     true,
	"bound":   true,
	"applyFn": true,
	"fn":      true,
	"match":   true,
	"collect": true,
}

// isBuiltin checks if a predicate is a built-in Mangle operator.
func (sv *SchemaValidator) isBuiltin(predicate string) bool {
	return mangleBuiltins[predicate]
}

// getAvailablePredicates returns a sorted list of available predicates for error messages.
func (sv *SchemaValidator) getAvailablePredicates() []string {
	predicates := make([]string, 0, len(sv.declaredPredicates))
	for p := range sv.declaredPredicates {
		predicates = append(predicates, p)
	}
	sort.Strings(predicates)
	return predicates
}

// IsDeclared checks if a predicate is declared.
func (sv *SchemaValidator) IsDeclared(predicate string) bool {
	return sv.declaredPredicates[predicate]
}

// GetDeclaredPredicates returns all declared predicate names.
func (sv *SchemaValidator) GetDeclaredPredicates() []string {
	return sv.getAvailablePredicates()
}

// GetArity returns the expected arity for a predicate, or -1 if unknown.
func (sv *SchemaValidator) GetArity(predicate string) int {
	if arity, ok := sv.predicateArities[predicate]; ok {
		return arity
	}
	return -1
}

// CheckArity validates that a predicate is called with the correct number of arguments.
// Returns nil if arity matches or is unknown, error otherwise.
func (sv *SchemaValidator) CheckArity(predicate string, actualArity int) error {
	expectedArity := sv.GetArity(predicate)
	if expectedArity < 0 {
		// Unknown arity - skip check
		return nil
	}
	if expectedArity != actualArity {
		return fmt.Errorf("arity mismatch for %s: expected %d arguments, got %d",
			predicate, expectedArity, actualArity)
	}
	return nil
}

// SetPredicateArity sets the expected arity for a predicate.
// This can be used to load arities from corpus or other sources.
func (sv *SchemaValidator) SetPredicateArity(predicate string, arity int) {
	sv.predicateArities[predicate] = arity
}
