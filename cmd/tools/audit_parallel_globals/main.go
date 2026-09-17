// audit_parallel_globals is a gate on tests that run in parallel and then read
// or write process-global state.
//
// It exists because of one CI failure that cost an evening and pointed at the
// wrong code the entire time:
//
//	--- FAIL: TestUpdate_NoGoroutineLeakOnQuit
//	    Possible goroutine leak: before=68 after=88
//
// There was no leak. The test built five models and shut each one down, and a
// probe that diffs the goroutine stacks across exactly that loop reports a
// delta of MINUS ONE. runtime.NumGoroutine counts the whole process, the test
// called t.Parallel(), and cmd/nerd/chat has 176 parallel tests — so what it
// measured was whichever of its neighbours the scheduler had running between
// the two snapshots.
//
// Go already treats this class as serious enough for runtime enforcement:
// t.Setenv PANICS when called from a parallel test, with the message "cannot
// set environment variables in parallel tests". That guard sees exactly one
// function. os.Setenv, os.Chdir, runtime.NumGoroutine, runtime.ReadMemStats
// and the runtime/debug setters are the same hazard with nothing watching, and
// this walker is the part the runtime cannot do.
//
// WHY IT IS WORSE THAN AN ORDINARY FLAKE. The three properties compound:
//
//   - It passes locally and on most CI runs, because it needs an interleaving
//     nobody constructs on purpose. Reproducing the failure above needed a
//     neighbour that resumes BEFORE the subject and grows DURING its window; a
//     first attempt whose probe ran last passed every time.
//   - The failure message names the innocent test. "before=68 after=88" is a
//     report about the subject, and the subject is not what moved.
//   - The two halves review clean. t.Parallel() is correct and desirable;
//     runtime.NumGoroutine is correct. Only the pair is wrong, and no diff
//     shows a pair.
//
// WHAT COUNTS AS PARALLEL. Scope by scope, not file by file. A test body is
// parallel if it calls .Parallel(); a subtest closure passed to .Run is
// parallel if IT calls .Parallel() or if any enclosing scope did. A serial
// subtest inside a serial parent is not flagged even when a SIBLING subtest is
// parallel, because parallel subtests resume only after the parent function
// returns, so the two never overlap. Flagging that would be the day-one false
// positive that teaches everyone to skim the list.
//
// IT FOLLOWS PACKAGE-LOCAL HELPERS, and the reason is that this walker's own
// author opened the hole. The fix for the failure above moved the count into
// assertNoGoroutineLeak, a helper in the same test package -- so a gate that
// only read test bodies would have gone quiet about the exact regression it
// exists to catch, which is somebody putting t.Parallel() back on one of those
// two tests. A fixed point over the package's test files records every helper
// that reaches a watched call, directly or through another helper, and a call
// to one of those from a parallel scope is a finding.
//
// The limit is stated rather than left to be found: it follows plain function
// calls between test files of one package. A watched call reached through a
// method, an interface, a function value or a helper in production code is not
// seen. Full call-graph analysis needs type information, which is the
// dead-code budget's job and a different tool.
//
// WHAT COUNTS AS GLOBAL. Two kinds, both resolved through the file's actual
// import names so a renamed import still matches and an unrelated package
// called os does not:
//
//	read    runtime.NumGoroutine, runtime.ReadMemStats
//	write   runtime.GOMAXPROCS(n) for n != 0, os.Setenv, os.Unsetenv,
//	        os.Clearenv, os.Chdir, debug.SetGCPercent, debug.SetMemoryLimit,
//	        debug.SetMaxStack
//
// A read in a parallel test is measuring its neighbours. A write in a parallel
// test is changing its neighbours. runtime.GOMAXPROCS(0) is a read of the
// current value and is not flagged.
//
// THE BASELINE IS EMPTY AND SHOULD STAY THAT WAY, which is what separates this
// from the dead-code and JSON budgets. Those measure a population that exists
// for defensible reasons. This one has a fix that is always available: take
// t.Parallel() off the test. The cost is a second or two of wall clock in a
// suite that already spends thirty, and what it buys is a measurement that
// means what it says. An entry here is for the case where that is genuinely
// wrong, and it has to say why.
//
//	go run ./cmd/tools/audit_parallel_globals            check against the baseline
//	go run ./cmd/tools/audit_parallel_globals -update    rewrite the baseline
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const baselinePath = "scripts/testdata/parallel-globals-baseline.txt"

// watched maps an import path to the identifiers on it that reach process-wide
// state. The value is the reason, which is printed with the finding so the
// report explains itself without anyone opening this file.
var watched = map[string]map[string]string{
	"runtime": {
		"NumGoroutine": "counts goroutines across the whole process, including every other parallel test's",
		"ReadMemStats": "reports allocation for the whole process, including every other parallel test's",
		"GOMAXPROCS":   "changes the scheduler for the whole process (reading it, GOMAXPROCS(0), is fine)",
	},
	"os": {
		"Setenv":   "changes the environment for the whole process; t.Setenv is the guarded form and panics here instead",
		"Unsetenv": "changes the environment for the whole process; t.Setenv is the guarded form and panics here instead",
		"Clearenv": "changes the environment for the whole process",
		"Chdir":    "changes the working directory for the whole process; t.Chdir is the guarded form",
	},
	"runtime/debug": {
		"SetGCPercent":   "changes the garbage collector for the whole process",
		"SetMemoryLimit": "changes the garbage collector for the whole process",
		"SetMaxStack":    "changes the stack limit for the whole process",
	},
}

type finding struct {
	pkg    string
	fn     string
	call   string
	reason string
	pos    string
}

func (f finding) key() string { return f.pkg + "\t" + f.fn + "\t" + f.call }

func main() {
	update := flag.Bool("update", false, "rewrite the baseline from what is found now")
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	module, err := moduleName(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	found, err := scan(root, module)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].key() < found[j].key() })

	if *update {
		if err := writeBaseline(filepath.Join(root, baselinePath), found); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("Baseline rewritten: %d parallel test(s) touching process-global state.\n", len(found))
		return
	}

	base, err := readBaseline(filepath.Join(root, baselinePath))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var added, removed []finding
	seen := make(map[string]bool, len(found))
	for _, f := range found {
		seen[f.key()] = true
		if !base[f.key()] {
			added = append(added, f)
		}
	}
	for key := range base {
		if !seen[key] {
			parts := strings.SplitN(key, "\t", 3)
			for len(parts) < 3 {
				parts = append(parts, "")
			}
			removed = append(removed, finding{pkg: parts[0], fn: parts[1], call: parts[2]})
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i].key() < removed[j].key() })

	if len(added) == 0 && len(removed) == 0 {
		if len(found) == 0 {
			fmt.Println("No parallel test reads or writes process-global state. The baseline is empty and stays that way.")
		} else {
			fmt.Printf("Parallel-globals budget holds: %d entry(ies), no drift.\n", len(found))
		}
		return
	}

	if len(added) > 0 {
		fmt.Printf("%d parallel test(s) touching process-global state, not in the baseline:\n\n", len(added))
		for _, f := range added {
			fmt.Printf("  %s\n", f.pos)
			fmt.Printf("    %s  %s  %s\n", f.pkg, f.fn, f.call)
			fmt.Printf("    %s\n", f.reason)
		}
		fmt.Println()
		fmt.Println("A parallel test reading a process-wide number is measuring its neighbours,")
		fmt.Println("and a parallel test writing process-wide state is changing them. Neither")
		fmt.Println("fails where the mistake is: the report names the test that was looking, not")
		fmt.Println("the tests that moved the number.")
		fmt.Println()
		fmt.Println("The fix is almost always to drop t.Parallel() from that test. Go runs a")
		fmt.Println("serial test \"in parallel with (and only with) other parallel tests\" -- which")
		fmt.Println("is to say never -- so a serial test gets the isolation a global measurement")
		fmt.Println("needs, at the cost of a second of wall clock.")
		fmt.Println()
		fmt.Println("If it is genuinely right as it stands, record it and say why:")
		fmt.Println()
		fmt.Println("  go run ./cmd/tools/audit_parallel_globals -update")
		fmt.Println()
	}
	if len(removed) > 0 {
		fmt.Printf("Entries that are no longer present (%d):\n", len(removed))
		for _, f := range removed {
			fmt.Printf("  %s\t%s\t%s\n", f.pkg, f.fn, f.call)
		}
		fmt.Println()
		fmt.Println("Good. Refresh the baseline so the count stays a real measurement:")
		fmt.Println("  go run ./cmd/tools/audit_parallel_globals -update")
		fmt.Println()
	}
	os.Exit(1)
}

func moduleName(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in go.mod")
}

// testFile is one parsed _test.go together with the local names it uses for
// the watched imports, which differ file by file.
type testFile struct {
	file  *ast.File
	names map[string]string
}

func scan(root, module string) ([]finding, error) {
	fset := token.NewFileSet()
	byDir := make(map[string][]testFile)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// A test file this walker cannot parse is not this gate's business
			// to report; the compiler says it louder.
			return nil
		}
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], testFile{file: file, names: importNames(file)})
		return nil
	})
	if err != nil {
		return nil, err
	}

	var found []finding
	for dir, files := range byDir {
		pkg := packagePath(root, module, filepath.Join(dir, "x_test.go"))
		found = append(found, scanPackage(fset, files, pkg)...)
	}
	return found, nil
}

func packagePath(root, module, path string) string {
	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil || rel == "." {
		return module
	}
	return module + "/" + filepath.ToSlash(rel)
}

// scanPackage reports the watched calls that sit in a parallel scope, across
// every test file of one package so that helpers are visible.
func scanPackage(fset *token.FileSet, files []testFile, pkg string) []finding {
	helpers := localHelpers(fset, files, pkg)
	var found []finding

	for _, tf := range files {
		for _, decl := range tf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			if !isTestLike(fn) {
				continue
			}
			walkScope(fset, fn.Body, false, pkg, fn.Name.Name, tf.names, helpers, &found)
		}
	}
	return found
}

// localHelpers records every package-local function that reaches a watched
// call, directly or through another local function, as a fixed point. The
// value is the reason the global is a global, carried through so a finding
// about a helper explains itself the same way a direct one does.
func localHelpers(fset *token.FileSet, files []testFile, pkg string) map[string]string {
	reach := make(map[string]string)

	for changed := true; changed; {
		changed = false
		for _, tf := range files {
			for _, decl := range tf.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Recv != nil || isTestLike(fn) {
					continue
				}
				if _, done := reach[fn.Name.Name]; done {
					continue
				}
				reason := ""
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if reason != "" {
						return false
					}
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if f, ok := watchedCall(fset, call, pkg, "", tf.names); ok {
						reason = f.reason
						return false
					}
					if ident, ok := call.Fun.(*ast.Ident); ok {
						if r, ok := reach[ident.Name]; ok {
							reason = r
							return false
						}
					}
					return true
				})
				if reason != "" {
					reach[fn.Name.Name] = reason
					changed = true
				}
			}
		}
	}
	return reach
}

// importNames maps the local name a file uses for each watched import path.
// A file that does not import one simply has no entry, so a local helper
// called os.Setenv in some other package cannot match.
func importNames(file *ast.File) map[string]string {
	names := make(map[string]string)
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if _, ok := watched[path]; !ok {
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue
			}
			local = spec.Name.Name
		}
		names[local] = path
	}
	return names
}

func isTestLike(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	return strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Fuzz")
}

// walkScope handles one scope: the body of a test, or the closure passed to
// .Run. It decides whether this scope is parallel, reports the watched calls
// inside it if so, and then recurses into each subtest closure carrying that
// decision down. Nested closures that are NOT subtests belong to this scope and
// are walked as part of it.
func walkScope(fset *token.FileSet, body *ast.BlockStmt, parentParallel bool, pkg, fn string, names, helpers map[string]string, found *[]finding) {
	parallel := parentParallel || callsParallel(body)

	var subtests []*ast.FuncLit
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if lit := subtestBody(call); lit != nil {
			subtests = append(subtests, lit)
			return false // its contents belong to the child scope
		}
		if !parallel {
			return true
		}
		if f, ok := watchedCall(fset, call, pkg, fn, names); ok {
			*found = append(*found, f)
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if reason, ok := helpers[ident.Name]; ok {
				pos := fset.Position(call.Pos())
				*found = append(*found, finding{
					pkg:    pkg,
					fn:     fn,
					call:   ident.Name + "()",
					reason: reason + " -- reached through " + ident.Name,
					pos:    fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line),
				})
			}
		}
		return true
	})

	for _, lit := range subtests {
		walkScope(fset, lit.Body, parallel, pkg, fn, names, helpers, found)
	}
}

// callsParallel reports whether this scope calls .Parallel() directly, without
// counting one made inside a subtest closure.
func callsParallel(body *ast.BlockStmt) bool {
	direct := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if subtestBody(call) != nil {
			return false
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" && len(call.Args) == 0 {
			if _, ok := sel.X.(*ast.Ident); ok {
				direct = true
			}
		}
		return true
	})
	return direct
}

// subtestBody returns the closure of a t.Run(name, func(t *testing.T){...})
// call, or nil when this call is not a subtest.
func subtestBody(call *ast.CallExpr) *ast.FuncLit {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" || len(call.Args) == 0 {
		return nil
	}
	lit, ok := call.Args[len(call.Args)-1].(*ast.FuncLit)
	if !ok || lit.Body == nil {
		return nil
	}
	return lit
}

func watchedCall(fset *token.FileSet, call *ast.CallExpr, pkg, fn string, names map[string]string) (finding, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return finding{}, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return finding{}, false
	}
	path, ok := names[ident.Name]
	if !ok {
		return finding{}, false
	}
	reason, ok := watched[path][sel.Sel.Name]
	if !ok {
		return finding{}, false
	}
	// GOMAXPROCS(0) reads the current value and changes nothing.
	if sel.Sel.Name == "GOMAXPROCS" && len(call.Args) == 1 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "0" {
			return finding{}, false
		}
	}
	pos := fset.Position(call.Pos())
	return finding{
		pkg:    pkg,
		fn:     fn,
		call:   ident.Name + "." + sel.Sel.Name,
		reason: reason,
		pos:    fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line),
	}, true
}

func readBaseline(path string) (map[string]bool, error) {
	base := make(map[string]bool)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return base, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		base[line] = true
	}
	return base, sc.Err()
}

func writeBaseline(path string, found []finding) error {
	var b strings.Builder
	b.WriteString("# Baseline for cmd/tools/audit_parallel_globals: tests that call\n")
	b.WriteString("# t.Parallel() and then read or write process-global state.\n")
	b.WriteString("#\n")
	b.WriteString("# Unlike the dead-code and JSON budgets, this one is meant to stay EMPTY.\n")
	b.WriteString("# A parallel test reading runtime.NumGoroutine or runtime.ReadMemStats is\n")
	b.WriteString("# measuring its neighbours, and one writing os.Setenv or os.Chdir is\n")
	b.WriteString("# changing them. The fix is available in every case: drop t.Parallel().\n")
	b.WriteString("#\n")
	b.WriteString("# An entry here is a claim that a test is right to be both parallel and\n")
	b.WriteString("# global, which is a claim that needs a reason in the commit that adds it.\n")
	b.WriteString("#\n")
	b.WriteString("# Regenerate with: go run ./cmd/tools/audit_parallel_globals -update\n")
	b.WriteString("# Format: package<TAB>function<TAB>call\n")
	for _, f := range found {
		b.WriteString(f.key())
		b.WriteString("\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
