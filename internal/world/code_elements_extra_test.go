package world

import (
	"os"
	"path/filepath"
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
