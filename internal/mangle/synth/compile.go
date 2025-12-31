package synth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
)

func Compile(spec Spec, options Options) (Result, error) {
	if options == (Options{}) {
		options = DefaultOptions()
	}
	if err := ValidateSpec(spec, options); err != nil {
		return Result{}, err
	}

	var lines []string
	var decls []string
	var clauses []string

	if spec.Program.Package != nil {
		line, err := renderPackage(*spec.Program.Package)
		if err != nil {
			return Result{}, err
		}
		lines = append(lines, line)
	}

	for _, use := range spec.Program.Use {
		line, err := renderUse(use)
		if err != nil {
			return Result{}, err
		}
		lines = append(lines, line)
	}

	for _, decl := range spec.Program.Decls {
		line, err := renderDecl(decl)
		if err != nil {
			return Result{}, err
		}
		lines = append(lines, line)
		decls = append(decls, line)
	}

	for _, clauseSpec := range spec.Program.Clauses {
		clause, err := buildClause(clauseSpec)
		if err != nil {
			return Result{}, err
		}
		line := clause.String()
		lines = append(lines, line)
		clauses = append(clauses, line)
	}

	source := strings.Join(lines, "\n")
	unit, err := parse.Unit(strings.NewReader(source))
	if err != nil {
		return Result{}, fmt.Errorf("mangle parse failed: %w", err)
	}
	if _, err := analysis.AnalyzeOneUnit(unit, nil); err != nil {
		return Result{}, fmt.Errorf("mangle analysis failed: %w", err)
	}

	return Result{
		Source:  source,
		Clauses: clauses,
		Decls:   decls,
	}, nil
}

func renderPackage(spec PackageSpec) (string, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return "", NewSpecError("program.package.name", "package name is required")
	}
	var sb strings.Builder
	sb.WriteString("Package ")
	sb.WriteString(spec.Name)
	if len(spec.Atoms) > 0 {
		sb.WriteString(" ")
		atoms, err := renderAtomList(spec.Atoms)
		if err != nil {
			return "", err
		}
		sb.WriteString(atoms)
	}
	sb.WriteString("!")
	return sb.String(), nil
}

func renderUse(spec UseSpec) (string, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return "", NewSpecError("program.use.name", "use name is required")
	}
	var sb strings.Builder
	sb.WriteString("Use ")
	sb.WriteString(spec.Name)
	if len(spec.Atoms) > 0 {
		sb.WriteString(" ")
		atoms, err := renderAtomList(spec.Atoms)
		if err != nil {
			return "", err
		}
		sb.WriteString(atoms)
	}
	sb.WriteString("!")
	return sb.String(), nil
}

func renderDecl(spec DeclSpec) (string, error) {
	head, err := buildAtom(spec.Atom)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("Decl ")
	sb.WriteString(head.String())
	if len(spec.Descr) > 0 {
		sb.WriteString(" descr ")
		atoms, err := renderAtomList(spec.Descr)
		if err != nil {
			return "", err
		}
		sb.WriteString(atoms)
	}
	for _, bound := range spec.Bounds {
		if len(bound.Terms) == 0 {
			continue
		}
		sb.WriteString(" bound [")
		for i, term := range bound.Terms {
			if i > 0 {
				sb.WriteString(", ")
			}
			bt, err := buildBaseTerm(term)
			if err != nil {
				return "", err
			}
			sb.WriteString(bt.String())
		}
		sb.WriteString("]")
	}
	if len(spec.Inclusion) > 0 {
		sb.WriteString(" inclusion ")
		atoms, err := renderAtomList(spec.Inclusion)
		if err != nil {
			return "", err
		}
		sb.WriteString(atoms)
	}
	sb.WriteString(".")
	return sb.String(), nil
}

func renderAtomList(atoms []AtomSpec) (string, error) {
	if len(atoms) == 0 {
		return "[]", nil
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, atom := range atoms {
		if i > 0 {
			sb.WriteString(", ")
		}
		parsed, err := buildAtom(atom)
		if err != nil {
			return "", err
		}
		sb.WriteString(parsed.String())
	}
	sb.WriteString("]")
	return sb.String(), nil
}

func buildClause(spec ClauseSpec) (ast.Clause, error) {
	head, err := buildAtom(spec.Head)
	if err != nil {
		return ast.Clause{}, err
	}
	if len(spec.Body) == 0 {
		return ast.Clause{Head: head}, nil
	}

	premises := make([]ast.Term, 0, len(spec.Body))
	for _, term := range spec.Body {
		parsed, err := buildTerm(term)
		if err != nil {
			return ast.Clause{}, err
		}
		premises = append(premises, parsed)
	}

	var transform *ast.Transform
	if spec.Transform != nil {
		built, err := buildTransform(*spec.Transform)
		if err != nil {
			return ast.Clause{}, err
		}
		transform = &built
	}

	return ast.Clause{
		Head:      head,
		Premises:  premises,
		Transform: transform,
	}, nil
}

func buildTransform(spec TransformSpec) (ast.Transform, error) {
	if len(spec.Statements) == 0 {
		return ast.Transform{}, NewSpecError("transform.statements", "transform statements are required")
	}
	stmts := make([]ast.TransformStmt, 0, len(spec.Statements))
	for _, stmt := range spec.Statements {
		kind := strings.ToLower(strings.TrimSpace(stmt.Kind))
		apply, err := buildApplyFn(stmt.Fn)
		if err != nil {
			return ast.Transform{}, err
		}
		switch kind {
		case "do":
			stmts = append(stmts, ast.TransformStmt{Var: nil, Fn: apply})
		case "let":
			if !isValidVariable(stmt.Var) {
				return ast.Transform{}, NewSpecError("transform.var", "let transform requires a valid variable")
			}
			variable := ast.Variable{Symbol: stmt.Var}
			stmts = append(stmts, ast.TransformStmt{Var: &variable, Fn: apply})
		default:
			return ast.Transform{}, NewSpecError("transform.kind", "transform kind must be \"do\" or \"let\"")
		}
	}
	return ast.Transform{Statements: stmts}, nil
}

func buildTerm(spec TermSpec) (ast.Term, error) {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	switch kind {
	case "atom":
		if spec.Atom == nil {
			return nil, NewSpecError("term.atom", "atom term requires atom")
		}
		atom, err := buildAtom(*spec.Atom)
		if err != nil {
			return nil, err
		}
		return atom, nil
	case "not":
		if spec.Atom == nil {
			return nil, NewSpecError("term.atom", "negated term requires atom")
		}
		atom, err := buildAtom(*spec.Atom)
		if err != nil {
			return nil, err
		}
		return ast.NegAtom{Atom: atom}, nil
	case "eq":
		left, right, err := buildCompareOperands(spec)
		if err != nil {
			return nil, err
		}
		return ast.Eq{Left: left, Right: right}, nil
	case "neq":
		left, right, err := buildCompareOperands(spec)
		if err != nil {
			return nil, err
		}
		return ast.Ineq{Left: left, Right: right}, nil
	case "cmp":
		left, right, err := buildCompareOperands(spec)
		if err != nil {
			return nil, err
		}
		op := strings.ToLower(strings.TrimSpace(spec.Op))
		switch op {
		case "lt":
			return ast.NewAtom(":lt", left, right), nil
		case "le":
			return ast.NewAtom(":le", left, right), nil
		case "gt":
			return ast.NewAtom(":gt", left, right), nil
		case "ge":
			return ast.NewAtom(":ge", left, right), nil
		default:
			return nil, NewSpecError("term.op", "cmp op must be lt, le, gt, or ge")
		}
	default:
		return nil, NewSpecError("term.kind", "term kind must be atom, not, eq, neq, or cmp")
	}
}

func buildCompareOperands(spec TermSpec) (ast.BaseTerm, ast.BaseTerm, error) {
	if spec.Left == nil || spec.Right == nil {
		return nil, nil, NewSpecError("term", "comparison requires left and right")
	}
	left, err := buildBaseTerm(*spec.Left)
	if err != nil {
		return nil, nil, err
	}
	right, err := buildBaseTerm(*spec.Right)
	if err != nil {
		return nil, nil, err
	}
	return left, right, nil
}

func buildAtom(spec AtomSpec) (ast.Atom, error) {
	predicate := strings.TrimSpace(spec.Pred)
	if predicate == "" {
		return ast.Atom{}, NewSpecError("atom.pred", "predicate is required")
	}
	args := make([]ast.BaseTerm, 0, len(spec.Args))
	for _, arg := range spec.Args {
		parsed, err := buildBaseTerm(arg)
		if err != nil {
			return ast.Atom{}, err
		}
		args = append(args, parsed)
	}
	return ast.NewAtom(predicate, args...), nil
}

func buildBaseTerm(spec ExprSpec) (ast.BaseTerm, error) {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	switch kind {
	case "var":
		if !isValidVariable(spec.Value) {
			return nil, NewSpecError("expr.value", "variable must be '_' or start with uppercase letter")
		}
		return ast.Variable{Symbol: spec.Value}, nil
	case "name":
		name, err := ast.Name(spec.Value)
		if err != nil {
			return nil, err
		}
		return name, nil
	case "string":
		return ast.String(spec.Value), nil
	case "bytes":
		return ast.Bytes([]byte(spec.Value)), nil
	case "number":
		if spec.Number != "" {
			value, err := spec.Number.Int64()
			if err != nil {
				return nil, err
			}
			return ast.Number(value), nil
		}
		value, err := parseInt(spec.Value)
		if err != nil {
			return nil, err
		}
		return ast.Number(value), nil
	case "float":
		if spec.Float != nil {
			return ast.Float64(*spec.Float), nil
		}
		value, err := parseFloat(spec.Value)
		if err != nil {
			return nil, err
		}
		return ast.Float64(value), nil
	case "apply":
		return buildApplyFn(spec)
	case "list", "map", "struct":
		apply := spec
		apply.Kind = "apply"
		apply.Function = "fn:" + kind
		return buildApplyFn(apply)
	default:
		return nil, NewSpecError("expr.kind", "expr kind must be var, name, string, bytes, number, float, apply, list, map, or struct")
	}
}

func buildApplyFn(spec ExprSpec) (ast.ApplyFn, error) {
	function := strings.TrimSpace(spec.Function)
	if function == "" {
		return ast.ApplyFn{}, NewSpecError("expr.function", "apply function name is required")
	}
	if !strings.HasPrefix(function, "fn:") {
		return ast.ApplyFn{}, NewSpecError("expr.function", "apply function must start with \"fn:\"")
	}
	args := make([]ast.BaseTerm, 0, len(spec.Args))
	for _, arg := range spec.Args {
		parsed, err := buildBaseTerm(arg)
		if err != nil {
			return ast.ApplyFn{}, err
		}
		args = append(args, parsed)
	}
	arity := len(args)
	if spec.Arity != nil {
		arity = *spec.Arity
	}
	return ast.ApplyFn{Function: ast.FunctionSym{Symbol: function, Arity: arity}, Args: args}, nil
}

func parseInt(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, NewSpecError("expr.value", "number value is required")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseFloat(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, NewSpecError("expr.value", "float value is required")
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
