package session

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"codenerd/internal/types"
)

// FunctionMetrics holds per-function measurements used for modularity policy.
// Complexity counts branching constructs as described below: base one plus one
// for each if, for, range, case and each && / || operator encountered in the
// function body.
type FunctionMetrics struct {
	Name       string
	Lines      int
	ParamCount int
	Complexity int
	Depth      int
}

// parseFunctionMetrics is a pure function that parses Go source text with
// go/ast and returns a metric record for each function or method declaration.
// Methods are named as Receiver.Method so a reader can distinguish them from
// plain functions. Source that does not parse as Go yields no metrics and no
// error so a partial edit or a non-Go file never blocks evaluation.
func parseFunctionMetrics(src string) []FunctionMetrics {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil || f == nil {
		return nil
	}
	var out []FunctionMetrics
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			if recv := receiverName(fn.Recv.List[0].Type); recv != "" {
				name = recv + "." + name
			}
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		lines := 0
		if end >= start {
			lines = end - start + 1
		} else {
			lines = 1
		}
		paramCount := 0
		if fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				if len(field.Names) > 0 {
					paramCount += len(field.Names)
				} else {
					paramCount++
				}
			}
		}
		complexity := complexityForFunc(fn)
		depth := nestingForFunc(fn)
		out = append(out, FunctionMetrics{
			Name:       name,
			Lines:      lines,
			ParamCount: paramCount,
			Complexity: complexity,
			Depth:      depth,
		})
	}
	return out
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

// complexityForFunc counts branching constructs for cyclomatic-style complexity.
// Convention: one plus number of if, for, range, case (including CommClause for
// select) and each && / || binary operator encountered in the body.
func complexityForFunc(fn *ast.FuncDecl) int {
	if fn.Body == nil {
		return 1
	}
	complexity := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

// nestingForFunc returns the maximum nesting depth of control structures inside
// the function. Only control constructs contribute to depth: if, for, range,
// switch, type-switch and select. The walk uses an ast.Visitor that threads
// the current depth so sibling branches do not accumulate each other's depth.
func nestingForFunc(fn *ast.FuncDecl) int {
	if fn.Body == nil {
		return 0
	}
	maxDepth := 0
	ast.Walk(nestingVisitor{depth: 0, max: &maxDepth}, fn.Body)
	return maxDepth
}

type nestingVisitor struct {
	depth int
	max     *int
}

func (v nestingVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	next := v.depth
	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		next++
		if next > *v.max {
			*v.max = next
		}
	}
	return nestingVisitor{depth: next, max: v.max}
}

// metricsToFacts turns parsed metrics into kernel facts that match the EDB
// Decls exactly: function_metrics(File, FuncName, Lines, Complexity),
// function_params(File, FuncName, ParamCount) and
// function_nesting(File, FuncName, Depth). File and FuncName are declared
// /string so they are stored via types.MangleString to avoid the defect where
// a Go string that looks like a name constant is silently stored as one; a
// path can start with a slash and must remain a string.
func metricsToFacts(path string, metrics []FunctionMetrics) []types.Fact {
	if len(metrics) == 0 {
		return nil
	}
	facts := make([]types.Fact, 0, len(metrics)*3)
	for _, m := range metrics {
		facts = append(facts,
			types.Fact{
				Predicate: "function_metrics",
				Args: []any{
					types.MangleString(path),
					types.MangleString(m.Name),
					int64(m.Lines),
					int64(m.Complexity),
				},
			},
			types.Fact{
				Predicate: "function_params",
				Args: []any{
					types.MangleString(path),
					types.MangleString(m.Name),
					int64(m.ParamCount),
				},
			},
			types.Fact{
				Predicate: "function_nesting",
				Args: []any{
					types.MangleString(path),
					types.MangleString(m.Name),
					int64(m.Depth),
				},
			},
		)
	}
	return facts
}

// modularityKernel is the narrow kernel surface needed for modularity
// evaluation. Defining a local interface instead of depending on a concrete
// kernel type lets tests supply a real kernel or a fake.
type modularityKernel interface {
	AssertBatch(facts []types.Fact) error
	Query(predicate string) ([]types.Fact, error)
	RetractFact(fact types.Fact) error
	RemoveFactsByPredicateSet(predicates map[string]struct{}) error
}

// evaluateModularity asserts modularity facts for path/content, queries the
// four derived predicates defined in coder_quality.mg and collects a readable
// violation line for each result belonging to this path. The facts are always
// retracted before return so evaluating a proposed file leaves no trace in the
// kernel. Content that does not parse as Go yields no violations and no error.
func evaluateModularity(k modularityKernel, path string, content string) ([]string, error) {
	metrics := parseFunctionMetrics(content)
	if len(metrics) == 0 {
		// Either the file does not parse or it contains no functions; in both
		// cases there is nothing to evaluate and the caller must not be blocked.
		return nil, nil
	}
	facts := metricsToFacts(path, metrics)
	if len(facts) == 0 {
		return nil, nil
	}
	// Ensure the kernel is clean even when a query or assertion fails.
	defer func() {
		for _, pred := range []string{"function_metrics", "function_params", "function_nesting"} {
			_ = k.RetractFact(types.Fact{Predicate: pred, Args: []any{types.MangleString(path)}})
		}
		_ = k.RemoveFactsByPredicateSet(map[string]struct{}{
			"function_metrics": {},
			"function_params":  {},
			"function_nesting": {},
		})
	}()

	if err := k.AssertBatch(facts); err != nil {
		return nil, fmt.Errorf("assert modularity facts: %w", err)
	}

	// Collect violations for predicates that are derived from the metrics.
	derived := []string{"function_too_long", "complexity_too_high", "too_many_params", "deep_nesting"}
	var violations []string
	for _, pred := range derived {
		results, err := k.Query(pred)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", pred, err)
		}
		for _, f := range results {
			if len(f.Args) < 2 {
				continue
			}
			fileArg := strings.TrimSpace(fmt.Sprint(f.Args[0]))
			funcArg := strings.TrimSpace(fmt.Sprint(f.Args[1]))
			if fileArg != strings.TrimSpace(path) {
				continue
			}
			// Name the function and the rule it violated.
			violations = append(violations, fmt.Sprintf("%s: %s in %s", funcArg, pred, fileArg))
		}
	}
	return violations, nil
}
