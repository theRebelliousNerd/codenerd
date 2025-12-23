package coder

import (
	"codenerd/internal/logging"
	"fmt"
	"strings"
)

// =============================================================================
// SAFETY CHECKS
// =============================================================================

// checkImpact checks if editing the target would have unsafe impact.
func (c *CoderShard) checkImpact(target string) (blocked bool, reason string) {
	logging.CoderDebug("checkImpact: checking safety for target=%s", target)

	if c.kernel == nil {
		logging.CoderDebug("checkImpact: no kernel available, skipping safety checks")
		return false, ""
	}

	// Query for block conditions
	results, err := c.kernel.Query("coder_block_write")
	if err != nil {
		logging.CoderWarn("checkImpact: failed to query coder_block_write: %v", err)
		return false, ""
	}

	for _, fact := range results {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[0].(string); ok && file == target {
				if r, ok := fact.Args[1].(string); ok {
					logging.Coder("checkImpact: BLOCKED target=%s reason=%s", target, r)
					return true, r
				}
			}
		}
	}

	// Check Code DOM edit_unsafe predicate
	unsafeResults, err := c.kernel.Query("edit_unsafe")
	if err != nil {
		logging.CoderWarn("checkImpact: failed to query edit_unsafe: %v", err)
	}
	for _, fact := range unsafeResults {
		if len(fact.Args) >= 2 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if reason, ok := fact.Args[1].(string); ok {
					blockReason := fmt.Sprintf("Code DOM safety: %s", reason)
					logging.Coder("checkImpact: BLOCKED target=%s reason=%s", target, blockReason)
					return true, blockReason
				}
			}
		}
	}

	// Check for critical breaking change risk
	breakingResults, err := c.kernel.Query("breaking_change_risk")
	if err != nil {
		logging.CoderWarn("checkImpact: failed to query breaking_change_risk: %v", err)
	}
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if level, ok := fact.Args[1].(string); ok && level == "/critical" {
					if reason, ok := fact.Args[2].(string); ok {
						blockReason := fmt.Sprintf("Critical breaking change: %s", reason)
						logging.Coder("checkImpact: BLOCKED target=%s reason=%s", target, blockReason)
						return true, blockReason
					}
				}
			}
		}
	}

	logging.CoderDebug("checkImpact: target=%s passed all safety checks", target)
	return false, ""
}
