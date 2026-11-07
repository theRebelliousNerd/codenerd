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
	for _, f := range []string{"../schemas_spec.mg", "../schemas_safety.mg", "spec_pairing.mg"} {
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

// One package, internal/core, reached by two sections. Each side carries a
// fingerprint; spec_confirmed says what both sides said when the link was last
// confirmed. Func has moved since (code fingerprint fp200 against a confirmed
// fp100) while the guide section has not; the notes section moved (fp300
// against fp250) while the code it names did not.
const specEDB = `
spec_section("Docs/guide.md:Guide > Install", "Docs/guide.md", "Install", "fp-guide").
spec_section("Docs/notes.md:Notes", "Docs/notes.md", "Notes", "fp300").

spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, "fp200").
spec_element("internal/core.Kernel", "internal/core", "internal/core/foo.go", /struct, /true, "fp-kernel").
spec_element("internal/core.Kernel.Eval", "internal/core", "internal/core/foo.go", /method, /true, "fp200").
spec_element("internal/core.Orphan", "internal/core", "internal/core/foo.go", /function, /true, "fp200").
spec_element("internal/core.helper", "internal/core", "internal/core/foo.go", /function, /false, "fp200").
spec_element("internal/core/foo.go:header", "internal/core", "internal/core/foo.go", /header, /true, "fp-file").
spec_element("internal/lonely.Thing", "internal/lonely", "internal/lonely/a.go", /function, /true, "fp200").

spec_link("Docs/guide.md:Guide > Install", "internal/core.Func", /cites, /doc_citation).
spec_link("Docs/guide.md:Guide > Install", "internal/core.Kernel", /cites, /doc_citation).
spec_link("Docs/notes.md:Notes", "internal/core.Kernel", /implements, /annotation).
spec_link("Docs/notes.md:Notes", "internal/core/foo.go:header", /cites, /doc_citation).
spec_link("Docs/guide.md:Guide > Install", "internal/core/foo.go:Removed", /cites, /doc_citation).
spec_link("Docs/gone.md:Gone Heading", "internal/core.Func", /specifies, /code_comment).

spec_confirmed("Docs/guide.md:Guide > Install", "internal/core.Func", "fp-guide", "fp100", 1000).
spec_confirmed("Docs/guide.md:Guide > Install", "internal/core.Kernel", "fp-guide", "fp-kernel", 1000).
spec_confirmed("Docs/notes.md:Notes", "internal/core.Kernel", "fp250", "fp-kernel", 1000).
spec_confirmed("Docs/notes.md:Notes", "internal/core/foo.go:header", "fp250", "fp-file", 1000).
spec_confirmed("Docs/gone.md:Gone Heading", "internal/core.Func", "fp-gone", "fp100", 1000).

spec_changed_since("Docs/guide.md:Guide > Install", "internal/core.Func", /code, 1500).
spec_changed_since("Docs/notes.md:Notes", "internal/core.Kernel", /spec, 1600).
`

func TestSpecDriftIsTheCodeSayingSomethingElseWhileTheSpecDoesNot(t *testing.T) {
	eng := specWorld(t, specEDB)
	// Func says fp200, confirmed fp100, and its section is as it was: drift. Kernel
	// is as confirmed on both of its links. A link whose section is gone has no
	// section to be the same as, so it is dangling, not drifted.
	wantRows(t, eng, "spec_drift", "Docs/guide.md:Guide > Install|internal/core.Func")
	wantRows(t, eng, "spec_drift_since", "Docs/guide.md:Guide > Install|internal/core.Func|1500")
}

func TestSpecStaleCodeIsTheSectionSayingSomethingElseWhileTheCodeDoesNot(t *testing.T) {
	eng := specWorld(t, specEDB)
	// The notes section says fp300, confirmed fp250, and Kernel and the header
	// are as confirmed: the spec moved and the code did not follow.
	wantRows(t, eng, "spec_stale_code",
		"Docs/notes.md:Notes|internal/core.Kernel",
		"Docs/notes.md:Notes|internal/core/foo.go:header",
	)
	wantRows(t, eng, "spec_stale_since", "Docs/notes.md:Notes|internal/core.Kernel|1600")
}

func TestNeitherSideMovingOrBothMovingIsNotDriftNorStale(t *testing.T) {
	eng := specWorld(t, `
spec_section("Docs/a.md:A", "Docs/a.md", "A", "s2").
spec_element("pkg.Both", "pkg", "pkg/x.go", /function, /true, "c2").
spec_element("pkg.Same", "pkg", "pkg/x.go", /function, /true, "c1").
spec_link("Docs/a.md:A", "pkg.Both", /cites, /doc_citation).
spec_link("Docs/a.md:A", "pkg.Same", /cites, /doc_citation).
spec_confirmed("Docs/a.md:A", "pkg.Both", "s1", "c1", 1).
spec_confirmed("Docs/a.md:A", "pkg.Same", "s2", "c1", 1).
`)
	// Both moved: the scan confirms the pair again before the kernel sees it
	// (specs.Reconcile), so a row like this one is judged neither way.
	wantRows(t, eng, "spec_drift")
	wantRows(t, eng, "spec_stale_code")
}

func TestALinkNeverConfirmedHasNoDrift(t *testing.T) {
	eng := specWorld(t, `
spec_section("Docs/a.md:A", "Docs/a.md", "A", "s1").
spec_element("pkg.F", "pkg", "pkg/x.go", /function, /true, "c1").
spec_link("Docs/a.md:A", "pkg.F", /cites, /doc_citation).
`)
	wantRows(t, eng, "spec_drift")
	wantRows(t, eng, "spec_stale_code")
}

func TestDriftNamesTheLatestCommitSinceTheLinkWasConfirmed(t *testing.T) {
	eng := specWorld(t, specEDB+`
spec_element("internal/core.Other", "internal/core", "internal/core/bar.go", /function, /true, "fp1").
git_history("internal/core/foo.go", "aaa111", "ann", 900, "before the link was confirmed").
git_history("internal/core/foo.go", "bbb222", "ann", 1200, "first change since").
git_history("internal/core/foo.go", "ccc333", "ann", 1400, "latest change").
git_history("internal/core/bar.go", "ddd444", "bob", 1450, "another file").
`)
	// Only commits to the drifted element's file at or after the confirmation count.
	wantRows(t, eng, "spec_drift_commit", "Docs/guide.md:Guide > Install|internal/core.Func|ccc333|1400")
}

func TestDriftHasNoCommitWhenTheChangeIsNotCommitted(t *testing.T) {
	eng := specWorld(t, specEDB+`
git_history("internal/core/foo.go", "aaa111", "ann", 900, "before the link was confirmed").
`)
	wantRows(t, eng, "spec_drift", "Docs/guide.md:Guide > Install|internal/core.Func")
	wantRows(t, eng, "spec_drift_commit")
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
spec_section("Docs/a.md:A", "Docs/a.md", "A", "fp1").
spec_element("pkg/x.go:header", "pkg", "pkg/x.go", /header, /true, "fp1").
spec_element("pkg.F", "pkg", "pkg/x.go", /function, /true, "fp1").
spec_link("Docs/a.md:A", "pkg/x.go:header", /cites, /doc_citation).
`)
	wantRows(t, eng, "spec_coverage", "pkg|0|1")
	wantRows(t, eng, "unspecced_element", "pkg.F")
}

const requirementEDB = `
spec_element("internal/core.Kernel.Eval", "internal/core", "internal/core/foo.go", /method, /true, "fp200").
spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, "fp200").
spec_element("internal/core.helper", "internal/core", "internal/core/foo.go", /function, /false, "fp200").
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
spec_element("internal/core.Func", "internal/core", "internal/core/foo.go", /function, /true, "fp1").
spec_element_property("internal/core.Func", /has_test).
spec_requirement(/ok, "fine").
requires(/ok, "internal/core.Func", /exists).
requires(/ok, "internal/core.Func", /exported).
requires(/ok, "internal/core.Func", /has_test).
`)
	wantRows(t, eng, "requirement_violated")
}
