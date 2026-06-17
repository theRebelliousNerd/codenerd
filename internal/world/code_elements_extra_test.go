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

	if facts := ElementsToFacts(elements); len(facts) == 0 {
		t.Error("ElementsToFacts should produce facts for function/method elements")
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

func TestGetElementsInRange(t *testing.T) {
	elements := []CodeElement{
		{Ref: "elem:A", StartLine: 10, EndLine: 20},
		{Ref: "elem:B", StartLine: 30, EndLine: 40},
		{Ref: "elem:C", StartLine: 45, EndLine: 50},
	}

	tests := []struct {
		name      string
		startLine int
		endLine   int
		want      []string
	}{
		{"inside element", 12, 15, []string{"elem:A"}},
		{"exact match", 30, 40, []string{"elem:B"}},
		{"overlap start", 25, 35, []string{"elem:B"}},
		{"overlap end", 35, 45, []string{"elem:B", "elem:C"}},
		{"overlap multiple", 15, 45, []string{"elem:A", "elem:B", "elem:C"}},
		{"no overlap before", 1, 5, []string{}},
		{"no overlap between", 21, 29, []string{}},
		{"no overlap after", 60, 70, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetElementsInRange(elements, tt.startLine, tt.endLine)

			if len(got) != len(tt.want) {
				t.Errorf("GetElementsInRange() returned %d elements, want %d", len(got), len(tt.want))
				return
			}

			for i, wantRef := range tt.want {
				if got[i].Ref != wantRef {
					t.Errorf("GetElementsInRange()[%d] = %v, want %v", i, got[i].Ref, wantRef)
				}
			}
		})
	}
}
