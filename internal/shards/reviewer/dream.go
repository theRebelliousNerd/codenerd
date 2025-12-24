// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains dream mode (simulation/learning) functionality.
package reviewer

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
)

// =============================================================================
// DREAM MODE (Simulation/Learning)
// =============================================================================

// describeDreamPlan returns a description of what the reviewer would do WITHOUT executing.
func (r *ReviewerShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	logging.ReviewerDebug("DREAM MODE - describing plan without execution")

	if r.llmClient == nil {
		return "ReviewerShard would analyze code for issues, but no LLM client available for dream description.", nil
	}

	prompt := fmt.Sprintf(`You are a code reviewer agent in DREAM MODE. Describe what you WOULD do for this task WITHOUT actually doing it.

Task: %s

Provide a structured analysis:
1. **Understanding**: What kind of review is being asked?
2. **Files to Review**: What files would I examine?
3. **Review Approach**: What checks would I perform? (style, security, complexity, etc.)
4. **Tools Needed**: What analysis tools would I use?
5. **Potential Findings**: What types of issues might I look for?
6. **Questions**: What would I need clarified?

Remember: This is a simulation. Describe the plan, don't execute it.`, task)

	response, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("ReviewerShard dream analysis failed: %v", err), nil
	}

	return response, nil
}
