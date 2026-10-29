// executive.go implements the Executive Policy system shard.
//
// The Executive Policy shard is the core OODA loop decision-maker:
// - Queries active_strategy to determine current operating mode
// - Derives next_action from Mangle policy rules
// - Checks block_commit and other barrier conditions
// - Emits pending_action facts for the Constitution Gate
//
// This shard is AUTO-START and runs continuously. It is LOGIC-PRIMARY,
// using pure Mangle evaluation with LLM only for:
// - Strategy refinement when rules are insufficient
// - Edge case handling via Autopoiesis
package system

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Strategy represents an active execution strategy.
type Strategy struct {
	Name        string // e.g., "tdd_repair_loop", "breadth_first_survey"
	ActivatedAt time.Time
	Context     map[string]string
}

// ActionDecision represents a derived next action.
type ActionDecision struct {
	ID          string
	Action      string
	Target      string
	Payload     map[string]any
	RawFact     types.Fact
	Rationale   string
	DerivedAt   time.Time
	FromRule    string
	Blocked     bool
	BlockReason string
}

// ExecutiveConfig holds configuration for the executive policy shard.
type ExecutiveConfig struct {
	// Performance
	TickInterval time.Duration // How often to evaluate policy (default: 100ms)

	// Behavior
	StrictBarriers             bool // Block all actions when barriers exist (default: true)
	DebugMode                  bool // Emit detailed derivation traces
	LearningCandidateThreshold int  // Repeats required before candidate (default: 3)
}

// DefaultExecutiveConfig returns sensible defaults.
func DefaultExecutiveConfig() ExecutiveConfig {
	return ExecutiveConfig{
		// 2s fallback poll — matches constitution gate. Sub-second ticks
		// stacked with heartbeats were saturating kernel evaluate.
		TickInterval:               2 * time.Second,
		StrictBarriers:             true,
		DebugMode:                  false,
		LearningCandidateThreshold: 3,
	}
}

// ExecutivePolicyShard is the core OODA loop decision-maker.
type ExecutivePolicyShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config ExecutiveConfig

	// State tracking
	activeStrategies []Strategy
	pendingActions   []ActionDecision
	blockedActions   []ActionDecision
	lastDecision     time.Time

	// Metrics
	decisionsCount  int
	blockCount      int
	strategyChanges int

	// Running state
	running bool

	// Autopoiesis tracking
	patternSuccess map[string]int // Track successful action patterns
	patternFailure map[string]int // Track failed action patterns
	learningStore  core.LearningStore
	candidateStore LearningCandidateStore

	// autopoiesisWg tracks the fire-and-forget autopoiesis-proposal
	// goroutines spawned from the heartbeat tick. Execute waits on this
	// before returning so the shard never declares itself terminated
	// while a proposal is still mutating learning/candidate stores.
	autopoiesisWg sync.WaitGroup

	// Mangle feedback loop for validated rule generation
	feedbackLoop          *feedback.FeedbackLoop
	budgetExhaustedLogged bool // Prevents repeated "budget exhausted" warnings

	// predicateSelector retains the JIT predicate selector handed to the
	// feedback loop, so a SetJITConfig that arrives after SetParentKernel
	// (the production order: registration wires the kernel first) can
	// still apply the configured bounds. predicateLimit/predicateVecLimit
	// are the last configured bounds; non-positive means "no config
	// arrived" and takes the JIT defaults.
	predicateSelector *prompt.PredicateSelector
	predicateLimit    int
	predicateVecLimit int

	// Boot guard: prevents action execution until first user interaction
	// This ensures session rehydration doesn't trigger old actions
	bootGuardActive bool
}

// NewExecutivePolicyShard creates a new Executive Policy shard.
func NewExecutivePolicyShard() *ExecutivePolicyShard {
	return NewExecutivePolicyShardWithConfig(DefaultExecutiveConfig())
}

// NewExecutivePolicyShardWithConfig creates an executive shard with custom config.
func NewExecutivePolicyShardWithConfig(cfg ExecutiveConfig) *ExecutivePolicyShard {
	logging.SystemShards("[ExecutivePolicy] Initializing executive policy shard")
	base := NewBaseSystemShard("executive_policy", StartupAuto)

	// Configure permissions - minimal, read-only
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
		types.PermissionCodeGraph,
		types.PermissionAskUser,
	}
	base.Config.Model = types.ModelConfig{} // No LLM by default - pure logic

	logging.SystemShardsDebug("[ExecutivePolicy] Config: tick_interval=%v, strict_barriers=%v",
		cfg.TickInterval, cfg.StrictBarriers)
	return &ExecutivePolicyShard{
		BaseSystemShard:  base,
		config:           cfg,
		activeStrategies: make([]Strategy, 0),
		pendingActions:   make([]ActionDecision, 0),
		blockedActions:   make([]ActionDecision, 0),
		patternSuccess:   make(map[string]int),
		patternFailure:   make(map[string]int),
		feedbackLoop:     feedback.NewFeedbackLoop(feedback.DefaultConfig()),
		bootGuardActive:  true, // Prevent actions until first user interaction
	}
}

// SetParentKernel wires the kernel and configures context-aware predicate selection.
func (e *ExecutivePolicyShard) SetParentKernel(k types.Kernel) {
	e.BaseSystemShard.SetParentKernel(k)
	var rk *core.RealKernel
	if kReal, ok := k.(*core.RealKernel); ok {
		rk = kReal
	} else if ck, ok := k.(*core.CortexKernel); ok {
		rk = ck.GetPrimaryRealKernel()
	}
	if rk != nil {
		if corpus := rk.GetPredicateCorpus(); corpus != nil {
			selector := prompt.NewPredicateSelector(corpus)
			e.applyPredicateLimits(selector)
			e.predicateSelector = selector
			e.feedbackLoop.SetPredicateSelector(selector)
		}
	}
}

// SetJITConfig stores the effective JIT configuration and applies the
// configured predicate bounds to the attached selector. Registration wires
// the kernel (and the selector) before it injects this config, so without
// the re-application a configured jit.predicate_limit would never reach it.
func (e *ExecutivePolicyShard) SetJITConfig(cfg config.JITConfig) {
	e.BaseSystemShard.SetJITConfig(cfg)
	e.predicateLimit = cfg.PredicateLimit
	e.predicateVecLimit = cfg.PredicateVecLimit
	e.applyPredicateLimits(e.predicateSelector)
}

// applyPredicateLimits stamps the configured predicate bounds onto sel. A
// non-positive bound means "no config arrived" (direct construction, or a
// zero RegistryContext) and takes the JIT default — the same bound
// NewPredicateSelector already carries, so the call is idempotent.
func (e *ExecutivePolicyShard) applyPredicateLimits(sel *prompt.PredicateSelector) {
	if sel == nil {
		return
	}
	limit, vec := e.predicateLimit, e.predicateVecLimit
	if limit <= 0 {
		limit = config.DefaultJITConfig().PredicateLimit
	}
	if vec <= 0 {
		vec = config.DefaultJITConfig().PredicateVecLimit
	}
	sel.SetMaxPredicates(limit)
	sel.SetVectorLimit(vec)
}

// SetLearningStore sets the learning store for persistent autopoiesis.
func (e *ExecutivePolicyShard) SetLearningStore(ls core.LearningStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.learningStore = ls
	e.loadLearnedPatterns()
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (e *ExecutivePolicyShard) loadLearnedPatterns() {
	if e.learningStore == nil {
		return
	}

	// Load success patterns.
	successLearnings, err := e.learningStore.LoadByPredicate("executive", "success_pattern")
	if err == nil {
		for _, learning := range successLearnings {
			if len(learning.FactArgs) < 1 {
				continue
			}
			pattern, ok := learning.FactArgs[0].(string)
			if !ok || pattern == "" {
				continue
			}
			count := 5
			if len(learning.FactArgs) >= 2 {
				switch v := learning.FactArgs[1].(type) {
				case int:
					count = v
				case int64:
					count = int(v)
				case float64:
					count = int(v)
				}
			}
			e.patternSuccess[pattern] = count
		}
		logging.SystemShardsDebug("[%s] Loaded %d success patterns", "executive", len(successLearnings))
	} else {
		logging.Get(logging.CategorySystemShards).Warn("[%s] Failed to load success patterns: %v", "executive", err)
	}

	// Load failure patterns.
	failureLearnings, err := e.learningStore.LoadByPredicate("executive", "failure_pattern")
	if err == nil {
		for _, learning := range failureLearnings {
			if len(learning.FactArgs) < 1 {
				continue
			}
			pattern, ok := learning.FactArgs[0].(string)
			if !ok || pattern == "" {
				continue
			}
			count := 3
			if len(learning.FactArgs) >= 3 {
				switch v := learning.FactArgs[2].(type) {
				case int:
					count = v
				case int64:
					count = int(v)
				case float64:
					count = int(v)
				}
			}
			e.patternFailure[pattern] = count
		}
		logging.SystemShardsDebug("[%s] Loaded %d failure patterns", "executive", len(failureLearnings))
	} else {
		logging.Get(logging.CategorySystemShards).Warn("[%s] Failed to load failure patterns: %v", "executive", err)
	}
}

// SetLearningCandidateStore wires a store for learning candidates (optional).
func (e *ExecutivePolicyShard) SetLearningCandidateStore(store LearningCandidateStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.candidateStore = store
}

// ResetValidationBudget resets the FeedbackLoop validation budget.
// This should be called at the start of a new turn or session to allow
// autopoiesis to resume after budget exhaustion.
func (e *ExecutivePolicyShard) ResetValidationBudget() {
	e.feedbackLoop.ResetBudget()
	e.mu.Lock()
	e.budgetExhaustedLogged = false
	e.mu.Unlock()
	logging.SystemShardsDebug("[ExecutivePolicy] Validation budget reset, autopoiesis re-enabled")
}

// DisableBootGuard disables the boot guard, allowing action execution.
// This should be called when the first user message is received to signal
// that the system is ready for normal operation.
func (e *ExecutivePolicyShard) DisableBootGuard() {
	e.mu.Lock()
	wasActive := e.bootGuardActive
	e.bootGuardActive = false
	e.mu.Unlock()
	if wasActive {
		logging.SystemShards("[ExecutivePolicy] Boot guard disabled, action execution enabled")
	}
}

// IsBootGuardActive returns whether the boot guard is currently active.
func (e *ExecutivePolicyShard) IsBootGuardActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.bootGuardActive
}

// trackSuccess records a successful action derivation.
func (e *ExecutivePolicyShard) trackSuccess(pattern string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.patternSuccess[pattern]++
	// Persist if significant
	if e.learningStore != nil && e.patternSuccess[pattern] >= 5 {
		_ = e.learningStore.Save("executive", "success_pattern", []any{pattern, e.patternSuccess[pattern]}, "")
	}
}

// trackFailure records a blocked or failed action.
func (e *ExecutivePolicyShard) trackFailure(pattern string, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.patternFailure[pattern]++
	// Persist if significant
	if e.learningStore != nil && e.patternFailure[pattern] >= 3 {
		_ = e.learningStore.Save("executive", "failure_pattern", []any{pattern, reason, e.patternFailure[pattern]}, "")
	}
}

// recordExecutiveError writes the executive_error fact that tells the rest of
// the system this shard's policy evaluation failed.
//
// The commit used to be `_ = tx.Commit()` at both call sites. A failed commit
// discards the whole buffered batch, so the one fact that records a policy
// evaluation failure vanished — and the log line above it is the only other
// trace, which no Mangle rule and no UI can see. That turned a broken executive
// into a quiet one. The loop must keep running either way, so this logs rather
// than returning.
func (e *ExecutivePolicyShard) recordExecutiveError(cause error) {
	tx := types.NewKernelTx(e.Kernel)
	tx.Assert(types.Fact{
		Predicate: "executive_error",
		Args:      []any{cause.Error(), time.Now().Unix()},
	})
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategorySystemShards).Error(
			"[ExecutivePolicy] executive_error fact for %q was not committed; the failure is invisible to the kernel: %v",
			cause, err)
	}
}

// Execute runs the Executive Policy's continuous decision loop.
// This shard is AUTO-START and runs for the entire session.
func (e *ExecutivePolicyShard) Execute(ctx context.Context, task string) (string, error) {
	logging.SystemShards("[ExecutivePolicy] Starting OODA decision loop")

	// Reset FeedbackLoop validation budget at session start to prevent
	// budget exhaustion from carrying over between sessions
	e.feedbackLoop.ResetBudget()
	e.mu.Lock()
	e.budgetExhaustedLogged = false
	e.mu.Unlock()
	logging.SystemShardsDebug("[ExecutivePolicy] FeedbackLoop validation budget reset at session start")

	e.SetState(types.ShardStateRunning)
	e.mu.Lock()
	e.running = true
	e.StartTime = time.Now()
	e.mu.Unlock()

	defer func() {
		// Wait for any in-flight autopoiesis proposals to finish before
		// transitioning to Completed; this prevents the shard manager
		// from spawning a replacement that races with stale goroutines.
		e.autopoiesisWg.Wait()
		e.SetState(types.ShardStateCompleted)
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
		logging.SystemShards("[ExecutivePolicy] Decision loop terminated")
	}()

	// Initialize kernel if not set
	if e.Kernel == nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Creating new kernel (none attached)")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		e.Kernel = kernel
	}

	// Clear stale intent facts from previous sessions to prevent startup loops (Bug #X: Infinite Loop Prevention)
	// This ensures the OODA loop doesn't process old user_intent facts that may have been
	// left over from persisted kernel state or previous sessions.
	if e.Kernel != nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Clearing stale intent facts from previous sessions")
		tx := types.NewKernelTx(e.Kernel)
		tx.Retract("user_intent")
		tx.Retract("processed_intent")
		tx.Retract("executive_processed_intent")
		tx.Retract("pending_action")
		if err := tx.Commit(); err != nil {
			logging.Get(logging.CategoryKernel).Error("[ExecutivePolicy] stale intent cleanup failed: %v", err)
		}
	}

	// Subscribe to fact events instead of polling
	factCh := e.SubscribeToFacts([]string{"user_intent", "next_action", "delegate_task", "tdd_next_action", "campaign_next_action", "repair_next_action"})
	// 15s heartbeat (was 5s) — live test: multi-shard 5s heartbeats stacked with
	// 14–24s kernel evals. Health rules only need existence of a heartbeat.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// Fallback ticker for when event bus is unavailable (e.g., tests)
	var fallbackTicker *time.Ticker
	if factCh == nil {
		fallbackTicker = time.NewTicker(e.config.TickInterval)
		defer fallbackTicker.Stop()
	}
	var fallbackCh <-chan time.Time
	if fallbackTicker != nil {
		fallbackCh = fallbackTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			logging.SystemShards("[ExecutivePolicy] Context cancelled, shutting down")
			return e.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-e.StopCh:
			logging.SystemShards("[ExecutivePolicy] Stop signal received")
			return e.generateShutdownSummary("stopped"), nil
		case <-factCh:
			// Event-driven: a relevant fact was asserted — evaluate policy
			if err := e.evaluatePolicy(ctx); err != nil {
				logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Policy evaluation error: %v", err)
				e.recordExecutiveError(err)
			}
		case <-fallbackCh:
			// Polling fallback when no event bus available
			if err := e.evaluatePolicy(ctx); err != nil {
				logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Policy evaluation error: %v", err)
				e.recordExecutiveError(err)
			}
		case <-heartbeat.C:
			// Heartbeat + periodic checks (runs every 5s regardless)
			_ = e.EmitHeartbeat()

			// Check for autopoiesis (strategy gaps) - run async to avoid blocking.
			// Tracked via e.autopoiesisWg so Execute can wait for in-flight
			// autopoiesis goroutines to finish before declaring the shard
			// terminated; previously these were fire-and-forget and the
			// shard could exit with goroutines still mutating its state.
			if e.Autopoiesis.ShouldPropose() {
				logging.SystemShardsDebug("[ExecutivePolicy] Triggering async autopoiesis rule proposal")
				e.autopoiesisWg.Add(1)
				go func() {
					defer e.autopoiesisWg.Done()
					// The shard context is the lifetime. A child deadline here
					// would win over the feedback loop's per-call timeout
					// (the shortest deadline wins) and cancel a repair that
					// was still making progress.
					e.handleAutopoiesis(ctx)
				}()
			}
		}
	}
}

// evaluatePolicy runs the core decision-making logic.
func (e *ExecutivePolicyShard) evaluatePolicy(ctx context.Context) error {
	if e.Kernel == nil {
		return nil
	}

	tx := types.NewKernelTx(e.Kernel)

	// 1. Query active strategies
	strategies, err := e.queryActiveStrategies()
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Strategy query failed: %v", err)
		return fmt.Errorf("strategy query failed: %w", err)
	}

	// Track strategy changes
	if !e.strategiesEqual(strategies) {
		logging.SystemShards("[ExecutivePolicy] Strategy change detected, new strategies: %d", len(strategies))
		e.mu.Lock()
		e.activeStrategies = strategies
		e.strategyChanges++
		e.mu.Unlock()

		// Emit strategy change fact
		for _, s := range strategies {
			logging.SystemShardsDebug("[ExecutivePolicy] Strategy activated: %s", s.Name)
			tx.Assert(types.Fact{
				Predicate: "strategy_activated",
				Args:      []any{s.Name, time.Now().Unix()},
			})
		}
	}

	// 2. Check barriers (block_commit, etc.)
	blocked, blockReason := e.checkBarriers()
	if blocked && e.config.StrictBarriers {
		logging.SystemShardsDebug("[ExecutivePolicy] Execution blocked: %s", blockReason)
		tx.Assert(types.Fact{
			Predicate: "executive_blocked",
			Args:      []any{blockReason, time.Now().Unix()},
		})
		return tx.Commit() // Don\'t derive actions when blocked
	}

	// 3. Query next_action
	actions, err := e.queryNextActions()
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Action query failed: %v", err)
		return fmt.Errorf("action query failed: %w", err)
	}

	latestIntent := e.latestUserIntent()

	// Boot guard: prevent action execution until first user interaction
	// This ensures session rehydration doesn't trigger old persisted actions
	e.mu.RLock()
	bootGuardActive := e.bootGuardActive
	e.mu.RUnlock()
	if bootGuardActive && len(actions) > 0 {
		logging.SystemShardsDebug("[ExecutivePolicy] Boot guard active: suppressing %d actions until user interaction", len(actions))
		return nil
	}

	// Every derived action is emitted. A slice here dropped work the kernel
	// had already decided was next; a run that is not progressing stops in
	// the working-context policy, not by a count of actions on one tick.

	// 4. Emit pending_action facts for Constitution Gate
	consumedCurrentIntent := false
	for _, action := range actions {
		if action.Blocked {
			logging.SystemShardsDebug("[ExecutivePolicy] Action blocked: %s (reason: %s)", action.Action, action.BlockReason)
			e.mu.Lock()
			e.blockedActions = append(e.blockedActions, action)
			e.blockCount++
			e.mu.Unlock()

			// Track blocked action pattern (autopoiesis)
			pattern := fmt.Sprintf("blocked:%s", action.Action)
			e.trackFailure(pattern, action.BlockReason)
			continue
		}

		logging.SystemShards("[ExecutivePolicy] Derived action: %s (from rule: %s)", action.Action, action.FromRule)
		// Emit pending_action for constitution gate to check.
		// If the action has no target/task binding, hydrate from the latest user_intent when applicable.
		actionCopy := action
		payload := copyStringAnyMap(action.Payload)
		target := action.Target
		if latestIntent != nil {
			target, payload = e.hydrateActionFromIntent(action.Action, target, payload, latestIntent)
		}
		if latestIntent != nil && latestIntent.ID == "/current_intent" {
			if v, ok := payload["intent_id"]; ok {
				if id, ok := v.(string); ok && id == latestIntent.ID {
					consumedCurrentIntent = true
				}
			}
		}
		actionCopy.Target = target
		actionCopy.Payload = payload
		tx.Assert(types.Fact{
			Predicate: "pending_action",
			Args:      []any{action.ID, action.Action, target, encodeActionPayload(payload), time.Now().Unix()},
		})
		// Consume one-shot next_action facts asserted by shards.
		// Derived next_action from policy are not in EDB, so this is a safe no-op for them.
		tx.RetractExactFact(action.RawFact)

		e.mu.Lock()
		e.pendingActions = append(e.pendingActions, actionCopy)
		e.decisionsCount++
		e.lastDecision = time.Now()
		e.mu.Unlock()

		// Track successful action derivation (autopoiesis)
		pattern := fmt.Sprintf("%s:%s", action.FromRule, action.Action)
		e.trackSuccess(pattern)

		// Emit debug trace if enabled
		if e.config.DebugMode {
			tx.Assert(types.Fact{
				Predicate: "executive_trace",
				Args: []any{
					action.Action,
					action.FromRule,
					action.Rationale,
					time.Now().Unix(),
				},
			})
		}
	}

	if consumedCurrentIntent {
		tx.Assert(types.Fact{
			Predicate: "executive_processed_intent",
			Args:      []any{"/current_intent"},
		})
	}

	return tx.Commit()
}

// GetActiveStrategies returns the currently active strategies.
func (e *ExecutivePolicyShard) GetActiveStrategies() []Strategy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Strategy, len(e.activeStrategies))
	copy(result, e.activeStrategies)
	return result
}

// GetMetrics returns execution metrics.
func (e *ExecutivePolicyShard) GetMetrics() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]int{
		"decisions":        e.decisionsCount,
		"blocked":          e.blockCount,
		"strategy_changes": e.strategyChanges,
	}
}

// RecordActionOutcome records whether a derived action succeeded or failed.
// This enables the executive to learn which action derivations work well.
func (e *ExecutivePolicyShard) RecordActionOutcome(action string, fromRule string, succeeded bool, errorMsg string) {
	pattern := fmt.Sprintf("%s:%s", fromRule, action)

	if succeeded {
		e.trackSuccess(pattern)
	} else {
		e.trackFailure(pattern, errorMsg)
	}

	// Also track strategy-level outcomes
	e.mu.RLock()
	strategies := e.activeStrategies
	e.mu.RUnlock()

	for _, strategy := range strategies {
		strategyPattern := fmt.Sprintf("strategy:%s:%s", strategy.Name, action)
		if succeeded {
			e.trackSuccess(strategyPattern)
		} else {
			e.trackFailure(strategyPattern, errorMsg)
		}
	}
}

// GetLearnedPatterns returns learned patterns for strategy refinement.
// Lists are sorted: the registries are maps, and unsorted lists would
// shuffle from call to call for any consumer that renders them.
func (e *ExecutivePolicyShard) GetLearnedPatterns() map[string][]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string][]string)

	// Successful action patterns
	var successful []string
	for pattern, count := range e.patternSuccess {
		if count >= 3 {
			successful = append(successful, pattern)
		}
	}
	slices.Sort(successful)
	result["successful"] = successful

	// Failed action patterns
	var failed []string
	for pattern, count := range e.patternFailure {
		if count >= 2 {
			failed = append(failed, pattern)
		}
	}
	slices.Sort(failed)
	result["failed"] = failed

	return result
}

// NOTE: executiveAutopoiesisPrompt constant DELETED (Dec 2024)
// Prompt atoms now live in internal/prompt/atoms/system/autopoiesis.yaml
// JIT compilation is required - no fallback.
