// audit_test_bodies rejects Go tests whose bodies contain only logging or are
// empty. Passing this check is not a claim of behavioral coverage; it closes
// the specific log-only placeholder failure without rewarding test counts.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func inert(body *ast.BlockStmt) bool {
	if body == nil {
		return true
	}
	for _, stmt := range body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Log" && sel.Sel.Name != "Logf") {
			return false
		}
	}
	return true
}

func audit(root string) ([]string, error) {
	var failures []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") || d.Name() == "vendor" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
				continue
			}
			if inert(fn.Body) {
				failures = append(failures, fmt.Sprintf("%s: %s has no executable behavior beyond logging", fset.Position(fn.Pos()), fn.Name.Name))
			}
		}
		return nil
	})
	return failures, err
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	failures, err := audit(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, f := range failures {
		fmt.Println(f)
	}
	if len(failures) > 0 {
		os.Exit(1)
	}
	fmt.Println("OK: no empty or log-only Go tests")
}
