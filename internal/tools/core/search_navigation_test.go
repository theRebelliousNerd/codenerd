package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/tools/codedom"
)

// N2 of Docs/audits/workspace-search-navigation-fidelity.md: the projection
// shortened method keys to the bare name, so A.Encode and B.Encode in one file
// became one ref, and following it reached the top-level Encode instead. Each
// printed ref must redeem through get_element to the element the hit was in.
func TestSearchCode_EveryRefRedeemsToTheElementItNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := fmt.Sprintf("nav_%d.go", codecFixtureSeq.Add(1))
	src := `package nav

type A struct{}
type B struct{}

func (A) Encode() int {
	return encodeA()
}

func (B) Encode() int {
	return encodeB()
}

func Encode() int {
	return encodeTop()
}
`
	if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := executeSearchCode(wsCtx(root), map[string]any{"pattern": "encode[ABT]"})
	if err != nil {
		t.Fatalf("search_code: %v", err)
	}
	want := map[string]string{"encodeA": "", "encodeB": "", "encodeTop": ""}
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[1], name+":") {
			refs = append(refs, fields[1])
		}
	}
	if len(refs) != 3 {
		t.Fatalf("want three distinct symbols, got %v:\n%s", refs, out)
	}
	getElement := codedom.GetElementTool()
	for _, ref := range refs {
		body, err := getElement.Execute(wsCtx(root), map[string]any{"ref": ref})
		if err != nil {
			t.Fatalf("get_element %s: %v", ref, err)
		}
		hits := 0
		for callee := range want {
			if strings.Contains(body, callee+"()") {
				want[callee] = ref
				hits++
			}
		}
		if hits != 1 {
			t.Fatalf("ref %s redeemed to an element that is not exactly one of the three:\n%s", ref, body)
		}
	}
	for callee, ref := range want {
		if ref == "" {
			t.Errorf("no ref redeemed to the element calling %s; refs %v", callee, refs)
		}
	}
}

// N1: a declaration line that matched on something other than the
// declared name is a mention, not a declaration, and a lexical edge is never
// called a reference.
func TestProject_ADeclarationLineMatchIsADeclarationOnlyWhenItNamesTheQuery(t *testing.T) {
	src := "package p\n\nfunc Decode(e Encoder) int {\n\treturn 0\n}\n\nfunc Encoder() {}\n"
	s := observation.Search{Query: "Encoder", Match: []observation.Match{
		{File: "p.go", Line: 3, Text: "func Decode(e Encoder) int {"},
		{File: "p.go", Line: 7, Text: "func Encoder() {}"},
	}}
	r := observation.Project(s, func(string) ([]byte, error) { return []byte(src), nil }, observation.Limits{})
	declared := map[string]bool{}
	for _, sym := range r.Symbols {
		declared[sym.Name] = sym.Declared
	}
	if declared["Decode"] || !declared["Encoder"] {
		t.Fatalf("declared = %v; only Encoder declares the query", declared)
	}
	if text := r.Text(""); !strings.Contains(text, "lexical") || strings.Contains(text, "references point to") {
		t.Fatalf("edges not labelled lexical:\n%s", text)
	}
}

// N3: the search read the matched lines, the projection read the file again.
// A file changed in between was projected with old text on new structure.
func TestProject_AFileChangedBetweenMatchingAndProjectionIsNotAttributed(t *testing.T) {
	now := "package p\n\nfunc B() {\n\treturn\n}\n"
	s := observation.Search{Query: "secret", Match: []observation.Match{{File: "p.go", Line: 4, Text: "secret()"}}}
	r := observation.Project(s, func(string) ([]byte, error) { return []byte(now), nil }, observation.Limits{})
	if len(r.Symbols) != 0 {
		t.Fatalf("a hit from the old bytes was attributed to %v in the new ones", r.Symbols)
	}
	if len(r.Changed) != 1 || r.Changed[0].File != "p.go" || r.Changed[0].Hits != 1 {
		t.Fatalf("changed = %v", r.Changed)
	}
	if !strings.Contains(r.Text(""), "changed between matching and projection") {
		t.Fatalf("the change is not reported:\n%s", r.Text(""))
	}
}
