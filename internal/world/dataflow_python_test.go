package world

import (
	"codenerd/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPython(t *testing.T) {
	pythonCode := `
def process_data(data):
    # Assignment
    x = get_data("key")

    # None check early return (guards_return)
    if x is None:
        return None

    # None check block (guards_block)
    if data is not None:
        y = data.value

    # Try/Except (error_checked_block)
    try:
        risky_op(x)
    except Exception:
        pass

    # Uses
    result = x.compute()

    # Call args
    process_result(result, x)

    return result
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(pythonCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.extractPython(tmpFile)
	if err != nil {
		t.Fatalf("extractPython failed: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("Expected facts from Python code, got none")
	}

	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	// Verify specific fact extractions

	// Function scope
	if factTypes["function_scope"] < 1 {
		t.Errorf("Expected at least 1 function_scope fact, got %d", factTypes["function_scope"])
	}

	// Assignment (nullable from get_data())
	if factTypes["assigns"] < 1 {
		t.Errorf("Expected at least 1 assigns fact, got %d", factTypes["assigns"])
	}

	// Guards return
	if factTypes["guards_return"] < 1 {
		t.Errorf("Expected at least 1 guards_return fact, got %d", factTypes["guards_return"])
	}
	if factTypes["guard_dominates"] < 1 {
		t.Errorf("Expected at least 1 guard_dominates fact, got %d", factTypes["guard_dominates"])
	}

	// Guards block
	if factTypes["guards_block"] < 1 {
		t.Errorf("Expected at least 1 guards_block fact, got %d", factTypes["guards_block"])
	}

	// Error checked block (try/except)
	if factTypes["error_checked_block"] < 1 {
		t.Errorf("Expected at least 1 error_checked_block fact, got %d", factTypes["error_checked_block"])
	}

	// Uses (attribute access)
	if factTypes["uses"] < 1 {
		t.Errorf("Expected at least 1 uses fact, got %d", factTypes["uses"])
	}

	// Call args
	if factTypes["call_arg"] < 1 {
		t.Errorf("Expected at least 1 call_arg fact, got %d", factTypes["call_arg"])
	}

	t.Logf("Python extraction summary: %v", factTypes)
}

func TestClassifyPythonAssignment(t *testing.T) {
	pythonCode := `
def test_assign():
    a = None
    b = get_data()
    c = load_info()
    d = normal_call()
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(pythonCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.extractPython(tmpFile)
	if err != nil {
		t.Fatalf("extractPython failed: %v", err)
	}

	assignsFacts := 0
	for _, f := range facts {
		if f.Predicate == "assigns" {
			assignsFacts++
			// Verify it classified them as nullable
			if len(f.Args) > 1 {
				class := f.Args[1].(core.MangleAtom)
				if class != core.MangleAtom("/nullable") {
					t.Errorf("Expected assignment to be classified as /nullable, got %v", class)
				}
			}
		}
	}

	// Should catch a, b, and c as nullable assignments
	if assignsFacts != 3 {
		t.Errorf("Expected 3 classified nullable assignments, got %d", assignsFacts)
	}
}

func TestCheckPythonNoneComparison(t *testing.T) {
	pythonCode := `
def test_none_comp(x, y):
    if x is None:
        pass
    if y is not None:
        pass
    if x == None:
        pass
    if y != None:
        pass
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(pythonCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.extractPython(tmpFile)
	if err != nil {
		t.Fatalf("extractPython failed: %v", err)
	}

	guardBlockFacts := 0
	for _, f := range facts {
		if f.Predicate == "guards_block" {
			guardBlockFacts++
		}
	}

	// We expect 2 guards_block facts because only the `is not None` and `!= None` generate guards_block
	// Since `is None` and `== None` generate early return check but `pass` has no early return, they won't emit anything
	if guardBlockFacts != 2 {
		t.Errorf("Expected 2 guards_block facts, got %d", guardBlockFacts)
	}
}
