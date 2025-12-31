package synth

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	predicatePattern = regexp.MustCompile(`^:?[a-z][A-Za-z0-9_:]*(\.[A-Za-z0-9_:]+)*$`)
)

func ValidateSpec(spec Spec, options Options) error {
	if spec.Format != FormatV1 {
		return NewSpecError("format", fmt.Sprintf("expected %q", FormatV1))
	}
	return validateProgramSpec(spec.Program, options)
}

func validateProgramSpec(program ProgramSpec, options Options) error {
	if program.Package == nil && len(program.Use) == 0 && len(program.Decls) == 0 && len(program.Clauses) == 0 {
		return NewSpecError("program", "program must contain at least one clause or declaration")
	}

	if program.Package != nil && !options.AllowPackage {
		return NewSpecError("program.package", "package declarations are not allowed")
	}
	if len(program.Use) > 0 && !options.AllowUse {
		return NewSpecError("program.use", "use declarations are not allowed")
	}
	if len(program.Decls) > 0 && !options.AllowDecls {
		return NewSpecError("program.decls", "decl declarations are not allowed")
	}
	if options.RequireSingleClause && len(program.Clauses) != 1 {
		return NewSpecError("program.clauses", "expected exactly one clause")
	}

	if program.Package != nil {
		if err := validatePackageSpec(*program.Package, "program.package"); err != nil {
			return err
		}
	}
	for i, use := range program.Use {
		if err := validateUseSpec(use, fmt.Sprintf("program.use[%d]", i)); err != nil {
			return err
		}
	}
	for i, decl := range program.Decls {
		if err := validateDeclSpec(decl, fmt.Sprintf("program.decls[%d]", i)); err != nil {
			return err
		}
	}
	for i, clause := range program.Clauses {
		if err := validateClauseSpec(clause, fmt.Sprintf("program.clauses[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

func validatePackageSpec(spec PackageSpec, path string) error {
	if strings.TrimSpace(spec.Name) == "" {
		return NewSpecError(path+".name", "package name is required")
	}
	if !predicatePattern.MatchString(spec.Name) {
		return NewSpecError(path+".name", "package name must be a valid NAME token")
	}
	for i, atom := range spec.Atoms {
		if err := validateAtomSpec(atom, fmt.Sprintf("%s.atoms[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func validateUseSpec(spec UseSpec, path string) error {
	if strings.TrimSpace(spec.Name) == "" {
		return NewSpecError(path+".name", "use name is required")
	}
	if !predicatePattern.MatchString(spec.Name) {
		return NewSpecError(path+".name", "use name must be a valid NAME token")
	}
	for i, atom := range spec.Atoms {
		if err := validateAtomSpec(atom, fmt.Sprintf("%s.atoms[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func validateDeclSpec(spec DeclSpec, path string) error {
	if err := validateAtomSpec(spec.Atom, path+".atom"); err != nil {
		return err
	}
	for i, descr := range spec.Descr {
		if err := validateAtomSpec(descr, fmt.Sprintf("%s.descr[%d]", path, i)); err != nil {
			return err
		}
	}
	for i, bound := range spec.Bounds {
		if len(bound.Terms) == 0 {
			return NewSpecError(fmt.Sprintf("%s.bounds[%d]", path, i), "bound terms are required")
		}
		for j, term := range bound.Terms {
			if err := validateExprSpec(term, fmt.Sprintf("%s.bounds[%d].terms[%d]", path, i, j)); err != nil {
				return err
			}
		}
	}
	for i, atom := range spec.Inclusion {
		if err := validateAtomSpec(atom, fmt.Sprintf("%s.inclusion[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func validateClauseSpec(spec ClauseSpec, path string) error {
	if err := validateAtomSpec(spec.Head, path+".head"); err != nil {
		return err
	}
	for i, term := range spec.Body {
		if err := validateTermSpec(term, fmt.Sprintf("%s.body[%d]", path, i)); err != nil {
			return err
		}
	}
	if spec.Transform != nil {
		if len(spec.Transform.Statements) == 0 {
			return NewSpecError(path+".transform.statements", "transform statements are required")
		}
		for i, stmt := range spec.Transform.Statements {
			if err := validateTransformStmt(stmt, fmt.Sprintf("%s.transform.statements[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTransformStmt(stmt TransformStmtSpec, path string) error {
	kind := strings.ToLower(strings.TrimSpace(stmt.Kind))
	if kind != "do" && kind != "let" {
		return NewSpecError(path+".kind", "transform kind must be \"do\" or \"let\"")
	}
	if kind == "let" && !isValidVariable(stmt.Var) {
		return NewSpecError(path+".var", "let transforms require a valid variable name")
	}
	if err := validateExprSpec(stmt.Fn, path+".fn"); err != nil {
		return err
	}
	if strings.ToLower(stmt.Fn.Kind) != "apply" {
		return NewSpecError(path+".fn.kind", "transform function must be an apply expression")
	}
	return nil
}

func validateTermSpec(spec TermSpec, path string) error {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	switch kind {
	case "atom":
		if spec.Atom == nil {
			return NewSpecError(path+".atom", "atom term requires atom")
		}
		return validateAtomSpec(*spec.Atom, path+".atom")
	case "not":
		if spec.Atom == nil {
			return NewSpecError(path+".atom", "negated term requires atom")
		}
		return validateAtomSpec(*spec.Atom, path+".atom")
	case "eq", "neq":
		if spec.Left == nil || spec.Right == nil {
			return NewSpecError(path, "comparison requires left and right")
		}
		if err := validateExprSpec(*spec.Left, path+".left"); err != nil {
			return err
		}
		return validateExprSpec(*spec.Right, path+".right")
	case "cmp":
		if spec.Left == nil || spec.Right == nil {
			return NewSpecError(path, "comparison requires left and right")
		}
		op := strings.ToLower(strings.TrimSpace(spec.Op))
		if op != "lt" && op != "le" && op != "gt" && op != "ge" {
			return NewSpecError(path+".op", "cmp op must be lt, le, gt, or ge")
		}
		if err := validateExprSpec(*spec.Left, path+".left"); err != nil {
			return err
		}
		return validateExprSpec(*spec.Right, path+".right")
	default:
		return NewSpecError(path+".kind", "term kind must be atom, not, eq, neq, or cmp")
	}
}

func validateAtomSpec(spec AtomSpec, path string) error {
	if strings.TrimSpace(spec.Pred) == "" {
		return NewSpecError(path+".pred", "predicate is required")
	}
	if strings.HasPrefix(spec.Pred, "fn:") {
		return NewSpecError(path+".pred", "predicate must not start with \"fn:\"")
	}
	if !predicatePattern.MatchString(spec.Pred) {
		return NewSpecError(path+".pred", "predicate must be a valid NAME token")
	}
	for i, arg := range spec.Args {
		if err := validateExprSpec(arg, fmt.Sprintf("%s.args[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func validateExprSpec(spec ExprSpec, path string) error {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	switch kind {
	case "var":
		if !isValidVariable(spec.Value) {
			return NewSpecError(path+".value", "variable must be '_' or start with uppercase letter")
		}
	case "name":
		if !strings.HasPrefix(spec.Value, "/") {
			return NewSpecError(path+".value", "name constant must start with '/'")
		}
	case "string":
		// strings may be empty
	case "bytes":
		// bytes may be empty
	case "number":
		if err := validateNumber(spec, path); err != nil {
			return err
		}
	case "float":
		if err := validateFloat(spec, path); err != nil {
			return err
		}
	case "apply":
		if strings.TrimSpace(spec.Function) == "" {
			return NewSpecError(path+".function", "apply function name is required")
		}
		if !strings.HasPrefix(spec.Function, "fn:") {
			return NewSpecError(path+".function", "apply function must start with \"fn:\"")
		}
		if err := validateArity(spec.Arity, len(spec.Args), path+".arity"); err != nil {
			return err
		}
		for i, arg := range spec.Args {
			if err := validateExprSpec(arg, fmt.Sprintf("%s.args[%d]", path, i)); err != nil {
				return err
			}
		}
	case "list", "map", "struct":
		if err := validateArity(spec.Arity, len(spec.Args), path+".arity"); err != nil {
			return err
		}
		if (kind == "map" || kind == "struct") && len(spec.Args)%2 != 0 {
			return NewSpecError(path+".args", "map/struct require even number of args")
		}
		for i, arg := range spec.Args {
			if err := validateExprSpec(arg, fmt.Sprintf("%s.args[%d]", path, i)); err != nil {
				return err
			}
		}
	default:
		return NewSpecError(path+".kind", "expr kind must be var, name, string, bytes, number, float, apply, list, map, or struct")
	}
	return nil
}

func validateNumber(spec ExprSpec, path string) error {
	if spec.Number != "" {
		if _, err := spec.Number.Int64(); err != nil {
			return NewSpecError(path+".number", "number must be an integer")
		}
		return nil
	}
	if spec.Value != "" {
		if _, err := strconv.ParseInt(spec.Value, 10, 64); err != nil {
			return NewSpecError(path+".value", "number value must be an integer")
		}
		return nil
	}
	return NewSpecError(path, "number requires number or value")
}

func validateFloat(spec ExprSpec, path string) error {
	if spec.Float != nil {
		return nil
	}
	if spec.Value != "" {
		if _, err := strconv.ParseFloat(spec.Value, 64); err != nil {
			return NewSpecError(path+".value", "float value must be numeric")
		}
		return nil
	}
	return NewSpecError(path, "float requires float or value")
}

func validateArity(arity *int, argLen int, path string) error {
	if arity == nil {
		return nil
	}
	if *arity == -1 {
		return nil
	}
	if *arity != argLen {
		return NewSpecError(path, fmt.Sprintf("arity %d does not match args length %d", *arity, argLen))
	}
	return nil
}

func isValidVariable(value string) bool {
	if value == "_" {
		return true
	}
	if value == "" {
		return false
	}
	if value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
			return false
		}
	}
	return true
}
