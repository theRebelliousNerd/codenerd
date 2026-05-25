package world

import (
	"codenerd/internal/core"
	"path/filepath"
	"testing"
)

// mockParser is a simple mock parser for testing ParserFactory.
type mockParser struct {
	language    string
	extensions  []string
	parseResult []CodeElement
	parseError  error
	factsResult []core.Fact
}

func (m *mockParser) Parse(path string, content []byte) ([]CodeElement, error) {
	return m.parseResult, m.parseError
}

func (m *mockParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *mockParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return m.factsResult
}

func (m *mockParser) Language() string {
	return m.language
}

func TestParserFactory_RegistrationAndRetrieval(t *testing.T) {
	factory := NewParserFactory("/project/root")
	if factory.ProjectRoot() != "/project/root" {
		t.Errorf("Expected project root '/project/root', got '%s'", factory.ProjectRoot())
	}

	mockP1 := &mockParser{
		language:   "lang1",
		extensions: []string{".l1", "L1"}, // should normalize "L1" to ".l1"
	}
	mockP2 := &mockParser{
		language:   "lang2",
		extensions: []string{".l2"},
	}

	factory.Register(mockP1)
	factory.Register(mockP2)

	// Test HasParser
	if !factory.HasParser("file.l1") {
		t.Error("Expected true for 'file.l1'")
	}
	if !factory.HasParser("file.L1") {
		t.Error("Expected true for 'file.L1', Should handle uppercase")
	}
	if !factory.HasParser("file.l2") {
		t.Error("Expected true for 'file.l2'")
	}
	if factory.HasParser("file.unknown") {
		t.Error("Expected false for 'file.unknown'")
	}

	// Test GetParser
	p1 := factory.GetParser("file.l1")
	if p1 != mockP1 {
		t.Errorf("Expected mockP1, got %v", p1)
	}

	p2 := factory.GetParser("TEST/FILE.L2")
	if p2 != mockP2 {
		t.Errorf("Expected mockP2, got %v", p2)
	}

	pNil := factory.GetParser("file.txt")
	if pNil != nil {
		t.Errorf("Expected nil, got %v", pNil)
	}

	// Test SupportedExtensions
	exts := factory.SupportedExtensions()
	if len(exts) != 2 {
		t.Errorf("Expected 2 extensions, got %v", len(exts))
	}

	// Test RegisteredLanguages
	langs := factory.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("Expected 2 languages, got %v", len(langs))
	}
}

func TestParserFactory_ParseAndFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	elements := []CodeElement{
		{Name: "Elem1", Type: "type1", Ref: "ref1"},
	}
	facts := []core.Fact{
		{Predicate: "test_fact", Args: []interface{}{"arg1"}},
	}

	mockP := &mockParser{
		language:    "mocklang",
		extensions:  []string{".mock"},
		parseResult: elements,
		factsResult: facts,
	}

	factory.Register(mockP)

	// Test Parse error when no parser
	_, err := factory.Parse("file.txt", nil)
	if err == nil {
		t.Error("Expected error when parsing with unknown extension")
	}

	// Test Parse success
	parsedElems, err := factory.Parse("file.mock", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(parsedElems) != 1 || parsedElems[0].Name != "Elem1" {
		t.Errorf("Unexpected parse result: %v", parsedElems)
	}

	// Test ParseWithFacts error
	_, err = factory.ParseWithFacts("file.txt", nil)
	if err == nil {
		t.Error("Expected error when parsing with facts for unknown extension")
	}

	// Test ParseWithFacts success
	result, err := factory.ParseWithFacts("file.mock", []byte("content"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(result.Elements) != 1 {
		t.Errorf("Expected 1 element, got %d", len(result.Elements))
	}
	if len(result.LanguageFacts) != 1 {
		t.Errorf("Expected 1 language fact, got %d", len(result.LanguageFacts))
	}

	// Test EmitAllFacts
	allFacts := factory.EmitAllFacts(result, "file.mock")
	if len(allFacts) == 0 {
		t.Error("Expected some facts to be emitted")
	}
}

func TestParserFactory_RelativePath(t *testing.T) {
	factory := NewParserFactory("/base/project")

	rel1 := factory.RelativePath("/base/project/src/main.go")
	if rel1 != filepath.ToSlash("src/main.go") {
		t.Errorf("Expected 'src/main.go', got '%s'", rel1)
	}

	rel2 := factory.RelativePath("/other/path/file.go")
	// If path is outside project root, relative path calculation might fail or return relative path navigating out.
	// `filepath.Rel` behavior: returns relative path from root to target.
	// Just test that it does not panic and returns a string.
	if rel2 == "" {
		t.Errorf("Expected a valid path, got empty string")
	}
}
