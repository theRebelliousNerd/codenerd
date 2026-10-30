package prompt

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/builtin"
	"codeberg.org/TauCeti/mangle-go/parse"
	"codeberg.org/TauCeti/mangle-go/symbols"
)

// The Mangle atoms teach a model how to write rules for the engine codeNERD
// pins (go.mod: codeberg.org/TauCeti/mangle-go) and how to drive that engine
// from Go. Nothing else compiles them: a wrong example sits in the corpus until
// a model imitates it. These tests are that compiler. They ask the pinned
// engine itself, so a version bump that changes an answer fails here instead
// of in a prompt.
//
// The contract an atom author writes to:
//   - a ```mangle fence parses and analyzes on the engine, or the atom is wrong;
//   - an example the engine refuses goes in a ```mangle-rejected fence, and the
//     engine must refuse it: "this is an error" is checked, not asserted;
//   - an fn: or : name exists in the pinned engine, unless the line says it does
//     not exist;
//   - a Go identifier qualified by a mangle-go (or codeNERD) package exists and
//     is not deprecated, and a ```go fence parses as Go;
//   - in the codenerd/ atoms, predicates are the kernel's own, at the kernel's
//     arity and types.

const (
	mangleModulePath      = "codeberg.org/TauCeti/mangle-go"
	mangleAtomsRoot       = "atoms"
	mangleKernelAtomsID   = "language/mangle/docs/codenerd/"
	mangleRejectedFenceTg = "mangle-rejected"
)

var (
	fenceRe = regexp.MustCompile("(?s)```([A-Za-z0-9_-]*)[ \t]*\n(.*?)```")
	// negativeLineRe marks a line whose job is to say a name or API is absent.
	negativeLineRe = regexp.MustCompile(`(?i)(do(es)? not exist|doesn't exist|no such|nonexistent|unknown function|not a (real|built-?in)|deprecated|absent|hallucinat|has no |there (is|are) no |is rejected|are rejected)`)
	fnNameRe       = regexp.MustCompile(`fn:[A-Za-z][A-Za-z0-9_]*(?::[A-Za-z][A-Za-z0-9_]*)*`)
	predNameRe     = regexp.MustCompile(`(?:^|[^\w:/.])(:[a-z][a-z0-9_]*(?::[a-z][a-z0-9_]*)*)\(`)
	goQualifiedRe  = regexp.MustCompile(`\b([a-z][a-z0-9]*)\.([A-Z][A-Za-z0-9_]*)\b`)
)

// stripFences removes every fence of the given tags from text.
func stripFences(text string, tags ...string) string {
	return fenceRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := fenceRe.FindStringSubmatch(m)
		for _, t := range tags {
			if sub[1] == t {
				return ""
			}
		}
		return m
	})
}

// mangleCorpusAtoms returns every embedded atom that carries Mangle text: the
// whole language/mangle family plus any atom elsewhere with a mangle fence or a
// Go snippet naming the engine.
func mangleCorpusAtoms(t *testing.T) []*PromptAtom {
	t.Helper()
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var out []*PromptAtom
	for _, a := range corpus.All() {
		if a == nil {
			continue
		}
		if strings.HasPrefix(a.ID, "language/mangle") || strings.HasPrefix(a.ID, "mangle/") ||
			strings.Contains(a.Content, "```mangle") || strings.Contains(a.Content, mangleModulePath) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) < 20 {
		t.Fatalf("found %d Mangle atoms; the walk no longer finds the corpus", len(out))
	}
	return out
}

// TestMangleAtoms_NoUpstreamGoogleImportPath: codeNERD pins TauCeti's
// mangle-go. A model taught github.com/google/mangle writes imports and calls
// against the wrong library. Every file under atoms/ is read, so a README or a
// non-atom document cannot keep the old path either.
func TestMangleAtoms_NoUpstreamGoogleImportPath(t *testing.T) {
	var scanned int
	err := filepath.WalkDir(mangleAtomsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "google/mangle") {
				t.Errorf("%s:%d teaches the old upstream path (go.mod pins %s): %s", path, i+1, mangleModulePath, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk atoms: %v", err)
	}
	if scanned < 100 {
		t.Fatalf("scanned %d files under %s; the walk is broken", scanned, mangleAtomsRoot)
	}
}

// engineNames is the pinned engine's own table of what exists.
type engineNames struct {
	fns   map[string]bool
	preds map[string]bool
}

func pinnedEngineNames() engineNames {
	n := engineNames{fns: map[string]bool{}, preds: map[string]bool{}}
	for sym := range builtin.Functions {
		n.fns[sym.Symbol] = true
	}
	for sym := range symbols.TypeConstructors {
		n.fns[sym] = true
	}
	for sym := range builtin.Predicates {
		n.preds[sym.Symbol] = true
	}
	for sym := range symbols.BuiltinRelations {
		n.preds[sym.Symbol] = true
	}
	// Names the engine evaluates but whose type table rejects (see
	// verified-traps: fn:map:get fails analysis) are not teachable.
	delete(n.fns, "fn:map:get")
	return n
}

// namesCatalogAtoms are atoms whose whole purpose is to list names that do not
// exist beside what to write instead; their tables name absent functions row by
// row. Keep this list to such atoms.
var namesCatalogAtoms = map[string]bool{
	"language/mangle/docs/builtins_not_real": true,
}

// TestMangleAtoms_BuiltinNamesExistOnPinnedEngine: an fn: or : name an atom
// uses must be one the pinned engine has. Lines that say a name does not exist
// may name it, and so may a ```mangle-rejected fence (the engine refuses those,
// which TestMangleAtoms_FencesMatchEngine checks).
func TestMangleAtoms_BuiltinNamesExistOnPinnedEngine(t *testing.T) {
	names := pinnedEngineNames()
	for _, a := range mangleCorpusAtoms(t) {
		if namesCatalogAtoms[a.ID] {
			continue
		}
		text := stripFences(a.Content, mangleRejectedFenceTg)
		for i, line := range strings.Split(text, "\n") {
			if negativeLineRe.MatchString(line) {
				continue
			}
			for _, loc := range fnNameRe.FindAllStringIndex(line, -1) {
				fn := line[loc[0]:loc[1]]
				if rest := line[loc[1]:]; strings.HasPrefix(rest, "*") || strings.HasPrefix(rest, ":*") {
					continue // a family written as fn:time:*
				}
				if !names.fns[fn] {
					t.Errorf("%s line %d: %s is not a function of the pinned engine: %s", a.ID, i+1, fn, strings.TrimSpace(line))
				}
			}
			for _, m := range predNameRe.FindAllStringSubmatch(line, -1) {
				if !names.preds[m[1]] {
					t.Errorf("%s line %d: %s is not a built-in predicate of the pinned engine: %s", a.ID, i+1, m[1], strings.TrimSpace(line))
				}
			}
		}
	}
}

// goPackageIdents maps an import-style qualifier to the exported identifiers of
// the package it names, with the deprecated ones apart.
type goPackageIdents struct {
	exported   map[string]bool
	deprecated map[string]bool
}

func readGoPackage(t *testing.T, dir string) goPackageIdents {
	t.Helper()
	p := goPackageIdents{exported: map[string]bool{}, deprecated: map[string]bool{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Join(dir, e.Name()), err)
		}
		note := func(name string, doc *goast.CommentGroup) {
			if !goast.IsExported(name) {
				return
			}
			p.exported[name] = true
			if doc != nil && strings.Contains(doc.Text(), "Deprecated:") {
				p.deprecated[name] = true
			}
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *goast.FuncDecl:
				if d.Recv == nil {
					note(d.Name.Name, d.Doc)
				}
			case *goast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *goast.TypeSpec:
						doc := s.Doc
						if doc == nil {
							doc = d.Doc
						}
						note(s.Name.Name, doc)
					case *goast.ValueSpec:
						doc := s.Doc
						if doc == nil {
							doc = d.Doc
						}
						for _, n := range s.Names {
							note(n.Name, doc)
						}
					}
				}
			}
		}
	}
	return p
}

// goQualifiers returns, for every package qualifier an atom may use, the
// directory whose exports it must match. The mangle-go packages come from the
// module the build resolved; the codeNERD ones from this tree.
func goQualifiers(t *testing.T) map[string]goPackageIdents {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", mangleModulePath).Output()
	if err != nil {
		t.Fatalf("locate %s with go list: %v", mangleModulePath, err)
	}
	modDir := strings.TrimSpace(string(out))
	if modDir == "" {
		t.Fatalf("go list gave no directory for %s", mangleModulePath)
	}
	dirs := map[string]string{
		"engine":     filepath.Join(modDir, "engine"),
		"analysis":   filepath.Join(modDir, "analysis"),
		"factstore":  filepath.Join(modDir, "factstore"),
		"parse":      filepath.Join(modDir, "parse"),
		"ast":        filepath.Join(modDir, "ast"),
		"builtin":    filepath.Join(modDir, "builtin"),
		"symbols":    filepath.Join(modDir, "symbols"),
		"provenance": filepath.Join(modDir, "provenance"),
		"unionfind":  filepath.Join(modDir, "unionfind"),
		"packages":   filepath.Join(modDir, "packages"),
		"core":       filepath.Join("..", "core"),
		"mangle":     filepath.Join("..", "mangle"),
		"types":      filepath.Join("..", "types"),
	}
	res := map[string]goPackageIdents{}
	for q, dir := range dirs {
		res[q] = readGoPackage(t, dir)
	}
	return res
}

// TestMangleAtoms_GoIdentifiersExistAndAreCurrent: a model copies the Go it is
// shown. An identifier qualified by a mangle-go or codeNERD package must exist
// in the pinned module, and must not be one the module marks Deprecated.
func TestMangleAtoms_GoIdentifiersExistAndAreCurrent(t *testing.T) {
	quals := goQualifiers(t)
	for _, a := range mangleCorpusAtoms(t) {
		text := stripFences(a.Content, mangleRejectedFenceTg)
		for i, line := range strings.Split(text, "\n") {
			if negativeLineRe.MatchString(line) {
				continue
			}
			for _, m := range goQualifiedRe.FindAllStringSubmatch(line, -1) {
				pkg, ident := m[1], m[2]
				q, ok := quals[pkg]
				if !ok {
					continue
				}
				switch {
				case !q.exported[ident]:
					t.Errorf("%s line %d: %s.%s does not exist: %s", a.ID, i+1, pkg, ident, strings.TrimSpace(line))
				case q.deprecated[ident]:
					t.Errorf("%s line %d: %s.%s is deprecated; teach the current API: %s", a.ID, i+1, pkg, ident, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestMangleAtoms_GoFencesParse: a ```go fence is Go: a file, declarations or
// statements.
func TestMangleAtoms_GoFencesParse(t *testing.T) {
	checked := 0
	for _, a := range mangleCorpusAtoms(t) {
		for _, m := range fenceRe.FindAllStringSubmatch(a.Content, -1) {
			if m[1] != "go" {
				continue
			}
			checked++
			if err := parsesAsGo(m[2]); err != nil {
				t.Errorf("%s: go fence does not parse: %v\n%s", a.ID, err, m[2])
			}
		}
	}
	if checked < 3 {
		t.Fatalf("checked %d go fences; the Mangle atoms no longer show Go", checked)
	}
}

func parsesAsGo(src string) error {
	fset := token.NewFileSet()
	candidates := []string{
		src,
		"package p\n" + src,
		"package p\nfunc _() {\n" + src + "\n}",
	}
	// A snippet that opens with an import block and continues with statements.
	if strings.HasPrefix(strings.TrimSpace(src), "import (") {
		if end := strings.Index(src, "\n)\n"); end >= 0 {
			candidates = append(candidates, "package p\n"+src[:end+3]+"func _() {\n"+src[end+3:]+"\n}")
		}
	}
	var first error
	for _, c := range candidates {
		if _, err := parser.ParseFile(fset, "fence.go", c, parser.AllErrors); err == nil {
			return nil
		} else if first == nil {
			first = err
		}
	}
	return first
}

// fenceFacts is what a fence defines and uses, for the synthetic declarations
// a snippet needs to be analyzed alone.
func fenceFacts(unit parse.SourceUnit) (defined, used map[ast.PredicateSym]int) {
	defined, used = map[ast.PredicateSym]int{}, map[ast.PredicateSym]int{}
	for _, d := range unit.Decls {
		defined[d.DeclaredAtom.Predicate] = d.DeclaredAtom.Predicate.Arity
	}
	var visit func(term ast.Term)
	visit = func(term ast.Term) {
		switch v := term.(type) {
		case ast.Atom:
			used[v.Predicate] = v.Predicate.Arity
		case ast.NegAtom:
			used[v.Atom.Predicate] = v.Atom.Predicate.Arity
		case ast.TemporalAtom:
			used[v.Atom.Predicate] = v.Atom.Predicate.Arity
		case ast.TemporalLiteral:
			visit(v.Literal)
		}
	}
	for _, c := range unit.Clauses {
		defined[c.Head.Predicate] = c.Head.Predicate.Arity
		for _, p := range c.Premises {
			visit(p)
		}
	}
	return defined, used
}

// analyzeFence parses and analyzes one Mangle snippet the way the kernel does
// for a generated rule: strict bounds, every predicate it does not define
// supplied as a declaration. extra are real declarations (the kernel's); with
// none, a predicate a snippet only reads gets a synthetic one.
func analyzeFence(src string, extra map[ast.PredicateSym]*ast.Decl) error {
	unit, err := mangle.ParseUnit(strings.NewReader(src))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	defined, used := fenceFacts(unit)
	supplied := map[ast.PredicateSym]ast.Decl{}
	if extra != nil {
		for sym, d := range extra {
			if _, own := defined[sym]; !own && d != nil {
				supplied[sym] = *d
			}
		}
	} else {
		for sym := range used {
			if _, own := defined[sym]; own || isBuiltinPred(sym) {
				continue
			}
			supplied[sym] = ast.NewSyntheticDeclFromSym(sym)
		}
	}
	units := []parse.SourceUnit{unit}
	// Types are checked strictly only where the fence (or the kernel) declares
	// some: a synthetic declaration is /any, and the strict checker refuses to
	// join /any with a constant or a negation.
	mode := analysis.NoBoundsChecking
	if extra != nil || strings.Contains(src, "bound [") {
		mode = analysis.ErrorForBoundsMismatch
	}
	info, err, panicked := analyzeWithMode(units, supplied, mode)
	if panicked || (err != nil && mode != analysis.NoBoundsChecking && strings.Contains(src, ":string:matches")) {
		// The strict checker panics on :match_entry over an /any column, and has
		// no type signature for :string:matches in v0.7 (it rejects every use).
		// Loose analysis still decides the rest.
		info, err, _ = analyzeWithMode(units, supplied, analysis.NoBoundsChecking)
	}
	if err == nil && info != nil {
		// Stratification is a separate pass: analysis alone accepts a negation cycle.
		_, _, err = analysis.Stratify(analysis.Program{
			EdbPredicates: info.EdbPredicates,
			IdbPredicates: info.IdbPredicates,
			Rules:         info.Rules,
		})
	}
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	return nil
}

func analyzeWithMode(units []parse.SourceUnit, supplied map[ast.PredicateSym]ast.Decl, mode analysis.BoundsCheckingMode) (info *analysis.ProgramInfo, err error, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			info, err, panicked = nil, fmt.Errorf("bounds checker panicked: %v", r), true
		}
	}()
	info, err = analysis.AnalyzeAndCheckBounds(units, supplied, mode)
	return info, err, false
}

func isBuiltinPred(sym ast.PredicateSym) bool {
	_, ok := builtin.Predicates[sym]
	return ok
}

// kernelDecls is the booted kernel's analyzed declarations.
func kernelDecls(t *testing.T) map[ast.PredicateSym]*ast.Decl {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	info := kernel.GetProgramInfo()
	if info == nil || len(info.Decls) < 500 {
		t.Fatalf("the kernel program declares %d predicates; it did not boot", func() int {
			if info == nil {
				return 0
			}
			return len(info.Decls)
		}())
	}
	return info.Decls
}

// TestMangleAtoms_FencesMatchEngine runs every Mangle fence in the corpus on the
// pinned engine. A ```mangle fence must analyze; a ```mangle-rejected fence
// must be refused (so a "this does not work" example is itself true); the
// codenerd/ atoms are analyzed against the kernel's own declarations, so a
// predicate they teach exists, at that arity and with those types.
func TestMangleAtoms_FencesMatchEngine(t *testing.T) {
	kernel := kernelDecls(t)
	var accepted, rejected, kernelBound int
	for _, a := range mangleCorpusAtoms(t) {
		var extra map[ast.PredicateSym]*ast.Decl
		if strings.HasPrefix(a.ID, mangleKernelAtomsID) {
			extra = kernel
		}
		for _, m := range fenceRe.FindAllStringSubmatch(a.Content, -1) {
			tag, body := m[1], m[2]
			switch tag {
			case "mangle":
				accepted++
				if extra != nil {
					kernelBound++
				}
				if err := analyzeFence(body, extra); err != nil {
					t.Errorf("%s: ```mangle fence is not valid Mangle on the pinned engine: %v\n%s", a.ID, err, body)
				}
			case mangleRejectedFenceTg:
				rejected++
				if err := analyzeFence(body, extra); err == nil {
					t.Errorf("%s: ```mangle-rejected fence is accepted by the pinned engine:\n%s", a.ID, body)
				}
			case "datalog", "mg", "souffle", "prolog":
				if tag == "datalog" || tag == "mg" {
					t.Errorf("%s: fence tagged %q escapes the engine check; use ```mangle or ```%s", a.ID, tag, mangleRejectedFenceTg)
				}
			default:
				if strings.HasPrefix(tag, "mangle") {
					t.Errorf("%s: unknown mangle fence tag %q", a.ID, tag)
				}
			}
		}
	}
	if accepted < 40 || rejected < 10 || kernelBound < 5 {
		t.Fatalf("checked %d accepted, %d rejected, %d kernel-bound fences; the extraction no longer finds the corpus", accepted, rejected, kernelBound)
	}
}
