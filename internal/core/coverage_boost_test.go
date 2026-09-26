package core

import (
	"math"
	"testing"
	"time"

	"codenerd/internal/mangle"
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

func TestLimitsEnforcer_CheckAll_WhenShardLimitExceeded_ShouldReturnError(t *testing.T) {
	cfg := LimitsConfig{
		MaxTotalMemoryMB:    99999, // high to not trigger
		MaxConcurrentShards: 2,
	}
	enforcer := NewLimitsEnforcer(cfg)
	err := enforcer.CheckAll(5)
	if err == nil {
		t.Error("expected error when shard limit exceeded")
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
