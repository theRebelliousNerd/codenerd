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
	"strconv"
	"strings"
	"sync"
	"time"

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
	Payload     map[string]interface{}
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
	StrictBarriers    bool // Block all actions when barriers exist (default: true)
	MaxActionsPerTick int  // Prevent action storms (default: 5)
	DebugMode         bool // Emit detailed derivation traces
	OODATimeout       time.Duration // How long to wait before declaring OODA stalled
}

// DefaultExecutiveConfig returns sensible defaults.
func DefaultExecutiveConfig() ExecutiveConfig {
	return ExecutiveConfig{
		TickInterval:      100 * time.Millisecond,
		StrictBarriers:    true,
		MaxActionsPerTick: 5,
		DebugMode:         false,
		OODATimeout:       30 * time.Second,
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

	// Mangle feedback loop for validated rule generation
	feedbackLoop          *feedback.FeedbackLoop
	budgetExhaustedLogged bool // Prevents repeated "budget exhausted" warnings

	// Boot guard: prevents action execution until first user interaction
	// This ensures session rehydration doesn't trigger old actions
	bootGuardActive bool

	// OODA stall tracking
	lastIntentFingerprint string
	pendingIntentSince    time.Time
	oodaTimeoutEmitted    bool
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

	logging.SystemShardsDebug("[ExecutivePolicy] Config: tick_interval=%v, strict_barriers=%v, max_actions=%d",
		cfg.TickInterval, cfg.StrictBarriers, cfg.MaxActionsPerTick)
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
	if rk, ok := k.(*core.RealKernel); ok {
		if corpus := rk.GetPredicateCorpus(); corpus != nil {
			e.feedbackLoop.SetPredicateSelector(prompt.NewPredicateSelector(corpus))
		}
	}
}

// SetLearningStore sets the learning store for persistent autopoiesis.
func (e *ExecutivePolicyShard) SetLearningStore(ls core.LearningStore) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.learningStore = ls
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
		e.resetOODATimeout()
		logging.SystemShards("[ExecutivePolicy] Boot guard disabled, action execution enabled")
	}
}

// IsBootGuardActive returns whether the boot guard is currently active.
func (e *ExecutivePolicyShard) IsBootGuardActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.bootGuardActive
}

func (e *ExecutivePolicyShard) resetOODATimeout() {
	e.mu.Lock()
	e.resetOODATimeoutLocked()
	e.mu.Unlock()
	if e.Kernel != nil {
		_ = e.Kernel.Retract("ooda_timeout")
	}
}

func (e *ExecutivePolicyShard) resetOODATimeoutLocked() {
	e.pendingIntentSince = time.Time{}
	e.oodaTimeoutEmitted = false
	e.lastIntentFingerprint = ""
}

func (e *ExecutivePolicyShard) hasPendingIntent() bool {
	if e.Kernel == nil {
		return false
	}
	facts, err := e.Kernel.Query("pending_intent")
	return err == nil && len(facts) > 0
}

func (e *ExecutivePolicyShard) intentFingerprint(intent *userIntentSnapshot) string {
	if intent == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s|%s", intent.Category, intent.Verb, intent.Target, intent.Constraint)
}

func (e *ExecutivePolicyShard) updateOODATimeout(intent *userIntentSnapshot, hasActions bool) {
	if e.Kernel == nil {
		return
	}

	pending := e.hasPendingIntent()
	fingerprint := e.intentFingerprint(intent)

	e.mu.Lock()
	bootGuardActive := e.bootGuardActive
	if fingerprint != "" && fingerprint != e.lastIntentFingerprint {
		e.lastIntentFingerprint = fingerprint
		e.pendingIntentSince = time.Now()
		e.oodaTimeoutEmitted = false
	}
	pendingSince := e.pendingIntentSince
	alreadyEmitted := e.oodaTimeoutEmitted
	timeout := e.config.OODATimeout
	e.mu.Unlock()

	if bootGuardActive || !pending || hasActions {
		e.resetOODATimeout()
		return
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if pendingSince.IsZero() {
		e.mu.Lock()
		e.pendingIntentSince = time.Now()
		e.oodaTimeoutEmitted = false
		e.mu.Unlock()
		return
	}

	if !alreadyEmitted && time.Since(pendingSince) >= timeout {
		if err := e.Kernel.Assert(types.Fact{Predicate: "ooda_timeout"}); err == nil {
			e.mu.Lock()
			e.oodaTimeoutEmitted = true
			e.mu.Unlock()
		}
	}
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
		_ = e.Kernel.Retract("user_intent")
		_ = e.Kernel.Retract("processed_intent")
		_ = e.Kernel.Retract("executive_processed_intent")
		_ = e.Kernel.Retract("pending_action")
		_ = e.Kernel.Retract("delegate_task")
	}

	ticker := time.NewTicker(e.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.SystemShards("[ExecutivePolicy] Context cancelled, shutting down")
			return e.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-e.StopCh:
			logging.SystemShards("[ExecutivePolicy] Stop signal received")
			return e.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Core OODA loop: Observe -> Orient -> Decide -> (emit for Act)
			if err := e.evaluatePolicy(ctx); err != nil {
				logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Policy evaluation error: %v", err)
				_ = e.Kernel.Assert(types.Fact{
					Predicate: "executive_error",
					Args:      []interface{}{err.Error(), time.Now().Unix()},
				})
			}

			// Emit heartbeat
			_ = e.EmitHeartbeat()

			// Check for autopoiesis (strategy gaps) - run async to avoid blocking OODA loop
			if e.Autopoiesis.ShouldPropose() {
				logging.SystemShardsDebug("[ExecutivePolicy] Triggering async autopoiesis rule proposal")
				go func() {
					autoCtx, cancel := context.WithTimeout(ctx, 3*time.Minute) // Extended for LLM rule generation
					defer cancel()
					e.handleAutopoiesis(autoCtx)
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
			_ = e.Kernel.Assert(types.Fact{
				Predicate: "strategy_activated",
				Args:      []interface{}{s.Name, time.Now().Unix()},
			})
		}
	}

	// 2. Check barriers (block_commit, etc.)
	blocked, blockReason := e.checkBarriers()
	if blocked && e.config.StrictBarriers {
		logging.SystemShardsDebug("[ExecutivePolicy] Execution blocked: %s", blockReason)
		_ = e.Kernel.Assert(types.Fact{
			Predicate: "executive_blocked",
			Args:      []interface{}{blockReason, time.Now().Unix()},
		})
		return nil // Don't derive actions when blocked
	}

	// 3. Query next_action
	actions, err := e.queryNextActions()
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Action query failed: %v", err)
		return fmt.Errorf("action query failed: %w", err)
	}

	latestIntent := e.latestUserIntent()
	e.updateOODATimeout(latestIntent, len(actions) > 0)

	// Boot guard: prevent action execution until first user interaction
	// This ensures session rehydration doesn't trigger old persisted actions
	e.mu.RLock()
	bootGuardActive := e.bootGuardActive
	e.mu.RUnlock()
	if bootGuardActive && len(actions) > 0 {
		logging.SystemShardsDebug("[ExecutivePolicy] Boot guard active: suppressing %d actions until user interaction", len(actions))
		return nil
	}

	// Limit actions per tick to prevent storms
	if len(actions) > e.config.MaxActionsPerTick {
		logging.Get(logging.CategorySystemShards).Warn("[ExecutivePolicy] Action storm prevention: limiting from %d to %d actions", len(actions), e.config.MaxActionsPerTick)
		actions = actions[:e.config.MaxActionsPerTick]
	}

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
		_ = e.Kernel.Assert(types.Fact{
			Predicate: "pending_action",
			Args:      []interface{}{action.ID, action.Action, target, payload, time.Now().Unix()},
		})
		// Consume one-shot next_action facts asserted by shards.
		// Derived next_action from policy are not in EDB, so this is a safe no-op for them.
		_ = e.Kernel.RetractExactFact(action.RawFact)

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
			_ = e.Kernel.Assert(types.Fact{
				Predicate: "executive_trace",
				Args: []interface{}{
					action.Action,
					action.FromRule,
					action.Rationale,
					time.Now().Unix(),
				},
			})
		}
	}

	if consumedCurrentIntent {
		_ = e.Kernel.Assert(types.Fact{
			Predicate: "executive_processed_intent",
			Args:      []interface{}{"/current_intent"},
		})
	}

	return nil
}

type userIntentSnapshot struct {
	ID         string
	Category   string
	Verb       string
	Target     string
	Constraint string
	Timestamp  int64
}

func (e *ExecutivePolicyShard) latestUserIntent() *userIntentSnapshot {
	if e.Kernel == nil {
		return nil
	}
	facts, err := e.Kernel.Query("user_intent")
	if err != nil {
		return nil
	}

	// Prefer the canonical, stable intent ID when present.
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		id := fmt.Sprintf("%v", f.Args[0])
		if id != "/current_intent" {
			continue
		}
		return &userIntentSnapshot{
			ID:         id,
			Category:   fmt.Sprintf("%v", f.Args[1]),
			Verb:       fmt.Sprintf("%v", f.Args[2]),
			Target:     fmt.Sprintf("%v", f.Args[3]),
			Constraint: fmt.Sprintf("%v", f.Args[4]),
			Timestamp:  time.Now().UnixNano(),
		}
	}

	var best *userIntentSnapshot
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		id := fmt.Sprintf("%v", f.Args[0])
		ts, ok := parseIntentTimestamp(id)
		if !ok {
			continue
		}
		if best == nil || ts > best.Timestamp {
			best = &userIntentSnapshot{
				ID:         id,
				Category:   fmt.Sprintf("%v", f.Args[1]),
				Verb:       fmt.Sprintf("%v", f.Args[2]),
				Target:     fmt.Sprintf("%v", f.Args[3]),
				Constraint: fmt.Sprintf("%v", f.Args[4]),
				Timestamp:  ts,
			}
		}
	}
	return best
}

func parseIntentTimestamp(intentID string) (int64, bool) {
	const prefix = "/intent_"
	if !strings.HasPrefix(intentID, prefix) {
		return 0, false
	}
	ts, err := strconv.ParseInt(strings.TrimPrefix(intentID, prefix), 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}

func (e *ExecutivePolicyShard) loadClarificationPayload(intentID string) (string, []interface{}) {
	if e.Kernel == nil || intentID == "" {
		return "", nil
	}

	question := ""
	if facts, err := e.Kernel.Query("clarification_question"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 2 {
				continue
			}
			if fmt.Sprintf("%v", f.Args[0]) != intentID {
				continue
			}
			if q, ok := f.Args[1].(string); ok {
				question = q
			} else {
				question = fmt.Sprintf("%v", f.Args[1])
			}
			break
		}
	}

	options := make([]interface{}, 0)
	if facts, err := e.Kernel.Query("clarification_option"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 3 {
				continue
			}
			if fmt.Sprintf("%v", f.Args[0]) != intentID {
				continue
			}
			verb := fmt.Sprintf("%v", f.Args[1])
			label := fmt.Sprintf("%v", f.Args[2])
			if label != "" && label != "<nil>" {
				options = append(options, fmt.Sprintf("%s (%s)", label, verb))
			} else {
				options = append(options, verb)
			}
		}
	}

	if question == "" {
		if facts, err := e.Kernel.Query("awaiting_clarification"); err == nil && len(facts) > 0 {
			if len(facts[0].Args) > 0 {
				if q, ok := facts[0].Args[0].(string); ok {
					question = q
				} else {
					question = fmt.Sprintf("%v", facts[0].Args[0])
				}
			}
		}
	}
	if strings.TrimSpace(question) == "" {
		question = "I need a bit more detail to proceed. What would you like me to do?"
	}

	return question, options
}

func copyStringAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (e *ExecutivePolicyShard) hydrateActionFromIntent(actionType string, target string, payload map[string]interface{}, intent *userIntentSnapshot) (string, map[string]interface{}) {
	if intent == nil {
		return target, payload
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	actionAtom := normalizeAtom(actionType)
	intentVerb := normalizeAtom(intent.Verb)
	intentTarget := strings.TrimSpace(intent.Target)
	intentConstraint := strings.TrimSpace(intent.Constraint)

	switch actionAtom {
	case "/interrogative_mode":
		payload["intent_id"] = intent.ID
		question, options := e.loadClarificationPayload(intent.ID)
		if strings.TrimSpace(question) != "" {
			target = question
		}
		if len(options) > 0 {
			payload["options"] = options
		}
		return target, payload
	case "/delegate_reviewer", "/delegate_coder", "/delegate_researcher", "/delegate_tool_generator":
		// For delegation actions, ensure we always supply a usable task string.
		task, _ := payload["task"].(string)
		task = strings.TrimSpace(task)
		if task == "" {
			task = strings.TrimSpace(target)
		}
		if task == "" {
			task = intentTarget
		}
		verb := strings.TrimPrefix(intentVerb, "/")
		if verb == "" {
			verb = "task"
		}
		if intentConstraint != "" && intentConstraint != "none" && intentConstraint != "_" {
			task = fmt.Sprintf("%s %s\nConstraint: %s", verb, task, intentConstraint)
		} else {
			task = fmt.Sprintf("%s %s", verb, task)
		}
		payload["task"] = task
		payload["intent_id"] = intent.ID
		return task, payload
	default:
		// Only hydrate target for actions where intent target is a reliable binding.
		// Avoid contaminating internal/TDD actions (e.g., read_error_log) with unrelated intent targets.
		switch actionAtom {
		case "/read_file", "/write_file", "/edit_file", "/delete_file", "/fs_read", "/fs_write", "/search_files", "/search_code", "/analyze_code":
			payload["intent_id"] = intent.ID
			if intentConstraint != "" && intentConstraint != "none" && intentConstraint != "_" {
				payload["intent_constraint"] = intentConstraint
			}
			if strings.TrimSpace(target) == "" && intentTarget != "" && intentTarget != "none" && intentTarget != "_" {
				return intentTarget, payload
			}
			return target, payload
		default:
			return target, payload
		}
	}
}

// queryActiveStrategies queries for currently active strategies.
func (e *ExecutivePolicyShard) queryActiveStrategies() ([]Strategy, error) {
	results, err := e.Kernel.Query("active_strategy")
	if err != nil {
		return nil, err
	}

	strategies := make([]Strategy, 0, len(results))
	for _, fact := range results {
		if len(fact.Args) < 1 {
			continue
		}
		name, ok := fact.Args[0].(string)
		if !ok {
			continue
		}
		strategies = append(strategies, Strategy{
			Name:        name,
			ActivatedAt: time.Now(),
		})
	}

	return strategies, nil
}

// queryNextActions queries for derived next actions.
func (e *ExecutivePolicyShard) queryNextActions() ([]ActionDecision, error) {
	results, err := e.Kernel.Query("next_action")
	if err != nil {
		// Record as unhandled for autopoiesis
		e.Autopoiesis.RecordUnhandled(
			"next_action",
			map[string]string{"error": err.Error()},
			nil,
		)
		return nil, err
	}

	// Also check for specific strategy-driven actions
	strategyActions := []string{
		"tdd_next_action",
		"campaign_next_action",
		"repair_next_action",
	}

	for _, predicate := range strategyActions {
		additional, err := e.Kernel.Query(predicate)
		if err == nil {
			results = append(results, additional...)
		}
	}

	actions := make([]ActionDecision, 0, len(results))
	for _, fact := range results {
		if len(fact.Args) < 1 {
			continue
		}
		actionName, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		decision := ActionDecision{
			Action:    actionName,
			DerivedAt: time.Now(),
			FromRule:  fact.Predicate,
			Payload:   make(map[string]interface{}),
			RawFact:   fact,
		}

		// Extract target if present
		if len(fact.Args) > 1 {
			decision.Target, _ = fact.Args[1].(string)
		}

		// Extract payload from remaining args
		if len(fact.Args) > 2 {
			for i := 2; i < len(fact.Args); i++ {
				if argMap, ok := fact.Args[i].(map[string]interface{}); ok {
					for k, v := range argMap {
						decision.Payload[k] = v
					}
					continue
				}
				key := fmt.Sprintf("arg%d", i-2)
				decision.Payload[key] = fact.Args[i]
			}
		}

		// Allow shards/policy to pre-seed action IDs via payload
		if rawID, ok := decision.Payload["action_id"]; ok {
			if idStr, ok := rawID.(string); ok && idStr != "" {
				decision.ID = idStr
			}
			delete(decision.Payload, "action_id")
		}
		if decision.ID == "" {
			decision.ID = fmt.Sprintf("action-%d", time.Now().UnixNano())
		}

		actions = append(actions, decision)
	}

	// If no actions derived BUT there is an active user intent, record for autopoiesis.
	// BUG FIX: Only record when there's a genuine gap (user intent exists but no action derived).
	// Recording on every empty tick causes autopoiesis spam at startup when no user has
	// interacted yet, leading to immediate budget exhaustion and wasted LLM calls.
	if len(actions) == 0 && len(results) == 0 {
		// Check if there's an active user intent that we failed to handle
		if intent := e.latestUserIntent(); intent != nil {
			if !e.IsBootGuardActive() {
				reason := "/no_action_derived"
				if unmapped, err := e.Kernel.Query("intent_unmapped"); err == nil && len(unmapped) > 0 {
					reason = "/unmapped_verb"
				}
				_ = e.Kernel.Assert(types.Fact{
					Predicate: "no_action_reason",
					Args:      []interface{}{intent.ID, types.MangleAtom(reason)},
				})
			}
			e.Autopoiesis.RecordUnhandled(
				"next_action",
				map[string]string{"reason": "no_action_derived", "intent_id": intent.ID},
				nil,
			)
		}
		// Otherwise: no user intent and no action is the NORMAL idle state - don't record
	}

	return actions, nil
}

// checkBarriers checks for blocking conditions.
func (e *ExecutivePolicyShard) checkBarriers() (bool, string) {
	barrierPredicates := []string{
		"block_commit",
		"block_action",
		"executive_blocked",
		"test_state_blocking",
	}

	for _, predicate := range barrierPredicates {
		results, err := e.Kernel.Query(predicate)
		if err == nil && len(results) > 0 {
			// Extract reason from first result
			reason := predicate
			if len(results[0].Args) > 0 {
				if r, ok := results[0].Args[0].(string); ok {
					reason = r
				}
			}
			return true, reason
		}
	}

	return false, ""
}

// strategiesEqual checks if current strategies match tracked strategies.
func (e *ExecutivePolicyShard) strategiesEqual(new []Strategy) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(new) != len(e.activeStrategies) {
		return false
	}

	newNames := make(map[string]bool)
	for _, s := range new {
		newNames[s.Name] = true
	}

	for _, s := range e.activeStrategies {
		if !newNames[s.Name] {
			return false
		}
	}

	return true
}

// executiveLLMAdapter wraps the shard's GuardedLLMCall to implement feedback.LLMClient.
type executiveLLMAdapter struct {
	shard *ExecutivePolicyShard
	ctx   context.Context
}

// Complete implements feedback.LLMClient.
func (a *executiveLLMAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.shard.GuardedLLMCall(ctx, systemPrompt, userPrompt)
}

func (a *executiveLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.Complete(ctx, systemPrompt, userPrompt)
}

// CompleteWithTools implements types.LLMClient interface.
// The executive policy shard doesn't use tool-calling directly.
func (a *executiveLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Executive policy uses standard completion, not tool calling
	return nil, fmt.Errorf("executiveLLMAdapter does not support CompleteWithTools")
}

// handleAutopoiesis uses the Mangle FeedbackLoop to propose and validate new policy rules.
func (e *ExecutivePolicyShard) handleAutopoiesis(ctx context.Context) {
	cases := e.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	if e.LLMClient == nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: no LLM client")
		return
	}

	if e.Kernel == nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: no kernel")
		return
	}

	// Check if FeedbackLoop's validation budget is exhausted BEFORE attempting
	// This prevents the infinite warning spam when budget is depleted
	if e.feedbackLoop.IsBudgetExhausted() {
		e.mu.Lock()
		alreadyLogged := e.budgetExhaustedLogged
		if !alreadyLogged {
			e.budgetExhaustedLogged = true
		}
		e.mu.Unlock()

		if !alreadyLogged {
			logging.SystemShards("[ExecutivePolicy] Autopoiesis suspended: FeedbackLoop validation budget exhausted (will resume on budget reset)")
		}
		// BUG FIX: Do NOT re-queue cases when budget is exhausted.
		// Re-queuing causes an infinite loop: cases get re-added to UnhandledCases,
		// ShouldPropose() returns true, handleAutopoiesis is called again, budget is
		// still exhausted, cases get re-queued, repeat forever.
		// Cases are discarded for this session. When budget is reset (on new turn/session),
		// fresh unhandled cases will naturally accumulate if needed.
		return
	}

	can, reason := e.CostGuard.CanCall()
	if !can {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis blocked: %s", reason)
		// Re-queue cases for later processing
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Build the user prompt describing unhandled cases
	userPrompt := e.buildPolicyProposalPrompt(cases)

	canRetry, reason := e.feedbackLoop.CanRetryPrompt(userPrompt)
	if !canRetry {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: FeedbackLoop budget exhausted (%s)", reason)
		return
	}

	// Use JIT prompt compilation (no fallback - atoms in internal/prompt/atoms/system/autopoiesis.yaml)
	systemPrompt, jitUsed := e.TryJITPrompt(ctx, "executive_autopoiesis")
	if !jitUsed || systemPrompt == "" {
		logging.SystemShards("[ExecutivePolicy] [ERROR] JIT compilation failed - skipping autopoiesis (ensure atoms exist)")
		return
	}
	logging.SystemShards("[ExecutivePolicy] [JIT] Using JIT-compiled autopoiesis prompt")

	// Create LLM adapter that wraps GuardedLLMCall
	llmAdapter := &executiveLLMAdapter{
		shard: e,
		ctx:   ctx,
	}

	// Use FeedbackLoop for validated rule generation with automatic retry
	logging.SystemShards("[ExecutivePolicy] Invoking FeedbackLoop for autopoiesis rule generation")
	result, err := e.feedbackLoop.GenerateAndValidate(
		ctx,
		llmAdapter,
		e.Kernel, // RealKernel implements RuleValidator
		systemPrompt,
		userPrompt,
		"executive",
	)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Warn(
			"[ExecutivePolicy] FeedbackLoop failed after %d attempts: %v",
			result.Attempts, err,
		)
		// BUG FIX: Do NOT re-queue cases when budget is exhausted.
		// This prevents the infinite loop where cases are re-added, causing
		// ShouldPropose() to return true again immediately.
		// Only re-queue for transient failures (context cancelled, LLM errors, etc.)
		// that might succeed on a later attempt.
		if strings.Contains(err.Error(), "validation budget exhausted") {
			logging.SystemShardsDebug("[ExecutivePolicy] Dropping %d autopoiesis cases due to budget exhaustion", len(cases))
			return
		}
		// For other errors (transient failures), re-queue for later processing
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// FeedbackLoop validated the rule; extract metadata via parseProposedRule
	// The rule is already loaded by HotLoadRule during validation
	proposedRule := e.parseProposedRule(result.Rule, cases)
	proposedRule.MangleCode = result.Rule // Use the validated (possibly sanitized) rule

	// If parseProposedRule couldn't extract confidence, use a high default since it validated
	if proposedRule.Confidence == 0 {
		proposedRule.Confidence = 0.9 // Validated rules have high implicit confidence
	}

	e.Autopoiesis.RecordProposal(proposedRule)

	// Rule is already loaded via FeedbackLoop's HotLoadRule validation
	if proposedRule.Confidence >= e.Autopoiesis.RuleConfidence {
		e.Autopoiesis.RecordApplied(proposedRule.MangleCode)
		logging.SystemShards("[ExecutivePolicy] Autopoiesis rule applied: %s (confidence: %.2f, attempts: %d, auto-fixed: %v)",
			truncateRule(proposedRule.MangleCode), proposedRule.Confidence, result.Attempts, result.AutoFixed)
	} else {
		// Low confidence rules are recorded but require approval
		if assertErr := e.Kernel.Assert(types.Fact{
			Predicate: "rule_proposal_pending",
			Args: []interface{}{
				"executive_policy",
				proposedRule.MangleCode,
				proposedRule.Rationale,
				proposedRule.Confidence,
				time.Now().Unix(),
			},
		}); assertErr != nil {
			logging.Get(logging.CategorySystemShards).Error(
				"[ExecutivePolicy] Failed to assert rule_proposal_pending: %v", assertErr,
			)
		}
		logging.SystemShards("[ExecutivePolicy] Autopoiesis rule pending approval: confidence %.2f < threshold %.2f",
			proposedRule.Confidence, e.Autopoiesis.RuleConfidence)
	}
}

// truncateRule returns a truncated version of a rule for logging.
func truncateRule(rule string) string {
	const maxLen = 80
	rule = strings.ReplaceAll(rule, "\n", " ")
	if len(rule) > maxLen {
		return rule[:maxLen] + "..."
	}
	return rule
}

// buildPolicyProposalPrompt creates a prompt for policy rule proposals.
func (e *ExecutivePolicyShard) buildPolicyProposalPrompt(cases []UnhandledCase) string {
	var sb strings.Builder
	sb.WriteString("The executive policy could not derive actions for these situations:\n\n")

	for i, cas := range cases {
		sb.WriteString(fmt.Sprintf("%d. Query: %s\n", i+1, cas.Query))
		if cas.Context != nil {
			for k, v := range cas.Context {
				sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
			}
		}
	}

	// Add learned patterns
	e.mu.RLock()
	if len(e.patternSuccess) > 0 {
		sb.WriteString("\nSUCCESSFUL PATTERNS (use as reference):\n")
		for pattern, count := range e.patternSuccess {
			if count >= 3 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}

	if len(e.patternFailure) > 0 {
		sb.WriteString("\nFAILED PATTERNS (avoid these):\n")
		for pattern, count := range e.patternFailure {
			if count >= 2 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}
	e.mu.RUnlock()

	sb.WriteString("\nPropose a Mangle policy rule to handle these cases.\n")
	sb.WriteString("The rule should derive next_action or active_strategy.\n")
	sb.WriteString("Format:\n")
	sb.WriteString("RULE: <mangle code>\n")
	sb.WriteString("CONFIDENCE: <0.0-1.0>\n")
	sb.WriteString("RATIONALE: <explanation>\n")

	return sb.String()
}

// parseProposedRule extracts a proposed rule from LLM output.
func (e *ExecutivePolicyShard) parseProposedRule(output string, cases []UnhandledCase) ProposedRule {
	rule := ProposedRule{
		BasedOn:    cases,
		ProposedAt: time.Now(),
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "RULE:") {
			rule.MangleCode = strings.TrimSpace(strings.TrimPrefix(line, "RULE:"))
		} else if strings.HasPrefix(line, "CONFIDENCE:") {
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			fmt.Sscanf(confStr, "%f", &rule.Confidence)
		} else if strings.HasPrefix(line, "RATIONALE:") {
			rule.Rationale = strings.TrimSpace(strings.TrimPrefix(line, "RATIONALE:"))
		}
	}

	return rule
}

// generateShutdownSummary creates a summary of the shard's activity.
func (e *ExecutivePolicyShard) generateShutdownSummary(reason string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return fmt.Sprintf(
		"Executive Policy shutdown (%s). Decisions: %d, Blocked: %d, Strategy changes: %d, Runtime: %s",
		reason,
		e.decisionsCount,
		e.blockCount,
		e.strategyChanges,
		time.Since(e.StartTime).String(),
	)
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
	result["successful"] = successful

	// Failed action patterns
	var failed []string
	for pattern, count := range e.patternFailure {
		if count >= 2 {
			failed = append(failed, pattern)
		}
	}
	result["failed"] = failed

	return result
}

// NOTE: executiveAutopoiesisPrompt constant DELETED (Dec 2024)
// Prompt atoms now live in internal/prompt/atoms/system/autopoiesis.yaml
// JIT compilation is required - no fallback.
