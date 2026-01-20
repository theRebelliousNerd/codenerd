package synth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompile_HappyPath(t *testing.T) {
	// A simple valid spec: p(X) :- q(X).
	// Note: We skip analysis because q(X) is not declared, which would fail semantic checks.
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

	opts := DefaultOptions()
	opts.SkipAnalysis = true
	result, err := Compile(spec, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "p(X) :- q(X)."
	if strings.TrimSpace(result.Source) != expected {
		t.Errorf("Expected source %q, got %q", expected, result.Source)
	}
}

func TestCompile_ComplexClause(t *testing.T) {
	// complex(X, Y) :- foo(X), Y = 10, X > Y |> do fn:group_by(Y).
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{
				{
					Head: AtomSpec{
						Pred: "complex",
						Args: []ExprSpec{
							{Kind: "var", Value: "X"},
							{Kind: "var", Value: "Y"},
						},
					},
					Body: []TermSpec{
						{
							Kind: "atom",
							Atom: &AtomSpec{
								Pred: "foo",
								Args: []ExprSpec{
									{Kind: "var", Value: "X"},
								},
							},
						},
						{
							Kind:  "eq",
							Left:  &ExprSpec{Kind: "var", Value: "Y"},
							Right: &ExprSpec{Kind: "number", Value: "10"},
						},
						{
							Kind:  "cmp",
							Left:  &ExprSpec{Kind: "var", Value: "X"},
							Op:    "gt",
							Right: &ExprSpec{Kind: "var", Value: "Y"},
						},
					},
					Transform: &TransformSpec{
						Statements: []TransformStmtSpec{
							{
								Kind: "do",
								Fn: ExprSpec{
									Kind:     "apply",
									Function: "fn:group_by",
									Args: []ExprSpec{
										{Kind: "var", Value: "Y"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.SkipAnalysis = true
	result, err := Compile(spec, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Basic containment check as string representation might vary slightly
	// Note: Mangle AST stringification might not put spaces after commas.
	if !strings.Contains(result.Source, "complex(X,Y) :-") {
		t.Errorf("Missing head. Got:\n%s", result.Source)
	}
	if !strings.Contains(result.Source, "foo(X)") {
		t.Error("Missing atom body")
	}
	if !strings.Contains(result.Source, "Y = 10") {
		t.Error("Missing eq")
	}
	// The CMP parsing output :gt(X,Y)
	if !strings.Contains(result.Source, ":gt(X,Y)") {
		t.Errorf("Missing cmp :gt. Got:\n%s", result.Source)
	}
	if !strings.Contains(result.Source, "|> do fn:group_by(Y).") {
		t.Error("Missing transform")
	}
}

func TestCompile_Decls(t *testing.T) {
	// Decl foo(X) bound [X].
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Decls: []DeclSpec{
				{
					Atom: AtomSpec{
						Pred: "foo",
						Args: []ExprSpec{{Kind: "var", Value: "X"}},
					},
					Bounds: []BoundSpec{
						{
							Terms: []ExprSpec{{Kind: "var", Value: "X"}},
						},
					},
				},
			},
		},
	}

	result, err := Compile(spec, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "Decl foo(X) bound [X]."
	if strings.TrimSpace(result.Source) != expected {
		t.Errorf("Expected %q, got %q", expected, result.Source)
	}
}

func TestCompile_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want string
	}{
		{
			name: "invalid format",
			spec: Spec{Format: "wrong", Program: ProgramSpec{Clauses: []ClauseSpec{{Head: AtomSpec{Pred: "p"}}}}},
			want: "format",
		},
		{
			name: "empty program",
			spec: Spec{Format: FormatV1, Program: ProgramSpec{}},
			want: "program must contain at least one",
		},
		{
			name: "bad predicate name",
			spec: Spec{Format: FormatV1, Program: ProgramSpec{
				Clauses: []ClauseSpec{{Head: AtomSpec{Pred: "BadName"}}},
			}},
			want: "predicate must be a valid NAME token",
		},
		{
			name: "bad variable name",
			spec: Spec{Format: FormatV1, Program: ProgramSpec{
				Clauses: []ClauseSpec{{Head: AtomSpec{Pred: "p", Args: []ExprSpec{{Kind: "var", Value: "lower"}}}}},
			}},
			want: "variable must be '_' or start with uppercase",
		},
		{
			name: "missing atom in term",
			spec: Spec{Format: FormatV1, Program: ProgramSpec{
				Clauses: []ClauseSpec{{
					Head: AtomSpec{Pred: "p"},
					Body: []TermSpec{{Kind: "atom", Atom: nil}},
				}},
			}},
			want: "atom term requires atom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.spec, DefaultOptions())
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestCompile_NumberJson(t *testing.T) {
	// p(10).
	spec := Spec{
		Format: FormatV1,
		Program: ProgramSpec{
			Clauses: []ClauseSpec{
				{
					Head: AtomSpec{
						Pred: "p",
						Args: []ExprSpec{
							{Kind: "number", Number: json.Number("10")},
						},
					},
				},
			},
		},
	}

	result, err := Compile(spec, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expected := "p(10)."
	if strings.TrimSpace(result.Source) != expected {
		t.Errorf("Expected %q, got %q", expected, result.Source)
	}
}
