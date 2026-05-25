package world

import (
	"codenerd/internal/core"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// mockParser implements CodeParser for testing ParserFactory.
type mockParser struct {
	exts      []string
	lang      string
	elements  []CodeElement
	facts     []core.Fact
	parseErr  error
}

func (m *mockParser) Parse(path string, content []byte) ([]CodeElement, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	return m.elements, nil
}

func (m *mockParser) SupportedExtensions() []string {
	return m.exts
}

func (m *mockParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return m.facts
}

func (m *mockParser) Language() string {
	return m.lang
}

func TestNewParserFactory(t *testing.T) {
	root := "/test/root"
	factory := NewParserFactory(root)

	if factory.projectRoot != root {
		t.Errorf("expected projectRoot to be %q, got %q", root, factory.projectRoot)
	}
	if factory.parsers == nil {
		t.Errorf("expected parsers map to be initialized")
	}
}

func TestParserFactory_RegisterAndGet(t *testing.T) {
	factory := NewParserFactory("/root")

	parser := &mockParser{
		exts: []string{".go", "TXT", "NoDot"},
		lang: "testlang",
	}

	factory.Register(parser)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Existing standard extension", "file.go", true},
		{"Existing uppercase extension", "file.txt", true},
		{"Existing without dot originally", "file.nodot", true},
		{"Non-existing extension", "file.cpp", false},
		{"Empty extension", "file", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			has := factory.HasParser(tc.path)
			if has != tc.expected {
				t.Errorf("HasParser(%q) = %v; want %v", tc.path, has, tc.expected)
			}

			p := factory.GetParser(tc.path)
			if tc.expected && p == nil {
				t.Errorf("GetParser(%q) returned nil, expected parser", tc.path)
			} else if !tc.expected && p != nil {
				t.Errorf("GetParser(%q) returned parser, expected nil", tc.path)
			}
		})
	}
}

func TestParserFactory_ParseMissingParser(t *testing.T) {
	factory := NewParserFactory("/root")

	elems := []CodeElement{{Ref: "test-ref"}}
	parser := &mockParser{
		exts:     []string{".go"},
		lang:     "go",
		elements: elems,
	}
	factory.Register(parser)

	// Test missing parser
	_, err := factory.Parse("test.py", []byte("content"))
	if err == nil {
		t.Errorf("Expected error for missing parser, got nil")
	}

	// Test parse error
	errParser := &mockParser{
		exts:     []string{".err"},
		lang:     "err",
		parseErr: fmt.Errorf("parse failure"),
	}
	factory.Register(errParser)

	_, err = factory.Parse("test.err", []byte("content"))
	if err == nil || err.Error() != "parse failure" {
		t.Errorf("Expected 'parse failure' error, got %v", err)
	}
}

func TestParserFactory_ParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	elems := []CodeElement{{Ref: "test-ref"}}
	facts := []core.Fact{core.Fact{Predicate: "test_fact"}}
	parser := &mockParser{
		exts:     []string{".go"},
		lang:     "go",
		elements: elems,
		facts:    facts,
	}
	factory.Register(parser)

	// Test successful ParseWithFacts
	res, err := factory.ParseWithFacts("test.go", []byte("content"))
	if err != nil {
		t.Fatalf("ParseWithFacts failed: %v", err)
	}
	if !reflect.DeepEqual(res.Elements, elems) {
		t.Errorf("Elements = %v; want %v", res.Elements, elems)
	}
	if !reflect.DeepEqual(res.LanguageFacts, facts) {
		t.Errorf("LanguageFacts = %v; want %v", res.LanguageFacts, facts)
	}

	// Test missing parser
	_, err = factory.ParseWithFacts("test.py", []byte("content"))
	if err == nil {
		t.Errorf("Expected error for missing parser, got nil")
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/root")

	elem := CodeElement{
		Ref:  "test-ref",
		Type: "/function",
	}
	langFact := core.Fact{Predicate: "lang_fact"}

	// Create a dummy result
	res := &ParseResult{
		Elements:      []CodeElement{elem},
		LanguageFacts: []core.Fact{langFact},
		Patterns:      CodePatterns{IsGenerated: true},
	}

	facts := factory.EmitAllFacts(res, "test.go")

	// Check if all expected facts are present.
	// We expect element facts, language facts, and pattern facts.
	if len(facts) == 0 {
		t.Fatalf("EmitAllFacts returned empty slice")
	}

	hasLangFact := false
	for _, f := range facts {
		if f.Predicate == "lang_fact" {
			hasLangFact = true
		}
	}

	if !hasLangFact {
		t.Errorf("EmitAllFacts did not include language facts")
	}
}

func TestParserFactory_SupportedExtensions(t *testing.T) {
	factory := NewParserFactory("/root")

	factory.Register(&mockParser{exts: []string{".go", ".txt"}, lang: "go"})
	factory.Register(&mockParser{exts: []string{".py"}, lang: "python"})

	exts := factory.SupportedExtensions()
	sort.Strings(exts)

	expected := []string{".go", ".py", ".txt"}
	if !reflect.DeepEqual(exts, expected) {
		t.Errorf("SupportedExtensions = %v; want %v", exts, expected)
	}
}

func TestParserFactory_RegisteredLanguages(t *testing.T) {
	factory := NewParserFactory("/root")

	factory.Register(&mockParser{exts: []string{".go"}, lang: "go"})
	factory.Register(&mockParser{exts: []string{".py"}, lang: "python"})
	factory.Register(&mockParser{exts: []string{".python3"}, lang: "python"}) // duplicate lang

	langs := factory.RegisteredLanguages()
	sort.Strings(langs)

	expected := []string{"go", "python"}
	if !reflect.DeepEqual(langs, expected) {
		t.Errorf("RegisteredLanguages = %v; want %v", langs, expected)
	}
}

func TestParserFactory_ProjectRoot(t *testing.T) {
	root := "/test/root"
	factory := NewParserFactory(root)

	if factory.ProjectRoot() != root {
		t.Errorf("ProjectRoot() = %q; want %q", factory.ProjectRoot(), root)
	}
}

func TestParserFactory_RelativePath(t *testing.T) {
	factory := NewParserFactory("/test/root")

	tests := []struct {
		absPath  string
		expected string
	}{
		{"/test/root/file.go", "file.go"},
		{"/test/root/dir/file.go", "dir/file.go"},
		{"/other/path/file.go", "../../other/path/file.go"},
	}

	for _, tc := range tests {
		res := factory.RelativePath(tc.absPath)
		if res != tc.expected {
			t.Errorf("RelativePath(%q) = %q; want %q", tc.absPath, res, tc.expected)
		}
	}
}

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", ".go"},
		{"go", ".go"},
		{".TXT", ".txt"},
		{"TXT", ".txt"},
		{"", "."},
	}

	for _, tc := range tests {
		res := normalizeExtension(tc.ext)
		if res != tc.expected {
			t.Errorf("normalizeExtension(%q) = %q; want %q", tc.ext, res, tc.expected)
		}
	}
}

func TestDefaultParserFactory(t *testing.T) {
	factory := DefaultParserFactory("/root")

	if factory == nil {
		t.Fatalf("DefaultParserFactory returned nil")
	}

	expectedExts := []string{".go", ".py", ".ts", ".rs"}
	for _, ext := range expectedExts {
		if !factory.HasParser("file" + ext) {
			t.Errorf("DefaultParserFactory missing parser for %s", ext)
		}
	}
}

func TestParserFactory_ParseSuccess(t *testing.T) {
	factory := NewParserFactory("/root")

	elems := []CodeElement{{Ref: "test-ref"}}
	parser := &mockParser{
		exts:     []string{".go"},
		lang:     "go",
		elements: elems,
	}
	factory.Register(parser)

	// Test successful parse
	res, err := factory.Parse("test.go", []byte("content"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !reflect.DeepEqual(res, elems) {
		t.Errorf("Parse() = %v; want %v", res, elems)
	}
}
