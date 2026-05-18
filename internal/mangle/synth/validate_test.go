package synth

import (
	"strings"
	"testing"
)

func TestValidateSpec_Format(t *testing.T) {
	err := ValidateSpec(Spec{Format: "wrong"}, DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "expected") {
		t.Errorf("Expected format error, got %v", err)
	}
}

func TestValidateProgramSpec_Empty(t *testing.T) {
	err := validateProgramSpec(ProgramSpec{}, DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "contain at least one") {
		t.Errorf("Expected error for empty program, got %v", err)
	}
}

func TestValidateProgramSpec_Options(t *testing.T) {
	// Program requires at least one clause or decl, etc., so add a dummy clause to bypass empty error
	baseProgram := func() ProgramSpec {
		return ProgramSpec{Clauses: []ClauseSpec{{Head: AtomSpec{Pred: "p"}}}}
	}

	opts := Options{} // all false

	// Test AllowPackage
	p1 := baseProgram()
	p1.Package = &PackageSpec{Name: "foo"}
	err := validateProgramSpec(p1, opts)
	if err == nil || !strings.Contains(err.Error(), "package declarations are not allowed") {
		t.Errorf("Expected error for AllowPackage=false, got %v", err)
	}

	// Test AllowUse
	p2 := baseProgram()
	p2.Use = []UseSpec{{Name: "bar"}}
	err = validateProgramSpec(p2, opts)
	if err == nil || !strings.Contains(err.Error(), "use declarations are not allowed") {
		t.Errorf("Expected error for AllowUse=false, got %v", err)
	}

	// Test AllowDecls
	p3 := baseProgram()
	p3.Decls = []DeclSpec{{Atom: AtomSpec{Pred: "p"}}}
	err = validateProgramSpec(p3, opts)
	if err == nil || !strings.Contains(err.Error(), "decl declarations are not allowed") {
		t.Errorf("Expected error for AllowDecls=false, got %v", err)
	}

	// Test RequireSingleClause
	opts.RequireSingleClause = true
	p4 := baseProgram()
	p4.Clauses = append(p4.Clauses, ClauseSpec{Head: AtomSpec{Pred: "q"}})
	err = validateProgramSpec(p4, opts)
	if err == nil || !strings.Contains(err.Error(), "expected exactly one clause") {
		t.Errorf("Expected error for RequireSingleClause, got %v", err)
	}
}

func TestValidatePackageSpec(t *testing.T) {
	err := validatePackageSpec(PackageSpec{Name: ""}, "path")
	if err == nil || !strings.Contains(err.Error(), "package name is required") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validatePackageSpec(PackageSpec{Name: "Invalid Name"}, "path")
	if err == nil || !strings.Contains(err.Error(), "valid NAME token") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validatePackageSpec(PackageSpec{Name: "valid", Atoms: []AtomSpec{{Pred: ""}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateUseSpec(t *testing.T) {
	err := validateUseSpec(UseSpec{Name: ""}, "path")
	if err == nil || !strings.Contains(err.Error(), "use name is required") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateUseSpec(UseSpec{Name: "Invalid Name"}, "path")
	if err == nil || !strings.Contains(err.Error(), "valid NAME token") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateUseSpec(UseSpec{Name: "valid", Atoms: []AtomSpec{{Pred: ""}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateDeclSpec(t *testing.T) {
	// Invalid Atom
	err := validateDeclSpec(DeclSpec{Atom: AtomSpec{Pred: ""}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}

	// Invalid Descr
	err = validateDeclSpec(DeclSpec{Atom: AtomSpec{Pred: "p"}, Descr: []AtomSpec{{Pred: ""}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}

	// Bounds
	err = validateDeclSpec(DeclSpec{Atom: AtomSpec{Pred: "p"}, Bounds: []BoundSpec{{Terms: nil}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "bound terms are required") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateDeclSpec(DeclSpec{Atom: AtomSpec{Pred: "p"}, Bounds: []BoundSpec{{Terms: []ExprSpec{{Kind: "invalid"}}}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	// Inclusion
	err = validateDeclSpec(DeclSpec{Atom: AtomSpec{Pred: "p"}, Inclusion: []AtomSpec{{Pred: ""}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateClauseSpec(t *testing.T) {
	// Invalid Head
	err := validateClauseSpec(ClauseSpec{Head: AtomSpec{Pred: ""}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}

	// Invalid Body
	err = validateClauseSpec(ClauseSpec{Head: AtomSpec{Pred: "p"}, Body: []TermSpec{{Kind: "invalid"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "term kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	// Transform Statement errors
	err = validateClauseSpec(ClauseSpec{Head: AtomSpec{Pred: "p"}, Transform: &TransformSpec{Statements: nil}}, "path")
	if err == nil || !strings.Contains(err.Error(), "transform statements are required") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateClauseSpec(ClauseSpec{Head: AtomSpec{Pred: "p"}, Transform: &TransformSpec{Statements: []TransformStmtSpec{{Kind: "invalid"}}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "transform kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateTransformStmt(t *testing.T) {
	err := validateTransformStmt(TransformStmtSpec{Kind: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "transform kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateTransformStmt(TransformStmtSpec{Kind: "let", Var: ""}, "path")
	if err == nil || !strings.Contains(err.Error(), "let transforms require a valid variable name") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateTransformStmt(TransformStmtSpec{Kind: "do", Fn: ExprSpec{Kind: "invalid"}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateTransformStmt(TransformStmtSpec{Kind: "do", Fn: ExprSpec{Kind: "string"}}, "path")
	if err == nil || !strings.Contains(err.Error(), "transform function must be an apply expression") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateTermSpec(t *testing.T) {
	// Atom
	err := validateTermSpec(TermSpec{Kind: "atom", Atom: nil}, "path")
	if err == nil || !strings.Contains(err.Error(), "atom term requires atom") {
		t.Errorf("Expected error, got %v", err)
	}

	// Not
	err = validateTermSpec(TermSpec{Kind: "not", Atom: nil}, "path")
	if err == nil || !strings.Contains(err.Error(), "negated term requires atom") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "not", Atom: &AtomSpec{Pred: ""}}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}

	// Eq/Neq
	err = validateTermSpec(TermSpec{Kind: "eq", Left: nil}, "path")
	if err == nil || !strings.Contains(err.Error(), "comparison requires left and right") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "eq", Left: &ExprSpec{Kind: "invalid"}, Right: &ExprSpec{Kind: "string"}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "eq", Left: &ExprSpec{Kind: "string"}, Right: &ExprSpec{Kind: "invalid"}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	// Cmp
	err = validateTermSpec(TermSpec{Kind: "cmp", Left: nil}, "path")
	if err == nil || !strings.Contains(err.Error(), "comparison requires left and right") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "cmp", Left: &ExprSpec{Kind: "string"}, Right: &ExprSpec{Kind: "string"}, Op: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "cmp op must be") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "cmp", Left: &ExprSpec{Kind: "invalid"}, Right: &ExprSpec{Kind: "string"}, Op: "lt"}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "cmp", Left: &ExprSpec{Kind: "string"}, Right: &ExprSpec{Kind: "invalid"}, Op: "lt"}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateTermSpec(TermSpec{Kind: "cmp", Left: &ExprSpec{Kind: "string"}, Right: &ExprSpec{Kind: "string"}, Op: "lt"}, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Invalid Kind
	err = validateTermSpec(TermSpec{Kind: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "term kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateAtomSpec(t *testing.T) {
	err := validateAtomSpec(AtomSpec{Pred: ""}, "path")
	if err == nil || !strings.Contains(err.Error(), "predicate is required") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateAtomSpec(AtomSpec{Pred: "fn:foo"}, "path")
	if err == nil || !strings.Contains(err.Error(), "must not start with \"fn:\"") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateAtomSpec(AtomSpec{Pred: "Invalid_Pattern"}, "path")
	if err == nil || !strings.Contains(err.Error(), "must be a valid NAME token") {
		t.Errorf("Expected error, got %v", err)
	}

	err = validateAtomSpec(AtomSpec{Pred: "valid", Args: []ExprSpec{{Kind: "invalid"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateExprSpec(t *testing.T) {
	// var
	err := validateExprSpec(ExprSpec{Kind: "var", Value: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "variable must be '_' or start with uppercase") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "var", Value: "V!@#"}, "path")
	if err == nil || !strings.Contains(err.Error(), "variable must be '_' or start with uppercase") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "var", Value: "_"}, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// name
	err = validateExprSpec(ExprSpec{Kind: "name", Value: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "name constant must start with '/'") {
		t.Errorf("Expected error, got %v", err)
	}

	// number
	err = validateExprSpec(ExprSpec{Kind: "number", Value: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "number value must be an integer") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "number"}, "path")
	if err == nil || !strings.Contains(err.Error(), "number requires number or value") {
		t.Errorf("Expected error, got %v", err)
	}

	// float
	err = validateExprSpec(ExprSpec{Kind: "float", Value: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "float value must be numeric") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "float"}, "path")
	if err == nil || !strings.Contains(err.Error(), "float requires float or value") {
		t.Errorf("Expected error, got %v", err)
	}

	// apply
	err = validateExprSpec(ExprSpec{Kind: "apply", Function: ""}, "path")
	if err == nil || !strings.Contains(err.Error(), "apply function name is required") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "apply", Function: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "apply function must start with \"fn:\"") {
		t.Errorf("Expected error, got %v", err)
	}
	arity := 2
	err = validateExprSpec(ExprSpec{Kind: "apply", Function: "fn:foo", Arity: &arity, Args: []ExprSpec{{Kind: "string"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "does not match args length") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "apply", Function: "fn:foo", Args: []ExprSpec{{Kind: "invalid"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	// map / struct / list
	err = validateExprSpec(ExprSpec{Kind: "map", Arity: &arity, Args: []ExprSpec{{Kind: "string"}, {Kind: "string"}, {Kind: "string"}, {Kind: "string"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "does not match args length") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "map", Args: []ExprSpec{{Kind: "string"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "require even number of args") {
		t.Errorf("Expected error, got %v", err)
	}
	err = validateExprSpec(ExprSpec{Kind: "map", Args: []ExprSpec{{Kind: "invalid"}, {Kind: "string"}}}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}

	// invalid
	err = validateExprSpec(ExprSpec{Kind: "invalid"}, "path")
	if err == nil || !strings.Contains(err.Error(), "expr kind must be") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateArity(t *testing.T) {
	// Nil arity
	err := validateArity(nil, 0, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// -1 arity
	arityAny := -1
	err = validateArity(&arityAny, 5, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// match
	arityMatch := 2
	err = validateArity(&arityMatch, 2, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// mismatch
	err = validateArity(&arityMatch, 3, "path")
	if err == nil || !strings.Contains(err.Error(), "does not match args length") {
		t.Errorf("Expected error, got %v", err)
	}
}

func TestValidateFloat(t *testing.T) {
	var f float64 = 1.0
	err := validateFloat(ExprSpec{Float: &f}, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = validateFloat(ExprSpec{Value: "1.0"}, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateNumber(t *testing.T) {
	err := validateNumber(ExprSpec{Value: "10"}, "path")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestIsValidVariable(t *testing.T) {
	valid := []string{"_", "A", "Var", "VAR", "Var1", "Var_1", "Z"}
	invalid := []string{"", "a", "var", "1Var", "V@r", "V-ar"}

	for _, v := range valid {
		if !isValidVariable(v) {
			t.Errorf("Expected %q to be a valid variable", v)
		}
	}

	for _, v := range invalid {
		if isValidVariable(v) {
			t.Errorf("Expected %q to be an invalid variable", v)
		}
	}
}

func TestValidateSpec_HappyPath(t *testing.T) {
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{
				{
					Head: AtomSpec{
						Pred: "p",
						Args: []ExprSpec{
							{Kind: "var", Value: "X"},
						},
					},
					Body: []TermSpec{
						{
							Kind: "atom",
							Atom: &AtomSpec{
								Pred: "q",
								Args: []ExprSpec{
									{Kind: "var", Value: "X"},
								},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateSpec(spec, DefaultOptions())
	if err != nil {
		t.Errorf("Expected nil error for happy path, got: %v", err)
	}
}

func TestValidateProgramSpec_HappyPath(t *testing.T) {
	program := ProgramSpec{
		Package: &PackageSpec{
			Name: "my_package",
			Atoms: []AtomSpec{
				{
					Pred: "p",
				},
			},
		},
		Use: []UseSpec{
			{
				Name: "other_package",
			},
		},
		Decls: []DeclSpec{
			{
				Atom: AtomSpec{
					Pred: "q",
				},
				Bounds: []BoundSpec{
					{
						Terms: []ExprSpec{
							{Kind: "var", Value: "X"},
						},
					},
				},
			},
		},
		Clauses: []ClauseSpec{
			{
				Head: AtomSpec{
					Pred: "p",
					Args: []ExprSpec{
						{Kind: "var", Value: "X"},
					},
				},
				Body: []TermSpec{
					{
						Kind: "atom",
						Atom: &AtomSpec{
							Pred: "q",
							Args: []ExprSpec{
								{Kind: "var", Value: "X"},
							},
						},
					},
				},
			},
		},
	}

	err := validateProgramSpec(program, DefaultOptions())
	if err != nil {
		t.Errorf("Expected nil error for happy path, got: %v", err)
	}
}
