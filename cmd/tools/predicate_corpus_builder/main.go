// Package main implements the predicate corpus builder tool for creating a baked-in
// database of all Mangle predicates with their signatures, types, categories, and
// usage examples. This corpus enables schema validation and JIT predicate selection.
//
// Usage: go run ./cmd/tools/predicate_corpus_builder
//
// Build with sqlite-vec support:
//
//	$env:CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers"; go run ./cmd/tools/predicate_corpus_builder
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const (
	// outputPath for the generated database
	outputPath = "internal/core/defaults/predicate_corpus.db"

	// schemasPath for the main schema file
	schemasPath = "internal/core/defaults/schemas.mg"

	// policyPath for the policy file (IDB rules)
	policyPath = "internal/core/defaults/policy.mg"

	// skillPath for mangle-programming skill resources
	skillPath = ".claude/skills/mangle-programming/references"
)

// PredicateEntry represents a single predicate definition.
type PredicateEntry struct {
	Name         string
	Arity        int
	Type         string // "EDB" or "IDB"
	Category     string // e.g., "core", "shard", "campaign", "safety"
	Description  string
	SafetyLevel  string // "safe", "requires_binding", "negation_sensitive", "stratification_critical"
	Domain       string // For JIT selection: "core", "shard_coder", "campaign", etc.
	Section      string // Source section header
	SourceFile   string
	ArgumentDefs []ArgumentDef
}

// ArgumentDef represents a single argument of a predicate.
type ArgumentDef struct {
	Position       int
	Name           string
	Type           string // "atom", "string", "number", "list", "map", "any"
	IsBoundRequired bool
}

// ErrorPattern represents a common error pattern for repair guidance.
type ErrorPattern struct {
	Name             string
	ErrorRegex       string
	CauseDescription string
	FixTemplate      string
	Severity         string // "silent_failure", "parse_error", "runtime_error"
}

// PredicateExample represents an example usage pattern.
type PredicateExample struct {
	PredicateName string
	ExampleCode   string
	Explanation   string
	IsCorrect     bool   // true = good example, false = anti-pattern
	ErrorType     string // For anti-patterns: what error this causes
	Source        string
}

func main() {
	fmt.Println("=================================================")
	fmt.Println("  PREDICATE CORPUS BUILDER")
	fmt.Println("  Mangle Schema & Self-Healing System")
	fmt.Println("=================================================")
	fmt.Println()

	// Step 1: Parse schemas.mg for EDB declarations
	fmt.Println("[1/5] Parsing schemas.mg for EDB declarations...")
	edbPredicates, err := parseSchemas(schemasPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to parse schemas: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("      Found %d EDB predicates\n", len(edbPredicates))

	// Step 2: Parse policy.mg for IDB rules (derived predicates)
	fmt.Println("[2/5] Parsing policy.mg for IDB rules...")
	idbPredicates, err := parsePolicy(policyPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to parse policy: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("      Found %d IDB predicates\n", len(idbPredicates))

	// Step 3: Merge and deduplicate
	fmt.Println("[3/5] Merging and deduplicating predicates...")
	allPredicates := mergePredicates(edbPredicates, idbPredicates)
	fmt.Printf("      Total unique predicates: %d\n", len(allPredicates))

	// Step 4: Create error patterns from AI failure modes
	fmt.Println("[4/6] Creating error patterns...")
	errorPatterns := createErrorPatterns()
	fmt.Printf("      Created %d error patterns\n", len(errorPatterns))

	// Step 5: Extract examples from skill resources
	fmt.Println("[5/6] Extracting examples from mangle-programming skill...")
	examples := extractExamplesFromSkill(skillPath)
	fmt.Printf("      Found %d predicate examples\n", len(examples))

	// Step 6: Create database and populate
	fmt.Println("[6/6] Creating database and populating...")
	db, err := createDatabase()
	if err != nil {
		fmt.Printf("ERROR: Failed to create database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := populateDatabase(db, allPredicates, errorPatterns, examples); err != nil {
		fmt.Printf("ERROR: Failed to populate database: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	printSummary(db)

	fmt.Println()
	fmt.Println("=================================================")
	fmt.Println("  PREDICATE CORPUS BUILD COMPLETE")
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println("=================================================")
}

// parseSchemas parses schemas.mg and extracts all Decl statements with metadata.
func parseSchemas(path string) ([]PredicateEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var predicates []PredicateEntry
	currentSection := "unknown"
	currentDomain := "core"

	// Regex for Decl statements: Decl predicate_name(arg1.Type<type>, arg2.Type<type>, ...).
	declPattern := regexp.MustCompile(`^Decl\s+([a-z_][a-z0-9_]*)\s*\(([^)]*)\)\.?`)
	sectionPattern := regexp.MustCompile(`^#\s*SECTION\s*(\d+[A-Z]?):\s*(.+)`)
	domainPattern := regexp.MustCompile(`^#\s*Domain:\s*(\S+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Track section headers
		if matches := sectionPattern.FindStringSubmatch(line); matches != nil {
			currentSection = matches[2]
			currentDomain = sectionToDomain(currentSection)
			continue
		}

		// Track explicit domain markers
		if matches := domainPattern.FindStringSubmatch(line); matches != nil {
			currentDomain = matches[1]
			continue
		}

		// Parse Decl statements
		if matches := declPattern.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			argsStr := matches[2]
			args := parseArgumentDefs(argsStr)

			entry := PredicateEntry{
				Name:         name,
				Arity:        len(args),
				Type:         "EDB",
				Category:     categoryFromSection(currentSection),
				Description:  extractDescription(lines, i),
				SafetyLevel:  inferSafetyLevel(name, currentSection),
				Domain:       currentDomain,
				Section:      currentSection,
				SourceFile:   filepath.Base(path),
				ArgumentDefs: args,
			}
			predicates = append(predicates, entry)
		}
	}

	return predicates, nil
}

// parseArgumentDefs parses argument definitions from a Decl statement.
func parseArgumentDefs(argsStr string) []ArgumentDef {
	if strings.TrimSpace(argsStr) == "" {
		return nil
	}

	var args []ArgumentDef
	// Split on commas, but be careful about nested types like Type<{/k: v}>
	parts := splitArguments(argsStr)

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		arg := ArgumentDef{Position: i}

		// Parse argument: Name.Type<type> or just Name
		if dotIdx := strings.Index(part, "."); dotIdx != -1 {
			arg.Name = part[:dotIdx]
			typeStr := part[dotIdx+1:]

			// Extract type from Type<...>
			if strings.HasPrefix(typeStr, "Type<") && strings.HasSuffix(typeStr, ">") {
				innerType := typeStr[5 : len(typeStr)-1]
				arg.Type = normalizeType(innerType)
			} else {
				arg.Type = normalizeType(typeStr)
			}
		} else {
			arg.Name = part
			arg.Type = "any"
		}

		args = append(args, arg)
	}

	return args
}

// splitArguments splits argument string, respecting nested brackets.
func splitArguments(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case '<':
			depth++
			current.WriteRune(ch)
		case '>':
			depth--
			current.WriteRune(ch)
		case '{':
			depth++
			current.WriteRune(ch)
		case '}':
			depth--
			current.WriteRune(ch)
		case '[':
			depth++
			current.WriteRune(ch)
		case ']':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// normalizeType converts Mangle type syntax to normalized form.
func normalizeType(t string) string {
	t = strings.TrimSpace(t)
	switch {
	case t == "int":
		return "number"
	case t == "float":
		return "number"
	case t == "string":
		return "string"
	case t == "n" || t == "name":
		return "atom"
	case strings.HasPrefix(t, "["):
		return "list"
	case strings.HasPrefix(t, "{"):
		return "map"
	case t == "Any":
		return "any"
	default:
		return "any"
	}
}

// sectionToDomain maps section names to JIT domains.
func sectionToDomain(section string) string {
	section = strings.ToLower(section)
	switch {
	case strings.Contains(section, "intent"):
		return "core"
	case strings.Contains(section, "focus"):
		return "core"
	case strings.Contains(section, "topology"):
		return "world_model"
	case strings.Contains(section, "symbol"):
		return "world_model"
	case strings.Contains(section, "diagnostic"):
		return "diagnostic"
	case strings.Contains(section, "shard"):
		return "shard_lifecycle"
	case strings.Contains(section, "memory"):
		return "memory"
	case strings.Contains(section, "knowledge"):
		return "memory"
	case strings.Contains(section, "test"):
		return "shard_tester"
	case strings.Contains(section, "action"):
		return "routing"
	case strings.Contains(section, "routing"):
		return "routing"
	case strings.Contains(section, "safety"):
		return "safety"
	case strings.Contains(section, "constitution"):
		return "safety"
	case strings.Contains(section, "appeal"):
		return "safety"
	case strings.Contains(section, "campaign"):
		return "campaign"
	case strings.Contains(section, "phase"):
		return "campaign"
	case strings.Contains(section, "tool"):
		return "tool"
	case strings.Contains(section, "ouroboros"):
		return "tool"
	case strings.Contains(section, "coder"):
		return "shard_coder"
	case strings.Contains(section, "reviewer"):
		return "shard_reviewer"
	default:
		return "core"
	}
}

// categoryFromSection extracts category from section name.
func categoryFromSection(section string) string {
	section = strings.ToLower(section)
	switch {
	case strings.Contains(section, "intent"):
		return "intent"
	case strings.Contains(section, "focus"):
		return "focus"
	case strings.Contains(section, "topology"):
		return "world"
	case strings.Contains(section, "symbol"):
		return "world"
	case strings.Contains(section, "diagnostic"):
		return "diagnostic"
	case strings.Contains(section, "shard"):
		return "shard"
	case strings.Contains(section, "memory"):
		return "memory"
	case strings.Contains(section, "knowledge"):
		return "memory"
	case strings.Contains(section, "test"):
		return "test"
	case strings.Contains(section, "action"):
		return "action"
	case strings.Contains(section, "routing"):
		return "routing"
	case strings.Contains(section, "safety"):
		return "safety"
	case strings.Contains(section, "constitution"):
		return "safety"
	case strings.Contains(section, "appeal"):
		return "safety"
	case strings.Contains(section, "campaign"):
		return "campaign"
	case strings.Contains(section, "phase"):
		return "campaign"
	case strings.Contains(section, "tool"):
		return "tool"
	case strings.Contains(section, "coder"):
		return "coder"
	case strings.Contains(section, "reviewer"):
		return "reviewer"
	default:
		return "core"
	}
}

// inferSafetyLevel infers safety level from predicate name and section.
func inferSafetyLevel(name, section string) string {
	section = strings.ToLower(section)

	// High-risk predicates
	if strings.Contains(name, "permitted") ||
		strings.Contains(name, "blocked") ||
		strings.Contains(name, "safety") ||
		strings.Contains(name, "violation") {
		return "stratification_critical"
	}

	// Predicates that must be in negation correctly
	if strings.Contains(name, "not_") ||
		strings.HasPrefix(name, "un") ||
		strings.Contains(name, "denied") {
		return "negation_sensitive"
	}

	// Virtual predicates requiring binding
	if strings.Contains(section, "virtual") {
		return "requires_binding"
	}

	return "safe"
}

// extractDescription extracts description from comments above the Decl.
func extractDescription(lines []string, declLine int) string {
	var descLines []string

	// Look backwards for comment lines
	for i := declLine - 1; i >= 0 && i > declLine-5; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#") {
			// Skip section headers
			if strings.Contains(line, "SECTION") || strings.Contains(line, "===") {
				break
			}
			// Extract comment text
			comment := strings.TrimPrefix(line, "#")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				descLines = append([]string{comment}, descLines...)
			}
		} else if line != "" {
			break
		}
	}

	return strings.Join(descLines, " ")
}

// parsePolicy parses policy.mg and extracts IDB rule heads.
func parsePolicy(path string) ([]PredicateEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Track unique predicates (name + arity)
	seen := make(map[string]bool)
	var predicates []PredicateEntry

	currentSection := "unknown"

	// Regex for rule heads: predicate_name(args) :-
	rulePattern := regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s*\(([^)]*)\)\s*:-`)
	sectionPattern := regexp.MustCompile(`^#\s*SECTION\s*(\d+[A-Z]?):\s*(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Track section headers
		if matches := sectionPattern.FindStringSubmatch(line); matches != nil {
			currentSection = matches[2]
			continue
		}

		// Parse rule heads
		if matches := rulePattern.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			argsStr := matches[2]
			args := countArgs(argsStr)

			key := fmt.Sprintf("%s/%d", name, args)
			if seen[key] {
				continue
			}
			seen[key] = true

			entry := PredicateEntry{
				Name:        name,
				Arity:       args,
				Type:        "IDB",
				Category:    categoryFromSection(currentSection),
				Description: extractDescription(lines, i),
				SafetyLevel: inferSafetyLevel(name, currentSection),
				Domain:      sectionToDomain(currentSection),
				Section:     currentSection,
				SourceFile:  filepath.Base(path),
			}
			predicates = append(predicates, entry)
		}
	}

	return predicates, nil
}

// countArgs counts the number of arguments in an argument string.
func countArgs(argsStr string) int {
	if strings.TrimSpace(argsStr) == "" {
		return 0
	}
	parts := splitArguments(argsStr)
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	return count
}

// extractExamplesFromSkill extracts predicate examples from mangle-programming skill files.
func extractExamplesFromSkill(skillDir string) []PredicateExample {
	var examples []PredicateExample

	// Process key files that contain examples
	files := []string{
		"150-AI_FAILURE_MODES.md",
		"300-PATTERN_LIBRARY.md",
		"450-PROMPT_ATOM_PREDICATES.md",
		"EXAMPLES.md",
	}

	for _, file := range files {
		path := filepath.Join(skillDir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("      Skipping %s: %v\n", file, err)
			continue
		}

		fileExamples := extractExamplesFromMarkdown(string(content), file)
		examples = append(examples, fileExamples...)
		if len(fileExamples) > 0 {
			fmt.Printf("      Extracted %d examples from %s\n", len(fileExamples), file)
		}
	}

	return examples
}

// extractExamplesFromMarkdown extracts Mangle code blocks from markdown.
func extractExamplesFromMarkdown(content, source string) []PredicateExample {
	var examples []PredicateExample

	// Find all ```mangle code blocks
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	var currentBlock strings.Builder
	var blockContext string
	isAntiPattern := false

	for i, line := range lines {
		// Track context (headers before code blocks)
		if strings.HasPrefix(line, "##") || strings.HasPrefix(line, "###") {
			blockContext = strings.TrimSpace(strings.TrimLeft(line, "#"))
			isAntiPattern = strings.Contains(strings.ToLower(blockContext), "wrong") ||
				strings.Contains(strings.ToLower(blockContext), "anti") ||
				strings.Contains(strings.ToLower(blockContext), "bad") ||
				strings.Contains(strings.ToLower(blockContext), "incorrect") ||
				strings.Contains(strings.ToLower(blockContext), "hallucination")
		}

		if strings.HasPrefix(strings.TrimSpace(line), "```mangle") {
			inCodeBlock = true
			currentBlock.Reset()
			continue
		}

		if inCodeBlock && strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = false
			code := strings.TrimSpace(currentBlock.String())
			if code != "" {
				// Extract predicates from this code block
				predicates := extractPredicatesFromCode(code)
				for _, pred := range predicates {
					examples = append(examples, PredicateExample{
						PredicateName: pred,
						ExampleCode:   code,
						Explanation:   blockContext,
						IsCorrect:     !isAntiPattern,
						ErrorType:     getErrorTypeFromContext(blockContext),
						Source:        source,
					})
				}
			}
			// Look ahead for context of next block
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				nextLine := lines[j]
				if strings.HasPrefix(nextLine, "##") || strings.HasPrefix(nextLine, "###") {
					blockContext = strings.TrimSpace(strings.TrimLeft(nextLine, "#"))
					break
				}
			}
			continue
		}

		if inCodeBlock {
			currentBlock.WriteString(line)
			currentBlock.WriteString("\n")
		}
	}

	return examples
}

// extractPredicatesFromCode finds predicate names in Mangle code.
func extractPredicatesFromCode(code string) []string {
	predicatePattern := regexp.MustCompile(`([a-z_][a-z0-9_]*)\s*\(`)
	matches := predicatePattern.FindAllStringSubmatch(code, -1)

	seen := make(map[string]bool)
	var predicates []string

	// Skip common built-ins
	builtins := map[string]bool{
		"fn": true, "do": true, "let": true, "not": true,
		"true": true, "false": true,
	}

	for _, match := range matches {
		if len(match) > 1 {
			pred := match[1]
			if !builtins[pred] && !seen[pred] {
				seen[pred] = true
				predicates = append(predicates, pred)
			}
		}
	}

	return predicates
}

// getErrorTypeFromContext extracts error type from section context.
func getErrorTypeFromContext(context string) string {
	lower := strings.ToLower(context)
	switch {
	case strings.Contains(lower, "atom") && strings.Contains(lower, "string"):
		return "atom_string_confusion"
	case strings.Contains(lower, "aggregation"):
		return "aggregation_syntax"
	case strings.Contains(lower, "negation"):
		return "unsafe_negation"
	case strings.Contains(lower, "variable"):
		return "unbound_variable"
	case strings.Contains(lower, "type"):
		return "type_mismatch"
	case strings.Contains(lower, "syntax"):
		return "parse_error"
	default:
		return ""
	}
}

// mergePredicates merges EDB and IDB predicates, with EDB taking precedence.
func mergePredicates(edb, idb []PredicateEntry) []PredicateEntry {
	seen := make(map[string]bool)
	var result []PredicateEntry

	// Add all EDB predicates first
	for _, p := range edb {
		key := fmt.Sprintf("%s/%d", p.Name, p.Arity)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	// Add IDB predicates that aren't already EDB
	for _, p := range idb {
		key := fmt.Sprintf("%s/%d", p.Name, p.Arity)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// createErrorPatterns creates error patterns from known AI failure modes.
func createErrorPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			Name:             "undefined_predicate",
			ErrorRegex:       `undefined predicates?: \[([^\]]+)\]`,
			CauseDescription: "Rule uses predicates that are not declared in schemas.mg",
			FixTemplate:      "Replace undefined predicate with a declared one from the available predicates list, or check for typos in predicate names",
			Severity:         "silent_failure",
		},
		{
			Name:             "unbound_variable_negation",
			ErrorRegex:       `unbound variable .* in negation`,
			CauseDescription: "Variable in negated atom is not bound by a positive atom first",
			FixTemplate:      "Add a positive atom that binds the variable before the negation: bound(X), not other(X)",
			Severity:         "runtime_error",
		},
		{
			Name:             "atom_string_confusion",
			ErrorRegex:       `type mismatch|cannot unify`,
			CauseDescription: "Using \"string\" where /atom is expected or vice versa",
			FixTemplate:      "Use /atom for enum values, identifiers, status flags. Use \"string\" only for human-readable text",
			Severity:         "silent_failure",
		},
		{
			Name:             "missing_do_keyword",
			ErrorRegex:       `expected 'do' after|parse error.*group_by`,
			CauseDescription: "Aggregation pipeline missing 'do' keyword before fn:group_by",
			FixTemplate:      "Use: source() |> do fn:group_by(X), let N = fn:Count()",
			Severity:         "parse_error",
		},
		{
			Name:             "wrong_aggregation_casing",
			ErrorRegex:       `undefined function fn:count|fn:sum|fn:min|fn:max`,
			CauseDescription: "Aggregation functions require capital letters: fn:Count, fn:Sum, fn:Min, fn:Max",
			FixTemplate:      "Use capital letters for aggregation functions: fn:Count(), fn:Sum(X), fn:Min(X), fn:Max(X)",
			Severity:         "runtime_error",
		},
		{
			Name:             "lowercase_variable",
			ErrorRegex:       `unexpected lowercase|invalid variable`,
			CauseDescription: "Variables must be UPPERCASE. Lowercase identifiers are atoms or predicates",
			FixTemplate:      "Change lowercase variables to UPPERCASE: x -> X, user -> User, result -> Result",
			Severity:         "parse_error",
		},
		{
			Name:             "missing_period",
			ErrorRegex:       `expected '\.'|unexpected end of rule`,
			CauseDescription: "Every Mangle rule and fact must end with a period (.)",
			FixTemplate:      "Add a period at the end of the rule: predicate(X) :- body(X).",
			Severity:         "parse_error",
		},
		{
			Name:             "wrong_decl_syntax",
			ErrorRegex:       `invalid declaration|\.decl`,
			CauseDescription: "Using Soufflé syntax (.decl) instead of Mangle syntax (Decl)",
			FixTemplate:      "Use Mangle Decl syntax: Decl predicate_name(Arg.Type<type>).",
			Severity:         "parse_error",
		},
		{
			Name:             "stratification_cycle",
			ErrorRegex:       `stratification|cyclic negation|recursive.*negation`,
			CauseDescription: "Rule has recursion through negation, creating unstable logic",
			FixTemplate:      "Break the cycle by introducing intermediate predicates without negation in the recursive path",
			Severity:         "runtime_error",
		},
		{
			Name:             "unsafe_head_variable",
			ErrorRegex:       `unsafe variable in head|unbound.*in head`,
			CauseDescription: "Variable in rule head is not bound by any atom in the body",
			FixTemplate:      "Ensure all head variables appear in at least one positive atom in the body",
			Severity:         "runtime_error",
		},
		{
			Name:             "wrong_negation_syntax",
			ErrorRegex:       `NOT|!.*expected`,
			CauseDescription: "Mangle uses 'not' (lowercase) for negation, not '!' or 'NOT'",
			FixTemplate:      "Use lowercase 'not': predicate(X) :- positive(X), not negative(X).",
			Severity:         "parse_error",
		},
		{
			Name:             "sql_style_aggregation",
			ErrorRegex:       `sum\(|count\(.*=`,
			CauseDescription: "Using SQL-style aggregation instead of Mangle pipeline syntax",
			FixTemplate:      "Use pipeline syntax: source(X) |> do fn:group_by(K), let Sum = fn:Sum(X).",
			Severity:         "parse_error",
		},
	}
}

// createDatabase creates the SQLite database with schema.
func createDatabase() (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Remove existing database
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing database: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create schema
	schema := `
		-- Core predicate definitions
		CREATE TABLE predicates (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			arity INTEGER NOT NULL,
			type TEXT NOT NULL,              -- 'EDB' | 'IDB'
			category TEXT NOT NULL,          -- 'core', 'shard', 'campaign', 'safety', 'tool', etc.
			description TEXT,
			safety_level TEXT,               -- 'safe', 'requires_binding', 'negation_sensitive', 'stratification_critical'
			domain TEXT,                     -- For JIT selection
			section TEXT,
			source_file TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name, arity)
		);
		CREATE INDEX idx_predicates_name ON predicates(name);
		CREATE INDEX idx_predicates_domain ON predicates(domain);
		CREATE INDEX idx_predicates_category ON predicates(category);
		CREATE INDEX idx_predicates_type ON predicates(type);

		-- Argument type specifications
		CREATE TABLE predicate_args (
			predicate_id INTEGER REFERENCES predicates(id),
			arg_position INTEGER NOT NULL,
			arg_name TEXT,
			arg_type TEXT NOT NULL,          -- 'atom', 'string', 'number', 'list', 'map', 'any'
			is_bound_required BOOLEAN DEFAULT FALSE,
			PRIMARY KEY (predicate_id, arg_position)
		);

		-- Domain groupings for JIT context selection
		CREATE TABLE predicate_domains (
			predicate_id INTEGER REFERENCES predicates(id),
			domain TEXT NOT NULL,
			relevance_score REAL DEFAULT 1.0,
			PRIMARY KEY (predicate_id, domain)
		);
		CREATE INDEX idx_predicate_domains_domain ON predicate_domains(domain);

		-- Common error patterns (for repair guidance)
		CREATE TABLE error_patterns (
			id INTEGER PRIMARY KEY,
			pattern_name TEXT NOT NULL UNIQUE,
			error_regex TEXT,                -- Regex to match error messages
			cause_description TEXT,
			fix_template TEXT,               -- Template for fix guidance
			severity TEXT                    -- 'silent_failure', 'parse_error', 'runtime_error'
		);

		-- Example usage patterns from skill resources
		CREATE TABLE predicate_examples (
			id INTEGER PRIMARY KEY,
			predicate_name TEXT NOT NULL,
			example_code TEXT NOT NULL,
			explanation TEXT,
			is_correct BOOLEAN NOT NULL,     -- TRUE = good example, FALSE = anti-pattern
			error_type TEXT,                 -- For anti-patterns: what error this causes
			source TEXT NOT NULL
		);
		CREATE INDEX idx_examples_predicate ON predicate_examples(predicate_name);
		CREATE INDEX idx_examples_correct ON predicate_examples(is_correct);

		-- Quick lookup view for predicate signatures
		CREATE VIEW predicate_signatures AS
		SELECT
			p.id,
			p.name,
			p.arity,
			p.type,
			p.category,
			p.domain,
			p.safety_level,
			GROUP_CONCAT(pa.arg_name || ':' || pa.arg_type, ', ') as signature
		FROM predicates p
		LEFT JOIN predicate_args pa ON p.id = pa.predicate_id
		GROUP BY p.id
		ORDER BY p.name;
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	fmt.Printf("      Database created: %s\n", outputPath)
	return db, nil
}

// populateDatabase inserts all predicates, error patterns, and examples.
func populateDatabase(db *sql.DB, predicates []PredicateEntry, errorPatterns []ErrorPattern, examples []PredicateExample) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert predicates
	predStmt, err := tx.Prepare(`
		INSERT INTO predicates (name, arity, type, category, description, safety_level, domain, section, source_file)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare predicate statement: %w", err)
	}
	defer predStmt.Close()

	argStmt, err := tx.Prepare(`
		INSERT INTO predicate_args (predicate_id, arg_position, arg_name, arg_type, is_bound_required)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare args statement: %w", err)
	}
	defer argStmt.Close()

	domainStmt, err := tx.Prepare(`
		INSERT INTO predicate_domains (predicate_id, domain, relevance_score)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare domain statement: %w", err)
	}
	defer domainStmt.Close()

	for _, p := range predicates {
		result, err := predStmt.Exec(p.Name, p.Arity, p.Type, p.Category, p.Description, p.SafetyLevel, p.Domain, p.Section, p.SourceFile)
		if err != nil {
			return fmt.Errorf("failed to insert predicate %s: %w", p.Name, err)
		}

		predID, _ := result.LastInsertId()

		// Insert arguments
		for _, arg := range p.ArgumentDefs {
			_, err := argStmt.Exec(predID, arg.Position, arg.Name, arg.Type, arg.IsBoundRequired)
			if err != nil {
				return fmt.Errorf("failed to insert arg for %s: %w", p.Name, err)
			}
		}

		// Insert domain mapping
		_, err = domainStmt.Exec(predID, p.Domain, 1.0)
		if err != nil {
			return fmt.Errorf("failed to insert domain for %s: %w", p.Name, err)
		}
	}

	// Insert error patterns
	errorStmt, err := tx.Prepare(`
		INSERT INTO error_patterns (pattern_name, error_regex, cause_description, fix_template, severity)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare error pattern statement: %w", err)
	}
	defer errorStmt.Close()

	for _, ep := range errorPatterns {
		_, err := errorStmt.Exec(ep.Name, ep.ErrorRegex, ep.CauseDescription, ep.FixTemplate, ep.Severity)
		if err != nil {
			return fmt.Errorf("failed to insert error pattern %s: %w", ep.Name, err)
		}
	}

	// Insert predicate examples
	exampleStmt, err := tx.Prepare(`
		INSERT INTO predicate_examples (predicate_name, example_code, explanation, is_correct, error_type, source)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare example statement: %w", err)
	}
	defer exampleStmt.Close()

	for _, ex := range examples {
		_, err := exampleStmt.Exec(ex.PredicateName, ex.ExampleCode, ex.Explanation, ex.IsCorrect, ex.ErrorType, ex.Source)
		if err != nil {
			return fmt.Errorf("failed to insert example for %s: %w", ex.PredicateName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("      Inserted %d predicates, %d error patterns, and %d examples\n", len(predicates), len(errorPatterns), len(examples))
	return nil
}

// printSummary prints statistics about the generated database.
func printSummary(db *sql.DB) {
	fmt.Println()
	fmt.Println("--- Summary ---")

	// Total predicates
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM predicates").Scan(&total); err == nil {
		fmt.Printf("  Total predicates: %d\n", total)
	}

	// By type
	fmt.Println("  By type:")
	rows, err := db.Query(`SELECT type, COUNT(*) FROM predicates GROUP BY type ORDER BY type`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var cnt int
			if err := rows.Scan(&t, &cnt); err == nil {
				fmt.Printf("    %-15s %d\n", t, cnt)
			}
		}
	}

	// By domain
	fmt.Println("  By domain:")
	rows, err = db.Query(`SELECT domain, COUNT(*) as cnt FROM predicates GROUP BY domain ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var domain string
			var cnt int
			if err := rows.Scan(&domain, &cnt); err == nil {
				fmt.Printf("    %-20s %d\n", domain, cnt)
			}
		}
	}

	// By category
	fmt.Println("  By category:")
	rows, err = db.Query(`SELECT category, COUNT(*) as cnt FROM predicates GROUP BY category ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var category string
			var cnt int
			if err := rows.Scan(&category, &cnt); err == nil {
				fmt.Printf("    %-20s %d\n", category, cnt)
			}
		}
	}

	// Error patterns
	var errorCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM error_patterns").Scan(&errorCount); err == nil {
		fmt.Printf("  Error patterns: %d\n", errorCount)
	}

	// Predicate examples
	var exampleCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM predicate_examples").Scan(&exampleCount); err == nil {
		fmt.Printf("  Predicate examples: %d\n", exampleCount)
	}

	// Example breakdown
	var correctCount, antiPatternCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM predicate_examples WHERE is_correct = 1").Scan(&correctCount); err == nil {
		fmt.Printf("    Correct examples: %d\n", correctCount)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM predicate_examples WHERE is_correct = 0").Scan(&antiPatternCount); err == nil {
		fmt.Printf("    Anti-patterns: %d\n", antiPatternCount)
	}

	// File size
	if info, err := os.Stat(outputPath); err == nil {
		fmt.Printf("  Database size: %.2f KB\n", float64(info.Size())/1024)
	}
}
