package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitArgumentsAndNormalizeTypes(t *testing.T) {
	args := "A.Type<string>, B.Type<{/k: v}>, C.Type<[int, string]>, D"
	parts := splitArguments(args)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}

	defs := parseArgumentDefs("User.Type<string>, Verb.Type<name>, Count.Type<int>")
	if len(defs) != 3 {
		t.Fatalf("expected 3 defs, got %d", len(defs))
	}
	if defs[0].Name != "User" || defs[0].Type != "string" {
		t.Fatalf("unexpected first arg: %+v", defs[0])
	}
	if defs[1].Type != "atom" {
		t.Fatalf("expected atom type for name, got %s", defs[1].Type)
	}
	if defs[2].Type != "number" {
		t.Fatalf("expected number type for int, got %s", defs[2].Type)
	}

	if normalizeType("float") != "number" {
		t.Fatalf("expected float -> number")
	}
	if normalizeType("{k: v}") != "map" {
		t.Fatalf("expected map type")
	}
	if normalizeType("[int]") != "list" {
		t.Fatalf("expected list type")
	}
}

func TestSectionAndSafetyInference(t *testing.T) {
	if sectionToDomain("Safety Gates") != "safety" {
		t.Fatalf("expected safety domain")
	}
	if categoryFromSection("Campaign Phases") != "campaign" {
		t.Fatalf("expected campaign category")
	}
	if inferSafetyLevel("permitted_action", "Safety") != "stratification_critical" {
		t.Fatalf("expected stratification_critical")
	}
	if inferActivationPriority("user_intent", "Intent") != 100 {
		t.Fatalf("expected highest priority for user_intent")
	}
}

func TestExtractDescription(t *testing.T) {
	lines := []string{
		"# SECTION 1A: Intent",
		"# User intent predicate",
		"Decl user_intent(User.Type<string>).",
	}
	desc := extractDescription(lines, 2)
	if desc != "User intent predicate" {
		t.Fatalf("unexpected description: %q", desc)
	}
}

func TestParseSchemaFileBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schemas_intent.mg")
	content := strings.Join([]string{
		"# SECTION 1A: Intent",
		"# Priority: 95",
		"# SerializationOrder: 10",
		"# User intent predicate",
		"Decl user_intent(User.Type<string>, Verb.Type<name>, Count.Type<int>).",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write schema file: %v", err)
	}

	preds, err := parseSchemaFile(path)
	if err != nil {
		t.Fatalf("parse schema file: %v", err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(preds))
	}
	if preds[0].Name != "user_intent" {
		t.Fatalf("unexpected predicate name: %s", preds[0].Name)
	}
	if preds[0].ActivationPriority != 95 {
		t.Fatalf("expected priority 95, got %d", preds[0].ActivationPriority)
	}
	if preds[0].SerializationOrder != 10 {
		t.Fatalf("expected serialization order 10, got %d", preds[0].SerializationOrder)
	}
	if preds[0].Category != "intent" {
		t.Fatalf("expected intent category, got %s", preds[0].Category)
	}
}

func TestExtractPredicatesAndErrorType(t *testing.T) {
	code := "user_intent(X) :- other(X), not blocked(X)."
	preds := extractPredicatesFromCode(code)
	if len(preds) != 3 {
		t.Fatalf("expected 3 predicates, got %d", len(preds))
	}

	if getErrorTypeFromContext("Aggregation Errors") != "aggregation_syntax" {
		t.Fatalf("expected aggregation_syntax")
	}
	if getErrorTypeFromContext("Atom/String Confusion") != "atom_string_confusion" {
		t.Fatalf("expected atom_string_confusion")
	}
}

func TestMergePredicatesPrefersEDB(t *testing.T) {
	edb := []PredicateEntry{{Name: "foo", Arity: 1, Type: "EDB"}}
	idb := []PredicateEntry{{Name: "foo", Arity: 1, Type: "IDB"}}
	merged := mergePredicates(edb, idb)
	if len(merged) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(merged))
	}
	if merged[0].Type != "EDB" {
		t.Fatalf("expected EDB to win, got %s", merged[0].Type)
	}
}
