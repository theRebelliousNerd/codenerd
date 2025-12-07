package coder

import (
	"fmt"
	"strings"
)

// =============================================================================
// SAFETY CHECKS
// =============================================================================

// checkImpact checks if editing the target would have unsafe impact.
func (c *CoderShard) checkImpact(target string) (blocked bool, reason string) {
	if c.kernel == nil {
		return false, ""
	}

	// Query for block conditions
	results, err := c.kernel.Query("coder_block_write")
	if err != nil {
		return false, ""
	}

	for _, fact := range results {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[0].(string); ok && file == target {
				if r, ok := fact.Args[1].(string); ok {
					return true, r
				}
			}
		}
	}

	// Check Code DOM edit_unsafe predicate
	unsafeResults, _ := c.kernel.Query("edit_unsafe")
	for _, fact := range unsafeResults {
		if len(fact.Args) >= 2 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if reason, ok := fact.Args[1].(string); ok {
					return true, fmt.Sprintf("Code DOM safety: %s", reason)
				}
			}
		}
	}

	// Check for critical breaking change risk
	breakingResults, _ := c.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if level, ok := fact.Args[1].(string); ok && level == "/critical" {
					if reason, ok := fact.Args[2].(string); ok {
						return true, fmt.Sprintf("Critical breaking change: %s", reason)
					}
				}
			}
		}
	}

	return false, ""
}
