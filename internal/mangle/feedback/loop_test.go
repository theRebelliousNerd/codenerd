package feedback

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle/synth"
)

func TestNewFeedbackLoop(t *testing.T) {
	config := RetryConfig{
		MaxRetries:          5,
		SessionBudget:       10,
		EnableAutoRepair:    true,
		InjectPredicates:    false,
		SimplifyOnLastRetry: true,
		PerAttemptTimeout:   10 * time.Second,
		TotalTimeout:        30 * time.Second,
	}

	fl := NewFeedbackLoop(config)

	if fl == nil {
		t.Fatal("NewFeedbackLoop returned nil")
	}

	if fl.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries to be 5, got %d", fl.config.MaxRetries)
	}

	if fl.config.SessionBudget != 10 {
		t.Errorf("expected SessionBudget to be 10, got %d", fl.config.SessionBudget)
	}

	if fl.preValidator == nil {
		t.Error("preValidator is nil")
	}

	if fl.errorClassifier == nil {
		t.Error("errorClassifier is nil")
	}

	if fl.promptBuilder == nil {
		t.Error("promptBuilder is nil")
	}

	if fl.sanitizer == nil {
		t.Error("sanitizer is nil")
	}

	if fl.budget == nil {
		t.Error("budget is nil")
	}

	if fl.synthMode != SynthModeOff {
		t.Errorf("expected default synthMode to be %v, got %v", SynthModeOff, fl.synthMode)
	}
}

func TestSetPredicateSelector(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())

	if fl.predicateSelector != nil {
		t.Error("predicateSelector should be nil initially")
	}

	type mockSelector struct {
		PredicateSelectorInterface
	}
	mock := &mockSelector{}

	fl.SetPredicateSelector(mock)
	if fl.predicateSelector == nil {
		t.Error("predicateSelector should not be nil after SetPredicateSelector")
	}
}

func TestSetSynthMode(t *testing.T) {
	fl := NewFeedbackLoop(DefaultConfig())
	opts := synth.Options{
		RequireSingleClause: true,
	}

	fl.SetSynthMode(SynthModeRequire, opts)

	if fl.synthMode != SynthModeRequire {
		t.Errorf("expected synthMode %v, got %v", SynthModeRequire, fl.synthMode)
	}
	if !fl.synthOptions.RequireSingleClause {
		t.Error("expected synthOptions.RequireSingleClause to be true")
	}
}

func TestFeedbackLoop_GetBudgetAndReset(t *testing.T) {
	fl := NewFeedbackLoop(RetryConfig{SessionBudget: 2})

	// record attempts
	fl.budget.RecordAttempt("hash1")
	fl.budget.RecordAttempt("hash2")

	if !fl.IsBudgetExhausted() {
		t.Error("expected budget to be exhausted")
	}

	b := fl.GetBudget()
	if b == nil {
		t.Error("GetBudget returned nil")
	}

	fl.ResetBudget()
	if fl.IsBudgetExhausted() {
		t.Error("expected budget to be reset")
	}
}

func TestBuildEnhancedSystemPrompt(t *testing.T) {
	base := "You are a helpful AI."
	preds := []string{"pred1(X)", "pred2(X, Y)"}

	result := BuildEnhancedSystemPrompt(base, preds)

	if len(result) <= len(base) {
		t.Error("expected result to be longer than base")
	}

	if !strings.Contains(result, "pred1(X)") || !strings.Contains(result, "pred2(X, Y)") {
		t.Error("expected result to contain declared predicates")
	}

	if !strings.Contains(result, defaultSyntaxReminder) {
		t.Error("expected result to contain syntax reminder")
	}

	// Check truncation behavior
	manyPreds := make([]string, 50)
	for i := range 50 {
		manyPreds[i] = "pred(X)"
	}
	result = BuildEnhancedSystemPrompt(base, manyPreds)

	if !strings.Contains(result, "... and 20 more") {
		t.Error("expected result to truncate and indicate remaining predicates")
	}
}
