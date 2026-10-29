package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCartographerMapFile(t *testing.T) {
	dir := t.TempDir()
	src := "package sample\n\ntype Widget struct{ N int }\n\nfunc (w Widget) Bump() int { return w.N + 1 }\n\nfunc Hello(name string) string { return \"hi \" + name }\n"
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewCartographer()
	facts, err := c.MapFile(path)
	if err != nil {
		t.Fatalf("MapFile: %v", err)
	}
	if len(facts) == 0 {
		t.Error("MapFile should produce code-element facts for a real Go file")
	}

	// A non-Go file is unsupported and yields no facts (no error).
	other := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(other, []byte("hello"), 0o644)
	if f, err := c.MapFile(other); err != nil || len(f) != 0 {
		t.Errorf("MapFile(.txt)=(%d facts,%v), want (0,nil)", len(f), err)
	}
}

func TestNewCodeElementParserWithFactory(t *testing.T) {
	factory := NewParserFactory("/test/root")
	parser := NewCodeElementParserWithFactory(factory)

	if parser == nil {
		t.Fatal("expected parser to be created")
	}
	if parser.Factory() != factory {
		t.Errorf("expected parser to have the provided factory")
	}
	if parser.projectRoot != "/test/root" {
		t.Errorf("expected project root to be '/test/root', got '%s'", parser.projectRoot)
	}
}

func TestNewCodeElementParserWithRoot(t *testing.T) {
	root := "/test/root"
	parser := NewCodeElementParserWithRoot(root)

	if parser == nil {
		t.Fatal("expected parser to be created")
	}
	factory := parser.Factory()
	if factory == nil {
		t.Fatal("expected default factory to be created")
	}
	if factory.ProjectRoot() != root {
		t.Errorf("expected factory project root to be '%s', got '%s'", root, factory.ProjectRoot())
	}
	if parser.projectRoot != root {
		t.Errorf("expected parser project root to be '%s', got '%s'", root, parser.projectRoot)
	}
}

func TestCodeElementParser_Factory(t *testing.T) {
	factory := NewParserFactory("/test/root")
	parser := NewCodeElementParserWithFactory(factory)

	if got := parser.Factory(); got != factory {
		t.Errorf("Factory() returned %v, want %v", got, factory)
	}
}

func TestCodeElementParser_ParseFile_NoFactory(t *testing.T) {
	parser := &CodeElementParser{} // no factory

	_, err := parser.ParseFile("test.go")
	if err == nil {
		t.Error("expected error when parsing without a factory")
	} else if err.Error() != "ParserFactory is required but not configured for CodeElementParser" {
		t.Errorf("unexpected error message: %v", err)
	}
}
