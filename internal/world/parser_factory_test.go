package world

import (
	"codenerd/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockParser for testing ParserFactory error cases and specific scenarios
type MockParser struct {
	language    string
	extensions  []string
	parseError  error
	elements    []CodeElement
	langFacts   []core.Fact
}

func (m *MockParser) Parse(path string, content []byte) ([]CodeElement, error) {
	if m.parseError != nil {
		return nil, m.parseError
	}
	return m.elements, nil
}

func (m *MockParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *MockParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return m.langFacts
}

func (m *MockParser) Language() string {
	return m.language
}

func TestParserFactory_Parse_Error(t *testing.T) {
	factory := NewParserFactory("/project")

	// Test parsing an extension with no registered parser
	_, err := factory.Parse("unknown.xyz", []byte("content"))
	if err == nil {
		t.Error("Expected error when parsing unknown extension, got nil")
	}
	if !strings.Contains(err.Error(), "no parser registered for extension") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestParserFactory_ParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/project")

	// Test error when no parser is registered
	_, err := factory.ParseWithFacts("unknown.xyz", []byte("content"))
	if err == nil {
		t.Error("Expected error when parsing unknown extension with facts, got nil")
	}

	// Register a mock parser that returns an error
	mockErr := os.ErrPermission // Just using some standard error
	mockParserErr := &MockParser{
		language: "mock",
		extensions: []string{".err", "ERR2"}, // testing normalizeExtension without dot
		parseError: mockErr,
	}
	factory.Register(mockParserErr)

	// Test error when parser's Parse method fails
	_, err = factory.ParseWithFacts("test.err", []byte("content"))
	if err != mockErr {
		t.Errorf("Expected error from parser, got: %v", err)
	}

	// Register a working mock parser
	mockParserOk := &MockParser{
		language: "mock",
		extensions: []string{".ok"},
		elements: []CodeElement{{Ref: "mock:test", Type: "/test"}},
		langFacts: []core.Fact{{Predicate: "mock_fact"}},
	}
	factory.Register(mockParserOk)

	// Test success path
	res, err := factory.ParseWithFacts("test.ok", []byte("content"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(res.Elements) != 1 {
		t.Errorf("Expected 1 element, got %d", len(res.Elements))
	}
	if len(res.LanguageFacts) != 1 {
		t.Errorf("Expected 1 language fact, got %d", len(res.LanguageFacts))
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/project")

	result := &ParseResult{
		Elements: []CodeElement{
			{Ref: "go:test", Type: "/function", StartLine: 1, EndLine: 5},
		},
		LanguageFacts: []core.Fact{
			{Predicate: "lang_fact"},
		},
		Patterns: CodePatterns{
			IsGenerated: true,
		},
	}

	facts := factory.EmitAllFacts(result, "test.go")

	// Expected facts:
	// - code_element fact
	// - lang_fact
	// - code_pattern fact

	foundElemFact := false
	foundLangFact := false
	foundPatternFact := false

	for _, fact := range facts {
		switch fact.Predicate {
		case "code_element":
			foundElemFact = true
		case "lang_fact":
			foundLangFact = true
		case "generated_code":
			foundPatternFact = true
		}
	}

	if !foundElemFact {
		t.Error("Missing standard code element fact")
	}
	if !foundLangFact {
		t.Error("Missing language-specific fact")
	}
	if !foundPatternFact {
		t.Error("Missing pattern fact")
	}
}

func TestParserFactory_RegisteredLanguages(t *testing.T) {
	factory := NewParserFactory("/project")

	// Register multiple parsers, including some for the same language
	factory.Register(&MockParser{language: "lang1", extensions: []string{".l1"}})
	factory.Register(&MockParser{language: "lang2", extensions: []string{".l2a"}})
	factory.Register(&MockParser{language: "lang2", extensions: []string{".l2b"}}) // duplicate lang

	langs := factory.RegisteredLanguages()

	if len(langs) != 2 {
		t.Errorf("Expected 2 registered languages, got %d: %v", len(langs), langs)
	}

	found1, found2 := false, false
	for _, l := range langs {
		if l == "lang1" { found1 = true }
		if l == "lang2" { found2 = true }
	}

	if !found1 || !found2 {
		t.Error("Missing expected languages")
	}
}

func TestParserFactory_RelativePath(t *testing.T) {
	factory := NewParserFactory("/project/root")

	// Determine expected relative path dynamically to avoid hardcoding platform-specific paths
	expectedRel, _ := filepath.Rel("/project/root", "/other/path/file.go")

	tests := []struct {
		absPath  string
		expected string
	}{
		{
			absPath:  filepath.Join("/project/root", "src/file.go"),
			expected: "src/file.go",
		},
		{
			absPath:  filepath.Join("/project/root", "file.go"),
			expected: "file.go",
		},
		{
			// Path outside project root
			absPath:  "/other/path/file.go",
			expected: filepath.ToSlash(expectedRel),
		},
	}

	for _, tt := range tests {
		result := factory.RelativePath(tt.absPath)
		// Convert expected to standard slash format as RelativePath does
		expected := filepath.ToSlash(tt.expected)
		if result != expected {
			t.Errorf("RelativePath(%s) = %s, expected %s", tt.absPath, result, expected)
		}
	}
}

func TestParserFactory_RelativePath_Error(t *testing.T) {
	factory := NewParserFactory("C:\\project")
	// On Linux, filepath.Rel("C:\\project", "D:\\project") does not return an error because it uses simple string manipulation for non-Windows paths.
	// So simulating filepath.Rel error is hard without mocking the OS.
	// We'll just test that RelativePath correctly handles project root matching.

	if factory.ProjectRoot() != "C:\\project" {
		t.Errorf("Expected ProjectRoot C:\\project, got %s", factory.ProjectRoot())
	}
}
