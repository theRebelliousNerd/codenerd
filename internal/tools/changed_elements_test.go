package tools

import (
	"strings"
	"testing"
)

const calcSrc = "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"

const genericSrc = "package elemprobe\n\ntype Box[T any] struct{ v T }\n\ntype Pair[A any, B any] struct{ a A; b B }\n\nfunc Get() int { return 1 }\n\nfunc (b *Box[T]) Get() T { return b.v }\n\nfunc (b Box[T]) Clone() Box[T] { return b }\n\nfunc (p *Pair[A, B]) First() A { return p.a }\n\nfunc (b (Box[T])) Size() int { return 0 }\n"

const mangleSrc = "# a doc line\nDecl p(X) bound [/string].\n\np(\"a\").\nq(X) :- p(X).\nq(X) :- r(X), !s(X).\n"

func declarations(edits []EditedElement) []EditedElement {
	var out []EditedElement
	for _, e := range edits {
		if e.Kind == "header" || e.Kind == "syntax_error" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func TestChangedElements_EditInsideAddLeavesSub(t *testing.T) {
	after := strings.Replace(calcSrc, "\treturn a + b\n", "\treturn a + b + 1\n", 1)
	edits := declarations(ChangedElements("calc.go", calcSrc, after))
	if len(edits) != 1 {
		t.Fatalf("got %+v", edits)
	}
	e := edits[0]
	if e.Name != "Add" || e.Kind != "function" || e.Package != "calc" || e.Receiver != "" || e.Removed || e.File != "calc.go" || e.Language != "go" {
		t.Fatalf("got %+v", e)
	}
}

func TestChangedElements_BothBodies(t *testing.T) {
	after := strings.Replace(calcSrc, "return a + b", "return a + b + 1", 1)
	after = strings.Replace(after, "return a - b", "return a - b - 1", 1)
	got := map[string]bool{}
	for _, e := range declarations(ChangedElements("calc.go", calcSrc, after)) {
		got[e.Kind+":"+e.Name] = true
	}
	if len(got) != 2 || !got["function:Add"] || !got["function:Sub"] {
		t.Fatalf("got %v", got)
	}
}

func TestChangedElements_BlankOrCommentBetweenFunctionsRecordsNothing(t *testing.T) {
	after := strings.Replace(calcSrc, "}\n\nfunc Sub", "}\n\n// note\n\nfunc Sub", 1)
	if eds := declarations(ChangedElements("calc.go", calcSrc, after)); len(eds) != 0 {
		t.Fatalf("a comment between functions is not either body: %+v", eds)
	}
}

func TestChangedElements_IdenticalAndLineEndingsRecordNothing(t *testing.T) {
	if eds := ChangedElements("calc.go", calcSrc, calcSrc); len(eds) != 0 {
		t.Fatalf("identical source: %+v", eds)
	}
	crlf := strings.ReplaceAll(calcSrc, "\n", "\r\n")
	if eds := ChangedElements("calc.go", crlf, calcSrc); len(eds) != 0 {
		t.Fatalf("line endings are not an element change: %+v", eds)
	}
}

func TestChangedElements_PackageClauseChangesTheHeaderOnly(t *testing.T) {
	after := strings.Replace(calcSrc, "package calc\n", "package calc2\n", 1)
	edits := ChangedElements("calc.go", calcSrc, after)
	if len(declarations(edits)) != 0 {
		t.Fatalf("function bodies did not change: %+v", edits)
	}
	if len(edits) != 1 || edits[0].Kind != "header" {
		t.Fatalf("got %+v", edits)
	}
}

func TestChangedElements_NewFileAndDeletedFile(t *testing.T) {
	created := ChangedElements("calc.go", "", calcSrc)
	names := map[string]bool{}
	var headers int
	for _, e := range created {
		if e.Removed {
			t.Fatalf("new file element marked removed: %+v", e)
		}
		if e.Kind == "header" {
			headers++
		}
		if e.Kind == "function" {
			names[e.Name] = true
		}
	}
	if headers != 1 || !names["Add"] || !names["Sub"] {
		t.Fatalf("new file: %+v", created)
	}

	removed := ChangedElements("calc.go", calcSrc, "")
	names = map[string]bool{}
	headers = 0
	for _, e := range removed {
		if !e.Removed {
			t.Fatalf("deleted file element not marked removed: %+v", e)
		}
		if e.Kind == "header" {
			headers++
		}
		if e.Kind == "function" {
			names[e.Name] = true
		}
	}
	if headers != 1 || !names["Add"] || !names["Sub"] {
		t.Fatalf("deleted file: %+v", removed)
	}
	if eds := ChangedElements("calc.go", "", ""); eds != nil {
		t.Fatalf("empty buffer is not a file of elements: %+v", eds)
	}
}

func TestChangedElements_GenericReceiverKeepsTheBaseType(t *testing.T) {
	created := ChangedElements("box.go", "", genericSrc)
	got := map[string]EditedElement{}
	for _, e := range declarations(created) {
		if strings.Contains(e.Receiver, "[") || strings.Contains(e.Receiver, "*") || strings.Contains(e.Receiver, "(") {
			t.Errorf("receiver %q keeps type arguments or a pointer", e.Receiver)
		}
		got[e.Kind+":"+e.Receiver+"."+e.Name] = e
	}
	for _, key := range []string{"function:.Get", "method:Box.Get", "method:Box.Clone", "method:Pair.First", "method:Box.Size", "struct:.Box", "struct:.Pair"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s in %+v", key, created)
		}
	}

	after := strings.Replace(genericSrc, "return b }", "return b /*x*/ }", 1)
	edits := declarations(ChangedElements("box.go", genericSrc, after))
	if len(edits) != 1 || edits[0].Kind != "method" || edits[0].Name != "Clone" || edits[0].Receiver != "Box" || edits[0].Package != "elemprobe" || edits[0].Removed {
		t.Fatalf("got %+v", edits)
	}
}

func TestChangedElements_MangleRuleBody(t *testing.T) {
	after := strings.Replace(mangleSrc, "q(X) :- p(X).", "q(X) :- p(X), p(X).", 1)
	edits := ChangedElements("x.mg", mangleSrc, after)
	var rules, other int
	var removed, added int
	for _, e := range edits {
		if e.Language != "mangle" {
			t.Errorf("language: %+v", e)
		}
		if e.Kind == "rule" && e.Name == "q" {
			rules++
			if e.Removed {
				removed++
			} else {
				added++
			}
			continue
		}
		other++
		t.Errorf("unchanged statement recorded: %+v", e)
	}
	// The rule key includes a hash of its text, so the old rule disappears
	// and the new text is a different element. The other q rule, the decl
	// and the fact keep their bytes.
	if rules != 2 || removed != 1 || added != 1 || other != 0 {
		t.Fatalf("rules=%d removed=%d added=%d other=%d %+v", rules, removed, added, other, edits)
	}
}

func TestChangedElements_LanguagesWithoutSpansReturnNil(t *testing.T) {
	before := "def add(a, b):\n    return a + b\n"
	after := "def add(a, b):\n    return a + b + 1\n"
	for _, rel := range []string{"calc.py", "calc.ts", "calc.rs"} {
		if eds := ChangedElements(rel, before, after); eds != nil {
			t.Fatalf("%s has no codemodel spans: %+v", rel, eds)
		}
	}
}
