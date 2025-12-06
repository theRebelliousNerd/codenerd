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
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Strategy represents an active execution strategy.
type Strategy struct {
	Name        string    // e.g., "tdd_repair_loop", "breadth_first_survey"
	ActivatedAt time.Time
	Context     map[string]string
}

// ActionDecision represents a derived next action.
type ActionDecision struct {
	Action     string
	Target     string
	Rationale  string
	DerivedAt  time.Time
	FromRule   string
	Blocked    bool
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
}

// DefaultExecutiveConfig returns sensible defaults.
func DefaultExecutiveConfig() ExecutiveConfig {
	return ExecutiveConfig{
		TickInterval:      100 * time.Millisecond,
		StrictBarriers:    true,
		MaxActionsPerTick: 5,
		DebugMode:         false,
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
}

// NewExecutivePolicyShard creates a new Executive Policy shard.
func NewExecutivePolicyShard() *ExecutivePolicyShard {
	return NewExecutivePolicyShardWithConfig(DefaultExecutiveConfig())
}

// NewExecutivePolicyShardWithConfig creates an executive shard with custom config.
func NewExecutivePolicyShardWithConfig(cfg ExecutiveConfig) *ExecutivePolicyShard {
	base := NewBaseSystemShard("executive_policy", StartupAuto)

	// Configure permissions - minimal, read-only
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionReadFile,
		core.PermissionCodeGraph,
		core.PermissionAskUser,
	}
	base.Config.Model = core.ModelConfig{} // No LLM by default - pure logic

	return &ExecutivePolicyShard{
		BaseSystemShard:  base,
		config:           cfg,
		activeStrategies: make([]Strategy, 0),
		pendingActions:   make([]ActionDecision, 0),
		blockedActions:   make([]ActionDecision, 0),
	}
}

// Execute runs the Executive Policy's continuous decision loop.
// This shard is AUTO-START and runs for the entire session.
func (e *ExecutivePolicyShard) Execute(ctx context.Context, task string) (string, error) {
	e.SetState(core.ShardStateRunning)
	e.mu.Lock()
	e.running = true
	e.StartTime = time.Now()
	e.mu.Unlock()

	defer func() {
		e.SetState(core.ShardStateCompleted)
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	// Initialize kernel if not set
	if e.Kernel == nil {
		e.Kernel = core.NewRealKernel()
	}

	ticker := time.NewTicker(e.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return e.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-e.StopCh:
			return e.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Core OODA loop: Observe -> Orient -> Decide -> (emit for Act)
			if err := e.evaluatePolicy(ctx); err != nil {
				// Log error but continue
				_ = e.Kernel.Assert(core.Fact{
					Predicate: "executive_error",
					Args:      []interface{}{err.Error(), time.Now().Unix()},
				})
			}

			// Emit heartbeat
			_ = e.EmitHeartbeat()

			// Check for autopoiesis (strategy gaps)
			if e.Autopoiesis.ShouldPropose() {
				e.handleAutopoiesis(ctx)
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
		return fmt.Errorf("strategy query failed: %w", err)
	}

	// Track strategy changes
	if !e.strategiesEqual(strategies) {
		e.mu.Lock()
		e.activeStrategies = strategies
		e.strategyChanges++
		e.mu.Unlock()

		// Emit strategy change fact
		for _, s := range strategies {
			_ = e.Kernel.Assert(core.Fact{
				Predicate: "strategy_activated",
				Args:      []interface{}{s.Name, time.Now().Unix()},
			})
		}
	}

	// 2. Check barriers (block_commit, etc.)
	blocked, blockReason := e.checkBarriers()
	if blocked && e.config.StrictBarriers {
		_ = e.Kernel.Assert(core.Fact{
			Predicate: "execution_blocked",
			Args:      []interface{}{blockReason, time.Now().Unix()},
		})
		return nil // Don't derive actions when blocked
	}

	// 3. Query next_action
	actions, err := e.queryNextActions()
	if err != nil {
		return fmt.Errorf("action query failed: %w", err)
	}

	// Limit actions per tick to prevent storms
	if len(actions) > e.config.MaxActionsPerTick {
		actions = actions[:e.config.MaxActionsPerTick]
	}

	// 4. Emit pending_action facts for Constitution Gate
	for _, action := range actions {
		if action.Blocked {
			e.mu.Lock()
			e.blockedActions = append(e.blockedActions, action)
			e.blockCount++
			e.mu.Unlock()

			// Track blocked action pattern (autopoiesis)
			pattern := fmt.Sprintf("blocked:%s", action.Action)
			e.trackFailure(pattern, action.BlockReason)
			continue
		}

		// Emit pending_action for constitution gate to check
		_ = e.Kernel.Assert(core.Fact{
			Predicate: "pending_action",
			Args:      []interface{}{action.Action, action.Target, time.Now().Unix()},
		})

		e.mu.Lock()
		e.pendingActions = append(e.pendingActions, action)
		e.decisionsCount++
		e.lastDecision = time.Now()
		e.mu.Unlock()

		// Track successful action derivation (autopoiesis)
		pattern := fmt.Sprintf("%s:%s", action.FromRule, action.Action)
		e.trackSuccess(pattern)

		// Emit debug trace if enabled
		if e.config.DebugMode {
			_ = e.Kernel.Assert(core.Fact{
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

	return nil
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
		}

		// Extract target if present
		if len(fact.Args) > 1 {
			decision.Target, _ = fact.Args[1].(string)
		}

		actions = append(actions, decision)
	}

	// If no actions derived, record for autopoiesis
	if len(actions) == 0 && len(results) == 0 {
		e.Autopoiesis.RecordUnhandled(
			"next_action",
			map[string]string{"reason": "no_action_derived"},
			nil,
		)
	}

	return actions, nil
}

// checkBarriers checks for blocking conditions.
func (e *ExecutivePolicyShard) checkBarriers() (bool, string) {
	barrierPredicates := []string{
		"block_commit",
		"block_action",
		"execution_blocked",
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

// handleAutopoiesis uses LLM to propose new policy rules.
func (e *ExecutivePolicyShard) handleAutopoiesis(ctx context.Context) {
	cases := e.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	if e.LLMClient == nil {
		return
	}

	can, _ := e.CostGuard.CanCall()
	if !can {
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	prompt := e.buildPolicyProposalPrompt(cases)

	result, err := e.GuardedLLMCall(ctx, executiveAutopoiesisPrompt, prompt)
	if err != nil {
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Parse and apply proposed rule
	proposedRule := e.parseProposedRule(result, cases)
	if proposedRule.MangleCode == "" {
		return
	}

	e.Autopoiesis.RecordProposal(proposedRule)

	if proposedRule.Confidence >= e.Autopoiesis.RuleConfidence {
		if err := e.Kernel.HotLoadRule(proposedRule.MangleCode); err == nil {
			e.Autopoiesis.RecordApplied(proposedRule.MangleCode)
		}
	} else {
		_ = e.Kernel.Assert(core.Fact{
			Predicate: "rule_proposal_pending",
			Args: []interface{}{
				"executive_policy",
				proposedRule.MangleCode,
				proposedRule.Rationale,
				proposedRule.Confidence,
				time.Now().Unix(),
			},
		})
	}
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

// executiveAutopoiesisPrompt is the system prompt for proposing policy rules.
const executiveAutopoiesisPrompt = `You are the Executive Policy's Autopoiesis system.
Your role is to propose new Mangle policy rules for decision-making.

Available patterns:
- next_action(ActionName) :- <conditions>.
- active_strategy(StrategyName) :- <conditions>.
- block_commit(Reason) :- <conditions>.

Current strategies the system uses:
- /tdd_repair_loop: Fix failing tests
- /breadth_first_survey: Explore codebase
- /depth_first_implement: Focused implementation
- /review_and_refactor: Code quality

When proposing rules:
1. Derive actions based on current state facts
2. Use appropriate strategy for the situation
3. Include necessary guard conditions
4. Keep rules specific and testable

DO NOT propose rules that:
- Bypass safety barriers
- Create infinite action loops
- Ignore test/build failures
- Skip required human confirmation`
