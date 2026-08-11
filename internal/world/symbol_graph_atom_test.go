package world

import (
	"strings"
	"testing"
)

// TestSymbolGraphTypeAndVisibilityAreAtoms verifies that symbol_graph facts
// emit Type (slot 2) and Visibility (slot 3) as Mangle atoms (leading slash)
// rather than plain strings. This matches the schema in schemas_world.mg and
// the convention used by mangle_fastparse.go ("/predicate", "/public").
// Every consumer matches /function, /struct etc. as name constants, so a
// bare "function" string never unifies and derivations like relevant_context_file
// derive nothing.
func TestSymbolGraphTypeAndVisibilityAreAtoms(t *testing.T) {
	parser := NewTreeSitterParser()
	defer parser.Close()

	src := "package main\n\n" +
		"type MyStruct struct {\n\tField int\n}\n\n" +
		"func ExportedFunc() {}\n" +
		"func privateFunc() {}\n"

	facts, err := parser.ParseGo("test.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseGo failed: %v", err)
	}

	// Collect symbol_graph facts
	var sg []Fact
	for _, f := range facts {
		if f.Predicate == "symbol_graph" {
			sg = append(sg, f)
		}
	}
	if len(sg) == 0 {
		t.Fatalf("no symbol_graph facts emitted")
	}

	// 1. All Type and Visibility slots must start with "/"
	for _, f := range sg {
		if len(f.Args) < 3 {
			t.Errorf("symbol_graph fact has too few args: %+v", f)
			continue
		}
		typ, _ := f.Args[1].(string)
		vis, _ := f.Args[2].(string)
		if !strings.HasPrefix(typ, "/") {
			t.Errorf("Type slot missing leading slash: got %q in fact %+v (want \"/function\" etc.)", typ, f)
		}
		if !strings.HasPrefix(vis, "/") {
			t.Errorf("Visibility slot missing leading slash: got %q in fact %+v (want \"/public\" etc.)", vis, f)
		}
		// Also assert bare strings are not present
		if typ == "function" || typ == "struct" || typ == "method" || typ == "field" || typ == "package" || typ == "interface" || typ == "class" || typ == "enum" || typ == "module" || typ == "interface_method" {
			t.Errorf("Type slot is bare string %q, must be atom with leading slash (e.g. %q)", typ, "/"+typ)
		}
		if vis == "public" || vis == "private" || vis == "protected" {
			t.Errorf("Visibility slot is bare string %q, must be atom with leading slash (e.g. %q)", vis, "/"+vis)
		}
	}

	// 2. Specific expectations: exported function -> /function /public, struct -> /struct /public
	foundExportedFunc := false
	foundStruct := false
	for _, f := range sg {
		typ, _ := f.Args[1].(string)
		vis, _ := f.Args[2].(string)
		id, _ := f.Args[0].(string)
		if typ == "/function" && vis == "/public" && strings.Contains(id, "ExportedFunc") {
			foundExportedFunc = true
		}
		if typ == "/struct" && vis == "/public" && strings.Contains(id, "MyStruct") {
			foundStruct = true
		}
	}
	if !foundExportedFunc {
		t.Errorf("expected exported function with Type \"/function\" and Visibility \"/public\" not found; facts: %+v", sg)
	}
	if !foundStruct {
		t.Errorf("expected struct MyStruct with Type \"/struct\" and Visibility \"/public\" not found; facts: %+v", sg)
	}
}
