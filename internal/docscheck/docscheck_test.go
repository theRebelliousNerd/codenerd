package docscheck

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// fixtureRoot locates a testdata fixture tree by name. Every fixture is a
// miniature workspace root holding Docs/architecture/pkg plus whatever the
// defect needs (the clean tree's file: witness target marker.txt included).
func fixtureRoot(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("testdata", name)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Fatalf("fixture %q missing: run the fixture build first", name)
	}
	return root
}

// wantProblem builds the expected Problem for package "pkg".
func wantProblem(file string, code Code, message string) Problem {
	return Problem{Package: "pkg", File: file, Code: code, Message: message}
}

func checkProblems(t *testing.T, fixture string, got, want []Problem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d problems, want %d:\n got: %#v\nwant: %#v", fixture, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: problem %d:\n got: %#v\nwant: %#v", fixture, i, got[i], want[i])
		}
	}
}

// TestCheckPackage_Fixtures pins every check: one clean package with zero
// problems, one fixture per problem code with exactly that problem (full
// struct: script-identical message plus workspace-relative file), and the
// accepted-not-implemented witness exemption.
func TestCheckPackage_Fixtures(t *testing.T) {
	const (
		cur = "Docs/architecture/pkg/02-CURRENT-STATE.md"
		gap = "Docs/architecture/pkg/03-GAP-ANALYSIS.md"
		adr = "Docs/architecture/pkg/adr/ADR-001-example.md"
		dir = "Docs/architecture/pkg"
	)
	tests := []struct {
		fixture string
		files   int
		want    []Problem
	}{
		{fixture: "clean", files: 13, want: nil},
		{fixture: "missing_front_matter", files: 13, want: []Problem{
			wantProblem(cur, CodeMissingFrontMatter, "02-CURRENT-STATE.md: no front-matter"),
		}},
		{fixture: "bad_doc_class", files: 13, want: []Problem{
			wantProblem(cur, CodeBadDocClass, "02-CURRENT-STATE.md: doc-class 'essay'"),
		}},
		{fixture: "bad_implementation_status", files: 13, want: []Problem{
			wantProblem(cur, CodeBadImplementationStatus, "02-CURRENT-STATE.md: implementation-status 'draft'"),
		}},
		{fixture: "bad_last_verified", files: 13, want: []Problem{
			wantProblem(cur, CodeBadLastVerified, "02-CURRENT-STATE.md: last-verified missing/malformed"),
		}},
		{fixture: "missing_verified_against", files: 13, want: []Problem{
			wantProblem(cur, CodeMissingVerifiedAgainst, "02-CURRENT-STATE.md: verified-against missing"),
		}},
		{fixture: "missing_gap_table", files: 13, want: []Problem{
			wantProblem(gap, CodeMissingGapTable, "03-GAP-ANALYSIS.md: no gap table with ID and exit columns"),
		}},
		{fixture: "empty_gap_table", files: 13, want: []Problem{
			wantProblem(gap, CodeEmptyGapTable, "03-GAP-ANALYSIS.md: gap table is empty"),
		}},
		{fixture: "gap_row_without_id", files: 13, want: []Problem{
			wantProblem(gap, CodeGapRowWithoutID, "03-GAP-ANALYSIS.md: row without a gap ID: 'the first gap'"),
		}},
		{fixture: "vague_gap_exit", files: 13, want: []Problem{
			wantProblem(gap, CodeVagueGapExit, "03-GAP-ANALYSIS.md: GAP-TEST-01 has no checkable exit criterion"),
		}},
		{fixture: "no_witness_line", files: 13, want: []Problem{
			wantProblem(adr, CodeNoWitnessLine, "adr/ADR-001-example.md: no **Witness:** line"),
		}},
		{fixture: "witness_unresolved", files: 13, want: []Problem{
			wantProblem(adr, CodeWitnessUnresolved, "adr/ADR-001-example.md: witness does not resolve: test:TestDocscheckFixtureNoSuchThingZZZ9"),
		}},
		{fixture: "missing_slot", files: 12, want: []Problem{
			wantProblem("Docs/architecture/pkg/TODO.md", CodeMissingSlot, "missing slot TODO.md"),
		}},
		{fixture: "missing_adr_dir", files: 12, want: []Problem{
			wantProblem("Docs/architecture/pkg/adr", CodeMissingADRDir, "missing adr/"),
		}},
		{fixture: "missing_capability_spec", files: 12, want: []Problem{
			wantProblem(dir, CodeMissingCapabilitySpec, "no capability spec (05-... onward)"),
		}},
		{fixture: "no_plan_layer", files: 13, want: []Problem{
			wantProblem(dir, CodeNoPlanLayer, "no plan layer: no file is planned/target-state"),
		}},
		{fixture: "only_planned", files: 13, want: []Problem{
			wantProblem(dir, CodeOnlyPlanned, "no shipped layer"),
		}},
		{fixture: "witness_exempt", files: 13, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			rep, err := NewChecker(fixtureRoot(t, tt.fixture)).CheckPackage("pkg")
			if err != nil {
				t.Fatalf("CheckPackage: %v", err)
			}
			if rep.Package != "pkg" {
				t.Errorf("Package = %q, want %q", rep.Package, "pkg")
			}
			if rep.Files != tt.files {
				t.Errorf("Files = %d, want %d", rep.Files, tt.files)
			}
			if rep.Count() != len(tt.want) {
				t.Errorf("Count = %d, want %d", rep.Count(), len(tt.want))
			}
			checkProblems(t, tt.fixture, rep.Problems, tt.want)
		})
	}
}

// TestReportText pins the human report byte for byte against the script's
// "{pkg:<16}{n:>4} md  {len:>3} problems" shape: "pkg" padded to 16, then
// "  13", " md  ", "  1", " problems".
func TestReportText(t *testing.T) {
	rep, err := NewChecker(fixtureRoot(t, "bad_doc_class")).CheckPackage("pkg")
	if err != nil {
		t.Fatalf("CheckPackage: %v", err)
	}
	want := "pkg               13 md    1 problems\n" +
		"    02-CURRENT-STATE.md: doc-class 'essay'\n"
	if got := rep.Text(); got != want {
		t.Errorf("Text:\n got: %q\nwant: %q", got, want)
	}
	if got := Text([]PackageReport{rep}); got != want {
		t.Errorf("Text(reports):\n got: %q\nwant: %q", got, want)
	}
	if got := Text(nil); got != "" {
		t.Errorf("Text(nil) = %q, want empty", got)
	}
}

// TestWriteJSON pins the --json shape: one problem object per line in report
// order, then a summary object carrying counts instead of a code.
func TestWriteJSON(t *testing.T) {
	rep, err := NewChecker(fixtureRoot(t, "bad_doc_class")).CheckPackage("pkg")
	if err != nil {
		t.Fatalf("CheckPackage: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, []PackageReport{rep}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2:\n%s", len(lines), buf.String())
	}
	var prob map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &prob); err != nil {
		t.Fatalf("problem line is not JSON: %v", err)
	}
	for k, want := range map[string]any{
		"package": "pkg",
		"file":    "Docs/architecture/pkg/02-CURRENT-STATE.md",
		"code":    "bad_doc_class",
		"message": "02-CURRENT-STATE.md: doc-class 'essay'",
	} {
		if prob[k] != want {
			t.Errorf("problem[%q] = %v, want %v", k, prob[k], want)
		}
	}
	var sum map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &sum); err != nil {
		t.Fatalf("summary line is not JSON: %v", err)
	}
	for k, want := range map[string]any{"packages": 1.0, "files": 13.0, "problems": 1.0} {
		if sum[k] != want {
			t.Errorf("summary[%q] = %v, want %v", k, sum[k], want)
		}
	}
	if _, ok := sum["code"]; ok {
		t.Errorf("summary carries a code key, so it is not distinguishable from a problem: %v", sum)
	}
	if got := Summarize([]PackageReport{rep}); got != (Summary{Packages: 1, Files: 13, Problems: 1}) {
		t.Errorf("Summarize = %+v", got)
	}
}

// TestProblemFact pins the kernel surface: doc_problem(Package, File, Code,
// Message) with the code as a /name, and every argument surviving ToAtom.
func TestProblemFact(t *testing.T) {
	p := Problem{
		Package: "pkg",
		File:    "Docs/architecture/pkg/adr/ADR-001-example.md",
		Code:    CodeNoWitnessLine,
		Message: "adr/ADR-001-example.md: no **Witness:** line",
	}
	f := p.Fact()
	if f.Predicate != "doc_problem" {
		t.Errorf("Predicate = %q, want doc_problem", f.Predicate)
	}
	if len(f.Args) != 4 {
		t.Fatalf("Args has %d entries, want 4", len(f.Args))
	}
	if got := types.ExtractString(f.Args[0]); got != "pkg" {
		t.Errorf("arg 0 = %q, want pkg", got)
	}
	if got, ok := f.Args[2].(types.MangleAtom); !ok || got != "/no_witness_line" {
		t.Errorf("arg 2 = %#v, want MangleAtom /no_witness_line", f.Args[2])
	}
	if _, err := f.ToAtom(); err != nil {
		t.Errorf("ToAtom: %v", err)
	}
	rep, err := NewChecker(fixtureRoot(t, "bad_doc_class")).CheckPackage("pkg")
	if err != nil {
		t.Fatalf("CheckPackage: %v", err)
	}
	if facts := rep.Facts(); len(facts) != 1 || facts[0].Predicate != "doc_problem" {
		t.Errorf("report Facts = %#v", facts)
	}
	if facts := Facts(nil); len(facts) != 0 {
		t.Errorf("Facts(nil) = %#v, want empty", facts)
	}
}

// TestDocProblemDeclMatchesProducer pins the Decl side of the contract
// TestProblemFact pins on the Go side. TestUndeclaredAssertBudget only
// checks the predicate name; a Decl whose bounds disagree with
// Problem.Fact fails silently (internal/mangle/agents.md), so the shape
// is pinned here, next to the producer.
func TestDocProblemDeclMatchesProducer(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(docscheckRepoRoot(t), "internal", "core", "defaults", "schemas_reviewer.mg"))
	if err != nil {
		t.Fatalf("read schemas_reviewer.mg: %v", err)
	}
	const want = "Decl doc_problem(Pkg, File, Code, Message) bound [/string, /string, /name, /string]."
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "Decl doc_problem(") {
			if trimmed != want {
				t.Errorf("Decl = %q, want %q", trimmed, want)
			}
			return
		}
	}
	t.Error("no Decl doc_problem in schemas_reviewer.mg")
}

// docscheckRepoRoot walks up from the package directory to the go.mod root.
func docscheckRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the docscheck package")
		}
		dir = parent
	}
}

// TestCheckPackage_Errors pins the fail-closed edges: unknown or escaping
// package names are errors, not empty grades.
func TestCheckPackage_Errors(t *testing.T) {
	c := NewChecker(fixtureRoot(t, "clean"))
	for _, pkg := range []string{"", ".", "..", "no-such-pkg", "a/b", `a\b`} {
		if _, err := c.CheckPackage(pkg); err == nil {
			t.Errorf("CheckPackage(%q): want error, got nil", pkg)
		}
	}
	if _, err := NewChecker(t.TempDir()).ListPackages(); err == nil {
		t.Errorf("ListPackages on a root without Docs/architecture: want error, got nil")
	}
}

// TestCheck_AllPackages covers the no-args path: every package directory,
// sorted, in one call.
func TestCheck_AllPackages(t *testing.T) {
	reports, err := NewChecker(fixtureRoot(t, "clean")).Check(nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(reports) != 1 || reports[0].Package != "pkg" || len(reports[0].Problems) != 0 {
		t.Fatalf("Check(nil) = %#v", reports)
	}
	if _, err := NewChecker(fixtureRoot(t, "clean")).Check([]string{"no-such-pkg"}); err == nil {
		t.Errorf("Check([unknown]): want error, got nil")
	}
}

// TestParity_FeaturesPackageHasZeroProblems runs the Go checker over the
// real Docs/architecture/features, which passed scripts/r6_structcheck.py on
// 2026-09-26: any problem here is a port divergence (or a docs regression
// since that date — read the problems before blaming the port).
func TestParity_FeaturesPackageHasZeroProblems(t *testing.T) {
	root := repoRoot(t)
	rep, err := NewChecker(root).CheckPackage("features")
	if err != nil {
		t.Fatalf("CheckPackage(features): %v", err)
	}
	if rep.Files == 0 {
		t.Fatalf("graded zero files: the package walk is broken, not clean")
	}
	if len(rep.Problems) != 0 {
		t.Fatalf("features has %d problems, want 0:\n%s", len(rep.Problems), rep.Text())
	}
}

// repoRoot walks up from the test's working directory to go.mod, as the
// fact-conventions guard does.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot determine working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repository root (go.mod) not found")
		}
		dir = parent
	}
}

func TestFrontMatter(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		want   map[string]string
		wantOK bool
	}{
		{"valid", "---\ndoc-class: shipped\nimplementation-status: shipped\nlast-verified: 2026-09-26\nverified-against: abc\n---\n\nbody\n",
			map[string]string{"doc-class": "shipped", "implementation-status": "shipped", "last-verified": "2026-09-26", "verified-against": "abc"}, true},
		{"no opening fence", "# body\n", nil, false},
		{"no closing fence", "---\ndoc-class: shipped\n", nil, false},
		{"value keeps its colon", "---\nverified-against: a:b:c\n---\n", map[string]string{"verified-against": "a:b:c"}, true},
		{"sides stripped", "---\n  doc-class  :  shipped  \n---\n", map[string]string{"doc-class": "shipped"}, true},
		{"line without colon skipped", "---\nnot a pair\ndoc-class: shipped\n---\n", map[string]string{"doc-class": "shipped"}, true},
		{"duplicate key keeps last", "---\ndoc-class: shipped\ndoc-class: essay\n---\n", map[string]string{"doc-class": "essay"}, true},
		{"empty", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := frontMatter(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestGapRows(t *testing.T) {
	t.Run("swapped columns map by header", func(t *testing.T) {
		rows, ok := gapRows("| Exit | Gap ID |\n|---|---|\n| ship it | GAP-SW-1 |\n")
		if !ok || len(rows) != 1 || rows[0] != (gapRow{id: "GAP-SW-1", exit: "ship it"}) {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
	t.Run("first table wins only when it has both columns", func(t *testing.T) {
		rows, ok := gapRows("| Name | Value |\n|---|---|\n| a | b |\n\n| Gap ID | Exit |\n|---|---|\n| GAP-T-2 | test passes |\n")
		if !ok || len(rows) != 1 || rows[0].id != "GAP-T-2" {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
	t.Run("same cell can hold both headers", func(t *testing.T) {
		rows, ok := gapRows("| Gap ID and exit |\n|---|\n| GAP-T-3 |\n")
		if !ok || len(rows) != 1 || rows[0].id != "GAP-T-3" || rows[0].exit != "GAP-T-3" {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
	t.Run("short rows skipped, table ends at first non-pipe line", func(t *testing.T) {
		rows, ok := gapRows("| Gap ID | Exit |\n|---|---|\n| GAP-T-4 |\n| GAP-T-5 | test passes |\nprose\n| GAP-T-6 | too late |\n")
		if !ok || len(rows) != 1 || rows[0].id != "GAP-T-5" {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
	t.Run("header at end of file is an empty table, not a crash", func(t *testing.T) {
		rows, ok := gapRows("prose\n| Gap ID | Exit |")
		if !ok || len(rows) != 0 {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
	t.Run("no table", func(t *testing.T) {
		if rows, ok := gapRows("no pipes here\n"); ok || rows != nil {
			t.Fatalf("got %#v, %v", rows, ok)
		}
	})
}

// TestVagueExit pins the VAGUE pattern's accept/reject edge. The empty exit
// is rejected by the caller's explicit check, not the pattern — both halves
// are asserted here.
func TestVagueExit(t *testing.T) {
	vague := []string{"Done", "done", "DONE", "TBD", "todo", "n/a", "N/A", "NA",
		"  none  ", "improve", "improved", "robust", "complete", "completed", "works", "work"}
	for _, s := range vague {
		if !vagueExitRe.MatchString(s) {
			t.Errorf("%q: want vague, got checkable", s)
		}
	}
	checkable := []string{"", "go test ./... passes", "Done, all green", "undone",
		"nearly done", "robustness review by the owner", "TBDs are tracked in TODO.md"}
	for _, s := range checkable {
		if vagueExitRe.MatchString(s) {
			t.Errorf("%q: want checkable, got vague", s)
		}
	}
	c := NewChecker(t.TempDir())
	probs := c.checkGapTable("pkg", "03-GAP-ANALYSIS.md", "| Gap ID | Exit |\n|---|---|\n| GAP-X-1 |  |\n")
	if len(probs) != 1 || probs[0].Code != CodeVagueGapExit {
		t.Fatalf("empty exit: got %#v", probs)
	}
}

// TestWitnessResolves exercises every witness kind against a fake workspace
// root, so the test: / symbol: / predicate: scans never depend on the real
// repo's contents.
func TestWitnessResolves(t *testing.T) {
	root := t.TempDir()
	goSrc := "package proof\n\nfunc FixtureWitnessTarget() {}\n\nvar FixtureWitnessSymbol = 1\n"
	if err := os.WriteFile(filepath.Join(root, "proof.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rules", "proof.mg"), []byte("fixture_pred(X) :- some_fact(X).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "marker.txt"), []byte("here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInitAdd(t, root)
	c := NewChecker(root)
	tests := []struct {
		witness string
		want    bool
	}{
		{"test:FixtureWitnessTarget", true},
		{"test:NoSuchTestZZZ", false},
		{"symbol:FixtureWitnessSymbol", true},
		{"symbol:NoSuchSymbolZZZ", false},
		{"predicate:fixture_pred", true},
		{"predicate:no_such_pred_zzz", false},
		{"file:sub/marker.txt", true},
		{"file:sub/marker.txt:12", true},
		{"file:`sub/marker.txt`", true},
		{"file:does/not/exist.txt", false},
		{"frobnicate:x", false},
		{"justwords", false},
		{"test:", false},
		{"test: ", false},
	}
	for _, tt := range tests {
		// Twice: the second call must serve the identical cached answer.
		for i := 0; i < 2; i++ {
			if got := resolveWitness(t, c, tt.witness); got != tt.want {
				t.Errorf("witnessResolves(%q) = %v, want %v", tt.witness, got, tt.want)
			}
		}
	}
}

// resolveWitness fails the test when witness resolution itself errors.
// A miss is false; a workspace that cannot be listed is an error.
func resolveWitness(t *testing.T, c *Checker, w string) bool {
	t.Helper()
	got, err := c.witnessResolves(w)
	if err != nil {
		t.Fatalf("witnessResolves(%q): %v", w, err)
	}
	return got
}

// gitInitAdd makes root its own repository and stages the current tree.
// git ls-files reads the index, so a commit is not required; ignored names
// stay untracked. The repo is under t.TempDir, not the codeNERD worktree.
func gitInitAdd(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = gitCommandEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// TestWitnessMatchIsLineOriented pins the grep -E parity: a pattern whose
// whitespace straddles a newline must not resolve.
func TestWitnessMatchIsLineOriented(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "split.go"), []byte("package proof\n\nfunc\nSplitAcrossLines() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInitAdd(t, root)
	if resolveWitness(t, NewChecker(root), "test:SplitAcrossLines") {
		t.Errorf("cross-line func match resolved; grep -E would not match it")
	}
}

// TestFileWitnessConfinedToWorkspace pins the file: rules the script does
// not have: a trailing :<line> is stripped only when it is all digits, and
// an absolute path, a ".." segment, or a cleaned path that leaves the
// workspace is unresolved even when the named file exists.
func TestFileWitnessConfinedToWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "marker.txt"), []byte("in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// C exists so a grader that cuts file:C:/... at the first colon stats
	// C and reports a hit.
	if err := os.WriteFile(filepath.Join(root, "C"), []byte("drive\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	relOut, err := filepath.Rel(root, outside)
	if err != nil {
		t.Fatal(err)
	}
	c := NewChecker(root)
	tests := []struct {
		witness string
		want    bool
	}{
		{"file:sub/marker.txt", true},
		{"file:sub/marker.txt:12", true},
		{"file:./sub/marker.txt", true},
		{"file:sub\\marker.txt", true},
		{"file:sub/marker.txt:12extra", false},
		{"file:sub/marker.txt:", false},
		{"file:sub/marker.txt:12:34", true},
		{"file:sub/marker.txt:12:34:56", false},
		{"file:C:/no/such:12", false},
		{"file:" + filepath.ToSlash(outside), false},
		{"file:" + outside, false},
		{"file:" + filepath.ToSlash(relOut), false},
		{"file:sub/../marker.txt", false},
		{"file:C:/no/such", false},
		{"file::12", false},
	}
	for _, tt := range tests {
		if got := resolveWitness(t, c, tt.witness); got != tt.want {
			t.Errorf("witnessResolves(%q) = %v, want %v", tt.witness, got, tt.want)
		}
	}
}

// TestWitnessTrackedFilesOnly pins that test, symbol, and predicate
// witnesses resolve from git ls-files, not from a worktree walk. An ignored
// crash dump and an untracked file must not satisfy a witness.
func TestWitnessTrackedFilesOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	trackedGo := "package p\n\nfunc TrackedWitnessTest() {}\n\nfunc GenericWitnessTarget[T any]() {}\n\nfunc GenericMapWitnessTarget[M map[string]int]() {}\n\nvar TrackedWitnessSymbol = 1\n"
	if err := os.WriteFile(filepath.Join(root, "tracked.go"), []byte(trackedGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rules", "tracked.mg"), []byte("tracked_pred(X) :- ok(X).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("debug_program_ERROR*.mg\nignored.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "debug_program_ERROR_dump.mg"), []byte("dump_only_pred(X) :- true.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.go"), []byte("package p\n\nfunc IgnoredOnlyTest() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInitAdd(t, root)
	if err := os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package p\n\nfunc UntrackedOnlyTest() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewChecker(root)
	tests := []struct {
		witness string
		want    bool
	}{
		{"test:TrackedWitnessTest", true},
		{"test:GenericWitnessTarget", true},
		{"test:GenericMapWitnessTarget", true},
		{"symbol:TrackedWitnessSymbol", true},
		{"predicate:tracked_pred", true},
		{"predicate:dump_only_pred", false},
		{"test:IgnoredOnlyTest", false},
		{"test:UntrackedOnlyTest", false},
	}
	for _, tt := range tests {
		if got := resolveWitness(t, c, tt.witness); got != tt.want {
			t.Errorf("witnessResolves(%q) = %v, want %v", tt.witness, got, tt.want)
		}
	}
}

// TestWitnessScanRequiresGitRepo pins that a workspace which is not a git
// repository is an error, not a silent walk of every file on disk. The
// .go file declares the witness, so a worktree scan would report a hit.
func TestWitnessScanRequiresGitRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "present_test.go"), []byte("package p\n\nfunc PresentTest() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewChecker(root).witnessResolves("test:PresentTest")
	if err == nil || got {
		t.Fatalf("witnessResolves = (%v, %v), want an error and no hit", got, err)
	}
	adrDir := filepath.Join(root, "Docs", "architecture", "pkg", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := "" +
		"---\n" +
		"doc-class: governance\n" +
		"implementation-status: shipped\n" +
		"last-verified: 2026-09-26\n" +
		"verified-against: abc\n" +
		"---\n" +
		"\n" +
		"**Witness:** test:PresentTest\n"
	if err := os.WriteFile(filepath.Join(adrDir, "ADR-001.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewChecker(root).CheckPackage("pkg"); err == nil {
		t.Fatal("CheckPackage on a non-repo workspace: want error, got nil")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abcdef", 3); got != "abc" {
		t.Errorf("ascii: got %q", got)
	}
	if got := truncateRunes("ab", 40); got != "ab" {
		t.Errorf("short: got %q", got)
	}
	// Each é is two bytes but one character: [:40] counts characters.
	s := strings.Repeat("é", 50)
	if got := truncateRunes(s, 40); got != strings.Repeat("é", 40) {
		t.Errorf("multibyte: got %d runes", len([]rune(got)))
	}
}
