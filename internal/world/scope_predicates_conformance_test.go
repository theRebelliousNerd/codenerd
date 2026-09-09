package world

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codenerd/internal/core"
)

// scopeFixtures are minimal sources that exercise every per-language Stratum-0
// emitter the parser factory routes to. They are deliberately ugly: the point
// is coverage of fact emission, not readable example code.
var scopeFixtures = map[string]string{
	"a.go": "package a\n" +
		"import \"context\"\n" +
		"type S struct{ F string `json:\"f\"` }\n" +
		"type I interface{ M() error }\n" +
		"func (s *S) M() error { return nil }\n" +
		"func Do(ctx context.Context) error { go func() {}(); return nil }\n",
	"b.py": "import asyncio\n" +
		"from pydantic import BaseModel\n" +
		"class C(BaseModel):\n    x: int\n" +
		"@dec\nasync def f(a: int) -> int:\n    return a\n",
	"c.ts": "export interface I { a: string; }\n" +
		"export class K extends B implements I { a = ''; }\n" +
		"export type T = string;\n" +
		"export async function f(): Promise<void> {}\n" +
		"export const C = () => { const [s, setS] = useState(0); return null; };\n",
	"d.rs": "use serde::Serialize;\n" +
		"#[derive(Serialize, Debug)]\n" +
		"pub struct S { #[serde(rename = \"x\")] pub a: String }\n" +
		"pub trait T { fn m(&self); }\n" +
		"pub async fn f() -> Result<(), ()> { unsafe {}; let v: Option<i32> = None; v.unwrap(); Ok(()) }\n",
	"e.mg": "Decl foo(A) bound [/string].\n" +
		"foo(X) :- bar(X), !baz(X).\n" +
		"bar(\"a\").\n" +
		"reach(X) :- reach(Y), edge(Y, X).\n" +
		"total(X) :- item(X) |> do fn:group_by(), let X = fn:count().\n" +
		"?foo(X)\n",
}

// emitScopePredicates parses every fixture through the real parser factory and
// returns the set of predicates a CodeDOM scope would assert for them.
func emitScopePredicates(t *testing.T) map[string]struct{} {
	t.Helper()
	dir := t.TempDir()
	pf := DefaultParserFactory(dir)
	got := make(map[string]struct{})

	for name, src := range scopeFixtures {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if !pf.HasParser(path) {
			t.Fatalf("no parser registered for %s — fixture set and parser factory have drifted", name)
		}
		res, err := pf.ParseWithFacts(path, []byte(src))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, f := range res.LanguageFacts {
			got[f.Predicate] = struct{}{}
		}
		for _, e := range res.Elements {
			for _, f := range e.ToFacts() {
				got[f.Predicate] = struct{}{}
			}
		}
	}
	return got
}

// TestCodeDOMScopePredicates_CoverEveryEmittedPredicate is the drift guard for
// a fact leak: core.clearCodeDOMFacts listed the element and diagnostic
// predicates but none of the per-language Stratum-0 predicates the parsers
// emit, so those facts were asserted on every open_file / edit_element /
// refresh_scope and never retracted. The EDB grew for the whole session and
// kept deriving from files that had left scope.
//
// Adding a new emitter to any parser without adding it to the retraction set
// fails here, which is the only place the two halves can be compared: world
// imports core, so the list lives in core and is pinned from here.
func TestCodeDOMScopePredicates_CoverEveryEmittedPredicate(t *testing.T) {
	emitted := emitScopePredicates(t)
	if len(emitted) == 0 {
		t.Fatal("fixtures emitted no facts at all; the probe itself is broken")
	}
	replaceSet := core.CodeDOMScopePredicates()

	var missing []string
	for pred := range emitted {
		if _, ok := replaceSet[pred]; !ok {
			missing = append(missing, pred)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("CodeDOM scope emits %d predicate(s) that clearCodeDOMFacts never retracts: %v\n"+
			"Add them to codeDOMScopePredicates in internal/core/virtual_store.go, or the facts leak for the session's lifetime.",
			len(missing), missing)
	}
}

// TestCodeDOMScopePredicates_ReturnsACopy pins the defensive copy. A shared map
// handed to a caller is one stray write away from a retraction list with a hole
// in it, and the hole would be invisible until facts started surviving a scope
// change.
func TestCodeDOMScopePredicates_ReturnsACopy(t *testing.T) {
	first := core.CodeDOMScopePredicates()
	if _, ok := first["code_element"]; !ok {
		t.Fatal("replace-set is missing code_element")
	}
	delete(first, "code_element")

	second := core.CodeDOMScopePredicates()
	if _, ok := second["code_element"]; !ok {
		t.Fatal("mutating the returned map corrupted the shared replace-set")
	}
}

// TestCodeDOMScopePredicates_CoversLanguageFamilies is a cheap explicit check
// that no whole language family was forgotten, so a fixture that silently stops
// parsing cannot make the conformance test vacuously pass.
func TestCodeDOMScopePredicates_CoversLanguageFamilies(t *testing.T) {
	replaceSet := core.CodeDOMScopePredicates()
	for _, pred := range []string{
		"go_struct", "method_of",
		"py_class", "has_pydantic_base",
		"ts_interface", "ts_component",
		"rs_struct", "rs_serde_rename",
		"mg_decl", "mg_query",
	} {
		if _, ok := replaceSet[pred]; !ok {
			t.Errorf("replace-set is missing %q", pred)
		}
	}
}
