// Package system implements Type 1 (Permanent) system shards for the codeNERD architecture.
// System shards run continuously in the background, providing core OODA loop functionality:
// - Perception (NL → atoms)
// - Executive (strategy → action)
// - Constitution (safety enforcement)
// - Routing (action → tool)
// - World Model (fact maintenance)
// - Planning (session orchestration)
//
// Unlike Type 2 (ephemeral) shards that spawn → execute → die, system shards
// run continuous loops and propagate facts to the parent kernel.
package system

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
)

// StartupMode determines when a system shard starts.
type StartupMode int

const (
	// StartupAuto starts the shard when the application initializes.
	StartupAuto StartupMode = iota
	// StartupOnDemand starts the shard only when explicitly requested.
	StartupOnDemand
)

// CostGuard provides guardrails to prevent runaway inference costs.
type CostGuard struct {
	mu sync.Mutex

	// Rate limiting
	MaxLLMCallsPerMinute  int           // Max LLM calls per minute (default: 10)
	MaxLLMCallsPerSession int           // Max LLM calls per session (default: 100)
	IdleTimeout           time.Duration // Auto-stop after inactivity
	CooldownAfterError    time.Duration // Backoff on failures

	// Validation budget (for Mangle rule generation retries)
	MaxValidationRetries  int // Max retries per rule (default: 3)
	ValidationBudget      int // Session-wide retry budget (default: 20)
	validationRetriesUsed int

	// Tracking
	callsThisMinute  int
	callsThisSession int
	lastCallTime     time.Time
	lastResetMinute  time.Time
	consecutiveErrs  int
	cooldownUntil    time.Time
}

// NewCostGuard creates a CostGuard with sensible defaults.
func NewCostGuard() *CostGuard {
	return &CostGuard{
		MaxLLMCallsPerMinute:  10,
		MaxLLMCallsPerSession: 100,
		IdleTimeout:           5 * time.Minute,
		CooldownAfterError:    time.Second,
		MaxValidationRetries:  3,
		ValidationBudget:      20,
		lastResetMinute:       time.Now(),
	}
}

// CanRetryValidation checks if another validation retry is allowed.
func (g *CostGuard) CanRetryValidation() (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.validationRetriesUsed >= g.ValidationBudget {
		return false, "session validation budget exhausted"
	}
	return true, ""
}

// RecordValidationRetry records a validation retry attempt.
func (g *CostGuard) RecordValidationRetry() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.validationRetriesUsed++
}

// ResetValidationBudget resets the validation retry counter.
func (g *CostGuard) ResetValidationBudget() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.validationRetriesUsed = 0
}

// ValidationStats returns validation budget statistics.
func (g *CostGuard) ValidationStats() (used, budget int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.validationRetriesUsed, g.ValidationBudget
}

// CanCall checks if an LLM call is allowed under the cost constraints.
func (g *CostGuard) CanCall() (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Check cooldown
	if now.Before(g.cooldownUntil) {
		return false, fmt.Sprintf("in cooldown until %s", g.cooldownUntil.Format(time.RFC3339))
	}

	// Reset minute counter if a minute has passed
	if now.Sub(g.lastResetMinute) >= time.Minute {
		g.callsThisMinute = 0
		g.lastResetMinute = now
	}

	// Check rate limit
	if g.callsThisMinute >= g.MaxLLMCallsPerMinute {
		return false, "rate limit exceeded (max calls per minute)"
	}

	// Check session cap
	if g.callsThisSession >= g.MaxLLMCallsPerSession {
		return false, "session cap exceeded (max calls per session)"
	}

	return true, ""
}

// RecordCall records a successful LLM call.
func (g *CostGuard) RecordCall() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callsThisMinute++
	g.callsThisSession++
	g.lastCallTime = time.Now()
	g.consecutiveErrs = 0
}

// RecordError records a failed LLM call and applies exponential backoff.
func (g *CostGuard) RecordError() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consecutiveErrs++
	// Exponential backoff: 1s, 2s, 4s, 8s, ... max 60s
	backoff := g.CooldownAfterError * time.Duration(1<<min(g.consecutiveErrs-1, 6))
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	g.cooldownUntil = time.Now().Add(backoff)
}

// ResetSession resets the session counter (e.g., on user interaction).
func (g *CostGuard) ResetSession() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callsThisSession = 0
}

// IsIdle checks if the shard has been idle beyond the timeout.
func (g *CostGuard) IsIdle() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastCallTime.IsZero() {
		return false // Never called yet, not idle
	}
	return time.Since(g.lastCallTime) > g.IdleTimeout
}

// UnhandledCase represents a situation where Mangle rules couldn't derive a result.
type UnhandledCase struct {
	Timestamp   time.Time
	Query       string            // The Mangle query that failed
	Context     map[string]string // Relevant context
	FactsAtTime []types.Fact      // Snapshot of facts when the case occurred
}

// ProposedRule represents an LLM-proposed Mangle rule for autopoiesis.
type ProposedRule struct {
	MangleCode string          // The proposed Mangle rule
	Confidence float64         // LLM's confidence in the rule (0-1)
	Rationale  string          // Why this rule was proposed
	BasedOn    []UnhandledCase // Cases that led to this proposal
	ProposedAt time.Time
}

// AutopoiesisLoop tracks unhandled cases and proposes new rules.
type AutopoiesisLoop struct {
	mu sync.Mutex

	// Configuration
	UnhandledThreshold int     // After N unhandled cases, invoke LLM (default: 3)
	RuleConfidence     float64 // Min confidence to auto-apply rule (default: 0.8)

	// State
	UnhandledCases []UnhandledCase
	ProposedRules  []ProposedRule
	AppliedRules   []string // Rules that were auto-applied
}

// NewAutopoiesisLoop creates an AutopoiesisLoop with sensible defaults.
func NewAutopoiesisLoop() *AutopoiesisLoop {
	return &AutopoiesisLoop{
		UnhandledThreshold: 3,
		RuleConfidence:     0.8,
		UnhandledCases:     make([]UnhandledCase, 0),
		ProposedRules:      make([]ProposedRule, 0),
		AppliedRules:       make([]string, 0),
	}
}

// RecordUnhandled records an unhandled case.
func (a *AutopoiesisLoop) RecordUnhandled(query string, ctx map[string]string, facts []types.Fact) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.UnhandledCases = append(a.UnhandledCases, UnhandledCase{
		Timestamp:   time.Now(),
		Query:       query,
		Context:     ctx,
		FactsAtTime: facts,
	})
}

// ShouldPropose checks if enough cases have accumulated to propose a rule.
func (a *AutopoiesisLoop) ShouldPropose() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.UnhandledCases) >= a.UnhandledThreshold
}

// GetUnhandledCases returns and clears the accumulated unhandled cases.
func (a *AutopoiesisLoop) GetUnhandledCases() []UnhandledCase {
	a.mu.Lock()
	defer a.mu.Unlock()
	cases := a.UnhandledCases
	a.UnhandledCases = make([]UnhandledCase, 0)
	return cases
}

// RecordProposal records a proposed rule.
func (a *AutopoiesisLoop) RecordProposal(rule ProposedRule) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ProposedRules = append(a.ProposedRules, rule)
}

// RecordApplied records a rule that was auto-applied.
func (a *AutopoiesisLoop) RecordApplied(mangleCode string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.AppliedRules = append(a.AppliedRules, mangleCode)
}

// BaseSystemShard provides common functionality for all system shards.
type BaseSystemShard struct {
	mu sync.RWMutex

	// Identity
	ID     string
	Config types.ShardConfig
	State  types.ShardState

	// Components
	Kernel       *core.RealKernel
	LLMClient    types.LLMClient
	VirtualStore *core.VirtualStore
	GlassBox     *transparency.GlassBoxEventBus // For Glass Box visibility events
	ToolEventBus *transparency.ToolEventBus     // For always-visible tool execution events
	ToolStore    *store.ToolStore               // For persisting full tool execution results

	// JIT prompt assembly (Phase 5)
	// Stored as interface{} to avoid import cycles - should be *articulation.PromptAssembler.
	// Set via SetPromptAssembler() which accepts interface{}.
	promptAssembler interface{}
	jitConfig       config.JITConfig

	// System shard specific
	StartupMode StartupMode
	CostGuard   *CostGuard
	Autopoiesis *AutopoiesisLoop

	// Learning infrastructure for autopoiesis
	learningStore   types.LearningStore
	patternSuccess  map[string]int // Track successful patterns
	patternFailure  map[string]int // Track failed patterns
	corrections     map[string]int // Track user corrections
	learningEnabled bool

	// Lifecycle
	StartTime time.Time
	StopCh    chan struct{}
}

// NewBaseSystemShard creates a base system shard with given ID and startup mode.
func NewBaseSystemShard(id string, mode StartupMode) *BaseSystemShard {
	modeStr := "auto"
	if mode == StartupOnDemand {
		modeStr = "on-demand"
	}
	logging.SystemShards("[BaseSystemShard] Creating shard: id=%s, mode=%s", id, modeStr)

	return &BaseSystemShard{
		ID:              id,
		Config:          coreshards.DefaultSystemConfig(id),
		State:           types.ShardStateIdle,
		StartupMode:     mode,
		CostGuard:       NewCostGuard(),
		Autopoiesis:     NewAutopoiesisLoop(),
		patternSuccess:  make(map[string]int),
		patternFailure:  make(map[string]int),
		corrections:     make(map[string]int),
		learningEnabled: true,
		StopCh:          make(chan struct{}),
	}
}

// GetID returns the shard ID.
func (b *BaseSystemShard) GetID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ID
}

// GetState returns the current state.
func (b *BaseSystemShard) GetState() types.ShardState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.State
}

// SetState sets the shard state.
func (b *BaseSystemShard) SetState(state types.ShardState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	oldState := b.State
	b.State = state
	if oldState != state {
		logging.SystemShardsDebug("[%s] State transition: %v -> %v", b.ID, oldState, state)
	}
}

// GetConfig returns the shard configuration.
func (b *BaseSystemShard) GetConfig() types.ShardConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Config
}

// Execute returns an error for the base shard; concrete shards must override.
func (b *BaseSystemShard) Execute(ctx context.Context, task string) (string, error) {
	_ = ctx
	_ = task
	return "", fmt.Errorf("execute not implemented for base system shard %s", b.ID)
}

// GetKernel returns the shard's kernel for fact propagation.
func (b *BaseSystemShard) GetKernel() *core.RealKernel {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Kernel
}

// Stop signals the shard to stop.
func (b *BaseSystemShard) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.State == types.ShardStateRunning {
		logging.SystemShards("[%s] Stopping shard (was running for %v)", b.ID, time.Since(b.StartTime))
		close(b.StopCh)
		b.State = types.ShardStateCompleted
	}
	return nil
}

// SetLLMClient sets the LLM client.
func (b *BaseSystemShard) SetLLMClient(client types.LLMClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.LLMClient = client
}

// SetParentKernel sets the Mangle kernel.
func (b *BaseSystemShard) SetParentKernel(k types.Kernel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		b.Kernel = rk
		logging.SystemShardsDebug("[%s] Parent kernel attached", b.ID)
	} else {
		logging.Get(logging.CategorySystemShards).Error("[%s] Invalid kernel type, requires *core.RealKernel", b.ID)
		panic("SystemShard requires *core.RealKernel")
	}
}

// SetSessionContext sets the session context (for dream mode, etc.).
func (b *BaseSystemShard) SetSessionContext(ctx *types.SessionContext) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Config.SessionContext = ctx
}

// SetVirtualStore sets the virtual store.
func (b *BaseSystemShard) SetVirtualStore(vs any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if v, ok := vs.(*core.VirtualStore); ok {
		b.VirtualStore = v
		logging.SystemShardsDebug("[%s] VirtualStore attached", b.ID)
	} else {
		logging.Get(logging.CategorySystemShards).Error("[%s] Invalid VirtualStore type", b.ID)
	}
}

// SetGlassBox sets the Glass Box event bus for visibility events.
func (b *BaseSystemShard) SetGlassBox(bus *transparency.GlassBoxEventBus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.GlassBox = bus
	if bus != nil {
		logging.SystemShardsDebug("[%s] GlassBox event bus attached", b.ID)
	}
}

// SetToolEventBus sets the tool event bus for always-visible tool execution events.
func (b *BaseSystemShard) SetToolEventBus(bus *transparency.ToolEventBus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ToolEventBus = bus
	if bus != nil {
		logging.SystemShardsDebug("[%s] ToolEventBus attached", b.ID)
	}
}

// SetToolStore sets the tool store for persisting full tool execution results.
func (b *BaseSystemShard) SetToolStore(ts *store.ToolStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ToolStore = ts
	if ts != nil {
		logging.SystemShardsDebug("[%s] ToolStore attached", b.ID)
	}
}

// truncateForLog limits string length for logging
func truncateForLog(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

// SetPromptAssembler sets the prompt assembler for JIT compilation.
// The assembler should be *articulation.PromptAssembler but is stored as interface{}
// to avoid import cycles.
func (b *BaseSystemShard) SetPromptAssembler(assembler interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.promptAssembler = assembler
	if assembler != nil {
		logging.SystemShardsDebug("[%s] PromptAssembler attached", b.ID)
	}
}

// SetJITConfig stores the effective JIT configuration for debug/trace gating.
func (b *BaseSystemShard) SetJITConfig(cfg config.JITConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.jitConfig = cfg
}

// TraceLLMIOEnabled returns true when raw prompt/response tracing is enabled.
func (b *BaseSystemShard) TraceLLMIOEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.jitConfig.TraceLLMIO
}

// GetPromptAssembler returns the prompt assembler if set.
func (b *BaseSystemShard) GetPromptAssembler() interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.promptAssembler
}

// TryJITPrompt attempts to assemble a system prompt using the JIT compiler.
// Returns the assembled prompt and true if JIT succeeded, or empty string and false if JIT
// is not available or failed, in which case the caller should fall back to the legacy prompt.
func (b *BaseSystemShard) TryJITPrompt(ctx context.Context, shardType string) (string, bool) {
	b.mu.RLock()
	pa := b.promptAssembler
	shardID := b.ID
	b.mu.RUnlock()

	if pa == nil {
		return "", false
	}

	// Type assert to actual PromptAssembler type
	assembler, ok := pa.(*articulation.PromptAssembler)
	if !ok {
		return "", false
	}

	if !assembler.JITReady() {
		return "", false
	}

	// Build proper PromptContext
	promptCtx := &articulation.PromptContext{
		ShardID:    shardID,
		ShardType:  shardType,
		SessionCtx: b.Config.SessionContext,
	}

	prompt, err := assembler.AssembleSystemPrompt(ctx, promptCtx)
	if err != nil {
		logging.SystemShards("[%s] JIT prompt assembly failed: %v", shardID, err)
		return "", false
	}

	logging.SystemShards("[%s] [JIT] Prompt assembled successfully (%d bytes)", shardID, len(prompt))
	return prompt, true
}

// SetLearningStore sets the learning store and loads existing patterns.
func (b *BaseSystemShard) SetLearningStore(ls core.LearningStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.learningStore = ls
	logging.SystemShardsDebug("[%s] Learning store attached, loading patterns...", b.ID)
	// Load existing patterns from store
	b.loadLearnedPatterns()
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (b *BaseSystemShard) loadLearnedPatterns() {
	if b.learningStore == nil {
		return
	}

	timer := logging.StartTimer(logging.CategorySystemShards, fmt.Sprintf("[%s] Loading learned patterns", b.ID))
	defer timer.Stop()

	// Load success patterns
	successLearnings, err := b.learningStore.LoadByPredicate(b.ID, "success_pattern")
	if err == nil {
		for _, learning := range successLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count to avoid re-learning
				b.patternSuccess[pattern] = 3
			}
		}
		logging.SystemShardsDebug("[%s] Loaded %d success patterns", b.ID, len(successLearnings))
	} else {
		logging.Get(logging.CategorySystemShards).Warn("[%s] Failed to load success patterns: %v", b.ID, err)
	}

	// Load failure patterns
	failureLearnings, err := b.learningStore.LoadByPredicate(b.ID, "failure_pattern")
	if err == nil {
		for _, learning := range failureLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				b.patternFailure[pattern] = 3
			}
		}
		logging.SystemShardsDebug("[%s] Loaded %d failure patterns", b.ID, len(failureLearnings))
	} else {
		logging.Get(logging.CategorySystemShards).Warn("[%s] Failed to load failure patterns: %v", b.ID, err)
	}

	// Load correction patterns
	correctionLearnings, err := b.learningStore.LoadByPredicate(b.ID, "correction_pattern")
	if err == nil {
		for _, learning := range correctionLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				b.corrections[pattern] = 3
			}
		}
		logging.SystemShardsDebug("[%s] Loaded %d correction patterns", b.ID, len(correctionLearnings))
	} else {
		logging.Get(logging.CategorySystemShards).Warn("[%s] Failed to load correction patterns: %v", b.ID, err)
	}
}

// trackSuccess tracks a successful pattern for autopoiesis.
func (b *BaseSystemShard) trackSuccess(pattern string) {
	if !b.learningEnabled {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.patternSuccess[pattern]++

	// Persist to LearningStore if count exceeds threshold
	if b.learningStore != nil && b.patternSuccess[pattern] >= 3 {
		_ = b.learningStore.Save(b.ID, "success_pattern", []any{pattern}, "")
	}
}

// trackFailure tracks a failed pattern for autopoiesis.
func (b *BaseSystemShard) trackFailure(pattern string, reason string) {
	if !b.learningEnabled {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	key := fmt.Sprintf("%s:%s", pattern, reason)
	b.patternFailure[key]++

	// Persist to LearningStore if count exceeds threshold
	if b.learningStore != nil && b.patternFailure[key] >= 2 {
		_ = b.learningStore.Save(b.ID, "failure_pattern", []any{pattern, reason}, "")
	}
}

// trackCorrection tracks a user correction for autopoiesis.
func (b *BaseSystemShard) trackCorrection(original, corrected string) {
	if !b.learningEnabled {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	key := fmt.Sprintf("%s→%s", original, corrected)
	b.corrections[key]++

	// Persist to LearningStore if count exceeds threshold
	if b.learningStore != nil && b.corrections[key] >= 2 {
		_ = b.learningStore.Save(b.ID, "correction_pattern", []any{original, corrected}, "")
	}
}

// persistLearning forces immediate persistence of current learning state.
func (b *BaseSystemShard) persistLearning() error {
	if b.learningStore == nil {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Persist all patterns above threshold
	for pattern, count := range b.patternSuccess {
		if count >= 3 {
			if err := b.learningStore.Save(b.ID, "success_pattern", []any{pattern}, ""); err != nil {
				return err
			}
		}
	}

	for pattern, count := range b.patternFailure {
		if count >= 2 {
			if err := b.learningStore.Save(b.ID, "failure_pattern", []any{pattern}, ""); err != nil {
				return err
			}
		}
	}

	for pattern, count := range b.corrections {
		if count >= 2 {
			if err := b.learningStore.Save(b.ID, "correction_pattern", []any{pattern}, ""); err != nil {
				return err
			}
		}
	}

	return nil
}

// EmitHeartbeat emits a heartbeat fact to the kernel.
func (b *BaseSystemShard) EmitHeartbeat() error {
	if b.Kernel == nil {
		return nil
	}
	return b.Kernel.Assert(types.Fact{
		Predicate: "system_heartbeat",
		Args:      []interface{}{b.ID, time.Now().Unix()},
	})
}

// GuardedLLMCall wraps an LLM call with cost guard checks and per-call timeout.
func (b *BaseSystemShard) GuardedLLMCall(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if b.LLMClient == nil {
		err := fmt.Errorf("no LLM client configured")
		logging.Get(logging.CategorySystemShards).Error("[%s] GuardedLLMCall failed: %v", b.ID, err)
		return "", err
	}

	can, reason := b.CostGuard.CanCall()
	if !can {
		err := fmt.Errorf("LLM call blocked: %s", reason)
		logging.Get(logging.CategorySystemShards).Warn("[%s] GuardedLLMCall blocked: %s", b.ID, reason)
		return "", err
	}

	// Apply per-call timeout if none exists
	// Note: Uses centralized timeout config to avoid conflicts with HTTP client timeouts
	timeouts := config.GetLLMTimeouts()
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeouts.PerCallTimeout)
		defer cancel()
	}

	timer := logging.StartTimer(logging.CategorySystemShards, fmt.Sprintf("[%s] LLM call", b.ID))
	result, err := b.LLMClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	elapsed := timer.Stop()

	if err != nil {
		// Log timeout specifically for debugging
		if errors.Is(err, context.DeadlineExceeded) {
			logging.Get(logging.CategorySystemShards).Warn("[%s] LLM call timed out after %v", b.ID, elapsed)
		}
		b.CostGuard.RecordError()
		logging.Get(logging.CategorySystemShards).Error("[%s] LLM call failed after %v: %v", b.ID, elapsed, err)
		return "", err
	}

	b.CostGuard.RecordCall()
	logging.SystemShardsDebug("[%s] LLM call succeeded in %v, response_len=%d", b.ID, elapsed, len(result))
	return result, nil
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
