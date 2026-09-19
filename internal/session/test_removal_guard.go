package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// removedTestFunctions reports test functions that existed in a file the
// turn wrote before the turn and are gone from the workspace afterwards.
// A test is the contract for behaviour; a turn that makes the gates green
// by removing one has not fixed anything. A test that moved — the same
// name is in some other _test.go under the workspace — is not removed.
func removedTestFunctions(workspace string, writtenPaths []string, preWrite map[string]string) []string {
	present := workspaceTestNames(workspace)
	var out []string
	for _, p := range writtenPaths {
		for _, n := range missingInPath(workspace, p, preWrite) {
			if !present[n] {
				out = append(out, p+":"+n)
			}
		}
	}
	sort.Strings(out)
	return out
}

// removedTestListing renders each removed test ("path:Name", as
// removedTestFunctions reports it) with its source as the turn found it. The
// harness holds the pre-write file, so putting a test back is a paste, not a
// reconstruction from memory.
func removedTestListing(removed []string, preWrite map[string]string) string {
	var b strings.Builder
	for _, entry := range removed {
		i := strings.LastIndex(entry, ":")
		if i < 0 {
			continue
		}
		path, name := entry[:i], entry[i+1:]
		b.WriteString(path + ": " + name + "\n")
		if src := testFuncSource(preWrite[path], name); src != "" {
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

func shouldCheck(path string, preWrite map[string]string) bool {
	if !isTestPath(path) {
		return false
	}
	_, ok := preWrite[path]
	return ok
}

func diskPath(workspace string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

func missingInPath(workspace string, path string, preWrite map[string]string) []string {
	if !shouldCheck(path, preWrite) {
		return nil
	}
	return diffTestNames(preWrite[path], diskPath(workspace, path))
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

func workspaceTestNames(workspace string) map[string]bool {
	out := map[string]bool{}
	_ = filepath.WalkDir(workspace, makeWalkFn(out))
	return out
}

func makeWalkFn(out map[string]bool) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if skipWalkDir(d) {
			return filepath.SkipDir
		}
		tryCollectTestFile(path, d, out)
		return nil
	}
}

func skipWalkDir(d fs.DirEntry) bool {
	if d == nil {
		return false
	}
	if !d.IsDir() {
		return false
	}
	return skippedDirName(d.Name())
}

func skippedDirName(name string) bool {
	for _, want := range []string{".git", ".nerd", "vendor", "node_modules"} {
		if name == want {
			return true
		}
	}
	return false
}

func isCollectable(path string, d fs.DirEntry) bool {
	if d == nil {
		return false
	}
	if d.IsDir() {
		return false
	}
	return isTestPath(path)
}

func tryCollectTestFile(path string, d fs.DirEntry, out map[string]bool) {
	if !isCollectable(path, d) {
		return
	}
	mergeFileTests(path, out)
}

func mergeFileTests(path string, out map[string]bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	names, ok := parseTestNames(string(data))
	if !ok {
		return
	}
	for n := range names {
		out[n] = true
	}
}
