// constitution.go implements the Constitution Gate system shard.
// This is the SAFETY-CRITICAL component of the codeNERD architecture.
//
// The Constitution Gate enforces:
// - permitted(Action) must be positively derived before any action executes
// - dangerous_action patterns are blocked unless admin_override exists
// - network_policy restricts domains to allowlist
// - security_violation facts are emitted for audit trail
//
// This shard is AUTO-START and runs continuously. It is LOGIC-PRIMARY,
// meaning it primarily uses deterministic Mangle rules with LLM only for:
// - Proposing new safety rules via Autopoiesis
// - Escalating ambiguous cases to human-in-the-loop
package system

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ConstitutionConfig holds configuration for the constitution gate.
type ConstitutionConfig struct {
	// Safety thresholds
	StrictMode           bool     // Block all actions not explicitly permitted (default: true)
	AllowedDomains       []string // Network allowlist
	DangerousPatterns    []string // Command patterns that trigger dangerous_action
	EscalateOnAmbiguity  bool     // Ask user for ambiguous cases (default: true)

	// Performance
	TickInterval time.Duration // How often to check pending actions (default: 50ms)
}

// DefaultConstitutionConfig returns sensible defaults.
func DefaultConstitutionConfig() ConstitutionConfig {
	return ConstitutionConfig{
		StrictMode: true,
		AllowedDomains: []string{
			"github.com",
			"golang.org",
			"pkg.go.dev",
			"docs.anthropic.com",
			"developer.mozilla.org",
		},
		DangerousPatterns: []string{
			`rm\s+-rf`,
			`mkfs`,
			`dd\s+if=`,
			`chmod\s+777`,
			`curl.*\|.*sh`,
			`wget.*\|.*sh`,
			`>.*\/etc\/`,
			`sudo\s+rm`,
		},
		EscalateOnAmbiguity: true,
		TickInterval:        50 * time.Millisecond,
	}
}

// SecurityViolation represents a blocked action.
type SecurityViolation struct {
	Timestamp   time.Time
	ActionType  string
	Target      string
	Reason      string
	Context     map[string]string
	WasEscalated bool
}

// ConstitutionGateShard is the safety enforcement system shard.
// It runs continuously and gates all pending actions through constitutional checks.
type ConstitutionGateShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config ConstitutionConfig

	// Compiled patterns
	dangerousPatterns []*regexp.Regexp

	// Audit trail
	violations []SecurityViolation
	permitted  []string // Actions that were permitted (for audit)

	// State
	running bool
}

// NewConstitutionGateShard creates a new Constitution Gate shard.
func NewConstitutionGateShard() *ConstitutionGateShard {
	return NewConstitutionGateShardWithConfig(DefaultConstitutionConfig())
}

// NewConstitutionGateShardWithConfig creates a constitution gate with custom config.
func NewConstitutionGateShardWithConfig(cfg ConstitutionConfig) *ConstitutionGateShard {
	base := NewBaseSystemShard("constitution_gate", StartupAuto)

	// Override permissions for constitution gate - minimal footprint
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionAskUser, // Only for escalation
	}
	base.Config.Model = core.ModelConfig{} // No LLM by default - pure logic

	shard := &ConstitutionGateShard{
		BaseSystemShard: base,
		config:          cfg,
		violations:      make([]SecurityViolation, 0),
		permitted:       make([]string, 0),
	}

	// Compile dangerous patterns
	shard.dangerousPatterns = make([]*regexp.Regexp, 0, len(cfg.DangerousPatterns))
	for _, pattern := range cfg.DangerousPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			shard.dangerousPatterns = append(shard.dangerousPatterns, re)
		}
	}

	return shard
}

// Execute runs the Constitution Gate's continuous safety loop.
// This shard is AUTO-START and runs for the entire session.
func (c *ConstitutionGateShard) Execute(ctx context.Context, task string) (string, error) {
	c.SetState(core.ShardStateRunning)
	c.mu.Lock()
	c.running = true
	c.StartTime = time.Now()
	c.mu.Unlock()

	defer func() {
		c.SetState(core.ShardStateCompleted)
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	// Initialize kernel if not set
	if c.Kernel == nil {
		c.Kernel = core.NewRealKernel()
	}

	ticker := time.NewTicker(c.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return c.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-c.StopCh:
			return c.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			if err := c.processPendingActions(ctx); err != nil {
				// Log error but continue - constitution must not crash
				c.recordViolation("internal_error", "", err.Error(), nil)
			}

			// Emit heartbeat
			_ = c.EmitHeartbeat()

			// Check for autopoiesis opportunity
			if c.Autopoiesis.ShouldPropose() {
				c.handleAutopoiesis(ctx)
			}
		}
	}
}

// processPendingActions checks all pending actions against constitutional rules.
func (c *ConstitutionGateShard) processPendingActions(ctx context.Context) error {
	if c.Kernel == nil {
		return nil
	}

	// Query pending actions
	pending, err := c.Kernel.Query("pending_action")
	if err != nil {
		return fmt.Errorf("failed to query pending_action: %w", err)
	}

	for _, fact := range pending {
		if len(fact.Args) < 1 {
			continue
		}

		actionType, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		var target string
		if len(fact.Args) > 1 {
			target, _ = fact.Args[1].(string)
		}

		// Check if action is permitted
		permitted, reason := c.checkPermitted(ctx, actionType, target)

		if permitted {
			// Mark as permitted
			_ = c.Kernel.Assert(core.Fact{
				Predicate: "action_permitted",
				Args:      []interface{}{actionType, target, time.Now().Unix()},
			})
			c.mu.Lock()
			c.permitted = append(c.permitted, actionType)
			c.mu.Unlock()
		} else {
			// Record violation
			c.recordViolation(actionType, target, reason, nil)

			// Emit security_violation fact
			_ = c.Kernel.Assert(core.Fact{
				Predicate: "security_violation",
				Args:      []interface{}{actionType, reason, time.Now().Unix()},
			})

			// Check if we should escalate to user
			if c.config.EscalateOnAmbiguity && c.shouldEscalate(reason) {
				c.escalateToUser(ctx, actionType, target, reason)
			}
		}

		// Clear the pending action regardless of result
		_ = c.Kernel.Retract("pending_action")
	}

	return nil
}

// checkPermitted determines if an action is permitted under constitutional rules.
func (c *ConstitutionGateShard) checkPermitted(ctx context.Context, actionType, target string) (bool, string) {
	// 1. Check for dangerous patterns in target
	if c.isDangerous(target) {
		return false, "matches dangerous command pattern"
	}

	// 2. Check network policy for network actions
	if actionType == "network" || actionType == "fetch" || actionType == "browse" {
		if !c.isAllowedDomain(target) {
			return false, fmt.Sprintf("domain not in allowlist: %s", target)
		}
	}

	// 3. Query Mangle for permitted(Action)
	// The kernel should derive permitted(Action) from safe_action or admin_override
	results, err := c.Kernel.Query(fmt.Sprintf("permitted(%s)", actionType))
	if err != nil {
		// If query fails, default deny in strict mode
		if c.config.StrictMode {
			return false, "query failed and strict mode enabled"
		}
		// Record as unhandled for autopoiesis
		c.Autopoiesis.RecordUnhandled(
			fmt.Sprintf("permitted(%s)", actionType),
			map[string]string{"action": actionType, "target": target},
			nil,
		)
		return true, "" // Allow if not strict
	}

	if len(results) == 0 {
		if c.config.StrictMode {
			// Record as unhandled for autopoiesis
			c.Autopoiesis.RecordUnhandled(
				fmt.Sprintf("permitted(%s)", actionType),
				map[string]string{"action": actionType, "target": target},
				nil,
			)
			return false, "not explicitly permitted (default deny)"
		}
		return true, ""
	}

	return true, ""
}

// isDangerous checks if a command matches dangerous patterns.
func (c *ConstitutionGateShard) isDangerous(target string) bool {
	for _, pattern := range c.dangerousPatterns {
		if pattern.MatchString(target) {
			return true
		}
	}
	return false
}

// isAllowedDomain checks if a URL/domain is in the allowlist.
func (c *ConstitutionGateShard) isAllowedDomain(target string) bool {
	target = strings.ToLower(target)
	for _, domain := range c.config.AllowedDomains {
		if strings.Contains(target, strings.ToLower(domain)) {
			return true
		}
	}
	return false
}

// shouldEscalate determines if a violation should be escalated to the user.
func (c *ConstitutionGateShard) shouldEscalate(reason string) bool {
	// Escalate on ambiguous cases (not explicit violations)
	ambiguousReasons := []string{
		"not explicitly permitted",
		"query failed",
		"domain not in allowlist",
	}
	for _, r := range ambiguousReasons {
		if strings.Contains(reason, r) {
			return true
		}
	}
	return false
}

// escalateToUser asks the user for permission on ambiguous cases.
func (c *ConstitutionGateShard) escalateToUser(ctx context.Context, actionType, target, reason string) {
	// Emit an escalation fact for the UI layer to handle
	_ = c.Kernel.Assert(core.Fact{
		Predicate: "escalation_needed",
		Args: []interface{}{
			"constitution_gate",
			actionType,
			target,
			reason,
			time.Now().Unix(),
		},
	})
}

// recordViolation records a security violation for audit.
func (c *ConstitutionGateShard) recordViolation(actionType, target, reason string, ctx map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.violations = append(c.violations, SecurityViolation{
		Timestamp:  time.Now(),
		ActionType: actionType,
		Target:     target,
		Reason:     reason,
		Context:    ctx,
	})
}

// handleAutopoiesis uses LLM to propose new constitutional rules.
func (c *ConstitutionGateShard) handleAutopoiesis(ctx context.Context) {
	cases := c.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	// If no LLM, we can't propose rules - just log
	if c.LLMClient == nil {
		return
	}

	// Check cost guard
	can, _ := c.CostGuard.CanCall()
	if !can {
		// Put cases back for later
		for _, cas := range cases {
			c.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Build prompt for rule proposal
	prompt := c.buildRuleProposalPrompt(cases)

	result, err := c.GuardedLLMCall(ctx, constitutionAutopoiesisPrompt, prompt)
	if err != nil {
		// Put cases back for later
		for _, cas := range cases {
			c.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Parse proposed rule
	proposedRule := c.parseProposedRule(result, cases)
	if proposedRule.MangleCode == "" {
		return
	}

	c.Autopoiesis.RecordProposal(proposedRule)

	// If high confidence, auto-apply; otherwise escalate
	if proposedRule.Confidence >= c.Autopoiesis.RuleConfidence {
		// Hot-load the rule
		if err := c.Kernel.HotLoadRule(proposedRule.MangleCode); err == nil {
			c.Autopoiesis.RecordApplied(proposedRule.MangleCode)
		}
	} else {
		// Escalate for human approval
		_ = c.Kernel.Assert(core.Fact{
			Predicate: "rule_proposal_pending",
			Args: []interface{}{
				"constitution_gate",
				proposedRule.MangleCode,
				proposedRule.Rationale,
				proposedRule.Confidence,
				time.Now().Unix(),
			},
		})
	}
}

// buildRuleProposalPrompt creates a prompt for the LLM to propose a new rule.
func (c *ConstitutionGateShard) buildRuleProposalPrompt(cases []UnhandledCase) string {
	var sb strings.Builder
	sb.WriteString("The following actions were not handled by existing constitutional rules:\n\n")

	for i, cas := range cases {
		sb.WriteString(fmt.Sprintf("%d. Query: %s\n", i+1, cas.Query))
		if cas.Context != nil {
			for k, v := range cas.Context {
				sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nPropose a Mangle rule that would handle these cases safely.\n")
	sb.WriteString("Format your response as:\n")
	sb.WriteString("RULE: <mangle code>\n")
	sb.WriteString("CONFIDENCE: <0.0-1.0>\n")
	sb.WriteString("RATIONALE: <explanation>\n")

	return sb.String()
}

// parseProposedRule extracts a proposed rule from LLM output.
func (c *ConstitutionGateShard) parseProposedRule(output string, cases []UnhandledCase) ProposedRule {
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
func (c *ConstitutionGateShard) generateShutdownSummary(reason string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return fmt.Sprintf(
		"Constitution Gate shutdown (%s). Violations: %d, Permitted: %d, Runtime: %s",
		reason,
		len(c.violations),
		len(c.permitted),
		time.Since(c.StartTime).String(),
	)
}

// GetViolations returns the audit trail of security violations.
func (c *ConstitutionGateShard) GetViolations() []SecurityViolation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]SecurityViolation, len(c.violations))
	copy(result, c.violations)
	return result
}

// AddAllowedDomain adds a domain to the network allowlist.
func (c *ConstitutionGateShard) AddAllowedDomain(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.AllowedDomains = append(c.config.AllowedDomains, domain)
}

// AddDangerousPattern adds a pattern to the dangerous action list.
func (c *ConstitutionGateShard) AddDangerousPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.DangerousPatterns = append(c.config.DangerousPatterns, pattern)
	c.dangerousPatterns = append(c.dangerousPatterns, re)
	return nil
}

// constitutionAutopoiesisPrompt is the system prompt for proposing new safety rules.
const constitutionAutopoiesisPrompt = `You are the Constitution Gate's Autopoiesis system.
Your role is to propose new Mangle safety rules based on unhandled action patterns.

Rules you propose MUST:
1. Follow the permitted(Action) pattern
2. Be conservative - when in doubt, deny
3. Be specific - avoid overly broad rules
4. Be safe - never propose rules that could bypass safety checks

Example valid rules:
- safe_action(/read_file).
- permitted(/search) :- user_intent(_, /query, _, _, _).
- dangerous_action(Action) :- action_contains_pattern(Action, "rm -rf").

DO NOT propose rules that:
- Grant blanket permissions
- Bypass admin_override requirements for dangerous actions
- Allow unrestricted network access
- Could enable code injection or arbitrary execution`
