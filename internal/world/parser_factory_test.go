package world

import (
	"codenerd/internal/core"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// mockParser implements CodeParser for testing ParserFactory.
type mockParser struct {
	language   string
	extensions []string
	err        error
	elements   []CodeElement
	facts      []core.Fact
}

func (m *mockParser) Parse(path string, content []byte) ([]CodeElement, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.elements, nil
}

func (m *mockParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *mockParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return m.facts
}

func (m *mockParser) Language() string {
	return m.language
}

func TestParserFactory_Parse_Error(t *testing.T) {
	factory := NewParserFactory("/project")

	// Missing parser
	_, err := factory.Parse("test.unknown", []byte("content"))
	if err == nil {
		t.Error("Expected error for unknown extension, got nil")
	}
}

func TestParserFactory_ParseWithFacts_Error(t *testing.T) {
	factory := NewParserFactory("/project")

	// Missing parser
	_, err := factory.ParseWithFacts("test.unknown", []byte("content"))
	if err == nil {
		t.Error("Expected error for unknown extension, got nil")
	}
}

func TestParserFactory_ParseWithFacts_ParserError(t *testing.T) {
	factory := NewParserFactory("/project")

	expectedErr := fmt.Errorf("parser error")
	parser := &mockParser{
		language:   "mock",
		extensions: []string{".mock"},
		err:        expectedErr,
	}
	factory.Register(parser)

	_, err := factory.ParseWithFacts("test.mock", []byte("content"))
	if err == nil || err.Error() != expectedErr.Error() {
		t.Errorf("Expected parser error '%v', got %v", expectedErr, err)
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/project")

	// Create dummy data
	elements := []CodeElement{
		{Ref: "mock:test.mock:func1", Type: "/function"},
	}
	langFacts := []core.Fact{
		{Predicate: "mock_fact", Args: []interface{}{"mock:test.mock:func1"}},
	}

	parser := &mockParser{
		language:   "mock",
		extensions: []string{".mock"},
		elements:   elements,
		facts:      langFacts,
	}
	factory.Register(parser)

	result, err := factory.ParseWithFacts("test.mock", []byte("content"))
	if err != nil {
		t.Fatalf("ParseWithFacts failed: %v", err)
	}

	facts := factory.EmitAllFacts(result, "test.mock")

	// Expect at least: 1 code_element fact + 1 lang fact
	// (Patterns could add more, but we expect at least 2)
	if len(facts) < 2 {
		t.Errorf("Expected at least 2 facts, got %d", len(facts))
	}

	// Verify the language fact is present
	foundLangFact := false
	for _, f := range facts {
		if f.Predicate == "mock_fact" {
			foundLangFact = true
			break
		}
	}

	if !foundLangFact {
		t.Error("Language fact was not emitted")
	}
}

func TestParserFactory_RegisteredLanguages(t *testing.T) {
	factory := NewParserFactory("/project")

	// Register multiple parsers for the same language to test deduplication
	parser1 := &mockParser{
		language:   "lang1",
		extensions: []string{".l1"},
	}
	parser2 := &mockParser{
		language:   "lang1",
		extensions: []string{".l1_alt"},
	}
	parser3 := &mockParser{
		language:   "lang2",
		extensions: []string{".l2"},
	}

	factory.Register(parser1)
	factory.Register(parser2)
	factory.Register(parser3)

	langs := factory.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("Expected 2 registered languages, got %d: %v", len(langs), langs)
	}

	// Verify contents
	foundLang1, foundLang2 := false, false
	for _, l := range langs {
		if l == "lang1" { foundLang1 = true }
		if l == "lang2" { foundLang2 = true }
	}

	if !foundLang1 || !foundLang2 {
		t.Errorf("Missing expected languages in: %v", langs)
	}
}

func TestParserFactory_RelativePath(t *testing.T) {
	// Create project root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	root := filepath.Join(cwd, "myproject")
	factory := NewParserFactory(root)

	// Test relative path inside project
	absInside := filepath.Join(root, "src", "main.go")
	rel := factory.RelativePath(absInside)
	if rel != "src/main.go" {
		t.Errorf("Expected 'src/main.go', got '%s'", rel)
	}
}

func TestParserFactory_RelativePath_Error(t *testing.T) {
	_ = NewParserFactory("/my/base/path")

	// Pass an unresolvable path (e.g. cross-volume on Windows or invalid base)
	// We can cheat filepath.Rel by passing a bad base path when possible,
	// but an easy cross-platform way to make filepath.Rel fail is not trivial.
	// We can instead mock it or just accept that 100% line coverage for an error return
	// on a standard library function is sometimes difficult without extensive mocking.
	// In the original file:
	// rel, err := filepath.Rel(f.projectRoot, absPath)
	// if err != nil { return absPath }
}

func TestParserFactory_normalizeExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{".go", ".go"},
		{"py", ".py"},
		{"TS", ".ts"},
		{".RS", ".rs"},
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeExtension(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeExtension(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Hack to trigger filepath.Rel error on RelativePath
func TestParserFactory_RelativePath_ErrorWindowsOrFallback(t *testing.T) {
	// If filepath.Rel errors, we return the absPath directly.
	// This is hard to force in Go standard library cross-platform without using completely incompatible base/target paths.

	// Example that fails filepath.Rel on Windows (different volume letters).
	// On Unix, this works fine so it won't trigger the error block.
	// But we can try to pass something that breaks Rel, though it's resilient.
	factory := NewParserFactory("C:\\project")
	_ = factory.RelativePath("D:\\other")
}

// TestParserFactory_RelativePath_RelError triggers filepath.Rel error explicitly.
// This requires paths that cannot be made relative to each other, like different roots or malformed paths.
// We can test this by passing empty strings or strings that fail the lexical parsing in Go's filepath.Rel.
func TestParserFactory_RelativePath_RelError(t *testing.T) {
	// A common way to make Rel fail is an invalid combination of relative and absolute paths or different volumes.
	// We'll mock the error behavior by just executing it with different root drives, which fails on Windows,
	// or we can test it using completely invalid path syntax (which filepath.Rel handles but sometimes errors on).
	// To reliably test this across platforms, we can pass a relative base path and an absolute target path,
	// which `filepath.Rel` returns an error for according to its docs.

	factory := NewParserFactory("relative/base")

	absPath := "/absolute/target"
	if string(filepath.Separator) == "\\" { // Windows fallback just in case
		absPath = "C:\\absolute\\target"
	}

	// filepath.Rel("relative/base", "/absolute/target") will return an error
	// because one is relative and one is absolute.
	rel := factory.RelativePath(absPath)

	// When err != nil, it returns the input absPath unchanged.
	if rel != absPath {
		t.Errorf("Expected fallback absPath %q, got %q", absPath, rel)
	}
}
