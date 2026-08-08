package mangle

import (
	"testing"
)

// TestNewSchemaValidator tests validator construction.
func TestNewSchemaValidator(t *testing.T) {
	sv := NewSchemaValidator("", "")
	if sv == nil {
		t.Fatal("Expected non-nil validator")
	}
	if sv.declaredPredicates == nil {
		t.Error("Expected declaredPredicates map to be initialized")
	}
	if sv.predicateArities == nil {
		t.Error("Expected predicateArities map to be initialized")
	}
	// TODO: TEST_GAP - Concurrent map access for declaredPredicates
}

// TestLoadDeclaredPredicates tests predicate extraction from schemas.
func TestLoadDeclaredPredicates(t *testing.T) {
	schemas := `
# Core predicates
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
Decl next_action(Action.Type<name>).
`
	sv := NewSchemaValidator(schemas, "")
	err := sv.LoadDeclaredPredicates()
	if err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	// Check that predicates were extracted
	if !sv.IsDeclared("user_intent") {
		t.Error("Expected user_intent to be declared")
	}
	if !sv.IsDeclared("file_topology") {
		t.Error("Expected file_topology to be declared")
	}
	if !sv.IsDeclared("next_action") {
		t.Error("Expected next_action to be declared")
	}

	// TODO: Missing Test: Nil/Missing learned text. Verify behavior when learnedText is empty but schemasText is populated.
	// Check that undeclared predicate returns false
	if sv.IsDeclared("nonexistent_predicate") {
		t.Error("Expected nonexistent_predicate to not be declared")
	}

	// TODO: TEST_GAP - Conflicting schema declarations (multiple arities)
}

// TestGetArity tests arity extraction from declarations.
func TestGetArity(t *testing.T) {
	schemas := `
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
Decl next_action(Action.Type<name>).
Decl diagnostic(File.Type<string>, Line.Type<int>, Col.Type<int>, Msg.Type<string>, Severity.Type<name>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	tests := []struct {
		name          string
		predicate     string
		expectedArity int
	}{
		{"user_intent has 5 args", "user_intent", 5},
		{"file_topology has 1 arg", "file_topology", 1},
		{"next_action has 1 arg", "next_action", 1},
		{"diagnostic has 5 args", "diagnostic", 5},
		{"unknown predicate returns -1", "unknown_pred", -1},
		// TODO: Missing Test: Type declaration comma vulnerability. E.g., `Decl generic_map(Map.Type<string, string>)`. `extractDeclsFromText` counts commas directly and will miscount generics.
	}

	// TODO: TEST_GAP - Malformed syntax (missing parens, trailing commas)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arity := sv.GetArity(tt.predicate)
			if arity != tt.expectedArity {
				t.Errorf("GetArity(%s) = %d, want %d", tt.predicate, arity, tt.expectedArity)
			}
		})
	}
}

// TestCheckArity tests arity validation.
func TestCheckArity(t *testing.T) {
	schemas := `
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	tests := []struct {
		name        string
		predicate   string
		actualArity int
		expectError bool
	}{
		{"correct arity passes", "user_intent", 5, false},
		{"wrong arity fails", "user_intent", 3, true},
		{"wrong arity fails (too many)", "user_intent", 7, true},
		{"correct single arg passes", "file_topology", 1, false},
		// TODO: Missing Test: 0-arity predicates. E.g., `trigger()`. The parser might miscount an empty arg list if formatted with spaces `( )`.
		// {"zero arity passes", "trigger", 0, false},
		{"unknown predicate passes", "unknown_pred", 10, false},
		// TODO: Missing Test: String literal comma vulnerability. E.g., `diagnostic("/src/main.go", 10, 5, "Error: expected }, found EOF", /high)`. The parser ignores quote state and will miscount commas inside strings.
		// TODO: TEST_GAP - Extreme Arity inputs (>1000 arguments)
	}

	// TODO: TEST_GAP - Malformed syntax (missing parens, trailing commas)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.CheckArity(tt.predicate, tt.actualArity)
			if tt.expectError && err == nil {
				t.Errorf("CheckArity(%s, %d) expected error, got nil", tt.predicate, tt.actualArity)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CheckArity(%s, %d) expected nil, got error: %v", tt.predicate, tt.actualArity, err)
			}
		})
	}
}

// TODO: Missing Test: Catastrophic Regex Backtracking (ReDoS). Inject 1MB of whitespace into a rule definition to ensure `ValidateRule` completes within bounded time.

// TODO: Missing Test: Extreme Arity and Recursive Nesting. Test a rule with 1,000 nested parenthesis levels and 10,000 arguments to verify O(N) performance bounds without stack overflows.

// TestSetPredicateArity tests manual arity setting.
func TestSetPredicateArity(t *testing.T) {
	sv := NewSchemaValidator("", "")

	// Initially unknown
	if sv.GetArity("custom_pred") != -1 {
		t.Error("Expected unknown arity for undeclared predicate")
	}

	// Set arity manually
	sv.SetPredicateArity("custom_pred", 3)

	// Now should be known
	if sv.GetArity("custom_pred") != 3 {
		t.Errorf("Expected arity 3, got %d", sv.GetArity("custom_pred"))
	}

	// Check arity validation with manually set arity
	if err := sv.CheckArity("custom_pred", 3); err != nil {
		t.Errorf("CheckArity with correct arity should pass: %v", err)
	}
	if err := sv.CheckArity("custom_pred", 5); err == nil {
		t.Error("CheckArity with wrong arity should fail")
	}
}

// TestValidateRule tests rule validation with declared predicates.
func TestValidateRule(t *testing.T) {
	schemas := `
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
Decl next_action(Action.Type<name>).
Decl diagnostic(File.Type<string>, Line.Type<int>, Col.Type<int>, Msg.Type<string>, Severity.Type<name>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	tests := []struct {
		name        string
		rule        string
		expectError bool
	}{
		{
			"valid rule with declared predicates",
			"next_action(/review) :- user_intent(_, /mutation, /review, _, _), file_topology(_).",
			false,
		},
		{
			"invalid rule with undefined predicate",
			"next_action(/review) :- undefined_predicate(X), file_topology(_).",
			true,
		},
		{
			"fact (no body) is valid",
			"file_topology(\"/src/main.go\").",
			false,
		},
		{
			"rule with only builtins in body is valid",
			"result(X) :- count(X).",
			false,
		},
		// TODO: Missing Test: Empty rule body / Dangling operator. E.g., `candidate_action(/test) :- .`
		// The regex `([a-z_][a-z0-9_]*)\s*\(` will find no matches, passing validation incorrectly.
	}

	// TODO: TEST_GAP - Malformed syntax (missing parens, trailing commas)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.ValidateRule(tt.rule)
			if tt.expectError && err == nil {
				t.Errorf("ValidateRule expected error for: %s", tt.rule)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateRule unexpected error for: %s: %v", tt.rule, err)
			}
		})
	}
}


// TestValidateRules tests validation of multiple rules at once.
func TestValidateRules(t *testing.T) {
	schemas := `
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
Decl next_action(Action.Type<name>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	tests := []struct {
		name          string
		rules         []string
		expectedError int
	}{
		{
			"all valid rules",
			[]string{
				"next_action(/review) :- user_intent(_, /mutation, /review, _, _), file_topology(_).",
				"file_topology(\"/src/main.go\").",
			},
			0,
		},
		{
			"partial failure",
			[]string{
				"next_action(/review) :- user_intent(_, /mutation, /review, _, _), file_topology(_).",
				"next_action(/review) :- undefined_predicate(X).", // Invalid
			},
			1,
		},
		{
			"all failure",
			[]string{
				"next_action(/review) :- undefined1(X).",
				"next_action(/review) :- undefined2(X).",
			},
			2,
		},
		{
			"empty array",
			[]string{},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := sv.ValidateRules(tt.rules)
			if len(errors) != tt.expectedError {
				t.Errorf("ValidateRules expected %d errors, got %d: %v", tt.expectedError, len(errors), errors)
			}
		})
	}
}

// TestValidateLearnedRule tests protection of forbidden learned heads.
func TestValidateLearnedRule(t *testing.T) {
	schemas := `
Decl permitted(Action.Type<name>).
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	tests := []struct {
		name        string
		rule        string
		expectError bool
	}{
		{
			"normal learned rule is valid",
			"candidate_action(/test) :- user_intent(_, _, /test, _, _).",
			false, // head predicates are valid; only body undefined predicates fail
		},
		{
			"learned rule for permitted is forbidden",
			"permitted(/dangerous) :- user_intent(_, _, _, _, _).",
			true,
		},
		{
			"learned rule for safe_action is forbidden",
			"safe_action(/rm) :- user_intent(_, _, _, _, _).",
			true,
		},
		// TODO: Missing Test: Unbalanced parentheses coercion. E.g., `user_intent(A, B, C`. The `validateHeadArity` depth loop might break early and silently allow it to pass.
		{
			"comment is valid",
			"# This is a comment",
			false,
		},
		// TODO: Missing Test: Whitespace/multiline coercion bypass. E.g., `  permitted(/dangerous) :- user_intent(_, _, _, _, _).`. The regex `(?m)^([a-z_][a-z0-9_]*)\s*\(` requires line start and misses indented forbidden heads.
		{
			"empty line is valid",
			"",
			false,
		},
		// TODO: TEST_GAP - Builtin redefinition attempts in learned rules
		// TODO: TEST_GAP - Unicode/Case sensitivity in predicate names
	}

	// TODO: TEST_GAP - Malformed syntax (missing parens, trailing commas)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.ValidateLearnedRule(tt.rule)
			if tt.expectError && err == nil {
				t.Errorf("ValidateLearnedRule expected error for: %s", tt.rule)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateLearnedRule unexpected error for: %s: %v", tt.rule, err)
			}
		})
	}
}

// TestGetDeclaredPredicates tests retrieval of all declared predicates.
func TestGetDeclaredPredicates(t *testing.T) {
	schemas := `
Decl user_intent(ID.Type<string>, Category.Type<name>, Verb.Type<name>, Target.Type<string>, Constraint.Type<string>).
Decl file_topology(Path.Type<string>).
Decl next_action(Action.Type<name>).
`
	sv := NewSchemaValidator(schemas, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	predicates := sv.GetDeclaredPredicates()
	if len(predicates) != 3 {
		t.Errorf("Expected 3 predicates, got %d", len(predicates))
	}

	// Check all expected predicates are present
	expected := map[string]bool{"user_intent": true, "file_topology": true, "next_action": true}
	for _, p := range predicates {
		if !expected[p] {
			t.Errorf("Unexpected predicate: %s", p)
		}
		delete(expected, p)
	}
	if len(expected) > 0 {
		t.Errorf("Missing predicates: %v", expected)
	}
}

// TODO: Missing Test: Concurrent Map Reads and Writes. Go maps are not thread-safe. Test rapidly reading (ValidateRule) and writing (SetPredicateArity) simultaneously to expose race conditions.

// TODO: Missing Test: Phantom State Across Evaluations. Test calling LoadDeclaredPredicates multiple times with different/contradictory schemas to verify if stale map state persists.

// TestLearnedRulesExtractHeads tests that rule heads from learned.mg are extracted.
func TestLearnedRulesExtractHeads(t *testing.T) {
	schemas := `
Decl base_predicate(X.Type<name>).
`
	learned := `
# Learned rules
derived_fact(X) :- base_predicate(X).
another_derived(Y) :- derived_fact(Y).
`
	sv := NewSchemaValidator(schemas, learned)
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("LoadDeclaredPredicates failed: %v", err)
	}

	// Base predicates from schemas should be declared
	if !sv.IsDeclared("base_predicate") {
		t.Error("Expected base_predicate to be declared")
	}

	// Head predicates from learned rules should also be declared
	if !sv.IsDeclared("derived_fact") {
		t.Error("Expected derived_fact to be declared (from learned head)")
	}
	if !sv.IsDeclared("another_derived") {
		t.Error("Expected another_derived to be declared (from learned head)")
	}
}
