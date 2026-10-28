package system

import (
	"fmt"
	"path/filepath"

	"codenerd/internal/approval"
	"codenerd/internal/logging"
)

// approvalJournalPath is where a workspace's approval lifecycle is recorded.
func approvalJournalPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "approvals", "journal.jsonl")
}

// initApprovalBroker opens the process's human-approval broker for the
// workspace, with the bounds from the `approval` config section. A journal
// that cannot be opened leaves the boot without a broker: every approvable
// denial then stays a refusal, which is the fail-closed direction.
func initApprovalBroker(bctx *bootContext) error {
	policy, err := bctx.appCfg.GetApprovalConfig().Resolve()
	if err != nil {
		return fmt.Errorf("approval config: %w", err)
	}
	if bctx.workspace == "" {
		return nil
	}
	broker, release, err := approval.OpenShared(approval.Options{
		TTL:              policy.RequestTTL,
		PreviewMaxBytes:  policy.PreviewMaxBytes,
		JournalRetention: policy.JournalRetention,
		JournalPath:      approvalJournalPath(bctx.workspace),
	})
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Human approval unavailable this session (approvable actions stay refused): %v", err)
		return nil
	}
	bctx.approvals = broker
	bctx.approvalsRelease = release
	return nil
}
