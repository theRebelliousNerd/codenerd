package world

import (
	"codenerd/internal/core"
	"path/filepath"
	"testing"
)

// mockParser is a simple mock of the CodeParser interface for testing.
type mockParser struct {
	lang       string
	extensions []string
}

func (m *mockParser) Parse(path string, content []byte) ([]CodeElement, error) {
	return []CodeElement{
		{Ref: "mock:test", Type: ElementFunction},
	}, nil
}

func (m *mockParser) SupportedExtensions() []string {
	return m.extensions
}

func (m *mockParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	return []core.Fact{
		{Predicate: "mock_fact", Args: []interface{}{"test"}},
	}
}

func (m *mockParser) Language() string {
	return m.lang
}

func TestParserFactory_RegisterAndGet(t *testing.T) {
	factory := NewParserFactory("/project/root")

	parser := &mockParser{
		lang:       "mock",
		extensions: []string{".mock", ".tst"},
	}

	factory.Register(parser)

	tests := []struct {
		name     string
		path     string
		expected CodeParser
	}{
		{"exact extension", "file.mock", parser},
		{"uppercase extension", "FILE.MOCK", parser},
		{"alternate extension", "path/to/file.tst", parser},
		{"unregistered extension", "file.txt", nil},
		{"no extension", "file", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factory.GetParser(tt.path)
			if got != tt.expected {
				t.Errorf("GetParser(%q) = %v, want %v", tt.path, got, tt.expected)
			}

			has := factory.HasParser(tt.path)
			expectedHas := tt.expected != nil
			if has != expectedHas {
				t.Errorf("HasParser(%q) = %v, want %v", tt.path, has, expectedHas)
			}
		})
	}
}

func TestParserFactory_Parse_Basic(t *testing.T) {
	factory := NewParserFactory("/project/root")
	parser := &mockParser{
		lang:       "mock",
		extensions: []string{".mock"},
	}
	factory.Register(parser)

	t.Run("registered extension", func(t *testing.T) {
		elements, err := factory.Parse("test.mock", []byte("content"))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if len(elements) != 1 || elements[0].Ref != "mock:test" {
			t.Errorf("Parse() returned unexpected elements: %v", elements)
		}
	})

	t.Run("unregistered extension", func(t *testing.T) {
		_, err := factory.Parse("test.txt", []byte("content"))
		if err == nil {
			t.Fatal("Parse() expected error for unregistered extension, got nil")
		}
	})
}

func TestParserFactory_ParseWithFacts(t *testing.T) {
	factory := NewParserFactory("/project/root")
	parser := &mockParser{
		lang:       "mock",
		extensions: []string{".mock"},
	}
	factory.Register(parser)

	t.Run("registered extension", func(t *testing.T) {
		result, err := factory.ParseWithFacts("test.mock", []byte("content"))
		if err != nil {
			t.Fatalf("ParseWithFacts() error = %v", err)
		}
		if len(result.Elements) != 1 || result.Elements[0].Ref != "mock:test" {
			t.Errorf("Unexpected elements: %v", result.Elements)
		}
		if len(result.LanguageFacts) != 1 || result.LanguageFacts[0].Predicate != "mock_fact" {
			t.Errorf("Unexpected language facts: %v", result.LanguageFacts)
		}
		if result.Patterns.IsGenerated {
			t.Errorf("Unexpected pattern generated")
		}
	})

	t.Run("unregistered extension", func(t *testing.T) {
		_, err := factory.ParseWithFacts("test.txt", []byte("content"))
		if err == nil {
			t.Fatal("ParseWithFacts() expected error for unregistered extension, got nil")
		}
	})
}

func TestParserFactory_Metadata(t *testing.T) {
	factory := NewParserFactory("/project/root")

	factory.Register(&mockParser{lang: "mock1", extensions: []string{".m1", ".mock"}})
	factory.Register(&mockParser{lang: "mock2", extensions: []string{".m2"}})

	t.Run("SupportedExtensions", func(t *testing.T) {
		exts := factory.SupportedExtensions()
		expected := []string{".m1", ".mock", ".m2"}

		if len(exts) != len(expected) {
			t.Errorf("SupportedExtensions() returned %d items, want %d", len(exts), len(expected))
		}

		for _, e := range expected {
			found := false
			for _, got := range exts {
				if got == e {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("SupportedExtensions() missing %q", e)
			}
		}
	})

	t.Run("RegisteredLanguages", func(t *testing.T) {
		langs := factory.RegisteredLanguages()
		expected := []string{"mock1", "mock2"}

		if len(langs) != len(expected) {
			t.Errorf("RegisteredLanguages() returned %d items, want %d", len(langs), len(expected))
		}

		for _, l := range expected {
			found := false
			for _, got := range langs {
				if got == l {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("RegisteredLanguages() missing %q", l)
			}
		}
	})
}

func TestParserFactory_Paths(t *testing.T) {
	factory := NewParserFactory("/project/root")

	if factory.ProjectRoot() != "/project/root" {
		t.Errorf("ProjectRoot() = %q, want %q", factory.ProjectRoot(), "/project/root")
	}

	tests := []struct {
		absPath  string
		expected string
	}{
		{filepath.Join("/project/root", "dir", "file.go"), "dir/file.go"},
		{filepath.Join("/project/root", "file.go"), "file.go"},
	}

	for _, tt := range tests {
		rel := factory.RelativePath(tt.absPath)
		// We use filepath.ToSlash in RelativePath, so we expect forward slashes
		expected := filepath.ToSlash(tt.expected)
		// the fallback test case logic might differ on windows vs linux for cross-drive,
		// but for simple paths it should just be the relative path
		if rel != expected {
			t.Errorf("RelativePath(%q) = %q, want %q", tt.absPath, rel, expected)
		}
	}

	// Test fallback path outside root
	fallbackPath := "/other/path/file.go"
	rel := factory.RelativePath(fallbackPath)
	expectedFallback, err := filepath.Rel("/project/root", fallbackPath)
	if err == nil {
		expectedFallback = filepath.ToSlash(expectedFallback)
		if rel != expectedFallback {
			t.Errorf("RelativePath(%q) = %q, want %q", fallbackPath, rel, expectedFallback)
		}
	} else {
		// Should return the original if error occurs in filepath.Rel (e.g., cross-drive on Windows)
		if rel != filepath.ToSlash(fallbackPath) {
			t.Errorf("RelativePath(%q) = %q, want %q", fallbackPath, rel, filepath.ToSlash(fallbackPath))
		}
	}
}

func TestNormalizeExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{".go", ".go"},
		{"go", ".go"},
		{".GO", ".go"},
		{"GO", ".go"},
		{"", "."},
	}

	for _, tt := range tests {
		got := normalizeExtension(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeExtension(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParserFactory_EmitAllFacts(t *testing.T) {
	factory := NewParserFactory("/project/root")

	// Create a dummy parse result
	result := &ParseResult{
		Elements: []CodeElement{
			{Ref: "go:test", Type: ElementFunction, StartLine: 1, EndLine: 5},
		},
		LanguageFacts: []core.Fact{
			{Predicate: "lang_fact", Args: []interface{}{"test"}},
		},
		Patterns: CodePatterns{
			IsGenerated: true,
			Generator:   "protobuf",
		},
	}

	facts := factory.EmitAllFacts(result, "test.go")

	// Verify we got facts from all 3 sources: CodeElements, LanguageFacts, and Patterns
	var hasElementFact, hasLangFact, hasPatternFact bool

	for _, f := range facts {
		if f.Predicate == "code_element" {
			hasElementFact = true
		} else if f.Predicate == "lang_fact" {
			hasLangFact = true
		} else if f.Predicate == "generated_code" {
			hasPatternFact = true
		}
	}

	if !hasElementFact {
		t.Error("EmitAllFacts did not include CodeElement facts")
	}
	if !hasLangFact {
		t.Error("EmitAllFacts did not include LanguageFacts")
	}
	if !hasPatternFact {
		t.Error("EmitAllFacts did not include Pattern facts")
	}
}

func TestDefaultParserFactory(t *testing.T) {
	factory := DefaultParserFactory("/project/root")

	// Ensure the built-in parsers are registered
	exts := []string{".go", ".py", ".ts", ".rs", ".mg"}
	for _, ext := range exts {
		if !factory.HasParser(ext) {
			t.Errorf("DefaultParserFactory missing parser for %q", ext)
		}
	}
}
