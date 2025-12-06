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
	"codenerd/internal/core"
	"context"
	"fmt"
	"sync"
	"time"
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
		lastResetMinute:       time.Now(),
	}
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
	FactsAtTime []core.Fact       // Snapshot of facts when the case occurred
}

// ProposedRule represents an LLM-proposed Mangle rule for autopoiesis.
type ProposedRule struct {
	MangleCode  string    // The proposed Mangle rule
	Confidence  float64   // LLM's confidence in the rule (0-1)
	Rationale   string    // Why this rule was proposed
	BasedOn     []UnhandledCase // Cases that led to this proposal
	ProposedAt  time.Time
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
func (a *AutopoiesisLoop) RecordUnhandled(query string, ctx map[string]string, facts []core.Fact) {
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
	Config core.ShardConfig
	State  core.ShardState

	// Components
	Kernel       *core.RealKernel
	LLMClient    core.LLMClient
	VirtualStore *core.VirtualStore

	// System shard specific
	StartupMode StartupMode
	CostGuard   *CostGuard
	Autopoiesis *AutopoiesisLoop

	// Lifecycle
	StartTime time.Time
	StopCh    chan struct{}
}

// NewBaseSystemShard creates a base system shard with given ID and startup mode.
func NewBaseSystemShard(id string, mode StartupMode) *BaseSystemShard {
	return &BaseSystemShard{
		ID:          id,
		Config:      core.DefaultSystemConfig(id),
		State:       core.ShardStateIdle,
		StartupMode: mode,
		CostGuard:   NewCostGuard(),
		Autopoiesis: NewAutopoiesisLoop(),
		StopCh:      make(chan struct{}),
	}
}

// GetID returns the shard ID.
func (b *BaseSystemShard) GetID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ID
}

// GetState returns the current state.
func (b *BaseSystemShard) GetState() core.ShardState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.State
}

// SetState sets the shard state.
func (b *BaseSystemShard) SetState(state core.ShardState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.State = state
}

// GetConfig returns the shard configuration.
func (b *BaseSystemShard) GetConfig() core.ShardConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Config
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
	if b.State == core.ShardStateRunning {
		close(b.StopCh)
		b.State = core.ShardStateCompleted
	}
	return nil
}

// SetLLMClient sets the LLM client.
func (b *BaseSystemShard) SetLLMClient(client core.LLMClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.LLMClient = client
}

// SetParentKernel sets the Mangle kernel.
func (b *BaseSystemShard) SetParentKernel(k core.Kernel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		b.Kernel = rk
	} else {
		panic("SystemShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store.
func (b *BaseSystemShard) SetVirtualStore(vs *core.VirtualStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.VirtualStore = vs
}

// EmitHeartbeat emits a heartbeat fact to the kernel.
func (b *BaseSystemShard) EmitHeartbeat() error {
	if b.Kernel == nil {
		return nil
	}
	return b.Kernel.Assert(core.Fact{
		Predicate: "system_heartbeat",
		Args:      []interface{}{b.ID, time.Now().Unix()},
	})
}

// GuardedLLMCall wraps an LLM call with cost guard checks.
func (b *BaseSystemShard) GuardedLLMCall(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if b.LLMClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	can, reason := b.CostGuard.CanCall()
	if !can {
		return "", fmt.Errorf("LLM call blocked: %s", reason)
	}

	result, err := b.LLMClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		b.CostGuard.RecordError()
		return "", err
	}

	b.CostGuard.RecordCall()
	return result, nil
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
