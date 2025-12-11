package core

import (
	"fmt"
	"strings"
)

// RuleCourt validates proposed policy rules before they are learned.
type RuleCourt struct {
	kernel *RealKernel
}

// NewRuleCourt creates a court backed by the provided kernel.
func NewRuleCourt(kernel *RealKernel) *RuleCourt {
	return &RuleCourt{kernel: kernel}
}

// RatifyRule validates a proposed rule against constitutional safety.
// It returns an error if the rule would deadlock the system or block emergency hatches.
func (c *RuleCourt) RatifyRule(newRule string) error {
	return RatifyRule(c.kernel, newRule)
}

// RatifyRule validates a rule using a sandboxed kernel.
func RatifyRule(kernel *RealKernel, newRule string) error {
	newRule = strings.TrimSpace(newRule)
	if newRule == "" {
		return fmt.Errorf("empty rule")
	}

	if kernel == nil {
		return fmt.Errorf("no kernel available for ratification")
	}

	// Build sandbox with current schemas/policy/learned rules
	sandbox, err := NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create sandbox kernel for ratification: %w", err)
	}
	sandbox.SetSchemas(kernel.GetSchemas())
	sandbox.SetPolicy(kernel.GetPolicy() + "\n\n# Proposed Rule (Legislator)\n" + newRule)
	sandbox.SetLearned(kernel.GetLearned())

	// Hydrate with current facts for liveness checks
	facts := kernel.GetFactsSnapshot()
	if len(facts) > 0 {
		_ = sandbox.LoadFacts(facts)
	}

	if err := sandbox.Evaluate(); err != nil {
		return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
	}

	// Liveness check: ensure at least one permitted action remains
	permitted, err := sandbox.Query("permitted")
	if err != nil || len(permitted) == 0 {
		return fmt.Errorf("VETO: rule causes total system deadlock (no permitted actions)")
	}

	// Safety hatch check: never block ask_user
	if strings.Contains(newRule, "ask_user") {
		return fmt.Errorf("VETO: cannot forbid emergency hatch 'ask_user'")
	}

	return nil
}
