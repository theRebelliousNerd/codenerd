package world

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCodeElementQueries(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:main", Type: ElementFunction, File: "a.go", StartLine: 1, EndLine: 5},
		{Ref: "struct:Foo", Type: ElementStruct, File: "a.go", StartLine: 7, EndLine: 10},
		{Ref: "method:Foo.Bar", Type: ElementMethod, Parent: "struct:Foo", File: "a.go", StartLine: 12, EndLine: 15},
	}

	if e := GetElement(elements, "fn:main"); e == nil || e.Ref != "fn:main" {
		t.Errorf("GetElement(fn:main)=%v, want the function element", e)
	}
	if GetElement(elements, "missing") != nil {
		t.Error("GetElement(missing) should be nil")
	}

	if fns := GetElementsByType(elements, ElementFunction); len(fns) != 1 || fns[0].Ref != "fn:main" {
		t.Errorf("GetElementsByType(function)=%v, want [fn:main]", fns)
	}

	methods := GetMethodsOfStruct(elements, "struct:Foo")
	if len(methods) != 1 || methods[0].Ref != "method:Foo.Bar" {
		t.Errorf("GetMethodsOfStruct(Foo)=%v, want [method:Foo.Bar]", methods)
	}
	if got := GetMethodsOfStruct(elements, "struct:Other"); len(got) != 0 {
		t.Errorf("GetMethodsOfStruct(Other)=%v, want empty", got)
	}

}

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

func TestGetMethodsOfStruct(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:main", Type: ElementFunction, File: "a.go", StartLine: 1, EndLine: 5},
		{Ref: "struct:MyStruct", Type: ElementStruct, File: "a.go", StartLine: 7, EndLine: 10},
		{Ref: "method:MyStruct.Method1", Type: ElementMethod, Parent: "struct:MyStruct", File: "a.go", StartLine: 12, EndLine: 15},
		{Ref: "method:MyStruct.Method2", Type: ElementMethod, Parent: "struct:MyStruct", File: "a.go", StartLine: 17, EndLine: 20},
		{Ref: "method:OtherStruct.Method", Type: ElementMethod, Parent: "struct:OtherStruct", File: "b.go", StartLine: 5, EndLine: 10},
		{Ref: "struct:OtherStruct", Type: ElementStruct, File: "b.go", StartLine: 1, EndLine: 4},
	}

	tests := []struct {
		name      string
		structRef string
		want      []CodeElement
	}{
		{
			name:      "Struct with multiple methods",
			structRef: "struct:MyStruct",
			want: []CodeElement{
				{Ref: "method:MyStruct.Method1", Type: ElementMethod, Parent: "struct:MyStruct", File: "a.go", StartLine: 12, EndLine: 15},
				{Ref: "method:MyStruct.Method2", Type: ElementMethod, Parent: "struct:MyStruct", File: "a.go", StartLine: 17, EndLine: 20},
			},
		},
		{
			name:      "Struct with one method",
			structRef: "struct:OtherStruct",
			want: []CodeElement{
				{Ref: "method:OtherStruct.Method", Type: ElementMethod, Parent: "struct:OtherStruct", File: "b.go", StartLine: 5, EndLine: 10},
			},
		},
		{
			name:      "Struct with no methods",
			structRef: "struct:MissingStruct",
			want:      nil,
		},
		{
			name:      "Empty elements",
			structRef: "struct:MyStruct",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testElements []CodeElement
			if tt.name != "Empty elements" {
				testElements = elements
			}

			got := GetMethodsOfStruct(testElements, tt.structRef)

			// DeepEqual treats nil slice and empty slice as not equal,
			// but our function returns a nil slice if nothing appended
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMethodsOfStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}
