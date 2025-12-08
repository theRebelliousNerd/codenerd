package mangle

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ============================================================================
// Grammar-Constrained Decoding (GCD) for Mangle Syntax
// Cortex 1.5.0 §1.1: Prevents "Hallucination of Agency" by enforcing valid atoms
// ============================================================================

// AtomValidator validates Mangle atom syntax.
// This implements the "Grammar Constrained Decoding" concept from the spec.
type AtomValidator struct {
	// ValidPredicates is the set of known predicates from schemas.mg
	ValidPredicates map[string]PredicateSpec

	// ValidNameConstants is the set of valid /name constants
	ValidNameConstants map[string]bool
}

// PredicateSpec describes a predicate's expected arity and argument types.
type PredicateSpec struct {
	Name  string
	Arity int
	Args  []ArgSpec
}

// ArgSpec describes an argument's expected type.
type ArgSpec struct {
	Name     string
	Type     ArgType
	Optional bool
}

// ArgType represents Mangle argument types.
type ArgType int

const (
	ArgTypeAny      ArgType = iota
	ArgTypeName             // /name_constant
	ArgTypeString           // "quoted string"
	ArgTypeNumber           // numeric value
	ArgTypeVariable         // Uppercase Variable
	ArgTypeBool             // true/false
)

// ValidationResult contains the result of atom validation.
type ValidationResult struct {
	Valid    bool
	Atom     string
	Errors   []ValidationError
	Repaired string // Suggested repair if invalid
}

// ValidationError describes a specific validation error.
type ValidationError struct {
	Position int
	Message  string
	Severity ErrorSeverity
}

// ErrorSeverity indicates how severe a validation error is.
type ErrorSeverity int

const (
	SeverityWarning ErrorSeverity = iota
	SeverityError
	SeverityFatal
)

// NewAtomValidator creates a validator with codeNERD's schema predicates.
func NewAtomValidator() *AtomValidator {
	v := &AtomValidator{
		ValidPredicates:    make(map[string]PredicateSpec),
		ValidNameConstants: make(map[string]bool),
	}
	v.loadCorePredicates()
	v.loadCoreNameConstants()
	return v
}

// loadCorePredicates loads the core predicates from schemas.mg.
func (v *AtomValidator) loadCorePredicates() {
	// Intent & Focus (§3)
	v.ValidPredicates["user_intent"] = PredicateSpec{
		Name: "user_intent", Arity: 5,
		Args: []ArgSpec{
			{Name: "ID", Type: ArgTypeName},
			{Name: "Category", Type: ArgTypeName},
			{Name: "Verb", Type: ArgTypeName},
			{Name: "Target", Type: ArgTypeString},
			{Name: "Constraint", Type: ArgTypeString},
		},
	}
	v.ValidPredicates["focus_resolution"] = PredicateSpec{
		Name: "focus_resolution", Arity: 4,
		Args: []ArgSpec{
			{Name: "RawRef", Type: ArgTypeString},
			{Name: "Path", Type: ArgTypeString},
			{Name: "Symbol", Type: ArgTypeString},
			{Name: "Confidence", Type: ArgTypeNumber},
		},
	}
	v.ValidPredicates["ambiguity_flag"] = PredicateSpec{
		Name: "ambiguity_flag", Arity: 3,
		Args: []ArgSpec{
			{Name: "MissingParam", Type: ArgTypeString},
			{Name: "ContextClue", Type: ArgTypeString},
			{Name: "Hypothesis", Type: ArgTypeString},
		},
	}

	// File System (§4.1)
	v.ValidPredicates["file_topology"] = PredicateSpec{
		Name: "file_topology", Arity: 5,
		Args: []ArgSpec{
			{Name: "Path", Type: ArgTypeString},
			{Name: "Hash", Type: ArgTypeString},
			{Name: "Language", Type: ArgTypeName},
			{Name: "LastModified", Type: ArgTypeNumber},
			{Name: "IsTestFile", Type: ArgTypeBool},
		},
	}
	v.ValidPredicates["file_content"] = PredicateSpec{
		Name: "file_content", Arity: 4,
		Args: []ArgSpec{
			{Name: "Path", Type: ArgTypeString},
			{Name: "StartLine", Type: ArgTypeNumber},
			{Name: "EndLine", Type: ArgTypeNumber},
			{Name: "Content", Type: ArgTypeString},
		},
	}

	// Diagnostics (§4.2)
	v.ValidPredicates["diagnostic"] = PredicateSpec{
		Name: "diagnostic", Arity: 5,
		Args: []ArgSpec{
			{Name: "Severity", Type: ArgTypeName},
			{Name: "FilePath", Type: ArgTypeString},
			{Name: "Line", Type: ArgTypeNumber},
			{Name: "ErrorCode", Type: ArgTypeString},
			{Name: "Message", Type: ArgTypeString},
		},
	}

	// Symbol Graph (§4.3)
	v.ValidPredicates["symbol_graph"] = PredicateSpec{
		Name: "symbol_graph", Arity: 5,
		Args: []ArgSpec{
			{Name: "SymbolID", Type: ArgTypeString},
			{Name: "Type", Type: ArgTypeName},
			{Name: "Visibility", Type: ArgTypeName},
			{Name: "DefinedAt", Type: ArgTypeString},
			{Name: "Signature", Type: ArgTypeString},
		},
	}
	v.ValidPredicates["dependency_link"] = PredicateSpec{
		Name: "dependency_link", Arity: 3,
		Args: []ArgSpec{
			{Name: "CallerID", Type: ArgTypeString},
			{Name: "CalleeID", Type: ArgTypeString},
			{Name: "ImportPath", Type: ArgTypeString},
		},
	}

	// State & Actions (§5)
	v.ValidPredicates["test_state"] = PredicateSpec{
		Name: "test_state", Arity: 1,
		Args: []ArgSpec{{Name: "State", Type: ArgTypeName}},
	}
	v.ValidPredicates["next_action"] = PredicateSpec{
		Name: "next_action", Arity: 1,
		Args: []ArgSpec{{Name: "Action", Type: ArgTypeName}},
	}
	v.ValidPredicates["permitted"] = PredicateSpec{
		Name: "permitted", Arity: 1,
		Args: []ArgSpec{{Name: "Action", Type: ArgTypeName}},
	}

	// Observations & Memory
	v.ValidPredicates["observation"] = PredicateSpec{
		Name: "observation", Arity: 2,
		Args: []ArgSpec{
			{Name: "Key", Type: ArgTypeName},
			{Name: "Value", Type: ArgTypeString},
		},
	}
	v.ValidPredicates["preference"] = PredicateSpec{
		Name: "preference", Arity: 2,
		Args: []ArgSpec{
			{Name: "Key", Type: ArgTypeName},
			{Name: "Value", Type: ArgTypeAny},
		},
	}

	// Shard Management (§7)
	v.ValidPredicates["shard_profile"] = PredicateSpec{
		Name: "shard_profile", Arity: 4,
		Args: []ArgSpec{
			{Name: "AgentName", Type: ArgTypeName},
			{Name: "Description", Type: ArgTypeString},
			{Name: "Topics", Type: ArgTypeString},
			{Name: "Tools", Type: ArgTypeString},
		},
	}
	v.ValidPredicates["delegate_task"] = PredicateSpec{
		Name: "delegate_task", Arity: 3,
		Args: []ArgSpec{
			{Name: "ShardType", Type: ArgTypeName},
			{Name: "Task", Type: ArgTypeString},
			{Name: "Result", Type: ArgTypeVariable},
		},
	}

	// Research & Knowledge (§9)
	v.ValidPredicates["knowledge_atom"] = PredicateSpec{
		Name: "knowledge_atom", Arity: 4,
		Args: []ArgSpec{
			{Name: "SourceURL", Type: ArgTypeString},
			{Name: "Concept", Type: ArgTypeString},
			{Name: "CodePattern", Type: ArgTypeString},
			{Name: "AntiPattern", Type: ArgTypeString},
		},
	}
	v.ValidPredicates["research_topic"] = PredicateSpec{
		Name: "research_topic", Arity: 3,
		Args: []ArgSpec{
			{Name: "AgentName", Type: ArgTypeName},
			{Name: "Topic", Type: ArgTypeString},
			{Name: "Status", Type: ArgTypeName},
		},
	}
}

// UpdateFromSchema updates ValidPredicates by parsing Decl statements from a schema string.
func (v *AtomValidator) UpdateFromSchema(schema string) error {
	// Simple regex-based parser for getting Decls to populate TypeMap
	// Pattern: Decl predicate(Type, Type).
	// Types: Name, String, Number, etc. (mapped to ArgType)

	// Normalize newlines
	schema = strings.ReplaceAll(schema, "\r\n", "\n")
	lines := strings.Split(schema, "\n")

	declRe := regexp.MustCompile(`^Decl\s+([a-z][a-z0-9_]*)\((.*)\)\.`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		matches := declRe.FindStringSubmatch(line)
		if Matches := matches; len(Matches) == 3 {
			predName := Matches[1]
			argsStr := Matches[2]

			// Parse args
			argParts := splitArgs(argsStr)
			var argSpecs []ArgSpec

			for i, argTypeStr := range argParts {
				argTypeStr = strings.TrimSpace(argTypeStr)
				typ := parseArgTypeFromSchema(argTypeStr)

				argSpecs = append(argSpecs, ArgSpec{
					Name: fmt.Sprintf("Arg%d", i),
					Type: typ,
				})
			}

			v.ValidPredicates[predName] = PredicateSpec{
				Name:  predName,
				Arity: len(argSpecs),
				Args:  argSpecs,
			}
		}
	}
	return nil
}

// splitArgs splits by comma, respecting parentheses if any (though simpl types usually don't have them)
func splitArgs(s string) []string {
	return strings.Split(s, ",")
}

// parseArgTypeFromSchema maps schema type names to ArgType
func parseArgTypeFromSchema(s string) ArgType {
	s = strings.TrimSpace(s)
	switch s {
	case "Name", "name":
		return ArgTypeName
	case "String", "string":
		return ArgTypeString
	case "Number", "number", "Int", "int", "Float", "float":
		return ArgTypeNumber
	case "Bool", "bool":
		return ArgTypeBool
	case "Any", "any":
		return ArgTypeAny
	default:
		return ArgTypeAny
	}
}

// loadCoreNameConstants loads valid /name constants.
func (v *AtomValidator) loadCoreNameConstants() {
	// Intent categories
	v.ValidNameConstants["/query"] = true
	v.ValidNameConstants["/mutation"] = true
	v.ValidNameConstants["/instruction"] = true

	// Intent verbs
	verbs := []string{
		"/explain", "/refactor", "/debug", "/generate", "/init",
		"/research", "/fix", "/test", "/delete", "/create",
		"/search", "/configure", "/read", "/write", "/review",
	}
	for _, verb := range verbs {
		v.ValidNameConstants[verb] = true
	}

	// Languages
	languages := []string{"/go", "/python", "/ts", "/rust", "/java", "/js", "/c", "/cpp"}
	for _, lang := range languages {
		v.ValidNameConstants[lang] = true
	}

	// Severities
	v.ValidNameConstants["/panic"] = true
	v.ValidNameConstants["/error"] = true
	v.ValidNameConstants["/warning"] = true
	v.ValidNameConstants["/info"] = true

	// Symbol types
	v.ValidNameConstants["/function"] = true
	v.ValidNameConstants["/class"] = true
	v.ValidNameConstants["/interface"] = true
	v.ValidNameConstants["/variable"] = true
	v.ValidNameConstants["/constant"] = true
	v.ValidNameConstants["/struct"] = true
	v.ValidNameConstants["/method"] = true

	// Visibility
	v.ValidNameConstants["/public"] = true
	v.ValidNameConstants["/private"] = true
	v.ValidNameConstants["/protected"] = true

	// Test states
	v.ValidNameConstants["/passing"] = true
	v.ValidNameConstants["/failing"] = true
	v.ValidNameConstants["/compiling"] = true
	v.ValidNameConstants["/unknown"] = true

	// Actions
	actions := []string{
		"/read_error_log", "/analyze_root_cause", "/generate_patch",
		"/run_tests", "/escalate_to_user", "/commit", "/rollback",
	}
	for _, action := range actions {
		v.ValidNameConstants[action] = true
	}

	// Shard types
	v.ValidNameConstants["/generalist"] = true
	v.ValidNameConstants["/specialist"] = true
	v.ValidNameConstants["/researcher"] = true
	v.ValidNameConstants["/coder"] = true
	v.ValidNameConstants["/tester"] = true
	v.ValidNameConstants["/reviewer"] = true
	v.ValidNameConstants["/tool_generator"] = true

	// Autopoiesis verbs
	v.ValidNameConstants["/generate_tool"] = true
	v.ValidNameConstants["/refine_tool"] = true
	v.ValidNameConstants["/list_tools"] = true
	v.ValidNameConstants["/tool_status"] = true
	v.ValidNameConstants["/delegate_tool_generator"] = true

	// Research status
	v.ValidNameConstants["/pending"] = true
	v.ValidNameConstants["/in_progress"] = true
	v.ValidNameConstants["/complete"] = true
	v.ValidNameConstants["/failed"] = true

	// Generic
	v.ValidNameConstants["/true"] = true
	v.ValidNameConstants["/false"] = true
	v.ValidNameConstants["/none"] = true
	v.ValidNameConstants["/current_intent"] = true
}

// ValidateAtom validates a single Mangle atom string.
func (v *AtomValidator) ValidateAtom(atom string) ValidationResult {
	result := ValidationResult{
		Atom:   atom,
		Valid:  true,
		Errors: []ValidationError{},
	}

	atom = strings.TrimSpace(atom)
	if atom == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Position: 0,
			Message:  "empty atom",
			Severity: SeverityFatal,
		})
		return result
	}

	// Remove trailing period if present
	atom = strings.TrimSuffix(atom, ".")

	// Parse predicate name
	parenIdx := strings.Index(atom, "(")
	if parenIdx == -1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Position: 0,
			Message:  "missing opening parenthesis",
			Severity: SeverityFatal,
		})
		return result
	}

	predicate := atom[:parenIdx]
	if !isValidPredicate(predicate) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Position: 0,
			Message:  fmt.Sprintf("invalid predicate name '%s': must be lowercase identifier", predicate),
			Severity: SeverityError,
		})
	}

	// Check if predicate is known
	spec, known := v.ValidPredicates[predicate]
	if !known {
		result.Errors = append(result.Errors, ValidationError{
			Position: 0,
			Message:  fmt.Sprintf("unknown predicate '%s'", predicate),
			Severity: SeverityWarning,
		})
	}

	// Parse arguments
	if !strings.HasSuffix(atom, ")") {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Position: len(atom),
			Message:  "missing closing parenthesis",
			Severity: SeverityFatal,
		})
		return result
	}

	argsStr := atom[parenIdx+1 : len(atom)-1]
	args := parseAtomArgs(argsStr)

	// Validate arity if predicate is known
	if known && len(args) != spec.Arity {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Position: parenIdx,
			Message:  fmt.Sprintf("wrong arity for '%s': expected %d args, got %d", predicate, spec.Arity, len(args)),
			Severity: SeverityError,
		})
	}

	// Validate each argument
	for i, arg := range args {
		argErrors := v.validateArg(arg, i, spec, known)
		result.Errors = append(result.Errors, argErrors...)
		for _, err := range argErrors {
			if err.Severity >= SeverityError {
				result.Valid = false
			}
		}
	}

	// Attempt repair if invalid
	if !result.Valid {
		result.Repaired = v.attemptRepair(atom, result.Errors)
	}

	return result
}

// validateArg validates a single argument.
func (v *AtomValidator) validateArg(arg string, idx int, spec PredicateSpec, known bool) []ValidationError {
	var errors []ValidationError
	arg = strings.TrimSpace(arg)

	if arg == "" {
		errors = append(errors, ValidationError{
			Position: idx,
			Message:  fmt.Sprintf("argument %d is empty", idx+1),
			Severity: SeverityError,
		})
		return errors
	}

	// Determine actual type
	actualType := inferArgType(arg)

	// Check type constraint if predicate is known
	if known && idx < len(spec.Args) {
		expectedType := spec.Args[idx].Type
		if expectedType != ArgTypeAny && !compatibleTypes(actualType, expectedType) {
			errors = append(errors, ValidationError{
				Position: idx,
				Message:  fmt.Sprintf("argument %d (%s): expected %s, got %s", idx+1, spec.Args[idx].Name, typeString(expectedType), typeString(actualType)),
				Severity: SeverityWarning,
			})
		}
	}

	// Validate name constants
	if actualType == ArgTypeName {
		if !v.ValidNameConstants[arg] {
			errors = append(errors, ValidationError{
				Position: idx,
				Message:  fmt.Sprintf("unknown name constant '%s'", arg),
				Severity: SeverityWarning,
			})
		}
	}

	// Validate string syntax
	if actualType == ArgTypeString {
		if !strings.HasPrefix(arg, "\"") || !strings.HasSuffix(arg, "\"") {
			errors = append(errors, ValidationError{
				Position: idx,
				Message:  fmt.Sprintf("malformed string argument %d", idx+1),
				Severity: SeverityError,
			})
		}
	}

	return errors
}

// attemptRepair tries to fix common syntax errors.
func (v *AtomValidator) attemptRepair(atom string, errors []ValidationError) string {
	repaired := atom

	for _, err := range errors {
		switch {
		case strings.Contains(err.Message, "missing closing parenthesis"):
			repaired = repaired + ")"
		case strings.Contains(err.Message, "missing opening parenthesis"):
			// Try to find predicate and add ()
			if idx := strings.Index(repaired, " "); idx > 0 {
				repaired = repaired[:idx] + "()" + repaired[idx:]
			}
		case strings.Contains(err.Message, "malformed string"):
			// Try to fix unquoted strings
			repaired = fixUnquotedStrings(repaired)
		}
	}

	return repaired
}

// ValidateAtoms validates multiple atoms and returns all results.
func (v *AtomValidator) ValidateAtoms(atoms []string) []ValidationResult {
	results := make([]ValidationResult, len(atoms))
	for i, atom := range atoms {
		results[i] = v.ValidateAtom(atom)
	}
	return results
}

// ============================================================================
// Helper Functions
// ============================================================================

// isValidPredicate checks if a string is a valid Mangle predicate name.
func isValidPredicate(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must start with lowercase letter
	if !unicode.IsLower(rune(s[0])) {
		return false
	}
	// Must contain only alphanumeric and underscore
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// parseAtomArgs splits argument string respecting quotes and parentheses.
func parseAtomArgs(argsStr string) []string {
	var args []string
	var current strings.Builder
	depth := 0
	inQuote := false

	for i := 0; i < len(argsStr); i++ {
		c := argsStr[i]

		if c == '"' && (i == 0 || argsStr[i-1] != '\\') {
			inQuote = !inQuote
		}

		if !inQuote {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
			} else if c == ',' && depth == 0 {
				args = append(args, current.String())
				current.Reset()
				continue
			}
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// inferArgType determines the type of an argument.
func inferArgType(arg string) ArgType {
	arg = strings.TrimSpace(arg)

	// Name constant starts with /
	if strings.HasPrefix(arg, "/") {
		return ArgTypeName
	}

	// String is quoted
	if strings.HasPrefix(arg, "\"") {
		return ArgTypeString
	}

	// Variable starts with uppercase
	if len(arg) > 0 && unicode.IsUpper(rune(arg[0])) {
		return ArgTypeVariable
	}

	// Boolean
	if arg == "true" || arg == "false" {
		return ArgTypeBool
	}

	// Number
	if isNumeric(arg) {
		return ArgTypeNumber
	}

	return ArgTypeAny
}

// isNumeric checks if a string represents a number.
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, s)
	return matched
}

// compatibleTypes checks if actual type is compatible with expected.
func compatibleTypes(actual, expected ArgType) bool {
	if expected == ArgTypeAny {
		return true
	}
	return actual == expected
}

// typeString returns a human-readable type name.
func typeString(t ArgType) string {
	switch t {
	case ArgTypeName:
		return "name constant (/...)"
	case ArgTypeString:
		return "quoted string"
	case ArgTypeNumber:
		return "number"
	case ArgTypeVariable:
		return "variable (Uppercase)"
	case ArgTypeBool:
		return "boolean"
	default:
		return "any"
	}
}

// fixUnquotedStrings attempts to quote unquoted string arguments.
func fixUnquotedStrings(atom string) string {
	// Simple heuristic: find unquoted multi-word args and quote them
	// This is a best-effort repair
	re := regexp.MustCompile(`\(([^"/)][^,)]*)\)`)
	return re.ReplaceAllStringFunc(atom, func(match string) string {
		inner := match[1 : len(match)-1]
		if !strings.HasPrefix(inner, "\"") && !strings.HasPrefix(inner, "/") && !isNumeric(inner) {
			return "(\"" + inner + "\")"
		}
		return match
	})
}

// ============================================================================
// GCD Repair Loop
// ============================================================================

// RepairLoop implements the GCD repair mechanism.
// If atoms are invalid, it returns an error prompt for the LLM to self-correct.
type RepairLoop struct {
	Validator  *AtomValidator
	MaxRetries int
}

// NewRepairLoop creates a new repair loop with default settings.
func NewRepairLoop() *RepairLoop {
	return &RepairLoop{
		Validator:  NewAtomValidator(),
		MaxRetries: 3,
	}
}

// ValidateAndRepair validates atoms and generates repair prompts if needed.
func (r *RepairLoop) ValidateAndRepair(atoms []string) ([]string, error, string) {
	results := r.Validator.ValidateAtoms(atoms)

	var validAtoms []string
	var invalidAtoms []ValidationResult

	for _, result := range results {
		if result.Valid {
			validAtoms = append(validAtoms, result.Atom)
		} else {
			invalidAtoms = append(invalidAtoms, result)
		}
	}

	if len(invalidAtoms) == 0 {
		return validAtoms, nil, ""
	}

	// Generate repair prompt
	repairPrompt := r.generateRepairPrompt(invalidAtoms)

	return validAtoms, fmt.Errorf("%d invalid atoms", len(invalidAtoms)), repairPrompt
}

// generateRepairPrompt creates a prompt to help the LLM fix invalid atoms.
func (r *RepairLoop) generateRepairPrompt(invalid []ValidationResult) string {
	var sb strings.Builder

	sb.WriteString("MANGLE SYNTAX ERROR - Please correct the following atoms:\n\n")

	for _, result := range invalid {
		sb.WriteString(fmt.Sprintf("Invalid: %s\n", result.Atom))
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", err.Message))
		}
		if result.Repaired != "" {
			sb.WriteString(fmt.Sprintf("  Suggestion: %s\n", result.Repaired))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("MANGLE SYNTAX RULES:\n")
	sb.WriteString("- Predicates must be lowercase_with_underscores\n")
	sb.WriteString("- Name constants start with / (e.g., /query, /mutation)\n")
	sb.WriteString("- Strings must be double-quoted (e.g., \"hello\")\n")
	sb.WriteString("- Variables start with uppercase (e.g., Result, X)\n")
	sb.WriteString("- Atoms end with period: predicate(arg1, arg2).\n")

	return sb.String()
}
