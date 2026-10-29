package codedom

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

const twoMethods = `package demo

import "fmt"

type T struct{ a, b int }

// M returns a.
func (t *T) M() int {
	return t.a
}

// N returns a as text.
func (t *T) N() string {
	return fmt.Sprint(t.a)
}
`

// editFixture writes content under a temp workspace and returns a registry
// holding the codedom tools, with no structure index registered: the element
// verbs work on one file without it.
func editFixture(t *testing.T, name, content string) (*tools.Registry, string, string) {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	return reg, abs, name
}

func execTool(reg *tools.Registry, name string, args map[string]any) (string, []tools.EditedElement, error) {
	var edits []tools.EditedElement
	reg.SetFactSink(func(_ context.Context, rec tools.ExecutionRecord) { edits = rec.Edits })
	res, err := reg.Execute(context.Background(), name, args)
	if err != nil {
		return "", nil, err
	}
	return res.Result, edits, nil
}

func fileText(t *testing.T, abs string) string {
	t.Helper()
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The R8 audit's failing tests for G2: no element-addressed edit existed on the
// model surface, and no model edit told the kernel what it changed.
func TestEditElement_ChangesOnlyTheTargetAndRecordsTheEdit(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	before := fileText(t, abs)
	out, edits, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "return t.a", "new": "return t.b"})
	if err != nil {
		t.Fatal(err)
	}
	after := fileText(t, abs)
	if !strings.Contains(after, "return t.b") || !strings.Contains(after, "return fmt.Sprint(t.a)") {
		t.Fatalf("only M changes:\n%s", after)
	}
	nStart := strings.Index(before, "// N returns")
	if before[nStart:] != after[nStart:] {
		t.Fatal("N's bytes must be identical after editing M")
	}
	if !strings.Contains(out, "./T.M  method  demo.go:7-10  rev ") || !strings.Contains(out, "9  	return t.b") {
		t.Fatalf("the answer carries the new text and rev:\n%s", out)
	}
	if len(edits) != 1 || edits[0].Name != "M" || edits[0].Receiver != "T" || edits[0].Kind != "method" || edits[0].Package != "demo" || edits[0].File != "demo.go" {
		t.Fatalf("the kernel is told exactly which element changed: %+v", edits)
	}
}

func TestEditElement_AnchorMustBeUniqueWithinTheElement(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	before := fileText(t, abs)
	// "fmt.Sprint" occurs in N only: an anchor scoped to M must not reach it.
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "fmt.Sprint", "new": "x"}); err == nil || !strings.Contains(err.Error(), "does not occur in ./T.M") {
		t.Fatalf("an anchor outside the element must be refused: %v", err)
	}
	// "t.a" occurs twice across the file but once in each method.
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.N", "old": "t.a", "new": "t.b"}); err != nil {
		t.Fatalf("a short anchor unique within the element is enough: %v", err)
	}
	reg2, _, rel2 := editFixture(t, "twice.go", "package p\n\nfunc F() int {\n\tx := 1\n\tx := 1\n\treturn x\n}\n")
	if _, _, err := execTool(reg2, "edit_element", map[string]any{"path": rel2, "ref": "F", "old": "x := 1", "new": "y := 2"}); err == nil || !strings.Contains(err.Error(), "occurs 2 times") || !strings.Contains(err.Error(), "lines 4, 5") {
		t.Fatalf("a repeated anchor is refused with its lines: %v", err)
	}
	_ = before
}

func TestEditElement_ToleratesIndentationOfWholeLines(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	_, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "    return t.a", "new": "return t.a + t.b"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fileText(t, abs), "\treturn t.a + t.b\n") {
		t.Fatalf("spaces for a tab still locate the line, and gofmt restores the indent:\n%s", fileText(t, abs))
	}
}

func TestEditElement_RevisionPreconditionAndRefusalsLeaveTheFile(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	before := fileText(t, abs)
	_, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "return t.a", "new": "return t.b", "revision": "000000000000"})
	if err == nil || !strings.Contains(err.Error(), "precondition failed") || !strings.Contains(err.Error(), "return t.a") {
		t.Fatalf("a stale rev is refused with the element as it is now: %v", err)
	}
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "return t.a", "new": "return t.a +"}); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Fatalf("an unparseable result is refused: %v", err)
	}
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "return t.a\n}", "new": "return t.a\n}\n\nfunc Extra() {"}); err == nil {
		t.Fatal("text that opens a declaration the element does not own must be refused")
	}
	if fileText(t, abs) != before {
		t.Fatal("a refused edit must leave the file byte-for-byte")
	}
}

func TestReplaceElement_ReplacesTheDocCommentToo(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	out, _, err := execTool(reg, "replace_element", map[string]any{
		"path": rel, "ref": "T.N",
		"source": "// N renders b.\nfunc (t *T) N() string {\n\treturn fmt.Sprint(t.b)\n}",
	})
	if err != nil {
		t.Fatal(err)
	}
	after := fileText(t, abs)
	if strings.Contains(after, "N returns a as text") || !strings.Contains(after, "// N renders b.") {
		t.Fatalf("the doc comment belongs to the element:\n%s", after)
	}
	if !strings.Contains(out, "rev ") || !strings.Contains(out, "// N renders b.") {
		t.Fatalf("answer:\n%s", out)
	}
}

func TestInsertElement_BeforeAfterHeaderAndEnd(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", twoMethods)
	for _, c := range []struct{ anchor, position, src string }{
		{"T.M", "after", "func (t *T) Between() {}"},
		{"T", "before", "// First is first.\nconst First = 1"},
		{"header", "after", "var afterHeader = 2"},
		{"end", "", "func Last() {}"},
	} {
		out, edits, err := execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": c.anchor, "position": c.position, "source": c.src})
		if err != nil {
			t.Fatalf("insert %s %s: %v", c.position, c.anchor, err)
		}
		if len(edits) == 0 || !strings.Contains(out, "rev ") {
			t.Fatalf("insert must report the new element: %+v\n%s", edits, out)
		}
	}
	after := fileText(t, abs)
	order := []string{"var afterHeader", "const First", "type T struct", "func (t *T) M()", "func (t *T) Between()", "func (t *T) N()", "func Last()"}
	last := -1
	for _, s := range order {
		i := strings.Index(after, s)
		if i < 0 || i < last {
			t.Fatalf("%q out of place:\n%s", s, after)
		}
		last = i
	}
	if _, _, err := execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "header", "position": "before", "source": "var x = 1"}); err == nil {
		t.Fatal("nothing goes before the header")
	}
}

func TestDeleteElement_NeedsTheIndexToCheckUses(t *testing.T) {
	reg, _, rel := editFixture(t, "demo.go", twoMethods)
	if _, _, err := execTool(reg, "delete_element", map[string]any{"path": rel, "ref": "T.N"}); err == nil || !strings.Contains(err.Error(), "structure index") {
		t.Fatalf("without an index a delete cannot know it is safe: %v", err)
	}
}

func TestElementEdits_KeepTheFilesLineEndings(t *testing.T) {
	reg, abs, rel := editFixture(t, "demo.go", strings.ReplaceAll(twoMethods, "\n", "\r\n"))
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "T.M", "old": "return t.a", "new": "return t.b"}); err != nil {
		t.Fatal(err)
	}
	after := fileText(t, abs)
	if strings.Count(after, "\r\n") != strings.Count(after, "\n") {
		t.Fatal("a CRLF file stays CRLF")
	}
}

func TestEditElement_PartNarrowsTheAnchor(t *testing.T) {
	src := "package p\n\nfunc F(x int) int {\n\tswitch x {\n\tcase 1:\n\t\tx++\n\tcase 2:\n\t\tx++\n\t}\n\treturn x\n}\n"
	reg, abs, rel := editFixture(t, "p.go", src)
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "F", "old": "x++", "new": "x--"}); err == nil {
		t.Fatal("x++ is not unique in F")
	}
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "F", "part": "1.2", "old": "x++", "new": "x--"}); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`case 1:\s+x\+\+\s+case 2:\s+x--`).MatchString(fileText(t, abs)) {
		t.Fatalf("only the second case changed:\n%s", fileText(t, abs))
	}
}

func TestEditElement_MangleRuleMustStillParse(t *testing.T) {
	src := "Decl p(X) bound [/string].\n\nq(X) :- p(X).\n"
	reg, abs, rel := editFixture(t, "x.mg", src)
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "decl:p/1", "old": "bound [/string]", "new": "bound [/string"}); err == nil {
		t.Fatal("a Mangle edit that does not parse must be refused")
	}
	out, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "decl:p/1", "old": "Decl p(X)", "new": "# p is the input.\nDecl p(X)"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fileText(t, abs), "# p is the input.") || !strings.Contains(out, "x.mg:decl:p/1") {
		t.Fatalf("%s", out)
	}
}

func TestReplaceElement_RepairsABrokenRegion(t *testing.T) {
	broken := strings.Replace(twoMethods, "return t.a\n}", "return t.a\n", 1)
	reg, abs, rel := editFixture(t, "demo.go", broken)
	if _, _, err := execTool(reg, "replace_element", map[string]any{
		"path": rel, "ref": "syntax_error",
		"source": "// M returns a.\nfunc (t *T) M() int {\n\treturn t.a\n}",
	}); err != nil {
		t.Fatal(err)
	}
	after := fileText(t, abs)
	if !strings.Contains(after, "func (t *T) M() int {\n\treturn t.a\n}") {
		t.Fatalf("the broken region is replaced:\n%s", after)
	}
}
