package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// parseSchemasDir parses all schemas*.mg files in a directory and extracts all Decl statements.
func parseSchemasDir(dirPath string) ([]PredicateEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var allPredicates []PredicateEntry
	seen := make(map[string]bool) // Deduplicate across files

	for _, entry := range entries {
		// Only process schema files (schemas.mg, schemas_*.mg)
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mg") {
			continue
		}
		if !strings.HasPrefix(name, "schemas") {
			continue
		}

		filePath := filepath.Join(dirPath, name)
		predicates, err := parseSchemaFile(filePath)
		if err != nil {
			fmt.Printf("      Warning: Failed to parse %s: %v\n", name, err)
			continue
		}

		for _, p := range predicates {
			key := fmt.Sprintf("%s/%d", p.Name, p.Arity)
			if !seen[key] {
				seen[key] = true
				allPredicates = append(allPredicates, p)
			}
		}
	}

	return allPredicates, nil
}

// parseSchemaFile parses a single schema .mg file and extracts all Decl statements with metadata.
func parseSchemaFile(path string) ([]PredicateEntry, error) {
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
	priorityPattern := regexp.MustCompile(`#\s*Priority:\s*(\d+)`)
	serializationPattern := regexp.MustCompile(`#\s*SerializationOrder:\s*(\d+)`)

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

			// Extract priority annotations from nearby comments (look up to 5 lines back)
			activationPriority := inferActivationPriority(name, currentSection) // Start with inferred default
			serializationOrder := 100                                           // Default order
			for j := i - 1; j >= 0 && j > i-5; j-- {
				commentLine := lines[j]
				if matches := priorityPattern.FindStringSubmatch(commentLine); matches != nil {
					if p, err := strconv.Atoi(matches[1]); err == nil {
						activationPriority = p
					}
				}
				if matches := serializationPattern.FindStringSubmatch(commentLine); matches != nil {
					if s, err := strconv.Atoi(matches[1]); err == nil {
						serializationOrder = s
					}
				}
			}

			entry := PredicateEntry{
				Name:               name,
				Arity:              len(args),
				Type:               "EDB",
				Category:           categoryFromSection(currentSection),
				Description:        extractDescription(lines, i),
				SafetyLevel:        inferSafetyLevel(name, currentSection),
				Domain:             currentDomain,
				Section:            currentSection,
				SourceFile:         filepath.Base(path),
				ArgumentDefs:       args,
				ActivationPriority: activationPriority,
				SerializationOrder: serializationOrder,
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
		if before, after, ok := strings.Cut(part, "."); ok {
			arg.Name = before
			typeStr := after

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

// inferActivationPriority infers activation priority from predicate name and section.
// Higher priority = more likely to be included in context window.
// These defaults match the hardcoded priorities in internal/context/types.go.
func inferActivationPriority(name, section string) int {
	section = strings.ToLower(section)

	// Core intent predicates (highest priority)
	if name == "user_intent" {
		return 100
	}

	// Critical diagnostic and test state
	if name == "diagnostic" || name == "test_state" {
		return 95
	}

	// Focus and goal predicates
	if strings.Contains(name, "focus") || strings.Contains(name, "active_goal") {
		return 90
	}

	// Campaign state
	if strings.Contains(name, "campaign") || strings.Contains(name, "phase") {
		return 85
	}

	// Safety and permission predicates
	if strings.Contains(name, "permitted") || strings.Contains(name, "blocked") ||
		strings.Contains(name, "safety") || strings.Contains(name, "violation") {
		return 85
	}

	// Shard lifecycle
	if strings.Contains(name, "shard") {
		return 80
	}

	// World model predicates
	if strings.Contains(name, "file_topology") || strings.Contains(name, "symbol_graph") ||
		strings.Contains(name, "dependency") {
		return 75
	}

	// Action routing
	if strings.Contains(name, "next_action") || strings.Contains(name, "action") {
		return 70
	}

	// Memory and knowledge
	if strings.Contains(section, "memory") || strings.Contains(section, "knowledge") {
		return 60
	}

	// Tool predicates
	if strings.Contains(section, "tool") {
		return 55
	}

	// Default priority
	return 50
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
