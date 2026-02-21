package mangle

import (
	"os"
	"testing"
)

func TestTestVerificationLogic(t *testing.T) {
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

	// Verify that test_passed_after_fix is derived even with 80% confidence (standard success)
	t.Run("test_passed_after_fix with 80 confidence", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []interface{}{"act1", "/run_tests", "/output_scan", int64(80), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "test_passed_after_fix")
		if len(result) == 0 {
			t.Error("Expected test_passed_after_fix() to be derived from action_verified(/run_tests, 80)")
		}
	})
}
