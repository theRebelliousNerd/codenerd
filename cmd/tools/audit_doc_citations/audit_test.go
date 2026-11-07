package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/docdecl"
)

func TestBrokenKindsAndCleanTree(t *testing.T) {
	root := writeFixture(t)

	bad, err := Audit(Options{RepoRoot: root, DocsRoot: "docs/bad"})
	if err != nil {
		t.Fatal(err)
	}
	if bad.DocsRoot != "docs/bad" {
		t.Errorf("docs root %q", bad.DocsRoot)
	}
	if bad.DocsScanned != 1 {
		t.Errorf("docs scanned %d", bad.DocsScanned)
	}
	if bad.Citations <= bad.Broken {
		t.Errorf("citations %d should exceed broken %d; valid paths were not counted", bad.Citations, bad.Broken)
	}
	assertFindings(t, badLines(), bad)

	clean, err := Audit(Options{RepoRoot: root, DocsRoot: "docs/clean"})
	if err != nil {
		t.Fatal(err)
	}
	if clean.DocsScanned != 1 {
		t.Errorf("clean docs scanned %d", clean.DocsScanned)
	}
	if clean.Citations == 0 {
		t.Fatal("clean tree recorded no citations")
	}
	if clean.Broken != 0 || len(clean.Findings) != 0 {
		for _, f := range clean.Findings {
			t.Errorf("unexpected %s:%d %s: %s: %s", f.Doc, f.Line, f.Citation, f.Kind, f.Reason)
		}
	}
	for _, kind := range []string{kindMissingPath, kindLineOutOfRange, kindSymbolNotDeclared, kindPredicateNotDeclared} {
		if _, ok := clean.ByKind[kind]; !ok {
			t.Errorf("clean by_kind missing %s", kind)
		}
	}

	empty, err := Audit(Options{RepoRoot: root, DocsRoot: "docs/empty"})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Broken != 0 || empty.DocsScanned != 0 || empty.Citations != 0 {
		t.Fatalf("empty tree: %+v", empty)
	}
}

func TestCLIExitCodes(t *testing.T) {
	root := writeFixture(t)

	var out, errb bytes.Buffer
	if code := run([]string{"-root", root, "-docs", "docs/bad"}, &out, &errb); code != 1 {
		t.Fatalf("bad exit %d\nstderr: %s\nstdout: %s", code, errb.String(), out.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr on a grade: %s", errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "summary: broken=") {
		t.Fatalf("no summary:\n%s", text)
	}
	lines := docdecl.SplitLines(text)
	if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], "summary:") {
		t.Fatalf("summary is not the last line:\n%s", text)
	}
	// Every broken citation is one line; the summary is the extra line.
	if got, want := len(lines)-1, len(wantFindings()); got != want {
		t.Fatalf("text findings %d, want %d\n%s", got, want, text)
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"-root", root, "-docs", "docs/clean"}, &out, &errb); code != 0 {
		t.Fatalf("clean exit %d\n%s\n%s", code, errb.String(), out.String())
	}
	if strings.Contains(out.String(), ": missing-path:") || strings.Contains(out.String(), ": symbol-not-declared:") {
		t.Fatalf("clean text has a finding:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "broken=0") {
		t.Fatalf("clean summary: %s", out.String())
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"-root", root, "-docs", "docs/bad", "-json"}, &out, &errb); code != 1 {
		t.Fatalf("json exit %d\n%s", code, errb.String())
	}
	var rep Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if rep.Broken != len(wantFindings()) || len(rep.Findings) != rep.Broken {
		t.Fatalf("json broken %d findings %d", rep.Broken, len(rep.Findings))
	}
	if rep.ByKind[kindMissingPath] == 0 || rep.ByKind[kindLineOutOfRange] == 0 ||
		rep.ByKind[kindSymbolNotDeclared] == 0 || rep.ByKind[kindPredicateNotDeclared] == 0 {
		t.Fatalf("json by_kind %#v", rep.ByKind)
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"-root", root, "-docs", "no/such"}, &out, &errb); code != 2 {
		t.Fatalf("missing docs exit %d stderr %s", code, errb.String())
	}
	if errb.Len() == 0 {
		t.Fatal("missing docs produced no stderr")
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"-root", root, "-docs", "docs/clean", "extra"}, &out, &errb); code != 2 {
		t.Fatalf("extra arg exit %d", code)
	}
}

func TestReportWritersPropagateErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		write  func(io.Writer, Report) error
		report Report
	}{
		{name: "text finding", write: writeText, report: Report{Findings: []Finding{{Doc: "docs/bad.md", Line: 1, Kind: kindMissingPath}}}},
		{name: "text summary", write: writeText, report: newReport()},
		{name: "json report", write: writeJSON, report: newReport()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader, writer := io.Pipe()
			t.Cleanup(func() {
				if err := writer.Close(); err != nil {
					t.Errorf("writer.Close() = %v, want nil", err)
				}
			})
			if err := reader.Close(); err != nil {
				t.Fatalf("reader.Close() = %v, want nil", err)
			}
			if err := tt.write(writer, tt.report); !errors.Is(err, io.ErrClosedPipe) {
				t.Errorf("write(closed pipe) = %v, want %v", err, io.ErrClosedPipe)
			}
		})
	}
}

func TestCLIRejectsReportWriteFailures(t *testing.T) {
	t.Parallel()
	root := writeFixture(t)
	for _, tt := range []struct {
		name string
		docs string
		json bool
	}{
		{name: "clean text", docs: "docs/clean"},
		{name: "broken text", docs: "docs/bad"},
		{name: "clean json", docs: "docs/clean", json: true},
		{name: "broken json", docs: "docs/bad", json: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader, writer := io.Pipe()
			t.Cleanup(func() {
				if err := writer.Close(); err != nil {
					t.Errorf("writer.Close() = %v, want nil", err)
				}
			})
			if err := reader.Close(); err != nil {
				t.Fatalf("reader.Close() = %v, want nil", err)
			}
			args := []string{"-root", root, "-docs", tt.docs}
			if tt.json {
				args = append(args, "-json")
			}
			var stderr bytes.Buffer
			if got := run(args, writer, &stderr); got != 2 {
				t.Errorf("run(%q, closed stdout) = %d, want 2", args, got)
			}
			if !strings.HasPrefix(stderr.String(), "audit_doc_citations: ") {
				t.Errorf("run(%q) stderr = %q, want an audit error diagnostic", args, stderr.String())
			}
		})
	}
}

func TestFindModuleRootStopsAtNearestGoMod(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	leaf := filepath.Join(nested, "leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "go.mod"), "module root\n\ngo 1.26\n")
	writeFile(t, filepath.Join(nested, "go.mod"), "module nested\n\ngo 1.26\n")
	got, err := findModuleRoot(leaf)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

type wantFinding struct {
	sub      string
	citation string
	kind     string
	reason   []string
}

func wantFindings() []wantFinding {
	return []wantFinding{
		{"internal/pkg/nope.go:5", "internal/pkg/nope.go:5", kindMissingPath, []string{"does not exist"}},
		{"cmd/tool/missing.go", "cmd/tool/missing.go", kindMissingPath, []string{"does not exist"}},
		{"internal/pkg/a.go:1000 in", "internal/pkg/a.go:1000", kindLineOutOfRange, []string{"line 1000 is past end of file"}},
		{"internal/pkg/a.go:1-1000", "internal/pkg/a.go:1-1000", kindLineOutOfRange, []string{"line 1000 is past end of file"}},
		{"internal/pkg/a.go:5-2", "internal/pkg/a.go:5-2", kindLineOutOfRange, []string{"ends before it starts"}},
		{"internal/pkg/a.go:1,1000", "internal/pkg/a.go:1,1000", kindLineOutOfRange, []string{"line 1000 is past end of file"}},
		{"internal/pkg/a.go:0", "internal/pkg/a.go:0", kindLineOutOfRange, []string{"line 0 is outside the file"}},
		{"`NotAFunc`", "`NotAFunc` at internal/pkg/a.go:1", kindSymbolNotDeclared, []string{"not declared"}},
		{"`Phantom`", "`Phantom` at internal/pkg/a.go:2", kindSymbolNotDeclared, []string{"not declared"}},
		{"`(*Other).Clear`", "`(*Other).Clear` at internal/pkg/a.go:1", kindSymbolNotDeclared, []string{"not declared"}},
		{"`(*Engine).Missing`", "`(*Engine).Missing` at internal/pkg/a.go:1", kindSymbolNotDeclared, []string{"not declared"}},
		{"`nope_pred`", "`nope_pred` at internal/pkg/rules.mg:1", kindPredicateNotDeclared, []string{"no Decl `nope_pred`"}},
		{"`ghost/1`", "`ghost/1`", kindPredicateNotDeclared, []string{"no Decl `ghost`"}},
		{"`ghost2/1`", "`ghost2/1`", kindPredicateNotDeclared, []string{"no Decl `ghost2`"}},
		{"`ghost3/1`", "`ghost3/1`", kindPredicateNotDeclared, []string{"no Decl `ghost3`"}},
		{"`pkg.Missing`", "`pkg.Missing`", kindSymbolNotDeclared, []string{"not declared", "internal/pkg"}},
		{"`sync.Missing`", "`sync.Missing`", kindSymbolNotDeclared, []string{"not declared", "internal/prompt/sync"}},
		{"defines `NotDefined`", "`NotDefined` at internal/pkg/a.go", kindSymbolNotDeclared, []string{"not declared"}},
		{"`Missing` (`internal/badparse/x.go`)", "`Missing` at internal/badparse/x.go", kindSymbolNotDeclared, []string{"not declared", "did not parse"}},
	}
}

func assertFindings(t *testing.T, lines []string, rep Report) {
	t.Helper()
	want := wantFindings()
	if len(rep.Findings) != len(want) {
		t.Errorf("findings %d, want %d", len(rep.Findings), len(want))
		for _, f := range rep.Findings {
			t.Errorf("  got %s:%d %s [%s] %s", f.Doc, f.Line, f.Citation, f.Kind, f.Reason)
		}
	}
	got := map[string]Finding{}
	for _, f := range rep.Findings {
		if strings.ContainsAny(f.Reason, "\r\n") || strings.ContainsAny(f.Citation, "\r\n") {
			t.Errorf("finding is not one line: %#v", f)
		}
		if f.Doc != "docs/bad/bad.md" {
			t.Errorf("doc %q", f.Doc)
		}
		if prev, ok := got[f.Citation]; ok {
			t.Errorf("duplicate citation %q (%s and %s)", f.Citation, prev.Kind, f.Kind)
		}
		got[f.Citation] = f
	}
	counts := map[string]int{}
	for _, w := range want {
		counts[w.kind]++
		f, ok := got[w.citation]
		if !ok {
			t.Errorf("missing finding %s", w.citation)
			continue
		}
		if f.Kind != w.kind {
			t.Errorf("%s kind %s, want %s", w.citation, f.Kind, w.kind)
		}
		ln := lineContaining(lines, w.sub)
		if ln < 1 {
			t.Errorf("substring %q not unique in the fixture (got %d)", w.sub, ln)
		} else if f.Line != ln {
			t.Errorf("%s line %d, want %d", w.citation, f.Line, ln)
		}
		for _, piece := range w.reason {
			if !strings.Contains(f.Reason, piece) {
				t.Errorf("%s reason %q does not contain %q", w.citation, f.Reason, piece)
			}
		}
	}
	for kind, n := range counts {
		if rep.ByKind[kind] != n {
			t.Errorf("by_kind %s = %d, want %d", kind, rep.ByKind[kind], n)
		}
	}
	if rep.Broken != len(want) {
		t.Errorf("broken %d, want %d", rep.Broken, len(want))
	}
}

func lineContaining(lines []string, sub string) int {
	n := 0
	found := 0
	for i, l := range lines {
		if strings.Contains(l, sub) {
			n = i + 1
			found++
		}
	}
	if found != 1 {
		return -found
	}
	return n
}

func badLines() []string {
	return []string{
		"# bad",
		"",
		"Missing file internal/pkg/nope.go:5 right here.",
		"",
		"Line past the end internal/pkg/a.go:1000 in the source.",
		"",
		"Range past the end internal/pkg/a.go:1-1000 is wrong.",
		"",
		"Inverted range internal/pkg/a.go:5-2 is wrong.",
		"",
		"Comma past the end internal/pkg/a.go:1,1000 is wrong.",
		"",
		"Zero line internal/pkg/a.go:0 is wrong.",
		"",
		"`NotAFunc` (`internal/pkg/a.go:1`) is not declared.",
		"",
		"`Phantom` (`internal/pkg/a.go:2`) is comment-only.",
		"",
		"`(*Other).Clear` (`internal/pkg/a.go:1`) is the wrong receiver.",
		"",
		"`(*Engine).Missing` (`internal/pkg/a.go:1`) is not a method.",
		"",
		"`nope_pred` (`internal/pkg/rules.mg:1`) has no Decl.",
		"",
		"`ghost/1` is only a hash-comment Decl.",
		"",
		"`ghost2/1` is only a block-comment Decl.",
		"",
		"`ghost3/1` is only a multi-line block-comment Decl.",
		"",
		"`pkg.Missing` is not in the package.",
		"",
		"`sync.Missing` is not declared in the nested package.",
		"",
		"internal/pkg/a.go defines `NotDefined`",
		"",
		"`Missing` (`internal/badparse/x.go`) does not parse.",
		"",
		"`NotInEmitter` (`emitter.go:10`) see `internal/pkg/a.go:1`",
		"",
		"The file internal/pkg/a.go is documented in the `vision` for this package.",
		"",
		"Not a rooted path: foo/internal/nope.go stays unchecked.",
		"",
		"Missing command cmd/tool/missing.go in the tree.",
	}
}

func cleanLines() []string {
	return []string{
		"# clean",
		"",
		"See internal/pkg/a.go:1 for the entry.",
		"",
		// Good is declared later in the file. The line number only has to
		// fall inside the file; the symbol has to be declared somewhere in
		// the file or its package.
		"`Good` (`internal/pkg/a.go:1`)",
		"",
		"`pkg.Good()` (`internal/pkg/a.go`)",
		"",
		"`(*Engine).Clear` (`internal/pkg/a.go:4`)",
		"",
		"`Engine.Cache` (`internal/pkg/a.go`)",
		"",
		"internal/pkg/a.go defines `OtherFile`",
		"",
		"`keep_me` (`internal/pkg/rules.mg:1`)",
		"",
		"`elsewhere/1` (`internal/pkg/rules.mg:1`)",
		"",
		"`pkg.OtherFile` lives in the other file.",
		"",
		"`keep_me/1` is declared in the corpus.",
		"",
		"Hash form internal/pkg/a.go#Good is present.",
		"",
		"A range inside the file internal/pkg/a.go:1-4 is fine.",
		"",
		"Comma list internal/pkg/a.go:1,3-4 is fine.",
		"",
		"Dot-slash ./internal/pkg/a.go:2 still resolves.",
		"",
		"Not a rooted path: foo/internal/nope.go stays unchecked.",
		"",
		"Parent path ../internal/nope.go is not rooted.",
		"",
		"Dotdot internal/../nope.go is not a citation.",
		"",
		"`json.Marshal` and `fmt.Sprintf` are not repo packages.",
		"",
		"`Good` / `OtherFile` (`internal/pkg/a.go:5`)",
		"",
		"internal/pkg/rules.mg#keep_me",
		"",
		"`Run` (`cmd/tool/main.go`)",
		"",
		"`main.Run` (`cmd/tool/main.go:1`)",
		"",
		"`tool.Run` is exported from the command.",
		"",
		// Filenames share a dot with pkg.Func. They are not citations, even
		// when a directory of that base name exists (cmd/nerd, internal/usage,
		// internal/types).
		"See `nerd.md`, `usage.json`, `types.go`, and `nerd.exe`.",
		"",
		"`usage.Load` is declared.",
		"",
		"`types.Placeholder` is declared.",
		"",
		// sync.Map is the standard library. The nested package declares Local
		// and does not declare Map; Map must not be graded against it.
		"`sync.Map` and `sync.Once` are the standard library. `sync.Local` is declared.",
		"",
		"`fmt.Errorf` (`internal/pkg/a.go:1`) calls the standard library.",
		"",
		"`filepath.Join` (`internal/pkg/a.go:2`) calls the standard library.",
		"",
		"`json.Marshal` (`internal/pkg/a.go`) is not a repo package.",
		"",
		"`acceptance_args_test.go` (`internal/pkg/a.go`) names a file.",
	}
}

func TestStdlibRecognizesSelectors(t *testing.T) {
	c := &checker{}
	if !c.stdlibHas("sync", "Map") || !c.stdlibHas("sync", "RWMutex") || !c.stdlibHas("sync", "Once") {
		t.Fatal("sync selectors were not recognised as the standard library")
	}
	if !c.stdlibHas("fmt", "Errorf") || !c.stdlibHas("filepath", "Join") || !c.stdlibHas("strings", "Contains") {
		t.Fatal("fmt/filepath/strings selectors were not recognised")
	}
	if c.stdlibHas("sync", "Local") || c.stdlibHas("context", "TokenCounter.charsPerToken") {
		t.Fatal("a selector the standard library does not declare was treated as one")
	}
	syncDir := stdlibDirs()["sync"]
	if filepath.Base(syncDir) != "sync" || strings.Contains(filepath.ToSlash(syncDir), "/internal/") {
		t.Fatalf("sync directory %q", syncDir)
	}
	if filepath.Base(stdlibDirs()["filepath"]) != "filepath" {
		t.Fatalf("filepath directory %q", stdlibDirs()["filepath"])
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.26\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "a.go"), `package pkg

// Phantom is only a comment, not a declaration.

func Good() {}

type Engine struct {
	Cache int
}

func (e *Engine) Clear() {}
`)
	writeFile(t, filepath.Join(root, "internal", "pkg", "b.go"), "package pkg\n\nfunc OtherFile() {}\n")
	// A sibling that does not parse must not fail the package load.
	writeFile(t, filepath.Join(root, "internal", "pkg", "broken.go"), "package pkg\n\nfunc (\n")
	writeFile(t, filepath.Join(root, "internal", "pkg", "rules.mg"), `# Decl ghost(X).
/*
Decl ghost3(X).
*/
/* Decl ghost2(X). */
Decl keep_me(X).
`)
	writeFile(t, filepath.Join(root, "internal", "other", "extra.mg"), "Decl elsewhere(X) bound [/string].\n")
	writeFile(t, filepath.Join(root, "internal", "badparse", "x.go"), "package badparse\n\nfunc (\n")
	writeFile(t, filepath.Join(root, "cmd", "tool", "main.go"), "package main\n\nfunc Run() {}\n")
	// Directory base names that collide with filenames and with the standard
	// library. The clean doc cites both shapes; only the real declarations count.
	writeFile(t, filepath.Join(root, "cmd", "nerd", "main.go"), "package main\n\nfunc NerdMain() {}\n")
	writeFile(t, filepath.Join(root, "internal", "usage", "usage.go"), "package usage\n\nfunc Load() {}\n")
	writeFile(t, filepath.Join(root, "internal", "types", "types.go"), "package types\n\nfunc Placeholder() {}\n")
	writeFile(t, filepath.Join(root, "internal", "prompt", "sync", "sync.go"), "package sync\n\nfunc Local() {}\n")
	writeFile(t, filepath.Join(root, "docs", "bad", "bad.md"), strings.Join(badLines(), "\n")+"\n")
	writeFile(t, filepath.Join(root, "docs", "clean", "clean.md"), strings.Join(cleanLines(), "\n")+"\n")
	// Not markdown: a dead path here must not be graded.
	writeFile(t, filepath.Join(root, "docs", "clean", "notes.txt"), "internal/pkg/nope.go\n")
	if err := os.MkdirAll(filepath.Join(root, "docs", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitPkg_PinSelectorShapes(t *testing.T) {
	cases := []struct {
		in  string
		pkg string
		sel string
		ok  bool
	}{
		{"pkg.Good", "pkg", "Good", true},
		{"sync.Map", "sync", "Map", true},
		{"a.b.c", "a", "b.c", true},
		{"(Engine).Clear", "", "", false},
		{"(*Engine).Clear", "", "", false},
		{"pkg/sub.Func", "", "", false},
		{".Func", "", "", false},
		{"pkg.", "", "", false},
		{"noseparator", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		pkg, sel, ok := splitPkg(tc.in)
		if pkg != tc.pkg || sel != tc.sel || ok != tc.ok {
			t.Errorf("splitPkg(%q) = %q,%q,%v want %q,%q,%v", tc.in, pkg, sel, ok, tc.pkg, tc.sel, tc.ok)
		}
	}
}

func TestLineReason_PinBranches(t *testing.T) {
	if got := lineReason([][2]int{{5, 2}}, 10); !strings.Contains(got, "ends before it starts") {
		t.Errorf("inverted range: %q", got)
	}
	if got := lineReason([][2]int{{0, 0}}, 8); !strings.Contains(got, "outside the file") {
		t.Errorf("zero line: %q", got)
	}
	if got := lineReason([][2]int{{1, 1000}}, 8); !strings.Contains(got, "past end of file") {
		t.Errorf("range past end: %q", got)
	}
	if got := lineReason([][2]int{{9, 9}}, 8); !strings.Contains(got, "past end of file") {
		t.Errorf("single past end: %q", got)
	}
	if got := lineReason([][2]int{{1, 2}}, 8); got != "" {
		t.Errorf("in-range reason should be empty, got %q", got)
	}
	if got := oneLine("a\r\nb\nc\rd"); got != "a b c d" {
		t.Errorf("oneLine = %q", got)
	}
	if skipDocsDir(".git") != true || skipDocsDir("vendor") != true || skipDocsDir("node_modules") != true {
		t.Error("excluded dirs must skip")
	}
	if skipDocsDir("docs") || skipDocsDir("internal") {
		t.Error("ordinary dirs must not skip")
	}
}

func TestPathHelpers_PinSlashSemantics(t *testing.T) {
	if got := slashBase("internal/pkg/a.go"); got != "a.go" {
		t.Errorf("slashBase = %q", got)
	}
	if got := slashBase("plain"); got != "plain" {
		t.Errorf("slashBase bare = %q", got)
	}
	if got := pkgDir("internal/pkg/a.go"); got != "internal/pkg" {
		t.Errorf("pkgDir = %q", got)
	}
	root := t.TempDir()
	if got := absJoin(root, "internal/pkg/a.go"); got != filepath.Join(root, "internal", "pkg", "a.go") {
		t.Errorf("absJoin = %q", got)
	}
	inside := filepath.Join(root, "sub", "f.md")
	if got := relSlash(root, inside); got != "sub/f.md" {
		t.Errorf("relSlash = %q", got)
	}
	if !withinRoot(root, inside) {
		t.Errorf("inside %q should be within %q", inside, root)
	}
	outside := filepath.Join(filepath.Dir(root), "outside-root-check.md")
	if withinRoot(root, outside) {
		t.Errorf("outside %q should not be within %q", outside, root)
	}
	if withinRoot(root, root) != true {
		t.Errorf("root should be within itself")
	}
}

func TestReportAdd_NormalizesMultilineReason(t *testing.T) {
	r := newReport()
	if len(r.Findings) != 0 || r.Broken != 0 {
		t.Fatalf("newReport should start empty: %+v", r)
	}
	r.add(Finding{Doc: "docs/d.md", Line: 1, Citation: "c", Kind: kindMissingPath, Reason: "a\nb\r\nc\rd"})
	if r.Broken != 1 || len(r.Findings) != 1 {
		t.Fatalf("add did not count: %+v", r)
	}
	if strings.ContainsAny(r.Findings[0].Reason, "\r\n") {
		t.Errorf("reason not one line: %q", r.Findings[0].Reason)
	}
	if r.ByKind[kindMissingPath] != 1 {
		t.Errorf("by_kind: %#v", r.ByKind)
	}
	var nilKind Report
	nilKind.add(Finding{Doc: "d", Citation: "c2", Kind: kindLineOutOfRange, Reason: "x"})
	if nilKind.ByKind[kindLineOutOfRange] != 1 {
		t.Errorf("nil ByKind map was not initialised: %#v", nilKind.ByKind)
	}
}

func TestAudit_RejectsBadRoots(t *testing.T) {
	if _, err := Audit(Options{RepoRoot: "   ", DocsRoot: "docs"}); err == nil {
		t.Error("empty repo root should fail")
	}
	root := t.TempDir()
	marker := filepath.Join(root, "file-not-dir")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Audit(Options{RepoRoot: marker, DocsRoot: "docs"}); err == nil {
		t.Error("file repo root should fail")
	}
	if _, err := Audit(Options{RepoRoot: root, DocsRoot: "no/such/docs"}); err == nil {
		t.Error("missing docs root should fail")
	}
	docsFile := filepath.Join(root, "docsfile.md")
	if err := os.WriteFile(docsFile, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Audit(Options{RepoRoot: root, DocsRoot: docsFile}); err == nil {
		t.Error("file docs root should fail")
	}
}

func TestMarkdownFiles_SkipsExcludedDirsAndSorts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "b.md"), "# b\n")
	writeFile(t, filepath.Join(root, "a.md"), "# a\n")
	writeFile(t, filepath.Join(root, "notes.txt"), "not markdown\n")
	writeFile(t, filepath.Join(root, "vendor", "hidden.md"), "# hidden\n")
	writeFile(t, filepath.Join(root, ".git", "hidden.md"), "# hidden\n")
	writeFile(t, filepath.Join(root, "node_modules", "hidden.md"), "# hidden\n")
	writeFile(t, filepath.Join(root, "sub", "c.MD"), "# upper ext\n")
	files, err := markdownFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.Contains(f, "vendor") || strings.Contains(f, ".git") || strings.Contains(f, "node_modules") {
			t.Errorf("excluded dir was walked: %s", f)
		}
		if filepath.Ext(f) == ".txt" {
			t.Errorf("non-markdown was collected: %s", f)
		}
	}
	if len(files) != 3 {
		t.Fatalf("files = %v", files)
	}
	if !(strings.HasSuffix(files[0], "a.md") && strings.HasSuffix(files[1], "b.md")) {
		t.Errorf("files are not sorted: %v", files)
	}
	if _, err := markdownFiles(filepath.Join(root, "no-such-dir")); err == nil {
		t.Error("missing docs dir should fail")
	}
}
