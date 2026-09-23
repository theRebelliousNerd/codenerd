package codedom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

const elementFixture = `// Package demo is a fixture.
package demo

import "strings"

// Limit bounds things.
const Limit = 3

var (
	// verbose logs more.
	verbose bool
	count   int
)

// T is a type.
type T struct{ x int }

// M does the thing.
func (t *T) M() string {
	return strings.Repeat("a", t.x)
}

// N does another.
func (t T) N() string { return "n" }

func Close() error { return nil }

type A struct{}

func (a *A) Close() error { return nil }
`

func writeElementFixture(t *testing.T, name, content string) (context.Context, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return tools.WithWorkspaceRoot(context.Background(), dir), name
}

func TestElementsFromSource_GoComesFromTheAST(t *testing.T) {
	t.Parallel()
	els := ElementsFromSource("demo.go", elementFixture)
	got := map[string]CodeElement{}
	for _, e := range els {
		got[e.Name] = e
	}
	for name, kind := range map[string]string{
		"Limit": "const", "verbose": "var", "count": "var", "T": "struct",
		"T.M": "method", "T.N": "method", "Close": "function", "A.Close": "method",
	} {
		if got[name].Type != kind {
			t.Errorf("%s: kind %q, want %q (have %v)", name, got[name].Type, kind, els)
		}
	}
	m := got["T.M"]
	if m.DeclLine != m.StartLine+1 {
		t.Fatalf("T.M spans its doc (start %d) and declares on the next line (decl %d)", m.StartLine, m.DeclLine)
	}
}

func TestElementsFromSource_RegexLanguagesStillQualifyMembers(t *testing.T) {
	t.Parallel()
	py := ElementsFromSource("x.py", "class A:\n    def close(self):\n        pass\n\n    def outer(self):\n        def inner():\n            pass\n\ndef close():\n    pass\n")
	names := map[string]string{}
	for _, e := range py {
		names[e.Name] = e.Type
	}
	if names["A.close"] != "method" || names["close"] != "function" {
		t.Fatalf("python: %v", py)
	}
	js := ElementsFromSource("x.js", "class A {\n  close() {\n    return '}';\n  }\n}\nclass B {\n  close() {\n    // {\n  }\n}\n")
	var qualified []string
	for _, e := range js {
		if e.Type == "method" {
			qualified = append(qualified, fmt.Sprintf("%s:%d-%d", e.Name, e.StartLine, e.EndLine))
		}
	}
	if strings.Join(qualified, ",") != "A.close:2-4,B.close:7-9" {
		t.Fatalf("js methods scoped to their class with brace-counted extents: %v", qualified)
	}
}

func TestGetElements_ListsEveryKindWithRefsAndRevisions(t *testing.T) {
	ctx, path := writeElementFixture(t, "demo.go", elementFixture)
	out, err := executeGetElements(ctx, map[string]any{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(go, package demo", "parses)",
		"demo.go:1-4  header  demo.go:header  rev ",
		"const  ./Limit  rev ", "var  ./verbose", "var  ./count", "struct  ./T  rev ",
		"method  ./T.M  rev ", "// M does the thing.", "method  ./A.Close",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("get_elements is missing %q:\n%s", want, out)
		}
	}
	only, err := executeGetElements(ctx, map[string]any{"path": path, "kind": "var"})
	if err != nil || strings.Contains(only, "method") || !strings.Contains(only, "-- 2 rows, complete") {
		t.Fatalf("kind filter: %v\n%s", err, only)
	}
}

func TestGetElements_BrokenFileStaysListedWithItsError(t *testing.T) {
	broken := strings.Replace(elementFixture, `strings.Repeat("a", t.x)`, `strings.Repeat("a", t.x`, 1)
	ctx, path := writeElementFixture(t, "demo.go", broken)
	out, err := executeGetElements(ctx, map[string]any{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DOES NOT PARSE", "syntax error at line", "syntax_error  demo.go:syntax_error", "./Limit", "./A.Close"} {
		if !strings.Contains(out, want) {
			t.Errorf("broken listing is missing %q:\n%s", want, out)
		}
	}
	region, err := executeGetElement(ctx, map[string]any{"ref": "demo.go:syntax_error"})
	if err != nil || !strings.Contains(region, "func (t *T) M()") || !strings.Contains(region, "does not parse") {
		t.Fatalf("the broken region is viewable by ref: %v\n%s", err, region)
	}
}

func TestGetElements_UnparsedLanguageSaysSo(t *testing.T) {
	ctx, path := writeElementFixture(t, "x.py", "def f():\n    pass\n")
	out, err := executeGetElements(ctx, map[string]any{"path": path})
	if err != nil || !strings.Contains(out, "no parser for this language") || !strings.Contains(out, "function  f") {
		t.Fatalf("%v\n%s", err, out)
	}
}

// The R8 audit's failing test for G1: get_element returned no body, so every
// look at code went through read_file.
func TestGetElement_ReturnsTheSourceWithItsDocCommentAndRevision(t *testing.T) {
	ctx, path := writeElementFixture(t, "demo.go", elementFixture)
	out, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "T.M"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"./T.M  method  demo.go:18-21  rev ",
		"18  // M does the thing.",
		"19  func (t *T) M() string {",
		`20  	return strings.Repeat("a", t.x)`,
		"21  }",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("get_element is missing %q:\n%s", want, out)
		}
	}
}

func TestGetElement_AcceptsEverySpellingAndRefusesAmbiguity(t *testing.T) {
	ctx, path := writeElementFixture(t, "demo.go", elementFixture)
	for _, ref := range []string{"T.M", "(*T).M", "*T.M", "./T.M", "demo.go:T.M"} {
		args := map[string]any{"ref": ref}
		if !strings.HasPrefix(ref, "demo.go:") {
			args["path"] = path
		}
		out, err := executeGetElement(ctx, args)
		if err != nil || !strings.Contains(out, "func (t *T) M()") {
			t.Errorf("ref %q: %v\n%s", ref, err, out)
		}
	}
	// Close is both a function and a method: the bare name picks the function.
	fn, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "Close"})
	if err != nil || !strings.Contains(fn, "func Close()") {
		t.Fatalf("bare Close: %v\n%s", err, fn)
	}
	twoMethods := elementFixture + "\ntype B struct{}\n\nfunc (b *B) Size() int { return 0 }\n\nfunc (a *A) Size() int { return 1 }\n"
	ctx2, path2 := writeElementFixture(t, "two.go", twoMethods)
	_, err = executeGetElement(ctx2, map[string]any{"path": path2, "ref": "Size"})
	if err == nil || !strings.Contains(err.Error(), "./B.Size") || !strings.Contains(err.Error(), "./A.Size") {
		t.Fatalf("an ambiguous name must be refused with every candidate ref: %v", err)
	}
	if _, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "Nope"}); err == nil || !strings.Contains(err.Error(), "get_elements") {
		t.Fatalf("not found must point at get_elements: %v", err)
	}
	if _, err := executeGetElement(ctx, map[string]any{"path": path}); err == nil {
		t.Fatal("a missing ref must be refused")
	}
	both, err := executeGetElement(ctx, map[string]any{"path": path, "refs": []any{"Limit", "T.N"}})
	if err != nil || !strings.Contains(both, "const Limit = 3") || !strings.Contains(both, `func (t T) N()`) {
		t.Fatalf("several refs in one call: %v\n%s", err, both)
	}
	hdr, err := executeGetElement(ctx, map[string]any{"ref": "demo.go:header"})
	if err != nil || !strings.Contains(hdr, `import "strings"`) || !strings.Contains(hdr, "// Package demo") {
		t.Fatalf("the header is an element: %v\n%s", err, hdr)
	}
}

func TestGetElement_LargeElementAnswersWithItsPartsAndPartNarrows(t *testing.T) {
	var body strings.Builder
	body.WriteString("package big\n\n// Huge is long.\nfunc Huge(x int) int {\n\tswitch x {\n")
	for i := range elementPageLines {
		fmt.Fprintf(&body, "\tcase %d:\n\t\tx++\n", i)
	}
	body.WriteString("\t}\n\treturn x\n}\n")
	ctx, path := writeElementFixture(t, "big.go", body.String())
	out, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "Huge"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "its parts") || !strings.Contains(out, "part 1  lines 5-") || !strings.Contains(out, "part 2  lines") {
		t.Fatalf("a large element answers with its outline:\n%.1500s", out)
	}
	if strings.Contains(out, "x++") {
		t.Fatal("the outline must not print the whole body")
	}
	part, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "Huge", "part": "1.301"})
	if err != nil || !strings.Contains(part, "case 300:") || strings.Contains(part, "case 299:") {
		t.Fatalf("part narrows to one case clause: %v\n%s", err, part)
	}
	whole, err := executeGetElement(ctx, map[string]any{"path": path, "ref": "Huge", "full": true})
	if err != nil || !strings.Contains(whole, "case 399:") {
		t.Fatalf("full returns everything: %v", err)
	}
}

func TestGetElement_MangleStatementsAreElements(t *testing.T) {
	src := "# Declares edges.\nDecl edge(X, Y) bound [/string, /string].\n\npath(X, Y) :- edge(X, Y).\n"
	ctx, path := writeElementFixture(t, "p.mg", src)
	list, err := executeGetElements(ctx, map[string]any{"path": path})
	if err != nil || !strings.Contains(list, "decl  p.mg:decl:edge/2") || !strings.Contains(list, "rule  p.mg:rule:path/2@") {
		t.Fatalf("%v\n%s", err, list)
	}
	decl, err := executeGetElement(ctx, map[string]any{"ref": "p.mg:decl:edge/2"})
	if err != nil || !strings.Contains(decl, "1  # Declares edges.") || !strings.Contains(decl, "Decl edge(X, Y)") {
		t.Fatalf("%v\n%s", err, decl)
	}
}
