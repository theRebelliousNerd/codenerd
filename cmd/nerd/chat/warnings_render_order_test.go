package chat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// A warning appended after the render is a warning nobody sees.
//
// processInput collects into one `warnings` slice all turn and renders it into
// the response exactly once. That makes position load-bearing in a way nothing
// in the syntax shows: an append below the render compiles, runs, and is
// simply never displayed. Two of them were there -- the kernel-assert loop for
// the model's own Mangle updates and the self-correction hypothesis -- and
// because both also discarded their errors, a rejected fact produced no
// symptom at all beyond the agent later behaving as though it had never been
// told.
//
// Testing the instances would not have caught it, because at the time neither
// appended anything. So this pins the shape instead: every append must precede
// the render.
func TestWarningsAreAppendedBeforeTheyAreRendered(t *testing.T) {
	const file = "process.go"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}

	renderPos := token.NoPos
	var appends []token.Pos
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			// The header the render writes. Matched on the literal because
			// that is the thing a reader recognises as "this is the render".
			if node.Kind == token.STRING && strings.Contains(node.Value, "System Warnings:") {
				if renderPos == token.NoPos || node.Pos() < renderPos {
					renderPos = node.Pos()
				}
			}
		case *ast.AssignStmt:
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			target, ok := node.Lhs[0].(*ast.Ident)
			if !ok || target.Name != "warnings" {
				return true
			}
			call, ok := node.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" {
				appends = append(appends, node.Pos())
			}
		}
		return true
	})

	if renderPos == token.NoPos {
		t.Fatalf("could not find the warnings render in %s; if it moved or was "+
			"renamed, this test needs to follow it rather than be deleted", file)
	}
	if len(appends) == 0 {
		t.Fatalf("found no `warnings = append(...)` in %s, so this test is "+
			"asserting nothing", file)
	}

	renderLine := fset.Position(renderPos).Line
	for _, pos := range appends {
		if pos > renderPos {
			t.Errorf("%s: warnings appended at line %d, after the render at line %d -- "+
				"this warning is collected and never shown",
				file, fset.Position(pos).Line, renderLine)
		}
	}
}
