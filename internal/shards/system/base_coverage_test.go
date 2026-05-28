// base_coverage_test.go — Coverage tests for base.go, payloads.go, and shared utilities.
package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// ─── CostGuard ───────────────────────────────────────────────────────────────

func TestCostGuard_NewCostGuard_WhenCreated_ShouldHaveDefaults(t *testing.T) {
	g := NewCostGuard()

	if g.MaxLLMCallsPerMinute != 10 {
		t.Errorf("MaxLLMCallsPerMinute = %d, want 10", g.MaxLLMCallsPerMinute)
	}
	if g.MaxLLMCallsPerSession != 100 {
		t.Errorf("MaxLLMCallsPerSession = %d, want 100", g.MaxLLMCallsPerSession)
	}
	if g.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want 5m", g.IdleTimeout)
	}
	if g.CooldownAfterError != time.Second {
		t.Errorf("CooldownAfterError = %v, want 1s", g.CooldownAfterError)
	}
	if g.MaxValidationRetries != 3 {
		t.Errorf("MaxValidationRetries = %d, want 3", g.MaxValidationRetries)
	}
	if g.ValidationBudget != 20 {
		t.Errorf("ValidationBudget = %d, want 20", g.ValidationBudget)
	}
}

func TestCostGuard_CanCall_WhenFresh_ShouldAllow(t *testing.T) {
	g := NewCostGuard()
	can, reason := g.CanCall()
	if !can {
		t.Errorf("CanCall() = false, reason=%q; want true", reason)
	}
	if reason != "" {
		t.Errorf("reason = %q, want empty", reason)
	}
}

func TestCostGuard_CanCall_WhenRateLimitExceeded_ShouldBlock(t *testing.T) {
	g := NewCostGuard()
	g.MaxLLMCallsPerMinute = 2

	g.RecordCall()
	g.RecordCall()

	can, reason := g.CanCall()
	if can {
		t.Error("CanCall() should return false after hitting per-minute limit")
	}
	if reason != "rate limit exceeded (max calls per minute)" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

func TestCostGuard_CanCall_WhenSessionCapExceeded_ShouldBlock(t *testing.T) {
	g := NewCostGuard()
	g.MaxLLMCallsPerSession = 3

	for range 3 {
		g.RecordCall()
	}

	can, reason := g.CanCall()
	if can {
		t.Error("CanCall() should return false after hitting session cap")
	}
	if reason != "session cap exceeded (max calls per session)" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

func TestCostGuard_CanCall_WhenInCooldown_ShouldBlock(t *testing.T) {
	g := NewCostGuard()
	g.CooldownAfterError = 500 * time.Millisecond

	g.RecordError()

	can, reason := g.CanCall()
	if can {
		t.Error("CanCall() should return false during cooldown")
	}
	if reason == "" {
		t.Error("expected non-empty reason during cooldown")
	}
}

func TestCostGuard_RecordCall_WhenCalled_ShouldResetConsecutiveErrors(t *testing.T) {
	g := NewCostGuard()
	g.RecordError()
	g.RecordError()

	// RecordCall resets consecutive errors
	g.RecordCall()

	if g.consecutiveErrs != 0 {
		t.Errorf("consecutiveErrs = %d, want 0 after RecordCall", g.consecutiveErrs)
	}
	if g.callsThisSession != 1 {
		t.Errorf("callsThisSession = %d, want 1", g.callsThisSession)
	}
	if g.callsThisMinute != 1 {
		t.Errorf("callsThisMinute = %d, want 1", g.callsThisMinute)
	}
}

func TestCostGuard_RecordError_WhenMultiple_ShouldExponentialBackoff(t *testing.T) {
	g := NewCostGuard()
	g.CooldownAfterError = 100 * time.Millisecond

	g.RecordError() // 1st: 100ms
	firstCooldown := g.cooldownUntil

	g.RecordError() // 2nd: 200ms
	secondCooldown := g.cooldownUntil

	if !secondCooldown.After(firstCooldown) {
		t.Error("second cooldown should be later than first (exponential backoff)")
	}
}

func TestCostGuard_ResetSession_WhenCalled_ShouldClearSessionCounter(t *testing.T) {
	g := NewCostGuard()
	g.RecordCall()
	g.RecordCall()

	g.ResetSession()

	if g.callsThisSession != 0 {
		t.Errorf("callsThisSession = %d, want 0 after ResetSession", g.callsThisSession)
	}
}

func TestCostGuard_IsIdle_WhenNeverCalled_ShouldReturnFalse(t *testing.T) {
	g := NewCostGuard()
	if g.IsIdle() {
		t.Error("IsIdle() should return false when never called")
	}
}

func TestCostGuard_IsIdle_WhenRecentCall_ShouldReturnFalse(t *testing.T) {
	g := NewCostGuard()
	g.IdleTimeout = 10 * time.Second

	g.RecordCall()
	if g.IsIdle() {
		t.Error("IsIdle() should return false right after a call")
	}
}

func TestCostGuard_CanRetryValidation_WhenBudgetExhausted_ShouldBlock(t *testing.T) {
	g := NewCostGuard()
	g.ValidationBudget = 2

	g.RecordValidationRetry()
	g.RecordValidationRetry()

	can, reason := g.CanRetryValidation()
	if can {
		t.Error("CanRetryValidation should be false after exhausting budget")
	}
	if reason != "session validation budget exhausted" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

func TestCostGuard_CanRetryValidation_WhenBudgetRemaining_ShouldAllow(t *testing.T) {
	g := NewCostGuard()
	g.ValidationBudget = 5

	g.RecordValidationRetry()

	can, reason := g.CanRetryValidation()
	if !can {
		t.Errorf("CanRetryValidation should be true, got reason: %q", reason)
	}
}

func TestCostGuard_ResetValidationBudget_ShouldClearCounter(t *testing.T) {
	g := NewCostGuard()
	g.ValidationBudget = 2

	g.RecordValidationRetry()
	g.RecordValidationRetry()
	g.ResetValidationBudget()

	can, _ := g.CanRetryValidation()
	if !can {
		t.Error("CanRetryValidation should be true after ResetValidationBudget")
	}
}

func TestCostGuard_ValidationStats_ShouldReturnUsedAndBudget(t *testing.T) {
	g := NewCostGuard()
	g.ValidationBudget = 10

	g.RecordValidationRetry()
	g.RecordValidationRetry()
	g.RecordValidationRetry()

	used, budget := g.ValidationStats()
	if used != 3 {
		t.Errorf("used = %d, want 3", used)
	}
	if budget != 10 {
		t.Errorf("budget = %d, want 10", budget)
	}
}

// ─── AutopoiesisLoop ────────────────────────────────────────────────────────

func TestAutopoiesisLoop_NewAutopoiesisLoop_ShouldHaveDefaults(t *testing.T) {
	a := NewAutopoiesisLoop()

	if a.UnhandledThreshold != 3 {
		t.Errorf("UnhandledThreshold = %d, want 3", a.UnhandledThreshold)
	}
	if a.RuleConfidence != 0.8 {
		t.Errorf("RuleConfidence = %f, want 0.8", a.RuleConfidence)
	}
	if len(a.UnhandledCases) != 0 {
		t.Errorf("UnhandledCases len = %d, want 0", len(a.UnhandledCases))
	}
	if len(a.ProposedRules) != 0 {
		t.Errorf("ProposedRules len = %d, want 0", len(a.ProposedRules))
	}
	if len(a.AppliedRules) != 0 {
		t.Errorf("AppliedRules len = %d, want 0", len(a.AppliedRules))
	}
}

func TestAutopoiesisLoop_ShouldPropose_WhenBelowThreshold_ShouldReturnFalse(t *testing.T) {
	a := NewAutopoiesisLoop()
	a.UnhandledThreshold = 3

	a.RecordUnhandled("q1", nil, nil)
	a.RecordUnhandled("q2", nil, nil)

	if a.ShouldPropose() {
		t.Error("ShouldPropose() should be false when below threshold")
	}
}

func TestAutopoiesisLoop_ShouldPropose_WhenAtThreshold_ShouldReturnTrue(t *testing.T) {
	a := NewAutopoiesisLoop()
	a.UnhandledThreshold = 3

	a.RecordUnhandled("q1", nil, nil)
	a.RecordUnhandled("q2", nil, nil)
	a.RecordUnhandled("q3", nil, nil)

	if !a.ShouldPropose() {
		t.Error("ShouldPropose() should be true at threshold")
	}
}

func TestAutopoiesisLoop_GetUnhandledCases_ShouldClearAndReturn(t *testing.T) {
	a := NewAutopoiesisLoop()

	ctx := map[string]string{"key": "val"}
	a.RecordUnhandled("query1", ctx, nil)
	a.RecordUnhandled("query2", nil, nil)

	cases := a.GetUnhandledCases()
	if len(cases) != 2 {
		t.Fatalf("GetUnhandledCases() returned %d cases, want 2", len(cases))
	}
	if cases[0].Query != "query1" {
		t.Errorf("first case query = %q, want %q", cases[0].Query, "query1")
	}
	if cases[0].Context["key"] != "val" {
		t.Errorf("first case context[key] = %q, want %q", cases[0].Context["key"], "val")
	}

	// Should be cleared after get
	remaining := a.GetUnhandledCases()
	if len(remaining) != 0 {
		t.Errorf("after GetUnhandledCases, remaining = %d, want 0", len(remaining))
	}
}

func TestAutopoiesisLoop_RecordProposal_ShouldAppend(t *testing.T) {
	a := NewAutopoiesisLoop()

	rule := ProposedRule{
		MangleCode: "test_rule() :- true.",
		Confidence: 0.9,
		Rationale:  "test rationale",
	}
	a.RecordProposal(rule)

	if len(a.ProposedRules) != 1 {
		t.Fatalf("ProposedRules len = %d, want 1", len(a.ProposedRules))
	}
	if a.ProposedRules[0].MangleCode != "test_rule() :- true." {
		t.Errorf("ProposedRules[0].MangleCode = %q", a.ProposedRules[0].MangleCode)
	}
}

func TestAutopoiesisLoop_RecordApplied_ShouldAppend(t *testing.T) {
	a := NewAutopoiesisLoop()

	a.RecordApplied("applied_rule() :- true.")

	if len(a.AppliedRules) != 1 {
		t.Fatalf("AppliedRules len = %d, want 1", len(a.AppliedRules))
	}
	if a.AppliedRules[0] != "applied_rule() :- true." {
		t.Errorf("AppliedRules[0] = %q", a.AppliedRules[0])
	}
}

// ─── BaseSystemShard ─────────────────────────────────────────────────────────

func TestBaseSystemShard_NewBaseSystemShard_WhenAutoMode_ShouldSetDefaults(t *testing.T) {
	base := NewBaseSystemShard("test_id", StartupAuto)

	if base.ID != "test_id" {
		t.Errorf("ID = %q, want %q", base.ID, "test_id")
	}
	if base.State != types.ShardStateIdle {
		t.Errorf("State = %v, want %v", base.State, types.ShardStateIdle)
	}
	if base.StartupMode != StartupAuto {
		t.Errorf("StartupMode = %v, want %v", base.StartupMode, StartupAuto)
	}
	if base.CostGuard == nil {
		t.Error("CostGuard should not be nil")
	}
	if base.Autopoiesis == nil {
		t.Error("Autopoiesis should not be nil")
	}
	if !base.learningEnabled {
		t.Error("learningEnabled should be true")
	}
}

func TestBaseSystemShard_NewBaseSystemShard_WhenOnDemandMode_ShouldSetMode(t *testing.T) {
	base := NewBaseSystemShard("on_demand_id", StartupOnDemand)

	if base.StartupMode != StartupOnDemand {
		t.Errorf("StartupMode = %v, want %v", base.StartupMode, StartupOnDemand)
	}
}

func TestBaseSystemShard_GetID_ShouldReturnID(t *testing.T) {
	base := NewBaseSystemShard("my_shard", StartupAuto)
	if id := base.GetID(); id != "my_shard" {
		t.Errorf("GetID() = %q, want %q", id, "my_shard")
	}
}

func TestBaseSystemShard_GetSetState_ShouldTrackTransitions(t *testing.T) {
	base := NewBaseSystemShard("state_test", StartupAuto)

	if state := base.GetState(); state != types.ShardStateIdle {
		t.Errorf("initial state = %v, want %v", state, types.ShardStateIdle)
	}

	base.SetState(types.ShardStateRunning)
	if state := base.GetState(); state != types.ShardStateRunning {
		t.Errorf("after SetState(Running) = %v, want %v", state, types.ShardStateRunning)
	}

	base.SetState(types.ShardStateCompleted)
	if state := base.GetState(); state != types.ShardStateCompleted {
		t.Errorf("after SetState(Completed) = %v, want %v", state, types.ShardStateCompleted)
	}
}

func TestBaseSystemShard_GetConfig_ShouldReturnConfig(t *testing.T) {
	base := NewBaseSystemShard("cfg_test", StartupAuto)
	cfg := base.GetConfig()
	if cfg.Name != "cfg_test" {
		t.Errorf("Config.Name = %q, want %q", cfg.Name, "cfg_test")
	}
}

func TestBaseSystemShard_Execute_ShouldReturnError(t *testing.T) {
	base := NewBaseSystemShard("exec_test", StartupAuto)
	result, err := base.Execute(context.Background(), "task")
	if err == nil {
		t.Error("Execute() should return error for base shard")
	}
	if result != "" {
		t.Errorf("Execute() result = %q, want empty", result)
	}
}

func TestBaseSystemShard_GetKernel_WhenNil_ShouldReturnNil(t *testing.T) {
	base := NewBaseSystemShard("kernel_test", StartupAuto)
	if k := base.GetKernel(); k != nil {
		t.Error("GetKernel() should return nil when no kernel attached")
	}
}

func TestBaseSystemShard_Stop_WhenRunning_ShouldTransitionToCompleted(t *testing.T) {
	base := NewBaseSystemShard("stop_test", StartupAuto)
	base.State = types.ShardStateRunning

	err := base.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if base.State != types.ShardStateCompleted {
		t.Errorf("State after Stop = %v, want %v", base.State, types.ShardStateCompleted)
	}
}

func TestBaseSystemShard_Stop_WhenNotRunning_ShouldBeNoop(t *testing.T) {
	base := NewBaseSystemShard("stop_noop_test", StartupAuto)
	base.State = types.ShardStateIdle

	err := base.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if base.State != types.ShardStateIdle {
		t.Errorf("State after Stop(idle) = %v, want %v", base.State, types.ShardStateIdle)
	}
}

func TestBaseSystemShard_SubscribeToFacts_WhenNoKernel_ShouldReturnNil(t *testing.T) {
	base := NewBaseSystemShard("sub_test", StartupAuto)
	ch := base.SubscribeToFacts([]string{"some_pred"})
	if ch != nil {
		t.Error("SubscribeToFacts() should return nil when kernel is nil")
	}
}

func TestBaseSystemShard_SetLLMClient_ShouldStoreClient(t *testing.T) {
	base := NewBaseSystemShard("llm_test", StartupAuto)
	mock := &mockLLMClient{response: "hello"}
	base.SetLLMClient(mock)

	if base.LLMClient == nil {
		t.Error("LLMClient should not be nil after SetLLMClient")
	}
}

func TestBaseSystemShard_SetParentKernel_WhenNil_ShouldNotPanic(t *testing.T) {
	base := NewBaseSystemShard("nil_kernel_test", StartupAuto)
	base.SetParentKernel(nil)
	if base.Kernel != nil {
		t.Error("Kernel should remain nil when SetParentKernel called with nil")
	}
}

func TestBaseSystemShard_SetSessionContext_ShouldStoreContext(t *testing.T) {
	base := NewBaseSystemShard("session_ctx_test", StartupAuto)
	ctx := &types.SessionContext{}
	base.SetSessionContext(ctx)
	if base.Config.SessionContext != ctx {
		t.Error("SessionContext not properly stored")
	}
}

func TestBaseSystemShard_SetVirtualStore_WhenInvalidType_ShouldNotSet(t *testing.T) {
	base := NewBaseSystemShard("vs_test", StartupAuto)
	base.SetVirtualStore("not_a_virtual_store")
	if base.VirtualStore != nil {
		t.Error("VirtualStore should be nil when invalid type passed")
	}
}

func TestBaseSystemShard_SetGlassBox_WhenNil_ShouldSetNil(t *testing.T) {
	base := NewBaseSystemShard("gb_test", StartupAuto)
	base.SetGlassBox(nil)
	if base.GlassBox != nil {
		t.Error("GlassBox should be nil")
	}
}

func TestBaseSystemShard_SetToolEventBus_WhenNil_ShouldSetNil(t *testing.T) {
	base := NewBaseSystemShard("teb_test", StartupAuto)
	base.SetToolEventBus(nil)
	if base.ToolEventBus != nil {
		t.Error("ToolEventBus should be nil")
	}
}

func TestBaseSystemShard_SetToolStore_WhenNil_ShouldSetNil(t *testing.T) {
	base := NewBaseSystemShard("ts_test", StartupAuto)
	base.SetToolStore(nil)
	if base.ToolStore != nil {
		t.Error("ToolStore should be nil")
	}
}

func TestBaseSystemShard_SetPromptAssembler_ShouldStoreAndRetrieve(t *testing.T) {
	base := NewBaseSystemShard("pa_test", StartupAuto)

	base.SetPromptAssembler(nil)
	if base.GetPromptAssembler() != nil {
		t.Error("GetPromptAssembler should return nil when set to nil")
	}

	base.SetPromptAssembler("some_assembler")
	if base.GetPromptAssembler() != "some_assembler" {
		t.Error("GetPromptAssembler should return the set value")
	}
}

func TestBaseSystemShard_SetJITConfig_ShouldStoreConfig(t *testing.T) {
	base := NewBaseSystemShard("jit_test", StartupAuto)
	cfg := config.JITConfig{TraceLLMIO: true}
	base.SetJITConfig(cfg)
	if !base.TraceLLMIOEnabled() {
		t.Error("TraceLLMIOEnabled() should return true after SetJITConfig with TraceLLMIO=true")
	}
}

func TestBaseSystemShard_TraceLLMIOEnabled_WhenDefault_ShouldReturnFalse(t *testing.T) {
	base := NewBaseSystemShard("trace_test", StartupAuto)
	if base.TraceLLMIOEnabled() {
		t.Error("TraceLLMIOEnabled() should return false by default")
	}
}

func TestBaseSystemShard_TryJITPrompt_WhenNoAssembler_ShouldReturnFalse(t *testing.T) {
	base := NewBaseSystemShard("jit_nil_test", StartupAuto)
	prompt, ok := base.TryJITPrompt(context.Background(), "test_type")
	if ok {
		t.Error("TryJITPrompt should return false when no assembler set")
	}
	if prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}
}

func TestBaseSystemShard_TryJITPrompt_WhenWrongType_ShouldReturnFalse(t *testing.T) {
	base := NewBaseSystemShard("jit_wrong_test", StartupAuto)
	base.SetPromptAssembler("not_a_prompt_assembler")

	prompt, ok := base.TryJITPrompt(context.Background(), "test_type")
	if ok {
		t.Error("TryJITPrompt should return false when wrong assembler type")
	}
	if prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}
}

func TestBaseSystemShard_EmitHeartbeat_WhenNoKernel_ShouldReturnNil(t *testing.T) {
	base := NewBaseSystemShard("hb_test", StartupAuto)
	err := base.EmitHeartbeat()
	if err != nil {
		t.Errorf("EmitHeartbeat() error = %v, want nil when no kernel", err)
	}
}

// ─── GuardedLLMCall ──────────────────────────────────────────────────────────

func TestBaseSystemShard_GuardedLLMCall_WhenNoClient_ShouldReturnError(t *testing.T) {
	base := NewBaseSystemShard("guarded_test", StartupAuto)
	_, err := base.GuardedLLMCall(context.Background(), "sys", "user")
	if err == nil {
		t.Error("GuardedLLMCall should return error when no LLM client")
	}
	if err.Error() != "no LLM client configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBaseSystemShard_GuardedLLMCall_WhenCostBlocked_ShouldReturnError(t *testing.T) {
	base := NewBaseSystemShard("guarded_blocked_test", StartupAuto)
	base.SetLLMClient(&mockLLMClient{response: "ok"})
	base.CostGuard.MaxLLMCallsPerSession = 0 // Block all calls

	_, err := base.GuardedLLMCall(context.Background(), "sys", "user")
	if err == nil {
		t.Error("GuardedLLMCall should return error when cost guard blocks")
	}
}

func TestBaseSystemShard_GuardedLLMCall_WhenSuccess_ShouldReturnResult(t *testing.T) {
	base := NewBaseSystemShard("guarded_success_test", StartupAuto)
	base.SetLLMClient(&mockLLMClient{response: "hello world"})

	result, err := base.GuardedLLMCall(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("GuardedLLMCall error = %v", err)
	}
	if result != "hello world" {
		t.Errorf("result = %q, want %q", result, "hello world")
	}
}

func TestBaseSystemShard_GuardedLLMCall_WhenClientError_ShouldRecordError(t *testing.T) {
	base := NewBaseSystemShard("guarded_err_test", StartupAuto)
	base.SetLLMClient(&mockLLMClient{err: errors.New("llm fail")})

	_, err := base.GuardedLLMCall(context.Background(), "sys", "user")
	if err == nil {
		t.Error("GuardedLLMCall should propagate client error")
	}
	// CostGuard should have recorded the error
	if base.CostGuard.consecutiveErrs != 1 {
		t.Errorf("consecutiveErrs = %d, want 1", base.CostGuard.consecutiveErrs)
	}
}

// ─── Learning Infrastructure ─────────────────────────────────────────────────

func TestBaseSystemShard_TrackSuccess_WhenDisabled_ShouldNotTrack(t *testing.T) {
	base := NewBaseSystemShard("learn_disabled_test", StartupAuto)
	base.learningEnabled = false

	base.trackSuccess("pattern1")

	if len(base.patternSuccess) != 0 {
		t.Error("should not track when learning disabled")
	}
}

func TestBaseSystemShard_TrackFailure_WhenDisabled_ShouldNotTrack(t *testing.T) {
	base := NewBaseSystemShard("learn_fail_disabled_test", StartupAuto)
	base.learningEnabled = false

	base.trackFailure("pattern1", "reason1")

	if len(base.patternFailure) != 0 {
		t.Error("should not track when learning disabled")
	}
}

func TestBaseSystemShard_TrackCorrection_WhenDisabled_ShouldNotTrack(t *testing.T) {
	base := NewBaseSystemShard("learn_corr_disabled_test", StartupAuto)
	base.learningEnabled = false

	base.trackCorrection("orig", "corrected")

	if len(base.corrections) != 0 {
		t.Error("should not track when learning disabled")
	}
}

func TestBaseSystemShard_PersistLearning_WhenNoStore_ShouldReturnNil(t *testing.T) {
	base := NewBaseSystemShard("persist_nil_test", StartupAuto)

	err := base.persistLearning()
	if err != nil {
		t.Errorf("persistLearning() error = %v, want nil", err)
	}
}

// ─── truncateForLog ──────────────────────────────────────────────────────────

func TestTruncateForLog_WhenShort_ShouldReturnUnchanged(t *testing.T) {
	result := truncateForLog("short", 10)
	if result != "short" {
		t.Errorf("truncateForLog('short', 10) = %q, want 'short'", result)
	}
}

func TestTruncateForLog_WhenLong_ShouldTruncate(t *testing.T) {
	result := truncateForLog("this is a very long string", 10)
	if result != "this is a ..." {
		t.Errorf("truncateForLog(..., 10) = %q, want 'this is a ...'", result)
	}
}

func TestTruncateForLog_WhenExactLength_ShouldReturnUnchanged(t *testing.T) {
	result := truncateForLog("12345", 5)
	if result != "12345" {
		t.Errorf("truncateForLog('12345', 5) = %q, want '12345'", result)
	}
}

// ─── min helper ──────────────────────────────────────────────────────────────

func TestMin_WhenFirstSmaller_ShouldReturnFirst(t *testing.T) {
	if got := min(3, 7); got != 3 {
		t.Errorf("min(3, 7) = %d, want 3", got)
	}
}

func TestMin_WhenSecondSmaller_ShouldReturnSecond(t *testing.T) {
	if got := min(7, 3); got != 3 {
		t.Errorf("min(7, 3) = %d, want 3", got)
	}
}

func TestMin_WhenEqual_ShouldReturnEither(t *testing.T) {
	if got := min(5, 5); got != 5 {
		t.Errorf("min(5, 5) = %d, want 5", got)
	}
}

// ─── mockLLMClient ───────────────────────────────────────────────────────────

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	ech := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(ech)
		if m.err != nil {
			ech <- m.err
			return
		}
		ch <- m.response
	}()
	return ch, ech
}

func (m *mockLLMClient) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &types.LLMToolResponse{Text: m.response, StopReason: "end_turn"}, nil
}

// ─── Payloads ────────────────────────────────────────────────────────────────

func TestEncodeActionPayload_WhenEmpty_ShouldReturnEmptyJSON(t *testing.T) {
	result := encodeActionPayload(nil)
	if result != "{}" {
		t.Errorf("encodeActionPayload(nil) = %q, want '{}'", result)
	}
}

func TestEncodeActionPayload_WhenNonEmpty_ShouldReturnJSON(t *testing.T) {
	payload := map[string]any{"key": "value"}
	result := encodeActionPayload(payload)
	if result != `{"key":"value"}` {
		t.Errorf("encodeActionPayload = %q", result)
	}
}

func TestDecodeActionPayload_WhenMapInput_ShouldReturnDirectly(t *testing.T) {
	input := map[string]any{"intent_id": "test123", "other": "data"}
	payload, intentID := decodeActionPayload(input)

	if intentID != "test123" {
		t.Errorf("intentID = %q, want %q", intentID, "test123")
	}
	if payload["other"] != "data" {
		t.Errorf("payload[other] = %v, want 'data'", payload["other"])
	}
}

func TestDecodeActionPayload_WhenJSONString_ShouldParse(t *testing.T) {
	input := `{"intent_id": "abc", "key": "val"}`
	payload, intentID := decodeActionPayload(input)

	if intentID != "abc" {
		t.Errorf("intentID = %q, want %q", intentID, "abc")
	}
	if payload["key"] != "val" {
		t.Errorf("payload[key] = %v, want 'val'", payload["key"])
	}
}

func TestDecodeActionPayload_WhenEmptyString_ShouldReturnEmpty(t *testing.T) {
	payload, intentID := decodeActionPayload("")
	if len(payload) != 0 {
		t.Errorf("payload len = %d, want 0", len(payload))
	}
	if intentID != "" {
		t.Errorf("intentID = %q, want empty", intentID)
	}
}

func TestDecodeActionPayload_WhenBracesOnly_ShouldReturnEmpty(t *testing.T) {
	payload, intentID := decodeActionPayload("{}")
	if len(payload) != 0 {
		t.Errorf("payload len = %d, want 0", len(payload))
	}
	if intentID != "" {
		t.Errorf("intentID = %q, want empty", intentID)
	}
}

func TestDecodeActionPayload_WhenPseudoMap_ShouldParse(t *testing.T) {
	input := "map[key:value intent_id:xyz]"
	payload, intentID := decodeActionPayload(input)

	if intentID != "xyz" {
		t.Errorf("intentID = %q, want %q", intentID, "xyz")
	}
	if payload["key"] != "value" {
		t.Errorf("payload[key] = %v, want 'value'", payload["key"])
	}
}

func TestParsePseudoMapPayload_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	result := parsePseudoMapPayload("map[]")
	if len(result) != 0 {
		t.Errorf("parsePseudoMapPayload('map[]') len = %d, want 0", len(result))
	}
}

func TestParsePseudoMapPayload_WhenInvalidField_ShouldSkip(t *testing.T) {
	result := parsePseudoMapPayload("map[nocolon valid:yes]")
	if len(result) != 1 {
		t.Errorf("len = %d, want 1", len(result))
	}
	if result["valid"] != "yes" {
		t.Errorf("result[valid] = %v, want 'yes'", result["valid"])
	}
}

func TestEscalationSubject_WhenEmptyTarget_ShouldReturnActionType(t *testing.T) {
	result := escalationSubject("exec_cmd", "")
	if result != "exec_cmd" {
		t.Errorf("escalationSubject('exec_cmd', '') = %q, want 'exec_cmd'", result)
	}
}

func TestEscalationSubject_WhenWhitespaceTarget_ShouldReturnActionType(t *testing.T) {
	result := escalationSubject("exec_cmd", "   ")
	if result != "exec_cmd" {
		t.Errorf("escalationSubject should return just actionType for whitespace target")
	}
}

func TestEscalationSubject_WhenNonEmptyTarget_ShouldJoin(t *testing.T) {
	result := escalationSubject("exec_cmd", "rm -rf /")
	expected := "exec_cmd:rm -rf /"
	if result != expected {
		t.Errorf("escalationSubject = %q, want %q", result, expected)
	}
}

// ─── normalizePayload / extractIntentIDFromPayloadString ─────────────────────

func TestNormalizePayload_WhenString_ShouldDelegateToDecode(t *testing.T) {
	payload, intentID := normalizePayload(`{"intent_id":"i123"}`)
	if intentID != "i123" {
		t.Errorf("intentID = %q, want %q", intentID, "i123")
	}
	if len(payload) == 0 {
		t.Error("payload should not be empty")
	}
}

func TestExtractIntentIDFromPayloadString_ShouldExtractID(t *testing.T) {
	id := extractIntentIDFromPayloadString(`{"intent_id":"test_id"}`)
	if id != "test_id" {
		t.Errorf("extractIntentIDFromPayloadString = %q, want %q", id, "test_id")
	}
}

func TestExtractIntentIDFromPayloadString_WhenNoID_ShouldReturnEmpty(t *testing.T) {
	id := extractIntentIDFromPayloadString(`{"key":"val"}`)
	if id != "" {
		t.Errorf("extractIntentIDFromPayloadString = %q, want empty", id)
	}
}

// ─── unixSecondsArg ──────────────────────────────────────────────────────────

func TestUnixSecondsArg_WhenInt64_ShouldReturn(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{"a", "b", int64(1234567890)}}
	ts, ok := unixSecondsArg(f, 2)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts != 1234567890 {
		t.Errorf("ts = %d, want 1234567890", ts)
	}
}

func TestUnixSecondsArg_WhenInt_ShouldConvert(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{42}}
	ts, ok := unixSecondsArg(f, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts != 42 {
		t.Errorf("ts = %d, want 42", ts)
	}
}

func TestUnixSecondsArg_WhenFloat64_ShouldConvert(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{float64(999)}}
	ts, ok := unixSecondsArg(f, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts != 999 {
		t.Errorf("ts = %d, want 999", ts)
	}
}

func TestUnixSecondsArg_WhenStringNumber_ShouldParse(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{"12345"}}
	ts, ok := unixSecondsArg(f, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts != 12345 {
		t.Errorf("ts = %d, want 12345", ts)
	}
}

func TestUnixSecondsArg_WhenInvalidString_ShouldFail(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{"not_a_number"}}
	_, ok := unixSecondsArg(f, 0)
	if ok {
		t.Error("expected ok=false for invalid string")
	}
}

func TestUnixSecondsArg_WhenOutOfBounds_ShouldFail(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{"a"}}
	_, ok := unixSecondsArg(f, 5)
	if ok {
		t.Error("expected ok=false for out-of-bounds index")
	}
}

func TestUnixSecondsArg_WhenNegativeIndex_ShouldFail(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{"a"}}
	_, ok := unixSecondsArg(f, -1)
	if ok {
		t.Error("expected ok=false for negative index")
	}
}

func TestUnixSecondsArg_WhenUnknownType_ShouldFail(t *testing.T) {
	f := types.Fact{Predicate: "test", Args: []any{true}}
	_, ok := unixSecondsArg(f, 0)
	if ok {
		t.Error("expected ok=false for bool type")
	}
}

// ─── StartupMode constants ──────────────────────────────────────────────────

func TestStartupModeConstants(t *testing.T) {
	if StartupAuto != 0 {
		t.Errorf("StartupAuto = %d, want 0", StartupAuto)
	}
	if StartupOnDemand != 1 {
		t.Errorf("StartupOnDemand = %d, want 1", StartupOnDemand)
	}
}

// ─── CostGuard minute reset ─────────────────────────────────────────────────

func TestCostGuard_CanCall_WhenMinuteResets_ShouldAllowAgain(t *testing.T) {
	g := NewCostGuard()
	g.MaxLLMCallsPerMinute = 1

	g.RecordCall()

	can, _ := g.CanCall()
	if can {
		t.Error("should be blocked after 1 call")
	}

	// Simulate minute passing
	g.mu.Lock()
	g.lastResetMinute = time.Now().Add(-2 * time.Minute)
	g.mu.Unlock()

	can, _ = g.CanCall()
	if !can {
		t.Error("should be allowed after minute reset")
	}
}

// ─── RecordError max backoff ────────────────────────────────────────────────

func TestCostGuard_RecordError_WhenManyErrors_ShouldCapBackoffAt60s(t *testing.T) {
	g := NewCostGuard()
	g.CooldownAfterError = 1 * time.Second

	// Record many errors to trigger max backoff
	for range 10 {
		g.RecordError()
	}

	// Cooldown should be capped at 60 seconds from now
	g.mu.Lock()
	remaining := time.Until(g.cooldownUntil)
	g.mu.Unlock()

	if remaining > 61*time.Second {
		t.Errorf("backoff = %v, should be capped at 60s", remaining)
	}
}

// ─── CostGuard loadLearnedPatterns edge ─────────────────────────────────────

func TestBaseSystemShard_LoadLearnedPatterns_WhenNoStore_ShouldNotPanic(t *testing.T) {
	base := NewBaseSystemShard("load_nil_test", StartupAuto)
	// loadLearnedPatterns is called during SetLearningStore, but if nil, no-op
	base.loadLearnedPatterns()
	// No panic = success
	if len(base.patternSuccess) != 0 {
		t.Error("should have empty patternSuccess without store")
	}
}

// ─── SetState same state should not log transition ──────────────────────────

func TestBaseSystemShard_SetState_WhenSameState_ShouldBeNoop(t *testing.T) {
	base := NewBaseSystemShard("same_state_test", StartupAuto)
	base.SetState(types.ShardStateIdle) // same as initial
	if base.GetState() != types.ShardStateIdle {
		t.Error("state should remain idle")
	}
}

// ─── decodeActionPayload type switch exhaustive ─────────────────────────────

func TestDecodeActionPayload_WhenIntegerInput_ShouldReturnEmptyPayload(t *testing.T) {
	// Non-string, non-map input — should return empty payload
	payload, intentID := decodeActionPayload(42)
	if len(payload) != 0 {
		t.Errorf("payload len = %d, want 0 for integer input", len(payload))
	}
	if intentID != "" {
		t.Errorf("intentID = %q, want empty for integer input", intentID)
	}
}

func TestDecodeActionPayload_WhenInvalidJSON_ShouldFallThrough(t *testing.T) {
	input := `{this is not valid json}`
	payload, intentID := decodeActionPayload(input)
	// Should not parse as JSON but also not panic
	if intentID != "" {
		t.Errorf("intentID = %q, want empty for invalid json", intentID)
	}
	_ = fmt.Sprintf("payload: %v", payload) // Use payload
}
