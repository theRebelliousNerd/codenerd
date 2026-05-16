package feedback

import (
	"testing"
)

// ============================================================================
// PreValidator Tests
// ============================================================================

func TestPreValidator_AtomStringConfusion(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		category ErrorCategory
	}{
		{
			name:     "string should be atom - status active",
			input:    `status(X, "active")`,
			wantErr:  true,
			category: CategoryAtomString,
		},
		{
			name:     "string should be atom - enum pending",
			input:    `state(X, "pending")`,
			wantErr:  true,
			category: CategoryAtomString,
		},
		{
			name:     "correct atom usage",
			input:    `status(X, /active)`,
			wantErr:  false,
			category: 0,
		},
		{
			name:     "string literal for actual text is OK",
			input:    `message(X, "Hello world")`,
			wantErr:  false, // Not an enum-like value
			category: 0,
		},
		{
			name:     "user_intent category should be atom",
			input:    `user_intent(Id, "review", /fix, /codebase, _)`,
			wantErr:  true,
			category: CategoryAtomString,
		},
		{
			name:     "user_intent verb should be atom",
			input:    `user_intent(Id, /review, "fix", /codebase, _)`,
			wantErr:  true,
			category: CategoryAtomString,
		},
		{
			name:     "user_intent target should be atom",
			input:    `user_intent(Id, /review, /fix, "codebase", _)`,
			wantErr:  true,
			category: CategoryAtomString,
		},
		{
			name:     "user_intent atoms are allowed",
			input:    `user_intent(Id, /review, /fix, /codebase, _)`,
			wantErr:  false,
			category: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := pv.Validate(tt.input)
			if tt.wantErr && len(errs) == 0 {
				t.Errorf("expected error for input %q, got none", tt.input)
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected error for input %q: %v", tt.input, errs)
			}
			if tt.wantErr && len(errs) > 0 && errs[0].Category != tt.category {
				t.Errorf("expected category %v, got %v", tt.category, errs[0].Category)
			}
		})
	}
}

func TestPreValidator_PrologNegation(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "prolog negation backslash plus",
			input:   "blocked(X) :- \\+ permitted(X).", // Raw \+ as it appears in LLM output
			wantErr: true,
		},
		{
			name:    "correct mangle negation",
			input:   `blocked(X) :- !permitted(X).`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := pv.Validate(tt.input)
			hasNegErr := false
			for _, e := range errs {
				if e.Category == CategoryPrologNegation {
					hasNegErr = true
					break
				}
			}
			if tt.wantErr && !hasNegErr {
				t.Errorf("expected prolog negation error for %q", tt.input)
			}
			if !tt.wantErr && hasNegErr {
				t.Errorf("unexpected prolog negation error for %q", tt.input)
			}
		})
	}
}

func TestPreValidator_AggregationSyntax(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "SQL-style aggregation",
			input:   `Total = sum(Amount)`,
			wantErr: true,
		},
		{
			name:    "missing do keyword",
			input:   `source() |> fn:group_by(X)`,
			wantErr: true,
		},
		{
			name:    "uppercase aggregate function",
			input:   `|> do fn:Count()`,
			wantErr: true,
		},
		{
			name:    "correct aggregation",
			input:   `source() |> do fn:group_by(X), let N = fn:count()`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := pv.Validate(tt.input)
			hasAggErr := false
			for _, e := range errs {
				if e.Category == CategoryAggregation {
					hasAggErr = true
					break
				}
			}
			if tt.wantErr && !hasAggErr {
				t.Errorf("expected aggregation error for %q", tt.input)
			}
			if !tt.wantErr && hasAggErr {
				t.Errorf("unexpected aggregation error for %q", tt.input)
			}
		})
	}
}

func TestPreValidator_UnboundNegation(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "negation before positive binding",
			input:   `blocked(X) :- !permitted(X).`,
			wantErr: true,
		},
		{
			name:    "correct - positive binding first",
			input:   `blocked(X) :- action(X), !permitted(X).`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := pv.Validate(tt.input)
			hasUnboundErr := false
			for _, e := range errs {
				if e.Category == CategoryUnboundNegation {
					hasUnboundErr = true
					break
				}
			}
			if tt.wantErr && !hasUnboundErr {
				t.Errorf("expected unbound negation error for %q", tt.input)
			}
			if !tt.wantErr && hasUnboundErr {
				t.Errorf("unexpected unbound negation error for %q", tt.input)
			}
		})
	}
}

func TestPreValidator_MissingPeriod(t *testing.T) {
	pv := NewPreValidator()

	input := `next_action(/run) :- test_state(/failing)`
	errs := pv.Validate(input)

	hasPeriodErr := false
	for _, e := range errs {
		if e.Category == CategoryMissingPeriod {
			hasPeriodErr = true
			break
		}
	}

	if !hasPeriodErr {
		t.Errorf("expected missing period error for %q", input)
	}
}

func TestPreValidator_QuickFix(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "fix prolog negation",
			input:    "\\+ permitted(X)", // Single backslash: \+ permitted(X)
			expected: "!permitted(X)",
		},
		{
			name:     "fix uppercase count",
			input:    `fn:Count()`,
			expected: `fn:count()`,
		},
		{
			name:     "fix uppercase sum",
			input:    `fn:Sum(X)`,
			expected: `fn:sum(X)`,
		},
		{
			name:     "fix missing do keyword",
			input:    `|> fn:group_by(X)`,
			expected: `|> do fn:group_by(X)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.QuickFix(tt.input)
			if result != tt.expected {
				t.Errorf("QuickFix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPreValidator_EmptyAndCommentLines(t *testing.T) {
	pv := NewPreValidator()
	code := `
# This is a comment

	# Indented comment
`
	errs := pv.Validate(code)
	if len(errs) > 0 {
		t.Errorf("Expected 0 errors for empty/comment lines, got %d", len(errs))
	}
}

func TestPreValidator_NilRegexPattern(t *testing.T) {
	pv := NewPreValidator()
	// Manually inject a pattern with nil regex
	pv.patterns = append(pv.patterns, compiledPattern{
		ErrorPattern: ErrorPattern{
			Category: CategorySyntax,
			Message:  "Should not match due to nil regex",
		},
		regex: nil,
	})

	code := "some code"
	errs := pv.Validate(code)
	for _, e := range errs {
		if e.Message == "Should not match due to nil regex" {
			t.Errorf("Pattern with nil regex was somehow matched")
		}
	}
}

func TestPreValidator_ValidateGlobal(t *testing.T) {
	pv := NewPreValidator()

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		category ErrorCategory
	}{
		{
			name:    "multi-line rule complete",
			input:   "rule(X) :-\n    cond(X).",
			wantErr: false,
		},
		{
			name:     "multi-line rule interrupted by EOF",
			input:    "rule(X) :- a\n    cond(X)",
			wantErr:  true,
			category: CategoryMissingPeriod,
		},
		{
			name:     "multi-line rule interrupted by space",
			input:    "rule(X) :- a\n    \n    cond(X).", // Empty line breaks the lookahead
			wantErr:  true,
			category: CategoryMissingPeriod, // Will trigger missing period because it thinks the first line is incomplete
		},
		{
			name:     "multi-line rule interrupted by comment",
			input:    "rule(X) :- a\n    # comment\n    cond(X).",
			wantErr:  true, // Will trigger missing period on line 1 because comment breaks lookahead
			category: CategoryMissingPeriod,
		},
		{
			name:     "unbalanced parentheses - extra open",
			input:    "rule(X :- cond(X).",
			wantErr:  true,
			category: CategorySyntax,
		},
		{
			name:     "unbalanced parentheses - extra close",
			input:    "rule(X) :- cond(X)).",
			wantErr:  true,
			category: CategorySyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := pv.Validate(tt.input)

			hasErr := false
			for _, e := range errs {
				if e.Category == tt.category {
					hasErr = true
					break
				}
			}

			if tt.wantErr && !hasErr {
				t.Errorf("Expected error of category %v for input %q", tt.category, tt.input)
			}
			if !tt.wantErr && hasErr {
				t.Errorf("Unexpected error for input %q", tt.input)
			}
		})
	}
}

func TestPreValidator_GetPatterns(t *testing.T) {
	pv := NewPreValidator()
	patterns := pv.GetPatterns()
	if len(patterns) == 0 {
		t.Errorf("Expected GetPatterns to return non-empty slice")
	}
	// Verify length matches original patterns
	if len(patterns) != len(pv.patterns) {
		t.Errorf("GetPatterns length %d != internal patterns length %d", len(patterns), len(pv.patterns))
	}

	// Verify we got actual error patterns with fields
	for i, p := range patterns {
		if p.Category != pv.patterns[i].Category {
			t.Errorf("Pattern %d category mismatch", i)
		}
	}
}
