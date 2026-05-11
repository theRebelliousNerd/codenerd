package synth

import (
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.AllowDecls {
		t.Errorf("DefaultOptions().AllowDecls = %v, want true", opts.AllowDecls)
	}
	if !opts.AllowPackage {
		t.Errorf("DefaultOptions().AllowPackage = %v, want true", opts.AllowPackage)
	}
	if !opts.AllowUse {
		t.Errorf("DefaultOptions().AllowUse = %v, want true", opts.AllowUse)
	}
	if opts.RequireSingleClause {
		t.Errorf("DefaultOptions().RequireSingleClause = %v, want false", opts.RequireSingleClause)
	}
	if opts.SkipAnalysis {
		t.Errorf("DefaultOptions().SkipAnalysis = %v, want false", opts.SkipAnalysis)
	}
}

func TestResultSingleClause(t *testing.T) {
	tests := []struct {
		name        string
		clauses     []string
		wantClause  string
		expectError bool
	}{
		{
			name:        "exactly one clause",
			clauses:     []string{"clause1"},
			wantClause:  "clause1",
			expectError: false,
		},
		{
			name:        "zero clauses",
			clauses:     []string{},
			wantClause:  "",
			expectError: true,
		},
		{
			name:        "multiple clauses",
			clauses:     []string{"clause1", "clause2"},
			wantClause:  "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := Result{Clauses: tc.clauses}
			clause, err := res.SingleClause()

			if tc.expectError {
				if err == nil {
					t.Errorf("SingleClause() expected error, got nil")
				}
				if clause != "" {
					t.Errorf("SingleClause() expected empty string with error, got %q", clause)
				}
			} else {
				if err != nil {
					t.Errorf("SingleClause() unexpected error: %v", err)
				}
				if clause != tc.wantClause {
					t.Errorf("SingleClause() = %q, want %q", clause, tc.wantClause)
				}
			}
		})
	}
}

func TestSpecErrorError(t *testing.T) {
	tests := []struct {
		name    string
		err     SpecError
		wantMsg string
	}{
		{
			name: "with path and message",
			err: SpecError{
				Path:    "program.clauses",
				Message: "expected exactly one clause",
			},
			wantMsg: "program.clauses: expected exactly one clause",
		},
		{
			name: "with only message",
			err: SpecError{
				Path:    "",
				Message: "general error occurred",
			},
			wantMsg: "general error occurred",
		},
		{
			name: "with empty struct",
			err:  SpecError{},
			wantMsg: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg != tc.wantMsg {
				t.Errorf("SpecError.Error() = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

func TestNewSpecError(t *testing.T) {
	path := "test.path"
	message := "test message"

	err := NewSpecError(path, message)

	if err.Path != path {
		t.Errorf("NewSpecError().Path = %q, want %q", err.Path, path)
	}
	if err.Message != message {
		t.Errorf("NewSpecError().Message = %q, want %q", err.Message, message)
	}
}
