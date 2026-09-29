package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	lines := splitLines(text)
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

func TestAcceptsPath(t *testing.T) {
	samples := []struct {
		line string
		want bool
	}{
		{"internal/pkg/a.go", true},
		{"See internal/pkg/a.go:1", true},
		{"`internal/pkg/a.go`", true},
		{"./internal/pkg/a.go", true},
		{"/internal/pkg/a.go", true},
		{" (./internal/pkg/a.go)", true},
		{"foo/internal/nope.go", false},
		{"../internal/nope.go", false},
		{"https://example.com/internal/nope.go", false},
		{"xinternal/pkg/a.go", false},
		{"cmd/tool/main.go", true},
	}
	for _, s := range samples {
		i := strings.Index(s.line, "internal/")
		if i < 0 {
			i = strings.Index(s.line, "cmd/")
		}
		if i < 0 {
			t.Fatal(s.line)
		}
		if got := acceptsPath(s.line, i); got != s.want {
			t.Errorf("%q accepts=%v want %v", s.line, got, s.want)
		}
	}
}

func TestParseSuffix(t *testing.T) {
	lines, hash, n := parseSuffix(":1-4, 10")
	if n != len(":1-4, 10") || hash != "" || len(lines) != 2 || lines[0] != [2]int{1, 4} || lines[1] != [2]int{10, 10} {
		t.Fatalf("list: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = parseSuffix(": the")
	if n != 0 || hash != "" || lines != nil {
		t.Fatalf("prose colon consumed: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = parseSuffix("#Good:2")
	if hash != "Good" || n != len("#Good:2") || len(lines) != 1 || lines[0] != [2]int{2, 2} {
		t.Fatalf("hash then line: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = parseSuffix(":12#TaxonomyEngine.ClassifyInput")
	if hash != "TaxonomyEngine.ClassifyInput" || len(lines) != 1 || lines[0] != [2]int{12, 12} {
		t.Fatalf("line then hash: lines=%v hash=%q n=%d", lines, hash, n)
	}
}

func TestGapBinds(t *testing.T) {
	if !gapBinds(" (`") {
		t.Fatal("parenthetical citation")
	}
	if !gapBinds(" defines ") {
		t.Fatal("defines")
	}
	if gapBinds(" is documented in the ") {
		t.Fatal("prose bound as a citation")
	}
	if gapBinds(" (`emitter.go:10`) see ") {
		t.Fatal("abbreviated filename should consume the symbol")
	}
	if !gapBinds(" / `OtherFile` | ") {
		t.Fatal("two symbols before one path")
	}
	if codeShaped("vision") || codeShaped("mu") {
		t.Fatal("lowercase prose is not code-shaped; mu binds only across a parenthetical gap")
	}
	if !codeShaped("NotDefined") || !codeShaped("keep_me") || !codeShaped("pkg.Good") {
		t.Fatal("code-shaped names")
	}
}

func TestNormalizeSymbol(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"var uiDiffEngine", "uiDiffEngine", true},
		{"pkg.Good()", "pkg.Good", true},
		{"context_must_retain(Predicate)", "context_must_retain", true},
		{"(*Engine).Clear()", "(*Engine).Clear", true},
		{"keep_me/1", "keep_me/1", true},
		{"diff.NewEngine", "diff.NewEngine", true},
		{"pkg.Go", "pkg.Go", true},
		{"sync.RWMutex", "sync.RWMutex", true},
		{"internal/pkg/a.go", "", false},
		{"not a symbol", "", false},
		{"(*Engine)", "", false},
		{"nerd.md", "", false},
		{"usage.json", "", false},
		{"feedback.go", "", false},
		{"types.go", "", false},
		{"nerd.exe", "", false},
		{"config.yaml", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizeSymbol(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("normalize %q = %q %v, want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDeclNamesIgnoreComments(t *testing.T) {
	src := "# Decl ghost(X).\n/*\nDecl ghost3(X).\n*/\n/* Decl ghost2(X). */\nDecl keep_me(X).\nDecl elsewhere(X) bound [/string].\n"
	got := declNames(src)
	if !got["keep_me"] || !got["elsewhere"] {
		t.Fatalf("decls %#v", got)
	}
	for _, bad := range []string{"ghost", "ghost2", "ghost3", "Declare"} {
		if got[bad] {
			t.Fatalf("counted %s in %#v", bad, got)
		}
	}
	if declNames("Declare something(X).\n")["Declare"] {
		t.Fatal("Declare is not Decl")
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
