package mangle

import (
	"os"
	"testing"
)

func TestIntentWiring(t *testing.T) {
	// Load the intent routing rules
	intentRoutingPath := findMangleFile(t, "intent_routing_rules.mg")
	data, err := os.ReadFile(intentRoutingPath)
	if err != nil {
		t.Fatalf("Failed to read intent_routing.mg: %v", err)
	}

	// Define base schema declarations that intent_routing.mg depends on
	// These are normally in schemas_*.mg, but we mock them here for isolation
	mockSchema := `
# Declarations the routing file no longer carries.
#
# intent_routing_rules.mg moved into internal/core/defaults/policy/, where the
# constitution already declares these five predicates. A duplicate Decl fails
# the whole program analysis with "declared more than once", so the file had to
# drop them — which means a standalone harness like this one has to supply what
# the corpus supplies in production.
Decl file_contains(FilePath, Pattern).
Decl file_imports(Importer, Imported).
Decl same_package(File1, File2).
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message).
Decl pytest_failure(TestName, ErrorCategory, RootFile, RootLine, Message).
# Mock Schema Declarations
Decl user_intent(ID, Category, Verb, Target, Constraint).
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).
Decl file_exists(Path).
Decl file_edited(Path).
Decl action_verified(ID, Type, Method, Confidence, Timestamp).
Decl test_state(State).
Decl tdd_state(State).
Decl next_action(Action).
Decl persona_tool_allowed(Persona, Tool).
`

	program := mockSchema + "\n" + string(data)

	t.Run("persona_tool_allowed derivation", func(t *testing.T) {
		// Test that /coder persona gets write_file permission
		// persona(/coder) :- user_intent(_, _, /fix, _, _).
		facts := []testFact{
			{"user_intent", []any{"id1", "/command", "/fix", "file.go", "/none"}},
		}
		result := evaluateAndQuery(t, program, facts, "persona_tool_allowed")

		found := false
		for _, f := range result {
			// persona_tool_allowed(Persona, Tool)
			if len(f.Args) == 2 {
				persona, ok1 := f.Args[0].(string)
				tool, ok2 := f.Args[1].(string)

				if ok1 && ok2 && persona == "/coder" && tool == "/write_file" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Errorf("Expected persona_tool_allowed(/coder, /write_file), got: %v", result)
		}
	})

	t.Run("code_modified_recently derivation", func(t *testing.T) {
		facts := []testFact{
			{"file_edited", []any{"main.go"}},
		}
		result := evaluateAndQuery(t, program, facts, "code_modified_recently")
		if len(result) == 0 {
			t.Error("Expected code_modified_recently() to be derived from file_edited()")
		}
	})

	t.Run("tests_run_recently derivation", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []any{"act1", "/run_tests", "/output_scan", int64(100), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "tests_run_recently")
		if len(result) == 0 {
			t.Error("Expected tests_run_recently() to be derived from action_verified(/run_tests)")
		}
	})

	t.Run("test_passed_after_fix derivation", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []any{"act1", "/run_tests", "/output_scan", int64(100), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "test_passed_after_fix")
		if len(result) == 0 {
			t.Error("Expected test_passed_after_fix() to be derived from action_verified(/run_tests, 100)")
		}
	})

	t.Run("diagnostic_active derivation", func(t *testing.T) {
		facts := []testFact{
			{"diagnostic", []any{"/error", "main.go", int64(10), "E01", "syntax error"}},
		}
		result := evaluateAndQuery(t, program, facts, "diagnostic_active")

		found := false
		for _, f := range result {
			// diagnostic_active(Path, Line, Severity, Message)
			// Args: ["main.go", 10, "/error", "syntax error"]
			if len(f.Args) == 4 {
				path, ok1 := f.Args[0].(string)
				sev, ok3 := f.Args[2].(string)
				msg, ok4 := f.Args[3].(string)

				if ok1 && ok3 && ok4 && path == "main.go" && sev == "/error" && msg == "syntax error" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Errorf("Expected diagnostic_active(main.go, 10, /error, syntax error), got: %v", result)
		}
	})

	t.Run("code_quality_issue derivation", func(t *testing.T) {
		facts := []testFact{
			{"diagnostic", []any{"/warning", "main.go", int64(5), "W01", "unused var"}},
		}
		result := evaluateAndQuery(t, program, facts, "code_quality_issue")

		found := false
		for _, f := range result {
			// code_quality_issue(/diagnostic, Message)
			if len(f.Args) == 2 {
				issue, ok1 := f.Args[0].(string)
				msg, ok2 := f.Args[1].(string)

				if ok1 && ok2 && issue == "/diagnostic" && msg == "unused var" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Errorf("Expected code_quality_issue(/diagnostic, unused var), got: %v", result)
		}
	})

	t.Run("tdd_state green derivation", func(t *testing.T) {
		// tdd_state(/green) :- !test_failed(_, _, _), code_modified_recently().
		// We assert file_edited -> code_modified_recently
		// We verify tdd_state(/green) is derived (assuming no test_failed facts)
		facts := []testFact{
			{"file_edited", []any{"main.go"}},
		}
		result := evaluateAndQuery(t, program, facts, "tdd_state")

		found := false
		for _, f := range result {
			if len(f.Args) > 0 {
				if state, ok := f.Args[0].(string); ok && state == "/green" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Error("Expected tdd_state(/green) when file edited and no failures")
		}
	})

	t.Run("test_failed derivation", func(t *testing.T) {
		// test_failed(Path, Name, Msg) :- pytest_failure(Name, _, Path, _, Msg).
		facts := []testFact{
			{"pytest_failure", []any{"test_foo", "/assertion", "foo_test.py", int64(10), "assert 1 == 2"}},
		}
		result := evaluateAndQuery(t, program, facts, "test_failed")

		found := false
		for _, f := range result {
			// test_failed(Path, Name, Msg)
			if len(f.Args) == 3 {
				path, ok1 := f.Args[0].(string)
				name, ok2 := f.Args[1].(string)
				msg, ok3 := f.Args[2].(string)

				if ok1 && ok2 && ok3 && path == "foo_test.py" && name == "test_foo" && msg == "assert 1 == 2" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Errorf("Expected test_failed(foo_test.py, test_foo, assert 1 == 2), got: %v", result)
		}
	})
}

func TestGroundedWebSearchRouting(t *testing.T) {
	intentRoutingPath := findMangleFile(t, "intent_routing_rules.mg")
	data, err := os.ReadFile(intentRoutingPath)
	if err != nil {
		t.Fatalf("Failed to read intent_routing.mg: %v", err)
	}
	mockSchema := `
# Declarations the routing file no longer carries; see the note above.
Decl file_contains(FilePath, Pattern).
Decl file_imports(Importer, Imported).
Decl same_package(File1, File2).
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message).
Decl pytest_failure(TestName, ErrorCategory, RootFile, RootLine, Message).
Decl user_intent(ID, Category, Verb, Target, Constraint).
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).
Decl file_exists(Path).
Decl file_edited(Path).
Decl action_verified(ID, Type, Method, Confidence, Timestamp).
Decl test_state(State).
Decl tdd_state(State).
Decl next_action(Action).
Decl persona_tool_allowed(Persona, Tool).
Decl modular_tool_allowed(Tool, Intent).
Decl modular_tool_priority(Tool, Priority).
`
	program := mockSchema + "\n" + string(data)

	t.Run("researcher persona has grounded_web_search", func(t *testing.T) {
		result := evaluateAndQuery(t, program, nil, "persona_tool_allowed")
		found := false
		for _, f := range result {
			if len(f.Args) == 2 {
				persona, ok1 := f.Args[0].(string)
				tool, ok2 := f.Args[1].(string)
				if ok1 && ok2 && persona == "/researcher" && tool == "/grounded_web_search" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Expected persona_tool_allowed(/researcher, /grounded_web_search), got: %v", result)
		}
	})

	t.Run("coder persona does NOT have grounded_web_search", func(t *testing.T) {
		result := evaluateAndQuery(t, program, []testFact{
			{"user_intent", []any{"id1", "/command", "/fix", "file.go", "/none"}},
		}, "persona_tool_allowed")
		for _, f := range result {
			if len(f.Args) == 2 {
				persona, ok1 := f.Args[0].(string)
				tool, ok2 := f.Args[1].(string)
				if ok1 && ok2 && persona == "/coder" && tool == "/grounded_web_search" {
					t.Errorf("coder must not have grounded_web_search, but found: %v", result)
					break
				}
			}
		}
	})

	t.Run("modular_tool_allowed for research", func(t *testing.T) {
		facts := []testFact{
			{"user_intent", []any{"id1", "/command", "/research", "topic", "/none"}},
		}
		result := evaluateAndQuery(t, program, facts, "modular_tool_allowed")
		found := false
		for _, f := range result {
			if len(f.Args) == 2 {
				tool, ok1 := f.Args[0].(string)
				intent, ok2 := f.Args[1].(string)
				if ok1 && ok2 && tool == "/grounded_web_search" && intent == "/research" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Expected modular_tool_allowed(/grounded_web_search, /research), got: %v", result)
		}
	})

	t.Run("modular_tool_allowed for verify", func(t *testing.T) {
		facts := []testFact{
			{"user_intent", []any{"id1", "/command", "/verify", "topic", "/none"}},
		}
		result := evaluateAndQuery(t, program, facts, "modular_tool_allowed")
		found := false
		for _, f := range result {
			if len(f.Args) == 2 {
				tool, ok1 := f.Args[0].(string)
				intent, ok2 := f.Args[1].(string)
				if ok1 && ok2 && tool == "/grounded_web_search" && intent == "/verify" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Expected modular_tool_allowed(/grounded_web_search, /verify), got: %v", result)
		}
	})

	t.Run("fix does NOT derive grounded_web_search", func(t *testing.T) {
		facts := []testFact{
			{"user_intent", []any{"id1", "/command", "/fix", "file.go", "/none"}},
		}
		result := evaluateAndQuery(t, program, facts, "modular_tool_allowed")
		for _, f := range result {
			if len(f.Args) == 2 {
				tool, ok1 := f.Args[0].(string)
				if ok1 && tool == "/grounded_web_search" {
					t.Errorf("fix intent must not derive grounded_web_search, got: %v", result)
					break
				}
			}
		}
	})

	t.Run("create code intent does NOT derive grounded_web_search", func(t *testing.T) {
		facts := []testFact{
			{"user_intent", []any{"id1", "/command", "/create", "file.go", "/none"}},
		}
		result := evaluateAndQuery(t, program, facts, "modular_tool_allowed")
		for _, f := range result {
			if len(f.Args) == 2 {
				tool, ok1 := f.Args[0].(string)
				if ok1 && tool == "/grounded_web_search" {
					t.Errorf("create code intent must not derive grounded_web_search, got: %v", result)
					break
				}
			}
		}
	})

	t.Run("explore research and validate verify also allow grounded", func(t *testing.T) {
		for _, verb := range []string{"/explore", "/validate"} {
			facts := []testFact{
				{"user_intent", []any{"id1", "/command", verb, "topic", "/none"}},
			}
			result := evaluateAndQuery(t, program, facts, "modular_tool_allowed")
			found := false
			for _, f := range result {
				if len(f.Args) == 2 {
					tool, ok1 := f.Args[0].(string)
					intent, ok2 := f.Args[1].(string)
					if ok1 && ok2 && tool == "/grounded_web_search" && intent == verb {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("Expected modular_tool_allowed(/grounded_web_search, %s), got: %v", verb, result)
			}
		}
	})
}
