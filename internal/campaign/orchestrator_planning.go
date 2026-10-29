package campaign

import "context"

// Decompose plans a campaign with the orchestrator's own decomposer: the
// planning client, the prompt provider (compiled for that client's model),
// shard discovery and the policy-stamped intelligence gatherer whose report
// reaches the planner's prompt (Step 0).
//
// Doors plan through here rather than with a decomposer of their own, so
// decomposition and every later replan run on one client and one wiring.
// Until 2026-10-27 the orchestrator built this decomposer and fed it every
// sensor while each door planned with a second, bare one on the main client.
func (o *Orchestrator) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	return o.decomposer.Decompose(ctx, req)
}
