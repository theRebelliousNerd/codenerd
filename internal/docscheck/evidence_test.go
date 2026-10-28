package docscheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// evWorkspace writes files (workspace-relative path -> content) into a fresh
// temp repository, stages them, and returns a Checker over it.
func evWorkspace(t *testing.T, files map[string]string) (*Checker, string) {
	t.Helper()
	root := t.TempDir()
	writeEv(t, root, files)
	gitInitAdd(t, root)
	return NewChecker(root), root
}

func writeEv(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fm(status string) string {
	return "---\ndoc-class: deep-dive\nimplementation-status: " + status + "\nlast-verified: 2026-10-28\nverified-against: abc\n---\n\n"
}

func adr(status, witness string) string {
	return fm(status) + "# ADR\n\n**Witness:** " + witness + "\n"
}

// byRef returns the (reference, resolution) pair whose spelling matches.
func byRef(t *testing.T, ev Evidence, spelling string) (Reference, Resolution) {
	t.Helper()
	for i, r := range ev.References {
		if r.Spelling == spelling {
			return r, ev.Resolutions[i]
		}
	}
	t.Fatalf("no reference spelled %q in %+v", spelling, ev.References)
	return Reference{}, Resolution{}
}

func evidenceOf(t *testing.T, c *Checker, pkgs ...string) Evidence {
	t.Helper()
	ev, err := c.Evidence(pkgs)
	if err != nil {
		t.Fatalf("Evidence: %v", err)
	}
	return ev
}

const (
	pkgA = "Docs/architecture/a/"
	pkgB = "Docs/architecture/b/"
)

// A planned capability cites the real code where it would attach. The
// reference and its resolution are recorded; nothing says the code realizes
// the capability, in the records or in the manifest's list of what the view
// does not produce.
func TestEvidencePlannedSeamIsReferenceNotRealization(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "05-SPEC.md":     fm("target-state") + "# Spec\n\nThe gate attaches at `internal/exec/exec.go:3-9` (`internal/exec/exec.go#Run`).\n",
		"internal/exec/exec.go": "package exec\n\nfunc Run() {}\n",
	})
	ev := evidenceOf(t, c, "a")
	ref, res := byRef(t, ev, "internal/exec/exec.go:3-9")
	if ref.Role != RolePlannedSeam || res.Grade != GradeFilePresent {
		t.Fatalf("seam citation: role %s grade %s", ref.Role, res.Grade)
	}
	_, sym := byRef(t, ev, "internal/exec/exec.go#Run")
	if sym.Grade != GradeDeclared || sym.Entity != "internal/exec.Run" {
		t.Fatalf("path#symbol: %+v", sym)
	}
	var out bytes.Buffer
	if err := WriteEvidenceJSON(&out, ev); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"realiz", "implemented", "aligned", "passed"} {
		if strings.Contains(strings.ToLower(out.String()), banned) {
			t.Errorf("evidence output asserts %q:\n%s", banned, out.String())
		}
	}
	want := map[string]bool{"relation_claim": true, "relation_disposition": true, "acceptance_obligation": true, "execution_receipt": true}
	for _, n := range ev.Manifest.NotObserved {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Errorf("manifest does not disclose unproduced families: %v", want)
	}
}

// A known-gap paragraph, a rejected alternative, and a heading that says the
// same cite existing symbols. They keep their text-derived role, and none of
// them can be a required resolution or a realization.
func TestEvidenceNegativeAndHistoricalCitationsKeepTheirRole(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "02-CURRENT.md": fm("shipped") + "# Current\n\n" +
			"This is not yet implemented in `internal/old/gone.go`.\n" +
			"The old design was removed; see `internal/old/legacy.go`.\n\n" +
			"## Rejected alternatives\n\nWe considered `internal/old/alt.go`.\n\n" +
			"## Design\n\nThe live path is `internal/old/live.go`.\n",
		"internal/old/live.go": "package old\n",
	})
	ev := evidenceOf(t, c, "a")
	roles := map[string]Role{
		"internal/old/gone.go": RoleNegative, "internal/old/legacy.go": RoleHistorical,
		"internal/old/alt.go": RoleHistorical, "internal/old/live.go": RoleCited,
	}
	for spelling, want := range roles {
		if ref, _ := byRef(t, ev, spelling); ref.Role != want {
			t.Errorf("%s: role %s, want %s", spelling, ref.Role, want)
		}
	}
	// gone/legacy/alt do not exist and are exempt; live exists. Nothing fails.
	if ev.Manifest.Failing != 0 || !ev.Clean() {
		t.Fatalf("exempt roles must not fail the scan: %+v", ev.Manifest)
	}
}

// Fenced examples are not citations.
func TestEvidenceIgnoresFencedBlocksAndFrontMatter(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "05-SPEC.md": fm("shipped") + "# S\n\n```\n`internal/fake/example.go`\n```\n",
	})
	if ev := evidenceOf(t, c, "a"); len(ev.References) != 0 {
		t.Fatalf("fenced example became a reference: %+v", ev.References)
	}
}

// A spelling that occurs only in a comment, a string, a use site, or a rule
// body is retained as text and never graded as a declaration.
func TestEvidenceTextOccurrenceIsNotDeclaration(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:OnlyInComment"),
		pkgA + "adr/ADR-002.md": adr("shipped", "symbol:OnlyInString"),
		pkgA + "adr/ADR-003.md": adr("shipped", "symbol:OnlyUsed"),
		pkgA + "adr/ADR-004.md": adr("shipped", "predicate:only_in_body"),
		pkgA + "adr/ADR-005.md": adr("shipped", "predicate:really_declared"),
		"p/p.go":                "package p\n\n// OnlyInComment is mentioned here.\nvar s = \"OnlyInString\"\n\nfunc f() { OnlyUsed() }\n",
		"p/rules.mg":            "Decl really_declared(X).\nonly_in_body(X) :- really_declared(X).\n",
	})
	ev := evidenceOf(t, c, "a")
	for sp, want := range map[string]Grade{
		"OnlyInComment": GradeOccurrenceOnly, "OnlyInString": GradeOccurrenceOnly,
		"OnlyUsed": GradeOccurrenceOnly, "only_in_body": GradeOccurrenceOnly,
		"really_declared": GradeDeclared,
	} {
		if _, res := byRef(t, ev, sp); res.Grade != want {
			t.Errorf("%s: grade %s, want %s (%s)", sp, res.Grade, want, res.Detail)
		}
	}
	if ev.Clean() {
		t.Fatal("occurrence-only witnesses must not leave the scan clean")
	}
}

// Identically named declarations in different packages are ambiguous, with
// every candidate listed; a package-qualified spelling picks one.
func TestEvidenceAmbiguousNamesYieldCandidates(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:Shared"),
		pkgA + "adr/ADR-002.md": adr("shipped", "symbol:one.Shared"),
		pkgA + "adr/ADR-003.md": adr("shipped", "symbol:internal/two.Shared"),
		"internal/one/one.go":   "package one\n\nfunc Shared() {}\n",
		"internal/two/two.go":   "package two\n\nfunc Shared() {}\n",
	})
	ev := evidenceOf(t, c, "a")
	_, amb := byRef(t, ev, "Shared")
	if amb.Grade != GradeAmbiguous || len(amb.Candidates) != 2 || amb.Entity != "" {
		t.Fatalf("bare name: %+v", amb)
	}
	if _, q := byRef(t, ev, "one.Shared"); q.Grade != GradeDeclared || q.Entity != "internal/one.Shared" {
		t.Fatalf("pkg-qualified: %+v", q)
	}
	if _, q := byRef(t, ev, "internal/two.Shared"); q.Grade != GradeDeclared || q.Entity != "internal/two.Shared" {
		t.Fatalf("dir-qualified: %+v", q)
	}
	// Text output keeps the candidates a reader needs.
	if txt := EvidenceText(ev); !strings.Contains(txt, "internal/one/one.go") || !strings.Contains(txt, "internal/two/two.go") {
		t.Fatalf("text report dropped candidates:\n%s", txt)
	}
}

// A test witness is a _test.go function with a signature go test runs. A
// look-alike in production code, a wrong signature, or only a comment is not
// a test identity, and a constrained one says so.
func TestEvidenceTestIdentityIsSeparateFromTextMatch(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "test:TestReal"),
		pkgA + "adr/ADR-002.md": adr("shipped", "test:TestInProd"),
		pkgA + "adr/ADR-003.md": adr("shipped", "test:TestWrongSig"),
		pkgA + "adr/ADR-004.md": adr("shipped", "test:TestOnlyLinux"),
		pkgA + "adr/ADR-005.md": adr("shipped", "test:TestNowhereAtAll"),
		"x/x_test.go":           "package x\n\nimport \"testing\"\n\nfunc TestReal(t *testing.T) {}\n\nfunc TestWrongSig(n int) {}\n",
		"x/linux_test.go":       "//go:build linux\n\npackage x\n\nimport \"testing\"\n\nfunc TestOnlyLinux(t *testing.T) {}\n",
		"x/prod.go":             "package x\n\nimport \"testing\"\n\nfunc TestInProd(t *testing.T) {}\n",
	})
	ev := evidenceOf(t, c, "a")
	for sp, want := range map[string]Grade{
		"TestReal": GradeTestDeclared, "TestInProd": GradeOccurrenceOnly, "TestWrongSig": GradeOccurrenceOnly,
		"TestOnlyLinux": GradeTestDeclared, "TestNowhereAtAll": GradeUnresolved,
	} {
		if _, res := byRef(t, ev, sp); res.Grade != want {
			t.Errorf("%s: grade %s, want %s", sp, res.Grade, want)
		}
	}
	if _, res := byRef(t, ev, "TestOnlyLinux"); !res.Constrained {
		t.Error("build-constrained test not flagged")
	}
}

// Same test name in two packages: neither discharges the other.
func TestEvidenceSameTestNameInTwoPackagesIsAmbiguous(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "test:TestSame"),
		"p/a_test.go":           "package p\n\nimport \"testing\"\n\nfunc TestSame(t *testing.T) {}\n",
		"q/a_test.go":           "package q\n\nimport \"testing\"\n\nfunc TestSame(t *testing.T) {}\n",
	})
	_, res := byRef(t, evidenceOf(t, c, "a"), "TestSame")
	if res.Grade != GradeAmbiguous || len(res.Candidates) != 2 {
		t.Fatalf("%+v", res)
	}
}

// An accepted-not-implemented ADR need not resolve; the same unresolved
// witness on a shipped ADR fails the scan. Several witnesses on one line are
// each their own reference.
func TestEvidencePlannedWitnessIsExemptAndMultiWitnessLinesSplit(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("accepted-not-implemented", "`predicate:later_pred/2` and `test:TestLater`"),
	})
	ev := evidenceOf(t, c, "a")
	if len(ev.References) != 2 {
		t.Fatalf("want two witnesses, got %+v", ev.References)
	}
	for _, r := range ev.References {
		if r.Role != RolePlannedWitness {
			t.Errorf("%s role %s", r.Spelling, r.Role)
		}
	}
	if !ev.Clean() {
		t.Fatalf("planned witnesses must not fail: %+v", ev.Manifest)
	}

	c2, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "test:TestLater"),
	})
	if ev := evidenceOf(t, c2, "a"); ev.Clean() || ev.Manifest.Failing != 1 {
		t.Fatalf("shipped unresolved witness must fail: %+v", ev.Manifest)
	}
}

// Any change to any input changes the generation; identical inputs do not.
func TestEvidenceGenerationTracksEveryInput(t *testing.T) {
	files := map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:Thing"),
		"p/p.go":                "package p\n\nfunc Thing() {}\n",
	}
	c, root := evWorkspace(t, files)
	g0 := evidenceOf(t, c, "a").Manifest.Generation
	if g1 := evidenceOf(t, NewChecker(root), "a").Manifest.Generation; g1 != g0 {
		t.Fatal("same inputs, different generation")
	}
	// Code edit, uncommitted: a commit id would not see it; the digest does.
	writeEv(t, root, map[string]string{"p/p.go": "package p\n\nfunc Thing() {}\n\n// edit\n"})
	g2 := evidenceOf(t, NewChecker(root), "a").Manifest.Generation
	if g2 == g0 {
		t.Fatal("code edit did not change the generation")
	}
	writeEv(t, root, map[string]string{pkgA + "adr/ADR-001.md": adr("shipped", "symbol:Thing") + "\nmore\n"})
	if g3 := evidenceOf(t, NewChecker(root), "a").Manifest.Generation; g3 == g2 {
		t.Fatal("document edit did not change the generation")
	}
}

// Scope is part of the result: a named subset is never labelled workspace,
// and an empty domain is unknown rather than clean.
func TestEvidenceScopeAndEmptyDomain(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "02-CURRENT.md": fm("shipped") + "# a\n",
		pkgB + "marker.txt":    "not markdown\n",
	})
	if ev := evidenceOf(t, c); ev.Manifest.Scope != ScopeWorkspace || len(ev.Manifest.Requested) != 2 {
		t.Fatalf("whole tree: %+v", ev.Manifest)
	}
	sub := evidenceOf(t, c, "a")
	if sub.Manifest.Scope != ScopeSubset || sub.Manifest.Terminal != TerminalComplete || !sub.Clean() {
		t.Fatalf("subset: %+v", sub.Manifest)
	}
	empty := evidenceOf(t, c, "b")
	if empty.Manifest.Terminal != TerminalEmpty || empty.Clean() {
		t.Fatalf("package with no documents must be unknown, not clean: %+v", empty.Manifest)
	}
	if txt := EvidenceText(empty); !strings.Contains(txt, "UNKNOWN") {
		t.Fatalf("text report must say unknown:\n%s", txt)
	}
	if _, err := c.Evidence([]string{"nope"}); err == nil {
		t.Fatal("unknown package must be an error, not an empty report")
	}
}

// A Go file that does not parse could hold the declaration being looked for,
// so a miss is unobserved, the scan is incomplete, and it is not clean.
func TestEvidenceUnparsedSourceMakesAbsenceUnobserved(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:Hidden"),
		"p/broken.go":           "package p\n\nfunc Broken( {\n",
	})
	ev := evidenceOf(t, c, "a")
	if _, res := byRef(t, ev, "Hidden"); res.Grade != GradeUnobserved {
		t.Fatalf("grade %s, want unobserved", res.Grade)
	}
	if ev.Manifest.Terminal != TerminalIncomplete || ev.Manifest.CodeFilesFailed != 1 || ev.Clean() {
		t.Fatalf("manifest: %+v", ev.Manifest)
	}
	found := false
	for _, d := range ev.Manifest.Deficits {
		found = found || (d.Code == "go_parse_failed" && d.File == "p/broken.go")
	}
	if !found {
		t.Fatalf("parse failure not listed: %+v", ev.Manifest.Deficits)
	}
}

// Outside a git repository the tracked source cannot be enumerated: path
// citations still stat, code-target witnesses are unobserved, and the scan
// is incomplete.
func TestEvidenceWithoutGitRepositoryIsIncomplete(t *testing.T) {
	root := t.TempDir()
	writeEv(t, root, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:Thing"),
		pkgA + "02-CURRENT.md":  fm("shipped") + "# c\n\nSee `internal/x/x.go`.\n",
		"internal/x/x.go":       "package x\n",
	})
	ev := evidenceOf(t, NewChecker(root), "a")
	if _, res := byRef(t, ev, "Thing"); res.Grade != GradeUnobserved {
		t.Fatalf("symbol grade %s", res.Grade)
	}
	if _, res := byRef(t, ev, "internal/x/x.go"); res.Grade != GradeFilePresent {
		t.Fatalf("path grade %s", res.Grade)
	}
	if ev.Manifest.Terminal != TerminalIncomplete {
		t.Fatalf("terminal %s", ev.Manifest.Terminal)
	}
}

// Fixture and crash-dump directories are not the corpus: a declaration that
// exists only under testdata does not resolve a citation.
func TestEvidenceSkipsFixtureDirectories(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "symbol:FixtureOnly"),
		"p/testdata/fx.go":      "package fx\n\nfunc FixtureOnly() {}\n",
	})
	if _, res := byRef(t, evidenceOf(t, c, "a"), "FixtureOnly"); res.Grade.satisfies() {
		t.Fatalf("testdata declaration resolved a citation: %+v", res)
	}
}

// JSON output: reference then resolution, manifest last, every object typed.
func TestEvidenceJSONShape(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		pkgA + "adr/ADR-001.md": adr("shipped", "file:marker.txt"),
		"marker.txt":            "x\n",
	})
	var out bytes.Buffer
	if err := WriteEvidenceJSON(&out, evidenceOf(t, c, "a")); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var o map[string]any
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		kinds = append(kinds, o["record"].(string))
	}
	if strings.Join(kinds, ",") != "reference,resolution,manifest" {
		t.Fatalf("record order %v", kinds)
	}
}

// The structural grade is unchanged by the evidence view: the same clean
// fixture still has zero problems, and its witness still resolves lexically.
func TestEvidenceLeavesStructuralGradeAlone(t *testing.T) {
	c := NewChecker(fixtureRoot(t, "clean"))
	rep, err := c.CheckPackage("pkg")
	if err != nil || len(rep.Problems) != 0 {
		t.Fatalf("clean fixture: %v %+v", err, rep.Problems)
	}
}

// The word index must answer exactly as the line-by-line regexes it
// replaced, including near-misses that share a prefix, a suffix or an
// underscore with the name.
func TestWitnessWordIndexMatchesLineScan(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{
		"a.go": "package a\n\nvar foo_bar = 1\nvar xfoo = 2\nvar fooX = 3\n// call Target(x)\nfunc Target2() {}\n",
		"b.go": "package b\n\nfunc Exact() {}\n\nfunc (r R) Method() {}\n",
		"r.mg": "ghost_pred (X) :- p(X).\nnear_pred_x(X) :- p(X).\n",
	})
	cases := []string{
		"symbol:foo", "symbol:foo_bar", "symbol:xfoo", "symbol:fooX", "symbol:Target", "symbol:Target2",
		"symbol:Exact", "symbol:R.Method", "symbol:Method", "symbol:a.Exact", "symbol:Missing",
		"test:Exact", "test:Target2", "test:Method", "test:Target",
		"predicate:ghost_pred", "predicate:near_pred", "predicate:near_pred_x", "predicate:p",
	}
	for _, w := range cases {
		kind, val, _ := strings.Cut(w, ":")
		var want bool
		for _, f := range []struct{ ext, body string }{
			{".go", mustRead(t, c.root, "a.go") + mustRead(t, c.root, "b.go")},
			{".mg", mustRead(t, c.root, "r.mg")},
		} {
			if (kind == "predicate") != (f.ext == ".mg") {
				continue
			}
			for _, line := range strings.Split(f.body, "\n") {
				switch kind {
				case "symbol":
					want = want || regexp.MustCompile(`\b`+regexp.QuoteMeta(val)+`\b`).MatchString(line)
				case "test":
					want = want || testDeclOnLine(line, val)
				case "predicate":
					want = want || regexp.MustCompile(`\b`+regexp.QuoteMeta(val)+`\s*\(`).MatchString(line)
				}
			}
		}
		if got := resolveWitness(t, c, w); got != want {
			t.Errorf("%s: word index says %v, line scan says %v", w, got, want)
		}
	}
}

// Many witnesses read each tracked file once: the word index is built a
// single time per extension no matter how many witnesses are asked.
func TestWitnessSearchReadsTrackedFilesOnce(t *testing.T) {
	c, _ := evWorkspace(t, map[string]string{"a.go": "package a\n\nfunc One() {}\nfunc Two() {}\n"})
	for _, w := range []string{"symbol:One", "symbol:Two", "symbol:Three", "test:One"} {
		resolveWitness(t, c, w)
	}
	first := c.words[".go"]
	resolveWitness(t, c, "symbol:One")
	resolveWitness(t, c, "symbol:Four")
	if len(first) != 1 || len(c.words[".go"]) != 1 || &first[0] != &c.words[".go"][0] {
		t.Fatalf("word index rebuilt: %v", c.words)
	}
}

func mustRead(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
