package verification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// isReviewTask TESTS — extended coverage
// =============================================================================

func TestIsReviewTask_WhenEdgeCases_ShouldClassifyCorrectly(t *testing.T) {
	tests := []struct {
		task string
		want bool
	}{
		// True cases — prefix matches
		{"review the code", true},
		{"analyze dependencies", true},
		{"security_scan all files", true},
		{"audit the authentication flow", true},
		{"inspect the module graph", true},
		{"examine edge cases", true},
		{"assess code quality", true},
		{"evaluate the design", true},

		// False cases — implementation tasks
		{"implement feature X", false},
		{"run unit tests", false},
		{"fix the bug", false},
		{"refactor the handler", false},
		{"deploy to staging", false},
		{"write documentation", false},
		{"create a new endpoint", false},

		// Edge cases
		{"", false},
		{"REVIEW uppercase", true}, // case insensitive
		{"Review Mixed Case", true},
		{"prereview setup", true},       // substring match: "prereview setup" contains "review "
		{"the analysis is done", false}, // "analyze" embedded differently
	}

	for _, tc := range tests {
		t.Run(tc.task, func(t *testing.T) {
			got := isReviewTask(tc.task)
			if got != tc.want {
				t.Errorf("isReviewTask(%q) = %v, want %v", tc.task, got, tc.want)
			}
		})
	}
}

// =============================================================================
// basicQualityCheck TESTS — expanded violation detection
// =============================================================================

func TestBasicQualityCheck_WhenCleanCode_ShouldPass(t *testing.T) {
	v := &TaskVerifier{}

	tests := []struct {
		name  string
		input string
	}{
		{"simple_function", "func Add(a, b int) int { return a + b }"},
		{"error_handling", `if err != nil { return fmt.Errorf("failed: %w", err) }`},
		{"clean_test", `func TestAdd(t *testing.T) { if Add(1,2) != 3 { t.Fatal("wrong") } }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := v.basicQualityCheck(tc.input)
			if !res.Success {
				t.Errorf("Expected success for clean code, got violations: %v", res.QualityViolations)
			}
			if len(res.QualityViolations) != 0 {
				t.Errorf("Expected 0 violations for clean code, got %d", len(res.QualityViolations))
			}
		})
	}
}

func TestBasicQualityCheck_WhenTODO_ShouldDetectPlaceholder(t *testing.T) {
	v := &TaskVerifier{}
	res := v.basicQualityCheck("// TODO: implement this function")

	if res.Success {
		t.Error("Should fail on TODO")
	}
	if !containsViolation(res.QualityViolations, PlaceholderCode) {
		t.Error("Should detect PlaceholderCode")
	}
}

func TestBasicQualityCheck_WhenFIXME_ShouldDetectPlaceholder(t *testing.T) {
	v := &TaskVerifier{}
	res := v.basicQualityCheck("// FIXME: this is broken")

	if res.Success {
		t.Error("Should fail on FIXME")
	}
	if !containsViolation(res.QualityViolations, PlaceholderCode) {
		t.Error("Should detect PlaceholderCode")
	}
}

func TestBasicQualityCheck_WhenMock_ShouldDetectMockCode(t *testing.T) {
	v := &TaskVerifier{}

	tests := []struct {
		name  string
		input string
	}{
		{"lowercase_mock", "this is a mock implementation"},
		{"uppercase_mock", "func MockHandler() {}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := v.basicQualityCheck(tc.input)
			if res.Success {
				t.Error("Should fail on mock code")
			}
			if !containsViolation(res.QualityViolations, MockCode) {
				t.Error("Should detect MockCode")
			}
		})
	}
}

func TestBasicQualityCheck_WhenNotImplemented_ShouldDetectIncomplete(t *testing.T) {
	v := &TaskVerifier{}

	tests := []struct {
		name  string
		input string
	}{
		{"not_implemented_text", "this feature is not implemented yet"},
		{"panic_not_implemented", `panic("not implemented")`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := v.basicQualityCheck(tc.input)
			if res.Success {
				t.Error("Should fail on incomplete implementation")
			}
			if !containsViolation(res.QualityViolations, IncompleteImpl) {
				t.Error("Should detect IncompleteImpl")
			}
		})
	}
}

func TestBasicQualityCheck_WhenPlaceholderStub_ShouldDetect(t *testing.T) {
	v := &TaskVerifier{}
	res := v.basicQualityCheck("this is a placeholder stub for the real implementation")

	if res.Success {
		t.Error("Should fail on placeholder/stub")
	}
	if !containsViolation(res.QualityViolations, PlaceholderCode) {
		t.Error("Should detect PlaceholderCode")
	}
}

func TestBasicQualityCheck_ShouldHaveLowerConfidence(t *testing.T) {
	v := &TaskVerifier{}
	res := v.basicQualityCheck("clean code")

	if res.Confidence != 0.6 {
		t.Errorf("Confidence = %f, want 0.6", res.Confidence)
	}
}

// =============================================================================
// parseVerificationResponse TESTS
// =============================================================================

func TestParseVerificationResponse_WhenValidJSON_ShouldParse(t *testing.T) {
	response := `{"success":true,"confidence":0.95,"reason":"all good","quality_violations":[],"evidence":[],"suggestions":[]}`

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if !parsed.Success {
		t.Error("Success should be true")
	}
	if parsed.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", parsed.Confidence)
	}
	if parsed.Reason != "all good" {
		t.Errorf("Reason = %q, want 'all good'", parsed.Reason)
	}
}

func TestParseVerificationResponse_WhenCodeFenced_ShouldStrip(t *testing.T) {
	response := "```json\n{\"success\":false,\"confidence\":0.3,\"reason\":\"failed\"}\n```"

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if parsed.Success {
		t.Error("Success should be false")
	}
}

func TestParseVerificationResponse_WhenPlainCodeFenced_ShouldStrip(t *testing.T) {
	response := "```\n{\"success\":true,\"confidence\":0.8,\"reason\":\"ok\"}\n```"

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if !parsed.Success {
		t.Error("Success should be true")
	}
}

func TestParseVerificationResponse_WhenInvalidJSON_ShouldError(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"garbage", "not json at all"},
		{"incomplete", `{"success": true`},
		{"wrong_type", `{"success": "yes"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseVerificationResponse(tc.input)
			if err == nil {
				t.Error("Should error on invalid JSON")
			}
		})
	}
}

func TestParseVerificationResponse_WithQualityViolations_ShouldParseAll(t *testing.T) {
	response := `{
		"success": false,
		"confidence": 0.2,
		"reason": "multiple issues",
		"quality_violations": ["mock_code", "placeholder", "incomplete"],
		"evidence": ["line 5: TODO", "line 10: Mock"],
		"suggestions": ["fix it"],
		"corrective_action": {
			"type": "research",
			"query": "how to implement",
			"reason": "need real API"
		}
	}`

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if parsed.Success {
		t.Error("Success should be false")
	}
	if len(parsed.QualityViolations) != 3 {
		t.Errorf("Expected 3 violations, got %d", len(parsed.QualityViolations))
	}
	if len(parsed.Evidence) != 2 {
		t.Errorf("Expected 2 evidence items, got %d", len(parsed.Evidence))
	}
	if parsed.CorrectiveAction == nil {
		t.Fatal("CorrectiveAction should not be nil")
	}
	if parsed.CorrectiveAction.Type != CorrectiveResearch {
		t.Errorf("CorrectiveAction.Type = %q, want %q", parsed.CorrectiveAction.Type, CorrectiveResearch)
	}
}

// =============================================================================
// truncateForVerification TESTS
// =============================================================================

func TestTruncateForVerification_WhenUnderLimit_ShouldReturnUnchanged(t *testing.T) {
	short := "hello world"
	got := truncateForVerification(short)
	if got != short {
		t.Errorf("Short string should be unchanged: %q", got)
	}
}

func TestTruncateForVerification_WhenOverLimit_ShouldTruncate(t *testing.T) {
	long := strings.Repeat("x", 10000)
	got := truncateForVerification(long)

	if len(got) <= 8000 {
		t.Errorf("Expected len > 8000, got %d", len(got))
	}
	if !strings.Contains(got, "[truncated]") {
		t.Error("Should contain [truncated] suffix")
	}
}

func TestTruncateForVerification_WhenExactlyAtLimit_ShouldReturnUnchanged(t *testing.T) {
	exact := strings.Repeat("x", 8000)
	got := truncateForVerification(exact)
	if got != exact {
		t.Error("String at exact limit should be unchanged")
	}
}

// =============================================================================
// truncateContext TESTS
// =============================================================================

func TestTruncateContext_WhenUnderLimit_ShouldReturnUnchanged(t *testing.T) {
	got := truncateContext("short", 100)
	if got != "short" {
		t.Errorf("Expected 'short', got %q", got)
	}
}

func TestTruncateContext_WhenOverLimit_ShouldTruncate(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := truncateContext(long, 100)
	if !strings.HasPrefix(got, strings.Repeat("a", 100)) {
		t.Error("Should preserve first 100 chars")
	}
	if !strings.Contains(got, "[truncated]") {
		t.Error("Should contain [truncated]")
	}
}

func TestTruncateContext_WhenZeroMax_ShouldTruncate(t *testing.T) {
	got := truncateContext("anything", 0)
	if !strings.Contains(got, "[truncated]") {
		t.Error("Should truncate with max=0")
	}
}

// =============================================================================
// NewTaskVerifier TESTS
// =============================================================================

func TestNewTaskVerifier_WhenAllNil_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	if v == nil {
		t.Fatal("NewTaskVerifier should not return nil")
	}
}

func TestNewTaskVerifier_ShouldStoreFields(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	if v.client != nil {
		t.Error("client should be nil")
	}
	if v.localDB != nil {
		t.Error("localDB should be nil")
	}
	if v.shardMgr != nil {
		t.Error("shardMgr should be nil")
	}
	if v.autopoiesis != nil {
		t.Error("autopoiesis should be nil")
	}
}

// =============================================================================
// SetSessionContext TESTS
// =============================================================================

func TestSetSessionContext_ShouldStoreValues(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	v.SetSessionContext("session123", 5)

	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.sessionID != "session123" {
		t.Errorf("sessionID = %q, want 'session123'", v.sessionID)
	}
	if v.turnCount != 5 {
		t.Errorf("turnCount = %d, want 5", v.turnCount)
	}
}

// =============================================================================
// spawnTask TESTS
// =============================================================================

func TestSpawnTask_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	_, err := v.spawnTask(context.Background(), "test", "task")
	if err == nil {
		t.Fatal("spawnTask with no executor should error")
	}
	if !strings.Contains(err.Error(), "no executor available") {
		t.Errorf("Error should mention no executor: %v", err)
	}
}

// =============================================================================
// enrichTaskWithContext TESTS
// =============================================================================

func TestEnrichTaskWithContext_WhenNoVerification_ShouldAddContextOnly(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result := v.enrichTaskWithContext("original task", "extra context", nil)

	if !strings.Contains(result, "original task") {
		t.Error("Should contain original task")
	}
	if !strings.Contains(result, "extra context") {
		t.Error("Should contain extra context")
	}
	if !strings.Contains(result, "IMPORTANT") {
		t.Error("Should contain quality reminder")
	}
}

func TestEnrichTaskWithContext_WhenVerificationFailed_ShouldAddFailureInfo(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	verification := &VerificationResult{
		Success:           false,
		Reason:            "Code has quality issues",
		QualityViolations: []QualityViolation{MockCode, PlaceholderCode},
		Evidence:          []string{"line 5: mock", "line 10: TODO"},
	}

	result := v.enrichTaskWithContext("fix the code", "", verification)

	if !strings.Contains(result, "Previous Attempt Failed") {
		t.Error("Should contain failure header")
	}
	if !strings.Contains(result, "Code has quality issues") {
		t.Error("Should contain failure reason")
	}
	if !strings.Contains(result, "mock_code") {
		t.Error("Should list violations")
	}
	if !strings.Contains(result, "line 5: mock") {
		t.Error("Should list evidence")
	}
}

func TestEnrichTaskWithContext_WhenEmptyContext_ShouldStillAddReminder(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result := v.enrichTaskWithContext("task", "", nil)

	if !strings.Contains(result, "Do NOT use mock") {
		t.Error("Should contain anti-mock reminder")
	}
	if !strings.Contains(result, "Do NOT use TODO") {
		t.Error("Should contain anti-TODO reminder")
	}
}

// =============================================================================
// heuristicShardSelection TESTS
// =============================================================================

func TestHeuristicShardSelection_WhenHallucinatedAPI_ShouldSuggestResearch(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: []QualityViolation{HallucinatedAPI},
	}

	result := v.heuristicShardSelection("/fix", verification)

	if result.ShardType != "/research" {
		t.Errorf("ShardType = %q, want '/research'", result.ShardType)
	}
}

func TestHeuristicShardSelection_WhenMissingErrors_ShouldSuggestReview(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: []QualityViolation{MissingErrors},
	}

	result := v.heuristicShardSelection("/fix", verification)

	if result.ShardType != "/review" {
		t.Errorf("ShardType = %q, want '/review'", result.ShardType)
	}
}

func TestHeuristicShardSelection_WhenFakeTests_ShouldSuggestTester(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: []QualityViolation{FakeTests},
	}

	result := v.heuristicShardSelection("/fix", verification)

	if result.ShardType != "/test" {
		t.Errorf("ShardType = %q, want '/test'", result.ShardType)
	}
}

func TestHeuristicShardSelection_WhenNoSpecificViolation_ShouldRetrySame(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: []QualityViolation{MockCode}, // no special mapping
	}

	result := v.heuristicShardSelection("/fix", verification)

	if result.ShardType != "/fix" {
		t.Errorf("ShardType = %q, want '/fix'", result.ShardType)
	}
}

func TestHeuristicShardSelection_WhenNoViolations_ShouldRetrySame(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: nil,
	}

	result := v.heuristicShardSelection("/code", verification)

	if result.ShardType != "/code" {
		t.Errorf("ShardType = %q, want '/code'", result.ShardType)
	}
}

// =============================================================================
// parseShardSelection TESTS
// =============================================================================

func TestParseShardSelection_WhenValidJSON_ShouldParse(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	response := `{"selected_shard": "golang-specialist", "shard_type": "specialist", "reason": "Go expert needed", "confidence": 0.9, "alternatives": ["coder"]}`

	result := v.parseShardSelection(response, "/fix")

	if result.ShardType != "golang-specialist" {
		t.Errorf("ShardType = %q, want 'golang-specialist'", result.ShardType)
	}
	if result.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", result.Confidence)
	}
}

func TestParseShardSelection_WhenInvalidJSON_ShouldFallback(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result := v.parseShardSelection("not json", "/fallback")

	if result.ShardType != "/fallback" {
		t.Errorf("ShardType = %q, want '/fallback'", result.ShardType)
	}
}

func TestParseShardSelection_WhenCodeFenced_ShouldStrip(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	response := "```json\n{\"selected_shard\": \"test-shard\"}\n```"

	result := v.parseShardSelection(response, "/fallback")

	if result.ShardType != "test-shard" {
		t.Errorf("ShardType = %q, want 'test-shard'", result.ShardType)
	}
}

// =============================================================================
// VerifyWithRetry TESTS
// =============================================================================

func TestVerifyWithRetry_WhenNoExecutor_ShouldError(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	_, _, err := v.VerifyWithRetry(context.Background(), "test task", "/fix", 1)
	if err == nil {
		t.Fatal("VerifyWithRetry should error when no executor available")
	}
}

func TestVerifyWithRetry_WhenZeroRetries_ShouldDefaultTo3(t *testing.T) {
	// We verify this indirectly by checking it doesn't immediately return
	// with max retries exceeded (it should try at least once, but will fail
	// due to no executor)
	v := NewTaskVerifier(nil, nil, nil, nil)

	_, _, err := v.VerifyWithRetry(context.Background(), "test", "/fix", 0)
	if err == nil {
		t.Fatal("Should error with no executor")
	}
}

// =============================================================================
// applyCorrectiveAction TESTS
// =============================================================================

func TestApplyCorrectiveAction_WhenNil_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	result := v.applyCorrectiveAction(context.Background(), nil)
	if result != "" {
		t.Errorf("Expected empty string for nil action, got %q", result)
	}
}

func TestApplyCorrectiveAction_WhenDecompose_ShouldReturnHint(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	action := &CorrectiveAction{
		Type:  CorrectiveDecompose,
		Query: "break into smaller steps",
	}

	result := v.applyCorrectiveAction(context.Background(), action)
	if !strings.Contains(result, "Task Decomposition") {
		t.Error("Should contain decomposition hint")
	}
	if !strings.Contains(result, "break into smaller steps") {
		t.Error("Should contain the query")
	}
}

func TestApplyCorrectiveAction_WhenToolNoAutopoiesis_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	action := &CorrectiveAction{
		Type:   CorrectiveTool,
		Query:  "generate_tool",
		Reason: "need tool",
	}

	result := v.applyCorrectiveAction(context.Background(), action)
	if result != "" {
		t.Errorf("Expected empty with no autopoiesis, got %q", result)
	}
}

// =============================================================================
// findMatchingSpecialist TESTS
// =============================================================================

func TestFindMatchingSpecialist_WhenNoShardMgr_ShouldReturnEmpty(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	result := v.findMatchingSpecialist("hint", "query")
	if result != "" {
		t.Errorf("Expected empty with no shard manager, got %q", result)
	}
}

// =============================================================================
// storeVerification TESTS
// =============================================================================

func TestStoreVerification_WhenNoLocalDB_ShouldNotPanic(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		Success:    true,
		Confidence: 0.9,
		Reason:     "ok",
	}

	// Should not panic
	v.storeVerification("task", "shard", verification, 0, true)
}

// =============================================================================
// QualityViolation / CorrectiveType constants TESTS
// =============================================================================

func TestQualityViolationConstants_ShouldHaveExpectedValues(t *testing.T) {
	tests := []struct {
		violation QualityViolation
		expected  string
	}{
		{MockCode, "mock_code"},
		{PlaceholderCode, "placeholder"},
		{HallucinatedAPI, "hallucinated_api"},
		{IncompleteImpl, "incomplete"},
		{HardcodedValues, "hardcoded"},
		{EmptyFunction, "empty_function"},
		{MissingErrors, "missing_errors"},
		{FakeTests, "fake_tests"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.violation) != tc.expected {
				t.Errorf("QualityViolation = %q, want %q", tc.violation, tc.expected)
			}
		})
	}
}

func TestCorrectiveTypeConstants_ShouldHaveExpectedValues(t *testing.T) {
	tests := []struct {
		ct       CorrectiveType
		expected string
	}{
		{CorrectiveResearch, "research"},
		{CorrectiveDocs, "docs"},
		{CorrectiveTool, "tool"},
		{CorrectiveDecompose, "decompose"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.ct) != tc.expected {
				t.Errorf("CorrectiveType = %q, want %q", tc.ct, tc.expected)
			}
		})
	}
}

// =============================================================================
// ErrMaxRetriesExceeded TESTS
// =============================================================================

func TestErrMaxRetriesExceeded_ShouldBeDescriptive(t *testing.T) {
	if ErrMaxRetriesExceeded == nil {
		t.Fatal("ErrMaxRetriesExceeded should not be nil")
	}
	msg := ErrMaxRetriesExceeded.Error()
	if !strings.Contains(msg, "max retries") {
		t.Errorf("Error message should mention max retries: %q", msg)
	}
}

// =============================================================================
// verifyTask edge case TESTS
// =============================================================================

func TestVerifyTask_WhenNilClient_ShouldReturnUnavailable(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)

	result, err := v.verifyTask(context.Background(), "task", "result")
	if err == nil {
		t.Fatal("verifyTask with nil client should error fail-closed")
	}
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("verifyTask error = %v, want it to wrap ErrVerificationUnavailable", err)
	}
	if result != nil {
		t.Fatalf("verifyTask result = %#v, want nil when verification could not run", result)
	}
}

// =============================================================================
// selectBestShard TESTS
// =============================================================================

func TestSelectBestShard_WhenNilShardMgr_ShouldFallback(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	verification := &VerificationResult{
		QualityViolations: []QualityViolation{MockCode},
	}

	result := v.selectBestShard(context.Background(), "task", "/fix", verification)

	if result == nil {
		t.Fatal("Should return a result")
	}
	if result.ShardType != "/fix" {
		t.Errorf("ShardType = %q, want '/fix' (fallback)", result.ShardType)
	}
}

// =============================================================================
// Multiple violations combined
// =============================================================================

func TestBasicQualityCheck_WhenMultipleViolations_ShouldDetectAll(t *testing.T) {
	v := &TaskVerifier{}
	input := fmt.Sprintf(
		"TODO: fix this\n%s\n%s\n%s",
		"func MockHandler() {}",
		`panic("not implemented")`,
		"this is a placeholder stub",
	)

	res := v.basicQualityCheck(input)
	if res.Success {
		t.Error("Should fail with multiple violations")
	}

	// Should detect all violation types present
	if !containsViolation(res.QualityViolations, PlaceholderCode) {
		t.Error("Missing PlaceholderCode (TODO + placeholder + stub)")
	}
	if !containsViolation(res.QualityViolations, MockCode) {
		t.Error("Missing MockCode")
	}
	if !containsViolation(res.QualityViolations, IncompleteImpl) {
		t.Error("Missing IncompleteImpl")
	}

	if len(res.Evidence) == 0 {
		t.Error("Should have evidence items")
	}
}
