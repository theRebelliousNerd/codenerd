package mangle

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/parse"
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
	// Use regex to extract Decl statements with full argument list
	// Pattern: Decl predicate_name(args...).
	declPattern := regexp.MustCompile(`(?m)^Decl\s+([a-z_][a-z0-9_]*)\s*\(([^)]*)\)`)
	matches := declPattern.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			predicate := match[1]
			sv.declaredPredicates[predicate] = true

			// Count arguments for arity validation
			if len(match) > 2 {
				argsStr := strings.TrimSpace(match[2])
				if argsStr == "" {
					sv.predicateArities[predicate] = 0
				} else {
					// Count commas + 1 = number of args
					sv.predicateArities[predicate] = strings.Count(argsStr, ",") + 1
				}
			}
		}
	}

	return nil
}

// extractHeadPredicatesFromText extracts predicates that are defined as rule heads.
// These are implicitly declared by being derived.
func (sv *SchemaValidator) extractHeadPredicatesFromText(text string) error {
	// Pattern: predicate(args) :- ...
	// or: predicate(args).
	headPattern := regexp.MustCompile(`(?m)^([a-z_][a-z0-9_]*)\s*\(`)
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
	// Extract predicates from rule body (everything after :-)
	parts := strings.Split(ruleText, ":-")
	if len(parts) < 2 {
		// Fact, not a rule - no body to validate
		return nil
	}

	body := parts[1]

	// Extract all predicate calls from body
	// Pattern: predicate_name(
	predicatePattern := regexp.MustCompile(`([a-z_][a-z0-9_]*)\s*\(`)
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
// Also validates that head predicates match declared arities.
func (sv *SchemaValidator) ValidateLearnedRule(ruleText string) error {
	trimmed := strings.TrimSpace(ruleText)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}

	head := sv.extractHeadPredicate(trimmed)
	if head != "" {
		if reason, forbidden := forbiddenLearnedHeads[head]; forbidden {
			return fmt.Errorf("learned rule defines protected predicate %q: %s", head, reason)
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
	if _, err := parse.Unit(strings.NewReader(trimmed)); err != nil {
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
	headIdx := strings.Index(line, headName)
	if headIdx < 0 {
		return nil
	}

	// Find the argument list
	afterHead := line[headIdx+len(headName):]
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
	actualArity := 0
	if argsStr != "" {
		// Count args by tracking commas at depth 0
		depth = 0
		actualArity = 1
		for _, c := range argsStr {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
			} else if c == ',' && depth == 0 {
				actualArity++
			}
		}
	}

	if actualArity != expectedArity {
		return fmt.Errorf("arity mismatch: %s has %d args but schema declares %d",
			headName, actualArity, expectedArity)
	}

	return nil
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

var learnedHeadPattern = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s*\(`)

func (sv *SchemaValidator) extractHeadPredicate(line string) string {
	match := learnedHeadPattern.FindStringSubmatch(line)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

// ValidateRules validates multiple rules at once.
func (sv *SchemaValidator) ValidateRules(rules []string) []error {
	var errors []error
	for i, rule := range rules {
		if err := sv.ValidateRule(rule); err != nil {
			errors = append(errors, fmt.Errorf("rule %d: %w", i+1, err))
		}
	}
	return errors
}

// ValidateProgram validates an entire Mangle program text.
// Returns errors for each invalid rule.
func (sv *SchemaValidator) ValidateProgram(programText string) error {
	// Parse the program to extract individual rules
	parsed, err := parse.Unit(strings.NewReader(programText))
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Analyze to get program info
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Extract rules from the analyzed program
	// For now, we'll use a simpler approach: split on lines and validate each rule-like line
	lines := strings.Split(programText, "\n")
	var errors []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Check if this is a rule (contains :-)
		if strings.Contains(line, ":-") {
			if err := sv.ValidateRule(line); err != nil {
				errors = append(errors, fmt.Sprintf("line %d: %v", i+1, err))
			}
		}
	}

	// Suppress "programInfo declared but not used" error by using it
	_ = programInfo

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// isBuiltin checks if a predicate is a built-in Mangle operator.
func (sv *SchemaValidator) isBuiltin(predicate string) bool {
	builtins := map[string]bool{
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
	return builtins[predicate]
}

// getAvailablePredicates returns a sorted list of available predicates for error messages.
func (sv *SchemaValidator) getAvailablePredicates() []string {
	var predicates []string
	for p := range sv.declaredPredicates {
		predicates = append(predicates, p)
	}
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
