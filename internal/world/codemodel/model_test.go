package codemodel

import (
	"strings"
	"testing"
)

const fixture = `// Package demo is a fixture.
//go:build linux

package demo

import (
	"fmt"
	"strings"
)

// Limit bounds things.
const Limit = 3

const (
	// A is first.
	A = 1
	Bee = 2 // trailing
)

var _ fmt.Stringer = T{}

// T is a type.
type T struct {
	x int
}

// M does the thing.
func (t *T) M() string {
	return strings.Repeat("a", t.x)
}

// N does another.
func (t T) N() string { return fmt.Sprint(t.x) }

func init() {}

func init() { _ = Limit }

func Generic[K comparable](k K) K { return k }
`

func TestParseGo_EveryDeclarationKindWithDocAndHeader(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	if !f.Parsed || f.Err != nil {
		t.Fatalf("fixture must parse: %v", f.Err)
	}
	if f.Package != "demo" || f.BuildConstraint != "linux" {
		t.Fatalf("package %q constraint %q", f.Package, f.BuildConstraint)
	}
	want := map[string]Kind{
		"header": KindHeader, "Limit": KindConst, "A": KindConst, "Bee": KindConst,
		"_": KindVar, "T": KindStruct, "T.M": KindMethod, "T.N": KindMethod,
		"init": KindFunction, "init#2": KindFunction, "Generic": KindFunction,
	}
	got := map[string]Kind{}
	for _, e := range f.Elements {
		got[e.Key] = e.Kind
	}
	for k, kind := range want {
		if got[k] != kind {
			t.Errorf("element %q: kind %q, want %q (have %v)", k, got[k], kind, got)
		}
	}
	m := f.Element("T.M")
	text := f.Text(m)
	if !strings.HasPrefix(text, "// M does the thing.") || !strings.HasSuffix(text, "}") {
		t.Fatalf("method span must run from its doc comment through its brace, got %q", text)
	}
	if m.DeclLine != m.StartLine+1 {
		t.Fatalf("DeclLine %d should follow the doc line %d", m.DeclLine, m.StartLine)
	}
	hdr := f.Header()
	if !strings.HasPrefix(f.Text(hdr), "// Package demo") || !strings.HasSuffix(f.Text(hdr), ")") {
		t.Fatalf("header must run from the file doc through the import block: %q", f.Text(hdr))
	}
	if len(f.Imports) != 2 || f.Imports[1].Path != "strings" {
		t.Fatalf("imports: %+v", f.Imports)
	}
	bee := f.Element("Bee")
	if !bee.Grouped || !strings.HasSuffix(f.Text(bee), "// trailing") {
		t.Fatalf("grouped spec keeps its line comment: %q", f.Text(bee))
	}
	if a := f.Element("A"); a.Doc != "A is first." || !strings.HasPrefix(f.Text(a), "// A is first.") {
		t.Fatalf("grouped spec doc: %q / %q", a.Doc, f.Text(a))
	}
	if g := f.Element("Generic"); !strings.Contains(g.Signature, "Generic") {
		t.Fatalf("signature %q", g.Signature)
	}
}

func TestRevision_IsTheElementsOwnBytes(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	edited := strings.Replace(fixture, "fmt.Sprint(t.x)", "fmt.Sprint(t.x + 1)", 1)
	g := ParseGo("demo.go", edited)
	if f.Element("T.M").Revision != g.Element("T.M").Revision {
		t.Fatal("editing N must not change M's revision")
	}
	if f.Element("T.N").Revision == g.Element("T.N").Revision {
		t.Fatal("editing N must change N's revision")
	}
	crlf := ParseGo("demo.go", strings.ReplaceAll(fixture, "\n", "\r\n"))
	if crlf.Element("T.M").Revision != f.Element("T.M").Revision {
		t.Fatal("line endings must not change a revision")
	}
}

func TestLookup_AcceptsTheSpellingsAModelUses(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	for _, q := range []string{"T.M", "(*T).M", "*T.M", "demo.T.M", "M"} {
		got := f.Lookup(q)
		if len(got) != 1 || got[0].Key != "T.M" {
			t.Errorf("Lookup(%q) = %d elements", q, len(got))
		}
	}
	if got := f.Lookup("init"); len(got) != 1 || got[0].Key != "init" {
		t.Errorf("an exact key wins: %v", got)
	}
}

func TestParseGo_BrokenFileKeepsGoodDeclarationsAndExposesTheBrokenRegion(t *testing.T) {
	broken := strings.Replace(fixture, "return strings.Repeat(\"a\", t.x)", "return strings.Repeat(\"a\", t.x", 1)
	f := ParseGo("demo.go", broken)
	if f.Parsed || len(f.Errors) == 0 {
		t.Fatal("broken source must report errors")
	}
	var region *Element
	for i := range f.Elements {
		if f.Elements[i].Kind == KindSyntaxError {
			region = &f.Elements[i]
		}
	}
	if region == nil {
		t.Fatalf("a broken file must expose a syntax_error element: %+v", f.Elements)
	}
	if !strings.Contains(f.Text(region), "func (t *T) M()") {
		t.Fatalf("the region must hold the broken declaration: %q", f.Text(region))
	}
	if f.Element("Limit") == nil || f.Element("header") == nil {
		t.Fatal("declarations away from the break stay addressable")
	}
}

func TestParts_AddressNestedBlocksByAST(t *testing.T) {
	src := `package p

func Big(x int) int {
	y := 0
	switch x {
	case 1:
		y = 1
	case 2:
		y = 2
		y++
	}
	if y > 0 {
		return y
	} else {
		return -y
	}
}
`
	f := ParseGo("p.go", src)
	e := f.Element("Big")
	parts := f.Parts(e)
	if len(parts) != 3 {
		t.Fatalf("three top-level statements, got %d", len(parts))
	}
	p, err := f.FindPart(e, "2.2")
	if err != nil {
		t.Fatal(err)
	}
	if p.Label != "case 2:" || len(p.Children) != 2 {
		t.Fatalf("part 2.2 = %q with %d children", p.Label, len(p.Children))
	}
	elseBranch, err := f.FindPart(e, "3.2")
	if err != nil || elseBranch.Label != "else" {
		t.Fatalf("part 3.2 must be the else branch: %v %+v", err, elseBranch)
	}
	if _, err := f.FindPart(e, "9"); err == nil {
		t.Fatal("an out-of-range part must be refused")
	}
}

func TestApply_EditsOneElementAndLeavesNeighboursIdentical(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	m := f.Element("T.M")
	text := f.Text(m)
	newText := strings.Replace(text, `strings.Repeat("a", t.x)`, `strings.Repeat("b",   t.x)`, 1)
	out, err := Apply(f, Change{Start: m.Start, End: m.End, Text: newText}, []string{"T.M"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	nm := out.File.Element("T.M")
	if !strings.Contains(out.File.Text(nm), `strings.Repeat("b", t.x)`) {
		t.Fatalf("the edited element is gofmt'd: %q", out.File.Text(nm))
	}
	for _, key := range []string{"T.N", "Limit", "A", "Bee", "header", "init#2"} {
		if out.File.Element(key).Revision != f.Element(key).Revision {
			t.Errorf("%s changed", key)
		}
	}
	if len(out.Touched) != 1 || out.Touched[0].Key != "T.M" {
		t.Fatalf("touched: %v", out.Touched)
	}
}

func TestApply_RefusesAnEditThatShredsANeighbour(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	m := f.Element("T.M")
	// Swallows N by leaving M open and closing it after N's body.
	n := f.Element("T.N")
	change := Change{Start: m.Start, End: n.End, Text: "func (t *T) M() string {\n\treturn \"\"\n}"}
	if _, err := Apply(f, change, []string{"T.M"}, nil); err == nil || !strings.Contains(err.Error(), "T.N") {
		t.Fatalf("an edit that removes an untargeted element must be refused naming it, got %v", err)
	}
}

func TestApply_RefusesAResultThatDoesNotParse(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	m := f.Element("T.M")
	_, err := Apply(f, Change{Start: m.Start, End: m.End, Text: "func (t *T) M() string {"}, []string{"T.M"}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("got %v", err)
	}
}

// A refused edit quotes the lines it wrote and nothing of the elements around
// it: an element verb that explained a parse error with its neighbour's source
// was a raw read of that neighbour.
func TestApply_ARefusalQuotesOnlyWhatTheEditWrote(t *testing.T) {
	const src = `package demo

// A holds the neighbour text.
func A() string {
	return "NEIGHBOURTEXT"
}

// B is the element the edit replaces.
func B() int {
	return 1
}
`
	f := ParseGo("demo.go", src)
	b := f.Element("B")
	if b == nil {
		t.Fatal("no element B")
	}
	_, err := Apply(f, Change{Start: b.Start, End: b.End, Text: "WRITTENLINE\n"}, []string{"B"}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("got %v, want a parse refusal", err)
	}
	if !strings.Contains(err.Error(), "WRITTENLINE") {
		t.Errorf("the refusal does not show what the edit wrote: %v", err)
	}
	if strings.Contains(err.Error(), "NEIGHBOURTEXT") {
		t.Errorf("the refusal quotes the neighbouring element: %v", err)
	}
}

type fakeResolver map[string]string

func (r fakeResolver) ResolveQualifier(q string) (string, []string) {
	if p, ok := r[q]; ok {
		return p, []string{p}
	}
	return "", nil
}
func (fakeResolver) DeclaredInPackage(string) bool { return false }
func (fakeResolver) ModulePath() string            { return "example.com/m" }

func TestApply_DerivesImports(t *testing.T) {
	f := ParseGo("demo.go", fixture)
	m := f.Element("T.M")
	newText := "// M does the thing.\nfunc (t *T) M() string {\n\treturn filepath.Join(\"a\", mod.Name)\n}"
	res := fakeResolver{"filepath": "path/filepath", "mod": "example.com/m/internal/mod"}
	out, err := Apply(f, Change{Start: m.Start, End: m.End, Text: newText}, []string{"T.M"}, res)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, imp := range out.File.Imports {
		got[imp.Path] = true
	}
	if !got["path/filepath"] || !got["example.com/m/internal/mod"] {
		t.Fatalf("new qualifiers must be imported: %v\n%s", got, out.File.Text(out.File.Header()))
	}
	if got["strings"] {
		t.Fatalf("strings is no longer used and must be dropped:\n%s", out.File.Text(out.File.Header()))
	}
	if !got["fmt"] {
		t.Fatal("fmt is still used by N")
	}
	if !out.Imports.Changed() {
		t.Fatal("the report must say imports moved")
	}
}

func TestParseMangle_KeysAreContentHashedNotOrdinal(t *testing.T) {
	src := `# a doc line
Decl p(X) bound [/string].

p("a").
q(X) :- p(X).
q(X) :- r(X), !s(X).
`
	f := ParseMangle("x.mg", src)
	if !f.Parsed {
		t.Fatalf("fixture must parse: %v", f.Err)
	}
	var rules []string
	for _, e := range f.Elements {
		if e.Kind == KindRule {
			rules = append(rules, e.Key)
		}
	}
	if len(rules) != 2 || !strings.HasPrefix(rules[0], "rule:q/1@") {
		t.Fatalf("rules %v", rules)
	}
	decl := f.Element("decl:p/1")
	if decl == nil || decl.Doc != "a doc line" || !strings.HasPrefix(f.Text(decl), "# a doc line") {
		t.Fatalf("decl with doc: %+v", decl)
	}
	inserted := ParseMangle("x.mg", strings.Replace(src, "p(\"a\").", "p(\"a\").\nq(X) :- t(X).", 1))
	for _, k := range rules {
		if inserted.Element(k) == nil {
			t.Fatalf("inserting a rule above must not rename %s", k)
		}
	}
	if got := MangleBodyPredicates("q(X) :- r(X), !s(X), fn:count(Y), :string:contains(A, B)."); strings.Join(got, ",") != "r,s" {
		t.Fatalf("body predicates %v", got)
	}
}
