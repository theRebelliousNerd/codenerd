package world

import (
	"codenerd/internal/core"
	"path/filepath"
	"reflect"
	"testing"
)

// mockCodeParser implements CodeParser for testing.
type mockCodeParser struct {
	lang       string
	extensions []string
	elements   []CodeElement
	facts      []core.Fact
	err        error
	parseCount int
}

func (m *mockCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	m.parseCount++
	return m.elements, m.err
}

func (m *mockCodeParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *mockCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return m.facts
}

func (m *mockCodeParser) Language() string {
	return m.lang
}

func TestNewParserFactory(t *testing.T) {
	root := "/test/root"
	factory := NewParserFactory(root)

	if factory.projectRoot != root {
		t.Errorf("expected projectRoot %q, got %q", root, factory.projectRoot)
	}
	if factory.parsers == nil {
		t.Error("expected parsers map to be initialized")
	}
	if len(factory.parsers) != 0 {
		t.Errorf("expected empty parsers map, got %d entries", len(factory.parsers))
	}
}

func TestDefaultParserFactory(t *testing.T) {
	root := "/test/root"
	factory := DefaultParserFactory(root)

	if factory.projectRoot != root {
		t.Errorf("expected projectRoot %q, got %q", root, factory.projectRoot)
	}
	if factory.parsers == nil {
		t.Error("expected parsers map to be initialized")
	}

	// Default factory should register multiple standard parsers
	if len(factory.parsers) == 0 {
		t.Error("expected default parsers to be registered")
	}

	// Verify some expected parsers exist
	expectedExts := []string{".go", ".py", ".ts", ".rs", ".mg"}
	for _, ext := range expectedExts {
		if !factory.HasParser("file" + ext) {
			t.Errorf("expected default factory to support %s", ext)
		}
	}
}
func TestParserFactoryRegistration(t *testing.T) {
	factory := NewParserFactory("/root")

	// Create a mock parser supporting .tst and .test
	mock := &mockCodeParser{
		lang:       "testlang",
		extensions: []string{".tst", "test"}, // "test" should be normalized to ".test"
	}

	factory.Register(mock)

	// Check SupportedExtensions
	exts := factory.SupportedExtensions()
	if len(exts) != 2 {
		t.Errorf("expected 2 supported extensions, got %d", len(exts))
	}

	// HasParser tests
	if !factory.HasParser("file.tst") {
		t.Error("expected HasParser to return true for .tst")
	}
	if !factory.HasParser("file.test") {
		t.Error("expected HasParser to return true for .test")
	}
	if !factory.HasParser("file.TEST") { // Should be case-insensitive
		t.Error("expected HasParser to return true for .TEST")
	}
	if factory.HasParser("file.unknown") {
		t.Error("expected HasParser to return false for .unknown")
	}

	// GetParser tests
	p1 := factory.GetParser("file.tst")
	if p1 != mock {
		t.Errorf("GetParser(.tst): expected mock parser, got %v", p1)
	}

	p2 := factory.GetParser("file.test")
	if p2 != mock {
		t.Errorf("GetParser(.test): expected mock parser, got %v", p2)
	}

	p3 := factory.GetParser("file.unknown")
	if p3 != nil {
		t.Errorf("GetParser(.unknown): expected nil, got %v", p3)
	}
}
func TestParserFactoryParse(t *testing.T) {
	factory := NewParserFactory("/root")

	expectedElems := []CodeElement{
		{Ref: "test:ref", Type: ElementFunction},
	}

	mock := &mockCodeParser{
		extensions: []string{".tst"},
		elements:   expectedElems,
	}

	factory.Register(mock)

	// Test successful parse
	elems, err := factory.Parse("file.tst", []byte("content"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(elems, expectedElems) {
		t.Errorf("expected elements %v, got %v", expectedElems, elems)
	}
	if mock.parseCount != 1 {
		t.Errorf("expected parseCount 1, got %d", mock.parseCount)
	}

	// Test missing parser
	_, err = factory.Parse("file.unknown", []byte("content"))
	if err == nil {
		t.Error("expected error for unregistered extension")
	}
}

func TestParserFactoryParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	expectedElems := []CodeElement{
		{Ref: "test:ref", Type: ElementFunction, StartLine: 1, EndLine: 2},
	}
	expectedFacts := []core.Fact{
		{Predicate: "test_fact", Args: []interface{}{"arg1"}},
	}

	mock := &mockCodeParser{
		extensions: []string{".tst"},
		elements:   expectedElems,
		facts:      expectedFacts,
	}

	factory.Register(mock)

	// Test successful parse
	result, err := factory.ParseWithFacts("file.tst", []byte("content"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !reflect.DeepEqual(result.Elements, expectedElems) {
		t.Errorf("expected elements %v, got %v", expectedElems, result.Elements)
	}
	if !reflect.DeepEqual(result.LanguageFacts, expectedFacts) {
		t.Errorf("expected language facts %v, got %v", expectedFacts, result.LanguageFacts)
	}

	// Test missing parser
	_, err = factory.ParseWithFacts("file.unknown", []byte("content"))
	if err == nil {
		t.Error("expected error for unregistered extension")
	}
}
func TestParserFactoryEmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	elem := CodeElement{
		Ref:       "test:file.go:Func",
		Type:      ElementFunction,
		StartLine: 1,
		EndLine:   10,
	}

	langFact := core.Fact{
		Predicate: "test_lang_fact",
		Args:      []interface{}{"arg1"},
	}

	result := &ParseResult{
		Elements:      []CodeElement{elem},
		LanguageFacts: []core.Fact{langFact},
		Patterns: CodePatterns{
			IsGenerated: true,
			Generator:   "testgen",
		},
	}

	facts := factory.EmitAllFacts(result, "file.go")

	// Expected facts:
	// 1. From elem.ToFacts(): code_element("test:file.go:Func", "file.go", "/function", 1, 10, "")
	// 2. From LanguageFacts: test_lang_fact("arg1")
	// 3. From Patterns: generated_code("file.go", "/testgen", "")

	if len(facts) < 3 {
		t.Fatalf("expected at least 3 facts, got %d", len(facts))
	}

	var foundElemFact, foundLangFact, foundPatternFact bool
	for _, f := range facts {
		switch f.Predicate {
		case "code_element":
			foundElemFact = true
		case "test_lang_fact":
			foundLangFact = true
		case "generated_code":
			foundPatternFact = true
		}
	}

	if !foundElemFact {
		t.Error("missing code_element fact")
	}
	if !foundLangFact {
		t.Error("missing language fact")
	}
	if !foundPatternFact {
		t.Error("missing pattern fact")
	}
}
func TestParserFactoryHelpers(t *testing.T) {
	factory := NewParserFactory("/root")

	if factory.ProjectRoot() != "/root" {
		t.Errorf("expected ProjectRoot '/root', got %q", factory.ProjectRoot())
	}

	// Test RelativePath
	rel := factory.RelativePath("/root/subdir/file.txt")
	if rel != "subdir/file.txt" {
		t.Errorf("expected RelativePath 'subdir/file.txt', got %q", rel)
	}

	relOutside := factory.RelativePath("/other/file.txt")
	// filepath.Rel might return "../other/file.txt" or an error depending on OS.
	// If it succeeds, it returns a relative path; if it fails, it returns absolute.
	expectedOutside, err := filepath.Rel("/root", "/other/file.txt")
	if err != nil {
		expectedOutside = "/other/file.txt"
	} else {
		expectedOutside = filepath.ToSlash(expectedOutside)
	}

	if relOutside != expectedOutside {
		t.Errorf("expected RelativePath %q, got %q", expectedOutside, relOutside)
	}

	// Test SupportedExtensions and RegisteredLanguages
	mock1 := &mockCodeParser{
		lang:       "lang1",
		extensions: []string{".l1", ".lang1"},
	}
	mock2 := &mockCodeParser{
		lang:       "lang2",
		extensions: []string{".l2"},
	}

	factory.Register(mock1)
	factory.Register(mock2)

	exts := factory.SupportedExtensions()
	if len(exts) != 3 {
		t.Errorf("expected 3 supported extensions, got %d", len(exts))
	}

	// Ensure all extensions are present
	extMap := make(map[string]bool)
	for _, e := range exts {
		extMap[e] = true
	}
	for _, expected := range []string{".l1", ".lang1", ".l2"} {
		if !extMap[expected] {
			t.Errorf("missing extension %s", expected)
		}
	}

	langs := factory.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("expected 2 registered languages, got %d", len(langs))
	}

	langMap := make(map[string]bool)
	for _, l := range langs {
		langMap[l] = true
	}
	for _, expected := range []string{"lang1", "lang2"} {
		if !langMap[expected] {
			t.Errorf("missing language %s", expected)
		}
	}
}
