package core

import (
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Remediation for kernel_validation TEST_GAP markers.
// QA: kernel_validation_test.go TEST_GAP comments (lines 7-14)
// ============================================================================

// TestKernelValidationGap_ValidateLearnedRules_NilEmpty verifies that
// ValidateLearnedRules handles nil and empty slices without panics.
func TestKernelValidationGap_ValidateLearnedRules_NilEmpty(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl foo(Name).")
	k.Evaluate()

	// nil slice
	errs := k.ValidateLearnedRules(nil)
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors for nil rules, got %d: %v", len(errs), errs)
	}

	// empty slice
	errs = k.ValidateLearnedRules([]string{})
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors for empty rules, got %d: %v", len(errs), errs)
	}

	// single empty-string rule
	errs = k.ValidateLearnedRules([]string{""})
	// Empty string should cause a syntax error or be ignored
	t.Logf("ValidateLearnedRules with single empty string: errors=%v", errs)
}

// TestKernelValidationGap_ValidateLearnedRule_SchemaValidatorNil verifies
// fail-open behavior when schemaValidator is nil.
func TestKernelValidationGap_ValidateLearnedRule_SchemaValidatorNil(t *testing.T) {
	// Create kernel WITHOUT calling SetSchemas (schemaValidator stays nil)
	k := &RealKernel{
		facts:       make([]Fact, 0),
		policyDirty: true,
		initialized: false,
	}

	err := k.ValidateLearnedRule("test_pred(X) :- some_pred(X).")
	if err == nil {
		t.Error("Expected error when schemaValidator is nil, got nil")
	}
	if !strings.Contains(err.Error(), "uninitialized") {
		t.Errorf("Expected 'uninitialized' error message, got: %v", err)
	}

	// Also test ValidateLearnedRules with nil validator
	errs := k.ValidateLearnedRules([]string{"rule."})
	if len(errs) == 0 {
		t.Error("Expected errors when schemaValidator is nil")
	}
	if !strings.Contains(errs[0].Error(), "uninitialized") {
		t.Errorf("Expected 'uninitialized' error message, got: %v", errs[0])
	}
}

// TestKernelValidationGap_CheckSyntax_LongLine verifies checkSyntax with
// an extremely long line (>1MB) to check for buffer exhaustion.
func TestKernelValidationGap_CheckSyntax_LongLine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-line test in short mode")
	}

	// Create a 1MB string that looks like a Mangle atom
	// pad(xxxxxxxx....xxx).
	padding := strings.Repeat("x", 1024*1024)
	longRule := "pad(" + padding + ")."

	err := checkSyntax(longRule)
	// Should either succeed (valid syntax) or fail gracefully — no panic
	t.Logf("checkSyntax with 1MB line: err=%v", err)
}

// TestKernelValidationGap_CheckSyntax_InvalidUTF8 verifies checkSyntax with
// invalid UTF-8 and binary null bytes.
func TestKernelValidationGap_CheckSyntax_InvalidUTF8(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"null bytes", "foo(\x00\x00\x00)."},
		{"invalid UTF-8 high bytes", "foo(\x80\x81\x82)."},
		{"mixed valid/invalid", "valid_pred(\xff\xfe)."},
		{"control characters", "foo(\x01\x02\x03)."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic — may return error or not
			err := checkSyntax(tt.input)
			t.Logf("checkSyntax(%q): err=%v", tt.name, err)
		})
	}
}

// TestKernelValidationGap_InfiniteLoopRisk_WhitespaceBypass verifies that
// checkInfiniteLoopRisk isn't bypassed by whitespace tricks.
func TestKernelValidationGap_InfiniteLoopRisk_WhitespaceBypass(t *testing.T) {
	k := setupMockKernel(t)

	tests := []struct {
		name     string
		rule     string
		wantRisk bool
	}{
		{
			name:     "normal ubiquitous predicate",
			rule:     "next_action(/do_something) :- current_time(T).",
			wantRisk: true,
		},
		{
			name:     "whitespace padded ubiquitous",
			rule:     "next_action(/do_something) :-  current_time ( T ) .",
			wantRisk: true, // Should detect even with extra whitespace
		},
		{
			name:     "tab-separated",
			rule:     "next_action(/do_something)\t:-\tcurrent_time(T).",
			wantRisk: true,
		},
		{
			name:     "safe rule with guard",
			rule:     "next_action(/do_something) :- current_time(T), task_pending(T, X).",
			wantRisk: false, // Has non-ubiquitous guard predicate
		},
		{
			name:     "unconditional system action",
			rule:     "next_action(/system_start).",
			wantRisk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := k.checkInfiniteLoopRisk(tt.rule)
			gotRisk := result != ""
			if gotRisk != tt.wantRisk {
				t.Errorf("checkInfiniteLoopRisk(%q) = %q, wantRisk=%v, gotRisk=%v",
					tt.rule, result, tt.wantRisk, gotRisk)
			}
		})
	}
}

// TestKernelValidationGap_ValidateLearnedRulesContent_ManyRules verifies
// validateLearnedRulesContent performance with a large number of rules.
func TestKernelValidationGap_ValidateLearnedRulesContent_ManyRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	k := setupMockKernel(t)
	k.SetSchemas("Decl stress_pred(Name).")
	k.Evaluate()

	// Generate 1000 rules (reduced from 100,000 for CI safety)
	var rules strings.Builder
	for range 1000 {
		rules.WriteString("stress_pred(\"rule_" + strings.Repeat("x", 10) + "\").\n")
	}

	// Should not OOM or stall
	result := k.validateLearnedRulesContent(rules.String(), "", false)
	t.Logf("Validated %d rules: valid=%d, invalid=%d",
		result.stats.TotalRules, result.stats.ValidRules, result.stats.InvalidRules)
}

// TestKernelValidationGap_Concurrency_SetSchemasWhileValidate verifies
// concurrent SetSchemas + ValidateLearnedRule interactions.
func TestKernelValidationGap_Concurrency_SetSchemasWhileValidate(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl base_pred(Name).")
	k.Evaluate()

	var wg sync.WaitGroup
	const goroutines = 20

	// Readers: validate rules concurrently
	for range goroutines {
		wg.Go(func() {
			_ = k.ValidateLearnedRule("base_pred(\"test\").")
		})
	}

	// Writers: update schemas concurrently
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			schema := "Decl schema_" + string(rune('a'+idx)) + "(Name)."
			k.SetSchemas(schema)
		}(i)
	}

	wg.Wait()
	// No panics or data races should have occurred (run with -race)
}

// TestKernelValidationGap_TOCTOU_ValidateLearnedRulesContent documents
// the TOCTOU vulnerability in validateLearnedRulesContent.
// Since the function takes a string (not a file path), the TOCTOU risk
// is in the caller that reads the file, not in validateLearnedRulesContent itself.
func TestKernelValidationGap_TOCTOU_ValidateLearnedRulesContent(t *testing.T) {
	k := setupMockKernel(t)
	k.SetSchemas("Decl toctou_pred(Name).")
	k.Evaluate()

	// The function takes string content, so there's no direct TOCTOU.
	// The risk is when the caller reads a file and passes it.
	// We verify the function handles concurrent calls safely.
	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			content := "toctou_pred(\"concurrent_" + string(rune('a'+idx)) + "\")."
			_ = k.validateLearnedRulesContent(content, "", false)
		}(i)
	}

	wg.Wait()
	t.Log("KNOWN: TOCTOU risk is in file reading callers (e.g., healLearnedRules), not in validateLearnedRulesContent itself.")
}
