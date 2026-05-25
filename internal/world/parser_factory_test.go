package world

import (
	"codenerd/internal/core"
	"errors"
	"path/filepath"
	"testing"
)

// MockCodeParser is a mock implementation of CodeParser for testing.
type MockCodeParser struct {
	lang          string
	exts          []string
	parseFunc     func(path string, content []byte) ([]CodeElement, error)
	emitFactsFunc func(elements []CodeElement) []core.Fact
}

func (m *MockCodeParser) Language() string {
	return m.lang
}

func (m *MockCodeParser) SupportedExtensions() []string {
	return m.exts
}

func (m *MockCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	if m.parseFunc != nil {
		return m.parseFunc(path, content)
	}
	return nil, nil
}

func (m *MockCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	if m.emitFactsFunc != nil {
		return m.emitFactsFunc(elements)
	}
	return nil
}

func TestParserFactory_RegisterAndGet(t *testing.T) {
	factory := NewParserFactory("/root")

	// Verify initially empty
	if parser := factory.GetParser("test.go"); parser != nil {
		t.Errorf("Expected nil, got %v", parser)
	}

	// Register a mock parser
	mockParser := &MockCodeParser{
		lang: "mock",
		exts: []string{".mock"},
	}
	factory.Register(mockParser)

	// Verify we can retrieve it
	if parser := factory.GetParser("test.mock"); parser != mockParser {
		t.Errorf("Expected mockParser, got %v", parser)
	}
	if !factory.HasParser("test.mock") {
		t.Errorf("Expected HasParser to return true")
	}

	// Verify case insensitivity and prefix handling
	if parser := factory.GetParser("test.MOCK"); parser != mockParser {
		t.Errorf("Expected mockParser for .MOCK, got %v", parser)
	}

	// Register a parser without leading dot in extension
	mockParserNoDot := &MockCodeParser{
		lang: "nodot",
		exts: []string{"nodot"},
	}
	factory.Register(mockParserNoDot)

	if parser := factory.GetParser("test.nodot"); parser != mockParserNoDot {
		t.Errorf("Expected mockParserNoDot, got %v", parser)
	}

	// Test normalizer for extensions without dot
	if ext := normalizeExtension("EXT"); ext != ".ext" {
	    t.Errorf("Expected .ext, got %s", ext)
	}
}

func TestParserFactory_Parse_Mock(t *testing.T) {
	factory := NewParserFactory("/root")

	// Parse with no parser registered
	_, err := factory.Parse("test.go", []byte("content"))
	if err == nil {
		t.Errorf("Expected error when no parser registered")
	}

	// Register a mock parser that returns elements
	expectedElements := []CodeElement{
		{Ref: "mock:test.mock:Func"},
	}
	mockParser := &MockCodeParser{
		lang: "mock",
		exts: []string{".mock"},
		parseFunc: func(path string, content []byte) ([]CodeElement, error) {
			return expectedElements, nil
		},
	}
	factory.Register(mockParser)

	// Parse with registered parser
	elements, err := factory.Parse("test.mock", []byte("content"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(elements) != len(expectedElements) || elements[0].Ref != expectedElements[0].Ref {
		t.Errorf("Expected elements %v, got %v", expectedElements, elements)
	}
}

func TestParserFactory_ParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	// Parse with no parser registered
	_, err := factory.ParseWithFacts("test.go", []byte("content"))
	if err == nil {
		t.Errorf("Expected error when no parser registered")
	}

	// Register a mock parser that returns elements
	expectedElements := []CodeElement{
		{Ref: "mock:test.mock:Func"},
	}
	mockParser := &MockCodeParser{
		lang: "mock",
		exts: []string{".mock"},
		parseFunc: func(path string, content []byte) ([]CodeElement, error) {
			return expectedElements, nil
		},
	}
	factory.Register(mockParser)

	// ParseWithFacts with registered parser
	result, err := factory.ParseWithFacts("test.mock", []byte("content"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("Expected non-nil result")
	}
	if len(result.Elements) != 1 {
		t.Errorf("Expected 1 element, got %d", len(result.Elements))
	}

	// Error case inside parse
	errorParser := &MockCodeParser{
		lang: "err",
		exts: []string{".err"},
		parseFunc: func(path string, content []byte) ([]CodeElement, error) {
			return nil, errors.New("parse error")
		},
	}
	factory.Register(errorParser)

	_, err = factory.ParseWithFacts("test.err", []byte("content"))
	if err == nil || err.Error() != "parse error" {
		t.Errorf("Expected 'parse error', got %v", err)
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
    factory := NewParserFactory("/root")

    // We need real facts to test this
    // We'll simulate facts returned by EmitLanguageFacts
    result := &ParseResult{
        Elements: []CodeElement{
            {Ref: "mock:test.mock:Func", Type: "/function"},
        },
        LanguageFacts: nil, // Add core.Fact here if needed, but ToFacts on CodeElement is enough
        Patterns: CodePatterns{
            IsGenerated: true,
            Generator: "protobuf",
        },
    }

    facts := factory.EmitAllFacts(result, "test.mock")
    if len(facts) == 0 {
        t.Errorf("Expected some facts, got 0")
    }
}

func TestParserFactory_SupportedExtensionsAndLanguages(t *testing.T) {
	factory := NewParserFactory("/root")

	mockParser1 := &MockCodeParser{
		lang: "mock1",
		exts: []string{".m1", ".mock1"},
	}
	mockParser2 := &MockCodeParser{
		lang: "mock2",
		exts: []string{".m2"},
	}

	// Register both parsers
	factory.Register(mockParser1)
	factory.Register(mockParser2)

	exts := factory.SupportedExtensions()
	if len(exts) != 3 {
		t.Errorf("Expected 3 supported extensions, got %d", len(exts))
	}

	// Check extensions exist
	extMap := make(map[string]bool)
	for _, ext := range exts {
		extMap[ext] = true
	}
	if !extMap[".m1"] || !extMap[".mock1"] || !extMap[".m2"] {
		t.Errorf("Missing expected extension in %v", exts)
	}

	langs := factory.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("Expected 2 registered languages, got %d", len(langs))
	}

	// Check languages exist
	langMap := make(map[string]bool)
	for _, lang := range langs {
		langMap[lang] = true
	}
	if !langMap["mock1"] || !langMap["mock2"] {
		t.Errorf("Missing expected language in %v", langs)
	}
}

func TestParserFactory_ProjectRoot(t *testing.T) {
	root := "/path/to/project"
	factory := NewParserFactory(root)

	if factory.ProjectRoot() != root {
		t.Errorf("Expected project root %s, got %s", root, factory.ProjectRoot())
	}

	absPath := filepath.Join(root, "dir", "file.go")
	relPath := factory.RelativePath(absPath)
	expectedRel := filepath.ToSlash(filepath.Join("dir", "file.go"))

	if relPath != expectedRel {
		t.Errorf("Expected relative path %s, got %s", expectedRel, relPath)
	}

	// Test with path not under root
	otherPath := "../other/path/file.go"
	relOther := factory.RelativePath(otherPath)
	if relOther == "" {
		t.Logf("relOther is empty")
	}
}

func TestParserFactory_DefaultFactory(t *testing.T) {
	// Simple smoke test to ensure DefaultParserFactory initializes correctly
	factory := DefaultParserFactory("/root")

	if factory == nil {
		t.Fatalf("Expected non-nil factory")
	}

	if factory.ProjectRoot() != "/root" {
		t.Errorf("Expected project root /root, got %s", factory.ProjectRoot())
	}

	// Should at least have go and mangle registered
	if !factory.HasParser("test.go") {
		t.Errorf("Expected factory to have go parser")
	}

	if !factory.HasParser("test.mg") {
		t.Errorf("Expected factory to have mangle parser")
	}
}
