package world

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Decl-bound conformance.
//
// A Decl's bound list is not documentation: a slot declared /name only ever
// unifies with a name constant, so a Go emitter that writes the bare string
// "function" where the Decl says /name produces a fact that matches no rule and
// no query — silently, with no error anywhere. That exact mismatch has already
// shipped more than once in this codebase (a quoted string queried against a
// /name slot in internal/mcp never matched).
//
// This test converts the facts the world scanners actually emit into the same
// Mangle terms the kernel would store (types.Fact.ToAtom) and checks each term
// against the declared bound parsed from the shipped schema files. It fails if
// an emitter drifts from a Decl, or a Decl drifts from its emitter.

var declRe = regexp.MustCompile(`(?m)^Decl\s+([a-z_0-9]+)\(([^)]*)\)\s*bound\s*\[([^\]]*)\]`)

// loadDeclBounds parses `Decl p(...) bound [/t, ...]` from the shipped default
// schema files.
func loadDeclBounds(t *testing.T) map[string][]string {
	t.Helper()
	dir := filepath.Join("..", "core", "defaults")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read schema dir: %v", err)
	}
	bounds := make(map[string][]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mg") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range declRe.FindAllStringSubmatch(string(data), -1) {
			pred := m[1]
			if _, dup := bounds[pred]; dup {
				continue // first declaration wins, as at load time
			}
			var kinds []string
			for _, k := range strings.Split(m[3], ",") {
				kinds = append(kinds, strings.TrimSpace(k))
			}
			bounds[pred] = kinds
		}
	}
	if len(bounds) == 0 {
		t.Fatal("parsed no Decl bounds; the schema location must have moved")
	}
	return bounds
}

// termKind classifies a stored Mangle term the way the evaluator does.
func termKind(rendered string) string {
	switch {
	case strings.HasPrefix(rendered, `"`):
		return "/string"
	case strings.HasPrefix(rendered, "/"):
		return "/name"
	case rendered == "":
		return "/empty"
	}
	if strings.IndexFunc(rendered, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return "/number"
	}
	if strings.HasPrefix(rendered, "-") {
		return "/number"
	}
	return "/other"
}

// knownDeclDrift records slots where the shipped Decl and the emitter disagree
// TODAY, with the emitter being the one every consuming rule was written
// against. Example: policy/data_flow.mg joins Var across assigns/uses/guards_*,
// and all four emitters write /name atoms, so the rules work — but
// schemas_reviewer.mg bounds those slots /string. Correcting the bound belongs
// to the owner of internal/core/defaults (a Decl edit is not additive, and
// concurrent work touches that tree); until then the mismatch is pinned here
// rather than left to be rediscovered, and anything NOT in this map fails.
//
// Nothing in this list is safe to grow casually: each entry is a slot where a
// rule written with a literal of the declared type would silently match zero
// facts.
var knownDeclDrift = map[string][]int{
	"uses":            {1, 2},
	"assigns":         {0},
	"guards_return":   {0},
	"guards_block":    {0},
	"safe_access":     {0},
	"function_scope":  {1},
	"guard_dominates": {1},
	"call_arg":        {0, 2},
}

func driftAllowed(pred string, arg int) bool {
	for _, i := range knownDeclDrift[pred] {
		if i == arg {
			return true
		}
	}
	return false
}

func checkFactAgainstDecl(t *testing.T, bounds map[string][]string, f core.Fact) {
	t.Helper()
	want, ok := bounds[f.Predicate]
	if !ok {
		return // predicate declared elsewhere (or intentionally undeclared)
	}
	if len(want) != len(f.Args) {
		t.Errorf("%s emitted with %d args but declared with %d: %v", f.Predicate, len(f.Args), len(want), f.Args)
		return
	}
	atom, err := types.Fact{Predicate: f.Predicate, Args: f.Args}.ToAtom()
	if err != nil {
		t.Errorf("%s cannot be converted to a Mangle atom: %v (%v)", f.Predicate, err, f.Args)
		return
	}
	for i, term := range atom.Args {
		got := termKind(term.String())
		if got == "/other" {
			continue // floats, times: no Decl in this package uses them
		}
		if got != want[i] && !driftAllowed(f.Predicate, i) {
			t.Errorf("%s arg %d is %s but the Decl bounds it as %s (value %v). A %s slot never unifies with a %s, so every rule reading this predicate derives nothing.",
				f.Predicate, i, got, want[i], f.Args[i], want[i], got)
		}
	}
}

// TestScannerFacts_WhenEmitted_ShouldMatchDeclaredBounds runs a real scan and
// checks every emitted fact against its Decl.
func TestScannerFacts_WhenEmitted_ShouldMatchDeclaredBounds(t *testing.T) {
	bounds := loadDeclBounds(t)

	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "internal/core/core.go", "package core\n\ntype Widget struct{ ID int }\n\nfunc Do() {}\n\nfunc (w *Widget) Run() {}\n")
	writeWorkspaceFile(t, root, "internal/core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestDo(t *testing.T) {}\n")
	writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Do() }\n")
	writeWorkspaceFile(t, root, "svc/app.py", "class Thing:\n    def run(self):\n        return 1\n")
	writeWorkspaceFile(t, root, "web/app.ts", "export class C { go() { return 1; } }\n")
	writeWorkspaceFile(t, root, "rs/lib.rs", "pub fn go() -> u32 { 1 }\n")
	writeWorkspaceFile(t, root, "policy/rules.mg", "Decl thing(X) bound [/string].\nthing(\"a\").\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) == 0 {
		t.Fatal("scan produced no facts")
	}
	for _, f := range facts {
		checkFactAgainstDecl(t, bounds, f)
	}
}

// TestDeepFacts_WhenEmitted_ShouldMatchDeclaredBounds does the same for the
// deep (Cartographer) layer, across every language it now maps.
func TestDeepFacts_WhenEmitted_ShouldMatchDeclaredBounds(t *testing.T) {
	bounds := loadDeclBounds(t)
	fixtures := map[string]string{
		"a.go": "package a\n\nfunc F() int { return G() }\n\nfunc G() int { return 1 }\n",
		"b.py": "def f():\n    return g()\n\ndef g():\n    return 1\n",
		"c.ts": "function f() { return g(); }\nfunction g() { return 1; }\n",
		"d.rs": "pub fn f() -> u32 { g() }\npub fn g() -> u32 { 1 }\n",
	}
	for name, src := range fixtures {
		t.Run(name, func(t *testing.T) {
			facts := mapFixture(t, name, src)
			if len(facts) == 0 {
				t.Fatalf("no deep facts for %s", name)
			}
			for _, f := range facts {
				checkFactAgainstDecl(t, bounds, f)
			}
		})
	}
}

// TestSymbolGraph_WhenEmitted_ShouldUseNameConstantsForTypeAndVisibility pins
// the specific slots called out in the world TODO: symbol_graph's Type and
// Visibility are declared /name, and a plain "function"/"public" string in
// those slots would be stored as a string constant that no /function pattern
// can match.
func TestSymbolGraph_WhenEmitted_ShouldUseNameConstantsForTypeAndVisibility(t *testing.T) {
	parser := NewTreeSitterParser()
	defer parser.Close()

	sources := map[string][]byte{
		"a.go": []byte("package main\n\ntype S struct{ F int }\n\nfunc Exported() {}\nfunc private() {}\n"),
		"b.py": []byte("class Thing:\n    def run(self):\n        return 1\n"),
		"c.ts": []byte("export class C { go() { return 1; } }\n"),
		"d.rs": []byte("pub struct S; pub fn go() {}\n"),
	}
	var all []core.Fact
	for name, src := range sources {
		var facts []core.Fact
		var err error
		switch filepath.Ext(name) {
		case ".go":
			facts, err = parser.ParseGo(name, src)
		case ".py":
			facts, err = parser.ParsePython(name, src)
		case ".ts":
			facts, err = parser.ParseTypeScript(name, src)
		case ".rs":
			facts, err = parser.ParseRust(name, src)
		}
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		all = append(all, facts...)
	}

	var seen int
	for _, f := range all {
		if f.Predicate != "symbol_graph" {
			continue
		}
		seen++
		atom, err := types.Fact{Predicate: f.Predicate, Args: f.Args}.ToAtom()
		if err != nil {
			t.Fatalf("symbol_graph -> atom: %v", err)
		}
		for _, i := range []int{1, 2} {
			if got := termKind(atom.Args[i].String()); got != "/name" {
				t.Errorf("symbol_graph arg %d stored as %s (%v); the Decl bounds it /name", i, got, f.Args[i])
			}
		}
		// Slot 3 is DefinedAt, declared /string: a path that Mangle mistakes
		// for a name constant would break every file join.
		if got := termKind(atom.Args[3].String()); got != "/string" {
			t.Errorf("symbol_graph DefinedAt stored as %s (%v); the Decl bounds it /string", got, f.Args[3])
		}
	}
	if seen == 0 {
		t.Fatal("no symbol_graph facts emitted by any language")
	}
}
