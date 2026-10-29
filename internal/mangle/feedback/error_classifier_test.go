package feedback

import (
	"testing"
)

func TestNewErrorClassifier(t *testing.T) {
	ec := NewErrorClassifier()
	if ec == nil {
		t.Fatal("NewErrorClassifier returned nil")
	}
	if len(ec.patterns) == 0 {
		t.Error("NewErrorClassifier did not compile patterns")
	}
}

func TestErrorClassifier_Classify(t *testing.T) {
	ec := NewErrorClassifier()

	tests := []struct {
		name     string
		errMsg   string
		wantCat  ErrorCategory
		wantLine int
		wantCol  int
	}{
		{
			name:     "Stratification Error",
			errMsg:   "stratification violation: rule creates cyclic negation dependency",
			wantCat:  CategoryStratification,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Undeclared Predicate",
			errMsg:   "undeclared predicate foo()",
			wantCat:  CategoryUndeclaredPredicate,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Parse Error Line/Col",
			errMsg:   "12:34 parse error",
			wantCat:  CategoryParse,
			wantLine: 12,
			wantCol:  34,
		},
		{
			name:     "Expected Base Term",
			errMsg:   "expected base term got xyz(",
			wantCat:  CategorySyntax,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Unrecognized Token",
			errMsg:   "token recognition error at: '\\+'",
			wantCat:  CategorySyntax,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "No Viable Alternative",
			errMsg:   "no viable alternative at input 'foo'",
			wantCat:  CategoryParse,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Mismatched Input",
			errMsg:   "mismatched input 'foo' expecting 'bar'",
			wantCat:  CategorySyntax,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Unbound Variable",
			errMsg:   "unsafe variable: variable not bound",
			wantCat:  CategoryUnboundNegation,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Type Mismatch",
			errMsg:   "type mismatch in arguments",
			wantCat:  CategoryTypeMismatch,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Arity Mismatch",
			errMsg:   "arity mismatch wrong number arguments",
			wantCat:  CategorySyntax,
			wantLine: 0,
			wantCol:  0,
		},
		{
			name:     "Generic Parse Error Fallback with line",
			errMsg:   "15:2 something totally unknown",
			wantCat:  CategoryParse,
			wantLine: 15,
			wantCol:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ec.Classify(tt.errMsg)
			if len(got) != 1 {
				t.Fatalf("Classify() returned %d errors, want 1", len(got))
			}

			if got[0].Category != tt.wantCat {
				t.Errorf("Classify() Category = %v, want %v", got[0].Category, tt.wantCat)
			}

			if got[0].Line != tt.wantLine {
				t.Errorf("Classify() Line = %d, want %d", got[0].Line, tt.wantLine)
			}

			if got[0].Column != tt.wantCol {
				t.Errorf("Classify() Column = %d, want %d", got[0].Column, tt.wantCol)
			}
		})
	}
}

func TestExtractLineCol(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int
		wantCol  int
	}{
		{
			name:     "standard line col at start",
			input:    "123:45 parse error",
			wantLine: 123,
			wantCol:  45,
		},
		{
			name:     "standard line col after space",
			input:    "Error: 12:34 something",
			wantLine: 12,
			wantCol:  34,
		},
		{
			name:     "line word lowercase",
			input:    "error at line 123",
			wantLine: 123,
			wantCol:  0,
		},
		{
			name:     "line word uppercase",
			input:    "Error Line 55",
			wantLine: 55,
			wantCol:  0,
		},
		{
			name:     "no match",
			input:    "just some generic error without line info",
			wantLine: 0,
			wantCol:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLine, gotCol := extractLineCol(tt.input)
			if gotLine != tt.wantLine {
				t.Errorf("extractLineCol() gotLine = %d, want %d", gotLine, tt.wantLine)
			}
			if gotCol != tt.wantCol {
				t.Errorf("extractLineCol() gotCol = %d, want %d", gotCol, tt.wantCol)
			}
		})
	}
}
