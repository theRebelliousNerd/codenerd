package session

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// A function taken out whole is a coarse question: R1-11's fix put both
// directions of an overlap check inside one function, so the function's
// removal failed the tests either way while the direction the report was
// about was pinned by nothing (N22b). The finer question is asked here: every
// condition on a line the turn changed is forced to true and to false, one at
// a time, and the turn's own tests are run against it. A condition the tests
// do not notice being forced is a decision the change made that nothing
// checks.
//
// Only conditions on changed lines are asked about, so the count is bounded
// by the size of the change and not by the file. A condition that is already
// a constant is not a decision, and a mutant that does not compile is no
// evidence either way -- as everywhere else in this gate.

// conditionUnits is every condition on a line the turn changed in cur,
// forced true and forced false.
func conditionUnits(path, cur string, pre PreImage) []pinUnit {
	changed := changedLines(pre.Content, cur)
	if len(changed) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", cur, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var units []pinUnit
	ast.Inspect(f, func(n ast.Node) bool {
		cond, init, body := branchParts(n)
		if cond == nil || body == nil || isConstantCondition(cond) {
			return true
		}
		from, to := fset.Position(cond.Pos()), fset.Position(cond.End())
		if !linesChanged(changed, from.Line, to.Line) {
			return true
		}
		for _, forced := range forcings(n) {
			content := cur
			// The body first: an edit at a later offset does not move an
			// earlier one. A forced condition can leave the names its own
			// init declared unused, which is a compile error in Go, so the
			// body keeps them used.
			if used := keepUsed(initNames(init)); used != "" {
				at := fset.Position(body.Lbrace).Offset + 1
				content = content[:at] + "\n" + used + content[at:]
			}
			content = content[:from.Offset] + forced + content[to.Offset:]
			units = append(units, pinUnit{
				path:    path,
				name:    fmt.Sprintf("line %d, the condition %s forced %s", from.Line, quoteCondition(cur[from.Offset:to.Offset]), forced),
				forced:  true,
				content: content,
			})
		}
		return true
	})
	return units
}

// forcings are the constants a condition is held at. A loop condition held
// true is a loop that never ends, which measures the clock and not the tests,
// so a loop is only asked what happens when it does not run.
func forcings(n ast.Node) []string {
	if _, isFor := n.(*ast.ForStmt); isFor {
		return []string{"false"}
	}
	return []string{"true", "false"}
}

// branchParts is the condition, the init statement and the body of an if or a
// for, and nil for anything else.
func branchParts(n ast.Node) (ast.Expr, ast.Stmt, *ast.BlockStmt) {
	switch s := n.(type) {
	case *ast.IfStmt:
		return s.Cond, s.Init, s.Body
	case *ast.ForStmt:
		return s.Cond, s.Init, s.Body
	}
	return nil, nil, nil
}

// isConstantCondition reports whether the condition is already a constant:
// forcing it is not a question about the change.
func isConstantCondition(cond ast.Expr) bool {
	id, ok := cond.(*ast.Ident)
	return ok && (id.Name == "true" || id.Name == "false")
}

// initNames are the names an if or for statement's init declares.
func initNames(init ast.Stmt) []string {
	assign, ok := init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil
	}
	var names []string
	for _, lhs := range assign.Lhs {
		if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
			names = append(names, id.Name)
		}
	}
	sort.Strings(names)
	return names
}

func keepUsed(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "_ = %s\n", n)
	}
	return b.String()
}

// quoteCondition renders a condition for the listing the model reads, on one
// line.
func quoteCondition(src string) string {
	one := strings.Join(strings.Fields(src), " ")
	return "`" + one + "`"
}

// linesChanged reports whether any line in [from, to] is one the turn
// changed.
func linesChanged(changed []LineRange, from, to int) bool {
	for _, r := range changed {
		if from <= r.End && r.Start <= to {
			return true
		}
	}
	return false
}
