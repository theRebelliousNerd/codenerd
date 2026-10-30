package world

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Review measurements for a Go file: the inputs reviewer.mg judges.
//
//	file_line_count(File, Lines)
//	cyclomatic_complexity(File, Function, Complexity)
//	nesting_depth(File, Function, Depth)
//	detected_security_flow(File, Line, SinkType, SourceType, Confidence)
//
// These are measurements only. Whether a function is too complex, a file too
// long, or a flow a finding is policy (reviewer.mg thresholds and
// flow_security_rule), so a threshold changes in one place and the facts replay.
//
// Ownership: scanner-owned (ScannerPredicates). A full scan replaces them
// wholesale; a delta scan retracts a file's previous rows with the rest of that
// file's facts. Test files are not measured: fixtures hold credentials and
// shell-outs by design, and the reviewer judges the code that ships.
//
// The security measurement is deliberately narrow. It reports a sink only when
// the dangerous argument is dynamic: built by concatenation or Sprintf, a
// function parameter, or a call to a known input source. A constant query or a
// literal command is never a flow. SourceType says what the dynamic value came
// from (/user_input, /env_var, /file_read, /parameter, /dynamic), Confidence how
// sure the scanner is (a parameter or input call 80; bare concatenation 60;
// weak crypto and literal credentials are certain about what they saw).

const maxReviewMeasureBytes = 2 << 20

// goReviewFacts measures one non-test Go file. A file that does not parse still
// yields its line count.
func goReviewFacts(canonical string, content []byte) []core.Fact {
	if len(content) > maxReviewMeasureBytes {
		return nil
	}
	facts := []core.Fact{{
		Predicate: "file_line_count",
		Args:      []any{canonical, int64(lineCount(content))},
	}}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, canonical, content, 0)
	if err != nil {
		logging.WorldDebug("review measurements: parse failed for %s: %v", canonical, err)
		return facts
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := funcDisplayName(fn)
		facts = append(facts,
			core.Fact{Predicate: "cyclomatic_complexity", Args: []any{canonical, name, int64(cyclomatic(fn.Body))}},
			core.Fact{Predicate: "nesting_depth", Args: []any{canonical, name, int64(maxNesting(fn.Body))}},
		)
		facts = append(facts, securityFlows(canonical, fset, fn)...)
	}
	facts = append(facts, hardcodedCredentials(canonical, fset, file)...)
	return facts
}

func lineCount(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := strings.Count(string(content), "\n")
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}

func funcDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name + "." + fn.Name.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name + "." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

// cyclomatic is McCabe's count: one plus each decision point.
func cyclomatic(body *ast.BlockStmt) int {
	n := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch s := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			if s.List != nil {
				n++
			}
		case *ast.CommClause:
			if s.Comm != nil {
				n++
			}
		case *ast.BinaryExpr:
			if s.Op == token.LAND || s.Op == token.LOR {
				n++
			}
		case *ast.FuncLit:
			return true
		}
		return true
	})
	return n
}

// maxNesting is the deepest chain of control-flow blocks.
func maxNesting(body *ast.BlockStmt) int {
	var walk func(node ast.Node, depth int) int
	walk = func(node ast.Node, depth int) int {
		deepest := depth
		ast.Inspect(node, func(child ast.Node) bool {
			if child == nil || child == node {
				return true
			}
			switch child.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				if d := walk(child, depth+1); d > deepest {
					deepest = d
				}
				return false
			}
			return true
		})
		return deepest
	}
	return walk(body, 0)
}

type sinkSpec struct {
	sinkType string
	argIndex int
}

// sqlSinks are database/sql style methods, keyed by selector name: the index of
// the query-text argument.
var sqlSinks = map[string]sinkSpec{
	"Exec": {"/sql_sink", 0}, "Query": {"/sql_sink", 0}, "QueryRow": {"/sql_sink", 0}, "Prepare": {"/sql_sink", 0},
	"ExecContext": {"/sql_sink", 1}, "QueryContext": {"/sql_sink", 1}, "QueryRowContext": {"/sql_sink", 1}, "PrepareContext": {"/sql_sink", 1},
}

// inputSources are calls whose result is outside the program's control.
var inputSources = map[string]string{
	"FormValue": "/user_input", "PostFormValue": "/user_input", "Cookie": "/user_input",
	"Getenv": "/env_var", "LookupEnv": "/env_var",
	"ReadFile": "/file_read", "ReadAll": "/file_read",
}

func securityFlows(canonical string, fset *token.FileSet, fn *ast.FuncDecl) []core.Fact {
	params := map[string]bool{}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, id := range field.Names {
				params[id.Name] = true
			}
		}
	}
	var facts []core.Fact
	emit := func(call ast.Node, sink, source string, confidence int64) {
		facts = append(facts, core.Fact{
			Predicate: "detected_security_flow",
			Args:      []any{canonical, int64(fset.Position(call.Pos()).Line), types.MangleAtom(sink), types.MangleAtom(source), confidence},
		})
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if spec, isSQL := sqlSinks[sel.Sel.Name]; isSQL && spec.argIndex < len(call.Args) {
			if source, conf, dynamic := dynamicValue(call.Args[spec.argIndex], params); dynamic {
				emit(call, spec.sinkType, source, conf)
			}
		}
		if pkg, ok := sel.X.(*ast.Ident); ok {
			switch {
			case pkg.Name == "exec" && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext"):
				start := 0
				if sel.Sel.Name == "CommandContext" {
					start = 1
				}
				for i := start; i < len(call.Args); i++ {
					if source, conf, dynamic := dynamicValue(call.Args[i], params); dynamic && source != "/dynamic" {
						emit(call, "/command_sink", source, conf)
						break
					}
				}
			case (pkg.Name == "md5" || pkg.Name == "sha1" || pkg.Name == "des" || pkg.Name == "rc4") &&
				(sel.Sel.Name == "New" || strings.HasPrefix(sel.Sel.Name, "Sum") || sel.Sel.Name == "NewCipher"):
				emit(call, "/weak_crypto", "/internal", 90)
			}
		}
		return true
	})
	return facts
}

// dynamicValue classifies an argument that is not a constant: its source and
// the scanner's confidence. A literal, or a name the scanner cannot connect to
// a parameter or input, is not dynamic for the command sink (a local variable
// is usually a constant in disguise); concatenation and Sprintf are dynamic.
func dynamicValue(expr ast.Expr, params map[string]bool) (source string, confidence int64, dynamic bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return "", 0, false
	case *ast.ParenExpr:
		return dynamicValue(e.X, params)
	case *ast.Ident:
		if params[e.Name] {
			return "/parameter", 80, true
		}
		return "", 0, false
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", 0, false
		}
		ls, lc, ld := dynamicValue(e.X, params)
		rs, rc, rd := dynamicValue(e.Y, params)
		switch {
		case ld && ls != "/dynamic":
			return ls, lc, true
		case rd && rs != "/dynamic":
			return rs, rc, true
		}
		if _, lit := e.X.(*ast.BasicLit); lit {
			if _, lit2 := e.Y.(*ast.BasicLit); lit2 {
				return "", 0, false
			}
		}
		_ = lc
		_ = rc
		return "/dynamic", 60, true
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if src, ok := inputSources[sel.Sel.Name]; ok {
				return src, 80, true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && sel.Sel.Name == "Sprintf" {
				for _, arg := range e.Args[min(1, len(e.Args)):] {
					if s, c, d := dynamicValue(arg, params); d && s != "/dynamic" {
						return s, c, true
					}
				}
				return "/dynamic", 60, true
			}
		}
	}
	return "", 0, false
}

var credentialNames = []string{"password", "passwd", "secret", "apikey", "api_key", "token", "privatekey"}

// hardcodedCredentials finds a string literal assigned to a credential-named
// identifier. Short literals and obvious placeholders are skipped.
func hardcodedCredentials(canonical string, fset *token.FileSet, file *ast.File) []core.Fact {
	var facts []core.Fact
	check := func(name string, value ast.Expr, pos token.Pos) {
		lit, ok := value.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || len(lit.Value) < 10 {
			return
		}
		lower := strings.ToLower(name)
		for _, c := range credentialNames {
			if strings.Contains(lower, c) {
				facts = append(facts, core.Fact{
					Predicate: "detected_security_flow",
					Args:      []any{canonical, int64(fset.Position(pos).Line), types.MangleAtom("/hardcoded_secret"), types.MangleAtom("/literal"), int64(70)},
				})
				return
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch s := node.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						check(id.Name, s.Rhs[i], s.Pos())
					}
				}
			}
		case *ast.ValueSpec:
			if len(s.Names) == len(s.Values) {
				for i, id := range s.Names {
					check(id.Name, s.Values[i], s.Pos())
				}
			}
		}
		return true
	})
	return facts
}
