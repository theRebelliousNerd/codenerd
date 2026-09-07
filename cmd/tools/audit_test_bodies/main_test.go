package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestLogOnlyNegativeControl(t *testing.T) {
	for _, tc := range []struct {
		body  string
		inert bool
	}{{`t.Log("cancellation tested")`, true}, {``, true}, {`cancel(); if err != context.Canceled { t.Fatal(err) }`, false}} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc TestProbe(t *testing.T){"+tc.body+"}", 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := inert(file.Decls[0].(*ast.FuncDecl).Body); got != tc.inert {
			t.Fatalf("inert(%q)=%v", tc.body, got)
		}
	}
}
