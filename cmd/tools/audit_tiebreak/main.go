// audit_tiebreak finds selections whose winner depends on iteration order.
//
// The shape is a loop that ranges with both a key and a value and, inside an
// inequality, records the KEY as the best one seen:
//
//	for file, count := range fileCount {
//	    if count > maxCount {
//	        maxCount, maxFile = count, file
//	    }
//	}
//
// Over a map that is a coin flip. Go randomises map iteration deliberately, so
// when two keys tie the winner is whichever the runtime reached first, and the
// program returns a different answer on different runs with nothing to explain
// it. Ties are not exotic — they are the ordinary case whenever the thing being
// counted is small.
//
// Five of these were live in this repo in September 2026, and not one was
// cosmetic:
//
//   - extractFileFromFindings chose which file to send the fixer to. Two
//     findings in one file and two in another is a normal review, so the same
//     review dispatched the fixer differently on two runs.
//   - Two separate scans chose the workspace's primary language, which decides
//     the build and test commands the agent reaches for. A repo with a Go
//     module and a Node package in sibling directories got a different answer
//     each run — observed directly: "typescript", then "go", same directory.
//   - The trace analyser chose the most common decision per topic and recorded
//     it as a LEARNED preference: a memory that disagreed with itself.
//   - The problem classifier chose a task's category, which drives prompt
//     evolution, so one task trained two different lessons.
//
// The fix in every case was to sort the keys before ranging them. The
// tie-break that produces is arbitrary — there is no principled ordering
// between "go" and "typescript" — but it is FIXED, and that is the whole
// difference between a bug and a haunting. A stable wrong answer can be
// reproduced and argued with.
//
// LIMITS, stated because a gate whose blind spots are undocumented gets
// trusted past them. This works on syntax, with no type information, so it
// cannot tell a map from a slice: the same loop over a slice is deterministic
// and legitimate, and would be reported here. That is why there is a baseline
// rather than a hard zero — an entry recorded with its reason in the commit is
// the escape hatch. It also only catches the two-variable form; a loop that
// ranges keys alone and looks the value up inside is invisible to it.
//
//	go run ./cmd/tools/audit_tiebreak            check against the baseline
//	go run ./cmd/tools/audit_tiebreak -update    rewrite the baseline
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

const baselinePath = "scripts/testdata/tiebreak-baseline.txt"

var roots = []string{"internal", "cmd", "pkg"}

type finding struct {
	key  string
	file string
	line int
}

// selectsKeyByComparison reports whether the body of rng records its key as a
// running best inside an inequality.
func selectsKeyByComparison(rng *ast.RangeStmt) bool {
	if rng.Key == nil || rng.Value == nil {
		return false
	}
	key, ok := rng.Key.(*ast.Ident)
	if !ok || key.Name == "_" {
		return false
	}

	found := false
	ast.Inspect(rng.Body, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		cmp, ok := ifs.Cond.(*ast.BinaryExpr)
		if !ok || (cmp.Op != token.GTR && cmp.Op != token.LSS) {
			return true
		}
		ast.Inspect(ifs.Body, func(m ast.Node) bool {
			assign, ok := m.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, rhs := range assign.Rhs {
				if id, ok := rhs.(*ast.Ident); ok && id.Name == key.Name {
					found = true
				}
			}
			return true
		})
		return true
	})
	return found
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
				rng, ok := n.(*ast.RangeStmt)
				if !ok || !selectsKeyByComparison(rng) {
					return true
				}
				keyName := rng.Key.(*ast.Ident).Name
				out = append(out, finding{
					key: fmt.Sprintf("%s\t%s\trange %s",
						filepath.ToSlash(filepath.Dir(path)),
						enclosingFunc(file, rng.Pos()), keyName),
					file: filepath.ToSlash(path),
					line: fset.Position(rng.Pos()).Line,
				})
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Zero files means the wrong working directory, not a clean repo. Without
	// this the gate reports every baseline entry as fixed and reads like an
	// improvement.
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

const header = `# Selections whose winner depends on iteration order — a loop ranging key and
# value that records the KEY as a running best inside an inequality.
#
# Over a map that is a coin flip: Go randomises map iteration, so a tie is
# broken by whichever key the runtime reached first. See cmd/tools/audit_tiebreak
# for the five live instances this was written after, and for why the check
# cannot tell a map from a slice — a legitimate slice case belongs here, with
# its reason in the commit that adds it.
#
# Regenerate: go run ./cmd/tools/audit_tiebreak -update
# Format: package<TAB>function<TAB>range key
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
		body := header
		if len(keys) > 0 {
			body += strings.Join(keys, "\n") + "\n"
		}
		if err := os.WriteFile(baselinePath, []byte(body), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("Baseline updated: %d order-dependent selection(s).\n", len(keys))
		return
	}

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\nRun with -update to create it.\n", baselinePath, err)
		os.Exit(2)
	}
	baseline := map[string]int{}
	total := 0
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[line]++
		total++
	}

	var added []string
	counts := map[string]int{}
	for k, v := range baseline {
		counts[k] = v
	}
	for _, k := range keys {
		if counts[k] > 0 {
			counts[k]--
			continue
		}
		added = append(added, k)
	}
	var fixed []string
	for k, n := range counts {
		for i := 0; i < n; i++ {
			fixed = append(fixed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(fixed)

	if len(added) == 0 && len(fixed) == 0 {
		if total == 0 {
			fmt.Println("No order-dependent selections. The baseline is empty and stays that way.")
		} else {
			fmt.Printf("Order-dependent selections: %d, matching the baseline.\n", total)
		}
		return
	}

	if len(added) > 0 {
		fmt.Fprintf(os.Stderr, "%d selection(s) whose winner depends on iteration order:\n\n", len(added))
		for _, k := range added {
			f := byKey[k]
			fmt.Fprintf(os.Stderr, "  %s:%d\n    %s\n", f.file, f.line, strings.ReplaceAll(k, "\t", "  "))
		}
		fmt.Fprintf(os.Stderr, `
If this ranges a MAP, the tie-break is Go's randomised iteration order and the
result changes between runs. Sort the keys before ranging them; an arbitrary
but fixed order is what separates a bug from a haunting.

If it ranges a SLICE it is already deterministic and this check cannot tell the
difference. Record it, with the reason in your commit message:

  go run ./cmd/tools/audit_tiebreak -update

`)
	}
	if len(fixed) > 0 {
		fmt.Fprintf(os.Stderr, "%d baseline entry/entries no longer present:\n\n", len(fixed))
		for _, k := range fixed {
			fmt.Fprintf(os.Stderr, "  %s\n", strings.ReplaceAll(k, "\t", "  "))
		}
		fmt.Fprintf(os.Stderr, "\nRecord the improvement:\n\n  go run ./cmd/tools/audit_tiebreak -update\n\n")
	}
	os.Exit(1)
}
