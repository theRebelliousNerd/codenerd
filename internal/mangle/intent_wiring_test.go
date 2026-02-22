package mangle

import (
	"os"
	"testing"
)

func TestIntentWiring(t *testing.T) {
	// Load the intent routing rules
	intentRoutingPath := findMangleFile(t, "intent_routing.mg")
	data, err := os.ReadFile(intentRoutingPath)
	if err != nil {
		t.Fatalf("Failed to read intent_routing.mg: %v", err)
	}

	// Define base schema declarations that intent_routing.mg depends on
	// These are normally in schemas_*.mg, but we mock them here for isolation
	mockSchema := `
# Mock Schema Declarations
Decl user_intent(ID, Category, Verb, Target, Constraint).
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).
Decl file_exists(Path).
Decl file_edited(Path).
Decl action_verified(ID, Type, Method, Confidence, Timestamp).
Decl diagnostic(Severity, FilePath, Line, Code, Message).
Decl test_state(State).
Decl tdd_state(State).
Decl next_action(Action).
Decl same_package(File1, File2).
Decl file_imports(Importer, Imported).
`

	program := mockSchema + "\n" + string(data)

	t.Run("code_modified_recently derivation", func(t *testing.T) {
		facts := []testFact{
			{"file_edited", []interface{}{"main.go"}},
		}
		result := evaluateAndQuery(t, program, facts, "code_modified_recently")
		if len(result) == 0 {
			t.Error("Expected code_modified_recently() to be derived from file_edited()")
		}
	})

	t.Run("tests_run_recently derivation", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []interface{}{"act1", "/run_tests", "/output_scan", int64(100), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "tests_run_recently")
		if len(result) == 0 {
			t.Error("Expected tests_run_recently() to be derived from action_verified(/run_tests)")
		}
	})

	t.Run("test_passed_after_fix derivation", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []interface{}{"act1", "/run_tests", "/output_scan", int64(100), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "test_passed_after_fix")
		if len(result) == 0 {
			t.Error("Expected test_passed_after_fix() to be derived from action_verified(/run_tests, 100)")
		}
	})

	t.Run("diagnostic_active derivation", func(t *testing.T) {
		facts := []testFact{
			{"diagnostic", []interface{}{"/error", "main.go", int64(10), "E01", "syntax error"}},
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
			{"diagnostic", []interface{}{"/warning", "main.go", int64(5), "W01", "unused var"}},
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
			{"file_edited", []interface{}{"main.go"}},
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
}
