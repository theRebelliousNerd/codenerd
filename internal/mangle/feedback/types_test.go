package feedback

import (
	"sync"
	"testing"
	"time"
)

func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{CategoryParse, "parse_error"},
		{CategoryAtomString, "atom_string_confusion"},
		{CategoryAggregation, "aggregation_syntax"},
		{CategoryMissingPeriod, "missing_period"},
		{CategoryUnboundNegation, "unbound_negation"},
		{CategoryUndeclaredPredicate, "undeclared_predicate"},
		{CategoryStratification, "stratification_violation"},
		{CategoryTypeMismatch, "type_mismatch"},
		{CategoryPrologNegation, "prolog_negation"},
		{CategorySyntax, "syntax_error"},
		{ErrorCategory(999), "unknown"}, // test default case
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("ErrorCategory.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCategory_IsAutoRepairable(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected bool
	}{
		{CategoryAtomString, true},
		{CategoryAggregation, true},
		{CategoryMissingPeriod, true},
		{CategoryPrologNegation, true},
		{CategoryUnboundNegation, true},
		{CategoryParse, false},
		{CategoryUndeclaredPredicate, false},
		{CategoryStratification, false},
		{CategoryTypeMismatch, false},
		{CategorySyntax, false},
		{ErrorCategory(999), false},
	}

	for _, tt := range tests {
		t.Run(tt.category.String(), func(t *testing.T) {
			if got := tt.category.IsAutoRepairable(); got != tt.expected {
				t.Errorf("ErrorCategory.IsAutoRepairable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   ValidationResult
		expected bool
	}{
		{
			name:     "no errors",
			result:   ValidationResult{Errors: []ValidationError{}},
			expected: false,
		},
		{
			name: "with errors",
			result: ValidationResult{
				Errors: []ValidationError{{Category: CategorySyntax}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("ValidationResult.HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	// We only test that it doesn't panic and returns a sensible default
	// because config.GetLLMTimeouts() might return different values based on environment
	config := DefaultConfig()
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if config.SessionBudget != 20 {
		t.Errorf("Expected SessionBudget 20, got %d", config.SessionBudget)
	}
	if !config.EnableAutoRepair {
		t.Errorf("Expected EnableAutoRepair true, got false")
	}
	if !config.InjectPredicates {
		t.Errorf("Expected InjectPredicates true, got false")
	}
	if !config.SimplifyOnLastRetry {
		t.Errorf("Expected SimplifyOnLastRetry true, got false")
	}
	if config.PerAttemptTimeout <= 0 {
		t.Errorf("Expected PerAttemptTimeout > 0, got %v", config.PerAttemptTimeout)
	}
	if config.TotalTimeout <= 0 {
		t.Errorf("Expected TotalTimeout > 0, got %v", config.TotalTimeout)
	}

	expectedTotal := config.PerAttemptTimeout * time.Duration(config.MaxRetries)
	if config.TotalTimeout != expectedTotal {
		t.Errorf("Expected TotalTimeout %v, got %v", expectedTotal, config.TotalTimeout)
	}
}

func TestNewValidationBudget(t *testing.T) {
	config := RetryConfig{
		MaxRetries:    5,
		SessionBudget: 50,
	}
	budget := NewValidationBudget(config)

	if budget.maxPerRule != 5 {
		t.Errorf("Expected maxPerRule 5, got %d", budget.maxPerRule)
	}
	if budget.sessionBudget != 50 {
		t.Errorf("Expected sessionBudget 50, got %d", budget.sessionBudget)
	}
	if budget.ruleAttempts == nil {
		t.Errorf("Expected ruleAttempts map to be initialized")
	}
}

func TestValidationBudget_CanRetry(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{
		MaxRetries:    2,
		SessionBudget: 3,
	})

	// Scenario 1: Fresh budget, rule "rule1"
	canRetry, reason := budget.CanRetry("rule1")
	if !canRetry {
		t.Errorf("Expected CanRetry to be true, got reason: %s", reason)
	}

	// Scenario 2: Exceeding max retries for a rule
	budget.RecordAttempt("rule1")
	budget.RecordAttempt("rule1")
	canRetry, reason = budget.CanRetry("rule1")
	if canRetry {
		t.Errorf("Expected CanRetry to be false due to max retries")
	}
	if reason != "max retries exceeded for this rule" {
		t.Errorf("Unexpected reason: %s", reason)
	}

	// Scenario 3: Different rule can still retry if session budget allows
	canRetry, reason = budget.CanRetry("rule2")
	if !canRetry {
		t.Errorf("Expected CanRetry to be true for new rule, got reason: %s", reason)
	}

	// Scenario 4: Exceeding session budget
	budget.RecordAttempt("rule2") // Total attempts now 3 (session budget limit)
	canRetry, reason = budget.CanRetry("rule3")
	if canRetry {
		t.Errorf("Expected CanRetry to be false due to session budget exhaustion")
	}
	if reason != "session validation budget exhausted" {
		t.Errorf("Unexpected reason: %s", reason)
	}
}

func TestValidationBudget_RecordAndGetAttemptCount(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 5, SessionBudget: 10})

	ruleHash := "testRule"

	if count := budget.GetAttemptCount(ruleHash); count != 0 {
		t.Errorf("Expected initial count 0, got %d", count)
	}

	budget.RecordAttempt(ruleHash)

	if count := budget.GetAttemptCount(ruleHash); count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	budget.RecordAttempt(ruleHash)

	if count := budget.GetAttemptCount(ruleHash); count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestTypes_ValidationBudget_Reset(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 5, SessionBudget: 10})

	budget.RecordAttempt("rule1")
	budget.RecordAttempt("rule2")

	sessionUsed, _ := budget.Stats()
	if sessionUsed != 2 {
		t.Errorf("Expected 2 sessionUsed, got %d", sessionUsed)
	}

	budget.Reset()

	sessionUsed, _ = budget.Stats()
	if sessionUsed != 0 {
		t.Errorf("Expected 0 sessionUsed after reset, got %d", sessionUsed)
	}

	if count := budget.GetAttemptCount("rule1"); count != 0 {
		t.Errorf("Expected 0 attempts for rule1 after reset, got %d", count)
	}
}

func TestValidationBudget_Stats(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 5, SessionBudget: 10})

	used, total := budget.Stats()
	if used != 0 || total != 10 {
		t.Errorf("Expected 0/10 stats, got %d/%d", used, total)
	}

	budget.RecordAttempt("rule1")

	used, total = budget.Stats()
	if used != 1 || total != 10 {
		t.Errorf("Expected 1/10 stats, got %d/%d", used, total)
	}
}

func TestTypes_ValidationBudget_IsSessionExhausted(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 5, SessionBudget: 2})

	if budget.IsSessionExhausted() {
		t.Errorf("Expected session not exhausted initially")
	}

	budget.RecordAttempt("rule1")
	if budget.IsSessionExhausted() {
		t.Errorf("Expected session not exhausted after 1 attempt")
	}

	budget.RecordAttempt("rule2")
	if !budget.IsSessionExhausted() {
		t.Errorf("Expected session exhausted after 2 attempts")
	}
}

func TestValidationBudget_Concurrency(t *testing.T) {
	budget := NewValidationBudget(RetryConfig{MaxRetries: 1000, SessionBudget: 1000})

	var wg sync.WaitGroup
	numWorkers := 10
	attemptsPerWorker := 50

	ruleHash := "concurrent_rule"

	for range numWorkers {
		wg.Go(func() {
			for range attemptsPerWorker {
				budget.RecordAttempt(ruleHash)
				budget.GetAttemptCount(ruleHash)
				budget.CanRetry(ruleHash)
				budget.Stats()
				budget.IsSessionExhausted()
			}
		})
	}

	wg.Wait()

	expectedAttempts := numWorkers * attemptsPerWorker
	if count := budget.GetAttemptCount(ruleHash); count != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, count)
	}

	used, _ := budget.Stats()
	if used != expectedAttempts {
		t.Errorf("Expected %d session used, got %d", expectedAttempts, used)
	}
}
