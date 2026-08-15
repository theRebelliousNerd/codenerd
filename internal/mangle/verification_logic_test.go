package mangle

import (
	"os"
	"testing"
)

func TestTestVerificationLogic(t *testing.T) {
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
`

	program := mockSchema + "\n" + string(data)

	// Verify that test_passed_after_fix is derived even with 80% confidence (standard success)
	t.Run("test_passed_after_fix with 80 confidence", func(t *testing.T) {
		facts := []testFact{
			{"action_verified", []any{"act1", "/run_tests", "/output_scan", int64(80), int64(12345)}},
		}
		result := evaluateAndQuery(t, program, facts, "test_passed_after_fix")
		if len(result) == 0 {
			t.Error("Expected test_passed_after_fix() to be derived from action_verified(/run_tests, 80)")
		}
	})
}
