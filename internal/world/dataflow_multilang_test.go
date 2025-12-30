package world

import (
	"codenerd/internal/core"
	"os"
	"path/filepath"
	"testing"
)

func TestMultiLangDataFlowExtractor_DetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.js", "javascript"},
		{"qux.jsx", "javascript"},
		{"quux.ts", "typescript"},
		{"corge.tsx", "typescript"},
		{"grault.rs", "rust"},
		{"garply.txt", ""},
		{"waldo.md", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectLanguage(tt.path)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMultiLangDataFlowExtractor_Python(t *testing.T) {
	pythonCode := `
def process_user(user_id):
    user = get_user(user_id)

    if user is None:
        return None

    # This should be safe after the guard
    name = user.name

    try:
        result = risky_operation(user)
    except Exception:
        return None

    return result

def unsafe_access(data):
    item = data.get("key")
    # This access is not guarded
    value = item.field
    return value
`

	// Write temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(pythonCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	// Verify we got facts
	if len(facts) == 0 {
		t.Error("Expected facts from Python code, got none")
	}

	// Check for specific fact types
	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	// Should have function_scope facts
	if factTypes["function_scope"] < 2 {
		t.Errorf("Expected at least 2 function_scope facts, got %d", factTypes["function_scope"])
	}

	// Should have guards_return for the None check
	if factTypes["guards_return"] == 0 {
		t.Error("Expected guards_return fact for 'if user is None: return'")
	}

	// Should have error_checked_block for try/except
	if factTypes["error_checked_block"] == 0 {
		t.Error("Expected error_checked_block fact for try/except")
	}

	// Should have uses for attribute accesses
	if factTypes["uses"] == 0 {
		t.Error("Expected uses facts for attribute accesses")
	}

	t.Logf("Python extraction summary: %v", factTypes)
}

func TestMultiLangDataFlowExtractor_TypeScript(t *testing.T) {
	tsCode := `
function processUser(userId: string): User | null {
    const user = getUser(userId);

    if (user === null) {
        return null;
    }

    // Safe after guard
    const name = user.name;

    // Optional chaining (safe by design)
    const email = user?.email;

    try {
        riskyOperation(user);
    } catch (e) {
        return null;
    }

    return user;
}

function unsafeAccess(data: any): string {
    const item = data.get("key");
    // Not guarded
    return item.value;
}
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.ts")
	if err := os.WriteFile(tmpFile, []byte(tsCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	if len(facts) == 0 {
		t.Error("Expected facts from TypeScript code, got none")
	}

	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	if factTypes["function_scope"] < 2 {
		t.Errorf("Expected at least 2 function_scope facts, got %d", factTypes["function_scope"])
	}

	if factTypes["guards_return"] == 0 {
		t.Error("Expected guards_return fact for null check")
	}

	if factTypes["error_checked_block"] == 0 {
		t.Error("Expected error_checked_block fact for try/catch")
	}

	t.Logf("TypeScript extraction summary: %v", factTypes)
}

func TestMultiLangDataFlowExtractor_JavaScript(t *testing.T) {
	jsCode := `
function processData(input) {
    const data = getData(input);

    if (!data) {
        return null;
    }

    // Safe after truthy check
    const value = data.value;

    // Optional chaining
    const nested = data?.nested?.value;

    return value;
}
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(tmpFile, []byte(jsCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	if len(facts) == 0 {
		t.Error("Expected facts from JavaScript code, got none")
	}

	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	t.Logf("JavaScript extraction summary: %v", factTypes)
}

func TestMultiLangDataFlowExtractor_Rust(t *testing.T) {
	rustCode := `
fn process_user(user_id: u64) -> Option<String> {
    let user = get_user(user_id);

    if user.is_none() {
        return None;
    }

    // if let pattern (safe extraction)
    if let Some(u) = user {
        let name = u.name.clone();
        return Some(name);
    }

    None
}

fn with_result(input: &str) -> Result<String, Error> {
    let data = parse_data(input)?;  // ? operator propagates error

    match data {
        Some(d) => Ok(d.value),
        None => Err(Error::NotFound),
    }
}
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.rs")
	if err := os.WriteFile(tmpFile, []byte(rustCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	if len(facts) == 0 {
		t.Error("Expected facts from Rust code, got none")
	}

	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	// Should have function_scope
	if factTypes["function_scope"] < 2 {
		t.Errorf("Expected at least 2 function_scope facts, got %d", factTypes["function_scope"])
	}

	// Should detect safe_access patterns (if_let, match)
	if factTypes["safe_access"] == 0 {
		t.Error("Expected safe_access facts for if let or match patterns")
	}

	t.Logf("Rust extraction summary: %v", factTypes)
}

func TestMultiLangDataFlowExtractor_UnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow should not error for unsupported: %v", err)
	}

	if facts != nil && len(facts) > 0 {
		t.Errorf("Expected no facts for unsupported language, got %d", len(facts))
	}
}

func TestMultiLangDataFlowExtractor_DelegatesGoToNativeParser(t *testing.T) {
	goCode := `
package test

func example() {
	x := GetValue()
	if x != nil {
		x.DoSomething()
	}
}
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte(goCode), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	extractor := NewMultiLangDataFlowExtractor()
	defer extractor.Close()

	facts, err := extractor.ExtractDataFlow(tmpFile)
	if err != nil {
		t.Fatalf("ExtractDataFlow failed: %v", err)
	}

	if len(facts) == 0 {
		t.Error("Expected facts from Go code, got none")
	}

	// Verify we got Go-style facts (uses native parser)
	factTypes := make(map[string]int)
	for _, f := range facts {
		factTypes[f.Predicate]++
	}

	if factTypes["guards_block"] == 0 {
		t.Error("Expected guards_block fact for nil check in Go")
	}

	t.Logf("Go extraction summary: %v", factTypes)
}

func TestSummarizeMultiLangDataFlow(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "assigns", Args: []interface{}{core.MangleAtom("/x"), core.MangleAtom("/nullable"), "test.py", int64(1)}},
		{Predicate: "assigns", Args: []interface{}{core.MangleAtom("/y"), core.MangleAtom("/option"), "test.rs", int64(2)}},
		{Predicate: "assigns", Args: []interface{}{core.MangleAtom("/z"), core.MangleAtom("/result"), "test.rs", int64(3)}},
		{Predicate: "guards_block", Args: []interface{}{core.MangleAtom("/x"), core.MangleAtom("/nil_check"), "test.ts", int64(4), int64(10)}},
		{Predicate: "guards_return", Args: []interface{}{core.MangleAtom("/x"), core.MangleAtom("/none_check"), "test.py", int64(5)}},
		{Predicate: "safe_access", Args: []interface{}{core.MangleAtom("/x"), core.MangleAtom("/optional_chain"), "test.ts", int64(6)}},
		{Predicate: "uses", Args: []interface{}{"test.py", core.MangleAtom("/func"), core.MangleAtom("/x"), int64(7)}},
		{Predicate: "call_arg", Args: []interface{}{core.MangleAtom("/callsite"), int64(0), core.MangleAtom("/x"), "test.js", int64(8)}},
	}

	summary := SummarizeMultiLangDataFlow(facts)

	if summary.TotalFacts != 8 {
		t.Errorf("TotalFacts = %d, want 8", summary.TotalFacts)
	}

	if summary.AssignmentsFacts != 3 {
		t.Errorf("AssignmentsFacts = %d, want 3", summary.AssignmentsFacts)
	}

	if summary.NullableFacts != 1 {
		t.Errorf("NullableFacts = %d, want 1", summary.NullableFacts)
	}

	if summary.OptionFacts != 1 {
		t.Errorf("OptionFacts = %d, want 1", summary.OptionFacts)
	}

	if summary.ResultFacts != 1 {
		t.Errorf("ResultFacts = %d, want 1", summary.ResultFacts)
	}

	if summary.GuardBlockFacts != 1 {
		t.Errorf("GuardBlockFacts = %d, want 1", summary.GuardBlockFacts)
	}

	if summary.GuardReturnFacts != 1 {
		t.Errorf("GuardReturnFacts = %d, want 1", summary.GuardReturnFacts)
	}

	if summary.SafeAccessFacts != 1 {
		t.Errorf("SafeAccessFacts = %d, want 1", summary.SafeAccessFacts)
	}

	if summary.UsesFacts != 1 {
		t.Errorf("UsesFacts = %d, want 1", summary.UsesFacts)
	}

	if summary.CallArgFacts != 1 {
		t.Errorf("CallArgFacts = %d, want 1", summary.CallArgFacts)
	}
}

func TestCartographer_SupportedLanguages(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	langs := c.SupportedLanguages()
	expected := []string{"go", "python", "typescript", "javascript", "rust"}

	if len(langs) != len(expected) {
		t.Errorf("SupportedLanguages() returned %d languages, want %d", len(langs), len(expected))
	}

	for _, want := range expected {
		found := false
		for _, got := range langs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %q in supported list", want)
		}
	}
}

func TestCartographer_IsLanguageSupported(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	tests := []struct {
		path string
		want bool
	}{
		{"foo.go", true},
		{"bar.py", true},
		{"baz.ts", true},
		{"qux.js", true},
		{"quux.rs", true},
		{"corge.txt", false},
		{"grault.md", false},
		{"garply.java", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := c.IsLanguageSupported(tt.path)
			if got != tt.want {
				t.Errorf("IsLanguageSupported(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
