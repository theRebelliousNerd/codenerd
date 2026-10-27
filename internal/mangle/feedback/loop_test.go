package feedback

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle/synth"
)

func TestNewFeedbackLoop(t *testing.T) {
	// TODO: Add test for zero-value config (MaxRetries: 0, SessionBudget: 0) to ensure IsBudgetExhausted returns correctly and gracefully handles empty settings.
	config := RetryConfig{
		MaxRetries:          5,
		SessionBudget:       10,
		EnableAutoRepair:    true,
		InjectPredicates:    false,
		SimplifyOnLastRetry: true,
		PerAttemptTimeout:   10 * time.Second,
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
	// TODO: Add test for empty string rule hashes (e.g. hashPrompt("")) to verify it doesn't cause unexpected nil dereferences or infinite loops.
	fl := NewFeedbackLoop(RetryConfig{SessionBudget: 2})

	// record attempts
	fl.budget.RecordAttempt("hash1")
	fl.budget.RecordAttempt("hash2")
	// TODO: Add concurrent test suite (go test -race) with 100+ goroutines concurrently calling TryRecord to ensure atomic thread safety and prevent over-claiming budget.
	// TODO: Verify documentation warning by interleaving CanRetry and RecordAttempt in a highly concurrent scenario to prove they over-claim if not using TryRecord.

	if !fl.IsBudgetExhausted() {
		t.Error("expected budget to be exhausted")
	}

	// TODO: Test for hash collisions in hashPrompt() when storing in ValidationBudget.ruleAttempts.
	// TODO: Explicitly test type coercion when string inputs fail to parse as AST atoms (e.g. strings containing malformed Mangle syntax).
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
	// TODO: Test with massive predicate lists (e.g., 50,000+ predicates) to ensure strings.Builder doesn't cause buffer overflow or excessive memory allocation.
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

	// The heading says "use ONLY these", so every declared predicate is
	// listed -- a cut would forbid predicates the kernel accepts -- and in a
	// stable order whatever order the caller's map produced.
	// TODO: Test hashPrompt behavior with gigantically large prompt strings (e.g. >500MB) to ensure memory constraints aren't violated and it processes gracefully.
	manyPreds := make([]string, 0, 50)
	for i := 49; i >= 0; i-- {
		manyPreds = append(manyPreds, fmt.Sprintf("pred_%02d/1", i))
	}
	result = BuildEnhancedSystemPrompt(base, manyPreds)
	for _, p := range manyPreds {
		if !strings.Contains(result, "- "+p+"\n") {
			t.Fatalf("declared predicate %s missing from the legal set", p)
		}
	}
	if strings.Contains(result, "more\n") {
		t.Error("the legal set must not end in an unrecoverable '... and N more'")
	}
	if strings.Index(result, "pred_00/1") > strings.Index(result, "pred_49/1") {
		t.Error("predicates must be listed sorted")
	}
	if manyPreds[0] != "pred_49/1" {
		t.Error("the caller's slice must not be reordered")
	}
}
