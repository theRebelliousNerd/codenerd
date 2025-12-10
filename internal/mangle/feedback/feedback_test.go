package feedback

import (
	"context"
	"errors"
	"strings"
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
			name:    "lowercase aggregate function",
			input:   `|> do fn:count()`,
			wantErr: true,
		},
		{
			name:    "correct aggregation",
			input:   `source() |> do fn:group_by(X), let N = fn:Count()`,
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
			name:     "fix lowercase count",
			input:    `fn:count()`,
			expected: `fn:Count()`,
		},
		{
			name:     "fix lowercase sum",
			input:    `fn:sum(X)`,
			expected: `fn:Sum(X)`,
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

// ============================================================================
// ErrorClassifier Tests
// ============================================================================

func TestErrorClassifier_Stratification(t *testing.T) {
	ec := NewErrorClassifier()

	errMsg := "stratification error: cyclic negation detected"
	errs := ec.Classify(errMsg)

	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}

	if errs[0].Category != CategoryStratification {
		t.Errorf("expected CategoryStratification, got %v", errs[0].Category)
	}
}

func TestErrorClassifier_UndeclaredPredicate(t *testing.T) {
	ec := NewErrorClassifier()

	errMsg := "undeclared predicate: foo_bar"
	errs := ec.Classify(errMsg)

	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}

	if errs[0].Category != CategoryUndeclaredPredicate {
		t.Errorf("expected CategoryUndeclaredPredicate, got %v", errs[0].Category)
	}
}

func TestErrorClassifier_ParseError(t *testing.T) {
	ec := NewErrorClassifier()

	errMsg := "123:45 parse error near 'foo'"
	errs := ec.Classify(errMsg)

	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}

	if errs[0].Line != 123 {
		t.Errorf("expected line 123, got %d", errs[0].Line)
	}
	if errs[0].Column != 45 {
		t.Errorf("expected column 45, got %d", errs[0].Column)
	}
}

func TestErrorClassifier_ClassifyWithContext(t *testing.T) {
	ec := NewErrorClassifier()

	code := `line1
line2
line3 has error`
	errMsg := "3:5 syntax error"

	errs := ec.ClassifyWithContext(errMsg, code)

	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}

	if errs[0].Wrong != "line3 has error" {
		t.Errorf("expected Wrong to be 'line3 has error', got %q", errs[0].Wrong)
	}
}

func TestExtractPredicateFromError(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected string
	}{
		{"undeclared predicate: foo_bar(", "foo_bar"},
		{"error in 'my_pred('", "my_pred"},
		{"no predicate here", ""},
	}

	for _, tt := range tests {
		result := ExtractPredicateFromError(tt.errMsg)
		if result != tt.expected {
			t.Errorf("ExtractPredicateFromError(%q) = %q, want %q",
				tt.errMsg, result, tt.expected)
		}
	}
}

// ============================================================================
// PromptBuilder Tests
// ============================================================================

func TestPromptBuilder_BuildFeedbackPrompt(t *testing.T) {
	pb := NewPromptBuilder()

	ctx := FeedbackContext{
		OriginalPrompt: "Generate a rule for test state",
		OriginalRule:   `next_action(/run) :- test_state("failing").`,
		Errors: []ValidationError{
			{
				Category:   CategoryAtomString,
				Line:       1,
				Message:    "String should be atom",
				Wrong:      `"failing"`,
				Correct:    `/failing`,
				Suggestion: "Use /atom syntax",
			},
		},
		AttemptNumber:       2,
		MaxAttempts:         3,
		AvailablePredicates: []string{"test_state/1", "next_action/1"},
		ValidExamples:       []string{`next_action(/run_tests) :- test_state(/failing).`},
	}

	result := pb.BuildFeedbackPrompt(ctx)

	// Check key elements are present
	if !strings.Contains(result, "attempt 2/3") {
		t.Error("expected attempt number in prompt")
	}
	if !strings.Contains(result, "String should be atom") {
		t.Error("expected error message in prompt")
	}
	if !strings.Contains(result, "test_state/1") {
		t.Error("expected available predicates in prompt")
	}
	if !strings.Contains(result, "next_action(/run_tests)") {
		t.Error("expected valid examples in prompt")
	}
}

func TestPromptBuilder_BuildInitialPromptAdditions(t *testing.T) {
	pb := NewPromptBuilder()

	predicates := []string{"user_intent/5", "next_action/1", "permitted/1"}
	result := pb.BuildInitialPromptAdditions(predicates)

	// Check syntax reminders included
	if !strings.Contains(result, "/atom") {
		t.Error("expected atom syntax reminder")
	}
	if !strings.Contains(result, "fn:group_by") {
		t.Error("expected aggregation syntax reminder")
	}

	// Check predicates listed
	for _, pred := range predicates {
		if !strings.Contains(result, pred) {
			t.Errorf("expected predicate %q in prompt", pred)
		}
	}
}

func TestExtractRuleFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name:     "markdown code block",
			response: "Here's the rule:\n```mangle\nnext_action(/run) :- test(/fail).\n```",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "generic code block",
			response: "```\nnext_action(/run) :- test(/fail).\n```",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "plain rule",
			response: "next_action(/run) :- test(/fail).",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "rule with explanation on separate line",
			response: "Here is the rule:\nnext_action(/run) :- test(/fail).\nThis handles test failures.",
			expected: "next_action(/run) :- test(/fail).",
		},
		// RULE: prefix handling (autopoiesis structured output format)
		{
			name:     "RULE prefix with CONFIDENCE and RATIONALE",
			response: "RULE: next_action(/run) :- test(/fail).\nCONFIDENCE: 0.9\nRATIONALE: Handles failing tests",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "RULE prefix only",
			response: "RULE: next_action(/run) :- test(/fail).",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "RULE prefix with multiline rule",
			response: "RULE: next_action(/run) :-\n    test(/fail),\n    !blocked(/test).\nCONFIDENCE: 0.85",
			expected: "next_action(/run) :-\n    test(/fail),\n    !blocked(/test).",
		},
		{
			name:     "RULE prefix with extra whitespace",
			response: "RULE:    next_action(/run) :- test(/fail).   \nCONFIDENCE: 0.9",
			expected: "next_action(/run) :- test(/fail).",
		},
		{
			name:     "RULE prefix in middle of response",
			response: "Here is my proposal:\nRULE: next_action(/run) :- test(/fail).\nCONFIDENCE: 0.8\nRATIONALE: Good rule",
			expected: "next_action(/run) :- test(/fail).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractRuleFromResponse(tt.response)
			if result != tt.expected {
				t.Errorf("ExtractRuleFromResponse() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidRuleExamples(t *testing.T) {
	executive := ValidRuleExamples("executive")
	if len(executive) == 0 {
		t.Error("expected executive examples")
	}

	constitution := ValidRuleExamples("constitution")
	if len(constitution) == 0 {
		t.Error("expected constitution examples")
	}

	generic := ValidRuleExamples("unknown")
	if len(generic) == 0 {
		t.Error("expected generic examples for unknown domain")
	}
}

// ============================================================================
// ValidationBudget Tests
// ============================================================================

func TestValidationBudget(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    3,
		SessionBudget: 5,
	}
	budget := NewValidationBudget(config)

	promptHash := "test123"

	// Should allow retries initially
	for i := 0; i < 3; i++ {
		can, _ := budget.CanRetry(promptHash)
		if !can {
			t.Errorf("expected to allow retry %d", i+1)
		}
		budget.RecordAttempt(promptHash)
	}

	// Should deny after max retries for this prompt
	can, reason := budget.CanRetry(promptHash)
	if can {
		t.Error("expected to deny retry after max retries")
	}
	if !strings.Contains(reason, "max retries") {
		t.Errorf("expected 'max retries' in reason, got %q", reason)
	}

	// Different prompt should still work
	can, _ = budget.CanRetry("different")
	if !can {
		t.Error("expected different prompt to be allowed")
	}
}

func TestValidationBudget_SessionLimit(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    10,
		SessionBudget: 3,
	}
	budget := NewValidationBudget(config)

	// Use up session budget
	for i := 0; i < 3; i++ {
		budget.RecordAttempt("prompt" + string(rune(i)))
	}

	// Should deny due to session budget
	can, reason := budget.CanRetry("newprompt")
	if can {
		t.Error("expected to deny due to session budget")
	}
	if !strings.Contains(reason, "session") {
		t.Errorf("expected 'session' in reason, got %q", reason)
	}
}

func TestValidationBudget_Reset(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    3,
		SessionBudget: 5,
	}
	budget := NewValidationBudget(config)

	// Use some budget
	budget.RecordAttempt("test")
	budget.RecordAttempt("test")

	// Reset
	budget.Reset()

	// Should allow again
	can, _ := budget.CanRetry("test")
	if !can {
		t.Error("expected retry to be allowed after reset")
	}

	used, total := budget.Stats()
	if used != 0 {
		t.Errorf("expected 0 used after reset, got %d", used)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
}

func TestValidationBudget_IsSessionExhausted(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    10,
		SessionBudget: 3,
	}
	budget := NewValidationBudget(config)

	// Initially not exhausted
	if budget.IsSessionExhausted() {
		t.Error("expected session to NOT be exhausted initially")
	}

	// Use some budget, but not all
	budget.RecordAttempt("rule1")
	budget.RecordAttempt("rule2")
	if budget.IsSessionExhausted() {
		t.Error("expected session to NOT be exhausted after 2 attempts (budget is 3)")
	}

	// Exhaust the budget
	budget.RecordAttempt("rule3")
	if !budget.IsSessionExhausted() {
		t.Error("expected session to BE exhausted after 3 attempts (budget is 3)")
	}

	// Reset should clear the exhaustion
	budget.Reset()
	if budget.IsSessionExhausted() {
		t.Error("expected session to NOT be exhausted after reset")
	}
}

// ============================================================================
// FeedbackLoop Integration Tests
// ============================================================================

// MockLLMClient for testing
type MockLLMClient struct {
	responses []string
	callCount int
}

func (m *MockLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.callCount >= len(m.responses) {
		return "", errors.New("no more responses")
	}
	response := m.responses[m.callCount]
	m.callCount++
	return response, nil
}

// MockRuleValidator for testing
type MockRuleValidator struct {
	validRules map[string]bool
	predicates []string
}

func (m *MockRuleValidator) HotLoadRule(rule string) error {
	if m.validRules[rule] {
		return nil
	}
	return errors.New("invalid rule: parse error")
}

func (m *MockRuleValidator) GetDeclaredPredicates() []string {
	return m.predicates
}

// FlexibleMockRuleValidator accepts rules that contain a specific substring
type FlexibleMockRuleValidator struct {
	acceptContains string
	predicates     []string
}

func (m *FlexibleMockRuleValidator) HotLoadRule(rule string) error {
	if strings.Contains(rule, m.acceptContains) {
		return nil
	}
	return errors.New("invalid rule: does not contain expected pattern")
}

func (m *FlexibleMockRuleValidator) GetDeclaredPredicates() []string {
	return m.predicates
}

func TestFeedbackLoop_SuccessOnFirstAttempt(t *testing.T) {
	config := DefaultConfig()
	config.EnableAutoRepair = false // Disable sanitizer to avoid rule transformation
	fl := NewFeedbackLoop(config)

	validRule := `next_action(/run_tests) :- test_state(/failing).`

	mockLLM := &MockLLMClient{
		responses: []string{validRule},
	}

	mockValidator := &MockRuleValidator{
		validRules: map[string]bool{validRule: true},
		predicates: []string{"test_state/1", "next_action/1"},
	}

	result, err := fl.GenerateAndValidate(
		context.Background(),
		mockLLM,
		mockValidator,
		"system prompt",
		"generate a test rule",
		"executive",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid result")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
	if result.Rule != validRule {
		t.Errorf("expected rule %q, got %q", validRule, result.Rule)
	}
}

func TestFeedbackLoop_SuccessAfterRetry(t *testing.T) {
	config := DefaultConfig()
	config.EnableAutoRepair = false // Disable sanitizer to avoid rule transformation
	fl := NewFeedbackLoop(config)

	invalidRule := `next_action(/run_tests) :- test_state("failing").` // String instead of atom
	validRule := `next_action(/run_tests) :- test_state(/failing).`

	mockLLM := &MockLLMClient{
		responses: []string{invalidRule, validRule},
	}

	mockValidator := &MockRuleValidator{
		validRules: map[string]bool{validRule: true},
		predicates: []string{"test_state/1", "next_action/1"},
	}

	result, err := fl.GenerateAndValidate(
		context.Background(),
		mockLLM,
		mockValidator,
		"system prompt",
		"generate a test rule",
		"executive",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid result")
	}
	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
}

func TestFeedbackLoop_FailAfterMaxRetries(t *testing.T) {
	config := RetryConfig{
		MaxRetries:       2,
		SessionBudget:    10,
		EnableAutoRepair: true,
	}
	fl := NewFeedbackLoop(config)

	invalidRule := `invalid syntax here`

	mockLLM := &MockLLMClient{
		responses: []string{invalidRule, invalidRule, invalidRule},
	}

	mockValidator := &MockRuleValidator{
		validRules: map[string]bool{}, // Nothing valid
		predicates: []string{},
	}

	result, err := fl.GenerateAndValidate(
		context.Background(),
		mockLLM,
		mockValidator,
		"system prompt",
		"generate a test rule",
		"executive",
	)

	if err == nil {
		t.Error("expected error after max retries")
	}
	if result.Valid {
		t.Error("expected invalid result")
	}
	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
}

func TestFeedbackLoop_AutoRepair(t *testing.T) {
	config := DefaultConfig()
	config.EnableAutoRepair = true
	fl := NewFeedbackLoop(config)

	// Rule with fixable issue (lowercase count)
	ruleWithIssue := `count_items(N) :- item(_) |> do fn:count(), let N = fn:count().`
	// After QuickFix, fn:count becomes fn:Count
	fixedRule := `count_items(N) :- item(_) |> do fn:Count(), let N = fn:Count().`

	mockLLM := &MockLLMClient{
		responses: []string{ruleWithIssue},
	}

	// Use flexible validator that accepts any rule containing the fixed function casing
	mockValidator := &FlexibleMockRuleValidator{
		acceptContains: "fn:Count()",
		predicates:     []string{"item/1", "count_items/1"},
	}

	result, err := fl.GenerateAndValidate(
		context.Background(),
		mockLLM,
		mockValidator,
		"system prompt",
		"generate a count rule",
		"executive",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid result after auto-repair")
	}
	if !result.AutoFixed {
		t.Error("expected AutoFixed to be true")
	}
	// Verify the fixed rule contains properly cased function
	if !strings.Contains(result.Rule, "fn:Count()") {
		t.Errorf("expected rule to contain fn:Count(), got %q", result.Rule)
	}
	_ = fixedRule // suppress unused warning
}

func TestFeedbackLoop_ValidateOnly(t *testing.T) {
	config := DefaultConfig()
	config.EnableAutoRepair = false // Disable sanitizer to avoid rule transformation
	fl := NewFeedbackLoop(config)

	validRule := `next_action(/test) :- condition(/met).`

	mockValidator := &MockRuleValidator{
		validRules: map[string]bool{validRule: true},
		predicates: []string{},
	}

	result := fl.ValidateOnly(validRule, mockValidator)

	if !result.Valid {
		t.Errorf("expected valid result, got errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestFeedbackLoop_PreValidateOnly(t *testing.T) {
	config := DefaultConfig()
	fl := NewFeedbackLoop(config)

	// Use regular string for proper escaping: \+ is single backslash followed by plus
	ruleWithIssues := "blocked(X) :- \\+ permitted(X), state(X, \"active\")."

	errors := fl.PreValidateOnly(ruleWithIssues)

	if len(errors) == 0 {
		t.Error("expected pre-validation errors")
	}

	// Should find prolog negation and atom/string issues
	categories := make(map[ErrorCategory]bool)
	for _, e := range errors {
		categories[e.Category] = true
	}

	if !categories[CategoryPrologNegation] {
		t.Error("expected prolog negation error")
	}
	if !categories[CategoryAtomString] {
		t.Error("expected atom/string error")
	}
}

// ============================================================================
// FormatErrorForFeedback Tests
// ============================================================================

func TestFormatErrorForFeedback(t *testing.T) {
	err := ValidationError{
		Category:   CategoryAtomString,
		Line:       5,
		Message:    "String should be atom",
		Wrong:      `"active"`,
		Correct:    `/active`,
		Suggestion: "Use /atom for identifiers",
	}

	result := FormatErrorForFeedback(err)

	if !strings.Contains(result, "LINE 5") {
		t.Error("expected line number in output")
	}
	if !strings.Contains(result, "String should be atom") {
		t.Error("expected message in output")
	}
	if !strings.Contains(result, "Use /atom") {
		t.Error("expected suggestion in output")
	}
}
