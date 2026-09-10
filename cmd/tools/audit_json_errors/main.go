// audit_json_errors is a budget on JSON calls whose error is discarded.
//
// It exists because of one September 2026 sweep that found the same defect
// four times, in four unrelated packages, none of which had a symptom anywhere
// near the code that caused it. The shape is always this:
//
//	metaJSON, _ := json.Marshal(v)
//	db.Exec("... VALUES (?)", string(metaJSON))
//
// json.Marshal returns nil on failure, string(nil) is "", and downstream ""
// almost never means "there was no value here". In this repo it meant, in
// turn: a row SQLite rejects with the bare message "malformed JSON"; an
// embedding column that keeps its row selected by every recall query and then
// fails to parse, so the row is unsearchable forever while the write reports
// success; a DELETE reading `fact_args = ”`, matching nothing, telling its
// caller a fact was retracted while the kernel still held it; and worst,
// ast.String(""), a perfectly valid Mangle constant asserted into the logic
// kernel in place of the argument that failed to encode, so every rule firing
// on it derived confidently from a value that was not the value.
//
// Unmarshal is the mirror. Its error was dropped in four vector-store scan
// loops, and the metadata-filtered one then dropped the row from the result
// set with nothing in the log — a silent wrong answer to a search.
//
// WHAT THIS IS NOT. It is not a claim that every entry below is a bug. Plenty
// are genuinely safe: json.Marshal of a []string or a struct of strings and
// ints cannot fail, and a discarded error on a value being written to a log
// line costs nothing. Deciding that needs full type information and a look at
// where the bytes go, which is a reviewer's job, not a walker's.
//
// So this is a budget, in the same spirit as scripts/deadcode-budget.sh: the
// baseline is a measurement, not a target of zero, and the gate fails when the
// number moves in EITHER direction. Fixing one means updating the baseline,
// which is cheap. Adding one means saying why, which is the point.
//
//	go run ./cmd/tools/audit_json_errors            check against the baseline
//	go run ./cmd/tools/audit_json_errors -update    rewrite the baseline
//
// Entries are keyed by package path, enclosing function and the source text of
// the argument rather than by line number, so an edit anywhere above a call
// does not churn the baseline.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const baselinePath = "scripts/testdata/json-errors-baseline.txt"

// roots are the trees that ship. testdata and vendor are excluded the way the
// Go toolchain excludes them, and _test.go files are out of scope: a test that
// drops an error fails visibly on the next line.
var roots = []string{"internal", "cmd", "pkg"}

type finding struct {
	key  string // pkg\tfunc\targ -- stable under edits above the call
	file string
	line int
}

// discarded reports the JSON function whose error is thrown away by this
// statement, or "" if the statement is not one.
//
// Two shapes count. `v, _ := json.Marshal(x)` names the error and assigns it
// to the blank identifier. `json.Unmarshal(b, &v)` as a bare expression
// statement discards it by never binding it at all -- the quieter of the two,
// because there is no `_` in the source to notice.
func discarded(stmt ast.Stmt) (fn string, arg ast.Expr, ok bool) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if len(s.Rhs) != 1 || len(s.Lhs) != 2 {
			return "", nil, false
		}
		blank, isIdent := s.Lhs[1].(*ast.Ident)
		if !isIdent || blank.Name != "_" {
			return "", nil, false
		}
		call, isCall := s.Rhs[0].(*ast.CallExpr)
		if !isCall {
			return "", nil, false
		}
		return jsonCall(call)
	case *ast.ExprStmt:
		call, isCall := s.X.(*ast.CallExpr)
		if !isCall {
			return "", nil, false
		}
		return jsonCall(call)
	}
	return "", nil, false
}

// jsonCall matches a call to the encoding/json functions that return an error
// worth keeping. The match is on the `json.` qualifier rather than on resolved
// types, which is what lets this run without loading the whole module; a local
// variable shadowing the package name would be a false positive and does not
// occur here.
func jsonCall(call *ast.CallExpr) (string, ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "json" {
		return "", nil, false
	}
	switch sel.Sel.Name {
	case "Marshal", "MarshalIndent", "Unmarshal":
		if len(call.Args) == 0 {
			return "", nil, false
		}
		return sel.Sel.Name, call.Args[0], true
	}
	return "", nil, false
}

func enclosingFunc(file *ast.File, pos token.Pos) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || pos < fn.Pos() || pos > fn.End() {
			continue
		}
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			var buf bytes.Buffer
			_ = printer.Fprint(&buf, token.NewFileSet(), fn.Recv.List[0].Type)
			return strings.TrimPrefix(buf.String(), "*") + "." + fn.Name.Name
		}
		return fn.Name.Name
	}
	return "<file scope>"
}

func scan() ([]finding, error) {
	var out []finding
	scanned := 0
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == "testdata" || (strings.HasPrefix(name, ".") && name != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			scanned++
			ast.Inspect(file, func(n ast.Node) bool {
				stmt, isStmt := n.(ast.Stmt)
				if !isStmt {
					return true
				}
				fn, arg, ok := discarded(stmt)
				if !ok {
					return true
				}
				var buf bytes.Buffer
				_ = printer.Fprint(&buf, fset, arg)
				pos := fset.Position(stmt.Pos())
				out = append(out, finding{
					key: fmt.Sprintf("%s\t%s\tjson.%s(%s)",
						filepath.ToSlash(filepath.Dir(path)),
						enclosingFunc(file, stmt.Pos()), fn, buf.String()),
					file: filepath.ToSlash(path),
					line: pos.Line,
				})
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	// Zero files means the tool is looking at the wrong tree, not that the
	// repo became clean. Without this the gate reports every baseline entry as
	// "fixed" and reads like an improvement -- the same failure the gofmt step
	// in CI guards against, where an argv overflow meant the check never ran a
	// single file while looking like it had.
	if scanned == 0 {
		return nil, fmt.Errorf("found no Go files under %s: run this from the repository root",
			strings.Join(roots, ", "))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].key != out[j].key {
			return out[i].key < out[j].key
		}
		return out[i].line < out[j].line
	})
	return out, nil
}

func readBaseline() ([]string, error) {
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		return nil, err
	}
	var keys []string
	// CRLF is tolerated: this repo is developed with core.autocrlf=true, and
	// the gate must give the same answer on both platforms.
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keys = append(keys, line)
	}
	sort.Strings(keys)
	return keys, nil
}

const header = `# Baseline for cmd/tools/audit_json_errors: JSON calls whose error is
# discarded. A budget, not a target of zero -- see that tool's doc comment for
# why some of these are fine and what the four that were not cost.
#
# Regenerate with: go run ./cmd/tools/audit_json_errors -update
# Format: package<TAB>function<TAB>call
`

func main() {
	update := flag.Bool("update", false, "rewrite the baseline from the current tree")
	flag.Parse()

	found, err := scan()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	keys := make([]string, len(found))
	byKey := make(map[string]finding, len(found))
	for i, f := range found {
		keys[i] = f.key
		if _, seen := byKey[f.key]; !seen {
			byKey[f.key] = f
		}
	}

	if *update {
		if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		body := header + strings.Join(keys, "\n") + "\n"
		if err := os.WriteFile(baselinePath, []byte(body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("Baseline updated: %d discarded JSON errors.\n", len(keys))
		return
	}

	baseline, err := readBaseline()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\nRun with -update to create it.\n", baselinePath, err)
		os.Exit(2)
	}

	added, removed := diff(baseline, keys)
	if len(added) == 0 && len(removed) == 0 {
		fmt.Printf("Discarded JSON errors: %d, matching the baseline.\n", len(keys))
		return
	}

	if len(added) > 0 {
		fmt.Fprintf(os.Stderr, "%d discarded JSON error(s) not in the baseline:\n\n", len(added))
		for _, k := range added {
			f := byKey[k]
			fmt.Fprintf(os.Stderr, "  %s:%d\n    %s\n", f.file, f.line, strings.ReplaceAll(k, "\t", "  "))
		}
		fmt.Fprintf(os.Stderr, `
Before adding to the baseline, check where the bytes go. A failed Marshal
yields "" and a failed Unmarshal leaves the target as it was. If either can
reach a database column, a comparison, a file or a fact, handle the error --
"" is not the absence of a value.

If the call is genuinely safe (a []string, a struct of strings and ints, a
value bound for a log line), record it:

  go run ./cmd/tools/audit_json_errors -update

`)
	}
	if len(removed) > 0 {
		fmt.Fprintf(os.Stderr, "%d baseline entry/entries no longer present (fixed or moved):\n\n", len(removed))
		for _, k := range removed {
			fmt.Fprintf(os.Stderr, "  %s\n", strings.ReplaceAll(k, "\t", "  "))
		}
		fmt.Fprintf(os.Stderr, "\nRecord the improvement:\n\n  go run ./cmd/tools/audit_json_errors -update\n\n")
	}
	os.Exit(1)
}

// diff compares two sorted multisets of keys. Multiset, not set: one function
// can hold two identical calls, and dropping the duplicate would let a second
// copy be added without the gate noticing.
func diff(baseline, found []string) (added, removed []string) {
	counts := make(map[string]int, len(baseline))
	for _, k := range baseline {
		counts[k]++
	}
	for _, k := range found {
		if counts[k] > 0 {
			counts[k]--
			continue
		}
		added = append(added, k)
	}
	for k, n := range counts {
		for i := 0; i < n; i++ {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
