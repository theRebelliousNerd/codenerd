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

func TestNormalizeTypeTable(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"int", "number"},
		{"float", "number"},
		{"string", "string"},
		{"n", "atom"},
		{"name", "atom"},
		{"[int]", "list"},
		{"[string, int]", "list"},
		{"{k: v}", "map"},
		{"{/k: v}", "map"},
		{"Any", "any"},
		{"any", "any"},
		{"bool", "any"},
		{"unknown_type", "any"},
		{"  string  ", "string"},
		{"", "any"},
	}
	for _, tc := range cases {
		if got := normalizeType(tc.in); got != tc.want {
			t.Errorf("normalizeType(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitArgumentsNestedAndEdge(t *testing.T) {
	got := splitArguments("A.Type<string>, B.Type<{/k: v, /j: w}>, C.Type<[int, string]>")
	if len(got) != 3 {
		t.Fatalf("expected 3 parts for nested commas, got %d: %q", len(got), got)
	}
	if got := splitArguments(""); len(got) != 0 {
		t.Errorf("expected no parts for empty input, got %q", got)
	}
	if got := splitArguments("   "); len(got) != 1 {
		t.Errorf("expected single whitespace part to be returned for caller to skip, got %q", got)
	}
	paren := splitArguments("f(X, Y), Z")
	if len(paren) != 2 {
		t.Errorf("expected parens to shield comma, got %q", paren)
	}
	if got := countArgs("A,,B"); got != 2 {
		t.Errorf("countArgs should skip empty segments, got %d", got)
	}
}

func TestParseArgumentDefsEdgeCases(t *testing.T) {
	if got := parseArgumentDefs(""); got != nil {
		t.Errorf("empty args should return nil, got %+v", got)
	}
	if got := parseArgumentDefs("   "); got != nil {
		t.Errorf("whitespace args should return nil, got %+v", got)
	}
	defs := parseArgumentDefs("User, Count.Type<int>, Raw.Type<custom>, Label.Type<string>")
	if len(defs) != 4 {
		t.Fatalf("expected 4 defs, got %d", len(defs))
	}
	if defs[0].Name != "User" || defs[0].Type != "any" {
		t.Errorf("bare name should default to any, got %+v", defs[0])
	}
	if defs[0].Position != 0 || defs[3].Position != 3 {
		t.Errorf("positions should track index, got %+v", defs)
	}
	if defs[2].Type != "any" {
		t.Errorf("unknown inner type should normalize to any, got %q", defs[2].Type)
	}
	if defs[3].Type != "string" {
		t.Errorf("string passthrough failed, got %q", defs[3].Type)
	}
	skipped := parseArgumentDefs("A.Type<int>, , B.Type<string>")
	if len(skipped) != 2 {
		t.Errorf("empty segment should be skipped, got %+v", skipped)
	}
}

func TestSectionDomainAndCategoryCoverage(t *testing.T) {
	domainCases := map[string]string{
		"Intent Definitions":        "core",
		"Focus State":               "core",
		"File Topology":             "world_model",
		"Symbol Graph":              "world_model",
		"Diagnostics":               "diagnostic",
		"Shard Lifecycle":           "shard_lifecycle",
		"Memory Store":              "memory",
		"Knowledge Base":            "memory",
		"Tester Hooks":              "shard_tester",
		"Action Dispatch":           "routing",
		"Routing Table":             "routing",
		"Safety Gates":              "safety",
		"Constitution Rules":        "safety",
		"Appeal Process":            "safety",
		"Campaign Phases":           "campaign",
		"Phase Transitions":         "campaign",
		"Tool Registry":             "tool",
		"Ouroboros Loop":            "tool",
		"Coder Module":              "shard_coder",
		"Reviewer Notes":            "shard_reviewer",
		"Something Completely Else": "core",
	}
	for section, want := range domainCases {
		if got := sectionToDomain(section); got != want {
			t.Errorf("sectionToDomain(%q)=%q, want %q", section, got, want)
		}
	}
	categoryCases := map[string]string{
		"Intent":       "intent",
		"Focus":        "focus",
		"Topology":     "world",
		"Symbol":       "world",
		"Diagnostic":   "diagnostic",
		"Shard":        "shard",
		"Memory":       "memory",
		"Knowledge":    "memory",
		"Test":         "test",
		"Action":       "action",
		"Routing":      "routing",
		"Safety":       "safety",
		"Constitution": "safety",
		"Appeal":       "safety",
		"Campaign":     "campaign",
		"Phase":        "campaign",
		"Tool":         "tool",
		"Coder":        "coder",
		"Reviewer":     "reviewer",
		"Mystery":      "core",
	}
	for section, want := range categoryCases {
		if got := categoryFromSection(section); got != want {
			t.Errorf("categoryFromSection(%q)=%q, want %q", section, got, want)
		}
	}
}

func TestInferSafetyLevelBranches(t *testing.T) {
	cases := []struct {
		name    string
		section string
		want    string
	}{
		{"blocked_action", "Safety", "stratification_critical"},
		{"safety_check", "Core", "stratification_critical"},
		{"violation_report", "Core", "stratification_critical"},
		{"not_ready", "Core", "negation_sensitive"},
		{"denied_request", "Core", "negation_sensitive"},
		{"unknown_thing", "Core", "negation_sensitive"},
		{"file_topology", "Virtual Predicates", "requires_binding"},
		{"file_topology", "World", "safe"},
		{"user_intent", "Intent", "safe"},
	}
	for _, tc := range cases {
		if got := inferSafetyLevel(tc.name, tc.section); got != tc.want {
			t.Errorf("inferSafetyLevel(%q,%q)=%q, want %q", tc.name, tc.section, got, tc.want)
		}
	}
}

func TestInferActivationPriorityBranches(t *testing.T) {
	cases := []struct {
		name    string
		section string
		want    int
	}{
		{"user_intent", "Anything", 100},
		{"diagnostic", "Anything", 95},
		{"test_state", "Anything", 95},
		{"focus_state", "Anything", 90},
		{"active_goal", "Anything", 90},
		{"campaign_state", "Anything", 85},
		{"phase_step", "Anything", 85},
		{"permitted_action", "Anything", 85},
		{"blocked_thing", "Anything", 85},
		{"shard_status", "Anything", 80},
		{"file_topology", "Anything", 75},
		{"symbol_graph", "Anything", 75},
		{"dependency_edge", "Anything", 75},
		{"next_action", "Anything", 70},
		{"action_route", "Anything", 70},
		{"recall_fact", "Memory Store", 60},
		{"learned_fact", "Knowledge Base", 60},
		{"tool_spec", "Tool Registry", 55},
		{"random_predicate", "Random Section", 50},
	}
	for _, tc := range cases {
		if got := inferActivationPriority(tc.name, tc.section); got != tc.want {
			t.Errorf("inferActivationPriority(%q,%q)=%d, want %d", tc.name, tc.section, got, tc.want)
		}
	}
}

func TestExtractDescriptionEdgeCases(t *testing.T) {
	multi := []string{
		"# first line",
		"# second line",
		"Decl foo(A.Type<int>).",
	}
	if got := extractDescription(multi, 2); got != "first line second line" {
		t.Errorf("multi-line description = %q", got)
	}
	withBreak := []string{
		"# kept",
		"",
		"# dropped",
		"Decl foo(A.Type<int>).",
	}
	if got := extractDescription(withBreak, 3); got != "kept dropped" {
		t.Errorf("blank lines are skipped, not a reset, got %q", got)
	}
	sectionStop := []string{
		"# SECTION 1A: Intent",
		"# User intent",
		"Decl foo(A.Type<int>).",
	}
	if got := extractDescription(sectionStop, 2); got != "User intent" {
		t.Errorf("section header should stop scan, got %q", got)
	}
	codeBreak := []string{
		"some_code(X) :- other(X).",
		"# comment above decl but broken by code",
		"Decl foo(A.Type<int>).",
	}
	_ = codeBreak
	nonComment := []string{
		"Decl bar(B.Type<int>).",
		"Decl foo(A.Type<int>).",
	}
	if got := extractDescription(nonComment, 1); got != "" {
		t.Errorf("non-comment line should stop scan with empty desc, got %q", got)
	}
	if got := extractDescription([]string{"Decl foo(A.Type<int>)."}, 0); got != "" {
		t.Errorf("decl on first line should have empty desc, got %q", got)
	}
}

func TestExtractExamplesFromMarkdownBranches(t *testing.T) {
	content := strings.Join([]string{
		"## Correct usage",
		"```mangle",
		"user_intent(X) :- focus(X).",
		"```",
		"## Wrong pattern hallucination",
		"```mangle",
		"bad_pred(X) :- missing(X).",
		"```",
		"## Empty block is skipped",
		"```mangle",
		"```",
		"## No predicates here",
		"```python",
		"print('hi')",
		"```",
	}, "\n")
	examples := extractExamplesFromMarkdown(content, "test.md")
	if len(examples) != 4 {
		t.Fatalf("expected 4 examples (2+2), got %d: %+v", len(examples), examples)
	}
	if !examples[0].IsCorrect || !examples[1].IsCorrect {
		t.Errorf("first block under Correct usage should be correct")
	}
	if examples[2].IsCorrect || examples[3].IsCorrect {
		t.Errorf("blocks under Wrong header should be marked anti-pattern")
	}
	if examples[0].Source != "test.md" {
		t.Errorf("source should propagate, got %q", examples[0].Source)
	}
	if got := extractExamplesFromMarkdown("no code blocks", "s.md"); len(got) != 0 {
		t.Errorf("content without mangle blocks should yield none, got %+v", got)
	}
}

func TestExtractPredicatesFromCodeFiltersBuiltins(t *testing.T) {
	got := extractPredicatesFromCode("user_intent(X) :- focus(X), not blocked(X).")
	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	for _, want := range []string{"user_intent", "focus", "blocked"} {
		if !seen[want] {
			t.Errorf("expected predicate %q in %q", want, got)
		}
	}
	builtins := extractPredicatesFromCode("fn(X), do(Y), let(Z), not(Q), true(X), false(Y)")
	if len(builtins) != 0 {
		t.Errorf("builtins should be filtered, got %q", builtins)
	}
	dupes := extractPredicatesFromCode("foo(X) :- foo(X), foo(Y).")
	if len(dupes) != 1 || dupes[0] != "foo" {
		t.Errorf("duplicates should collapse, got %q", dupes)
	}
	if got := extractPredicatesFromCode("no parens here"); len(got) != 0 {
		t.Errorf("code without calls should yield none, got %q", got)
	}
}

func TestGetErrorTypeFromContextBranches(t *testing.T) {
	cases := map[string]string{
		"Atom string confusion":    "atom_string_confusion",
		"Aggregation pipeline":     "aggregation_syntax",
		"Negation safety":          "unsafe_negation",
		"Unbound variable in head": "unbound_variable",
		"Variable binding":         "unbound_variable",
		"Type mismatch":            "type_mismatch",
		"Syntax error":             "parse_error",
		"Something unrelated":      "",
		"":                         "",
	}
	for ctx, want := range cases {
		if got := getErrorTypeFromContext(ctx); got != want {
			t.Errorf("getErrorTypeFromContext(%q)=%q, want %q", ctx, got, want)
		}
	}
	if got := getErrorTypeFromContext("ATOM STRING"); got != "atom_string_confusion" {
		t.Errorf("matching should be case-insensitive, got %q", got)
	}
}

func TestMergePredicatesSortsAndKeepsArityDistinct(t *testing.T) {
	edb := []PredicateEntry{
		{Name: "zebra", Arity: 1, Type: "EDB"},
		{Name: "apple", Arity: 1, Type: "EDB"},
		{Name: "apple", Arity: 1, Type: "EDB"},
	}
	idb := []PredicateEntry{
		{Name: "mango", Arity: 2, Type: "IDB"},
		{Name: "apple", Arity: 2, Type: "IDB"},
		{Name: "zebra", Arity: 1, Type: "IDB"},
	}
	merged := mergePredicates(edb, idb)
	if len(merged) != 4 {
		t.Fatalf("expected 4 entries (dedup + arity-distinct), got %d: %+v", len(merged), merged)
	}
	for i := 1; i < len(merged); i++ {
		if merged[i-1].Name > merged[i].Name {
			t.Fatalf("merged output should be sorted by name, got %+v", merged)
		}
	}
	if got := mergePredicates(nil, nil); len(got) != 0 {
		t.Errorf("merging two empty slices should be empty, got %+v", got)
	}
	idbOnly := mergePredicates(nil, []PredicateEntry{{Name: "solo", Arity: 1, Type: "IDB"}})
	if len(idbOnly) != 1 || idbOnly[0].Type != "IDB" {
		t.Errorf("IDB-only merge should keep IDB, got %+v", idbOnly)
	}
}

func TestCreateErrorPatternsAreUsable(t *testing.T) {
	patterns := createErrorPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected at least one error pattern")
	}
	seen := map[string]bool{}
	for _, p := range patterns {
		if p.Name == "" || p.ErrorRegex == "" || p.CauseDescription == "" || p.FixTemplate == "" || p.Severity == "" {
			t.Errorf("error pattern must populate all fields, got %+v", p)
		}
		if seen[p.Name] {
			t.Errorf("duplicate error pattern name %q", p.Name)
		}
		seen[p.Name] = true
		switch p.Severity {
		case "silent_failure", "parse_error", "runtime_error":
		default:
			t.Errorf("unexpected severity %q for %q", p.Severity, p.Name)
		}
	}
}

func TestPredicateKeyStringFormat(t *testing.T) {
	k := predicateKey{Name: "user_intent", Arity: 2, Type: "EDB"}
	if got, want := k.String(), "user_intent/2 (EDB)"; got != want {
		t.Errorf("String()=%q, want %q", got, want)
	}
}

func TestReadCommittedPredicatesMissingFile(t *testing.T) {
	if _, err := readCommittedPredicates(filepath.Join(t.TempDir(), "does-not-exist.db")); err == nil {
		t.Error("expected error for missing corpus file")
	}
}

func TestParseSchemasDirFiltersAndDedups(t *testing.T) {
	dir := t.TempDir()
	schemaA := "# SECTION 1A: Intent\nDecl user_intent(A.Type<string>).\nDecl dup_pred(A.Type<int>).\n"
	schemaB := "# SECTION 2A: Memory Store\nDecl dup_pred(A.Type<int>).\nDecl memory_fact(A.Type<string>).\n"
	if err := os.WriteFile(filepath.Join(dir, "schemas_a.mg"), []byte(schemaA), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schemas_b.mg"), []byte(schemaB), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.mg"), []byte("Decl should_be_ignored(A.Type<int>).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Decl ignored(A.Type<int>).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preds, err := parseSchemasDir(dir)
	if err != nil {
		t.Fatalf("parseSchemasDir: %v", err)
	}
	names := map[string]int{}
	for _, p := range preds {
		names[p.Name]++
	}
	if names["user_intent"] != 1 || names["memory_fact"] != 1 {
		t.Errorf("expected one of each schema predicate, got %+v", names)
	}
	if names["dup_pred"] != 1 {
		t.Errorf("dup_pred should be deduplicated across files, got %d", names["dup_pred"])
	}
	if names["should_be_ignored"] != 0 {
		t.Errorf("non-schemas .mg file must be skipped")
	}
	if _, err := parseSchemasDir(filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for missing directory")
	}
	emptyDir := t.TempDir()
	if got, err := parseSchemasDir(emptyDir); err != nil || len(got) != 0 {
		t.Errorf("empty dir should yield no predicates without error, got %+v err=%v", got, err)
	}
}

func TestParseSchemaFileDomainPriorityAndSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schemas_x.mg")
	content := strings.Join([]string{
		"# SECTION 5A: Memory Store",
		"# Domain: custom_domain",
		"Decl first_pred(A.Type<string>).",
		"# Priority: 77",
		"# SerializationOrder: 3",
		"# documented second",
		"Decl second_pred(A.Type<int>, B.Type<name>).",
		"not a decl line",
		"Decl third_pred().",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	preds, err := parseSchemaFile(path)
	if err != nil {
		t.Fatalf("parseSchemaFile: %v", err)
	}
	if len(preds) != 3 {
		t.Fatalf("expected 3 predicates, got %d: %+v", len(preds), preds)
	}
	if preds[0].Domain != "custom_domain" {
		t.Errorf("explicit Domain marker should win, got %q", preds[0].Domain)
	}
	if preds[0].Arity != 1 || preds[2].Arity != 0 {
		t.Errorf("arity mismatch: %+v", preds)
	}
	if preds[1].ActivationPriority != 77 || preds[1].SerializationOrder != 3 {
		t.Errorf("priority annotations should apply, got %+v", preds[1])
	}
	if !strings.Contains(preds[1].Description, "documented second") {
		t.Errorf("description should contain trailing comment, got %q", preds[1].Description)
	}
	if preds[0].Type != "EDB" {
		t.Errorf("schema predicates should be EDB, got %q", preds[0].Type)
	}
	if _, err := parseSchemaFile(filepath.Join(dir, "missing.mg")); err == nil {
		t.Error("expected error for missing schema file")
	}
}

func TestExtractExamplesFromSkillMissingDir(t *testing.T) {
	if got := extractExamplesFromSkill(filepath.Join(t.TempDir(), "no-such-dir")); len(got) != 0 {
		t.Errorf("missing skill dir should yield no examples, got %+v", got)
	}
	skillDir := t.TempDir()
	md := "## Correct\n```mangle\nuser_intent(X) :- focus(X).\n```\n"
	if err := os.WriteFile(filepath.Join(skillDir, "300-PATTERN_LIBRARY.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	got := extractExamplesFromSkill(skillDir)
	if len(got) != 2 {
		t.Fatalf("should extract one example per predicate in the block, got %+v", got)
	}
	if got[0].PredicateName != "user_intent" && got[0].PredicateName != "focus" {
		t.Errorf("should extract examples from present skill files, got %+v", got)
	}
}
