package session

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// removedTestFunctions reports test functions that existed in a file the
// turn wrote before the turn and are gone from the workspace afterwards.
// A test is the contract for behaviour; a turn that makes the gates green
// by removing one has not fixed anything.
//
// A test that moved is not removed, and a test moves within its package: the
// same name in another _test.go file of the same directory that the default
// build includes. A namesake in another package is a different test (external
// audit F6, 2026-09-19: deleting alpha.TestContract was accepted because an
// unrelated beta.TestContract existed -- ordinary names like TestParse make
// that the common case, not a contrivance), and a copy behind a build tag the
// default build excludes does not run.
func removedTestFunctions(workspace string, writtenPaths []string, preWrite map[string]PreImage) []string {
	var out []string
	inPackage := map[string]map[string]bool{}
	deleted := deletedProductionFuncs(workspace, writtenPaths, preWrite)
	for _, p := range writtenPaths {
		missing := missingInPath(workspace, p, preWrite)
		if len(missing) == 0 {
			continue
		}
		dir := filepath.Dir(diskPath(workspace, p))
		present, ok := inPackage[dir]
		if !ok {
			present = packageTestNames(dir)
			inPackage[dir] = present
		}
		for _, n := range missing {
			if present[n] || testSubjectWasDeleted(preWrite[p].Content, n, deleted) {
				continue
			}
			out = append(out, p+":"+n)
		}
	}
	sort.Strings(out)
	return out
}

// deletedProductionFuncs is every function the turn removed outright from a
// non-test Go file it wrote, by bare name.
//
// A file the turn created has no pre-image and deletes nothing; a file whose
// pre-image or current content does not parse is skipped rather than read as
// having deleted everything in it.
func deletedProductionFuncs(workspace string, writtenPaths []string, preWrite map[string]PreImage) map[string]bool {
	out := map[string]bool{}
	for _, p := range writtenPaths {
		if isTestPath(p) || !strings.HasSuffix(p, ".go") {
			continue
		}
		pre, ok := preWrite[p]
		if !ok || !pre.Existed {
			continue
		}
		before, ok := funcDecls(pre.Content)
		if !ok {
			continue
		}
		after := map[string]funcDecl{}
		if data, err := os.ReadFile(diskPath(workspace, p)); err == nil {
			parsed, ok := funcDecls(string(data))
			if !ok {
				continue
			}
			after = parsed
		}
		for name := range before {
			if _, survives := after[name]; survives {
				continue
			}
			// funcKey reports a method as "Recv.Name"; a call site names the
			// method alone.
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:]
			}
			out[name] = true
		}
	}
	return out
}

// testSubjectWasDeleted reports whether the removed test named a function the
// same turn deleted from production -- in which case the test is deleted with
// its subject, not in place of fixing it (ladder run R1-18, 2026-09-19).
//
// The test is read from its pre-image, since it is gone from disk. Matching is
// on the whole identifier: tokenText writes one token per line and an
// identifier as "IDENT <name>", so a test naming netDelimitersOf is not
// released by the deletion of netDelimiters.
func testSubjectWasDeleted(beforeSrc, testName string, deleted map[string]bool) bool {
	if len(deleted) == 0 || beforeSrc == "" {
		return false
	}
	decls, ok := funcDecls(beforeSrc)
	if !ok {
		return false
	}
	decl, ok := decls[testName]
	if !ok {
		return false
	}
	for _, line := range strings.Split(decl.tokens, "\n") {
		name, isIdent := strings.CutPrefix(line, "IDENT ")
		if isIdent && deleted[name] {
			return true
		}
	}
	return false
}

// packageTestNames is every test function of the package in dir as the
// default build sees it: each _test.go file there that build.Default includes,
// internal and external test packages alike (go test runs both).
func packageTestNames(dir string) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !isTestPath(e.Name()) {
			continue
		}
		if included, err := build.Default.MatchFile(dir, e.Name()); err != nil || !included {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if names, ok := parseTestNames(string(data)); ok {
			for n := range names {
				out[n] = true
			}
		}
	}
	return out
}

// removedTestListing renders each removed test ("path:Name", as
// removedTestFunctions reports it) with its source as the turn found it. The
// harness holds the pre-write file, so putting a test back is a paste, not a
// reconstruction from memory.
func removedTestListing(removed []string, preWrite map[string]PreImage) string {
	var b strings.Builder
	for _, entry := range removed {
		i := strings.LastIndex(entry, ":")
		if i < 0 {
			continue
		}
		path, name := entry[:i], entry[i+1:]
		b.WriteString(path + ": " + name + "\n")
		if src := testFuncSource(preWrite[path].Content, name); src != "" {
			b.WriteString("```go\n" + src + "\n```\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// testFuncSource returns the source of the top-level function name in src,
// with its doc comment, or "" when src does not parse or has no such function.
func testFuncSource(src, name string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != name {
			continue
		}
		start := fn.Pos()
		if fn.Doc != nil {
			start = fn.Doc.Pos()
		}
		return src[fset.Position(start).Offset:fset.Position(fn.End()).Offset]
	}
	return ""
}

func isTestPath(p string) bool {
	return strings.HasSuffix(p, "_test.go")
}

// shouldCheck: a test file with a known preimage. One whose preimage is
// unknown (it could not be read before the write) cannot be compared, so the
// guard has nothing to say about it rather than reading it as empty.
func shouldCheck(path string, preWrite map[string]PreImage) bool {
	if !isTestPath(path) {
		return false
	}
	pre, ok := preWrite[path]
	return ok && pre.Known()
}

func diskPath(workspace string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

func missingInPath(workspace string, path string, preWrite map[string]PreImage) []string {
	if !shouldCheck(path, preWrite) {
		return nil
	}
	return diffTestNames(preWrite[path].Content, diskPath(workspace, path))
}

func diffTestNames(beforeSrc string, currentDiskPath string) []string {
	before, ok := parseTestNames(beforeSrc)
	if !ok {
		return nil
	}
	after, ok := readCurrentTests(currentDiskPath)
	if !ok {
		return nil
	}
	return missingNames(before, after)
}

func readCurrentTests(disk string) (map[string]bool, bool) {
	data, err := os.ReadFile(disk)
	if err != nil {
		return map[string]bool{}, true
	}
	names, ok := parseTestNames(string(data))
	if !ok {
		return nil, false
	}
	return names, true
}

func missingNames(before map[string]bool, after map[string]bool) []string {
	var out []string
	for n := range before {
		if !after[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func parseTestNames(src string) (map[string]bool, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	if f == nil {
		return nil, false
	}
	return collectNames(f), true
}

func collectNames(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv != nil {
			continue
		}
		if isTestFuncDecl(fn) {
			out[fn.Name.Name] = true
		}
	}
	return out
}

func isTestFuncDecl(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if isExampleName(name) {
		return true
	}
	if !hasTestPrefix(name) {
		return false
	}
	return hasTestingParam(fn)
}

func isExampleName(name string) bool {
	return strings.HasPrefix(name, "Example")
}

var testNamePrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

func hasTestPrefix(name string) bool {
	for _, pre := range testNamePrefixes {
		if strings.HasPrefix(name, pre) {
			return true
		}
	}
	return false
}

func hasTestingParam(fn *ast.FuncDecl) bool {
	if fn.Type == nil {
		return false
	}
	if fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		if fieldTypeIsTesting(field.Type) {
			return true
		}
	}
	return false
}

func fieldTypeIsTesting(e ast.Expr) bool {
	s, ok := unwrapStar(e)
	if !ok {
		return false
	}
	return isTestingSelector(s)
}

func unwrapStar(e ast.Expr) (*ast.SelectorExpr, bool) {
	if s, ok := e.(*ast.SelectorExpr); ok {
		return s, true
	}
	star, ok := e.(*ast.StarExpr)
	if !ok {
		return nil, false
	}
	s, ok := star.X.(*ast.SelectorExpr)
	return s, ok
}

func isTestingSelector(s *ast.SelectorExpr) bool {
	if s == nil {
		return false
	}
	if !isTestingIdent(s.X) {
		return false
	}
	return selNameIsTesting(s.Sel.Name)
}

func isTestingIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "testing"
}

func selNameIsTesting(name string) bool {
	for _, want := range []string{"T", "B", "F"} {
		if name == want {
			return true
		}
	}
	return false
}
