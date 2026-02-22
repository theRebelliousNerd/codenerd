package mangle

import (
	"os"
	"testing"
)

// TestIntentImports reproduces the missing wiring for imported files in intent_routing.mg.
func TestIntentImports(t *testing.T) {
	// Load the intent routing rules
	intentRoutingPath := findMangleFile(t, "intent_routing.mg")
	data, err := os.ReadFile(intentRoutingPath)
	if err != nil {
		t.Fatalf("Failed to read intent_routing.mg: %v", err)
	}

	// Mock schema declarations to support the test.
	// We include file_imports here, which is normally in schemas_codedom_polyglot.mg
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

# The predicate we want to wire:
Decl file_imports(Importer, Imported).
`

	program := mockSchema + "\n" + string(data)

	t.Run("context_priority for imported files", func(t *testing.T) {
		targetFile := "main.go"
		importedFile := "helper.go"

		facts := []testFact{
			// User is interested in main.go
			{"user_intent", []interface{}{"intent1", "/command", "/fix", targetFile, ""}},

			// main.go imports helper.go
			{"file_imports", []interface{}{targetFile, importedFile}},

			// Both files exist
			{"file_exists", []interface{}{targetFile}},
			{"file_exists", []interface{}{importedFile}},
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
