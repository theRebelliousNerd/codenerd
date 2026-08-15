package mangle

import (
	"os"
	"testing"
)

// TestIntentImports reproduces the missing wiring for imported files in intent_routing.mg.
func TestIntentImports(t *testing.T) {
	// Load the intent routing rules
	intentRoutingPath := findMangleFile(t, "intent_routing_rules.mg")
	data, err := os.ReadFile(intentRoutingPath)
	if err != nil {
		t.Fatalf("Failed to read intent_routing.mg: %v", err)
	}

	// Mock schema declarations to support the test.
	// We include file_imports here, which is normally in schemas_codedom_polyglot.mg
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

# The predicate we want to wire:
`

	program := mockSchema + "\n" + string(data)

	t.Run("context_priority for imported files", func(t *testing.T) {
		targetFile := "main.go"
		importedFile := "helper.go"

		facts := []testFact{
			// User is interested in main.go
			{"user_intent", []any{"intent1", "/command", "/fix", targetFile, ""}},

			// main.go imports helper.go
			{"file_imports", []any{targetFile, importedFile}},

			// Both files exist
			{"file_exists", []any{targetFile}},
			{"file_exists", []any{importedFile}},
		}

		// Query context_priority for the imported file
		// Expected rule in intent_routing.mg:
		// context_priority(Path, 50) :- user_intent(..., Target, ...), imports(Target, Path), file_exists(Path).
		// And imports(Target, Path) :- file_imports(Target, Path). (MISSING)

		result := evaluateAndQuery(t, program, facts, "context_priority")

		found := false
		for _, f := range result {
			// context_priority(Path, Priority)
			if len(f.Args) == 2 {
				path, ok1 := f.Args[0].(string)
				prio, ok2 := f.Args[1].(int64)

				if ok1 && ok2 && path == importedFile && prio == 50 {
					found = true
					break
				}
			}
		}

		if !found {
			t.Errorf("Expected context_priority(%s, 50), got: %v", importedFile, result)
		}
	})
}
