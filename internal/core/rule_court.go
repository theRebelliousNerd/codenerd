package core

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ratifyEvalTimeout bounds sandbox.Evaluate() during rule ratification. A
// proposed rule that cannot reach fixpoint within this window is treated as
// a VETO — Mangle evaluation should be polynomial in fact count, so anything
// substantially slower is either a runaway recursion or an adversarial
// rule. Exposed as a var so tests may shorten it.
var ratifyEvalTimeout = 5 * time.Second

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

	// Reject malformed input before the parser sees it. The Mangle parser
	// tolerates embedded null bytes and invalid UTF-8 inside string literals,
	// which can produce facts that are unsafe to round-trip through downstream
	// systems. Catch both here as syntactic violations.
	if strings.ContainsRune(newRule, '\x00') {
		return fmt.Errorf("rule rejected by sandbox compiler: contains null byte")
	}
	if !utf8.ValidString(newRule) {
		return fmt.Errorf("rule rejected by sandbox compiler: invalid UTF-8 sequence")
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

	// Bounded sandbox.Evaluate() — runaway derivation (e.g., cyclic recursive
	// rules) is treated as a VETO so a hallucinated rule cannot hang the
	// ratification loop indefinitely. Evaluate itself does not accept a
	// context, so we run it in a goroutine and race the result against a
	// timeout.
	evalCtx, cancel := context.WithTimeout(context.Background(), ratifyEvalTimeout)
	defer cancel()
	evalDone := make(chan error, 1)
	go func() {
		evalDone <- sandbox.Evaluate()
	}()
	select {
	case err := <-evalDone:
		if err != nil {
			return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
		}
	case <-evalCtx.Done():
		return fmt.Errorf("VETO: sandbox evaluation timed out after %s (possible runaway recursion)", ratifyEvalTimeout)
	}

	// Liveness check: only veto if the rule eliminates an existing permitted
	// action set. If the base kernel itself has no permitted actions (e.g.,
	// during quiescent boot or a schema-less bootstrap), we cannot infer
	// anything about deadlock from this query alone and skip the veto.
	basePermitted, baseErr := kernel.Query("permitted")
	permitted, err := sandbox.Query("permitted")
	if err != nil {
		return fmt.Errorf("rule rejected by sandbox query: %w", err)
	}
	if baseErr == nil && len(basePermitted) > 0 && len(permitted) == 0 {
		return fmt.Errorf("VETO: rule causes total system deadlock (no permitted actions)")
	}

	// Safety hatch check: never block ask_user
	if strings.Contains(newRule, "ask_user") {
		return fmt.Errorf("VETO: cannot forbid emergency hatch 'ask_user'")
	}

	return nil
}
