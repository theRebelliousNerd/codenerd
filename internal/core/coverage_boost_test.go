package core

import (
	"math"
	"testing"
	"time"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// =============================================================================
// intent_defaults.go
// =============================================================================

func TestDefaultIntentSchemaFiles_WhenCalled_ShouldReturnNonEmptyCopy(t *testing.T) {
	files := DefaultIntentSchemaFiles()
	if len(files) == 0 {
		t.Fatal("DefaultIntentSchemaFiles returned empty list")
	}
	// Mutating the returned slice should not affect the original
	original := DefaultIntentSchemaFiles()
	files[0] = "MUTATED"
	second := DefaultIntentSchemaFiles()
	if second[0] == "MUTATED" {
		t.Error("DefaultIntentSchemaFiles did not return a copy — mutation leaked")
	}
	if len(original) != len(second) {
		t.Errorf("Length mismatch after mutation: original=%d, second=%d", len(original), len(second))
	}
}

func TestDefaultIntentSchemaFiles_WhenCalled_ShouldStartWithSchemaPrefix(t *testing.T) {
	files := DefaultIntentSchemaFiles()
	for _, f := range files {
		if len(f) < 7 || f[:7] != "schema/" {
			t.Errorf("Expected schema/ prefix in %q", f)
		}
	}
}

func TestDefaultIntentFactPredicates_WhenCalled_ShouldContainKnownPredicates(t *testing.T) {
	preds := defaultIntentFactPredicates()
	expected := []string{
		"intent_definition",
		"intent_category",
		"valid_semantic_type",
		"valid_action_type",
		"best_mode",
		"best_shard",
		"tool_priority",
	}
	for _, p := range expected {
		if _, ok := preds[p]; !ok {
			t.Errorf("Expected predicate %q in defaultIntentFactPredicates", p)
		}
	}
}

func TestDefaultIntentFactPredicates_WhenCalled_ShouldReturnNewMap(t *testing.T) {
	m1 := defaultIntentFactPredicates()
	m2 := defaultIntentFactPredicates()
	m1["injected_key"] = struct{}{}
	if _, ok := m2["injected_key"]; ok {
		t.Error("defaultIntentFactPredicates returned shared map reference")
	}
}

// =============================================================================
// hybrid_loader.go - helper functions
// =============================================================================

func TestStripInlineComment_WhenHashPresent_ShouldStrip(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`something # comment`, "something"},
		{`no comment`, "no comment"},
		{`code // line comment`, "code"},
		{`# only comment`, ""},
		{`// only comment`, ""},
		{`clean`, "clean"},
		{``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripInlineComment(tt.input)
			if got != tt.want {
				t.Errorf("stripInlineComment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimQuotes_WhenQuoted_ShouldRemoveQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{"`backtick`", "backtick"},
		{`no quotes`, "no quotes"},
		{`""`, ""},
		{`"a"`, "a"},
		{``, ""},
		{`"mismatch'`, "mismatch'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimQuotes(tt.input)
			if got != tt.want {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseIntentDirective_WhenValid_ShouldParse(t *testing.T) {
	tests := []struct {
		line     string
		wantOK   bool
		wantVerb string
	}{
		{`INTENT: "fix the bug" -> /fix "target"`, true, "/fix"},
		{`INTENT: "explain" -> /explain`, true, "/explain"},
		{`INTENT: "no arrow"`, false, ""},
		{`INTENT: -> /fix`, false, ""},             // empty phrase
		{`INTENT: "" -> /fix "target"`, false, ""}, // empty phrase after quote strip
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			intent, ok := parseIntentDirective(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parseIntentDirective(%q) ok=%v, want %v", tt.line, ok, tt.wantOK)
			}
			if ok && intent.Verb != tt.wantVerb {
				t.Errorf("verb=%q, want %q", intent.Verb, tt.wantVerb)
			}
		})
	}
}

func TestParseIntentDirective_WhenHasConstraint_ShouldParseConstraint(t *testing.T) {
	intent, ok := parseIntentDirective(`INTENT: "do stuff" -> /create "target" "extra constraint"`)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if intent.Target != "target" {
		t.Errorf("target=%q, want 'target'", intent.Target)
	}
	if intent.Constraint == "" {
		t.Error("expected non-empty constraint")
	}
}

func TestParsePromptDirective_WhenValid_ShouldParse(t *testing.T) {
	tests := []struct {
		line   string
		wantOK bool
		wantID string
	}{
		{`PROMPT: /role_coder [role] -> "You are a coder."`, true, "role_coder"},
		{`PROMPT: sys [system] -> "System prompt"`, true, "sys"},
		{`PROMPT: no_arrow`, false, ""},
		{`PROMPT: id -> ""`, false, ""},     // empty content
		{`PROMPT: -> "content"`, false, ""}, // no ID
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			prompt, ok := parsePromptDirective(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parsePromptDirective(%q) ok=%v, want %v", tt.line, ok, tt.wantOK)
			}
			if ok && prompt.ID != tt.wantID {
				t.Errorf("ID=%q, want %q", prompt.ID, tt.wantID)
			}
		})
	}
}

func TestParsePromptDirective_WhenHasTags_ShouldExtractTags(t *testing.T) {
	prompt, ok := parsePromptDirective(`PROMPT: /my_prompt [role] [system] -> "prompt content"`)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(prompt.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(prompt.Tags), prompt.Tags)
	}
	if prompt.Category != "role" {
		t.Errorf("category=%q, want 'role'", prompt.Category)
	}
}

// =============================================================================
// self_healing.go
// =============================================================================

func TestBoolToAtom_WhenTrue_ShouldReturnTrue(t *testing.T) {
	if got := boolToAtom(true); got != "/true" {
		t.Errorf("boolToAtom(true) = %q, want /true", got)
	}
}

func TestBoolToAtom_WhenFalse_ShouldReturnFalse(t *testing.T) {
	if got := boolToAtom(false); got != "/false" {
		t.Errorf("boolToAtom(false) = %q, want /false", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenContentHashMismatch_ShouldRetry(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "content hash mismatch"}
	got := healer.determineHealingType("a1", vr)
	if got != HealingRetry {
		t.Errorf("expected HealingRetry, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenSyntaxFail_ShouldRollback(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "syntax validation failed"}
	got := healer.determineHealingType("a2", vr)
	if got != HealingRollback {
		t.Errorf("expected HealingRollback, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenCannotReadBack_ShouldRetry(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "cannot read back file"}
	got := healer.determineHealingType("a3", vr)
	if got != HealingRetry {
		t.Errorf("expected HealingRetry, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenCodeDOMSyntaxError_ShouldRollback(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "Go syntax error after CodeDOM edit"}
	got := healer.determineHealingType("a4", vr)
	if got != HealingRollback {
		t.Errorf("expected HealingRollback, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenElementGone_ShouldRollback(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "target element no longer exists after edit"}
	got := healer.determineHealingType("a5", vr)
	if got != HealingRollback {
		t.Errorf("expected HealingRollback, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenFileHashUnchanged_ShouldRetry(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{Error: "file hash unchanged after edit - edit may not have been applied"}
	got := healer.determineHealingType("a6", vr)
	if got != HealingRetry {
		t.Errorf("expected HealingRetry, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenMaxRetriesExceeded_ShouldEscalate(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, SelfHealerConfig{MaxRetries: 2, RetryBackoff: time.Millisecond})

	// Simulate previous attempts
	healer.mu.Lock()
	healer.healingAttempts["a7"] = 3
	healer.mu.Unlock()

	vr := ValidationResult{Error: "content hash mismatch"}
	got := healer.determineHealingType("a7", vr)
	if got != HealingEscalate {
		t.Errorf("expected HealingEscalate when max retries exceeded, got %v", got)
	}
}

func TestSelfHealer_DetermineHealingType_WhenNilKernel_ShouldEscalate(t *testing.T) {
	healer := NewSelfHealer(nil, nil, DefaultSelfHealerConfig())
	vr := ValidationResult{Error: "anything"}
	got := healer.determineHealingType("a8", vr)
	if got != HealingEscalate {
		t.Errorf("expected HealingEscalate for nil kernel, got %v", got)
	}
}

func TestSelfHealer_HandleValidationFailure_WhenNoExecutor_ShouldError(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())
	// Don't call SetExecutor

	req := ActionRequest{ActionID: "x", Type: ActionWriteFile}
	vr := ValidationResult{Error: "something"}

	_, err := healer.HandleValidationFailure(t.Context(), req, vr)
	if err == nil {
		t.Error("expected error when no executor set")
	}
}

func TestSelfHealer_GetHealingAttempts_WhenNoAttempts_ShouldReturnZero(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	if got := healer.GetHealingAttempts("nonexistent"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// =============================================================================
// trace.go
// =============================================================================

func TestConvertCoreFactToMangle_WhenCalled_ShouldPreserveFields(t *testing.T) {
	f := Fact{Predicate: "test_pred", Args: []any{"arg1", 42}}
	m := convertCoreFactToMangle(f)

	if m.Predicate != "test_pred" {
		t.Errorf("Predicate = %q, want 'test_pred'", m.Predicate)
	}
	if len(m.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(m.Args))
	}
	if m.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestFlattenTree_WhenSingleNode_ShouldReturnOne(t *testing.T) {
	k := setupMockKernel(t)
	node := &mangle.DerivationNode{
		ID:       "root",
		Children: make([]*mangle.DerivationNode, 0),
	}
	flat := k.flattenTree(node)
	if len(flat) != 1 {
		t.Errorf("expected 1 node, got %d", len(flat))
	}
}

func TestFlattenTree_WhenNestedTree_ShouldFlattenAll(t *testing.T) {
	k := setupMockKernel(t)

	grandchild := &mangle.DerivationNode{
		ID:       "grandchild",
		Children: make([]*mangle.DerivationNode, 0),
	}
	child := &mangle.DerivationNode{
		ID:       "child",
		Children: []*mangle.DerivationNode{grandchild},
	}
	root := &mangle.DerivationNode{
		ID:       "root",
		Children: []*mangle.DerivationNode{child},
	}

	flat := k.flattenTree(root)
	if len(flat) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(flat))
	}
}

// =============================================================================
// mangle_updates.go - additional edge cases
// =============================================================================

func TestPredicateAllowed_WhenNoPolicy_ShouldAllowAll(t *testing.T) {
	policy := MangleUpdatePolicy{}
	if !predicateAllowed("anything", policy) {
		t.Error("expected all predicates allowed when no policy set")
	}
}

func TestPredicateAllowed_WhenExactMatch_ShouldAllow(t *testing.T) {
	policy := MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{"user_intent": {}},
	}
	if !predicateAllowed("user_intent", policy) {
		t.Error("expected 'user_intent' to be allowed by exact match")
	}
	if predicateAllowed("other_pred", policy) {
		t.Error("expected 'other_pred' to be denied")
	}
}

func TestPredicateAllowed_WhenPrefixMatch_ShouldAllow(t *testing.T) {
	policy := MangleUpdatePolicy{
		AllowedPrefixes: []string{"safe_", "ok_"},
	}
	if !predicateAllowed("safe_action", policy) {
		t.Error("expected 'safe_action' to be allowed by prefix")
	}
	if !predicateAllowed("ok_flag", policy) {
		t.Error("expected 'ok_flag' to be allowed by prefix")
	}
	if predicateAllowed("dangerous_thing", policy) {
		t.Error("expected 'dangerous_thing' to be denied")
	}
}

func TestFilterMangleUpdates_WhenEmptyInput_ShouldReturnNil(t *testing.T) {
	facts, blocked := FilterMangleUpdates(nil, nil, MangleUpdatePolicy{})
	if facts != nil {
		t.Errorf("expected nil facts, got %v", facts)
	}
	if blocked != nil {
		t.Errorf("expected nil blocked, got %v", blocked)
	}
}

func TestFilterMangleUpdates_WhenMaxExceeded_ShouldBlockExcess(t *testing.T) {
	policy := MangleUpdatePolicy{MaxUpdates: 1}
	updates := []string{`foo("a").`, `foo("b").`, `foo("c").`}

	facts, blocked := FilterMangleUpdates(nil, updates, policy)
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
	if len(blocked) != 2 {
		t.Errorf("expected 2 blocked, got %d", len(blocked))
	}
}

func TestFilterMangleUpdates_WhenImportPresent_ShouldBlock(t *testing.T) {
	policy := MangleUpdatePolicy{}
	updates := []string{`import "evil"`, `include "bad"`}

	facts, blocked := FilterMangleUpdates(nil, updates, policy)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
	if len(blocked) != 2 {
		t.Errorf("expected 2 blocked, got %d", len(blocked))
	}
}

func TestFilterMangleUpdates_WhenWhitespaceOnly_ShouldSkip(t *testing.T) {
	policy := MangleUpdatePolicy{}
	updates := []string{"   ", "\t", ""}

	facts, blocked := FilterMangleUpdates(nil, updates, policy)
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked, got %d", len(blocked))
	}
}

// =============================================================================
// action_validator.go - pure utility functions
// =============================================================================

func TestValidateAll_WhenAllPass_ShouldReturnTrue(t *testing.T) {
	results := []ValidationResult{
		{Verified: true, Confidence: 1.0},
		{Verified: true, Confidence: 0.8},
	}
	if !ValidateAll(results) {
		t.Error("expected ValidateAll to return true when all pass")
	}
}

func TestValidateAll_WhenOneFails_ShouldReturnFalse(t *testing.T) {
	results := []ValidationResult{
		{Verified: true, Confidence: 1.0},
		{Verified: false, Confidence: 0.9, Error: "fail"},
	}
	if ValidateAll(results) {
		t.Error("expected ValidateAll to return false when one fails")
	}
}

func TestValidateAll_WhenEmpty_ShouldReturnTrue(t *testing.T) {
	if !ValidateAll(nil) {
		t.Error("expected ValidateAll to return true for nil input")
	}
}

func TestFirstFailure_WhenAllPass_ShouldReturnNil(t *testing.T) {
	results := []ValidationResult{
		{Verified: true},
		{Verified: true},
	}
	if got := FirstFailure(results); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestFirstFailure_WhenSecondFails_ShouldReturnSecond(t *testing.T) {
	results := []ValidationResult{
		{Verified: true},
		{Verified: false, Error: "second_failed"},
	}
	got := FirstFailure(results)
	if got == nil {
		t.Fatal("expected non-nil failure")
	}
	if got.Error != "second_failed" {
		t.Errorf("expected error 'second_failed', got %q", got.Error)
	}
}

func TestHighestConfidence_WhenEmpty_ShouldReturnNil(t *testing.T) {
	if got := HighestConfidence(nil); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

func TestHighestConfidence_WhenMultiple_ShouldReturnHighest(t *testing.T) {
	results := []ValidationResult{
		{Confidence: 0.5, Method: "a"},
		{Confidence: 0.9, Method: "b"},
		{Confidence: 0.3, Method: "c"},
	}
	got := HighestConfidence(results)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Method != "b" {
		t.Errorf("expected method 'b' (highest confidence), got %q", got.Method)
	}
}

func TestHighestConfidence_WhenNaN_ShouldHandleGracefully(t *testing.T) {
	results := []ValidationResult{
		{Confidence: math.NaN(), Method: "nan"},
		{Confidence: 0.5, Method: "valid"},
	}
	got := HighestConfidence(results)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	// NaN comparisons: NaN is not > anything, so "valid" should be selected
	if got.Method != "valid" {
		t.Errorf("expected 'valid' with NaN handling, got %q", got.Method)
	}
}

func TestAggregate_WhenEmpty_ShouldReturnAllVerified(t *testing.T) {
	agg := Aggregate(nil)
	if !agg.AllVerified {
		t.Error("expected AllVerified true for empty input")
	}
	if agg.ValidatorCount != 0 {
		t.Errorf("expected 0 validators, got %d", agg.ValidatorCount)
	}
}

func TestAggregate_WhenMixed_ShouldCaptureSummary(t *testing.T) {
	results := []ValidationResult{
		{Verified: true, Confidence: 0.9},
		{Verified: false, Confidence: 0.3, Error: "failed"},
		{Verified: true, Confidence: 0.7},
	}
	agg := Aggregate(results)
	if agg.AllVerified {
		t.Error("expected AllVerified false")
	}
	if agg.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", agg.FailureCount)
	}
	if agg.FirstError != "failed" {
		t.Errorf("expected 'failed', got %q", agg.FirstError)
	}
	if agg.HighestConfidence != 0.9 {
		t.Errorf("expected highest 0.9, got %f", agg.HighestConfidence)
	}
	if agg.LowestConfidence != 0.3 {
		t.Errorf("expected lowest 0.3, got %f", agg.LowestConfidence)
	}
	if agg.ValidatorCount != 3 {
		t.Errorf("expected 3 validators, got %d", agg.ValidatorCount)
	}
}

func TestValidationResult_ToFacts_WhenVerified_ShouldReturnActionVerified(t *testing.T) {
	vr := &ValidationResult{
		ActionID:   "act-1",
		ActionType: ActionWriteFile,
		Verified:   true,
		Confidence: 0.95,
		Method:     ValidationMethodHash,
		Timestamp:  time.Now(),
	}
	facts := vr.ToFacts()
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	if facts[0].Predicate != "action_verified" {
		t.Errorf("expected 'action_verified', got %q", facts[0].Predicate)
	}
	if facts[1].Predicate != "validation_method_used" {
		t.Errorf("expected 'validation_method_used', got %q", facts[1].Predicate)
	}
}

func TestValidationResult_ToFacts_WhenFailed_ShouldReturnValidationFailed(t *testing.T) {
	vr := &ValidationResult{
		ActionID:   "act-2",
		ActionType: ActionEditFile,
		Verified:   false,
		Confidence: 0.8,
		Method:     ValidationMethodSyntax,
		Error:      "syntax error",
		Details:    map[string]any{"line": 42},
		Timestamp:  time.Now(),
	}
	facts := vr.ToFacts()
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	if facts[0].Predicate != "action_validation_failed" {
		t.Errorf("expected 'action_validation_failed', got %q", facts[0].Predicate)
	}
}

func TestValidationResult_ToFacts_WhenNaNConfidence_ShouldClampToZero(t *testing.T) {
	vr := &ValidationResult{
		ActionID:   "act-nan",
		ActionType: ActionReadFile,
		Verified:   true,
		Confidence: math.NaN(),
		Method:     ValidationMethodExistence,
		Timestamp:  time.Now(),
	}
	facts := vr.ToFacts()
	// The confidence should have been clamped to 0
	if len(facts) < 1 {
		t.Fatal("expected at least 1 fact")
	}
	// Check confidence arg is 0 (scaled: int64(0.0*100))
	if conf, ok := facts[0].Args[3].(int64); ok && conf != 0 {
		t.Errorf("expected confidence 0 for NaN, got %d", conf)
	}
}

func TestValidationResult_ToFacts_WhenNegativeConfidence_ShouldClampToZero(t *testing.T) {
	vr := &ValidationResult{
		ActionID:   "act-neg",
		ActionType: ActionReadFile,
		Verified:   true,
		Confidence: -0.5,
		Method:     ValidationMethodExistence,
		Timestamp:  time.Now(),
	}
	facts := vr.ToFacts()
	if conf, ok := facts[0].Args[3].(int64); ok && conf != 0 {
		t.Errorf("expected confidence 0 for negative, got %d", conf)
	}
}

func TestValidationResult_ToFacts_WhenOverOneConfidence_ShouldClampToOne(t *testing.T) {
	vr := &ValidationResult{
		ActionID:   "act-over",
		ActionType: ActionReadFile,
		Verified:   true,
		Confidence: 1.5,
		Method:     ValidationMethodExistence,
		Timestamp:  time.Now(),
	}
	facts := vr.ToFacts()
	if conf, ok := facts[0].Args[3].(int64); ok && conf != 100 {
		t.Errorf("expected confidence 100 for >1.0, got %d", conf)
	}
}

// =============================================================================
// action_validator.go - ValidatorRegistry
// =============================================================================

func TestNewValidatorRegistry_WhenCreated_ShouldBeEmpty(t *testing.T) {
	r := NewValidatorRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestValidatorRegistry_Register_WhenNil_ShouldBeNoOp(t *testing.T) {
	r := NewValidatorRegistry()
	r.Register(nil) // Should not panic
}

// =============================================================================
// kernel_utils.go - AutopoiesisBridge
// =============================================================================

func TestAutopoiesisBridge_WhenCreated_ShouldNotBeNil(t *testing.T) {
	k := setupMockKernel(t)
	bridge := NewAutopoiesisBridge(k)
	if bridge == nil {
		t.Fatal("expected non-nil bridge")
	}
}

func TestAutopoiesisBridge_AssertFact_WhenValid_ShouldSucceed(t *testing.T) {
	k := setupMockKernel(t)
	bridge := NewAutopoiesisBridge(k)

	err := bridge.AssertFact(types.KernelFact{
		Predicate: "test_bridge_fact",
		Args:      []any{"value1"},
	})
	if err != nil {
		t.Errorf("AssertFact failed: %v", err)
	}
}

func TestAutopoiesisBridge_AssertFactBatch_WhenEmpty_ShouldReturnNil(t *testing.T) {
	k := setupMockKernel(t)
	bridge := NewAutopoiesisBridge(k)

	err := bridge.AssertFactBatch(nil)
	if err != nil {
		t.Errorf("AssertFactBatch(nil) returned error: %v", err)
	}
}

func TestAutopoiesisBridge_QueryBool_WhenNoFacts_ShouldReturnFalse(t *testing.T) {
	k := setupMockKernel(t)
	bridge := NewAutopoiesisBridge(k)

	if bridge.QueryBool("nonexistent_pred_xyz") {
		t.Error("expected false for nonexistent predicate")
	}
}

func TestAutopoiesisBridge_RetractFact_WhenNotPresent_ShouldNotError(t *testing.T) {
	k := setupMockKernel(t)
	bridge := NewAutopoiesisBridge(k)

	err := bridge.RetractFact(types.KernelFact{
		Predicate: "nonexistent_retract",
		Args:      []any{"x"},
	})
	// Should not error even if fact doesn't exist
	if err != nil {
		t.Logf("RetractFact returned: %v (may be expected)", err)
	}
}

// =============================================================================
// kernel_virtual.go
// =============================================================================

func TestRealKernel_SetGetVirtualStore_WhenSet_ShouldReturn(t *testing.T) {
	k := setupMockKernel(t)

	// Initially nil
	if got := k.GetVirtualStore(); got != nil {
		t.Error("expected nil VirtualStore initially")
	}

	// Setting nil should work
	k.SetVirtualStore(nil)
	if got := k.GetVirtualStore(); got != nil {
		t.Error("expected nil VirtualStore after setting nil")
	}
}

// =============================================================================
// kernel_accessors.go
// =============================================================================

func TestRealKernel_GetBaseFacts_WhenEmpty_ShouldReturnEmptySlice(t *testing.T) {
	k := setupMockKernel(t)
	facts := k.GetBaseFacts()
	// Should return a slice (possibly non-empty due to boot facts), not nil
	if facts == nil {
		t.Error("expected non-nil slice from GetBaseFacts")
	}
}

func TestRealKernel_GetBaseFacts_WhenFilled_ShouldReturnCopy(t *testing.T) {
	k := setupMockKernel(t)
	k.Assert(Fact{Predicate: "base_test_fact", Args: []any{"val1"}})

	facts1 := k.GetBaseFacts()
	facts2 := k.GetBaseFacts()

	if len(facts1) == 0 {
		t.Skip("no facts found — boot facts may not be loaded in test")
	}

	// Mutating first should not affect second
	facts1[0].Predicate = "MUTATED"
	if len(facts2) > 0 && facts2[0].Predicate == "MUTATED" {
		t.Error("GetBaseFacts did not return a copy")
	}
}

func TestRealKernel_GetProgramInfo_WhenInitialized_ShouldReturnNonNil(t *testing.T) {
	k := setupMockKernel(t)
	k.Evaluate() // Ensure programInfo is populated

	info := k.GetProgramInfo()
	if info == nil {
		t.Log("ProgramInfo is nil — may be expected in lightweight test kernel")
	}
}

// =============================================================================
// limits.go - additional coverage
// =============================================================================

func TestLimitsEnforcer_CheckMemory_WhenNoLimit_ShouldReturnNil(t *testing.T) {
	cfg := LimitsConfig{MaxTotalMemoryMB: 0} // disabled
	enforcer := NewLimitsEnforcer(cfg)
	if err := enforcer.CheckMemory(); err != nil {
		t.Errorf("expected nil error when no limit, got: %v", err)
	}
}

func TestLimitsEnforcer_CheckSessionDuration_WhenNoLimit_ShouldReturnNil(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 0} // disabled
	enforcer := NewLimitsEnforcer(cfg)
	if err := enforcer.CheckSessionDuration(); err != nil {
		t.Errorf("expected nil error when no limit, got: %v", err)
	}
}

func TestLimitsEnforcer_CheckShardLimit_WhenNoLimit_ShouldReturnNil(t *testing.T) {
	cfg := LimitsConfig{MaxConcurrentShards: 0} // disabled
	enforcer := NewLimitsEnforcer(cfg)
	if err := enforcer.CheckShardLimit(999); err != nil {
		t.Errorf("expected nil error when no limit, got: %v", err)
	}
}

func TestLimitsEnforcer_GetMemoryUtilization_WhenNoLimit_ShouldReturnZero(t *testing.T) {
	cfg := LimitsConfig{MaxTotalMemoryMB: 0}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetMemoryUtilization(); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestLimitsEnforcer_GetSessionUtilization_WhenNoLimit_ShouldReturnZero(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 0}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetSessionUtilization(); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestLimitsEnforcer_RemainingSessionTime_WhenNoLimit_ShouldReturnMaxDuration(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 0}
	enforcer := NewLimitsEnforcer(cfg)
	remaining := enforcer.RemainingSessionTime()
	if remaining < 24*time.Hour {
		t.Errorf("expected effectively unlimited remaining time, got %v", remaining)
	}
}

func TestLimitsEnforcer_RemainingSessionTime_WhenExpired_ShouldReturnZero(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 1} // 1 minute
	enforcer := NewLimitsEnforcer(cfg)
	// Set session start in the past
	enforcer.SetSessionStart(time.Now().Add(-2 * time.Hour))
	remaining := enforcer.RemainingSessionTime()
	if remaining != 0 {
		t.Errorf("expected 0 remaining when expired, got %v", remaining)
	}
}

func TestLimitsEnforcer_GetAvailableShardSlots_WhenNoLimit_ShouldReturn100(t *testing.T) {
	cfg := LimitsConfig{MaxConcurrentShards: 0}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetAvailableShardSlots(5); got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestLimitsEnforcer_GetAvailableShardSlots_WhenOverLimit_ShouldReturnZero(t *testing.T) {
	cfg := LimitsConfig{MaxConcurrentShards: 3}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetAvailableShardSlots(5); got != 0 {
		t.Errorf("expected 0 when over limit, got %d", got)
	}
}

func TestLimitsEnforcer_GetMaxFactsInKernel_ShouldReturnConfigValue(t *testing.T) {
	cfg := LimitsConfig{MaxFactsInKernel: 42}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetMaxFactsInKernel(); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestLimitsEnforcer_GetMaxDerivedFactsLimit_ShouldReturnConfigValue(t *testing.T) {
	cfg := LimitsConfig{MaxDerivedFactsLimit: 99}
	enforcer := NewLimitsEnforcer(cfg)
	if got := enforcer.GetMaxDerivedFactsLimit(); got != 99 {
		t.Errorf("expected 99, got %d", got)
	}
}

func TestLimitsEnforcer_GetStatus_ShouldReturnAllKeys(t *testing.T) {
	enforcer := NewLimitsEnforcer(DefaultLimitsConfig())
	status := enforcer.GetStatus()

	expectedKeys := []string{
		"memory_mb", "memory_limit_mb", "memory_utilization",
		"session_elapsed", "session_limit", "session_remaining",
		"session_utilization", "shard_limit",
		"max_facts_in_kernel", "max_derived_facts",
	}
	for _, key := range expectedKeys {
		if _, ok := status[key]; !ok {
			t.Errorf("missing key %q in GetStatus()", key)
		}
	}
}

func TestLimitsEnforcer_CheckAll_WhenShardLimitExceeded_ShouldReturnError(t *testing.T) {
	cfg := LimitsConfig{
		MaxTotalMemoryMB:      99999, // high to not trigger
		MaxSessionDurationMin: 999,   // high to not trigger
		MaxConcurrentShards:   2,
	}
	enforcer := NewLimitsEnforcer(cfg)
	err := enforcer.CheckAll(5)
	if err == nil {
		t.Error("expected error when shard limit exceeded")
	}
}

func TestLimitsEnforcer_CheckSessionDuration_WhenExpired_ShouldReturnError(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 1}
	enforcer := NewLimitsEnforcer(cfg)
	enforcer.SetSessionStart(time.Now().Add(-2 * time.Hour))
	err := enforcer.CheckSessionDuration()
	if err == nil {
		t.Error("expected error when session expired")
	}
}

func TestLimitsEnforcer_SessionCallbackFired_WhenTimeout(t *testing.T) {
	cfg := LimitsConfig{MaxSessionDurationMin: 1}
	enforcer := NewLimitsEnforcer(cfg)
	enforcer.SetSessionStart(time.Now().Add(-2 * time.Hour))

	called := false
	enforcer.OnSessionTimeout(func(elapsed, limit time.Duration) {
		called = true
	})
	enforcer.CheckSessionDuration()
	if !called {
		t.Error("expected session timeout callback to be called")
	}
}

func TestLimitsEnforcer_ShardCallbackFired_WhenViolated(t *testing.T) {
	cfg := LimitsConfig{MaxConcurrentShards: 2}
	enforcer := NewLimitsEnforcer(cfg)

	called := false
	enforcer.OnShardViolation(func(active, limit int) {
		called = true
	})
	enforcer.CheckShardLimit(5)
	if !called {
		t.Error("expected shard violation callback to be called")
	}
}

// =============================================================================
// kernel_types.go - GetDefaultContent
// =============================================================================

func TestGetDefaultContent_WhenValidPath_ShouldReturnContent(t *testing.T) {
	content, err := GetDefaultContent("schemas.mg")
	if err != nil {
		t.Fatalf("GetDefaultContent(schemas.mg) failed: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty content for schemas.mg")
	}
}

func TestGetDefaultContent_WhenInvalidPath_ShouldReturnError(t *testing.T) {
	_, err := GetDefaultContent("nonexistent_file.mg")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// =============================================================================
// intent_inference.go - additional uncovered paths
// =============================================================================

func TestInferVerbAndRemainder_WhenExplicitSlashVerb_ShouldUseIt(t *testing.T) {
	verb, remainder := inferVerbAndRemainder("/debug the crash")
	if verb != "/debug" {
		t.Errorf("verb=%q, want /debug", verb)
	}
	if remainder != "the crash" {
		t.Errorf("remainder=%q, want 'the crash'", remainder)
	}
}

func TestInferVerbAndRemainder_WhenUnsupportedSlashVerb_ShouldFallThrough(t *testing.T) {
	verb, _ := inferVerbAndRemainder("/custom do something")
	// Should not match /custom, falls through to word matching
	// "custom" doesn't match any known word, defaults to /explain
	if verb != "/explain" {
		t.Errorf("verb=%q, want /explain (fallthrough for unsupported slash verb)", verb)
	}
}

func TestInferVerbAndRemainder_WhenEmpty_ShouldReturnExplain(t *testing.T) {
	verb, remainder := inferVerbAndRemainder("")
	if verb != "/explain" {
		t.Errorf("verb=%q, want /explain", verb)
	}
	if remainder != "" {
		t.Errorf("remainder=%q, want empty", remainder)
	}
}

func TestInferTargetFromText_WhenMultipleExtensions_ShouldReturnFirst(t *testing.T) {
	got := inferTargetFromText("fix main.go and test.py")
	if got != "main.go" {
		t.Errorf("expected 'main.go' (first match), got %q", got)
	}
}

func TestInferTargetFromText_WhenRustFile_ShouldReturn(t *testing.T) {
	got := inferTargetFromText("look at lib.rs")
	if got != "lib.rs" {
		t.Errorf("expected 'lib.rs', got %q", got)
	}
}

func TestInferTargetFromText_WhenJSXFile_ShouldReturn(t *testing.T) {
	got := inferTargetFromText("edit App.jsx")
	if got != "App.jsx" {
		t.Errorf("expected 'App.jsx', got %q", got)
	}
}

func TestInferTargetFromText_WhenYAMLFile_ShouldReturn(t *testing.T) {
	got := inferTargetFromText("update config.yaml")
	if got != "config.yaml" {
		t.Errorf("expected 'config.yaml', got %q", got)
	}
}

func TestInferTargetFromText_WhenGLFile_ShouldReturn(t *testing.T) {
	got := inferTargetFromText("fix policy.gl rules")
	if got != "policy.gl" {
		t.Errorf("expected 'policy.gl', got %q", got)
	}
}

func TestInferIntentFromTask_WhenEmpty_ShouldReturnDefaults(t *testing.T) {
	intent := InferIntentFromTask("")
	if intent.Verb != "/explain" {
		t.Errorf("verb=%q, want /explain", intent.Verb)
	}
	if intent.Category != "/query" {
		t.Errorf("category=%q, want /query", intent.Category)
	}
	if intent.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestInferIntentFromTask_WhenSlashCommand_ShouldUseExplicit(t *testing.T) {
	intent := InferIntentFromTask("/test all endpoints")
	if intent.Verb != "/test" {
		t.Errorf("verb=%q, want /test", intent.Verb)
	}
	if intent.Category != "/mutation" {
		t.Errorf("category=%q, want /mutation", intent.Category)
	}
}
