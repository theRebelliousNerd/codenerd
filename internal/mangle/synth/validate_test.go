package synth

import (
	"strings"
	"testing"
)

func TestValidateSpec(t *testing.T) {
	opts := DefaultOptions()
	tests := []struct {
		name    string
		spec    Spec
		wantErr bool
		errStr  string
	}{
		{
			name: "invalid format",
			spec: Spec{
				Format: "invalid_format",
			},
			wantErr: true,
			errStr:  "format: expected \"mangle_synth_v1\"",
		},
		{
			name: "valid format, invalid program (empty)",
			spec: Spec{
				Format: FormatV1,
			},
			wantErr: true,
			errStr:  "program: program must contain at least one clause or declaration",
		},
		{
			name: "valid spec",
			spec: Spec{
				Format: FormatV1,
				Program: ProgramSpec{
					Clauses: []ClauseSpec{
						{
							Head: AtomSpec{
								Pred: "test",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpec(tt.spec, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("ValidateSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateProgramSpec(t *testing.T) {
	tests := []struct {
		name    string
		program ProgramSpec
		options Options
		wantErr bool
		errStr  string
	}{
		{
			name:    "empty program",
			program: ProgramSpec{},
			options: DefaultOptions(),
			wantErr: true,
			errStr:  "program must contain at least one clause or declaration",
		},
		{
			name: "package not allowed",
			program: ProgramSpec{
				Package: &PackageSpec{Name: "pkg"},
			},
			options: Options{AllowPackage: false},
			wantErr: true,
			errStr:  "package declarations are not allowed",
		},
		{
			name: "use not allowed",
			program: ProgramSpec{
				Use: []UseSpec{{Name: "use"}},
			},
			options: Options{AllowUse: false},
			wantErr: true,
			errStr:  "use declarations are not allowed",
		},
		{
			name: "decl not allowed",
			program: ProgramSpec{
				Decls: []DeclSpec{{Atom: AtomSpec{Pred: "decl"}}},
			},
			options: Options{AllowDecls: false},
			wantErr: true,
			errStr:  "decl declarations are not allowed",
		},
		{
			name: "require single clause, got 0",
			program: ProgramSpec{
				Decls: []DeclSpec{{Atom: AtomSpec{Pred: "decl"}}}, // At least one decl so not empty
			},
			options: Options{RequireSingleClause: true, AllowDecls: true},
			wantErr: true,
			errStr:  "expected exactly one clause",
		},
		{
			name: "require single clause, got 2",
			program: ProgramSpec{
				Clauses: []ClauseSpec{
					{Head: AtomSpec{Pred: "c1"}},
					{Head: AtomSpec{Pred: "c2"}},
				},
			},
			options: Options{RequireSingleClause: true},
			wantErr: true,
			errStr:  "expected exactly one clause",
		},
		{
			name: "invalid package name",
			program: ProgramSpec{
				Package: &PackageSpec{Name: ""},
			},
			options: Options{AllowPackage: true},
			wantErr: true,
			errStr:  "package name is required",
		},
		{
			name: "invalid use name",
			program: ProgramSpec{
				Use: []UseSpec{{Name: ""}},
			},
			options: Options{AllowUse: true},
			wantErr: true,
			errStr:  "use name is required",
		},
		{
			name: "invalid decl",
			program: ProgramSpec{
				Decls: []DeclSpec{{Atom: AtomSpec{Pred: ""}}},
			},
			options: Options{AllowDecls: true},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "invalid clause",
			program: ProgramSpec{
				Clauses: []ClauseSpec{{Head: AtomSpec{Pred: ""}}},
			},
			options: DefaultOptions(),
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "valid program",
			program: ProgramSpec{
				Package: &PackageSpec{Name: "pkg"},
				Use:     []UseSpec{{Name: "use_pkg"}},
				Decls:   []DeclSpec{{Atom: AtomSpec{Pred: "decl_pred"}}},
				Clauses: []ClauseSpec{{Head: AtomSpec{Pred: "clause_pred"}}},
			},
			options: Options{AllowPackage: true, AllowUse: true, AllowDecls: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProgramSpec(tt.program, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProgramSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateProgramSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidatePackageSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    PackageSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "empty name",
			spec:    PackageSpec{Name: "  "},
			wantErr: true,
			errStr:  "package name is required",
		},
		{
			name:    "invalid name token",
			spec:    PackageSpec{Name: "123invalid"},
			wantErr: true,
			errStr:  "package name must be a valid NAME token",
		},
		{
			name: "invalid atom",
			spec: PackageSpec{
				Name:  "valid",
				Atoms: []AtomSpec{{Pred: ""}},
			},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "valid",
			spec: PackageSpec{
				Name:  "valid.pkg",
				Atoms: []AtomSpec{{Pred: "pred"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePackageSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePackageSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validatePackageSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateUseSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    UseSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "empty name",
			spec:    UseSpec{Name: "  "},
			wantErr: true,
			errStr:  "use name is required",
		},
		{
			name:    "invalid name token",
			spec:    UseSpec{Name: "123invalid"},
			wantErr: true,
			errStr:  "use name must be a valid NAME token",
		},
		{
			name: "invalid atom",
			spec: UseSpec{
				Name:  "valid",
				Atoms: []AtomSpec{{Pred: ""}},
			},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "valid",
			spec: UseSpec{
				Name:  "valid.pkg",
				Atoms: []AtomSpec{{Pred: "pred"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUseSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUseSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateUseSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateDeclSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    DeclSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "invalid atom",
			spec:    DeclSpec{Atom: AtomSpec{Pred: ""}},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "invalid descr",
			spec: DeclSpec{
				Atom:  AtomSpec{Pred: "valid"},
				Descr: []AtomSpec{{Pred: ""}},
			},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "empty bound terms",
			spec: DeclSpec{
				Atom:   AtomSpec{Pred: "valid"},
				Bounds: []BoundSpec{{Terms: nil}},
			},
			wantErr: true,
			errStr:  "bound terms are required",
		},
		{
			name: "invalid bound term",
			spec: DeclSpec{
				Atom:   AtomSpec{Pred: "valid"},
				Bounds: []BoundSpec{{Terms: []ExprSpec{{Kind: "invalid"}}}},
			},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name: "invalid inclusion",
			spec: DeclSpec{
				Atom:      AtomSpec{Pred: "valid"},
				Inclusion: []AtomSpec{{Pred: ""}},
			},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "valid",
			spec: DeclSpec{
				Atom:      AtomSpec{Pred: "valid"},
				Descr:     []AtomSpec{{Pred: "descr"}},
				Bounds:    []BoundSpec{{Terms: []ExprSpec{{Kind: "var", Value: "A"}}}},
				Inclusion: []AtomSpec{{Pred: "incl"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeclSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeclSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateDeclSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateClauseSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    ClauseSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "invalid head atom",
			spec:    ClauseSpec{Head: AtomSpec{Pred: ""}},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name: "invalid body term",
			spec: ClauseSpec{
				Head: AtomSpec{Pred: "valid"},
				Body: []TermSpec{{Kind: "invalid"}},
			},
			wantErr: true,
			errStr:  "term kind must be",
		},
		{
			name: "empty transform statements",
			spec: ClauseSpec{
				Head:      AtomSpec{Pred: "valid"},
				Transform: &TransformSpec{Statements: nil},
			},
			wantErr: true,
			errStr:  "transform statements are required",
		},
		{
			name: "invalid transform statement",
			spec: ClauseSpec{
				Head: AtomSpec{Pred: "valid"},
				Transform: &TransformSpec{
					Statements: []TransformStmtSpec{{Kind: "invalid"}},
				},
			},
			wantErr: true,
			errStr:  "transform kind must be",
		},
		{
			name: "valid",
			spec: ClauseSpec{
				Head: AtomSpec{Pred: "valid"},
				Body: []TermSpec{{Kind: "atom", Atom: &AtomSpec{Pred: "body_pred"}}},
				Transform: &TransformSpec{
					Statements: []TransformStmtSpec{
						{Kind: "do", Fn: ExprSpec{Kind: "apply", Function: "fn:foo"}},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClauseSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateClauseSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateClauseSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateTransformStmt(t *testing.T) {
	tests := []struct {
		name    string
		stmt    TransformStmtSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "invalid kind",
			stmt:    TransformStmtSpec{Kind: "invalid", Fn: ExprSpec{Kind: "apply", Function: "fn:foo"}},
			wantErr: true,
			errStr:  "transform kind must be \"do\" or \"let\"",
		},
		{
			name:    "let without valid var",
			stmt:    TransformStmtSpec{Kind: "let", Var: "invalidVar", Fn: ExprSpec{Kind: "apply", Function: "fn:foo"}},
			wantErr: true,
			errStr:  "let transforms require a valid variable name",
		},
		{
			name:    "invalid fn expr",
			stmt:    TransformStmtSpec{Kind: "do", Fn: ExprSpec{Kind: "invalid"}},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "fn is not apply",
			stmt:    TransformStmtSpec{Kind: "do", Fn: ExprSpec{Kind: "var", Value: "A"}},
			wantErr: true,
			errStr:  "transform function must be an apply expression",
		},
		{
			name:    "valid do",
			stmt:    TransformStmtSpec{Kind: "do", Fn: ExprSpec{Kind: "apply", Function: "fn:foo"}},
			wantErr: false,
		},
		{
			name:    "valid let",
			stmt:    TransformStmtSpec{Kind: "let", Var: "A", Fn: ExprSpec{Kind: "apply", Function: "fn:foo"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransformStmt(tt.stmt, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransformStmt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateTransformStmt() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateTermSpec(t *testing.T) {
	validExpr := &ExprSpec{Kind: "var", Value: "A"}
	tests := []struct {
		name    string
		spec    TermSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "invalid kind",
			spec:    TermSpec{Kind: "invalid"},
			wantErr: true,
			errStr:  "term kind must be atom, not, eq, neq, or cmp",
		},
		{
			name:    "atom term missing atom",
			spec:    TermSpec{Kind: "atom", Atom: nil},
			wantErr: true,
			errStr:  "atom term requires atom",
		},
		{
			name:    "atom term invalid atom",
			spec:    TermSpec{Kind: "atom", Atom: &AtomSpec{Pred: ""}},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name:    "not term missing atom",
			spec:    TermSpec{Kind: "not", Atom: nil},
			wantErr: true,
			errStr:  "negated term requires atom",
		},
		{
			name:    "eq missing left",
			spec:    TermSpec{Kind: "eq", Right: validExpr},
			wantErr: true,
			errStr:  "comparison requires left and right",
		},
		{
			name:    "neq missing right",
			spec:    TermSpec{Kind: "neq", Left: validExpr},
			wantErr: true,
			errStr:  "comparison requires left and right",
		},
		{
			name:    "eq invalid left",
			spec:    TermSpec{Kind: "eq", Left: &ExprSpec{Kind: "invalid"}, Right: validExpr},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "cmp missing left",
			spec:    TermSpec{Kind: "cmp", Right: validExpr},
			wantErr: true,
			errStr:  "comparison requires left and right",
		},
		{
			name:    "cmp invalid op",
			spec:    TermSpec{Kind: "cmp", Left: validExpr, Right: validExpr, Op: "invalid"},
			wantErr: true,
			errStr:  "cmp op must be lt, le, gt, or ge",
		},
		{
			name:    "cmp valid",
			spec:    TermSpec{Kind: "cmp", Left: validExpr, Right: validExpr, Op: "le"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTermSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTermSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateTermSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateAtomSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    AtomSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "empty predicate",
			spec:    AtomSpec{Pred: ""},
			wantErr: true,
			errStr:  "predicate is required",
		},
		{
			name:    "fn prefix",
			spec:    AtomSpec{Pred: "fn:foo"},
			wantErr: true,
			errStr:  "predicate must not start with \"fn:\"",
		},
		{
			name:    "invalid name token",
			spec:    AtomSpec{Pred: "123invalid"},
			wantErr: true,
			errStr:  "predicate must be a valid NAME token",
		},
		{
			name:    "invalid args",
			spec:    AtomSpec{Pred: "valid", Args: []ExprSpec{{Kind: "invalid"}}},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "valid",
			spec:    AtomSpec{Pred: "valid_pred", Args: []ExprSpec{{Kind: "var", Value: "A"}}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAtomSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAtomSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateAtomSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateExprSpec(t *testing.T) {
	three := 3
	two := 2
	f := 3.14

	tests := []struct {
		name    string
		spec    ExprSpec
		wantErr bool
		errStr  string
	}{
		{
			name:    "invalid kind",
			spec:    ExprSpec{Kind: "invalid"},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "var invalid",
			spec:    ExprSpec{Kind: "var", Value: "invalidVar"},
			wantErr: true,
			errStr:  "variable must be '_' or start with uppercase letter",
		},
		{
			name:    "var valid",
			spec:    ExprSpec{Kind: "var", Value: "Var"},
			wantErr: false,
		},
		{
			name:    "name invalid",
			spec:    ExprSpec{Kind: "name", Value: "invalid"},
			wantErr: true,
			errStr:  "name constant must start with '/'",
		},
		{
			name:    "name valid",
			spec:    ExprSpec{Kind: "name", Value: "/valid"},
			wantErr: false,
		},
		{
			name:    "string",
			spec:    ExprSpec{Kind: "string"},
			wantErr: false,
		},
		{
			name:    "bytes",
			spec:    ExprSpec{Kind: "bytes"},
			wantErr: false,
		},
		{
			name:    "number json.Number invalid",
			spec:    ExprSpec{Kind: "number", Number: "abc"},
			wantErr: true,
			errStr:  "number must be an integer",
		},
		{
			name:    "number json.Number valid",
			spec:    ExprSpec{Kind: "number", Number: "123"},
			wantErr: false,
		},
		{
			name:    "number value invalid",
			spec:    ExprSpec{Kind: "number", Value: "abc"},
			wantErr: true,
			errStr:  "number value must be an integer",
		},
		{
			name:    "number value valid",
			spec:    ExprSpec{Kind: "number", Value: "123"},
			wantErr: false,
		},
		{
			name:    "number missing both",
			spec:    ExprSpec{Kind: "number"},
			wantErr: true,
			errStr:  "number requires number or value",
		},
		{
			name:    "float value invalid",
			spec:    ExprSpec{Kind: "float", Value: "abc"},
			wantErr: true,
			errStr:  "float value must be numeric",
		},
		{
			name:    "float value valid",
			spec:    ExprSpec{Kind: "float", Value: "3.14"},
			wantErr: false,
		},
		{
			name:    "float literal valid",
			spec:    ExprSpec{Kind: "float", Float: &f},
			wantErr: false,
		},
		{
			name:    "float missing both",
			spec:    ExprSpec{Kind: "float"},
			wantErr: true,
			errStr:  "float requires float or value",
		},
		{
			name:    "apply missing function",
			spec:    ExprSpec{Kind: "apply", Function: ""},
			wantErr: true,
			errStr:  "apply function name is required",
		},
		{
			name:    "apply bad function prefix",
			spec:    ExprSpec{Kind: "apply", Function: "foo"},
			wantErr: true,
			errStr:  "apply function must start with \"fn:\"",
		},
		{
			name:    "apply invalid args",
			spec:    ExprSpec{Kind: "apply", Function: "fn:foo", Args: []ExprSpec{{Kind: "invalid"}}},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "apply arity mismatch",
			spec:    ExprSpec{Kind: "apply", Function: "fn:foo", Arity: &three, Args: []ExprSpec{{Kind: "var", Value: "A"}}},
			wantErr: true,
			errStr:  "arity 3 does not match args length 1",
		},
		{
			name:    "apply valid",
			spec:    ExprSpec{Kind: "apply", Function: "fn:foo", Args: []ExprSpec{{Kind: "var", Value: "A"}}},
			wantErr: false,
		},
		{
			name:    "list invalid args",
			spec:    ExprSpec{Kind: "list", Args: []ExprSpec{{Kind: "invalid"}}},
			wantErr: true,
			errStr:  "expr kind must be",
		},
		{
			name:    "map odd args length",
			spec:    ExprSpec{Kind: "map", Args: []ExprSpec{{Kind: "var", Value: "A"}}},
			wantErr: true,
			errStr:  "map/struct require even number of args",
		},
		{
			name:    "struct arity mismatch",
			spec:    ExprSpec{Kind: "struct", Arity: &two, Args: []ExprSpec{}},
			wantErr: true,
			errStr:  "arity 2 does not match args length 0",
		},
		{
			name:    "struct valid",
			spec:    ExprSpec{Kind: "struct", Args: []ExprSpec{{Kind: "var", Value: "A"}, {Kind: "var", Value: "B"}}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExprSpec(tt.spec, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExprSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("validateExprSpec() error = %v, want substring %q", err, tt.errStr)
			}
		})
	}
}

func TestValidateArity(t *testing.T) {
	three := 3
	negOne := -1

	tests := []struct {
		name    string
		arity   *int
		argLen  int
		wantErr bool
	}{
		{
			name:    "nil arity",
			arity:   nil,
			argLen:  5,
			wantErr: false,
		},
		{
			name:    "arity -1",
			arity:   &negOne,
			argLen:  5,
			wantErr: false,
		},
		{
			name:    "match",
			arity:   &three,
			argLen:  3,
			wantErr: false,
		},
		{
			name:    "mismatch",
			arity:   &three,
			argLen:  2,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArity(tt.arity, tt.argLen, "path")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidVariable(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"underscore", "_", true},
		{"empty", "", false},
		{"lowercase start", "aVariable", false},
		{"number start", "1Variable", false},
		{"valid single char", "A", true},
		{"valid multi char", "Variable123_", true},
		{"invalid char inside", "Var!able", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidVariable(tt.value); got != tt.want {
				t.Errorf("isValidVariable(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
