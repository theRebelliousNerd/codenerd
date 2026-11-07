package specs

import (
	"sort"
	"strings"
	"testing"
)

// fixtureIndex is a small workspace: a core package with a function, a type
// and its method, another package that also declares Func (so the bare name
// is a search), and a Mangle policy with one Decl.
func fixtureIndex() *CodeIndex {
	elems := []Element{
		{Ref: "internal/core/foo.go:header", Key: "header", Kind: "header", File: "internal/core/foo.go", Dir: "internal/core", Exported: true},
		{Ref: "internal/core.Func", Key: "Func", ID: "core.Func", Name: "Func", Kind: "function", File: "internal/core/foo.go", Dir: "internal/core", Exported: true, HasDoc: true},
		{Ref: "internal/core.Kernel", Key: "Kernel", ID: "core.Kernel", Name: "Kernel", Kind: "struct", File: "internal/core/foo.go", Dir: "internal/core", Exported: true},
		{Ref: "internal/core.Kernel.Eval", Key: "Kernel.Eval", ID: "core.Kernel.Eval", Name: "Eval", Receiver: "Kernel", Kind: "method", File: "internal/core/foo.go", Dir: "internal/core", Exported: true},
		{Ref: "internal/core.helper", Key: "helper", ID: "core.helper", Name: "helper", Kind: "function", File: "internal/core/foo.go", Dir: "internal/core"},
		{Ref: "internal/core.Unspecced", Key: "Unspecced", ID: "core.Unspecced", Name: "Unspecced", Kind: "function", File: "internal/core/foo.go", Dir: "internal/core", Exported: true},
		{Ref: "internal/core.TestFunc", Key: "TestFunc", ID: "core.TestFunc", Name: "TestFunc", Kind: "function", File: "internal/core/foo_test.go", Dir: "internal/core", Exported: true, Test: true},
		{Ref: "internal/other.Func", Key: "Func", ID: "other.Func", Name: "Func", Kind: "function", File: "internal/other/bar.go", Dir: "internal/other", Exported: true},
		{Ref: "internal/core/policy.mg:decl:impact_caller/2", Key: "decl:impact_caller/2", Name: "impact_caller", Kind: "decl", File: "internal/core/policy.mg", Dir: "internal/core", Exported: true},
	}
	return NewCodeIndex(elems, []string{"internal/core/foo.go", "internal/other/bar.go", "internal/core/policy.mg", "Docs/guide.md"})
}

const guide = "# Guide\n" +
	"The package is `internal/core/foo.go`.\n" +
	"\n" +
	"## Install\n" +
	"Install uses `internal/core/foo.go:Func`, internal/core/foo.go#Kernel.Eval and the `Kernel` type.\n" +
	"\n" +
	"### Linux\n" +
	"The file `internal/core/foo.go` defines `Unspecced`. The policy `impact_caller/2` joins it.\n" +
	"Bare `Func` is declared twice. Stale: `internal/core/foo.go:Gone`, internal/core/gone.go and internal/core/foo.go#Vanished.\n" +
	"Spelled with a receiver: (`(*Kernel).Eval`) in `internal/core/foo.go`.\n" +
	"A selector from elsewhere (`fmt.Errorf`) next to `internal/core/foo.go` is not a stale citation.\n" +
	"<!-- implements: internal/core/foo.go:Kernel.Eval, internal/other/bar.go#Func, internal/core/foo.go:Nope -->\n" +
	"\n" +
	"```go\n" +
	"internal/core/foo.go:Unspecced\n" +
	"```\n" +
	"\n" +
	"## Install\n" +
	"The second Install says `core.Kernel`.\n"

func scanGuide(t *testing.T) *DocScan {
	t.Helper()
	return ScanDoc("Docs/guide.md", guide, CodeLexer([]string{"internal", "Docs"}))
}

func linkSet(res Result) map[string]bool {
	out := map[string]bool{}
	for _, l := range res.Links {
		out[strings.Join([]string{l.SpecRef, l.CodeRef, string(l.Relation), string(l.Source)}, " | ")] = true
	}
	return out
}

func TestSectionsAreKeyedByHeadingPathAndDuplicates(t *testing.T) {
	d := scanGuide(t)
	var keys []string
	for _, s := range d.Sections {
		keys = append(keys, s.Ref)
	}
	want := []string{"Docs/guide.md:Guide", "Docs/guide.md:Guide > Install", "Docs/guide.md:Guide > Install > Linux", "Docs/guide.md:Guide > Install#2"}
	if strings.Join(keys, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sections:\n%s\nwant:\n%s", strings.Join(keys, "\n"), strings.Join(want, "\n"))
	}
}

func TestEverySpellingOfACitationLinks(t *testing.T) {
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex()})
	got := linkSet(res)
	const (
		guideRef   = "Docs/guide.md:Guide"
		installRef = "Docs/guide.md:Guide > Install"
		linuxRef   = "Docs/guide.md:Guide > Install > Linux"
		dupRef     = "Docs/guide.md:Guide > Install#2"
	)
	want := []string{
		// a bare path cites the file, by its header element
		guideRef + " | internal/core/foo.go:header | cites | doc_citation",
		// path:Symbol, path#Symbol and a backticked unique identifier
		installRef + " | internal/core.Func | cites | doc_citation",
		installRef + " | internal/core.Kernel.Eval | cites | doc_citation",
		installRef + " | internal/core.Kernel | cites | doc_citation",
		// a symbol bound to a path by adjacency, a Mangle predicate with its arity
		linuxRef + " | internal/core.Unspecced | cites | doc_citation",
		linuxRef + " | internal/core/policy.mg:decl:impact_caller/2 | cites | doc_citation",
		// the receiver spelled as Go writes it
		linuxRef + " | internal/core.Kernel.Eval | cites | doc_citation",
		// an explicit annotation, for links prose does not express
		linuxRef + " | internal/core.Kernel.Eval | implements | annotation",
		linuxRef + " | internal/other.Func | implements | annotation",
		// the package-qualified spelling resolves in the duplicate section
		dupRef + " | internal/core.Kernel | cites | doc_citation",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing link %s", w)
		}
	}
}

func TestStaleCitationsLinkToTheRefTheyWroteSoTheyCanBeFlagged(t *testing.T) {
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex()})
	got := linkSet(res)
	linux := "Docs/guide.md:Guide > Install > Linux"
	for _, stale := range []string{
		"internal/core/foo.go:Gone",     // file there, symbol gone
		"internal/core/foo.go:Vanished", // #Symbol spelling
		"internal/core/gone.go:header",  // file gone
		"internal/core/foo.go:Nope",     // annotation naming nothing
	} {
		found := false
		for k := range got {
			if strings.HasPrefix(k, linux+" | "+stale+" |") {
				found = true
			}
		}
		if !found {
			t.Errorf("no stale link to %s in %v", stale, sortedKeys(got))
		}
	}
	for k := range got {
		if strings.Contains(k, "fmt.Errorf") || strings.Contains(k, ":fmt") {
			t.Errorf("a standard-library selector next to a path became a link: %s", k)
		}
	}
}

func TestAmbiguousNameIsRefusedNotGuessed(t *testing.T) {
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex()})
	for k := range linkSet(res) {
		if strings.Contains(k, "internal/other.Func | cites") && strings.Contains(k, "Linux") {
			t.Errorf("bare `Func` (declared in two packages) was linked: %s", k)
		}
	}
	var found bool
	for _, p := range res.Problems {
		if p.Text == "`Func`" && strings.Contains(p.Reason, "internal/core.Func") && strings.Contains(p.Reason, "internal/other.Func") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the ambiguity was not reported with both candidates: %+v", res.Problems)
	}
}

func TestFencedCodeIsAnExampleNotACitation(t *testing.T) {
	src := "# Doc\n\n## Example\nRun it:\n\n```sh\ngo test internal/core/foo.go:Func `Kernel`\n```\n\nAfter the fence, `Kernel.Eval`.\n"
	d := ScanDoc("Docs/doc.md", src, CodeLexer([]string{"internal"}))
	res := Resolve(Input{Docs: []*DocScan{d}, Code: fixtureIndex()})
	if len(res.Links) != 1 || res.Links[0].CodeRef != "internal/core.Kernel.Eval" {
		t.Fatalf("only the prose after the fence cites code: %+v", res.Links)
	}
	if res.Links[0].SpecRef != "Docs/doc.md:Doc > Example" {
		t.Fatalf("spec ref: %s", res.Links[0].SpecRef)
	}
}

func TestLinksAreDeduplicated(t *testing.T) {
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex()})
	seen := map[Link]int{}
	for _, l := range res.Links {
		seen[l]++
	}
	for l, n := range seen {
		if n > 1 {
			t.Errorf("link repeated %d times: %+v", n, l)
		}
	}
}

func TestDocCommentSpecLinesSpecifyAnElement(t *testing.T) {
	code := `package core

// Func does the thing.
//
// spec: Docs/guide.md#Guide > Install > Linux
// spec: Docs/guide.md Install
// spec: Docs/guide.md:Gone Heading
// spec: Docs/missing.md#Anything
// spec: Docs/guide.md
func Func() {}
`
	cs := ScanCode("internal/core/foo.go", code)
	if len(cs.Cites) != 5 {
		t.Fatalf("cites: %+v", cs.Cites)
	}
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Codes: []*CodeScan{cs}, Code: fixtureIndex()})
	got := linkSet(res)
	for _, w := range []string{
		"Docs/guide.md:Guide > Install > Linux | internal/core.Func | specifies | code_comment",
		// "Install" names two sections (the heading repeats): refused
		// a missing heading is stale, and so is a missing document
		"Docs/guide.md:Gone Heading | internal/core.Func | specifies | code_comment",
		"Docs/missing.md:Anything | internal/core.Func | specifies | code_comment",
	} {
		if !got[w] {
			t.Errorf("missing %s in %v", w, sortedKeys(got))
		}
	}
	var ambiguous, bare bool
	for _, p := range res.Problems {
		if strings.Contains(p.Reason, "names 2 sections") {
			ambiguous = true
		}
		if strings.Contains(p.Reason, "not a section") {
			bare = true
		}
	}
	if !ambiguous || !bare {
		t.Fatalf("ambiguous heading and bare document must be reported: %+v", res.Problems)
	}
}

func TestCommentsWithoutSpecLinesAreNotParsed(t *testing.T) {
	if MayCiteSpecs("package x\n// plain comment\nfunc F() {}\n") {
		t.Fatal("a file with no spec: marker needs no parse")
	}
	if cs := ScanCode("internal/core/foo.go", "package x\n// plain\nfunc F() {}\n"); len(cs.Cites) != 0 {
		t.Fatalf("cites: %+v", cs.Cites)
	}
}

func TestElementsAreTargetsPlusTheExportedUniverseOfLinkedPackages(t *testing.T) {
	res := Resolve(Input{
		Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex(),
		Modified: func(f string) int64 { return int64(len(f)) },
	})
	refs := map[string]bool{}
	for _, e := range res.Elements {
		refs[e.Ref] = true
	}
	for _, w := range []string{"internal/core.Func", "internal/core.Unspecced", "internal/core.Kernel", "internal/other.Func"} {
		if !refs[w] {
			t.Errorf("%s should be reported", w)
		}
	}
	for _, no := range []string{"internal/core.helper", "internal/core.TestFunc"} {
		if refs[no] {
			t.Errorf("%s is outside the universe (unexported or a test)", no)
		}
	}
	if res.ElementModified["internal/core.Func"] != int64(len("internal/core/foo.go")) {
		t.Fatalf("element time not measured through the caller's clock: %v", res.ElementModified)
	}
}

func TestSectionsListedAreOnlyThoseWithALink(t *testing.T) {
	res := Resolve(Input{Docs: []*DocScan{scanGuide(t)}, Code: fixtureIndex(), Modified: func(string) int64 { return 7 }})
	if len(res.Sections) != 4 {
		t.Fatalf("sections: %+v", res.Sections)
	}
	for _, s := range res.Sections {
		if res.SectionModified[s.Ref] != 7 {
			t.Fatalf("section time: %v", res.SectionModified)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
