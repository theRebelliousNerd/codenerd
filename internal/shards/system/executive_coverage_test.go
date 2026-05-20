// executive_coverage_test.go — Coverage tests for executive.go, perception.go,
// campaign_runner.go, and mangle_repair.go utility/helper functions.
package system

import (
	"context"
	"testing"
	"time"
)

// ─── Executive Policy ────────────────────────────────────────────────────────

func TestExecutivePolicyShard_NewExecutivePolicyShard_ShouldHaveDefaults(t *testing.T) {
	shard := NewExecutivePolicyShard()
	if shard.ID != "executive_policy" {
		t.Errorf("ID = %q, want %q", shard.ID, "executive_policy")
	}
	if shard.StartupMode != StartupAuto {
		t.Errorf("StartupMode = %v, want Auto", shard.StartupMode)
	}
	if !shard.config.StrictBarriers {
		t.Error("StrictBarriers should default to true")
	}
	if shard.config.MaxActionsPerTick != 5 {
		t.Errorf("MaxActionsPerTick = %d, want 5", shard.config.MaxActionsPerTick)
	}
	if !shard.bootGuardActive {
		t.Error("bootGuardActive should default to true")
	}
	if shard.feedbackLoop == nil {
		t.Error("feedbackLoop should not be nil")
	}
}

func TestDefaultExecutiveConfig_ShouldHaveReasonableDefaults(t *testing.T) {
	cfg := DefaultExecutiveConfig()
	if cfg.TickInterval == 0 {
		t.Error("TickInterval should not be zero")
	}
	if !cfg.StrictBarriers {
		t.Error("StrictBarriers should default to true")
	}
	if cfg.MaxActionsPerTick != 5 {
		t.Errorf("MaxActionsPerTick = %d, want 5", cfg.MaxActionsPerTick)
	}
	if cfg.OODATimeout == 0 {
		t.Error("OODATimeout should not be zero")
	}
	if cfg.LearningCandidateThreshold != 3 {
		t.Errorf("LearningCandidateThreshold = %d, want 3", cfg.LearningCandidateThreshold)
	}
}

func TestExecutivePolicyShard_BootGuard_WhenActive_ShouldReturnTrue(t *testing.T) {
	shard := NewExecutivePolicyShard()
	if !shard.IsBootGuardActive() {
		t.Error("boot guard should be active on creation")
	}
}

func TestExecutivePolicyShard_DisableBootGuard_ShouldDeactivate(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.DisableBootGuard()
	if shard.IsBootGuardActive() {
		t.Error("boot guard should be inactive after DisableBootGuard")
	}
}

func TestExecutivePolicyShard_DisableBootGuard_WhenCalledTwice_ShouldBeIdempotent(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.DisableBootGuard()
	shard.DisableBootGuard()
	if shard.IsBootGuardActive() {
		t.Error("boot guard should remain inactive")
	}
}

func TestExecutivePolicyShard_ResetValidationBudget_ShouldNotPanic(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.ResetValidationBudget()
	// No panic = success
}

func TestExecutivePolicyShard_TrackSuccess_ShouldIncrementCounter(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.trackSuccess("test_pattern")
	shard.trackSuccess("test_pattern")

	shard.mu.RLock()
	count := shard.patternSuccess["test_pattern"]
	shard.mu.RUnlock()

	if count != 2 {
		t.Errorf("patternSuccess count = %d, want 2", count)
	}
}

func TestExecutivePolicyShard_TrackFailure_ShouldIncrementCounter(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.trackFailure("fail_pattern", "some reason")
	shard.trackFailure("fail_pattern", "another reason")
	shard.trackFailure("fail_pattern", "third reason")

	shard.mu.RLock()
	count := shard.patternFailure["fail_pattern"]
	shard.mu.RUnlock()

	if count != 3 {
		t.Errorf("patternFailure count = %d, want 3", count)
	}
}

func TestParseIntentTimestamp_WhenValidPrefix_ShouldReturnTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTS  int64
		wantOK  bool
	}{
		{"valid timestamp", "/intent_1234567890", 1234567890, true},
		{"invalid prefix", "/foo_123", 0, false},
		{"no prefix", "123", 0, false},
		{"empty", "", 0, false},
		{"non-numeric", "/intent_abc", 0, false},
		{"current_intent", "/current_intent", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, ok := parseIntentTimestamp(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseIntentTimestamp(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ts != tt.wantTS {
				t.Errorf("parseIntentTimestamp(%q) ts = %d, want %d", tt.input, ts, tt.wantTS)
			}
		})
	}
}

func TestCopyStringAnyMap_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	result := copyStringAnyMap(nil)
	if len(result) != 0 {
		t.Errorf("copyStringAnyMap(nil) len = %d, want 0", len(result))
	}
}

func TestCopyStringAnyMap_WhenPopulated_ShouldReturnCopy(t *testing.T) {
	src := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	dst := copyStringAnyMap(src)
	if len(dst) != 2 {
		t.Fatalf("copy len = %d, want 2", len(dst))
	}
	if dst["key1"] != "value1" {
		t.Errorf("copy[key1] = %v, want value1", dst["key1"])
	}

	// Modifying copy should not affect source
	dst["key3"] = "new"
	if _, ok := src["key3"]; ok {
		t.Error("modifying copy should not affect source")
	}
}

func TestDelegatedShardToAction_WhenKnownShards_ShouldReturnAction(t *testing.T) {
	tests := []struct {
		shardType string
		want      string
	}{
		{"/reviewer", "/delegate_reviewer"},
		{"reviewer", "/delegate_reviewer"},
		{"/coder", "/delegate_coder"},
		{"coder", "/delegate_coder"},
		{"/tester", "/delegate_tester"},
		{"tester", "/delegate_tester"},
		{"/researcher", "/delegate_researcher"},
		{"researcher", "/delegate_researcher"},
		{"/tool_generator", "/delegate_tool_generator"},
		{"tool_generator", "/delegate_tool_generator"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.shardType, func(t *testing.T) {
			got := delegatedShardToAction(tt.shardType)
			if got != tt.want {
				t.Errorf("delegatedShardToAction(%q) = %q, want %q", tt.shardType, got, tt.want)
			}
		})
	}
}

func TestExecutivePolicyShard_IntentFingerprint_WhenNil_ShouldReturnEmpty(t *testing.T) {
	shard := NewExecutivePolicyShard()
	result := shard.intentFingerprint(nil)
	if result != "" {
		t.Errorf("intentFingerprint(nil) = %q, want empty", result)
	}
}

func TestExecutivePolicyShard_IntentFingerprint_WhenPopulated_ShouldReturnComposite(t *testing.T) {
	shard := NewExecutivePolicyShard()
	intent := &userIntentSnapshot{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "main.go",
		Constraint: "fix error",
	}
	result := shard.intentFingerprint(intent)
	expected := "/mutation|/fix|main.go|fix error"
	if result != expected {
		t.Errorf("intentFingerprint = %q, want %q", result, expected)
	}
}

func TestExecutivePolicyShard_EvaluatePolicy_WhenNilKernel_ShouldReturnNil(t *testing.T) {
	shard := NewExecutivePolicyShard()
	err := shard.evaluatePolicy(context.Background())
	if err != nil {
		t.Errorf("evaluatePolicy with nil kernel = %v, want nil", err)
	}
}

func TestExecutivePolicyShard_GenerateShutdownSummary_ShouldContainStats(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.StartTime = time.Now().Add(-10 * time.Second)
	shard.mu.Lock()
	shard.decisionsCount = 42
	shard.blockCount = 5
	shard.strategyChanges = 3
	shard.mu.Unlock()

	summary := shard.generateShutdownSummary("test reason")
	if summary == "" {
		t.Error("summary should not be empty")
	}
	if !containsAll(summary, "test reason", "Decisions: 42", "Blocked: 5", "Strategy changes: 3") {
		t.Errorf("summary missing expected content: %q", summary)
	}
}

func TestExecutivePolicyShard_SetLearningStore_ShouldSetStore(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.SetLearningStore(nil) // Just testing it doesn't panic with nil
	// No panic = success
}

// ─── Perception ──────────────────────────────────────────────────────────────

func TestPerceptionFirewallShard_NewPerceptionFirewallShard_ShouldHaveDefaults(t *testing.T) {
	shard := NewPerceptionFirewallShard()
	if shard.ID != "perception_firewall" {
		t.Errorf("ID = %q, want %q", shard.ID, "perception_firewall")
	}
	if shard.StartupMode != StartupAuto {
		t.Errorf("StartupMode = %v, want Auto", shard.StartupMode)
	}
	if shard.config.ConfidenceThreshold != 0.85 {
		t.Errorf("ConfidenceThreshold = %v, want 0.85", shard.config.ConfidenceThreshold)
	}
	if shard.config.AmbiguityThreshold != 0.7 {
		t.Errorf("AmbiguityThreshold = %v, want 0.7", shard.config.AmbiguityThreshold)
	}
	if len(shard.verbPatterns) == 0 {
		t.Error("verbPatterns should not be empty")
	}
}

func TestDefaultPerceptionConfig_ShouldHaveReasonableDefaults(t *testing.T) {
	cfg := DefaultPerceptionConfig()
	if cfg.ConfidenceThreshold != 0.85 {
		t.Errorf("ConfidenceThreshold = %v, want 0.85", cfg.ConfidenceThreshold)
	}
	if cfg.AmbiguityThreshold != 0.7 {
		t.Errorf("AmbiguityThreshold = %v, want 0.7", cfg.AmbiguityThreshold)
	}
	if cfg.TickInterval == 0 {
		t.Error("TickInterval should not be zero")
	}
	if cfg.MaxQueueSize <= 0 {
		t.Error("MaxQueueSize should be positive")
	}
	if !cfg.UseFallbackParsing {
		t.Error("UseFallbackParsing should default to true")
	}
}

func TestNormalizeAtom_ShouldAddSlashPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"explain", "/explain"},
		{"/explain", "/explain"},
		{"none", "none"},
		{"", ""},
		{"  /review  ", "/review"},
		{"fix", "/fix"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAtom(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAtom(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildVerbPatterns_ShouldReturnNonEmpty(t *testing.T) {
	patterns := buildVerbPatterns()
	if len(patterns) == 0 {
		t.Error("buildVerbPatterns should return non-empty map")
	}

	expectedVerbs := []string{"explain", "review", "fix", "refactor", "create", "delete", "test", "search", "debug", "implement", "run", "research"}
	for _, verb := range expectedVerbs {
		if _, ok := patterns[verb]; !ok {
			t.Errorf("missing verb pattern for %q", verb)
		}
	}
}

func TestPerceptionFirewallShard_ParseWithFallback_WhenExplainInput_ShouldDetectVerb(t *testing.T) {
	shard := NewPerceptionFirewallShard()

	tests := []struct {
		name     string
		input    string
		wantVerb string
		wantCat  string
	}{
		{"explain", "explain how the kernel works", "/explain", "/query"},
		{"review", "please review my changes", "/review", "/query"},
		{"fix", "fix the bug in main.go", "/fix", "/mutation"},
		{"create", "create a new handler module", "/create", "/mutation"},
		{"search", "search for function declarations", "/search", "/query"},
		{"debug", "debug the crash", "/debug", "/instruction"},
		{"run", "please run the process now", "/run", "/instruction"},
		{"research", "investigate the algorithm", "/research", "/instruction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := shard.parseWithFallback(tt.input)
			if intent.Verb != tt.wantVerb {
				t.Errorf("verb = %q, want %q", intent.Verb, tt.wantVerb)
			}
			if intent.Category != tt.wantCat {
				t.Errorf("category = %q, want %q", intent.Category, tt.wantCat)
			}
			if intent.Confidence != 0.6 {
				t.Errorf("confidence = %v, want 0.6", intent.Confidence)
			}
		})
	}
}

func TestPerceptionFirewallShard_ParseWithFallback_ShouldExtractFilePath(t *testing.T) {
	shard := NewPerceptionFirewallShard()
	intent := shard.parseWithFallback("fix the bug in main.go")
	if intent.Target != "main.go" {
		t.Errorf("target = %q, want %q", intent.Target, "main.go")
	}
}

func TestPerceptionFirewallShard_SubmitInput_WhenQueueAvailable_ShouldSucceed(t *testing.T) {
	shard := NewPerceptionFirewallShard()
	err := shard.SubmitInput("test input")
	if err != nil {
		t.Fatalf("SubmitInput error = %v", err)
	}
}

func TestPerceptionFirewallShard_SubmitInput_WhenQueueFull_ShouldReturnError(t *testing.T) {
	cfg := DefaultPerceptionConfig()
	cfg.MaxQueueSize = 1
	shard := NewPerceptionFirewallShardWithConfig(cfg)

	// Fill queue
	err := shard.SubmitInput("first")
	if err != nil {
		t.Fatalf("first SubmitInput error = %v", err)
	}

	// Overflow
	err = shard.SubmitInput("second")
	if err == nil {
		t.Error("SubmitInput should return error when queue is full")
	}
}

func TestPerceptionFirewallShard_ResolveTarget_WhenFilePath_ShouldResolveHighConfidence(t *testing.T) {
	shard := NewPerceptionFirewallShard()
	// Need kernel for resolution
	shard.Kernel = nil // Will create one internally

	resolution := shard.resolveTarget(context.Background(), "internal/core/kernel.go")
	if resolution.ConfidencePercent < 85 {
		t.Errorf("confidence = %d, want >= 85 for file path", resolution.ConfidencePercent)
	}
	if resolution.ResolvedPath != "internal/core/kernel.go" {
		t.Errorf("resolvedPath = %q, want %q", resolution.ResolvedPath, "internal/core/kernel.go")
	}
}

func TestPerceptionFirewallShard_GenerateShutdownSummary_ShouldContainStats(t *testing.T) {
	shard := NewPerceptionFirewallShard()
	shard.StartTime = time.Now().Add(-5 * time.Second)
	shard.mu.Lock()
	shard.intentsProcessed = 10
	shard.clarifications = 2
	shard.mu.Unlock()

	summary := shard.generateShutdownSummary("test")
	if summary == "" {
		t.Error("summary should not be empty")
	}
	if !containsAll(summary, "test", "Intents: 10", "Clarifications: 2") {
		t.Errorf("summary missing expected content: %q", summary)
	}
}

func TestGuardedPerceptionClient_Complete_WhenNilBase_ShouldReturnError(t *testing.T) {
	client := guardedPerceptionClient{base: nil}
	_, err := client.Complete(context.Background(), "prompt")
	if err == nil {
		t.Error("Complete should return error when base is nil")
	}
}

func TestGuardedPerceptionClient_CompleteWithSystem_WhenNilBase_ShouldReturnError(t *testing.T) {
	client := guardedPerceptionClient{base: nil}
	_, err := client.CompleteWithSystem(context.Background(), "sys", "user")
	if err == nil {
		t.Error("CompleteWithSystem should return error when base is nil")
	}
}

func TestGuardedPerceptionClient_CompleteWithTools_WhenNilBase_ShouldReturnError(t *testing.T) {
	client := guardedPerceptionClient{base: nil}
	_, err := client.CompleteWithTools(context.Background(), "sys", "user", nil)
	if err == nil {
		t.Error("CompleteWithTools should return error when base is nil")
	}
}

// ─── Campaign Runner ─────────────────────────────────────────────────────────

func TestCampaignRunnerShard_NewCampaignRunnerShard_ShouldHaveDefaults(t *testing.T) {
	shard := NewCampaignRunnerShard()
	if shard.ID != "campaign_runner" {
		t.Errorf("ID = %q, want %q", shard.ID, "campaign_runner")
	}
	if shard.StartupMode != StartupOnDemand {
		t.Errorf("StartupMode = %v, want OnDemand", shard.StartupMode)
	}
	if shard.config.TickInterval == 0 {
		t.Error("TickInterval should not be zero")
	}
}

func TestDefaultCampaignRunnerConfig_ShouldHaveReasonableDefaults(t *testing.T) {
	cfg := DefaultCampaignRunnerConfig()
	if cfg.TickInterval == 0 {
		t.Error("TickInterval should not be zero")
	}
}

func TestCampaignRunnerShard_SetWorkspaceRoot_ShouldSetWorkspace(t *testing.T) {
	shard := NewCampaignRunnerShard()
	shard.SetWorkspaceRoot("/test/workspace")

	shard.mu.RLock()
	ws := shard.workspace
	shard.mu.RUnlock()

	if ws != "/test/workspace" {
		t.Errorf("workspace = %q, want %q", ws, "/test/workspace")
	}
}

func TestParseCampaignRunnerConsultationConfidence_WhenValid_ShouldParse(t *testing.T) {
	tests := []struct {
		name   string
		advice string
		want   float64
	}{
		{"percentage", "CONFIDENCE: 85%", 0.85},
		{"raw number", "CONFIDENCE: 90", 0.9},
		{"decimal", "CONFIDENCE: 0.75", 0.75},
		{"multiline", "ADVICE: do stuff\nCONFIDENCE: 70\nCAVEATS: none", 0.7},
		{"no confidence", "just advice", 0.7}, // default
		{"lowercase", "confidence: 85%", 0.85},
		{"with spaces", "CONFIDENCE:  95 ", 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCampaignRunnerConsultationConfidence(tt.advice)
			if got != tt.want {
				t.Errorf("parseCampaignRunnerConsultationConfidence(%q) = %v, want %v", tt.advice, got, tt.want)
			}
		})
	}
}

func TestBuildCampaignRunnerConsultationPrompt_ShouldContainQuestion(t *testing.T) {
	prompt := buildCampaignRunnerConsultationPrompt("Is this approach safe?", "We're refactoring auth")
	if !containsAll(prompt, "Is this approach safe?", "We're refactoring auth", "CONSULTATION REQUEST") {
		t.Errorf("prompt missing expected content: %q", prompt)
	}
}

func TestBuildCampaignRunnerConsultationPrompt_WhenNoContext_ShouldExcludeContext(t *testing.T) {
	prompt := buildCampaignRunnerConsultationPrompt("question", "")
	if !containsAll(prompt, "question", "CONSULTATION REQUEST") {
		t.Errorf("prompt missing expected content: %q", prompt)
	}
}

func TestBuildCampaignRunnerConsultationPrompt_WhenNoQuestion_ShouldExcludeQuestion(t *testing.T) {
	prompt := buildCampaignRunnerConsultationPrompt("", "context only")
	if !containsAll(prompt, "CONSULTATION REQUEST", "context only") {
		t.Errorf("prompt missing expected content: %q", prompt)
	}
}

// ─── Mangle Repair ───────────────────────────────────────────────────────────

func TestMangleRepairShard_NewMangleRepairShard_ShouldHaveDefaults(t *testing.T) {
	shard := NewMangleRepairShard()
	if shard.ID != "mangle_repair" {
		t.Errorf("ID = %q, want %q", shard.ID, "mangle_repair")
	}
	if shard.StartupMode != StartupAuto {
		t.Errorf("StartupMode = %v, want Auto", shard.StartupMode)
	}
	if shard.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", shard.maxRetries)
	}
	if shard.preValidator == nil {
		t.Error("preValidator should not be nil")
	}
	if shard.errorClassifier == nil {
		t.Error("errorClassifier should not be nil")
	}
}

func TestMangleRepairShard_SetMaxRetries_WhenValid_ShouldUpdate(t *testing.T) {
	shard := NewMangleRepairShard()
	shard.SetMaxRetries(5)

	shard.mu.RLock()
	retries := shard.maxRetries
	shard.mu.RUnlock()

	if retries != 5 {
		t.Errorf("maxRetries = %d, want 5", retries)
	}
}

func TestMangleRepairShard_SetMaxRetries_WhenInvalid_ShouldNotUpdate(t *testing.T) {
	shard := NewMangleRepairShard()
	shard.SetMaxRetries(0)  // Too low
	shard.SetMaxRetries(11) // Too high

	shard.mu.RLock()
	retries := shard.maxRetries
	shard.mu.RUnlock()

	if retries != 3 {
		t.Errorf("maxRetries = %d, want 3 (unchanged)", retries)
	}
}

func TestMangleRepairShard_Execute_WhenEmptyTask_ShouldReturnReady(t *testing.T) {
	shard := NewMangleRepairShard()
	result, err := shard.Execute(context.Background(), "")
	if err != nil {
		t.Fatalf("Execute('') error = %v", err)
	}
	if !containsAll(result, "MangleRepair ready") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestMangleRepairShard_Execute_WhenSystemStart_ShouldReturnReady(t *testing.T) {
	shard := NewMangleRepairShard()
	result, err := shard.Execute(context.Background(), "system_start")
	if err != nil {
		t.Fatalf("Execute('system_start') error = %v", err)
	}
	if !containsAll(result, "MangleRepair ready") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestMangleRepairShard_ExtractPredicatesFromRule_ShouldExtractNames(t *testing.T) {
	shard := NewMangleRepairShard()

	tests := []struct {
		name string
		rule string
		want []string
	}{
		{
			"simple rule",
			"impacted(X) :- dependency_link(X, Y, _), modified(Y).",
			[]string{"dependency_link", "modified"},
		},
		{
			"fact",
			"next_action(/start).",
			[]string{"next_action"},
		},
		{
			"no builtins",
			"foo(X) :- bar(X), fn:string_contains(X, \"test\").",
			[]string{"bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shard.extractPredicatesFromRule(tt.rule)
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing predicate %q in %v", w, got)
				}
			}
		})
	}
}

func TestMangleRepairShard_CheckSafety_WhenMissingPeriod_ShouldReturnError(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkSafety("foo(X) :- bar(X)")
	found := false
	for _, e := range errors {
		if containsString(e, "missing terminal period") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'missing terminal period' in errors: %v", errors)
	}
}

func TestMangleRepairShard_CheckSafety_WhenHasPeriod_ShouldNotReturnPeriodError(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkSafety("foo(X) :- bar(X).")
	for _, e := range errors {
		if containsString(e, "missing terminal period") {
			t.Errorf("unexpected 'missing terminal period' error")
		}
	}
}

func TestMangleRepairShard_CheckInfiniteLoopRisk_WhenUnconditionalNextAction_ShouldWarn(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkInfiniteLoopRisk("next_action(/system_start).")
	if len(errors) == 0 {
		t.Error("expected infinite loop risk warning for unconditional next_action(/system_start)")
	}
}

func TestMangleRepairShard_CheckInfiniteLoopRisk_WhenConditional_ShouldNotWarn(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkInfiniteLoopRisk("next_action(/fix) :- test_state(/failing), retry_count(N), N < 3.")
	if len(errors) != 0 {
		t.Errorf("unexpected errors for conditional next_action: %v", errors)
	}
}

func TestMangleRepairShard_CheckInfiniteLoopRisk_WhenComment_ShouldSkip(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkInfiniteLoopRisk("# next_action(/system_start).")
	if len(errors) != 0 {
		t.Errorf("unexpected errors for comment: %v", errors)
	}
}

func TestMangleRepairShard_CheckInfiniteLoopRisk_WhenNotNextAction_ShouldSkip(t *testing.T) {
	shard := NewMangleRepairShard()
	errors := shard.checkInfiniteLoopRisk("foo(/bar).")
	if len(errors) != 0 {
		t.Errorf("unexpected errors for non-next_action: %v", errors)
	}
}

func TestMangleRepairShard_ExtractErrorTypes_ShouldDetectDomains(t *testing.T) {
	shard := NewMangleRepairShard()

	tests := []struct {
		name   string
		errors []string
		want   []string
	}{
		{
			"shard error",
			[]string{"undefined predicate: shard_state"},
			[]string{"shard"},
		},
		{
			"campaign error",
			[]string{"undefined predicate: campaign_phase"},
			[]string{"campaign"},
		},
		{
			"tool error",
			[]string{"tool capability not found"},
			[]string{"tool"},
		},
		{
			"routing error",
			[]string{"next_action predicate missing"},
			[]string{"routing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shard.extractErrorTypes(tt.errors)
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing error type %q in %v", w, got)
				}
			}
		})
	}
}

func TestIsIdentChar_ShouldMatchIdentChars(t *testing.T) {
	tests := []struct {
		char byte
		want bool
	}{
		{'a', true}, {'z', true}, {'A', true}, {'Z', true},
		{'0', true}, {'9', true}, {'_', true},
		{' ', false}, {'.', false}, {'(', false}, {')', false},
	}
	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			got := isIdentChar(tt.char)
			if got != tt.want {
				t.Errorf("isIdentChar(%q) = %v, want %v", tt.char, got, tt.want)
			}
		})
	}
}

// mockLLMClient is defined in base_coverage_test.go


// ─── llmClientAdapter (legislator) ──────────────────────────────────────────

func TestLLMClientAdapter_Complete_WhenNilClient_ShouldReturnError(t *testing.T) {
	adapter := &llmClientAdapter{client: nil}
	_, err := adapter.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Error("Complete should return error when client is nil")
	}
}

func TestLLMClientAdapter_Complete_WhenCostBlocked_ShouldReturnError(t *testing.T) {
	g := NewCostGuard()
	g.MaxLLMCallsPerSession = 0

	adapter := &llmClientAdapter{
		client:    &mockLLMClient{response: "ok"},
		costGuard: g,
		shardID:   "test",
	}
	_, err := adapter.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Error("Complete should return error when cost guard blocks")
	}
}

func TestLLMClientAdapter_CompleteWithTools_WhenNilClient_ShouldReturnError(t *testing.T) {
	adapter := &llmClientAdapter{client: nil}
	_, err := adapter.CompleteWithTools(context.Background(), "sys", "user", nil)
	if err == nil {
		t.Error("CompleteWithTools should return error when client is nil")
	}
}

// ─── executiveLLMAdapter ────────────────────────────────────────────────────

func TestExecutiveLLMAdapter_CompleteWithTools_ShouldReturnError(t *testing.T) {
	shard := NewExecutivePolicyShard()
	adapter := &executiveLLMAdapter{shard: shard, ctx: context.Background()}
	_, err := adapter.CompleteWithTools(context.Background(), "sys", "user", nil)
	if err == nil {
		t.Error("CompleteWithTools should return error (not supported)")
	}
}

func TestExecutiveLLMAdapter_CompleteWithSystem_ShouldDelegateToComplete(t *testing.T) {
	shard := NewExecutivePolicyShard()
	shard.LLMClient = &mockLLMClient{response: "test result"}
	adapter := &executiveLLMAdapter{shard: shard, ctx: context.Background()}
	result, err := adapter.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem error = %v", err)
	}
	if result != "test result" {
		t.Errorf("result = %q, want %q", result, "test result")
	}
}
