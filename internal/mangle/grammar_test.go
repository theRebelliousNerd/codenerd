package mangle

import (
	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"strings"
	"testing"
)

func TestNewRepairLoop(t *testing.T) {
	loop := NewRepairLoop()
	if loop == nil {
		t.Fatal("NewRepairLoop() returned nil")
	}
	if loop.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", loop.MaxRetries)
	}
	if loop.Validator == nil {
		t.Fatal("NewRepairLoop() Validator is nil")
	}
}

func TestRepairLoop_ValidateAndRepair(t *testing.T) {
	loop := NewRepairLoop()

	// Pre-populate validator with user_intent so it validates cleanly without warnings
	loop.Validator.ValidPredicates["user_intent"] = PredicateSpec{
		Name:  "user_intent",
		Arity: 5,
		Args: []ArgSpec{
			{Name: "Arg0", Type: ArgTypeName},
			{Name: "Arg1", Type: ArgTypeName},
			{Name: "Arg2", Type: ArgTypeName},
			{Name: "Arg3", Type: ArgTypeString},
			{Name: "Arg4", Type: ArgTypeString},
		},
	}

	tests := []struct {
		name         string
		atoms        []string
		wantValid    []string
		wantErr      bool
		wantPromptIn []string
	}{
		{
			name:      "All Valid",
			atoms:     []string{"user_intent(/intent_1, /code, /create, \"target\", \"constraint\")."},
			wantValid: []string{"user_intent(/intent_1, /code, /create, \"target\", \"constraint\")."},
			wantErr:   false,
		},
		{
			name: "Some Invalid",
			atoms: []string{
				"user_intent(/intent_1, /code, /create, \"target\", \"constraint\").",
				"invalid_predicate", // No parentheses, will be marked invalid
			},
			wantValid:    []string{"user_intent(/intent_1, /code, /create, \"target\", \"constraint\")."},
			wantErr:      true,
			wantPromptIn: []string{"MANGLE SYNTAX ERROR", "Invalid: invalid_predicate"},
		},
		{
			name:      "No Atoms",
			atoms:     []string{},
			wantValid: nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validAtoms, err, prompt := loop.ValidateAndRepair(tt.atoms)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndRepair() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(validAtoms) != len(tt.wantValid) {
				t.Errorf("ValidateAndRepair() valid atoms len = %d, want %d", len(validAtoms), len(tt.wantValid))
			} else {
				for i, v := range validAtoms {
					if v != tt.wantValid[i] {
						t.Errorf("ValidateAndRepair() valid atom %d = %v, want %v", i, v, tt.wantValid[i])
					}
				}
			}

			if tt.wantErr {
				for _, p := range tt.wantPromptIn {
					if !strings.Contains(prompt, p) {
						t.Errorf("ValidateAndRepair() prompt missing %q\ngot:\n%s", p, prompt)
					}
				}
			} else if prompt != "" {
				t.Errorf("ValidateAndRepair() prompt = %q, want empty string", prompt)
			}
		})
	}
}

func TestRepairLoop_UpdateFromProgramInfo(t *testing.T) {
	loop := NewRepairLoop()

	// Create a dummy ProgramInfo with a custom declaration
	// We'll create minimal ast representations just enough to not panic
	// and register a declaration
	predSym := ast.PredicateSym{Symbol: "my_custom_pred", Arity: 1}

	declAtom := ast.NewAtom("my_custom_pred", ast.Variable{Symbol: "X"})

	// For BoundDecl, since Mangle API varies, let's just not pass any Bounds to simplify
	// The function should gracefully handle len(decl.Bounds) == 0
	decl, _ := ast.NewDecl(declAtom, nil, nil, nil)

	info := &analysis.ProgramInfo{
		Decls: map[ast.PredicateSym]*ast.Decl{
			predSym: &decl,
		},
	}

	loop.UpdateFromProgramInfo(info)

	// Check that Validator was updated
	spec, known := loop.Validator.ValidPredicates["my_custom_pred"]
	if !known {
		t.Fatal("Validator was not updated with custom predicate")
	}
	if spec.Name != "my_custom_pred" || spec.Arity != 1 {
		t.Errorf("Validator has wrong spec: %+v", spec)
	}
	if len(spec.Args) != 1 || spec.Args[0].Name != "X" {
		t.Errorf("Validator extracted wrong args: %+v", spec.Args)
	}
}
