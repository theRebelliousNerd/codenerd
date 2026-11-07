package policy

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

// specWorld loads schemas_spec.mg and spec_pairing.mg into an engine and
// asserts the EDB. Rows are Mangle fact text, one per line.
func specWorld(t *testing.T, edb string) *mangle.Engine {
	t.Helper()
	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	for _, f := range []string{"../schemas_spec.mg", "spec_pairing.mg"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := eng.LoadSchemaString(string(src)); err != nil {
			t.Fatalf("load %s: %v", f, err)
		}
	}
	loadEDBViaEngine(t, eng, edb)
	return eng
}

// rows renders every derived row of pred as "a|b|c", sorted.
func rows(t *testing.T, eng *mangle.Engine, pred string) []string {
	t.Helper()
	facts, err := eng.GetFacts(pred)
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range facts {
		parts := make([]string, len(f.Args))
		for i, a := range f.Args {
			parts[i] = fmt.Sprint(a)
		}
		out = append(out, strings.Join(parts, "|"))
	}
	sort.Strings(out)
	return out
}

func wantRows(t *testing.T, eng *mangle.Engine, pred string, want ...string) {
	t.Helper()
	got := rows(t, eng, pred)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n got  %q\n want %q", pred, got, want)
	}
}

// One package, internal/core, reached by two sections. Spec times: guide 100,
// notes 300. Code times: foo.go 200 (after guide, before notes).
const specEDB = `
spec_section("Docs/guide.md:Guide > Install", "Docs/guide.md", "Install", 100).
spec_section("Docs/notes.md:Notes", "Docs/notes.md", "Notes", 300).

spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, 200).
spec_element("internal/core.Kernel", "internal/core", "internal/core/foo.go", /struct, /true, 200).
spec_element("internal/core.Kernel.Eval", "internal/core", "internal/core/foo.go", /method, /true, 200).
spec_element("internal/core.Orphan", "internal/core", "internal/core/foo.go", /function, /true, 200).
spec_element("internal/core.helper", "internal/core", "internal/core/foo.go", /function, /false, 200).
spec_element("internal/core/foo.go:header", "internal/core", "internal/core/foo.go", /header, /true, 200).
spec_element("internal/lonely.Thing", "internal/lonely", "internal/lonely/a.go", /function, /true, 200).

spec_link("Docs/guide.md:Guide > Install", "internal/core.Func", /cites, /doc_citation).
spec_link("Docs/guide.md:Guide > Install", "internal/core.Kernel", /cites, /doc_citation).
spec_link("Docs/notes.md:Notes", "internal/core.Kernel", /implements, /annotation).
spec_link("Docs/notes.md:Notes", "internal/core/foo.go:header", /cites, /doc_citation).
spec_link("Docs/guide.md:Guide > Install", "internal/core/foo.go:Removed", /cites, /doc_citation).
spec_link("Docs/gone.md:Gone Heading", "internal/core.Func", /specifies, /code_comment).
`

func TestSpecDriftIsTheCodeChangingAfterTheSection(t *testing.T) {
	eng := specWorld(t, specEDB)
	// foo.go (200) is newer than guide.md (100): both guide links drift. It is
	// older than notes.md (300): neither notes link does. A link whose section
	// is gone has no section time to compare.
	wantRows(t, eng, "spec_drift",
		"Docs/guide.md:Guide > Install|internal/core.Func",
		"Docs/guide.md:Guide > Install|internal/core.Kernel",
	)
}

func TestDanglingLinkHasAnEndThatIsGone(t *testing.T) {
	eng := specWorld(t, specEDB)
	wantRows(t, eng, "dangling_spec_link",
		"Docs/guide.md:Guide > Install|internal/core/foo.go:Removed", // the cited element is gone
		"Docs/gone.md:Gone Heading|internal/core.Func",               // the section is gone
	)
}

func TestUnspeccedElementsAreExportedAndInAReachedPackage(t *testing.T) {
	eng := specWorld(t, specEDB)
	// Orphan and Eval are exported declarations of a reached package nothing
	// links to. helper is unexported, the header is not a declaration, and
	// internal/lonely is reached by no spec.
	wantRows(t, eng, "unspecced_element", "internal/core.Kernel.Eval", "internal/core.Orphan")
}

func TestSpecCoverageCountsExportedDeclarationsPerReachedPackage(t *testing.T) {
	eng := specWorld(t, specEDB)
	// Exported declarations of internal/core: Func, Kernel, Eval, Orphan (4);
	// Func and Kernel are linked (2).
	wantRows(t, eng, "spec_coverage", "internal/core|2|4")
}

func TestSpecCoverageOfAReachedPackageWithNothingSpeccedIsZero(t *testing.T) {
	eng := specWorld(t, `
spec_section("Docs/a.md:A", "Docs/a.md", "A", 1).
spec_element("pkg/x.go:header", "pkg", "pkg/x.go", /header, /true, 1).
spec_element("pkg.F", "pkg", "pkg/x.go", /function, /true, 1).
spec_link("Docs/a.md:A", "pkg/x.go:header", /cites, /doc_citation).
`)
	wantRows(t, eng, "spec_coverage", "pkg|0|1")
	wantRows(t, eng, "unspecced_element", "pkg.F")
}

const requirementEDB = `
spec_element("internal/core.Kernel.Eval", "internal/core", "internal/core/foo.go", /method, /true, 200).
spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, 200).
spec_element("internal/core.helper", "internal/core", "internal/core/foo.go", /function, /false, 200).
spec_element_property("internal/core.Func", /has_doc_comment).
spec_element_property("internal/core.Func", /has_test).
spec_element_calls("internal/core.Func", "internal/core.helper").

spec_requirement(/req, "stated obligations").
requires(/req, "internal/core.Func", /exists).
requires(/req, "internal/core.Func", /exported).
requires(/req, "internal/core.Func", /has_test).
requires(/req, "internal/core.Func", /has_doc_comment).
requires_calls(/req, "internal/core.Func", "internal/core.helper").

requires(/req, "internal/core.Kernel.Eval", /has_test).
requires(/req, "internal/core.Kernel.Eval", /has_doc_comment).
requires_calls(/req, "internal/core.Kernel.Eval", "internal/core.Func").

requires(/req, "internal/core.helper", /exported).

requires(/req, "internal/core.Removed", /exists).
requires_calls(/req, "internal/core.Removed", "internal/core.Func").
`

func TestRequirementViolatedAgainstTheCodeModel(t *testing.T) {
	eng := specWorld(t, requirementEDB)
	wantRows(t, eng, "requirement_violated",
		// Func meets all five: nothing listed for it.
		// Eval was measured as having neither a test nor a doc comment, and calls nothing.
		"/req|internal/core.Kernel.Eval|/has_test",
		"/req|internal/core.Kernel.Eval|/has_doc_comment",
		"/req|internal/core.Kernel.Eval|/calls",
		// helper is unexported.
		"/req|internal/core.helper|/exported",
		// Removed is not in the code model: every demand of it is violated.
		"/req|internal/core.Removed|/exists",
		"/req|internal/core.Removed|/calls",
	)
}

func TestRequirementNeedsNothingWhenTheCodeMeetsIt(t *testing.T) {
	eng := specWorld(t, `
spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, 1).
spec_element_property("internal/core.Func", /has_test).
spec_requirement(/ok, "fine").
requires(/ok, "internal/core.Func", /exists).
requires(/ok, "internal/core.Func", /exported).
requires(/ok, "internal/core.Func", /has_test).
`)
	wantRows(t, eng, "requirement_violated")
}
