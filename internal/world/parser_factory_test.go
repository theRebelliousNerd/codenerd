package world

import (
	"codenerd/internal/core"
	"fmt"
	"testing"
)

// mockCodeParser implements CodeParser for testing.
type mockCodeParser struct {
	language    string
	extensions  []string
	parseFunc   func(path string, content []byte) ([]CodeElement, error)
	factsFunc   func(elements []CodeElement) []core.Fact
}

func (m *mockCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	if m.parseFunc != nil {
		return m.parseFunc(path, content)
	}
	return nil, nil
}

func (m *mockCodeParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *mockCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	if m.factsFunc != nil {
		return m.factsFunc(elements)
	}
	return nil
}

func (m *mockCodeParser) Language() string {
	return m.language
}

func TestNewParserFactory(t *testing.T) {
	root := "/test/root"
	factory := NewParserFactory(root)
	if factory.projectRoot != root {
		t.Errorf("expected project root %s, got %s", root, factory.projectRoot)
	}
	if factory.parsers == nil {
		t.Error("parsers map not initialized")
	}
}

func TestParserFactory_Register(t *testing.T) {
	factory := NewParserFactory("/test")
	mock := &mockCodeParser{
		language:   "testlang",
		extensions: []string{".test", "TEST2"},
	}

	factory.Register(mock)

	if len(factory.parsers) != 2 {
		t.Errorf("expected 2 registered parsers, got %d", len(factory.parsers))
	}
	if p := factory.parsers[".test"]; p != mock {
		t.Errorf("expected mock parser for .test, got %v", p)
	}
	if p := factory.parsers[".test2"]; p != mock {
		t.Errorf("expected mock parser for .test2, got %v", p)
	}
}

func TestParserFactory_GetParser(t *testing.T) {
	factory := NewParserFactory("/test")
	mock := &mockCodeParser{
		extensions: []string{".go"},
	}
	factory.Register(mock)

	tests := []struct {
		name string
		path string
		want CodeParser
	}{
		{"exact match", "file.go", mock},
		{"uppercase match", "FILE.GO", mock},
		{"no match", "file.txt", nil},
		{"no extension", "file", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factory.GetParser(tt.path)
			if got != tt.want {
				t.Errorf("GetParser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParserFactory_HasParser(t *testing.T) {
	factory := NewParserFactory("/test")
	factory.Register(&mockCodeParser{
		extensions: []string{".go"},
	})

	if !factory.HasParser("file.go") {
		t.Error("expected true for file.go")
	}
	if factory.HasParser("file.txt") {
		t.Error("expected false for file.txt")
	}
}

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{"lowercase with dot", ".go", ".go"},
		{"uppercase with dot", ".GO", ".go"},
		{"lowercase no dot", "go", ".go"},
		{"uppercase no dot", "GO", ".go"},
		{"empty string", "", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeExtension(tt.ext); got != tt.want {
				t.Errorf("normalizeExtension(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestParserFactory_ParseFunc(t *testing.T) {
	factory := NewParserFactory("/test")
	mockElements := []CodeElement{{Ref: "test-ref"}}

	mock := &mockCodeParser{
		extensions: []string{".test"},
		parseFunc: func(path string, content []byte) ([]CodeElement, error) {
			if string(content) == "error" {
				return nil, fmt.Errorf("missing")
			}
			return mockElements, nil
		},
	}
	factory.Register(mock)

	// Test success
	elems, err := factory.Parse("file.test", []byte("content"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(elems) != 1 || elems[0].Ref != "test-ref" {
		t.Errorf("unexpected elements: %v", elems)
	}

	// Test parser error
	_, err = factory.Parse("file.test", []byte("error"))
	if err.Error() != "missing" {
		t.Errorf("expected missing error, got %v", err)
	}

	// Test no parser registered
	_, err = factory.Parse("file.unknown", []byte("content"))
	if err == nil || err.Error() != "no parser registered for extension: .unknown" {
		t.Errorf("expected no parser error, got %v", err)
	}
}

func TestParserFactory_ParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/test")
	mockElements := []CodeElement{{Ref: "test-ref"}}
	mockFacts := []core.Fact{{Predicate: "test_fact", Args: []interface{}{"arg"}}}

	mock := &mockCodeParser{
		extensions: []string{".test"},
		parseFunc: func(path string, content []byte) ([]CodeElement, error) {
			if string(content) == "error" {
				return nil, fmt.Errorf("missing")
			}
			return mockElements, nil
		},
		factsFunc: func(elements []CodeElement) []core.Fact {
			return mockFacts
		},
	}
	factory.Register(mock)

	// Test success
	res, err := factory.ParseWithFacts("file.test", []byte("content"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Elements) != 1 || res.Elements[0].Ref != "test-ref" {
		t.Errorf("unexpected elements: %v", res.Elements)
	}
	if len(res.LanguageFacts) != 1 || res.LanguageFacts[0].Predicate != "test_fact" {
		t.Errorf("unexpected language facts: %v", res.LanguageFacts)
	}

	// Test parser error
	_, err = factory.ParseWithFacts("file.test", []byte("error"))
	if err.Error() != "missing" {
		t.Errorf("expected missing error, got %v", err)
	}

	// Test no parser registered
	_, err = factory.ParseWithFacts("file.unknown", []byte("content"))
	if err == nil || err.Error() != "no parser registered for extension: .unknown" {
		t.Errorf("expected no parser error, got %v", err)
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/test")

	result := &ParseResult{
		Elements: []CodeElement{
			{Ref: "elem1", Type: "/function"},
		},
		LanguageFacts: []core.Fact{
			{Predicate: "lang_fact", Args: []interface{}{"arg"}},
		},
		Patterns: CodePatterns{
			HasCGo: true,
		},
	}

	facts := factory.EmitAllFacts(result, "file.go")

	// 1 code_element fact + 1 lang_fact + 1 cgo pattern fact
	if len(facts) != 5 {
		t.Errorf("expected 5 facts, got %d", len(facts))
	}

	hasCodeElemFact := false
	hasLangFact := false
	hasPatternFact := false

	for _, f := range facts {
		switch f.Predicate {
		case "code_element":
			hasCodeElemFact = true
		case "lang_fact":
			hasLangFact = true
		case "cgo_code":
			hasPatternFact = true
		}
	}

	if !hasCodeElemFact {
		t.Error("missing code_element fact")
	}
	if !hasLangFact {
		t.Error("missing lang_fact")
	}
	if !hasPatternFact {
		t.Error("missing cgo_code fact")
	}
}

func TestParserFactory_SupportedExtensions(t *testing.T) {
	factory := NewParserFactory("/test")
	factory.Register(&mockCodeParser{extensions: []string{".a", ".b"}})
	factory.Register(&mockCodeParser{extensions: []string{".c"}})

	exts := factory.SupportedExtensions()
	if len(exts) != 3 {
		t.Errorf("expected 3 extensions, got %d", len(exts))
	}

	hasExt := func(e string) bool {
		for _, x := range exts {
			if x == e {
				return true
			}
		}
		return false
	}

	if !hasExt(".a") || !hasExt(".b") || !hasExt(".c") {
		t.Errorf("missing expected extensions, got %v", exts)
	}
}

func TestParserFactory_RegisteredLanguages(t *testing.T) {
	factory := NewParserFactory("/test")
	factory.Register(&mockCodeParser{language: "lang1", extensions: []string{".a"}})
	factory.Register(&mockCodeParser{language: "lang1", extensions: []string{".b"}})
	factory.Register(&mockCodeParser{language: "lang2", extensions: []string{".c"}})

	langs := factory.RegisteredLanguages()
	if len(langs) != 2 {
		t.Errorf("expected 2 unique languages, got %d", len(langs))
	}

	hasLang := func(l string) bool {
		for _, x := range langs {
			if x == l {
				return true
			}
		}
		return false
	}

	if !hasLang("lang1") || !hasLang("lang2") {
		t.Errorf("missing expected languages, got %v", langs)
	}
}

func TestParserFactory_ProjectRoot(t *testing.T) {
	root := "/my/project"
	factory := NewParserFactory(root)
	if factory.ProjectRoot() != root {
		t.Errorf("expected %s, got %s", root, factory.ProjectRoot())
	}
}

func TestParserFactory_RelativePath(t *testing.T) {
	factory := NewParserFactory("/project/root")

	tests := []struct {
		name    string
		absPath string
		want    string
	}{
		{"inside root", "/project/root/src/main.go", "src/main.go"},
		{"outside root", "/other/dir/file.go", "../../other/dir/file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factory.RelativePath(tt.absPath)
			if got != tt.want {
				t.Errorf("RelativePath(%q) = %q, want %q", tt.absPath, got, tt.want)
			}
		})
	}
}

func TestDefaultParserFactory(t *testing.T) {
	factory := DefaultParserFactory("/project")

	if factory.projectRoot != "/project" {
		t.Errorf("expected /project, got %s", factory.projectRoot)
	}

	// Should have registered several built-in parsers
	if len(factory.parsers) == 0 {
		t.Error("expected default parsers to be registered, got 0")
	}

	// Verify standard extensions are present
	expectedExts := []string{".go", ".py", ".ts", ".rs"}
	for _, ext := range expectedExts {
		if !factory.HasParser("file" + ext) {
			t.Errorf("expected parser for %s", ext)
		}
	}
}
