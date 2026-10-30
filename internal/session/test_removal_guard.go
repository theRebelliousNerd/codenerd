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

	"golang.org/x/mod/modfile"
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
			if present[n] || testSubjectWasDeleted(workspace, dir, preWrite[p].Content, n, deleted) {
				continue
			}
			out = append(out, p+":"+n)
		}
	}
	sort.Strings(out)
	return out
}

// deletedFuncs are the production functions a turn removed, by package
// directory then name (a method by its bare name, as a call site names it).
type deletedFuncs map[string]map[string]bool

// deletedProductionFuncs is every function the turn removed outright from
// the package of a non-test Go file it wrote: declared there before the turn
// and declared in no non-test file of that package directory now. A function
// moved to another file of its package is not deleted.
//
// A file the turn created has no pre-image and deletes nothing; a file whose
// pre-image or current content does not parse is skipped rather than read as
// having deleted everything in it.
func deletedProductionFuncs(workspace string, writtenPaths []string, preWrite map[string]PreImage) deletedFuncs {
	out := deletedFuncs{}
	surviving := map[string]map[string]bool{}
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
		if data, err := os.ReadFile(diskPath(workspace, p)); err == nil {
			if _, ok := funcDecls(string(data)); !ok {
				continue
			}
		}
		dir := filepath.Dir(diskPath(workspace, p))
		now, ok := surviving[dir]
		if !ok {
			now = packageFuncKeys(dir)
			surviving[dir] = now
		}
		for key := range before {
			if now[key] {
				continue
			}
			name := key
			if i := strings.LastIndex(name, "."); i >= 0 {
				name = name[i+1:]
			}
			if out[dir] == nil {
				out[dir] = map[string]bool{}
			}
			out[dir][name] = true
		}
	}
	return out
}

// packageFuncKeys is every function key declared in the non-test Go files of
// dir as they stand.
func packageFuncKeys(dir string) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || isTestPath(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if decls, ok := funcDecls(string(data)); ok {
			for key := range decls {
				out[key] = true
			}
		}
	}
	return out
}

// testSubjectWasDeleted reports whether the removed test referred to a
// function the same turn deleted from production -- in which case the test is
// deleted with its subject, not in place of fixing it (ladder run R1-18,
// 2026-09-19).
//
// The reference is resolved, not matched by spelling: a name deleted from
// another package does not release a test whose subject survives in its own
// (two packages each with a Reset). An unqualified name, or a method selector
// on a value, refers to the test's own package (dir) -- unless the test file
// is an external test package, whose unqualified names are its own; a
// package-qualified selector refers to the imported package, resolved through
// the file's imports and the workspace module. A reference that cannot be
// resolved releases nothing. The test is read from its pre-image, since it is
// gone from disk.
func testSubjectWasDeleted(workspace, dir, beforeSrc, testName string, deleted deletedFuncs) bool {
	if len(deleted) == 0 || beforeSrc == "" {
		return false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", beforeSrc, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == testName {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return false
	}
	external := strings.HasSuffix(f.Name.Name, "_test")
	imports := importDirs(workspace, f)
	own := deleted[dir]
	found := false
	selected := map[*ast.Ident]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch x := n.(type) {
		case *ast.SelectorExpr:
			selected[x.Sel] = true
			if id, ok := x.X.(*ast.Ident); ok {
				if pkgDir, isImport := imports[id.Name]; isImport {
					selected[id] = true
					found = deleted[pkgDir][x.Sel.Name]
					return !found
				}
			}
			// A method on a value: its type is declared in the package
			// under test.
			found = own[x.Sel.Name]
		case *ast.Ident:
			if !selected[x] && !external {
				found = own[x.Name]
			}
		}
		return !found
	})
	return found
}

// importDirs maps each import's local name in f to its package directory,
// for imports under the workspace's module; other imports are not resolved.
func importDirs(workspace string, f *ast.File) map[string]string {
	out := map[string]string{}
	module := workspaceModulePath(workspace)
	if module == "" {
		return out
	}
	for _, spec := range f.Imports {
		path := strings.Trim(spec.Path.Value, "\"`")
		rest, ok := strings.CutPrefix(path, module)
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		out[name] = filepath.Join(workspace, filepath.FromSlash(strings.TrimPrefix(rest, "/")))
	}
	return out
}

// workspaceModulePath is the module path the workspace's go.mod declares.
func workspaceModulePath(workspace string) string {
	data, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return ""
	}
	return modfile.ModulePath(data)
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
