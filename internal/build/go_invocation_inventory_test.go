package build

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file is the adoption mandate for internal/build, expressed as a test
// instead of a paragraph in a doc.
//
// Every production `exec.Command("go", …)` in the repo must be classified:
// either the site assigns cmd.Env from this package, or its file appears in
// goSpawnExemptions with a reason. A new unmarked `go` invocation fails this
// test; a stale exemption (the file stopped spawning `go`, or adopted
// internal/build) also fails it, so the inventory cannot quietly rot the way
// the old package-comment consumer list did.
//
// Scope: non-test .go files. Test files compile throwaway modules in temp dirs
// where the monorepo's CGO headers are irrelevant, and holding every future
// fixture to this rule would only teach people to work around the check.

// goSpawnExemptions maps a repo-relative file path to the reason its `go`
// invocations do not route through internal/build.
var goSpawnExemptions = map[string]string{
	// Operator-facing CLI verification. These run in the operator's own shell
	// with cmd.Env unset, so the toolchain inherits the full ambient
	// environment — including any CGO_CFLAGS/GOFLAGS the operator exported.
	// Narrowing them to the filtered build env would drop vars the operator
	// deliberately set for their own session.
	"cmd/nerd/dom_cmd.go":         "exempt: operator-invoked CLI verification inherits the operator's ambient shell environment",
	"cmd/nerd/dom_apply_cmd.go":   "exempt: operator-invoked CLI verification inherits the operator's ambient shell environment",
	"cmd/nerd/dom_replace_cmd.go": "exempt: operator-invoked CLI verification inherits the operator's ambient shell environment",

	// `go mod tidy` is module resolution, not compilation. It needs the
	// credentials the build env filter deliberately drops (GOPROXY auth,
	// .netrc, GOPRIVATE), and it links nothing, so CGO flags are irrelevant.
	// The compile and test steps in the same file do use GetBuildEnvForCompile.
	"internal/autopoiesis/tool_compiler.go": "exempt: `go mod tidy` needs the ambient module-resolution credentials the build env filter drops; the build/test steps in this file do use internal/build",

	// Runs the user's own tests in the user's own project root as a tool call.
	// Pending adoption: it should take the session's UserConfig and route
	// through GetBuildEnvForTest. Tracked in Docs/architecture/build/TODO.md.
	"internal/tools/codedom/run_impacted_tests.go": "pending adoption: should route through GetBuildEnvForTest with the session UserConfig (Docs/architecture/build/TODO.md P1)",
}

type goSpawnSite struct {
	file    string // repo-relative, slash-separated
	line    int
	fn      string
	usesEnv bool // cmd.Env assigned from a build.* expression
}

func TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt(t *testing.T) {
	root := repoRoot(t)
	sites := scanGoSpawnSites(t, root)

	if len(sites) == 0 {
		t.Fatal("scanner found no `go` invocations at all — the scanner is broken, not the repo")
	}

	// Emit the inventory so `go test -v ./internal/build/...` prints the
	// current, authoritative answer to "who spawns go?".
	byFile := map[string][]goSpawnSite{}
	for _, s := range sites {
		byFile[s.file] = append(byFile[s.file], s)
	}
	for _, file := range slices.Sorted(mapKeys(byFile)) {
		// The status has to be read off the sites, not assumed. It used to
		// default to "uses internal/build" and be overridden only by an
		// exemption, so a file that spawned `go` WITHOUT the build env printed
		// "uses internal/build" — in the very inventory this test exists to
		// publish. The assertion below still caught it, but anyone reading the
		// -v output was told the opposite of the truth about the one file that
		// mattered.
		allUseEnv := true
		for _, s := range byFile[file] {
			if !s.usesEnv {
				allUseEnv = false
				break
			}
		}
		var status string
		switch {
		case allUseEnv:
			status = "uses internal/build"
		case goSpawnExemptions[file] != "":
			// Some reasons already open with "exempt:"; do not stutter.
			status = strings.TrimPrefix(goSpawnExemptions[file], "exempt: ")
			status = "exempt: " + status
		default:
			status = "VIOLATION: spawns go without the build env and is not exempt"
		}
		lines := make([]string, 0, len(byFile[file]))
		for _, s := range byFile[file] {
			lines = append(lines, s.fn+":"+strconv.Itoa(s.line))
		}
		t.Logf("%-46s %s  [%s]", file, strings.Join(lines, " "), status)
	}

	var unmarked []string
	filesWithUnmarked := map[string]bool{}
	for _, s := range sites {
		if s.usesEnv {
			continue
		}
		filesWithUnmarked[s.file] = true
		if _, ok := goSpawnExemptions[s.file]; !ok {
			unmarked = append(unmarked, s.file+":"+strconv.Itoa(s.line)+" ("+s.fn+")")
		}
	}
	sort.Strings(unmarked)

	if len(unmarked) > 0 {
		t.Errorf("unmarked `go` invocations (neither cmd.Env from internal/build nor listed in goSpawnExemptions):\n  %s\n\n"+
			"Fix one of two ways:\n"+
			"  1. set cmd.Env = build.GetBuildEnv/GetBuildEnvForTest/GetBuildEnvForCompile(...), or\n"+
			"  2. add the file to goSpawnExemptions in this file with a real reason.",
			strings.Join(unmarked, "\n  "))
	}

	for file := range goSpawnExemptions {
		if !filesWithUnmarked[file] {
			t.Errorf("stale exemption for %q: it no longer has any unmarked `go` invocation. Remove the entry from goSpawnExemptions.", file)
		}
	}
}

// TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented keeps the package
// comment's consumer list honest. That list was fiction for a long time — it
// named preflight, attack_runner and tester, which never imported this package,
// while the real importers went unmentioned. A prose list cannot notice when it
// stops being true, so this test owns it: adding or dropping an importer fails
// here until the comment and 08-WIRING-AND-INTEGRATION.md are updated too.
func TestBuildImporters_WhenNewConsumerAppears_ShouldBeDocumented(t *testing.T) {
	root := repoRoot(t)

	documented := map[string]bool{
		"internal/autopoiesis": true, // tool_compiler.go, thunderdome.go
		"internal/session":     true, // build_verify.go, test_verify.go, coverage_profile.go, lsp_diagnostics.go
		"internal/core":        true, // virtual_store_actions.go (shell-run go test/build)
		"internal/system":      true, // factory_execution.go (tactile ExecutorConfig.BaseEnvironment)
	}

	seen := map[string][]string{}
	for _, file := range scanBuildImporters(t, root) {
		pkg := filepath.ToSlash(filepath.Dir(file))
		seen[pkg] = append(seen[pkg], filepath.Base(file))
	}

	for pkg, files := range seen {
		t.Logf("importer %-24s %s", pkg, strings.Join(files, " "))
		if !documented[pkg] {
			t.Errorf("new internal/build consumer %q (%s): update the package comment in env.go and Docs/architecture/build/08-WIRING-AND-INTEGRATION.md, then add it here",
				pkg, strings.Join(files, " "))
		}
	}
	for pkg := range documented {
		if len(seen[pkg]) == 0 {
			t.Errorf("documented consumer %q no longer imports internal/build: update the package comment and this list", pkg)
		}
	}
}

// scanBuildImporters returns repo-relative non-test .go files importing this package.
func scanBuildImporters(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, imp := range file.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr == nil && p == "codenerd/internal/build" {
				rel, rerr := filepath.Rel(root, path)
				if rerr == nil {
					files = append(files, filepath.ToSlash(rel))
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(files)
	return files
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if strings.HasPrefix(string(data), "module codenerd") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate the codenerd module root")
		}
		dir = parent
	}
}

var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "testdata": true,
	".nerd": true, "sqlite_headers": true,
	".claude": true, // agent worktrees — complete repo copies, so walking it duplicates every file under a non-checkout path
}

func scanGoSpawnSites(t *testing.T, root string) []goSpawnSite {
	t.Helper()

	var sites []goSpawnSite
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable trees are not this test's problem
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // a package mid-edit by another worker should not fail this test
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			sites = append(sites, scanFuncForGoSpawns(fset, rel, fn)...)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return sites
}

// scanFuncForGoSpawns finds `x := exec.Command[Context](…, "go", …)` inside one
// function and decides whether x.Env is later assigned from a build.* call.
func scanFuncForGoSpawns(fset *token.FileSet, rel string, fn *ast.FuncDecl) []goSpawnSite {
	type pending struct {
		varName string
		line    int
	}
	var found []pending

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) == 0 {
			return true
		}
		call := unwrapGoExecCall(assign.Rhs[0])
		if call == nil {
			return true
		}
		name := ""
		if id, ok := assign.Lhs[0].(*ast.Ident); ok {
			name = id.Name
		}
		found = append(found, pending{varName: name, line: fset.Position(call.Pos()).Line})
		return true
	})

	if len(found) == 0 {
		return nil
	}

	// Collect variables whose .Env is assigned from an expression mentioning the
	// build package, anywhere in this function.
	envFromBuild := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Env" {
			return true
		}
		base, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if mentionsBuildPackage(assign.Rhs[0]) {
			envFromBuild[base.Name] = true
		}
		return true
	})

	sites := make([]goSpawnSite, 0, len(found))
	for _, p := range found {
		sites = append(sites, goSpawnSite{
			file:    rel,
			line:    p.line,
			fn:      fn.Name.Name,
			usesEnv: p.varName != "" && envFromBuild[p.varName],
		})
	}
	return sites
}

// unwrapGoExecCall returns the exec.Command/CommandContext call whose binary is
// the literal "go", looking through wrapper calls such as
// processutil.NonInteractive(exec.CommandContext(...)).
func unwrapGoExecCall(expr ast.Expr) *ast.CallExpr {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if isGoExecCall(call) {
		return call
	}
	for _, arg := range call.Args {
		if inner := unwrapGoExecCall(arg); inner != nil {
			return inner
		}
	}
	return nil
}

func isGoExecCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return false
	}
	binArg := -1
	switch sel.Sel.Name {
	case "Command":
		binArg = 0
	case "CommandContext":
		binArg = 1
	default:
		return false
	}
	if len(call.Args) <= binArg {
		return false
	}
	lit, ok := call.Args[binArg].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	val, err := strconv.Unquote(lit.Value)
	return err == nil && val == "go"
}

// mentionsBuildPackage reports whether an expression references the `build`
// package identifier (build.GetBuildEnv, build.MergeEnv(build.GetBuildEnv…), …).
func mentionsBuildPackage(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "build" {
			found = true
			return false
		}
		return true
	})
	return found
}
